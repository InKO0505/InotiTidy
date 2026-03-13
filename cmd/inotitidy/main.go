package main

import (
	"InotiTidy/internal/config"
	"InotiTidy/internal/watcher"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Critical: failed to load config: %v", err)
	}

	app := &watcher.App{Config: cfg}

	
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := app.Start(); err != nil {
			log.Fatalf("Watcher failed: %v", err)
		}
	}()

	<-sig
	log.Println("Gracefully shutting down InotiTidy...")
}