package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Settings struct {
	Limit int `json:"limit"`
}

func settingsPath() string {
	return filepath.Join(os.Getenv("HOME"), ".claude", "menubar_settings.json")
}

func loadSettings() Settings {
	s := Settings{Limit: defaultLimit}
	data, err := os.ReadFile(settingsPath())
	if err != nil {
		return s
	}
	json.Unmarshal(data, &s)
	if s.Limit <= 0 {
		s.Limit = defaultLimit
	}
	return s
}

func saveSettings(s Settings) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(settingsPath(), data, 0644)
}
