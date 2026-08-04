// Package watcher contains InotiTidy's file-sorting engine and the event-driven
// daemon that drives it.
package watcher

import (
	"InotiTidy/internal/config"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

// App is the sorting engine. A single App may run as a daemon (Start) or perform
// one-shot scans (ScanAll). It is safe for concurrent file handling.
type App struct {
	// Logger, if set, receives every log line (used by the TUI). When nil, logs
	// go to slog so systemd/journalctl captures them.
	Logger func(string)

	// DryRun makes the engine plan and log actions without touching files.
	DryRun bool

	cfgPtr  atomic.Pointer[config.Config]
	stats   *statsStore
	history *history
	batchID string

	flightMu sync.Mutex
	flight   map[string]struct{}
}

// New builds an App with stats and history stores initialized.
func New(cfg *config.Config) *App {
	a := &App{
		stats:   newStatsStore(),
		history: newHistory(),
		batchID: newBatchID(),
	}
	a.SetConfig(cfg)
	return a
}

// Cfg returns the current configuration (safe under concurrent reload).
func (a *App) Cfg() *config.Config { return a.cfgPtr.Load() }

// SetConfig atomically swaps the active configuration, pre-compiling regexes.
func (a *App) SetConfig(cfg *config.Config) {
	if cfg != nil {
		cfg.Compile()
	}
	a.cfgPtr.Store(cfg)
}

func newBatchID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

func (a *App) ensure() {
	if a.stats == nil {
		a.stats = newStatsStore()
	}
	if a.history == nil {
		a.history = newHistory()
	}
	if a.batchID == "" {
		a.batchID = newBatchID()
	}
	// Pre-compile rule regexes single-threaded before any concurrent matching.
	if cfg := a.Cfg(); cfg != nil {
		cfg.Compile()
	}
}

func (a *App) log(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	if a.Logger != nil {
		a.Logger(msg)
	} else {
		slog.Info(msg)
	}
}

// LoadStats loads persisted counters. Snapshot exposes them for display.
func (a *App) LoadStats() { a.ensure(); a.stats.Load() }

// Snapshot returns a copy of the current stats.
func (a *App) Snapshot() Stats { a.ensure(); return a.stats.Snapshot() }

// FlushStats forces any pending stats to disk.
func (a *App) FlushStats() { a.ensure(); a.stats.Flush() }

// claim ensures only one goroutine processes a given path at a time.
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

// Start runs the event-driven daemon until ctx is cancelled. It performs an
// initial scan, watches all configured directories (recursively where asked),
// and hot-reloads config.yaml when it changes.
func (a *App) Start(ctx context.Context) error {
	a.ensure()
	a.stats.Load()

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer fsw.Close()

	a.log("Performing initial scan of watch directories...")
	a.ScanAll()

	a.addWatchDirs(fsw)
	a.watchConfigFile(fsw)

	// Periodic stats flush so bulk activity does not thrash the disk.
	flush := time.NewTicker(2 * time.Second)
	defer flush.Stop()

	a.log("InotiTidy (Event-Driven) started successfully")

	for {
		select {
		case <-ctx.Done():
			a.log("InotiTidy stopping...")
			a.stats.Flush()
			return nil
		case <-flush.C:
			a.stats.Flush()
		case event, ok := <-fsw.Events:
			if !ok {
				a.stats.Flush()
				return nil
			}
			a.onFSEvent(fsw, event)
		case err, ok := <-fsw.Errors:
			if !ok {
				return nil
			}
			a.log("Watcher error: %v", err)
		}
	}
}

func (a *App) addWatchDirs(fsw *fsnotify.Watcher) {
	for _, w := range a.Cfg().Watch {
		abs, _ := filepath.Abs(w.Path)
		if w.Recursive {
			a.addTree(fsw, abs)
		} else if err := fsw.Add(abs); err != nil {
			a.log("Error watching %s: %v", w.Path, err)
		} else {
			a.log("Watching: %s", w.Path)
		}
	}
}

func (a *App) addTree(fsw *fsnotify.Watcher, root string) {
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if aerr := fsw.Add(p); aerr != nil {
				a.log("Error watching %s: %v", p, aerr)
			}
		}
		return nil
	})
	if err != nil {
		a.log("Error walking %s: %v", root, err)
	} else {
		a.log("Watching (recursive): %s", root)
	}
}

func (a *App) watchConfigFile(fsw *fsnotify.Watcher) {
	// Watch the config directory; edits usually replace the file, so watching
	// the dir catches CREATE/RENAME as well as WRITE.
	_ = fsw.Add(config.GetConfigDir())
}

