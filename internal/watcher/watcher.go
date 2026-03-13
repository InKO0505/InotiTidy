package watcher

import (
	"InotiTidy/internal/config"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type App struct {
	Config *config.Config
	Logger func(string) // Custom logger for TUI feedback
}

func (a *App) log(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	if a.Logger != nil {
		a.Logger(msg)
	} else {
		log.Println(msg)
	}
}

func (a *App) Start(ctx context.Context) error {
	seen := make(map[string]struct{})
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	a.log("InotiTidy started successfully")
	for {
		select {
		case <-ctx.Done():
			a.log("InotiTidy stopping...")
			return nil
		case <-ticker.C:
			for _, dir := range a.Config.WatchDirs {
				entries, err := os.ReadDir(dir)
				if err != nil {
					a.log("Error reading %s: %v", dir, err)
					continue
				}

				for _, entry := range entries {
					if entry.IsDir() {
						continue
					}
					path := filepath.Join(dir, entry.Name())
					if _, ok := seen[path]; ok {
						continue
					}
					seen[path] = struct{}{}
					
					// Run event handler in a goroutine to not block polling
					go a.handleEvent(path)
				}
			}
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
				a.move(path, rule.Target, fileName)
				return
			}
		}
	}
}

func (a *App) move(src, targetDir, name string) {
	_ = os.MkdirAll(targetDir, 0o755)
	dest := filepath.Join(targetDir, name)

	if _, err := os.Stat(dest); err == nil {
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		dest = filepath.Join(targetDir, fmt.Sprintf("%s_%d%s", base, time.Now().Unix(), ext))
	}

	if err := os.Rename(src, dest); err != nil {
		a.log("Move error: %v", err)
	} else {
		a.log("Sorted: %s", name)
	}
}
