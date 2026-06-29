package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/systray"
)

const (
	windowHours     = 5
	refreshInterval = 30 * time.Second
	defaultLimit    = 1_000_000
	cacheReadWeight = 1.0 / 150.0
	httpPort        = 52413
)

type AppState struct {
	mu            sync.Mutex
	Pct           float64
	Total         int
	Limit         int
	Messages      int
	Input         int
	Output        int
	CacheCreate   int
	CacheRead     int
	WebPct        *float64
	WebSource     string
	WebLastUpdate string
	Alerted       map[int]bool
}

var state = &AppState{
	Alerted: make(map[int]bool),
}

// Menu items
var (
	mUsage      *systray.MenuItem
	mMessages   *systray.MenuItem
	mInput      *systray.MenuItem
	mOutput     *systray.MenuItem
	mCacheCreate *systray.MenuItem
	mCacheRead  *systray.MenuItem
	mWebUsage   *systray.MenuItem
	mWebLast    *systray.MenuItem
	mWindow     *systray.MenuItem
	mLimit      *systray.MenuItem
	mSetLimit   *systray.MenuItem
	mCalibrate  *systray.MenuItem
	mResetAlerts *systray.MenuItem
	mRefresh    *systray.MenuItem
	mQuit       *systray.MenuItem
)

func main() {
	s := loadSettings()
	state.Limit = s.Limit
	initIcons()
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(makeIcon(0))
	systray.SetTooltip("Claude Usage")

	mUsage = systray.AddMenuItem("Usage: ...", "")
	mUsage.Disable()
	mMessages = systray.AddMenuItem("Messages (5h): 0", "")
	mMessages.Disable()
	systray.AddSeparator()
	mInput = systray.AddMenuItem("  Input:          0", "")
	mInput.Disable()
	mOutput = systray.AddMenuItem("  Output:         0", "")
	mOutput.Disable()
	mCacheCreate = systray.AddMenuItem("  Cache created:  0", "")
	mCacheCreate.Disable()
	mCacheRead = systray.AddMenuItem("  Cache read:     0", "")
	mCacheRead.Disable()
	systray.AddSeparator()
	mWebUsage = systray.AddMenuItem("Web: waiting for extension...", "")
	mWebUsage.Disable()
	mWebLast = systray.AddMenuItem("  Last sync: —", "")
	mWebLast.Disable()
	systray.AddSeparator()
	mWindow = systray.AddMenuItem(fmt.Sprintf("Window: last %d hours", windowHours), "")
	mWindow.Disable()
	mLimit = systray.AddMenuItem(fmt.Sprintf("Limit: %s tokens", fmtTokens(state.Limit)), "")
	mLimit.Disable()
	mSetLimit = systray.AddMenuItem("Set Limit...", "")
	mCalibrate = systray.AddMenuItem("Calibrate from Website...", "")
	mResetAlerts = systray.AddMenuItem("Reset Alerts", "")
	systray.AddSeparator()
	mRefresh = systray.AddMenuItem("Refresh Now", "")
	mQuit = systray.AddMenuItem("Quit", "")

	go startServer()
	go refreshLoop()
	go handleClicks()
}

func onExit() {}

func refreshLoop() {
	doRefresh()
	ticker := time.NewTicker(refreshInterval)
	for range ticker.C {
		doRefresh()
	}
}

func doRefresh() {
	usage := getUsage(windowHours)

	state.mu.Lock()
	state.Total = usage.Total
	state.Messages = usage.Messages
	state.Input = usage.Input
	state.Output = usage.Output
	state.CacheCreate = usage.CacheCreate
	state.CacheRead = usage.CacheRead

	pct := 0.0
	if state.Limit > 0 {
		pct = math.Min(100.0, float64(usage.Total)/float64(state.Limit)*100.0)
	}
	state.Pct = pct
	alerted := state.Alerted
	limit := state.Limit
	total := usage.Total
	state.mu.Unlock()

	systray.SetIcon(makeIcon(pct))
	systray.SetTooltip(fmt.Sprintf("Claude Usage: %.0f%%", pct))
	if isRunningOnMac() {
		systray.SetTitle(fmt.Sprintf(" %d%%", int(pct)))
	}

	updateMenu()

	crossed := int(pct/10) * 10
	for m := 10; m <= crossed; m += 10 {
		if !alerted[m] {
			state.mu.Lock()
			state.Alerted[m] = true
			state.mu.Unlock()

			var msg string
			switch {
			case m >= 90:
				msg = "Approaching limit — consider pausing!"
			case m >= 70:
				msg = "Getting close to your limit."
			default:
				msg = fmt.Sprintf("Used %s tokens in the last %d hours.", fmtTokens(total), windowHours)
			}
			go notify("Claude Usage", fmt.Sprintf("%d%% of limit reached", m), msg)
		}
	}
	_ = limit
}

