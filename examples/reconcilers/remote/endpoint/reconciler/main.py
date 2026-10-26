#!/usr/bin/env python3
"""
Remote reconciler for the WebApp CRD in Python, standard library only. It
answers POST /reconcile with intent-form resources; see the README for the
contract.

    python3 main.py                              # listens on :8025
    PORT=9000 python3 main.py                    # custom port
    RECONCILER_TOKEN=secret python3 main.py      # with Bearer auth
"""

import json
import logging
import os
from http.server import BaseHTTPRequestHandler, HTTPServer

logging.basicConfig(level=logging.INFO, format="%(asctime)s  %(levelname)s  %(message)s")
log = logging.getLogger(__name__)

TOKEN = os.environ.get("RECONCILER_TOKEN", "")


def reconcile(body: dict) -> dict:
    obj  = body.get("object") or {}
    meta = obj.get("metadata", {})
    spec = obj.get("spec", {})
    args = body.get("args", {})

    # Prefer injected args where available; fall back to object fields.
    name      = args.get("appName") or meta.get("name", "webapp")
    namespace = meta.get("namespace", "default")
    image     = spec.get("image", "nginx:latest")
    replicas  = spec.get("replicas", 1)
    port      = spec.get("port", 80)
    expose    = spec.get("expose", True) is not False

    log_level   = args.get("logLevel", "info")
    environment = args.get("environment", "development")

    log.info("reconcile  key=%s/%s  image=%s  logLevel=%s  env=%s",
             namespace, name, image, log_level, environment)

    resources = [
        {
            "type": "deployment",
            "fields": {
                "name":     name,
                "image":    image,
                "replicas": replicas,
                "port":     port,
            },
        },
    ]
    # Without expose the Service is left out; prune deletes the one applied before.
    if expose:
        resources.append(
            {
                "type": "service",
                "fields": {
                    "name":       f"{name}-svc",
                    "port":       80,
                    "targetPort": port,
                },
            }
        )

    return {
        "result": "ok",
        "status": {
            "phase":    "Running",
            "endpoint": f"{name}-svc.{namespace}.svc.cluster.local",
            "replicas": replicas,
        },
        # Set on the WebApp itself; dropping a key here removes it.
        "labels":      {"environment": environment},
        "annotations": {"webapp.example.com/image": image},
        "resources": resources,
    }


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        if self.path != "/reconcile":
            self.send_response(404)
            self.end_headers()
            return

        if TOKEN:
            auth = self.headers.get("Authorization", "")
            if auth != f"Bearer {TOKEN}":
                log.warning("rejected: got %r", auth)
                self.send_response(401)
                self.end_headers()
                self.wfile.write(b"unauthorized")
                return

        length = int(self.headers.get("Content-Length", 0))
        try:
            body = json.loads(self.rfile.read(length))
        except json.JSONDecodeError as exc:
            log.error("bad request body: %s", exc)
            self.send_response(400)
            self.end_headers()
            return

        try:
            result = reconcile(body)
        except Exception as exc:  # noqa: BLE001
            log.exception("reconcile error")
            result = {"result": "error", "error": str(exc)}

        payload = json.dumps(result).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def log_message(self, fmt, *args):
        pass  # silence http.server's per-request access log; reconcile() logs each call



if __name__ == "__main__":
    port = int(os.environ.get("PORT", 8025))
    log.info("python reconciler listening on :%d%s", port,
             "  (auth enabled)" if TOKEN else "  (no auth)")
    HTTPServer(("", port), Handler).serve_forever()
