#!/bin/bash
# Claude Meter — one-line installer
# Usage: curl -fsSL https://raw.githubusercontent.com/sijokurian/claude-meter/main/install.sh | bash

set -e

REPO="sijokurian/claude-meter"
INSTALL_DIR="$HOME/.local/bin"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()  { echo -e "${GREEN}[claude-meter]${NC} $*"; }
warn()  { echo -e "${YELLOW}[claude-meter]${NC} $*"; }
error() { echo -e "${RED}[claude-meter]${NC} $*" >&2; exit 1; }

# ── Detect OS + arch ─────────────────────────────────────────────────
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$OS" in
  darwin) PLATFORM="darwin" ;;
  linux)  PLATFORM="linux" ;;
  *)      error "Unsupported OS: $OS" ;;
esac
case "$ARCH" in
  x86_64|amd64) GOARCH="amd64" ;;
  aarch64|arm64) GOARCH="arm64" ;;
  *)             error "Unsupported architecture: $ARCH" ;;
esac
info "Detected: ${PLATFORM}/${GOARCH}"

# ── Download binary ──────────────────────────────────────────────────
BINARY="claude-meter-${PLATFORM}-${GOARCH}"
LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')

if [ -z "$LATEST" ]; then
  warn "Could not fetch latest release. Building from source..."
  if ! command -v go &>/dev/null; then
    error "Go not found. Install Go from https://go.dev/dl/ or download a release binary from https://github.com/${REPO}/releases"
  fi
  SRCDIR=$(mktemp -d)
  info "Cloning and building..."
  git clone --depth 1 "https://github.com/${REPO}.git" "$SRCDIR/claude-meter"
  cd "$SRCDIR/claude-meter"
  go build -o claude-meter .
  mkdir -p "$INSTALL_DIR"
  mv claude-meter "$INSTALL_DIR/claude-meter"
  rm -rf "$SRCDIR"
else
  DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST}/${BINARY}"
  info "Downloading ${LATEST} for ${PLATFORM}/${GOARCH}..."
  mkdir -p "$INSTALL_DIR"
  curl -fsSL "$DOWNLOAD_URL" -o "$INSTALL_DIR/claude-meter" \
    || error "Download failed. Check https://github.com/${REPO}/releases"
fi

chmod +x "$INSTALL_DIR/claude-meter"
info "Installed to $INSTALL_DIR/claude-meter"

# ── Auto-start setup ─────────────────────────────────────────────────
if [ "$PLATFORM" = "darwin" ]; then
  PLIST="$HOME/Library/LaunchAgents/com.claude.usagebar.plist"
  mkdir -p "$HOME/Library/LaunchAgents"
  cat > "$PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.claude.usagebar</string>
    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/claude-meter</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/claude-meter.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/claude-meter.err</string>
</dict>
</plist>
EOF
  launchctl unload "$PLIST" 2>/dev/null || true
  launchctl load "$PLIST"
  info "Auto-start configured via LaunchAgent."
else
  mkdir -p "$HOME/.config/autostart"
  cat > "$HOME/.config/autostart/claude-usage.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=Claude Meter
Exec=${INSTALL_DIR}/claude-meter
Hidden=false
X-GNOME-Autostart-enabled=true
EOF
  info "Auto-start configured via ~/.config/autostart."
  nohup "$INSTALL_DIR/claude-meter" >/dev/null 2>&1 &
  info "App launched."
fi

echo ""
echo -e "${GREEN}✓ Claude Meter installed successfully!${NC}"
echo ""
echo "  Binary   : $INSTALL_DIR/claude-meter"
echo "  Single Go binary — no extra dependencies."
echo ""
echo "  To uninstall:"
if [ "$PLATFORM" = "darwin" ]; then
  echo "    launchctl unload ~/Library/LaunchAgents/com.claude.usagebar.plist"
  echo "    rm -f $INSTALL_DIR/claude-meter ~/Library/LaunchAgents/com.claude.usagebar.plist"
else
  echo "    rm -f $INSTALL_DIR/claude-meter ~/.config/autostart/claude-usage.desktop"
fi
