#!/bin/sh
set -e

REPO="AgusRdz/nvy"

# Detect OS
OS="$(uname -s)"
case "$OS" in
  Linux*)  OS="linux" ;;
  Darwin*) OS="darwin" ;;
  MINGW*|MSYS*|CYGWIN*) OS="windows" ;;
  *) echo "unsupported OS: $OS" >&2; exit 1 ;;
esac

# Set default install dir (Windows uses AppData, Unix uses ~/.local/bin)
if [ -z "$NVY_INSTALL_DIR" ]; then
  if [ "$OS" = "windows" ]; then
    INSTALL_DIR="$(cygpath "$LOCALAPPDATA/Programs/nvy" 2>/dev/null || echo "$HOME/AppData/Local/Programs/nvy")"
  else
    INSTALL_DIR="$HOME/.local/bin"
  fi
else
  INSTALL_DIR="$NVY_INSTALL_DIR"
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

BINARY="nvy-${OS}-${ARCH}${EXT}"

# Get latest version
if [ -z "$NVY_VERSION" ]; then
  # Use the most recent release from the list (newest first) rather than
  # /releases/latest, which lags briefly while GitHub flips the "latest" flag.
  NVY_VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')
fi

if [ -z "$NVY_VERSION" ]; then
  echo "failed to determine latest version" >&2
  exit 1
fi

URL="https://github.com/${REPO}/releases/download/${NVY_VERSION}/${BINARY}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${NVY_VERSION}/checksums.txt"

echo "installing nvy ${NVY_VERSION} (${OS}/${ARCH})..."

mkdir -p "$INSTALL_DIR"
TMPFILE="${INSTALL_DIR}/nvy${EXT}.tmp"

curl -fsSL "$URL" -o "$TMPFILE"

# Verify SHA256 checksum before installing
CHECKSUMS=$(curl -fsSL "$CHECKSUMS_URL") || { echo "failed to download checksums.txt" >&2; rm -f "$TMPFILE"; exit 1; }
EXPECTED=$(printf '%s' "$CHECKSUMS" | grep " ${BINARY}$" | awk '{print $1}')
if [ -z "$EXPECTED" ]; then
  echo "checksum not found for ${BINARY}" >&2
  rm -f "$TMPFILE"
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL=$(sha256sum "$TMPFILE" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL=$(shasum -a 256 "$TMPFILE" | awk '{print $1}')
else
  echo "warning: sha256sum/shasum not found, skipping checksum verification" >&2
  ACTUAL="$EXPECTED"
fi

if [ "$ACTUAL" != "$EXPECTED" ]; then
  echo "checksum mismatch for ${BINARY}: expected ${EXPECTED}, got ${ACTUAL}" >&2
  rm -f "$TMPFILE"
  exit 1
fi

mv "$TMPFILE" "${INSTALL_DIR}/nvy${EXT}"
chmod +x "${INSTALL_DIR}/nvy${EXT}"

echo "installed nvy to ${INSTALL_DIR}/nvy${EXT}"
echo ""

# Check if install dir is in PATH
case ":$PATH:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    if [ "$OS" = "windows" ]; then
      # Convert to Windows path for registry update
      WIN_DIR=$(cygpath -w "$INSTALL_DIR" 2>/dev/null || echo "$INSTALL_DIR")
      powershell.exe -NoProfile -Command "\$p = [Environment]::GetEnvironmentVariable('Path', 'User'); \$d = '${WIN_DIR}'.TrimEnd('\\'); if ((\$p -split ';' | ForEach-Object { \$_.TrimEnd('\\') }) -notcontains \$d) { [Environment]::SetEnvironmentVariable('Path', \"\$d;\$p\", 'User'); Write-Host \"Added \$d to User PATH\" }"
      export PATH="${INSTALL_DIR}:$PATH"
    else
    # Detect shell config file
    SHELL_NAME="$(basename "${SHELL:-}")"
    case "$SHELL_NAME" in
      zsh)  SHELL_RC="$HOME/.zshrc" ;;
      bash) SHELL_RC="$HOME/.bashrc" ;;
      *)    SHELL_RC="" ;;
    esac

    PATH_LINE="export PATH=\"${INSTALL_DIR}:\$PATH\""

    if [ -n "$SHELL_RC" ]; then
      # Only add if not already present
      if ! grep -qF "$INSTALL_DIR" "$SHELL_RC" 2>/dev/null; then
        printf '\n# nvy\n%s\n' "$PATH_LINE" >> "$SHELL_RC"
        echo "Added ${INSTALL_DIR} to PATH in $SHELL_RC"
        echo "Reload your shell with: source $SHELL_RC"
      fi
    else
      echo "NOTE: ${INSTALL_DIR} is not in your PATH."
      echo "Add this line to your shell config file:"
      echo "  $PATH_LINE"
    fi
    echo ""
    fi
    ;;
esac

# Run init to install the shell prompt hook and schedule the expiration check
"${INSTALL_DIR}/nvy${EXT}" init || true

echo ""
echo "Installation complete."
echo ""
echo "  nvy has installed its shell hook and scheduled a daily expiration check."
echo "  This script runs in a child process, so it cannot activate the hook in your"
echo "  CURRENT shell. To start using it now, run:"
echo ""
echo "    source ~/.zshrc   # or ~/.bashrc — or just open a new shell"
echo ""
echo "  After that, global vars and project .env files sync automatically."
