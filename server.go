package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

var (
	lastPayloads   []map[string]interface{}
	payloadsMu     sync.Mutex
	maxPayloads    = 10
)

type webUsageRequest struct {
	Percentage float64 `json:"percentage"`
	Source     string  `json:"source"`
	Timestamp  string  `json:"timestamp"`
}

func startServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/web-usage", handleWebUsage)
	mux.HandleFunc("/api/status", handleStatus)
	mux.HandleFunc("/api/debug", handleDebug)

	server := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", httpPort),
		Handler: corsMiddleware(mux),
	}
	if err := server.ListenAndServe(); err != nil {
		log.Printf("[claude-meter] HTTP server error: %v", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func handleWebUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonResponse(w, 405, map[string]string{"error": "method not allowed"})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		jsonResponse(w, 400, map[string]string{"error": "bad request"})
		return
	}

	var req webUsageRequest
	if json.Unmarshal(body, &req) != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid json"})
		return
	}

	var raw map[string]interface{}
	json.Unmarshal(body, &raw)
	payloadsMu.Lock()
	lastPayloads = append(lastPayloads, raw)
	if len(lastPayloads) > maxPayloads {
		lastPayloads = lastPayloads[1:]
	}
	payloadsMu.Unlock()

	log.Printf("[claude-meter] POST /api/web-usage: %.1f%% via %s", req.Percentage, req.Source)

	if req.Percentage > 0 && req.Percentage <= 100 {
		state.mu.Lock()
		state.WebPct = &req.Percentage
		state.WebSource = req.Source
		ts := req.Timestamp
		if ts == "" {
			ts = time.Now().UTC().Format(time.RFC3339)
		}
		state.WebLastUpdate = ts

		if state.Total > 0 {
			newLimit := int(float64(state.Total) / (req.Percentage / 100.0))
			if newLimit != state.Limit {
				state.Limit = newLimit
				state.Alerted = make(map[int]bool)
				log.Printf("[claude-meter] Auto-calibrated limit to %d from web %.1f%%", newLimit, req.Percentage)
				go func() {
					saveSettings(Settings{Limit: newLimit})
				}()
			}
		}
		state.mu.Unlock()
		go doRefresh()
	} else if req.Percentage == 0 {
		state.mu.Lock()
		zero := 0.0
		state.WebPct = &zero
		state.WebSource = req.Source
		state.WebLastUpdate = req.Timestamp
		state.mu.Unlock()
		updateMenu()
	}

	jsonResponse(w, 200, map[string]bool{"ok": true})
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	state.mu.Lock()
	resp := map[string]interface{}{
		"app":             "claude-meter",
		"cli_pct":         state.Pct,
		"web_pct":         state.WebPct,
		"web_source":      state.WebSource,
		"web_last_update": state.WebLastUpdate,
	}
	state.mu.Unlock()
	jsonResponse(w, 200, resp)
}

func handleDebug(w http.ResponseWriter, r *http.Request) {
	state.mu.Lock()
	stateMap := map[string]interface{}{
		"pct":             state.Pct,
		"total":           state.Total,
		"limit":           state.Limit,
		"messages":        state.Messages,
		"input":           state.Input,
		"output":          state.Output,
		"cache_create":    state.CacheCreate,
		"cache_read":      state.CacheRead,
		"web_pct":         state.WebPct,
		"web_source":      state.WebSource,
		"web_last_update": state.WebLastUpdate,
	}
	state.mu.Unlock()

	payloadsMu.Lock()
	p := make([]map[string]interface{}, len(lastPayloads))
	copy(p, lastPayloads)
	payloadsMu.Unlock()

	jsonResponse(w, 200, map[string]interface{}{
		"last_payloads": p,
		"state":         stateMap,
	})
}

func jsonResponse(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}
