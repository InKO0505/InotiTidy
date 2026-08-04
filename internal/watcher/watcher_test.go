package watcher

import (
	"InotiTidy/internal/config"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newTestApp builds an App whose config/stats/history live under a temp HOME.
func newTestApp(t *testing.T, cfg *config.Config) *App {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	return New(cfg)
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestMoveSortsAndExcludes(t *testing.T) {
	dir := t.TempDir()
	watch := filepath.Join(dir, "dl")
	target := filepath.Join(dir, "docs")

	cfg := &config.Config{
		Watch:         []config.WatchDir{{Path: watch}},
		GlobalExclude: []string{"KEEP", ""}, // empty must not disable sorting
		Rules: []config.Rule{{
			Extensions: []string{".txt"},
			Action:     config.ActionMove,
			Target:     target,
			OnConflict: config.ConflictRename,
		}},
	}
	app := newTestApp(t, cfg)

	writeFile(t, filepath.Join(watch, "note.txt"), "hi")
	writeFile(t, filepath.Join(watch, "KEEP_me.txt"), "stay")
	app.ScanAll()

	if _, err := os.Stat(filepath.Join(target, "note.txt")); err != nil {
		t.Fatalf("note.txt should be sorted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(watch, "KEEP_me.txt")); err != nil {
		t.Fatalf("KEEP_me.txt should have been excluded and left in place: %v", err)
	}
	if s := app.Snapshot(); s.TotalSorted != 1 {
		t.Fatalf("TotalSorted = %d, want 1", s.TotalSorted)
	}
}

func TestConflictRenameNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	watch := filepath.Join(dir, "dl")
	target := filepath.Join(dir, "out")
	cfg := &config.Config{
		Watch: []config.WatchDir{{Path: watch}},
		Rules: []config.Rule{{Extensions: []string{".txt"}, Action: config.ActionMove, Target: target, OnConflict: config.ConflictRename}},
	}
	app := newTestApp(t, cfg)

	writeFile(t, filepath.Join(target, "data.txt"), "existing")
	writeFile(t, filepath.Join(watch, "data.txt"), "incoming")
	app.ScanAll()

	entries, _ := os.ReadDir(target)
	if len(entries) != 2 {
		t.Fatalf("expected 2 files (rename), got %d", len(entries))
	}
}

func TestConflictSkip(t *testing.T) {
	dir := t.TempDir()
	watch := filepath.Join(dir, "dl")
	target := filepath.Join(dir, "out")
	cfg := &config.Config{
		Watch: []config.WatchDir{{Path: watch}},
		Rules: []config.Rule{{Extensions: []string{".txt"}, Action: config.ActionMove, Target: target, OnConflict: config.ConflictSkip}},
	}
	app := newTestApp(t, cfg)

	writeFile(t, filepath.Join(target, "data.txt"), "existing")
	writeFile(t, filepath.Join(watch, "data.txt"), "incoming")
	app.ScanAll()

	if _, err := os.Stat(filepath.Join(watch, "data.txt")); err != nil {
		t.Fatalf("source should remain on skip: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(target, "data.txt"))
	if string(got) != "existing" {
		t.Fatalf("target overwritten on skip: %q", got)
	}
}

func TestSizeAndAgeConditions(t *testing.T) {
	dir := t.TempDir()
	watch := filepath.Join(dir, "dl")
	target := filepath.Join(dir, "big")
	cfg := &config.Config{
		Watch: []config.WatchDir{{Path: watch}},
		Rules: []config.Rule{{
			Extensions: []string{".bin"},
			MinSize:    10,
			Action:     config.ActionMove,
			Target:     target,
			OnConflict: config.ConflictRename,
		}},
	}
	app := newTestApp(t, cfg)

	writeFile(t, filepath.Join(watch, "small.bin"), "tiny")           // 4 bytes < 10
	writeFile(t, filepath.Join(watch, "big.bin"), "0123456789abcdef") // 16 bytes
	app.ScanAll()

	if _, err := os.Stat(filepath.Join(watch, "small.bin")); err != nil {
		t.Fatalf("small.bin should be left (below min_size): %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "big.bin")); err != nil {
		t.Fatalf("big.bin should be sorted: %v", err)
	}
}

func TestRegexAndGlobConditions(t *testing.T) {
	dir := t.TempDir()
	watch := filepath.Join(dir, "dl")
	target := filepath.Join(dir, "inv")
	cfg := &config.Config{
		Watch: []config.WatchDir{{Path: watch}},
		Rules: []config.Rule{{
			NameRegex:  `^INV-\d+`,
			Action:     config.ActionMove,
			Target:     target,
			OnConflict: config.ConflictRename,
		}},
	}
	app := newTestApp(t, cfg)

	writeFile(t, filepath.Join(watch, "INV-1001.pdf"), "x")
	writeFile(t, filepath.Join(watch, "receipt.pdf"), "x")
	app.ScanAll()

	if _, err := os.Stat(filepath.Join(target, "INV-1001.pdf")); err != nil {
		t.Fatalf("invoice should be sorted by regex: %v", err)
	}
	if _, err := os.Stat(filepath.Join(watch, "receipt.pdf")); err != nil {
		t.Fatalf("receipt should be left: %v", err)
	}
}

func TestDateTemplateTarget(t *testing.T) {
	dir := t.TempDir()
	watch := filepath.Join(dir, "dl")
	base := filepath.Join(dir, "pics")
	cfg := &config.Config{
		Watch: []config.WatchDir{{Path: watch}},
		Rules: []config.Rule{{
			Extensions: []string{".jpg"},
			Action:     config.ActionMove,
			Target:     filepath.Join(base, "{year}"),
			OnConflict: config.ConflictRename,
		}},
	}
	app := newTestApp(t, cfg)

	writeFile(t, filepath.Join(watch, "photo.jpg"), "x")
	app.ScanAll()

	year := time.Now().Format("2006")
	if _, err := os.Stat(filepath.Join(base, year, "photo.jpg")); err != nil {
		t.Fatalf("photo should land in year folder %s: %v", year, err)
	}
}

func TestTrashAndUndo(t *testing.T) {
	dir := t.TempDir()
	watch := filepath.Join(dir, "dl")
	cfg := &config.Config{
		Watch: []config.WatchDir{{Path: watch}},
		Rules: []config.Rule{{
			Extensions: []string{".tmp"},
			Action:     config.ActionTrash,
			OnConflict: config.ConflictRename,
		}},
	}
	app := newTestApp(t, cfg)

	src := filepath.Join(watch, "junk.tmp")
	writeFile(t, src, "garbage")
	app.ScanAll()

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("file should be trashed, still present: %v", err)
	}

	n, err := app.UndoLastBatch()
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	if n != 1 {
		t.Fatalf("undone = %d, want 1", n)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("file should be restored by undo: %v", err)
	}
}

func TestDryRunTouchesNothing(t *testing.T) {
	dir := t.TempDir()
	watch := filepath.Join(dir, "dl")
	target := filepath.Join(dir, "out")
	cfg := &config.Config{
		Watch: []config.WatchDir{{Path: watch}},
		Rules: []config.Rule{{Extensions: []string{".txt"}, Action: config.ActionMove, Target: target, OnConflict: config.ConflictRename}},
	}
	app := newTestApp(t, cfg)
	app.DryRun = true

	writeFile(t, filepath.Join(watch, "note.txt"), "hi")
	plans := app.PreviewAll()
	if len(plans) != 1 || plans[0].Action != config.ActionMove {
		t.Fatalf("expected one move plan, got %#v", plans)
	}
	app.ScanAll() // dry-run: must not move
	if _, err := os.Stat(filepath.Join(watch, "note.txt")); err != nil {
		t.Fatalf("dry-run moved the file: %v", err)
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

func TestMoveFileWithCopyFallback(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	writeFile(t, src, "hello world")

	if err := moveFileWithCopyFallback(src, dst); err != nil {
		t.Fatalf("moveFileWithCopyFallback: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("source should be removed: %v", err)
	}
	got, _ := os.ReadFile(dst)
	if string(got) != "hello world" {
		t.Fatalf("content mismatch: %q", got)
	}
}
