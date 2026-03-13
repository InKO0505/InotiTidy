package main

import (
	"InotiTidy/internal/config"
	"InotiTidy/internal/watcher"
	"context"
	"flag"
	"log"
)

func main() {
	daemon := flag.Bool("daemon", false, "Run as a background daemon")
	flag.Parse()

	if *daemon {
		cfg, err := config.Load()
		if err != nil {
			log.Fatalf("Failed to load config for daemon: %v", err)
		}
		
		w := &watcher.App{Config: cfg}
		log.Println("InotiTidy starting in daemon mode...")
		if err := w.Start(context.Background()); err != nil {
			log.Fatalf("Daemon error: %v", err)
		}
		return
	}

	// Default: Launch the TUI Management Console.
	if err := handleTUI(); err != nil {
		log.Fatalf("Critical TUI error: %v", err)
	}
}