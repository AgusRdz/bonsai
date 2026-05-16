#!/bin/sh
set -e

REPO="AgusRdz/bonsai"

# Detect OS
OS="$(uname -s)"
case "$OS" in
  Linux*)  OS="linux" ;;
  Darwin*) OS="darwin" ;;
  MINGW*|MSYS*|CYGWIN*) OS="windows" ;;
  *) echo "unsupported OS: $OS" >&2; exit 1 ;;
esac

# Set default install dir
if [ -z "$BONSAI_INSTALL_DIR" ]; then
  if [ "$OS" = "windows" ]; then
    INSTALL_DIR="$(cygpath "$LOCALAPPDATA/Programs/bonsai" 2>/dev/null || echo "$HOME/AppData/Local/Programs/bonsai")"
  else
    INSTALL_DIR="$HOME/.local/bin"
  fi
else
  INSTALL_DIR="$BONSAI_INSTALL_DIR"
fi

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

EXT=""
if [ "$OS" = "windows" ]; then
  EXT=".exe"
fi

BINARY="bonsai-${OS}-${ARCH}${EXT}"

# Get latest version
if [ -z "$BONSAI_VERSION" ]; then
  BONSAI_VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed 's/.*"tag_name": *"//;s/".*//')
fi

if [ -z "$BONSAI_VERSION" ]; then
  echo "failed to determine latest version" >&2
  exit 1
fi

URL="https://github.com/${REPO}/releases/download/${BONSAI_VERSION}/${BINARY}"

echo "installing bonsai ${BONSAI_VERSION} (${OS}/${ARCH})..."

mkdir -p "$INSTALL_DIR"
curl -fsSL "$URL" -o "${INSTALL_DIR}/bonsai${EXT}"
chmod +x "${INSTALL_DIR}/bonsai${EXT}"

echo "installed bonsai to ${INSTALL_DIR}/bonsai${EXT}"
echo ""

# PATH management
case ":$PATH:" in
  *":${INSTALL_DIR}:"*)
    # Install dir is already in PATH — bonsai is immediately usable
    echo "bonsai is ready. Run 'bonsai help' to get started."
    ;;
  *)
    if [ "$OS" = "windows" ]; then
      WIN_DIR=$(cygpath -w "$INSTALL_DIR" 2>/dev/null || echo "$INSTALL_DIR")
      powershell.exe -NoProfile -Command "\$p = [Environment]::GetEnvironmentVariable('Path', 'User'); \$d = '${WIN_DIR}'.TrimEnd('\\'); if ((\$p -split ';' | ForEach-Object { \$_.TrimEnd('\\') }) -notcontains \$d) { [Environment]::SetEnvironmentVariable('Path', \"\$d;\$p\", 'User') }"
      echo "To use bonsai in this terminal:"
      echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
      echo ""
      echo "Future terminals will find it automatically."
    else
      # Write to shell rc for future sessions
      SHELL_NAME="$(basename "${SHELL:-}")"
      case "$SHELL_NAME" in
        zsh)  SHELL_RC="$HOME/.zshrc" ;;
        bash) SHELL_RC="$HOME/.bashrc" ;;
        *)    SHELL_RC="" ;;
      esac

      if [ -n "$SHELL_RC" ]; then
        if ! grep -qF "$INSTALL_DIR" "$SHELL_RC" 2>/dev/null; then
          printf '\n# bonsai\nexport PATH="%s:$PATH"\n' "$INSTALL_DIR" >> "$SHELL_RC"
        fi
        echo "To use bonsai in this terminal:"
        echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
        echo ""
        echo "Future terminals will find it automatically (added to $SHELL_RC)."
      else
        echo "To use bonsai in this terminal:"
        echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
        echo ""
        echo "Add that line to your shell config to make it permanent."
      fi
    fi
    ;;
esac
