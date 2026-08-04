package watcher

import (
	"InotiTidy/internal/config"
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// Record is one reversible operation appended to the history journal.
type Record struct {
	Time    time.Time `json:"time"`
	Batch   string    `json:"batch"`
	Action  string    `json:"action"` // move | copy | trash
	Src     string    `json:"src"`    // original location
	Dest    string    `json:"dest"`   // where the file ended up
	TrashID string    `json:"trash_info,omitempty"`
}

// history is an append-only journal of executed operations used to undo them.
type history struct {
	mu   sync.Mutex
	path string
}

func newHistory() *history { return &history{path: config.GetHistoryPath()} }

func (h *history) append(r Record) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(h.path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(h.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	if data, err := json.Marshal(r); err == nil {
		f.Write(append(data, '\n'))
	}
}

func (h *history) readAll() ([]Record, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	f, err := os.Open(h.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var recs []Record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var r Record
		if json.Unmarshal(line, &r) == nil {
			recs = append(recs, r)
		}
	}
	return recs, sc.Err()
}

func (h *history) rewrite(recs []Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(recs) == 0 {
		return os.Remove(h.path)
	}
	tmp := h.path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	for _, r := range recs {
		data, _ := json.Marshal(r)
		w.Write(append(data, '\n'))
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, h.path)
}

// UndoLastBatch reverses every operation from the most recent batch and returns
// how many operations were undone.
func (a *App) UndoLastBatch() (int, error) {
	recs, err := a.history.readAll()
	if err != nil || len(recs) == 0 {
		return 0, err
	}

	lastBatch := recs[len(recs)-1].Batch
	var remaining []Record
	var toUndo []Record
	for _, r := range recs {
		if r.Batch == lastBatch {
			toUndo = append(toUndo, r)
		} else {
			remaining = append(remaining, r)
		}
	}

	undone := 0
	// Reverse in LIFO order so later operations are undone first.
	for _, rec := range slices.Backward(toUndo) {
		if err := a.reverse(rec); err != nil {
			a.log("Undo failed for %s: %v", rec.Src, err)
			remaining = append(remaining, rec) // keep unrecoverable entries
			continue
		}
		undone++
	}
	if err := a.history.rewrite(remaining); err != nil {
		return undone, err
	}
	return undone, nil
}

// reverse undoes a single recorded operation.
func (a *App) reverse(r Record) error {
	switch r.Action {
	case config.ActionCopy:
		// A copy left the source in place; just remove the copy.
		return os.Remove(r.Dest)
	case config.ActionMove, config.ActionTrash:
		if err := os.MkdirAll(filepath.Dir(r.Src), 0o755); err != nil {
			return err
		}
		dest := r.Src
		if _, err := os.Stat(dest); err == nil {
			dest = uniqueDest(filepath.Dir(r.Src), filepath.Base(r.Src), filepath.Ext(r.Src))
		}
		if err := os.Rename(r.Dest, dest); err != nil {
			if cerr := moveFileWithCopyFallback(r.Dest, dest); cerr != nil {
				return cerr
			}
		}
		if r.TrashID != "" {
			_ = os.Remove(r.TrashID)
		}
		a.log("Restored: %s", filepath.Base(dest))
		return nil
	}
	return nil
}
