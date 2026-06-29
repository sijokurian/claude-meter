package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

type Settings struct {
	NotificationsEnabled bool `json:"notifications_enabled"`
}

func settingsPath() string {
	var dir string
	if runtime.GOOS == "darwin" {
		dir = filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "claude-meter")
	} else {
		dir = filepath.Join(os.Getenv("HOME"), ".config", "claude-meter")
	}
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "settings.json")
}

func loadSettings() Settings {
	s := Settings{NotificationsEnabled: true}
	data, err := os.ReadFile(settingsPath())
	if err != nil {
		return s
	}
	json.Unmarshal(data, &s)
	return s
}

func saveSettings(s Settings) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(settingsPath(), data, 0644)
}
