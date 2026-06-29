# claude-meter

Real-time Claude Code token usage in your menu bar. Single binary — no Python, no pip, no sudo.

![macOS menu bar showing Claude usage at 39%](https://raw.githubusercontent.com/sijokurian/claude-meter/main/claude_icon.png)

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

```bash
git clone https://github.com/sijokurian/claude-meter.git
cd claude-meter
go build -o claude-meter .
./claude-meter
```

**Build requirements:** Go 1.22+, GTK3 dev headers on Linux (`sudo apt install libgtk-3-dev libayatana-appindicator3-dev`).

---

## What it shows

The menu bar displays the Claude logo with your current usage percentage:

```
[Claude icon]  7%
```

Click the icon to expand:

```
Usage: 3.07M / 43.87M  (7.0%)
Messages (5h): 744

  Input:          2.9K
  Output:         249.0K
  Cache created:  2.35M
  Cache read:     69.74M

Web (claude.ai): 7.0%
  Last sync: just now

Window: last 5 hours
Limit: 43.87M tokens
Set Limit...
Calibrate from Website...
Reset Alerts
────────────────
Refresh Now
Quit
```

### Alerts

Desktop notification at every 10% milestone (10%, 20%, … 90%).

---

## How it works

Claude Code writes every API response to `~/.claude/projects/**/*.jsonl`. claude-meter reads those files, sums `input_tokens + output_tokens + cache_creation_input_tokens` (cache reads weighted at 1/150), and divides by your token limit.

---

## Browser Extension (auto-sync with claude.ai)

Install the **Claude Meter** browser extension to automatically sync usage from claude.ai. It reads the usage percentage and sends it to the tray app, which auto-calibrates the token limit.

### Install the extension

1. Open `chrome://extensions` in Chrome (or any Chromium-based browser)
2. Enable **Developer mode** (toggle in the top-right)
3. Click **Load unpacked**
4. Select the `extension/` folder inside this repo
5. Open [claude.ai](https://claude.ai) — the extension starts syncing automatically

The tray app listens on `localhost:52413` for data from the extension.

---

## Manual Calibration

If you don't want the extension:

1. Open [claude.ai](https://claude.ai) and note your usage %
2. Click the tray icon → **Calibrate from Website…**
3. Enter the percentage — the app recalculates your exact limit

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
