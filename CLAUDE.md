# Claude Meter

System tray app + browser extension that monitors Claude AI usage in real time. The app requires the browser extension — all usage data comes from claude.ai via the extension.

## Architecture

### Go Desktop App

A single-binary system tray application. Displays session and weekly usage percentages with reset times in the tray icon and menu.

- **`main.go`** — Entry point. Sets up systray menu, runs a 30-second refresh loop, starts the HTTP server. `AppState` holds all runtime state (web percentage, sections, alerts). Shows setup instructions when extension is not connected, switches to usage display when connected.
- **`server.go`** — HTTP server on `127.0.0.1:52413`. `POST /api/web-usage` receives usage data from the browser extension (percentage, source, sections with type/reset times). `GET /api/status` and `GET /api/debug` for diagnostics. Filters out unclassified sections.
- **`icon.go`** — Renders tray icons. On macOS: static Claude symbol. On Linux: dynamic icon with percentage text using system fonts (DejaVu, Liberation, or Noto Sans). Gray desaturated icon when extension is not connected. Caches rendered icons.
- **`settings.go`** — Reads/writes settings file. Only stores `notifications_enabled` (boolean).
- **`platform.go`** — OS-specific notifications (`terminal-notifier`/`osascript` on macOS, `notify-send` on Linux) and dialogs (`osascript`/`zenity`).

### Browser Extension (`extension/`)

Manifest V3 Chrome extension. Passively intercepts usage data from claude.ai page requests. When the tab is in the background, makes one API request per minute to keep data fresh.

- **`injector.js`** — Runs in MAIN world. Monkey-patches `window.fetch` to intercept usage API responses (`/api/(usage|rate|limit|billing|quota|entitlement)`). Posts intercepted data via `window.postMessage` to the content script.
- **`content.js`** — Runs in isolated world. Listens for messages from injector, extracts usage data from the `limits` array in API responses. Tracks the last successful API URL for background refresh. Responds to `REFRESH_USAGE` messages from the service worker by re-fetching the usage API directly. Sends to `http://127.0.0.1:52413/api/web-usage`. Debounces (5s minimum) and deduplicates.
- **`background.js`** — Stores last sync status in `chrome.storage.local`. Uses `chrome.alarms` to send `REFRESH_USAGE` to claude.ai tabs every 1 minute, ensuring the tray stays updated even when the tab is not focused.
- **`popup.html` / `popup.js`** — Shows connection status, last synced data, and configurable port.

### Data Flow

```
Tab active:
  claude.ai page (existing browser requests)
    -> injector.js (intercepts fetch responses, MAIN world)
    -> content.js (extracts usage data, isolated world)
    -> HTTP POST localhost:52413/api/web-usage
    -> Go app updates tray icon + menu

Tab in background:
  background.js (chrome.alarms, every 1 min)
    -> sends REFRESH_USAGE to content.js
    -> content.js re-fetches last known usage API URL
    -> HTTP POST localhost:52413/api/web-usage
    -> Go app updates tray icon + menu
```

### systray_fork/

Local fork of `fyne.io/systray` v1.12.2. Removes GTK3 dependency and uses D-Bus directly via `godbus/dbus`. Replaced via `go.mod` replace directive.

## Data Sources

The app reads **no local Claude files**. All usage data comes from the browser extension.

| Source | What it provides |
|--------|-----------------|
| Browser extension (from claude.ai page) | Session usage %, weekly usage %, reset times — these reflect **all usage across all devices and sessions** as reported by claude.ai |
| Settings file | Notification preference only (`notifications_enabled`) |

Settings path: `~/.config/claude-meter/settings.json` (Linux) / `~/Library/Application Support/claude-meter/settings.json` (macOS)

## Build & Run

```bash
make build     # go build -o claude-meter
make clean     # remove binary
```

## Configuration

- HTTP server port: `52413` (hardcoded)
- Refresh interval: 30 seconds
- Extension send debounce: 5 seconds
- Background refresh interval: 1 minute (when claude.ai tab is not focused)

## Dependencies

- Go 1.25+
- `fyne.io/systray` v1.12.2 (forked in `systray_fork/`, replaces GTK3 with D-Bus)
- `golang.org/x/image` (font rendering for Linux tray icon)
- Linux: `notify-send` for desktop notifications, `zenity` for dialogs
- macOS: `osascript` (built-in), optionally `terminal-notifier`
