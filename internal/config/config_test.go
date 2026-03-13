package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseYAMLConfig(t *testing.T) {
	src := `watch_directories:
  - "~/Downloads"
exclude_keywords:
  - "KEEP"
rules:
  - extensions: [".PDF", ".txt"]
    target: "~/Documents"
`

	cfg, err := parseYAMLConfig(src)
	if err != nil {
		t.Fatalf("parseYAMLConfig error: %v", err)
	}

	if len(cfg.WatchDirs) != 1 || cfg.WatchDirs[0] != "~/Downloads" {
		t.Fatalf("unexpected watch dirs: %#v", cfg.WatchDirs)
	}
	if len(cfg.Excludes) != 1 || cfg.Excludes[0] != "KEEP" {
		t.Fatalf("unexpected excludes: %#v", cfg.Excludes)
	}
	wantExt := []string{".pdf", ".txt"}
	if len(cfg.Rules) != 1 || !reflect.DeepEqual(cfg.Rules[0].Extensions, wantExt) {
		t.Fatalf("unexpected rule extensions: %#v", cfg.Rules)
	}
}

func TestSaveAndLoadFromPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := &Config{
		WatchDirs: []string{filepath.Join(home, "Downloads")},
		Excludes:  []string{"KEEP"},
		Rules: []Rule{{
			Extensions: []string{".pdf"},
			Target:     filepath.Join(home, "Documents"),
		}},
	}

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("LoadFromPath error: %v", err)
	}

	if !reflect.DeepEqual(loaded.WatchDirs, cfg.WatchDirs) {
		t.Fatalf("watch dirs mismatch: got %#v want %#v", loaded.WatchDirs, cfg.WatchDirs)
	}
	if !reflect.DeepEqual(loaded.Excludes, cfg.Excludes) {
		t.Fatalf("excludes mismatch: got %#v want %#v", loaded.Excludes, cfg.Excludes)
	}
	if !reflect.DeepEqual(loaded.Rules, cfg.Rules) {
		t.Fatalf("rules mismatch: got %#v want %#v", loaded.Rules, cfg.Rules)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved file error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("saved file is empty")
	}
}
