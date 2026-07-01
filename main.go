package main

import (
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/systray"
)

const (
	refreshInterval  = 30 * time.Second
	httpPort         = 52413
	staleThreshold   = 5 * time.Minute
)

type AppState struct {
	mu                   sync.Mutex
	WebPct               *float64
	WebSource            string
	WebLastUpdate        string
	WebResetsAt          string
	WebSections          []webUsageSection
	Alerted              map[int]bool
	NotificationsEnabled bool
}

var state = &AppState{
	Alerted:              make(map[int]bool),
	NotificationsEnabled: true,
}

// Menu items
var (
	mUsage         *systray.MenuItem
	mSession       *systray.MenuItem
	mWeekly        *systray.MenuItem
	mSyncStatus    *systray.MenuItem
	mNotifications *systray.MenuItem
	mRefresh       *systray.MenuItem
	mQuit          *systray.MenuItem
)

func main() {
	s := loadSettings()
	state.NotificationsEnabled = s.NotificationsEnabled
	initIcons()
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(errorIconData)
	systray.SetTooltip("Claude Meter — waiting for extension")

	mUsage = systray.AddMenuItem("Extension not connected", "")
	mUsage.Disable()
	mSession = systray.AddMenuItem("  1. Install the browser extension", "")
	mSession.Disable()
	mWeekly = systray.AddMenuItem("  2. Login to claude.ai in Chrome", "")
	mWeekly.Disable()
	systray.AddSeparator()
	mSyncStatus = systray.AddMenuItem("Last sync: —", "")
	mSyncStatus.Disable()
	if state.NotificationsEnabled {
		mNotifications = systray.AddMenuItem("Disable Notifications", "")
	} else {
		mNotifications = systray.AddMenuItem("Enable Notifications", "")
	}
	systray.AddSeparator()
	mRefresh = systray.AddMenuItem("Refresh Now", "")
	mQuit = systray.AddMenuItem("Quit", "")

	go startServer()
	go refreshLoop()
	go handleClicks()
}

func onExit() {}

var firstRefreshDone atomic.Bool

func refreshLoop() {
	doRefresh()
	firstRefreshDone.Store(true)
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()
	for range ticker.C {
		doRefresh()
	}
}

func isConnected() bool {
	if state.WebPct == nil && len(state.WebSections) == 0 {
		return false
	}
	if state.WebLastUpdate == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, state.WebLastUpdate)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, state.WebLastUpdate)
	}
	if err != nil {
		return false
	}
	return time.Since(t) < staleThreshold
}

func doRefresh() {
	state.mu.Lock()
	alerted := state.Alerted
	connected := isConnected()

	sessionPct := 0.0
	if connected {
		for _, sec := range state.WebSections {
			if sec.Type == "session" {
				sessionPct = sec.Percentage
				break
			}
		}
		if sessionPct == 0 && state.WebPct != nil {
			sessionPct = *state.WebPct
		}
	}
	state.mu.Unlock()

	if connected {
		systray.SetIcon(trayIconData)
		systray.SetTooltip(fmt.Sprintf("Claude Usage: %.0f%%", sessionPct))
		if isRunningOnMac() {
			systray.SetTitle(fmt.Sprintf(" %d%%", int(sessionPct)))
		} else {
			iconName, themePath := writeIconFile(sessionPct)
			systray.SetIconByName(iconName, themePath)
			systray.SetLabel(fmt.Sprintf("%d%%", int(sessionPct)))
		}
	} else {
		systray.SetIcon(errorIconData)
		systray.SetTooltip("Claude Meter — extension not connected")
		if isRunningOnMac() {
			systray.SetTitle(" —")
		} else {
			iconName, themePath := writeErrorIconFile()
			systray.SetIconByName(iconName, themePath)
			systray.SetLabel("—")
		}
	}

	updateMenu()

	if connected {
		crossed := int(sessionPct/10) * 10
		highestNew := 0
		for m := 10; m <= crossed; m += 10 {
			if !alerted[m] {
				highestNew = m
				state.mu.Lock()
				state.Alerted[m] = true
				state.mu.Unlock()
			}
		}

		state.mu.Lock()
		notifEnabled := state.NotificationsEnabled
		sessionReset := ""
		for _, sec := range state.WebSections {
			if sec.Type == "session" && sec.ResetsAt != "" {
				sessionReset = sec.ResetsAt
				break
			}
		}
		state.mu.Unlock()

		if highestNew > 0 && firstRefreshDone.Load() && notifEnabled {
			var msg string
			switch {
			case highestNew >= 90:
				msg = "Approaching limit — consider pausing!"
			case highestNew >= 70:
				msg = "Getting close to your limit."
			default:
				if sessionReset != "" {
					msg = fmt.Sprintf("Session %s.", sessionReset)
				} else {
					msg = "Current session usage."
				}
			}
			go notify("Claude Usage", fmt.Sprintf("%d%% of limit reached.", highestNew), msg)
		}
	}
}

