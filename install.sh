#!/usr/bin/env bash
# Inrun CLI installer
#
# Usage:
#   curl -sSL https://get.inrun.dev | bash
#
# Environment variables:
#   INRUN_VERSION           — pin a specific version tag (default: latest release)
#   INRUN_INSTALL_DIR       — install directory (default: ~/.inrun/bin)
#   INRUN_SKIP_CONSOLE      — skip the console binary (default: false)
#   INRUN_SKIP_COMPLETION   — skip shell completion setup (default: false)
#   INRUN_REQUIRE_SIGNATURE — fail when cosign is not installed (default: false)
#
# Verification: checksums.txt is signed keyless with cosign by the release
# workflow. With cosign installed, the installer checks that signature, then
# every archive against checksums.txt. Without it, only the checksums are
# checked.
#
# 1. Add the binary name to BINARIES.
# 2. If it should be skippable, add a case entry in component_skipped().
# That is the only change needed — download URL, checksum, and extraction
# all follow the same convention automatically.
#
set -euo pipefail

# Config
REPO="intentd/inrun"
INSTALL_DIR="${INRUN_INSTALL_DIR:-${HOME}/.inrun/bin}"
VERSION="${INRUN_VERSION:-}"
SKIP_CC="${INRUN_SKIP_CONSOLE:-false}"
SKIP_COMPLETION="${INRUN_SKIP_COMPLETION:-false}"
REQUIRE_SIGNATURE="${INRUN_REQUIRE_SIGNATURE:-false}"
CERT_IDENTITY_PREFIX="https://github.com/${REPO}/.github/workflows/sign-and-release.yml@refs/tags/"
OIDC_ISSUER="https://token.actions.githubusercontent.com"

# All Inrun CLI binaries, in install order.
# To add a new binary, append it here.
BINARIES=("inrun" "inrun-console")

# Colours
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
RESET='\033[0m'

info()    { echo -e "${BLUE}[inrun]${RESET} $*"; }
success() { echo -e "${GREEN}[inrun]${RESET} $*"; }
warn()    { echo -e "${YELLOW}[inrun]${RESET} $*"; }
error()   { echo -e "${RED}[inrun]${RESET} $*" >&2; }
fatal()   { error "$*"; exit 1; }

# Banner
echo -e "${BOLD}Inrun${RESET} installer"
echo

# Dependencies
check_deps() {
    for cmd in curl tar grep sed; do
        command -v "${cmd}" >/dev/null 2>&1 \
            || fatal "Required command not found: ${cmd}"
    done
}

# Platform detection
detect_platform() {
    local os arch

    case "$(uname -s)" in
        Linux)   os="linux"  ;;
        Darwin)  os="darwin" ;;
        *)       fatal "Unsupported OS: $(uname -s). Use WSL on Windows." ;;
    esac

    case "$(uname -m)" in
        x86_64)        arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *)             fatal "Unsupported architecture: $(uname -m)" ;;
    esac

    echo "${os}_${arch}"
}

