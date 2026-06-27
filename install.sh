#!/bin/bash
# Claude Usage Bar — one-line installer
# Usage: curl -fsSL https://raw.githubusercontent.com/YOUR_USER/claude-usage/main/install.sh | bash

set -e

REPO="sijokurian/claude-meter"
BRANCH="main"
RAW="https://raw.githubusercontent.com/$REPO/$BRANCH"
INSTALL_DIR="$HOME/.local/share/claude-usage"

# ── Colours ────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()    { echo -e "${GREEN}[claude-usage]${NC} $*"; }
warn()    { echo -e "${YELLOW}[claude-usage]${NC} $*"; }
error()   { echo -e "${RED}[claude-usage]${NC} $*" >&2; exit 1; }

# ── Detect OS ──────────────────────────────────────────────────────────────
OS=$(uname -s)
case "$OS" in
  Darwin) PLATFORM="macos" ;;
  Linux)  PLATFORM="ubuntu" ;;
  *)      error "Unsupported OS: $OS" ;;
esac
info "Detected platform: $PLATFORM"

# ── Check Python 3 ─────────────────────────────────────────────────────────
if ! command -v python3 &>/dev/null; then
  error "python3 not found. Install it first:\n  macOS:  brew install python\n  Ubuntu: sudo apt install python3 python3-pip"
fi
PYTHON=$(python3 -c "import sys; print(sys.executable)")
PY_VER=$(python3 -c "import sys; print(f'{sys.version_info.major}.{sys.version_info.minor}')")
info "Using Python $PY_VER at $PYTHON"

# ── Check pip3 is available ────────────────────────────────────────────────
if ! command -v pip3 &>/dev/null && ! "$PYTHON" -m pip --version &>/dev/null; then
  if [ "$PLATFORM" = "macos" ]; then
    error "pip3 not found. Run: brew install python"
  else
    error "pip3 not found. Run: sudo apt install python3-pip"
  fi
fi

# ── Install Python packages ─────────────────────────────────────────────────
info "Installing Python dependencies..."
# --user keeps packages in ~/.local (safe, no sudo)
# --break-system-packages bypasses Ubuntu 22.04+ PEP 668 restriction
"$PYTHON" -m pip install pystray pillow --user --break-system-packages --quiet \
  || "$PYTHON" -m pip install pystray pillow --user --quiet \
  || error "pip install failed. Make sure pip3 is installed and try again."
info "Python packages installed."

# ── Ubuntu: ensure AppIndicator tray support ──────────────────────────────
if [ "$PLATFORM" = "ubuntu" ]; then
  GI_LOCAL="$HOME/.local/share/girepository-1.0"
  export GI_TYPELIB_PATH="$GI_LOCAL:${GI_TYPELIB_PATH:-}"
  if ! python3 -c "import gi; gi.require_version('AyatanaAppIndicator3','0.1')" 2>/dev/null && \
     ! python3 -c "import gi; gi.require_version('AppIndicator3','0.1')" 2>/dev/null; then
    info "Installing AppIndicator support (required for GNOME tray icon)..."
    if ! sudo apt install -y gir1.2-ayatanaappindicator3-0.1 2>/dev/null; then
      info "sudo not available — extracting typelib to $GI_LOCAL ..."
      mkdir -p "$GI_LOCAL"
      GIR_DEB=$(mktemp -d)/gir.deb
      apt download gir1.2-ayatanaappindicator3-0.1 -o APT::Get::Download-Only=true 2>/dev/null \
        && mv gir1.2-ayatanaappindicator3-*.deb "$GIR_DEB" 2>/dev/null
      if [ -f "$GIR_DEB" ]; then
        GIR_TMP=$(mktemp -d)
        dpkg-deb -x "$GIR_DEB" "$GIR_TMP"
        find "$GIR_TMP" -name '*.typelib' -exec cp {} "$GI_LOCAL/" \;
        rm -rf "$GIR_TMP" "$GIR_DEB"
        info "Typelib extracted successfully."
      else
        warn "Could not download gir1.2-ayatanaappindicator3-0.1.\n  Run: sudo apt install gir1.2-ayatanaappindicator3-0.1"
      fi
    fi
  fi
fi

# ── Download app files ─────────────────────────────────────────────────────
info "Downloading app to $INSTALL_DIR ..."
mkdir -p "$INSTALL_DIR"

dl() {
  local file=$1
  curl -fsSL "$RAW/$file" -o "$INSTALL_DIR/$file" || error "Failed to download $file. Check your internet connection."
}

dl claude_usage_bar.py
dl claude_icon.png
dl claude_symbol.png

chmod +x "$INSTALL_DIR/claude_usage_bar.py"
info "Files downloaded."

# ── Auto-start setup ───────────────────────────────────────────────────────
if [ "$PLATFORM" = "macos" ]; then
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
        <string>$PYTHON</string>
        <string>$INSTALL_DIR/claude_usage_bar.py</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/claude_usagebar.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/claude_usagebar.err</string>
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
Name=Claude Usage
Exec=$PYTHON $INSTALL_DIR/claude_usage_bar.py
Hidden=false
X-GNOME-Autostart-enabled=true
EOF
  info "Auto-start configured via ~/.config/autostart."
  nohup "$PYTHON" "$INSTALL_DIR/claude_usage_bar.py" >/dev/null 2>&1 &
  info "App launched."
fi

# ── Done ───────────────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}✓ Claude Usage Bar installed successfully!${NC}"
echo ""
echo "  Location : $INSTALL_DIR"
if [ "$PLATFORM" = "macos" ]; then
  echo "  Auto-start: Login Items via LaunchAgent (already running)"
  echo "  Logs      : /tmp/claude_usagebar.log"
else
  echo "  Auto-start: ~/.config/autostart/claude-usage.desktop"
  echo "  Run again : $PYTHON $INSTALL_DIR/claude_usage_bar.py"
fi
echo ""
echo "  To uninstall:"
if [ "$PLATFORM" = "macos" ]; then
  echo "    launchctl unload ~/Library/LaunchAgents/com.claude.usagebar.plist"
  echo "    rm -rf $INSTALL_DIR ~/Library/LaunchAgents/com.claude.usagebar.plist"
else
  echo "    rm -rf $INSTALL_DIR ~/.config/autostart/claude-usage.desktop"
fi
