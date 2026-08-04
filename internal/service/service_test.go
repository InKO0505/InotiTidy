package service

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestUnitPathHonorsXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	want := filepath.Join("/tmp/xdg", "systemd", "user", Unit)
	if got := UnitPath(); got != want {
		t.Fatalf("UnitPath = %q, want %q", got, want)
	}
}

func TestUnitContent(t *testing.T) {
	c := unitContent("/usr/bin/inotitidy", "/home/u/.config/inotitidy")
	for _, must := range []string{
		"ExecStart=/usr/bin/inotitidy --daemon",
		"WorkingDirectory=/home/u/.config/inotitidy",
		"WantedBy=default.target", // user service target
		"Restart=always",
	} {
		if !strings.Contains(c, must) {
			t.Errorf("unit missing %q in:\n%s", must, c)
		}
	}
}

func TestJournalArgsUser(t *testing.T) {
	args := JournalArgs()
	if args[0] != "--user" {
		t.Fatalf("journal must target the user bus, got %v", args)
	}
}