# Version resolution
resolve_version() {
    if [[ -n "${VERSION}" ]]; then
        echo "${VERSION}"
        return
    fi

    info "Fetching latest version..." >&2

    local tag
    tag=$(curl -sSf "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' \
        | sed -E 's/.*"tag_name": "([^"]+)".*/\1/')

    if [[ -z "${tag}" ]]; then
        fatal "Could not resolve latest version. Check your network or set INRUN_VERSION explicitly."
    fi

    echo "${tag}"
}

# Component skip rules
# Returns 0 (true) when the binary should be skipped.
# Add a case entry here when a new binary needs a skip flag.
component_skipped() {
    local binary="$1"
    case "${binary}" in
        inrun-console) [[ "${SKIP_CC}" == "true" ]] ;;
        *)     return 1 ;;
    esac
}

# Checksum verification
# Uses -c (POSIX short form) which works on GNU coreutils, BusyBox, and macOS.
# --check is GNU-only and fails on BusyBox (Alpine/CI containers).
verify_checksum() {
    local archive="$1"
    local checksums_file="$2"

    if command -v sha256sum >/dev/null 2>&1; then
        grep "${archive}" "${checksums_file}" | sha256sum -c \
            || fatal "Checksum verification failed for ${archive}"
    elif command -v shasum >/dev/null 2>&1; then
        grep "${archive}" "${checksums_file}" | shasum -a 256 -c \
            || fatal "Checksum verification failed for ${archive}"
    else
        fatal "No checksum tool found (sha256sum / shasum)"
    fi
}

# Signature verification
# Fetches checksums.txt and its cosign bundle once, and checks the signature
# was made by Inrun's release workflow for this tag.
fetch_checksums() {
    local version="$1"
    local dir="$2"
    local base="https://github.com/${REPO}/releases/download/${version}"

    curl -sSfL "${base}/checksums.txt" -o "${dir}/checksums.txt" \
        || fatal "checksums.txt not found for ${version}"

    if ! command -v cosign >/dev/null 2>&1; then
        if [[ "${REQUIRE_SIGNATURE}" == "true" ]]; then
            fatal "cosign not found and INRUN_REQUIRE_SIGNATURE=true. Install cosign: https://docs.sigstore.dev/cosign/system_config/installation/"
        fi
        warn "cosign not found: checking checksums only, not their signature"
        return 0
    fi

    curl -sSfL "${base}/checksums.txt.sigstore.json" -o "${dir}/checksums.txt.sigstore.json" \
        || fatal "Signature bundle not found for ${version}"

    info "Verifying the release signature..."
    cosign verify-blob "${dir}/checksums.txt" \
        --bundle "${dir}/checksums.txt.sigstore.json" \
        --certificate-identity "${CERT_IDENTITY_PREFIX}${version}" \
        --certificate-oidc-issuer "${OIDC_ISSUER}" >/dev/null 2>&1 \
        || fatal "Signature verification failed for checksums.txt (${version})"
    success "Signature verified"
}

# Single component installer
install_component() {
    local binary="$1"
    local platform="$2"
    local version="$3"
    local checksums="$4"

    if component_skipped "${binary}"; then
        info "Skipping ${binary} (disabled by flag)"
        return 0
    fi

    local archive="${binary}_${platform}.tar.gz"
    local download_url="https://github.com/${REPO}/releases/download/${version}/${archive}"

    local tmp_dir
    tmp_dir=$(mktemp -d)
    # shellcheck disable=SC2064
    trap "rm -rf '${tmp_dir}'" RETURN

    info "Downloading ${archive}..."
    if ! curl -sSfL "${download_url}" -o "${tmp_dir}/${archive}"; then
        warn "${binary} not available for ${version} — skipping"
        return 0
    fi

    info "Verifying checksum..."
    (cd "${tmp_dir}" && verify_checksum "${archive}" "${checksums}")

    info "Extracting ${archive}..."
    tar -xzf "${tmp_dir}/${archive}" -C "${tmp_dir}"

    local src="${tmp_dir}/${binary}"
    if [[ ! -f "${src}" ]]; then
        fatal "Expected binary '${binary}' not found in archive"
    fi

    mkdir -p "${INSTALL_DIR}"
    install -m 755 "${src}" "${INSTALL_DIR}/${binary}"

    success "${binary} ${version} installed to ${INSTALL_DIR}/${binary}"
}

# PATH setup
# Adds INSTALL_DIR to PATH in the user's shell profile if not already present.
setup_path() {
    local export_line="export PATH=\"${INSTALL_DIR}:\$PATH\""

    # Detect which profile file to update
    local profile_file
    local shell_name
    shell_name=$(basename "${SHELL:-bash}")

    case "${shell_name}" in
        zsh)  profile_file="${ZDOTDIR:-${HOME}}/.zshrc" ;;
        fish) profile_file="${HOME}/.config/fish/config.fish"
              export_line="set -gx PATH \"${INSTALL_DIR}\" \$PATH" ;;
        *)    profile_file="${HOME}/.bashrc" ;;
    esac

    # Check if already in PATH or profile
    if echo "${PATH}" | tr ':' '\n' | grep -qxF "${INSTALL_DIR}"; then
        return 0
    fi

    if [[ -f "${profile_file}" ]] && grep -qF "${INSTALL_DIR}" "${profile_file}"; then
        return 0
    fi

    echo "" >> "${profile_file}"
    echo "# Inrun CLI" >> "${profile_file}"
    echo "${export_line}" >> "${profile_file}"

    info "Added ${INSTALL_DIR} to PATH in ${profile_file}"
    warn "Run 'exec \$SHELL' to reload your shell, or open a new terminal"
}

