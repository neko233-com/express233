#!/bin/bash
set -e

# express233-server 安装脚本
# curl -fsSL .../install-server.sh | bash -s -- v0.1.0

VERSION="${1:-latest}"
BINARY_NAME="express233-server"
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
    local v="${1#v}"
    v="${v#V}"
    echo "$v"
}

OS=$(detect_os)
ARCH=$(detect_arch)
[ "$OS" = "unsupported" ] && echo "Unsupported OS" && exit 1

[ "$VERSION" = "latest" ] && VERSION=$(get_latest_version)
VERSION=$(normalize_version "$VERSION")

ext=""
[ "$OS" = "windows" ] && ext=".exe"

asset="${BINARY_NAME}-${OS}-${ARCH}${ext}"
url="https://github.com/${REPO}/releases/download/v${VERSION}/${asset}"

install_dir="${EXPRESS233_SERVER_INSTALL:-/usr/local/bin}"
[ "$OS" = "windows" ] && install_dir="${LOCALAPPDATA}/express233"

target="${BINARY_NAME}${ext}"

mkdir -p "$install_dir"
echo "Downloading $url ..."
TMP=$(mktemp -d)
curl -fsSL "$url" -o "${TMP}/${target}"

if [ -w "$install_dir" ]; then
    mv -f "${TMP}/${target}" "${install_dir}/${target}"
else
    sudo mv -f "${TMP}/${target}" "${install_dir}/${target}"
fi
chmod +x "${install_dir}/${target}" 2>/dev/null || true
rm -rf "$TMP"

echo "Installed ${BINARY_NAME} v${VERSION} -> ${install_dir}/${target}"
echo "Run: EXPRESS233_DATA=~/.express233-server ${BINARY_NAME}${ext} start"
echo "Status: ${BINARY_NAME}${ext} status"
echo "Enable boot autostart: ${BINARY_NAME}${ext} enable-autostart"
echo "Change port: ${BINARY_NAME}${ext} set-port 32380"
echo "Self update: ${BINARY_NAME}${ext} update"
echo "Hot reload server.yaml: ${BINARY_NAME}${ext} reload-config"
echo "Force reset root password: ${BINARY_NAME}${ext} reset-root-password --password <NEW_PASSWORD>"
