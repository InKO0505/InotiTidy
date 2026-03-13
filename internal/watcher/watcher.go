package watcher

import (
	"InotiTidy/internal/config"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Stats struct {
	TotalSorted     int            `json:"total_sorted"`
	TodaySorted     int            `json:"today_sorted"`
	LastResetDate   string         `json:"last_reset_date"`
	ExtensionCounts map[string]int `json:"extension_counts"`
}

type App struct {
	Config *config.Config
	Logger func(string)
	Stats  *Stats
	mu     sync.Mutex
}

func (a *App) log(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	if a.Logger != nil {
		a.Logger(msg)
	} else {
		log.Println(msg)
	}
}

func (a *App) LoadStats() {
	a.mu.Lock()
	defer a.mu.Unlock()

	path := filepath.Join(config.GetConfigPath(), "..", "stats.json")
	data, err := os.ReadFile(path)
	if err != nil {
		a.Stats = &Stats{ExtensionCounts: make(map[string]int)}
		return
	}

	var s Stats
	if err := json.Unmarshal(data, &s); err != nil {
		a.Stats = &Stats{ExtensionCounts: make(map[string]int)}
		return
	}
	a.Stats = &s
	if a.Stats.ExtensionCounts == nil {
		a.Stats.ExtensionCounts = make(map[string]int)
	}

	// Reset daily stats if date changed
	today := time.Now().Format("2006-01-02")
	if a.Stats.LastResetDate != today {
		a.Stats.TodaySorted = 0
		a.Stats.LastResetDate = today
	}
}

func (a *App) SaveStats() {
	a.mu.Lock()
	defer a.mu.Unlock()

	path := filepath.Join(config.GetConfigPath(), "..", "stats.json")
	data, _ := json.MarshalIndent(a.Stats, "", "  ")
	_ = os.WriteFile(path, data, 0644)
}

func (a *App) IncrementStats(ext string) {
	a.mu.Lock()
	a.Stats.TotalSorted++
	a.Stats.TodaySorted++
	a.Stats.ExtensionCounts[strings.ToLower(ext)]++
	a.mu.Unlock()
	a.SaveStats()
}

func (a *App) Start(ctx context.Context) error {
	a.LoadStats()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	// Initial Scan
	a.log("Performing initial scan of watch directories...")
	a.ScanAll()

	// Add directories to watch
	for _, dir := range a.Config.WatchDirs {
		absPath, _ := filepath.Abs(dir)
		err = watcher.Add(absPath)
		if err != nil {
			a.log("Error watching %s: %v", dir, err)
		} else {
			a.log("Watching: %s", dir)
		}
	}

	a.log("InotiTidy (Event-Driven) started successfully")

	for {
		select {
		case <-ctx.Done():
			a.log("InotiTidy stopping...")
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			// We only care about file creation or moves into the directory
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
				// Small delay to let file system settle
				go func(p string) {
					time.Sleep(100 * time.Millisecond)
					a.handleEvent(p)
				}(event.Name)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			a.log("Watcher error: %v", err)
		}
	}
}

// ScanAll performs a bulk sort of all files currently in watch directories
func (a *App) ScanAll() {
	for _, dir := range a.Config.WatchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			a.log("Error scanning %s: %v", dir, err)
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			go a.handleEvent(path)
		}
	}
}

func (a *App) handleEvent(path string) {
	var prevSize int64 = -1
	for {
		stat, err := os.Stat(path)
		if err != nil {
			return
		}
		if stat.Size() == prevSize {
			break
		}
		prevSize = stat.Size()
		time.Sleep(500 * time.Millisecond)
	}

	fileName := filepath.Base(path)
	upperName := strings.ToUpper(fileName)
	for _, key := range a.Config.Excludes {
		if strings.Contains(upperName, strings.ToUpper(key)) {
			return
		}
	}

	lowerName := strings.ToLower(fileName)
	for _, rule := range a.Config.Rules {
		for _, e := range rule.Extensions {
			if strings.HasSuffix(lowerName, strings.ToLower(e)) {
				ext := filepath.Ext(fileName)
				a.move(path, rule.Target, fileName, ext)
				return
			}
		}
	}
}

func (a *App) move(src, targetDir, name, ext string) {
	_ = os.MkdirAll(targetDir, 0o755)
	dest := filepath.Join(targetDir, name)

	if _, err := os.Stat(dest); err == nil {
		base := strings.TrimSuffix(name, ext)
		dest = filepath.Join(targetDir, fmt.Sprintf("%s_%d%s", base, time.Now().Unix(), ext))
	}

	if err := os.Rename(src, dest); err != nil {
		a.log("Move error: %v", err)
	} else {
		a.log("Sorted: %s", name)
		a.IncrementStats(ext)
	}
}
