package main

import (
	"InotiTidy/internal/config"
	"InotiTidy/internal/service"
	"InotiTidy/internal/watcher"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

// version is set via -ldflags at release time.
var version = "dev"

func main() {
	var (
		daemon      = flag.Bool("daemon", false, "Run the background daemon (used by systemd)")
		scan        = flag.Bool("scan", false, "Sort all existing files once, then exit")
		dryRun      = flag.Bool("dry-run", false, "Show what would be sorted without moving anything")
		undo        = flag.Bool("undo", false, "Undo the most recent batch of moves")
		install     = flag.Bool("install", false, "Install and enable the systemd --user service")
		showVersion = flag.Bool("version", false, "Print version and exit")
	)
	flag.Parse()

	switch {
	case *showVersion:
		fmt.Println("inotitidy", version)
	case *install:
		mustInstall()
	case *undo:
		runUndo()
	case *daemon:
		runDaemon()
	case *scan || *dryRun:
		runScan(*dryRun)
	default:
		if err := handleTUI(); err != nil {
			fmt.Fprintln(os.Stderr, "TUI error:", err)
			os.Exit(1)
		}
	}
}

func loadOrExit() *config.Config {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		fmt.Fprintf(os.Stderr, "expected config at %s\n", config.GetConfigPath())
		os.Exit(1)
	}
	return cfg
}

func runDaemon() {
	cfg := loadOrExit()
	app := watcher.New(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.Start(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "daemon error:", err)
		os.Exit(1)
	}
}

func runScan(dry bool) {
	cfg := loadOrExit()
	app := watcher.New(cfg)
	app.DryRun = dry
	app.LoadStats()

	if dry {
		plans := app.PreviewAll()
		if len(plans) == 0 {
			fmt.Println("Nothing to sort.")
			return
		}
		fmt.Printf("Would sort %d file(s):\n", len(plans))
		for _, p := range plans {
			fmt.Printf("  %-8s %s -> %s\n", p.Action, filepath.Base(p.Src), p.Dest)
		}
		return
	}

	app.ScanAll()
	s := app.Snapshot()
	fmt.Printf("Done. Total sorted: %d (today: %d)\n", s.TotalSorted, s.TodaySorted)
}

func runUndo() {
	cfg := loadOrExit()
	app := watcher.New(cfg)
	n, err := app.UndoLastBatch()
	if err != nil {
		fmt.Fprintln(os.Stderr, "undo error:", err)
		os.Exit(1)
	}
	fmt.Printf("Undid %d operation(s).\n", n)
}

func mustInstall() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot locate executable:", err)
		os.Exit(1)
	}
	exe, _ = filepath.Abs(exe)

	if err := service.Install(exe, config.GetConfigDir()); err != nil {
		fmt.Fprintln(os.Stderr, "install failed:", err)
		os.Exit(1)
	}
	if err := service.Enable(); err != nil {
		fmt.Fprintln(os.Stderr, "enable failed:", err)
		os.Exit(1)
	}
	fmt.Printf("Installed and started %s (systemd --user).\n", service.Unit)
	fmt.Println("Follow logs with: journalctl --user -u", service.Unit, "-f")
}
