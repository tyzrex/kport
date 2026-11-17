#!/bin/sh
set -e

# killport installer script
# Usage: curl -fsSL https://raw.githubusercontent.com/tyzrex/kport/main/install.sh | sh

REPO="tyzrex/kport"
BIN_NAME="killport"
ALIAS_NAME="kport"

# Determine target install directory
if [ -w "/usr/local/bin" ]; then
    INSTALL_DIR="/usr/local/bin"
elif [ -d "$HOME/.local/bin" ]; then
    INSTALL_DIR="$HOME/.local/bin"
elif [ -d "$HOME/bin" ]; then
    INSTALL_DIR="$HOME/bin"
else
    mkdir -p "$HOME/.local/bin"
    INSTALL_DIR="$HOME/.local/bin"
fi

echo "==> Installing killport to $INSTALL_DIR..."

if command -v go >/dev/null 2>&1; then
    echo "==> Building from source with Go..."
    go install github.com/tyzrex/kport/cmd/killport@latest
    GO_BIN="$(go env GOBIN)"
    if [ -z "$GO_BIN" ]; then GO_BIN="$(go env GOPATH)/bin"; fi
    if [ -x "$GO_BIN/killport" ]; then
        cp "$GO_BIN/killport" "$INSTALL_DIR/killport"
        ln -sf "$INSTALL_DIR/killport" "$INSTALL_DIR/$ALIAS_NAME"
        echo "Installed kport and killport in $INSTALL_DIR"
        exit 0
    fi
fi

# Detect OS and Arch for prebuilt binary download
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

LATEST_TAG=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
if [ -z "$LATEST_TAG" ]; then
    echo "Could not determine the latest release. Install Go or download from https://github.com/$REPO/releases" >&2
    exit 1
fi

URL="https://github.com/$REPO/releases/download/$LATEST_TAG/killport_${OS}_${ARCH}.tar.gz"

echo "==> Downloading $URL..."
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

if curl -sSL -o "$TMP_DIR/killport.tar.gz" "$URL"; then
    tar -xzf "$TMP_DIR/killport.tar.gz" -C "$TMP_DIR"
    chmod +x "$TMP_DIR/killport"
    mv "$TMP_DIR/killport" "$INSTALL_DIR/killport"
    ln -sf "$INSTALL_DIR/killport" "$INSTALL_DIR/$ALIAS_NAME"
    echo "✓ killport $LATEST_TAG successfully installed to $INSTALL_DIR/killport"
    echo "✓ Alias '$ALIAS_NAME' created!"
else
    echo "Failed to download prebuilt binary. Please install using 'go install github.com/tyzrex/kport/cmd/killport@latest'" >&2
    exit 1
fi
