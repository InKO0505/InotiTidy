// Package service manages InotiTidy's systemd *user* service. Running under
// `systemctl --user` means no root, sudo or pkexec is ever required: the daemon
// runs as the user who owns the files it sorts.
package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Unit is the systemd unit name.
const Unit = "inotitidy.service"

// UnitPath returns ~/.config/systemd/user/inotitidy.service.
func UnitPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "systemd", "user", Unit)
}

func unitContent(execPath, workingDir string) string {
	return fmt.Sprintf(`[Unit]
Description=InotiTidy File Organizer
After=default.target

[Service]
Type=simple
ExecStart=%s --daemon
WorkingDirectory=%s
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
`, execPath, workingDir)
}

// Install writes the unit file, reloads the user daemon and enables lingering so
// the service keeps running without an active login session. It does not start
// the service; call Enable or Start for that.
func Install(execPath, workingDir string) error {
	path := UnitPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create unit dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(unitContent(execPath, workingDir)), 0o644); err != nil {
		return fmt.Errorf("write unit: %w", err)
	}
	if err := systemctl("daemon-reload"); err != nil {
		return fmt.Errorf("daemon-reload: %w", err)
	}
	// Best effort: allow the service to run without an active session.
	_ = enableLinger()
	return nil
}

// Enable enables and starts the service on boot/login.
func Enable() error { return systemctl("enable", "--now", Unit) }

// Start, Stop and Restart control the running service.
func Start() error   { return systemctl("start", Unit) }
func Stop() error    { return systemctl("stop", Unit) }
func Restart() error { return systemctl("restart", Unit) }

// IsActive reports whether the service is currently running.
func IsActive() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "systemctl", "--user", "is-active", "--quiet", Unit)
	return cmd.Run() == nil
}

// Installed reports whether the unit file exists.
func Installed() bool {
	_, err := os.Stat(UnitPath())
	return err == nil
}

func systemctl(args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	full := append([]string{"--user"}, args...)
	cmd := exec.CommandContext(ctx, "systemctl", full...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return err
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func enableLinger() error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	user := os.Getenv("USER")
	if user == "" {
		return nil
	}
	return exec.CommandContext(ctx, "loginctl", "enable-linger", user).Run()
}

// JournalArgs returns the journalctl args to follow the service logs (user bus).
func JournalArgs() []string {
	return []string{"--user", "-u", Unit, "-f", "-n", "50", "--no-hostname"}
}