func updateMenu() {
	state.mu.Lock()
	defer state.mu.Unlock()

	connected := isConnected()

	if !connected {
		mUsage.SetTitle("Extension not connected")
		mSession.SetTitle("  1. Install the browser extension")
		mWeekly.SetTitle("  2. Login to claude.ai in Chrome")
		mSyncStatus.SetTitle("Last sync: —")
	} else {
		sessPct := -1.0
		sessionReset := ""
		weeklyPct := -1.0
		weeklyReset := ""
		for _, sec := range state.WebSections {
			switch sec.Type {
			case "session":
				sessPct = sec.Percentage
				sessionReset = sec.ResetsAt
			case "weekly_all":
				weeklyPct = sec.Percentage
				weeklyReset = sec.ResetsAt
			}
		}

		displayPct := 0.0
		if sessPct >= 0 {
			displayPct = sessPct
		} else if state.WebPct != nil {
			displayPct = *state.WebPct
		}

		mUsage.SetTitle(fmt.Sprintf("Usage: %.0f%%", displayPct))

		if sessPct >= 0 && sessionReset != "" {
			mSession.SetTitle(fmt.Sprintf("  Session: %.0f%% — %s", sessPct, sessionReset))
		} else if sessPct >= 0 {
			mSession.SetTitle(fmt.Sprintf("  Session: %.0f%%", sessPct))
		} else {
			mSession.SetTitle("  Session: —")
		}

		if weeklyPct >= 0 && weeklyReset != "" {
			mWeekly.SetTitle(fmt.Sprintf("  Weekly: %.0f%% — %s", weeklyPct, weeklyReset))
		} else if weeklyPct >= 0 {
			mWeekly.SetTitle(fmt.Sprintf("  Weekly: %.0f%%", weeklyPct))
		} else {
			mWeekly.SetTitle("  Weekly: —")
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
					mSyncStatus.SetTitle("Last sync: just now")
				case mins < 60:
					mSyncStatus.SetTitle(fmt.Sprintf("Last sync: %dm ago", mins))
				default:
					mSyncStatus.SetTitle(fmt.Sprintf("Last sync: %dh %dm ago", mins/60, mins%60))
				}
			}
		}
	}

}

func handleClicks() {
	for {
		select {
		case <-mNotifications.ClickedCh:
			go onToggleNotifications()
		case <-mRefresh.ClickedCh:
			go doRefresh()
		case <-mQuit.ClickedCh:
			systray.Quit()
		}
	}
}

func onToggleNotifications() {
	state.mu.Lock()
	state.NotificationsEnabled = !state.NotificationsEnabled
	enabled := state.NotificationsEnabled
	state.mu.Unlock()

	if enabled {
		mNotifications.SetTitle("Disable Notifications")
	} else {
		mNotifications.SetTitle("Enable Notifications")
	}

	s := loadSettings()
	s.NotificationsEnabled = enabled
	saveSettings(s)
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
	return runtime.GOOS == "darwin"
}
