package watcher

import (
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