func (a *App) onFSEvent(fsw *fsnotify.Watcher, event fsnotify.Event) {
	// Config hot-reload.
	if filepath.Clean(event.Name) == filepath.Clean(config.GetConfigPath()) {
		if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
			a.reloadConfig(fsw)
		}
		return
	}

	if !event.Has(fsnotify.Create) && !event.Has(fsnotify.Rename) {
		return
	}

	// A newly created directory under a recursive root should be watched too.
	if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
		if a.isUnderRecursiveRoot(event.Name) {
			_ = fsw.Add(event.Name)
			go a.handleTree(event.Name)
		}
		return
	}

	go func(p string) {
		time.Sleep(100 * time.Millisecond)
		a.handleEvent(p)
	}(event.Name)
}

func (a *App) isUnderRecursiveRoot(path string) bool {
	for _, w := range a.Cfg().Watch {
		if !w.Recursive {
			continue
		}
		root, _ := filepath.Abs(w.Path)
		if abs, _ := filepath.Abs(path); abs == root || hasPrefixDir(abs, root) {
			return true
		}
	}
	return false
}

func hasPrefixDir(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !filepath.IsAbs(rel) && !startsWithDotDot(rel)
}

func startsWithDotDot(rel string) bool {
	return rel == ".." || (len(rel) >= 3 && rel[0] == '.' && rel[1] == '.' && rel[2] == filepath.Separator)
}

func (a *App) handleTree(root string) {
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			a.handleEvent(p)
		}
		return nil
	})
}

func (a *App) reloadConfig(fsw *fsnotify.Watcher) {
	cfg, err := config.Load()
	if err != nil {
		a.log("Config reload skipped: %v", err)
		return
	}
	a.SetConfig(cfg)
	a.addWatchDirs(fsw)
	a.log("Configuration reloaded")
	a.ScanAll()
}

// ScanAll sorts every file currently in the watch directories. It processes
// files concurrently but blocks until all are handled, then flushes stats.
func (a *App) ScanAll() {
	a.ensure()
	var wg sync.WaitGroup
	for _, w := range a.Cfg().Watch {
		a.scanDir(w, &wg)
	}
	wg.Wait()
	a.stats.Flush()
}

func (a *App) scanDir(w config.WatchDir, wg *sync.WaitGroup) {
	walkFn := func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if !w.Recursive && path != cleanPath(w.Path) {
				return filepath.SkipDir
			}
			return nil
		}
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			a.handleEvent(p)
		}(path)
		return nil
	}

	if w.Recursive {
		if err := filepath.WalkDir(w.Path, walkFn); err != nil {
			a.log("Error scanning %s: %v", w.Path, err)
		}
		return
	}

	entries, err := os.ReadDir(w.Path)
	if err != nil {
		a.log("Error scanning %s: %v", w.Path, err)
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(w.Path, entry.Name())
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			a.handleEvent(p)
		}(path)
	}
}

func cleanPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	return abs
}

// PreviewAll returns the plan for every file that would be acted upon, without
// changing anything. Used by dry-run mode in the CLI and TUI.
func (a *App) PreviewAll() []Plan {
	a.ensure()
	var plans []Plan
	for _, w := range a.Cfg().Watch {
		walkFn := func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if !w.Recursive && path != cleanPath(w.Path) {
					return filepath.SkipDir
				}
				return nil
			}
			if p := a.Evaluate(path, nil); p != nil {
				plans = append(plans, *p)
			}
			return nil
		}
		if w.Recursive {
			filepath.WalkDir(w.Path, walkFn)
		} else {
			entries, _ := os.ReadDir(w.Path)
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				path := filepath.Join(w.Path, e.Name())
				if p := a.Evaluate(path, nil); p != nil {
					plans = append(plans, *p)
				}
			}
		}
	}
	return plans
}

// handleEvent waits for a file to stop changing, then evaluates and applies the
// first matching rule.
func (a *App) handleEvent(path string) {
	if !a.claim(path) {
		return
	}
	defer a.release(path)

	info := a.settle(path)
	if info == nil {
		return
	}

	plan := a.Evaluate(path, info)
	if plan == nil {
		return
	}
	if plan.ruleIdx < 0 || plan.ruleIdx >= len(a.Cfg().Rules) {
		return
	}
	a.Apply(plan, &a.Cfg().Rules[plan.ruleIdx])
}

// settle blocks until the file size is stable, returning its final FileInfo, or
// nil if the file vanished or is not a regular file.
func (a *App) settle(path string) os.FileInfo {
	interval := a.Cfg().SettleInterval
	if interval <= 0 {
		interval = config.DefaultSettleInterval
	}
	var prevSize int64 = -1
	for {
		info, err := os.Stat(path)
		if err != nil {
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if info.Size() == prevSize {
			return info
		}
		prevSize = info.Size()
		time.Sleep(interval)
	}
}
