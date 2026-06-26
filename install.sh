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
  error "python3 not found. Install it first:\n  macOS:  brew install python\n  Ubuntu: sudo apt install python3 python3-venv"
fi
PYTHON=$(python3 -c "import sys; print(sys.executable)")
PY_VER=$(python3 -c "import sys; print(f'{sys.version_info.major}.{sys.version_info.minor}')")
info "Using Python $PY_VER at $PYTHON"

# ── Install Python packages ─────────────────────────────────────────────────
info "Installing Python dependencies..."

if [ "$PLATFORM" = "macos" ]; then
  # macOS: install into user site or break-system-packages if needed
  if ! "$PYTHON" -m pip --version &>/dev/null; then
    error "pip not found. Run: brew install python"
  fi
  "$PYTHON" -m pip install pystray pillow --quiet \
    || "$PYTHON" -m pip install pystray pillow --quiet --break-system-packages \
    || error "pip install failed. Try: pip3 install pystray pillow --break-system-packages"
else
  # Ubuntu 22.04+ blocks pip outside a venv (PEP 668) — create one in INSTALL_DIR
  VENV_DIR="$INSTALL_DIR/venv"
  info "Creating virtual environment at $VENV_DIR ..."
  "$PYTHON" -m venv "$VENV_DIR" 2>/dev/null \
    || error "python3-venv not found. Run: sudo apt install python3-venv"
  "$VENV_DIR/bin/pip" install pystray pillow --quiet \
    || error "pip install failed inside venv."
  # Use the venv Python for running the app
  PYTHON="$VENV_DIR/bin/python"
  info "Virtual environment ready."
fi
info "Python packages installed."

# ── Ubuntu: check AppIndicator (optional, soft warning only) ───────────────
if [ "$PLATFORM" = "ubuntu" ]; then
  if ! python3 -c "import gi; gi.require_version('AyatanaAppIndicator3','0.1')" 2>/dev/null && \
     ! python3 -c "import gi; gi.require_version('AppIndicator3','0.1')" 2>/dev/null; then
    warn "AppIndicator not found. Tray icon will use X11 fallback (set PYSTRAY_BACKEND=xorg if icon is missing)."
    warn "To fix permanently (needs sudo): sudo apt install gir1.2-ayatana-appindicator3-0.1"
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
  # Launch in background on Ubuntu (LaunchAgent handles macOS)
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
