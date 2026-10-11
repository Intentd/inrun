#!/usr/bin/env bash
# Remote reconciler for the WebApp CRD in bash (needs socat and jq). It
# answers POST /reconcile with intent-form resources; see the README for the
# contract.
#
#   ./reconciler.sh           # listens on :8025
#   PORT=9000 ./reconciler.sh # custom port

PORT="${PORT:-8025}"

HANDLER=$(mktemp /tmp/webapp-handler-XXXX.sh)
chmod +x "$HANDLER"
trap "rm -f $HANDLER" EXIT

cat > "$HANDLER" << 'HANDLER_SCRIPT'
#!/usr/bin/env bash

# ── read HTTP headers ────────────────────────────────────────────────────────
content_length=0
while IFS= read -r line; do
    line="${line%$'\r'}"
    [[ -z "$line" ]] && break
    if [[ "$line" =~ ^Content-Length:[[:space:]]*([0-9]+) ]]; then
        content_length="${BASH_REMATCH[1]}"
    fi
done

body=$(head -c "$content_length")

# ── extract WebApp spec and injected args ────────────────────────────────────
name=$(jq -r      '.object.metadata.name       // "webapp"'   <<< "$body")
namespace=$(jq -r '.object.metadata.namespace  // "default"'  <<< "$body")
image=$(jq -r     '.object.spec.image          // "nginx:latest"' <<< "$body")
replicas=$(jq -r  '.object.spec.replicas       // 1'          <<< "$body")
port=$(jq -r      '.object.spec.port           // 80'         <<< "$body")

app_name=$(jq -r    '.args.appName     // empty' <<< "$body")
log_level=$(jq -r   '.args.logLevel   // "info"' <<< "$body")
environment=$(jq -r '.args.environment // "development"' <<< "$body")

[[ -n "$app_name" ]] && name="$app_name"

echo "$(date)  reconcile  key=$namespace/$name  image=$image  replicas=$replicas  logLevel=$log_level  env=$environment" >&2

# ── build intent-form resources ──────────────────────────────────────────────
deployment=$(jq -cn \
    --arg  name     "$name" \
    --arg  image    "$image" \
    --argjson replicas "$replicas" \
    --argjson port     "$port" \
    '{
        type: "deployment",
        fields: {
            name:     $name,
            image:    $image,
            replicas: $replicas,
            port:     $port
        }
    }')

service=$(jq -cn \
    --arg  svc_name  "${name}-svc" \
    --argjson port   "$port" \
    '{
        type: "service",
        fields: {
            name:       $svc_name,
            port:       80,
            targetPort: $port
        }
    }')

# ── build response ───────────────────────────────────────────────────────────
response=$(jq -cn \
    --argjson deployment "$deployment" \
    --argjson service    "$service" \
    --arg name      "$name" \
    --arg namespace "$namespace" \
    --argjson replicas "$replicas" \
    --arg environment "$environment" \
    --arg image     "$image" \
    '{
        result: "ok",
        status: {
            phase:    "Running",
            endpoint: (($name + "-svc.") + $namespace + ".svc.cluster.local"),
            replicas: $replicas
        },
        labels:      { environment: $environment },
        annotations: { "webapp.example.com/image": $image },
        resources: [$deployment, $service]
    }')

len=${#response}
printf 'HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s' \
    "$len" "$response"
HANDLER_SCRIPT

echo "$(date)  bash reconciler listening on :$PORT" >&2
exec socat TCP-LISTEN:"$PORT",reuseaddr,fork EXEC:"$HANDLER"