# Shell completion
install_completion() {
    if [[ "${SKIP_COMPLETION}" == "true" ]]; then
        info "Skipping shell completion (INRUN_SKIP_COMPLETION=true)"
        return
    fi

    local shell_name
    shell_name=$(basename "${SHELL:-bash}")

    case "${shell_name}" in
        bash)
            local dir="${HOME}/.bash_completion.d"
            mkdir -p "${dir}"
            info "Installing bash completion → ${dir}/inrun"
            "${INSTALL_DIR}/inrun" completion bash > "${dir}/inrun" 2>/dev/null || true

            local rc="${HOME}/.bashrc"
            local source_line="source ${dir}/inrun"
            if [[ -f "${rc}" ]] && grep -qF "${source_line}" "${rc}"; then
                :
            else
                echo "" >> "${rc}"
                echo "# Inrun CLI completion" >> "${rc}"
                echo "${source_line}" >> "${rc}"
            fi
            ;;
        zsh)
            local dir="${HOME}/.zsh/completions"
            mkdir -p "${dir}"
            info "Installing zsh completion → ${dir}/_inrun"
            "${INSTALL_DIR}/inrun" completion zsh > "${dir}/_inrun" 2>/dev/null || true

            local rc="${ZDOTDIR:-${HOME}}/.zshrc"
            local fpath_line="fpath=(${dir} \$fpath)"
            if [[ -f "${rc}" ]] && grep -qF "${dir}" "${rc}"; then
                :
            else
                echo "" >> "${rc}"
                echo "# Inrun CLI completion" >> "${rc}"
                echo "${fpath_line}" >> "${rc}"
                echo "autoload -U compinit && compinit" >> "${rc}"
            fi
            ;;
        fish)
            local dir="${HOME}/.config/fish/completions"
            mkdir -p "${dir}"
            info "Installing fish completion → ${dir}/inrun.fish"
            "${INSTALL_DIR}/inrun" completion fish > "${dir}/inrun.fish" 2>/dev/null || true
            # fish auto-loads from this directory — no rc change needed
            ;;
        *)
            warn "Shell '${shell_name}' not recognised — skipping completion."
            warn "Run 'inrun completion <shell>' to generate it manually."
            return
            ;;
    esac

    success "Shell completion installed for ${shell_name}"
}

# Post-install summary

print_summary() {
    local version="$1"
    echo
    success "Installation complete  (${version})"
    echo

    for binary in "${BINARIES[@]}"; do
        if component_skipped "${binary}"; then
            continue
        fi
        local bin_path="${INSTALL_DIR}/${binary}"
        if [[ -x "${bin_path}" ]]; then
            echo -e "  ${GREEN}✓${RESET}  ${bin_path}"
        fi
    done

    echo
    echo -e "  ${BOLD}Get started:${RESET}"
    echo -e "    inrun init my-app            Scaffold a Blueprint, a CRD and a CR"
    echo -e "    inrun validate               Validate a Blueprint"
    echo -e "    inrun                        Run the runtime against your cluster"
    echo -e "    inrun push name:v1 ./dir     Push a pattern to the registry"
    echo
    echo -e "  ${BOLD}Console:${RESET}"
    echo -e "    inrun console                Start the web UI (port 8081)"
    echo
    echo -e "  ${BOLD}Documentation:${RESET}"
    echo -e "    https://inrun.dev/docs/"
    echo
}

# Main
main() {
    check_deps

    local platform version
    platform=$(detect_platform)
    version=$(resolve_version)

    info "Platform: ${platform}  Version: ${version}"
    info "Install directory: ${INSTALL_DIR}"
    echo

    local verify_dir
    verify_dir=$(mktemp -d)
    # shellcheck disable=SC2064
    trap "rm -rf '${verify_dir}'" EXIT
    fetch_checksums "${version}" "${verify_dir}"

    for binary in "${BINARIES[@]}"; do
        install_component "${binary}" "${platform}" "${version}" "${verify_dir}/checksums.txt"
    done

    setup_path
    install_completion

    print_summary "${version}"
}

main
