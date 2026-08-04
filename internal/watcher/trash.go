package watcher

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// trashDir returns the XDG "home" trash directory, creating files/ and info/.
func trashDir() (files, info string, err error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", "", herr
		}
		base = filepath.Join(home, ".local", "share")
	}
	root := filepath.Join(base, "Trash")
	files = filepath.Join(root, "files")
	info = filepath.Join(root, "info")
	if err := os.MkdirAll(files, 0o700); err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(info, 0o700); err != nil {
		return "", "", err
	}
	return files, info, nil
}

// trash moves src into the freedesktop.org trash and records a .trashinfo file,
// so the file can be restored from the file manager or via undo.
func (a *App) trash(src string) error {
	filesDir, infoDir, err := trashDir()
	if err != nil {
		return err
	}

	name := filepath.Base(src)
	dest := uniqueDest(filesDir, name, filepath.Ext(name))
	infoPath := filepath.Join(infoDir, filepath.Base(dest)+".trashinfo")

	absSrc, _ := filepath.Abs(src)
	content := fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		url.PathEscape(absSrc), time.Now().Format("2006-01-02T15:04:05"))
	if err := os.WriteFile(infoPath, []byte(content), 0o600); err != nil {
		return err
	}

	if err := os.Rename(src, dest); err != nil {
		if cerr := moveFileWithCopyFallback(src, dest); cerr != nil {
			_ = os.Remove(infoPath)
			return cerr
		}
	}
	a.record(Record{Action: "trash", Src: absSrc, Dest: dest, TrashID: infoPath})
	a.log("Trashed: %s", name)
	a.stats.Increment(filepath.Ext(name))
	return nil
}
