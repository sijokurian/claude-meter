# claude-meter

Real-time Claude Code token usage in your menu bar.

![macOS menu bar showing Claude usage at 39%](https://raw.githubusercontent.com/sijokurian/claude-meter/main/claude_icon.png)

Works on **macOS** and **Ubuntu**.

---

## Requirements

| Requirement | macOS | Ubuntu |
|---|---|---|
| Python 3 | `brew install python` | `sudo apt install python3` |
| pip3 | Included with Python | `sudo apt install python3-pip` |
| [Claude Code](https://claude.ai/code) | Must be installed and used at least once | Same |
| OS | macOS 12+ | Ubuntu 20.04+ with GNOME |

---

## Install

### 1. Install Python 3 and pip3 (if not already installed)

**macOS**
```bash
brew install python
```

**Ubuntu**
```bash
sudo apt install python3 python3-pip
```

### 2. Install Claude Code (if not already installed)

Follow the instructions at [claude.ai/code](https://claude.ai/code) and run at least one session so that `~/.claude/projects/` exists.

### 3. Run the installer

```bash
curl -fsSL https://raw.githubusercontent.com/sijokurian/claude-meter/main/install.sh | bash
```

The script will:
- Install `pystray` and `pillow` Python packages (no sudo needed)
- Download the app to `~/.local/share/claude-usage/`
- Set up auto-start on login
- Launch the app immediately

---

## What it shows

The menu bar displays the Claude logo with your current usage percentage next to it:

```
[Claude icon]  39%
```

Click the icon to expand:

```
Usage: 391K / 1.00M  (39.1%)
Messages (5h): 42

  Input:          269
  Output:         82.9K
  Cache created:  270K
  Cache read:     10.82M

Window: last 5 hours
Limit: 1.00M tokens
Set Limit...
Calibrate from Website...
Reset Alerts
────────────────
Refresh Now
Quit
```

### Alerts

You get a macOS/desktop notification at every 10% milestone (10%, 20%, … 90%).

---

## How it works

Claude Code writes every API response to `~/.claude/projects/**/*.jsonl`. claude-meter reads those files, sums `input_tokens + output_tokens + cache_creation_input_tokens` (cache reads are excluded — Anthropic does not rate-limit on them), and divides by your token limit.

The default limit is **1,000,000 tokens per 5-hour rolling window**, which matches a standard Claude subscription.

---

## Calibration

If the percentage doesn't match claude.ai:

1. Open [claude.ai](https://claude.ai) and note your usage %
2. Click the menu bar icon → **Calibrate from Website…**
3. Enter the percentage — the app recalculates your exact limit

---

## Uninstall

**macOS**
```bash
launchctl unload ~/Library/LaunchAgents/com.claude.usagebar.plist
rm -rf ~/.local/share/claude-usage ~/Library/LaunchAgents/com.claude.usagebar.plist
```

**Ubuntu**
```bash
rm -rf ~/.local/share/claude-usage ~/.config/autostart/claude-usage.desktop
```

---

## Ubuntu notes

The tray icon requires AppIndicator support. If the icon doesn't appear:

```bash
# Option 1 — install AppIndicator (needs sudo, one-time)
sudo apt install gir1.2-ayatana-appindicator3-0.1

# Option 2 — use X11 fallback (no sudo, X11 sessions only)
PYSTRAY_BACKEND=xorg python3 ~/.local/share/claude-usage/claude_usage_bar.py
```
