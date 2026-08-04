package watcher

import (
	"InotiTidy/internal/config"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestMoveFileWithCopyFallback(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	content := []byte("hello world")

	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	if err := moveFileWithCopyFallback(src, dst); err != nil {
		t.Fatalf("moveFileWithCopyFallback: %v", err)
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("source should be removed, stat err=%v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("content mismatch: got %q want %q", string(got), string(content))
	}
}

func TestUniqueDestAvoidsOverwrite(t *testing.T) {
	dir := t.TempDir()

	// First destination is free.
	if got := uniqueDest(dir, "report.pdf", ".pdf"); got != filepath.Join(dir, "report.pdf") {
		t.Fatalf("expected plain name, got %q", got)
	}

	// Occupy report.pdf and report_1.pdf; next should be report_2.pdf.
	for _, name := range []string{"report.pdf", "report_1.pdf"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}
	want := filepath.Join(dir, "report_2.pdf")
	if got := uniqueDest(dir, "report.pdf", ".pdf"); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestMoveNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out")
	app := &App{Config: &config.Config{}, Stats: &Stats{ExtensionCounts: map[string]int{}}}

	// Move two distinct files that share the same name into the same target.
	for i, body := range []string{"first", "second"} {
		src := filepath.Join(dir, fmt.Sprintf("src%d", i), "data.txt")
		if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		app.move(src, target, "data.txt", ".txt")
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 files preserved, got %d", len(entries))
	}
}

func TestMatchesExtension(t *testing.T) {
	cases := []struct {
		name string
		ext  string
		want bool
	}{
		{"report.pdf", ".pdf", true},
		{"report.pdf", "pdf", true},  // missing leading dot is normalized
		{"report.pdf", ".PDF", true}, // rule case-insensitive
		{"report.pdf", "df", false},  // must match on a dot boundary
		{"archive.tar.gz", ".gz", true},
		{"archive.tar.gz", ".tar.gz", true}, // compound extension
		{"noext", ".pdf", false},
		{"anything", "", false}, // empty ext never matches
		{"anything", ".", false},
	}
	for _, c := range cases {
		if got := matchesExtension(c.name, c.ext); got != c.want {
			t.Errorf("matchesExtension(%q, %q) = %v, want %v", c.name, c.ext, got, c.want)
		}
	}
}

func TestHandleEventIgnoresEmptyExclude(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "sorted")
	src := filepath.Join(dir, "watch", "file.txt")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(src, []byte("data"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	app := &App{
		Config: &config.Config{
			Excludes: []string{""}, // empty keyword must not block sorting
			Rules:    []config.Rule{{Extensions: []string{".txt"}, Target: target}},
		},
		Stats: &Stats{ExtensionCounts: map[string]int{}},
	}
	app.handleEvent(src)

	if _, err := os.Stat(filepath.Join(target, "file.txt")); err != nil {
		t.Fatalf("file should have been sorted despite empty exclude keyword: %v", err)
	}
}

func TestClaimDeduplicates(t *testing.T) {
	app := &App{}
	if !app.claim("/tmp/foo") {
		t.Fatal("first claim should succeed")
	}
	if app.claim("/tmp/foo") {
		t.Fatal("second concurrent claim should fail")
	}
	app.release("/tmp/foo")
	if !app.claim("/tmp/foo") {
		t.Fatal("claim after release should succeed")
	}
}
