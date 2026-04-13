#!/bin/sh
# install.sh — install the latest cncf-maintainers binary
# Usage: curl -sSL https://raw.githubusercontent.com/idvoretskyi/cncf-github-maintainers/main/install.sh | sh

set -e

REPO="idvoretskyi/cncf-github-maintainers"
BINARY="cncf-maintainers"
INSTALL_DIR="/usr/local/bin"

# Detect OS
case "$(uname -s)" in
Darwin) OS="darwin" ;;
Linux) OS="linux" ;;
*)
	echo "Unsupported OS: $(uname -s)" >&2
	exit 1
	;;
esac

# Detect architecture
case "$(uname -m)" in
x86_64) ARCH="amd64" ;;
aarch64 | arm64) ARCH="arm64" ;;
*)
	echo "Unsupported architecture: $(uname -m)" >&2
	exit 1
	;;
esac

TARBALL="${BINARY}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/latest/download/${TARBALL}"

echo "Downloading ${BINARY} (${OS}/${ARCH})..."
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

curl -sSL "$URL" | tar -xz -C "$TMP"

echo "Installing to ${INSTALL_DIR}/${BINARY}..."
if [ -w "$INSTALL_DIR" ]; then
	mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
else
	sudo mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
fi

echo "Installed successfully. Run: ${BINARY} --version"
