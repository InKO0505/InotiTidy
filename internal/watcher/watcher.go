package watcher

import (
	"InotiTidy/internal/config"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

type App struct {
	Config *config.Config
}

func (a *App) Start() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok { return }
				if event.Op&fsnotify.Create == fsnotify.Create {
					a.handleEvent(event.Name)
				}
			case err, ok := <-watcher.Errors:
				if !ok { return }
				log.Println("Watcher error:", err)
			}
		}
	}()

	for _, dir := range a.Config.WatchDirs {
		if err := watcher.Add(dir); err != nil {
			log.Printf("Error watching %s: %v", dir, err)
		}
	}

	log.Println("InotiTidy started successfully")
	select {} 
}

func (a *App) handleEvent(path string) {
	
	var prevSize int64 = -1
	for {
		stat, err := os.Stat(path)
		if err != nil { return }
		if stat.Size() == prevSize { break }
		prevSize = stat.Size()
		time.Sleep(500 * time.Millisecond)
	}

	fileName := filepath.Base(path)
	for _, key := range a.Config.Excludes {
		if strings.Contains(strings.ToUpper(fileName), strings.ToUpper(key)) {
			return
		}
	}

	ext := strings.ToLower(filepath.Ext(fileName))
	for _, rule := range a.Config.Rules {
		for _, e := range rule.Extensions {
			if e == ext {
				a.move(path, rule.Target, fileName)
				return
			}
		}
	}
}

func (a *App) move(src, targetDir, name string) {
	os.MkdirAll(targetDir, 0755)
	dest := filepath.Join(targetDir, name)

	if _, err := os.Stat(dest); err == nil {
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		dest = filepath.Join(targetDir, fmt.Sprintf("%s_%d%s", base, time.Now().Unix(), ext))
	}

	if err := os.Rename(src, dest); err != nil {
		log.Printf("Move error: %v", err)
	} else {
		log.Printf("Sorted: %s", name)
	}
}