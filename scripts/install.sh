#!/bin/bash
set -euo pipefail

# express233-cli installer
# Usage: curl -fsSL https://raw.githubusercontent.com/neko233-com/express233/main/scripts/install.sh | bash
# Or: curl -fsSL .../install.sh | bash -s -- v0.1.0

VERSION="${1:-latest}"
BINARY_NAME="express233-cli"
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
        *)              echo "unsupported" ;;
    esac
}

get_latest_version() {
    curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | \
        sed -nE 's/.*"tag_name": *"v([^"]+)".*/\1/p'
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
    local install_dir="${EXPRESS233_INSTALL_DIR:-/usr/local/bin}"
    local target="${BINARY_NAME}"

    if [ "$os" = "windows" ]; then
        install_dir="${LOCALAPPDATA:-$HOME/AppData/Local}/express233"
        target="${BINARY_NAME}${ext}"
        mkdir -p "$install_dir"
    fi

    local tmp_dir base downloaded expected actual entry name candidate
    tmp_dir=$(mktemp -d)
    trap 'rm -rf -- "$tmp_dir"; trap - RETURN' RETURN
    base="https://github.com/${REPO}/releases/download/v${ver}"
    curl -fsSL "$base/SHA256SUMS" -o "$tmp_dir/SHA256SUMS"
    local entries=("${asset}${ext}")
    if [ "$os" != "windows" ]; then entries+=(safe-deploy.sh batch-deploy.sh); fi
    for entry in "${entries[@]}"; do
        downloaded="$tmp_dir/$entry"
        curl -fsSL "$base/$entry" -o "$downloaded"
        expected=$(awk -v name="dist/$entry" '$2==name {print $1}' "$tmp_dir/SHA256SUMS")
        [[ "$expected" =~ ^[a-f0-9]{64}$ ]] || { echo 'missing official artifact checksum' >&2; return 1; }
        if command -v sha256sum >/dev/null 2>&1; then actual=$(sha256sum "$downloaded" | awk '{print $1}'); else actual=$(shasum -a 256 "$downloaded" | awk '{print $1}'); fi
        [[ "$actual" == "$expected" ]] || { echo 'official artifact checksum mismatch' >&2; return 1; }
        if [[ "$entry" == *.sh ]]; then bash -n "$downloaded"; fi
    done
    for entry in "${entries[@]}"; do
        name="$entry"
        [[ "$entry" != "${asset}${ext}" ]] || name="$target"
        candidate="$install_dir/.$name.candidate.$$"
        if [ -w "$install_dir" ]; then
            cp "$tmp_dir/$entry" "$candidate"; chmod 755 "$candidate"; mv -f "$candidate" "$install_dir/$name"
        else
            sudo cp "$tmp_dir/$entry" "$candidate"; sudo chmod 755 "$candidate"; sudo mv -f "$candidate" "$install_dir/$name"
        fi
        echo "Installed verified artifact: $name"
    done
    echo "Run: express233-cli --help"
}

main() {
    OS=$(detect_os)
    ARCH=$(detect_arch)

    if [ "$OS" = "unsupported" ] || [ "$ARCH" = "unsupported" ]; then
        echo "Unsupported operating system."
        exit 1
    fi

    if [ "$VERSION" = "latest" ] || [ -z "$VERSION" ]; then
        VERSION=$(get_latest_version)
    fi
    VERSION=$(normalize_version "$VERSION")
    [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][A-Za-z0-9._-]+)?$ ]] || { echo 'invalid release version' >&2; exit 1; }

    echo "Detected: ${OS}/${ARCH}"
    echo "Installing express233-cli v${VERSION}..."
    install_binary "$OS" "$ARCH" "$VERSION"
    echo "Done."
}

main "$@"
