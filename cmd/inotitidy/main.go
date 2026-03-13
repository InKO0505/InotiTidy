package main

import (
	"log"
)

func main() {
	// Everything is now unified into the TUI Management Console.
	// The daemon can be started, stopped and monitored directly from the UI.
	if err := handleTUI(); err != nil {
		log.Fatalf("Critical TUI error: %v", err)
	}
}