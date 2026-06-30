# claude-meter

Real-time Claude usage in your system tray. Single Go binary — requires the browser extension.

![claude-meter icon](claude_icon.png)

![claude-meter screenshot](screenshot.png)

Works on **macOS** and **Ubuntu/Linux**.

---

## Install

### One-liner

```bash
curl -fsSL https://raw.githubusercontent.com/sijokurian/claude-meter/main/install.sh | bash
```

The script downloads the binary to `~/.local/bin/claude-meter`, sets up auto-start on login, and launches the app.

### Manual

Download the binary for your platform from [Releases](https://github.com/sijokurian/claude-meter/releases), make it executable, and run:

```bash
chmod +x claude-meter
./claude-meter
```

### Build from source

**Prerequisites:**

- Go 1.22+
- GTK3 dev headers (Linux only):

```bash
# Ubuntu / Debian
sudo apt install libgtk-3-dev

# Fedora
sudo dnf install gtk3-devel

# Arch
sudo pacman -S gtk3
```

**Build and run:**

```bash
git clone https://github.com/sijokurian/claude-meter.git
cd claude-meter
go build -o claude-meter .
./claude-meter
```

---

## Setup

The app requires the browser extension to work. Without it, the tray shows a gray icon and instructions.

1. Install the browser extension (see below)
2. Login to [claude.ai](https://claude.ai) in Chrome
3. The tray icon turns orange and shows your current session usage

---

## What it shows

The tray displays the Claude logo with your current session usage:

```
[Claude icon]  51%
```

Click the icon to expand:

```
Usage: 51%
  Session: 51% — Resets in 2 hr 23 min
  Weekly: 80% — Resets Wed 1:30 PM
────────────────
Last sync: just now
Disable Notifications
────────────────
Refresh Now
Quit
```

Session and weekly percentages with reset times come directly from claude.ai via the browser extension.

### Alerts

Desktop notification at every 10% milestone (10%, 20%, ... 90%). Toggle from the menu.

---

## Browser Extension

Install the **Claude Meter** browser extension to sync usage from claude.ai.

### Install the extension

1. Open `chrome://extensions` in Chrome (or any Chromium-based browser)
2. Enable **Developer mode** (toggle in the top-right)
3. Click **Load unpacked**
4. Select the `extension/` folder inside this repo
5. Login to [claude.ai](https://claude.ai) — the extension starts syncing automatically

The tray app listens on `localhost:52413` for data from the extension.

---

## Uninstall

**macOS**
```bash
launchctl unload ~/Library/LaunchAgents/com.claude.usagebar.plist
rm -f ~/.local/bin/claude-meter ~/Library/LaunchAgents/com.claude.usagebar.plist
```

**Linux**
```bash
rm -f ~/.local/bin/claude-meter ~/.config/autostart/claude-usage.desktop
```

---

## Privacy

All data is processed locally on your machine. Nothing is sent to any external server, third party, or cloud service.

### What the browser extension reads

The extension **does not make any API requests to Claude**. It passively reads data the browser is already loading:

| Method | What it reads | How |
|--------|--------------|-----|
| Fetch interception | API responses the claude.ai page already requests (e.g. usage percentage, rate limits) | Monkey-patches `window.fetch` to clone and read responses — no extra network requests |

The only outbound request the extension makes is to **`localhost:52413`** — your own desktop app on your machine.

### Settings

Stored locally at `~/.config/claude-meter/settings.json` (Linux) or `~/Library/Application Support/claude-meter/settings.json` (macOS). Only stores notification preference.

### Summary

- No data is sent to any external server, third party, or cloud service.
- No analytics, telemetry, tracking, or remote data collection of any kind.
