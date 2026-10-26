#!/usr/bin/env python3
"""
Remote reconciler for the Website CRD in Python, standard library only. It
reads only the args the Blueprint maps from the CR (never the CR itself) and
answers POST /reconcile with status and intent-form resources.

    python3 main.py                              # listens on :8025
    PORT=9000 python3 main.py                    # custom port
    RECONCILER_TOKEN=secret python3 main.py      # with Bearer auth
"""

import json
import logging
import os
from dataclasses import dataclass
from http.server import BaseHTTPRequestHandler, HTTPServer

logging.basicConfig(level=logging.INFO, format="%(asctime)s  %(levelname)s  %(message)s")
log = logging.getLogger(__name__)

TOKEN = os.environ.get("RECONCILER_TOKEN", "")


@dataclass
class WebsiteArgs:
    """Typed view of reconcile.args (flat, in this reconciler's own names)."""
    app_name:  str
    image:     str
    namespace: str = "default"
    replicas:  int = 1
    port:      int = 80

    @classmethod
    def from_dict(cls, raw: dict) -> "WebsiteArgs":
        def pick(key, default=...):
            if key in raw:
                return raw[key]
            if default is ...:
                raise ValueError(f"missing required arg: {key}")
            return default

        return cls(
            app_name=pick("appName"),
            image=pick("image"),
            namespace=pick("namespace", "default"),
            replicas=int(pick("replicas", 1)),
            port=int(pick("port", 80)),
        )


def reconcile(body: dict) -> dict:
    # Everything comes from reconcile.args in blueprint.yaml:
    # flat, in this reconciler's own names.
    args = WebsiteArgs.from_dict(body.get("args", {}))

    log.info("reconcile  key=%s/%s  image=%s  replicas=%s  port=%s",
             args.namespace, args.app_name, args.image, args.replicas, args.port)

    return {
        "result": "ok",
        "status": {
            "phase":    "Running",
            "endpoint": f"{args.app_name}-svc.{args.namespace}.svc.cluster.local",
        },
        "resources": [
            {
                "type": "deployment",
                "fields": {
                    "name":     args.app_name,
                    "image":    args.image,
                    "replicas": args.replicas,
                    "port":     args.port,
                },
            },
            {
                "type": "service",
                "fields": {
                    "name":       f"{args.app_name}-svc",
                    "port":       80,
                    "targetPort": args.port,
                },
            },
        ],
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
