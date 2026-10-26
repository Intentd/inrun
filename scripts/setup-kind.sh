#!/usr/bin/env bash
# setup-kind.sh — create or delete a kind cluster for Inrun development
#
# Usage:
#   ./setup-kind.sh            # create cluster named "inrun-playground"
#   ./setup-kind.sh create     # same as above
#   ./setup-kind.sh delete     # delete cluster
#   ./setup-kind.sh <name>     # create cluster with given name
#   ./setup-kind.sh delete <name> # delete cluster with given name
#
# Behavior:
# - Ensures `kind` is installed via `go install sigs.k8s.io/kind@v0.31.0` if missing.
# - Ensures the directory containing the installed binary is on PATH for the session.
# - Default cluster name: inrun estra-playground
# - Exits non-zero on failure.

set -euo pipefail

DEFAULT_NAME="inrun-playground"
KIND_MODULE="sigs.k8s.io/kind@v0.31.0"

# Resolve GOBIN or GOPATH/bin fallback
resolve_bin_dir() {
  if [[ -n "${GOBIN:-}" ]]; then
    echo "$GOBIN"
    return
  fi
  if [[ -n "${GOPATH:-}" ]]; then
    echo "${GOPATH}/bin"
    return
  fi
  # default GOPATH
  echo "$HOME/go/bin"
}

BIN_DIR="$(resolve_bin_dir)"
KIND_BIN="${BIN_DIR}/kind"

log() { printf '%s\n' "$*"; }
err() { printf 'ERROR: %s\n' "$*" >&2; }

ensure_go() {
  if ! command -v go >/dev/null 2>&1; then
    err "go is not installed or not on PATH. Install Go to use 'go install'."
    exit 2
  fi
}

ensure_kind_installed() {
  if command -v kind >/dev/null 2>&1; then
    # kind already on PATH (system or previously installed)
    return 0
  fi

  # If kind binary exists in expected GOBIN/GOPATH/bin, add it to PATH for this session
  if [[ -x "$KIND_BIN" ]]; then
    export PATH="$BIN_DIR:$PATH"
    log "Added $BIN_DIR to PATH for this session."
    return 0
  fi

  # Try to install via `go install`
  ensure_go
  log "Installing kind via: go install ${KIND_MODULE}"
  if go install "${KIND_MODULE}"; then
    # ensure the bin dir is on PATH for this session
    export PATH="$BIN_DIR:$PATH"
    log "Installed kind to ${BIN_DIR}. Added to PATH for this session."
    return 0
  else
    err "go install failed"
    exit 3
  fi
}

create_cluster() {
  local name="$1"
  ensure_kind_installed

  # If cluster already exists, inform and exit successfully
  if kind get clusters | grep -qx "$name"; then
    log "Cluster '$name' already exists. Skipping creation."
    return 0
  fi

  log "Creating kind cluster '$name'..."
  # You can customize the node image or config here if needed
  if kind create cluster --name "$name"; then
    log "Cluster '$name' created successfully."
  else
    err "Failed to create cluster '$name'."
    exit 4
  fi
}

delete_cluster() {
  local name="$1"
  ensure_kind_installed

  if ! kind get clusters | grep -qx "$name"; then
    log "Cluster '$name' does not exist. Nothing to delete."
    return 0
  fi

  log "Deleting kind cluster '$name'..."
  if kind delete cluster --name "$name"; then
    log "Cluster '$name' deleted."
  else
    err "Failed to delete cluster '$name'."
    exit 5
  fi
}

print_usage() {
  cat <<EOF
Usage:
  $0                # create default cluster: ${DEFAULT_NAME}
  $0 create         # same as above
  $0 delete         # delete default cluster
  $0 <name>         # create cluster with name
  $0 delete <name>  # delete cluster with name
EOF
}

# --- main ---
if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  print_usage
  exit 0
fi

case "${1:-}" in
  "" )
    CLUSTER_NAME="$DEFAULT_NAME"
    create_cluster "$CLUSTER_NAME"
    ;;
  create )
    # create [name]
    if [[ -n "${2:-}" ]]; then
      CLUSTER_NAME="$2"
    else
      CLUSTER_NAME="$DEFAULT_NAME"
    fi
    create_cluster "$CLUSTER_NAME"
    ;;
  delete )
    if [[ -n "${2:-}" ]]; then
      CLUSTER_NAME="$2"
    else
      CLUSTER_NAME="$DEFAULT_NAME"
    fi
    delete_cluster "$CLUSTER_NAME"
    ;;
  * )
    # If first arg is not "create" or "delete", treat it as cluster name to create
    if [[ "${1:-}" == "help" ]]; then
      print_usage
      exit 0
    fi
    CLUSTER_NAME="$1"
    create_cluster "$CLUSTER_NAME"
    ;;
esac
