#!/bin/sh
set -e

REPO="joshjon/verve"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
BINARY="verve"

main() {
    need_cmd curl
    need_cmd tar
    need_cmd uname

    os="$(detect_os)"
    arch="$(detect_arch)"
    version="$(resolve_version "${1:-latest}")"

    if [ -z "$version" ]; then
        err "could not determine latest version"
    fi

    # Archive names use title-cased OS and mapped arch (see .goreleaser.yaml)
    archive_os="$(title_case "$os")"
    archive_arch="$arch"
    case "$arch" in
        x86_64) archive_arch="x86_64" ;;
        arm64)  archive_arch="arm64"  ;;
        aarch64) archive_arch="arm64" ;;
        *)      err "unsupported architecture: $arch" ;;
    esac

    url="https://github.com/${REPO}/releases/download/v${version}/${BINARY}_${archive_os}_${archive_arch}.tar.gz"

    info "installing verve v${version} (${os}/${archive_arch})"
    info "downloading ${url}"

    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"' EXIT

    curl -fsSL "$url" -o "${tmp}/verve.tar.gz"
    tar -xzf "${tmp}/verve.tar.gz" -C "$tmp"

    mkdir -p "$INSTALL_DIR"
    mv "${tmp}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    chmod +x "${INSTALL_DIR}/${BINARY}"

    info "installed to ${INSTALL_DIR}/${BINARY}"

    if ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
        warn "${INSTALL_DIR} is not in your PATH"
        warn "add it with: export PATH=\"${INSTALL_DIR}:\$PATH\""
    fi

    info "run 'verve --help' to get started"
}

resolve_version() {
    requested="$1"
    if [ "$requested" != "latest" ]; then
        # Strip leading v if present
        echo "$requested" | sed 's/^v//'
        return
    fi
    curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' \
        | head -1 \
        | sed 's/.*"v\([^"]*\)".*/\1/'
}

detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        *)       err "unsupported operating system: $(uname -s)" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "x86_64" ;;
        arm64|aarch64) echo "arm64" ;;
        *)             err "unsupported architecture: $(uname -m)" ;;
    esac
}

title_case() {
    echo "$1" | awk '{print toupper(substr($0,1,1)) tolower(substr($0,2))}'
}

need_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        err "required command not found: $1"
    fi
}

info() {
    printf '\033[0;32m=>\033[0m %s\n' "$1"
}

warn() {
    printf '\033[0;33m=>\033[0m %s\n' "$1"
}

err() {
    printf '\033[0;31merror:\033[0m %s\n' "$1" >&2
    exit 1
}

main "$@"
