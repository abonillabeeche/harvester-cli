#!/usr/bin/env sh
# Install the harvester CLI from GitHub Releases.
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/abonillabeeche/harvester-cli/main/install.sh | sh
#   INSTALL_DIR=~/.local/bin sh install.sh

set -e

REPO="abonillabeeche/harvester-cli"
BIN="harvester"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# ── Detect OS ──────────────────────────────────────────────────────────────────
OS=$(uname -s)
case "$OS" in
  Linux)  OS="linux"  ;;
  Darwin) OS="darwin" ;;
  *)
    echo "Unsupported OS: $OS"
    exit 1
    ;;
esac

# ── Detect architecture ────────────────────────────────────────────────────────
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)        ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

# ── Resolve latest release tag ─────────────────────────────────────────────────
VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' \
  | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$VERSION" ]; then
  echo "Could not determine latest version from GitHub API."
  exit 1
fi

VER="${VERSION#v}"   # strip leading v for filename

# ── Download and extract ───────────────────────────────────────────────────────
ARCHIVE="harvester_${VER}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

echo "Installing harvester ${VERSION} (${OS}/${ARCH}) → ${INSTALL_DIR}/${BIN}"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

curl -fsSL "${URL}" -o "${TMP}/${ARCHIVE}"
tar -xzf "${TMP}/${ARCHIVE}" -C "${TMP}"

install -m 0755 "${TMP}/${BIN}" "${INSTALL_DIR}/${BIN}"

# ── macOS: remove quarantine flag so Gatekeeper doesn't block the binary ───────
if [ "$OS" = "darwin" ]; then
  xattr -d com.apple.quarantine "${INSTALL_DIR}/${BIN}" 2>/dev/null || true
fi

echo ""
echo "Installed successfully!"
"${INSTALL_DIR}/${BIN}" --version
