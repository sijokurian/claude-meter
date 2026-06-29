package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Usage struct {
	Input       int
	Output      int
	CacheCreate int
	CacheRead   int
	Total       int
	Messages    int
}

type jsonlEntry struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	RequestID string          `json:"requestId"`
	Message   json.RawMessage `json:"message"`
}

type messageBody struct {
	Usage *tokenUsage `json:"usage"`
}

type tokenUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

func getUsage(windowHours int) Usage {
	base := filepath.Join(os.Getenv("HOME"), ".claude", "projects")
	cutoff := time.Now().UTC().Add(-time.Duration(windowHours) * time.Hour)
	seen := make(map[string]bool)

	var u Usage

	filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

		for scanner.Scan() {
			var entry jsonlEntry
			if json.Unmarshal(scanner.Bytes(), &entry) != nil {
				continue
			}
			if entry.Type != "assistant" || entry.Message == nil {
				continue
			}
			if entry.Timestamp == "" {
				continue
			}
			t, err := time.Parse(time.RFC3339Nano, entry.Timestamp)
			if err != nil {
				t, err = time.Parse("2006-01-02T15:04:05.000Z", entry.Timestamp)
				if err != nil {
					continue
				}
			}
			if t.Before(cutoff) {
				continue
			}
			if entry.RequestID != "" {
				if seen[entry.RequestID] {
					continue
				}
				seen[entry.RequestID] = true
			}

			var msg messageBody
			if json.Unmarshal(entry.Message, &msg) != nil || msg.Usage == nil {
				continue
			}

			u.Input += msg.Usage.InputTokens
			u.Output += msg.Usage.OutputTokens
			u.CacheCreate += msg.Usage.CacheCreationInputTokens
			u.CacheRead += msg.Usage.CacheReadInputTokens
			u.Messages++
		}
		return nil
	})

	u.Total = u.Input + u.Output + u.CacheCreate + int(float64(u.CacheRead)*cacheReadWeight)
	return u
}
