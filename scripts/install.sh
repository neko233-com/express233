#!/bin/bash
set -e

# express233 CLI installer
# Usage: curl -fsSL https://raw.githubusercontent.com/neko233-com/express233/main/scripts/install.sh | bash
# Or: curl -fsSL .../install.sh | bash -s -- v0.1.0

VERSION="${1:-latest}"
BINARY_NAME="express233"
REPO="neko233-com/express233"

detect_os() {
    case "$(uname -s)" in
        Linux*)     echo "linux" ;;
        Darwin*)    echo "darwin" ;;
        CYGWIN*|MINGW*|MSYS*) echo "windows" ;;
        *)          echo "unsupported" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)   echo "amd64" ;;
        aarch64|arm64)  echo "arm64" ;;
        *)              echo "amd64" ;;
    esac
}

get_latest_version() {
    curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | \
        grep '"tag_name":' | head -1 | sed -E 's/.*"v([^"]+)".*/\1/' || echo "0.1.0"
}

normalize_version() {
    local v="$1"
    v="${v#v}"
    v="${v#V}"
    echo "$v"
}

install_binary() {
    local os="$1"
    local arch="$2"
    local ver="$3"

    local asset="${BINARY_NAME}-${os}-${arch}"
    local ext=""
    [ "$os" = "windows" ] && ext=".exe"

    local url="https://github.com/${REPO}/releases/download/v${ver}/${asset}${ext}"
    local install_dir="/usr/local/bin"
    local target="${BINARY_NAME}"

    if [ "$os" = "windows" ]; then
        install_dir="${LOCALAPPDATA:-$HOME/AppData/Local}/express233"
        target="${BINARY_NAME}.exe"
        mkdir -p "$install_dir"
    fi

    echo "Downloading ${url}..."
    TMPDIR=$(mktemp -d)
    curl -fsSL "$url" -o "${TMPDIR}/${target}${ext}"

    if [ -w "$install_dir" ]; then
        mv -f "${TMPDIR}/${target}${ext}" "${install_dir}/${target}${ext}"
    else
        sudo mv -f "${TMPDIR}/${target}${ext}" "${install_dir}/${target}${ext}"
    fi

    chmod +x "${install_dir}/${target}${ext}" 2>/dev/null || true
    rm -rf "$TMPDIR"

    echo "Installed to ${install_dir}/${target}${ext}"
    echo "Run: express233 --help"
}

main() {
    OS=$(detect_os)
    ARCH=$(detect_arch)

    if [ "$OS" = "unsupported" ]; then
        echo "Unsupported operating system."
        exit 1
    fi

    if [ "$VERSION" = "latest" ] || [ -z "$VERSION" ]; then
        VERSION=$(get_latest_version)
    fi
    VERSION=$(normalize_version "$VERSION")

    echo "Detected: ${OS}/${ARCH}"
    echo "Installing express233 v${VERSION}..."
    install_binary "$OS" "$ARCH" "$VERSION"
    echo "Done."
}

main "$@"
