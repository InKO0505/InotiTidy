package watcher

import (
	"InotiTidy/internal/config"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Plan describes what would happen to a file, without performing it.
type Plan struct {
	Src    string
	Rule   string
	Action string
	Dest   string // resolved destination for move/copy; trash dir for trash

	ruleIdx int // index into Config.Rules, for exact lookup on apply
}

// Evaluate returns the plan for the first matching rule, or nil if the file is
// excluded or matches nothing. info may be nil (it will be stat-ed).
func (a *App) Evaluate(path string, info os.FileInfo) *Plan {
	if info == nil {
		var err error
		info, err = os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
	}
	base := filepath.Base(path)

	// Global excludes skip the file entirely.
	for _, kw := range a.Cfg().GlobalExclude {
		if kw != "" && strings.Contains(strings.ToUpper(base), strings.ToUpper(kw)) {
			return nil
		}
	}

	for i := range a.Cfg().Rules {
		rule := &a.Cfg().Rules[i]
		if a.ruleMatches(rule, base, path, info) {
			label := rule.Name
			if label == "" {
				label = strings.Join(rule.Extensions, ",")
			}
			plan := &Plan{Src: path, Rule: label, Action: rule.Action, ruleIdx: i}
			if rule.Action != config.ActionTrash {
				plan.Dest = filepath.Join(resolveTarget(rule.Target, base, info), base)
			} else {
				plan.Dest = "(trash)"
			}
			return plan
		}
	}
	return nil
}

func (a *App) ruleMatches(rule *config.Rule, base, path string, info os.FileInfo) bool {
	// Per-rule excludes.
	for _, kw := range rule.Exclude {
		if kw != "" && strings.Contains(strings.ToUpper(base), strings.ToUpper(kw)) {
			return false
		}
	}

	if len(rule.Extensions) > 0 {
		if !a.extensionMatches(rule, base, path) {
			return false
		}
	}
	if rule.NameGlob != "" {
		if ok, _ := filepath.Match(strings.ToLower(rule.NameGlob), strings.ToLower(base)); !ok {
			return false
		}
	}
	if re := rule.Regex(); re != nil && !re.MatchString(base) {
		return false
	}
	if rule.MinSize > 0 && info.Size() < rule.MinSize {
		return false
	}
	if rule.MaxSize > 0 && info.Size() > rule.MaxSize {
		return false
	}
	age := time.Since(info.ModTime())
	if rule.OlderThan > 0 && age < rule.OlderThan {
		return false
	}
	if rule.NewerThan > 0 && age > rule.NewerThan {
		return false
	}
	return true
}

func (a *App) extensionMatches(rule *config.Rule, base, path string) bool {
	if rule.ByContent {
		for _, ext := range detectExtensions(path) {
			if slices.Contains(rule.Extensions, ext) {
				return true
			}
		}
		return false
	}
	lower := strings.ToLower(base)
	for _, e := range rule.Extensions {
		if strings.HasSuffix(lower, e) {
			return true
		}
	}
	return false
}

// detectExtensions sniffs the file content and returns candidate extensions
// (e.g. [".png"]) derived from the detected MIME type.
func detectExtensions(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	ct := http.DetectContentType(buf[:n])
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	exts, _ := mime.ExtensionsByType(ct)
	for i := range exts {
		exts[i] = strings.ToLower(exts[i])
	}
	return exts
}

// resolveTarget expands {year} {month} {day} {ext} templates in a target dir.
func resolveTarget(target, base string, info os.FileInfo) string {
	if !strings.Contains(target, "{") {
		return target
	}
	t := time.Now()
	if info != nil {
		t = info.ModTime()
	}
	ext := strings.TrimPrefix(filepath.Ext(base), ".")
	r := strings.NewReplacer(
		"{year}", t.Format("2006"),
		"{month}", t.Format("01"),
		"{day}", t.Format("02"),
		"{ext}", ext,
	)
	return r.Replace(target)
}

// Apply executes a plan and journals it for undo. In dry-run mode it only logs.
func (a *App) Apply(plan *Plan, rule *config.Rule) {
	if a.DryRun {
		a.log("[dry-run] %s -> %s (%s)", filepath.Base(plan.Src), plan.Dest, plan.Action)
		return
	}

	switch plan.Action {
	case config.ActionTrash:
		if err := a.trash(plan.Src); err != nil {
			a.log("Trash error: %v", err)
		}
	case config.ActionCopy:
		a.place(plan.Src, plan.Dest, rule.OnConflict, config.ActionCopy)
	default: // move
		a.place(plan.Src, plan.Dest, rule.OnConflict, config.ActionMove)
	}
}

// place moves or copies src to dest applying the conflict strategy.
func (a *App) place(src, dest, onConflict, action string) {
	targetDir := filepath.Dir(dest)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		a.log("Cannot create %s: %v", targetDir, err)
		return
	}
	name := filepath.Base(dest)
	ext := filepath.Ext(name)

	if _, err := os.Stat(dest); err == nil {
		switch onConflict {
		case config.ConflictSkip:
			a.log("Skipped (exists): %s", name)
			return
		case config.ConflictOverwrite:
			_ = os.Remove(dest)
		case config.ConflictTrash:
			_ = a.trash(dest)
		default: // rename
			dest = uniqueDest(targetDir, name, ext)
		}
	}

	if action == config.ActionCopy {
		if err := copyFile(src, dest); err != nil {
			a.log("Copy error: %v", err)
			return
		}
		a.record(Record{Action: config.ActionCopy, Src: src, Dest: dest})
		a.log("Copied: %s", filepath.Base(dest))
		a.stats.Increment(ext)
		return
	}

	if err := os.Rename(src, dest); err != nil {
		if cerr := moveFileWithCopyFallback(src, dest); cerr != nil {
			a.log("Move error: %v", cerr)
			return
		}
	}
	a.record(Record{Action: config.ActionMove, Src: src, Dest: dest})
	a.log("Sorted: %s", filepath.Base(dest))
	a.stats.Increment(ext)
}

func (a *App) record(r Record) {
	r.Time = time.Now()
	r.Batch = a.batchID
	a.history.append(r)
}

// uniqueDest returns a path in targetDir that does not yet exist, appending
// _1, _2, … before the extension so files are never silently overwritten.
func uniqueDest(targetDir, name, ext string) string {
	dest := filepath.Join(targetDir, name)
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		return dest
	}
	base := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		candidate := filepath.Join(targetDir, fmt.Sprintf("%s_%d%s", base, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dest)
		return err
	}
	return out.Close()
}

func moveFileWithCopyFallback(src, dest string) error {
	if err := copyFile(src, dest); err != nil {
		return err
	}
	return os.Remove(src)
}
