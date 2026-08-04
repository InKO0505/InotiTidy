package watcher

import (
	"InotiTidy/internal/config"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Stats is the persisted counter set shown on the dashboard.
type Stats struct {
	TotalSorted     int            `json:"total_sorted"`
	TodaySorted     int            `json:"today_sorted"`
	LastResetDate   string         `json:"last_reset_date"`
	ExtensionCounts map[string]int `json:"extension_counts"`
}

// statsStore holds the counters and flushes them to disk in a debounced way so
// a bulk scan of hundreds of files does not trigger hundreds of fsyncs.
type statsStore struct {
	mu    sync.Mutex
	s     Stats
	dirty bool
	path  string
}

func newStatsStore() *statsStore {
	return &statsStore{
		path: config.GetStatsPath(),
		s:    Stats{ExtensionCounts: map[string]int{}},
	}
}

// Load reads stats from disk, resetting the daily counter on a new day.
func (st *statsStore) Load() {
	st.mu.Lock()
	defer st.mu.Unlock()

	data, err := os.ReadFile(st.path)
	if err == nil {
		var s Stats
		if json.Unmarshal(data, &s) == nil {
			st.s = s
		}
	}
	if st.s.ExtensionCounts == nil {
		st.s.ExtensionCounts = map[string]int{}
	}
	today := time.Now().Format("2006-01-02")
	if st.s.LastResetDate != today {
		st.s.TodaySorted = 0
		st.s.LastResetDate = today
		st.dirty = true
	}
}

// Snapshot returns a copy of the current stats for display.
func (st *statsStore) Snapshot() Stats {
	st.mu.Lock()
	defer st.mu.Unlock()
	cp := st.s
	cp.ExtensionCounts = make(map[string]int, len(st.s.ExtensionCounts))
	maps.Copy(cp.ExtensionCounts, st.s.ExtensionCounts)
	return cp
}

// Increment records one sorted file. It only marks dirty; Flush does the write.
func (st *statsStore) Increment(ext string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.s.ExtensionCounts == nil {
		st.s.ExtensionCounts = map[string]int{}
	}
	if st.s.LastResetDate == "" {
		st.s.LastResetDate = time.Now().Format("2006-01-02")
	}
	st.s.TotalSorted++
	st.s.TodaySorted++
	if ext = strings.ToLower(ext); ext != "" {
		st.s.ExtensionCounts[ext]++
	}
	st.dirty = true
}

// Flush writes stats to disk if they changed since the last flush.
func (st *statsStore) Flush() {
	st.mu.Lock()
	if !st.dirty {
		st.mu.Unlock()
		return
	}
	data, _ := json.MarshalIndent(st.s, "", "  ")
	st.dirty = false
	path := st.path
	st.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}
