package watcher

import (
	"InotiTidy/internal/config"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

	flightMu sync.Mutex
	flight   map[string]struct{} // paths currently being processed
}

// claim marks a path as being processed. It returns false if another goroutine
// is already handling that path, preventing duplicate work and races when
// fsnotify and the initial scan target the same file.
func (a *App) claim(path string) bool {
	a.flightMu.Lock()
	defer a.flightMu.Unlock()
	if a.flight == nil {
		a.flight = make(map[string]struct{})
	}
	if _, busy := a.flight[path]; busy {
		return false
	}
	a.flight[path] = struct{}{}
	return true
}

func (a *App) release(path string) {
	a.flightMu.Lock()
	defer a.flightMu.Unlock()
	delete(a.flight, path)
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

	path := config.GetStatsPath()
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

func (a *App) ensureStatsLocked() {
	if a.Stats == nil {
		a.Stats = &Stats{ExtensionCounts: make(map[string]int)}
	}
	if a.Stats.ExtensionCounts == nil {
		a.Stats.ExtensionCounts = make(map[string]int)
	}
	if a.Stats.LastResetDate == "" {
		a.Stats.LastResetDate = time.Now().Format("2006-01-02")
	}
}

func (a *App) SaveStats() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ensureStatsLocked()

	path := config.GetStatsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		a.log("Failed to create stats directory: %v", err)
		return
	}

	data, _ := json.MarshalIndent(a.Stats, "", "  ")
	if err := os.WriteFile(path, data, 0644); err != nil {
		a.log("Failed to write stats: %v", err)
	}
}

func (a *App) IncrementStats(ext string) {
	a.mu.Lock()
	a.ensureStatsLocked()
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

// ScanAll performs a bulk sort of all files currently in watch directories.
// It processes files concurrently but blocks until every file has been handled,
// so callers can rely on stats being fully updated when it returns.
func (a *App) ScanAll() {
	var wg sync.WaitGroup
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
			wg.Add(1)
			go func(p string) {
				defer wg.Done()
				a.handleEvent(p)
			}(path)
		}
	}
	wg.Wait()
}

func (a *App) handleEvent(path string) {
	// Skip if another goroutine is already processing this exact path.
	if !a.claim(path) {
		return
	}
	defer a.release(path)

	var prevSize int64 = -1
	for {
		stat, err := os.Stat(path)
		if err != nil {
			return
		}
		if !stat.Mode().IsRegular() {
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
		// An empty keyword would match every filename ("" is a substring of
		// anything) and silently disable all sorting — skip it.
		if key == "" {
			continue
		}
		if strings.Contains(upperName, strings.ToUpper(key)) {
			return
		}
	}

	lowerName := strings.ToLower(fileName)
	for _, rule := range a.Config.Rules {
		for _, e := range rule.Extensions {
			if matchesExtension(lowerName, e) {
				ext := filepath.Ext(fileName)
				a.move(path, rule.Target, fileName, ext)
				return
			}
		}
	}
}

// matchesExtension reports whether a lowercased filename ends with the given
// rule extension. The rule extension is normalized to start with a dot so that
// "pdf" and ".pdf" behave identically and a bare "df" cannot match "report.pdf"
// via a raw suffix check. Compound extensions like ".tar.gz" are still honored.
func matchesExtension(lowerName, ext string) bool {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext == "" || ext == "." {
		return false
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return strings.HasSuffix(lowerName, ext)
}

func (a *App) move(src, targetDir, name, ext string) {
	_ = os.MkdirAll(targetDir, 0o755)
	dest := uniqueDest(targetDir, name, ext)

	if err := os.Rename(src, dest); err != nil {
		if copyErr := moveFileWithCopyFallback(src, dest); copyErr != nil {
			a.log("Move error: %v", copyErr)
			return
		}
	}

	a.log("Sorted: %s", filepath.Base(dest))
	a.IncrementStats(ext)
}

// uniqueDest returns a destination path inside targetDir that does not yet
// exist. If name is taken it appends _1, _2, … before the extension so files
// are never silently overwritten.
func uniqueDest(targetDir, name, ext string) string {
	dest := filepath.Join(targetDir, name)
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		return dest
	}

	base := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		candidate := filepath.Join(targetDir, fmt.Sprintf("%s_%d%s", base, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

func moveFileWithCopyFallback(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dest)
		return err
	}

	if err := out.Close(); err != nil {
		_ = os.Remove(dest)
		return err
	}

	return os.Remove(src)
}
