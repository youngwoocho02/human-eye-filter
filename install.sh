#!/bin/sh
set -e

REPO="youngwoocho02/human-eye-filter"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  linux)  ;;
  darwin) ;;
  *)      echo "Unsupported OS: $OS (use Windows instructions in README)"; exit 1 ;;
esac

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)   ARCH="amd64" ;;
  aarch64|arm64)  ARCH="arm64" ;;
  *)              echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

INSTALL_DIR="$HOME/.local/bin"
mkdir -p "$INSTALL_DIR"

URL="https://github.com/${REPO}/releases/latest/download/humaneye-${OS}-${ARCH}"

echo "Downloading humaneye for ${OS}/${ARCH}..."
curl -fsSL "$URL" -o "$INSTALL_DIR/humaneye"
chmod +x "$INSTALL_DIR/humaneye"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    export PATH="$INSTALL_DIR:$PATH"
    LINE="export PATH=\"$INSTALL_DIR:\$PATH\""
    SHELL_NAME="$(basename "$SHELL")"
    case "$SHELL_NAME" in
      zsh)  RC_FILE="$HOME/.zshrc" ;;
      bash) RC_FILE="$HOME/.bashrc" ;;
      *)    RC_FILE="$HOME/.profile" ;;
    esac
    touch "$RC_FILE"
    echo "$LINE" >> "$RC_FILE"
    echo "Added $INSTALL_DIR to PATH (restart shell to apply)" ;;
esac

echo "Installed humaneye to $INSTALL_DIR/humaneye"
"$INSTALL_DIR/humaneye" version
