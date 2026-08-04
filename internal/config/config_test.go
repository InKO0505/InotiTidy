package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseLegacyFormat(t *testing.T) {
	src := []byte(`watch_directories:
  - "~/Downloads"
exclude_keywords:
  - "KEEP"
rules:
  - extensions: [".PDF", "txt"]
    target: "~/Documents"
`)
	cfg, err := parse(src)
	if err != nil {
		t.Fatalf("parse legacy: %v", err)
	}
	if len(cfg.Watch) != 1 || cfg.Watch[0].Path != "~/Downloads" {
		t.Fatalf("watch: %#v", cfg.Watch)
	}
	if len(cfg.GlobalExclude) != 1 || cfg.GlobalExclude[0] != "KEEP" {
		t.Fatalf("excludes: %#v", cfg.GlobalExclude)
	}
	r := cfg.Rules[0]
	if len(r.Extensions) != 2 || r.Extensions[0] != ".pdf" || r.Extensions[1] != ".txt" {
		t.Fatalf("extensions not normalized: %#v", r.Extensions)
	}
	if r.Action != ActionMove || r.OnConflict != ConflictRename {
		t.Fatalf("defaults not applied: %+v", r)
	}
}

func TestParseV2Format(t *testing.T) {
	src := []byte(`version: 2
settle_interval: 250ms
watch:
  - path: "~/Downloads"
    recursive: true
global_exclude: ["KEEP"]
rules:
  - name: Big videos
    extensions: [mkv, mp4]
    min_size: 100MB
    older_than: 24h
    action: move
    target: "~/Videos"
    on_conflict: skip
`)
	cfg, err := parse(src)
	if err != nil {
		t.Fatalf("parse v2: %v", err)
	}
	if cfg.SettleInterval != 250*time.Millisecond {
		t.Fatalf("settle: %v", cfg.SettleInterval)
	}
	if !cfg.Watch[0].Recursive {
		t.Fatalf("recursive lost")
	}
	r := cfg.Rules[0]
	if r.MinSize != 100*1024*1024 {
		t.Fatalf("min_size: %d", r.MinSize)
	}
	if r.OlderThan != 24*time.Hour {
		t.Fatalf("older_than: %v", r.OlderThan)
	}
	if r.OnConflict != ConflictSkip {
		t.Fatalf("on_conflict: %q", r.OnConflict)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")

	cfg := &Config{
		Watch:          []WatchDir{{Path: filepath.Join(home, "Downloads"), Recursive: true}},
		GlobalExclude:  []string{"KEEP"},
		SettleInterval: 300 * time.Millisecond,
		Rules: []Rule{{
			Name:       "Docs",
			Extensions: []string{".pdf"},
			MinSize:    1024,
			Action:     ActionMove,
			Target:     filepath.Join(home, "Documents"),
			OnConflict: ConflictRename,
		}},
	}

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := cfg.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Watch[0].Path != cfg.Watch[0].Path || !loaded.Watch[0].Recursive {
		t.Fatalf("watch mismatch: %#v", loaded.Watch)
	}
	if loaded.Rules[0].Target != cfg.Rules[0].Target || loaded.Rules[0].MinSize != 1024 {
		t.Fatalf("rule mismatch: %#v", loaded.Rules[0])
	}
	if loaded.SettleInterval != 300*time.Millisecond {
		t.Fatalf("settle mismatch: %v", loaded.SettleInterval)
	}

	// Saved file must collapse home to ~.
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "~/Downloads") {
		t.Fatalf("home not collapsed:\n%s", data)
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"no watch", Config{}, true},
		{"good", Config{
			Watch: []WatchDir{{Path: "/x"}},
			Rules: []Rule{{Extensions: []string{".pdf"}, Action: ActionMove, Target: "/y", OnConflict: ConflictRename}},
		}, false},
		{"bad action", Config{
			Watch: []WatchDir{{Path: "/x"}},
			Rules: []Rule{{Extensions: []string{".pdf"}, Action: "nuke", Target: "/y", OnConflict: ConflictRename}},
		}, true},
		{"move without target", Config{
			Watch: []WatchDir{{Path: "/x"}},
			Rules: []Rule{{Extensions: []string{".pdf"}, Action: ActionMove, OnConflict: ConflictRename}},
		}, true},
		{"trash without target ok", Config{
			Watch: []WatchDir{{Path: "/x"}},
			Rules: []Rule{{Extensions: []string{".tmp"}, Action: ActionTrash, OnConflict: ConflictRename}},
		}, false},
		{"rule without conditions", Config{
			Watch: []WatchDir{{Path: "/x"}},
			Rules: []Rule{{Action: ActionMove, Target: "/y", OnConflict: ConflictRename}},
		}, true},
		{"bad regex", Config{
			Watch: []WatchDir{{Path: "/x"}},
			Rules: []Rule{{NameRegex: "(", Action: ActionMove, Target: "/y", OnConflict: ConflictRename}},
		}, true},
	}
	for _, c := range cases {
		err := c.cfg.Validate()
		if (err != nil) != c.wantErr {
			t.Errorf("%s: got err=%v, wantErr=%v", c.name, err, c.wantErr)
		}
	}
}

func TestParseSize(t *testing.T) {
	cases := map[string]int64{
		"":      0,
		"1024":  1024,
		"1KB":   1024,
		"1K":    1024,
		"100MB": 100 * 1024 * 1024,
		"2G":    2 * 1024 * 1024 * 1024,
		"1.5MB": int64(1.5 * 1024 * 1024),
	}
	for in, want := range cases {
		got, err := ParseSize(in)
		if err != nil {
			t.Fatalf("ParseSize(%q): %v", in, err)
		}
		if got != want {
			t.Errorf("ParseSize(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestPathHelpers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")

	wantDir := filepath.Join(home, ".config", "inotitidy")
	if GetConfigDir() != wantDir {
		t.Fatalf("GetConfigDir = %q, want %q", GetConfigDir(), wantDir)
	}
	if GetStatsPath() != filepath.Join(wantDir, "stats.json") {
		t.Fatalf("GetStatsPath = %q", GetStatsPath())
	}
	if GetHistoryPath() != filepath.Join(wantDir, "history.jsonl") {
		t.Fatalf("GetHistoryPath = %q", GetHistoryPath())
	}
}