func updateMenu() {
	state.mu.Lock()
	mUsage.SetTitle(fmt.Sprintf("Usage: %s / %s  (%.1f%%)", fmtTokens(state.Total), fmtTokens(state.Limit), state.Pct))
	mMessages.SetTitle(fmt.Sprintf("Messages (5h): %d", state.Messages))
	mInput.SetTitle(fmt.Sprintf("  Input:          %s", fmtTokens(state.Input)))
	mOutput.SetTitle(fmt.Sprintf("  Output:         %s", fmtTokens(state.Output)))
	mCacheCreate.SetTitle(fmt.Sprintf("  Cache created:  %s", fmtTokens(state.CacheCreate)))
	mCacheRead.SetTitle(fmt.Sprintf("  Cache read:     %s", fmtTokens(state.CacheRead)))
	mLimit.SetTitle(fmt.Sprintf("Limit: %s tokens", fmtTokens(state.Limit)))

	if state.WebPct != nil {
		mWebUsage.SetTitle(fmt.Sprintf("Web (claude.ai): %.1f%%", *state.WebPct))
	} else {
		mWebUsage.SetTitle("Web: waiting for extension...")
	}

	if state.WebLastUpdate != "" {
		t, err := time.Parse(time.RFC3339, state.WebLastUpdate)
		if err != nil {
			t, err = time.Parse(time.RFC3339Nano, state.WebLastUpdate)
		}
		if err == nil {
			ago := time.Since(t)
			mins := int(ago.Minutes())
			switch {
			case mins < 1:
				mWebLast.SetTitle("  Last sync: just now")
			case mins < 60:
				mWebLast.SetTitle(fmt.Sprintf("  Last sync: %dm ago", mins))
			default:
				mWebLast.SetTitle(fmt.Sprintf("  Last sync: %dh %dm ago", mins/60, mins%60))
			}
		}
	} else {
		mWebLast.SetTitle("  Last sync: —")
	}
	state.mu.Unlock()
}

func handleClicks() {
	for {
		select {
		case <-mSetLimit.ClickedCh:
			go onSetLimit()
		case <-mCalibrate.ClickedCh:
			go onCalibrate()
		case <-mResetAlerts.ClickedCh:
			go onResetAlerts()
		case <-mRefresh.ClickedCh:
			go doRefresh()
		case <-mQuit.ClickedCh:
			systray.Quit()
		}
	}
}

func onSetLimit() {
	state.mu.Lock()
	current := state.Limit
	state.mu.Unlock()

	val, ok := askInput("Set Usage Limit",
		"Enter token limit for 5-hour window\n(e.g. 22800000 for 22.8M):",
		strconv.Itoa(current))
	if !ok {
		return
	}

	val = strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(val), ",", ""), "_", "")
	newLimit, err := strconv.Atoi(val)
	if err != nil || newLimit <= 0 {
		showAlert("Invalid value", "Please enter a positive number.")
		return
	}

	state.mu.Lock()
	state.Limit = newLimit
	state.Alerted = make(map[int]bool)
	state.mu.Unlock()
	saveSettings(Settings{Limit: newLimit})
	doRefresh()
}

func onCalibrate() {
	state.mu.Lock()
	total := state.Total
	state.mu.Unlock()

	val, ok := askInput("Calibrate from Website",
		fmt.Sprintf("Open claude.ai and check your current usage %%.\nEnter that percentage to calibrate the token limit.\n\nCurrent measured tokens: %s", fmtTokens(total)),
		"")
	if !ok || total == 0 {
		return
	}

	val = strings.TrimSpace(strings.TrimRight(val, "%"))
	sitePct, err := strconv.ParseFloat(val, 64)
	if err != nil || sitePct <= 0 || sitePct > 100 {
		showAlert("Invalid value", "Enter a percentage between 1 and 100.")
		return
	}

	newLimit := int(float64(total) / (sitePct / 100.0))
	state.mu.Lock()
	state.Limit = newLimit
	state.Alerted = make(map[int]bool)
	state.mu.Unlock()
	saveSettings(Settings{Limit: newLimit})
	doRefresh()
	showAlert("Calibrated", fmt.Sprintf("Token limit set to %s\nbased on %.1f%% from website.", fmtTokens(newLimit), sitePct))
}

func onResetAlerts() {
	state.mu.Lock()
	state.Alerted = make(map[int]bool)
	state.mu.Unlock()
	notify("Claude Usage", "Alerts reset", "You'll be notified again at each 10% milestone.")
}

func fmtTokens(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.2fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return strconv.Itoa(n)
}

func isRunningOnMac() bool {
	return false // overridden by build tag if needed
}
