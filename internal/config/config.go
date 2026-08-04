// Package config defines InotiTidy's configuration model and (de)serialization.
//
// The v2 model is a small rules engine: each rule combines a set of conditions
// (extensions, glob, regex, size, age, MIME) with a single action (move, copy or
// trash) and a conflict strategy. The legacy flat format
// (watch_directories / exclude_keywords / rules:[{extensions,target}]) is still
// accepted and transparently upgraded, so existing configs keep working.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Action kinds.
const (
	ActionMove  = "move"
	ActionCopy  = "copy"
	ActionTrash = "trash"
)

// Conflict strategies applied when the destination already exists.
const (
	ConflictRename    = "rename"
	ConflictSkip      = "skip"
	ConflictOverwrite = "overwrite"
	ConflictTrash     = "trash"
)

// DefaultSettleInterval is how long a file must stop changing size before it is
// considered ready to sort.
const DefaultSettleInterval = 500 * time.Millisecond

// WatchDir is a directory InotiTidy monitors.
type WatchDir struct {
	Path      string
	Recursive bool
}

// Rule pairs match conditions with a single action. A file matches a rule only
// when every condition that is set passes; unset conditions are ignored.
type Rule struct {
	Name string

	// Conditions.
	Extensions []string // normalized to ".ext", lowercase
	NameGlob   string   // shell glob against the base name
	NameRegex  string   // regexp against the base name
	MinSize    int64    // bytes, 0 = unset
	MaxSize    int64    // bytes, 0 = unset
	OlderThan  time.Duration
	NewerThan  time.Duration
	ByContent  bool     // match Extensions against the detected MIME, not the name
	Exclude    []string // per-rule substring excludes

	// Action.
	Action     string // move | copy | trash
	Target     string // destination dir; supports {year} {month} {day} {ext} templates
	OnConflict string // rename | skip | overwrite | trash

	regex *regexp.Regexp // compiled NameRegex cache
}

// Regex returns the compiled NameRegex (nil if unset). Callers that read this
// concurrently must call Config.Compile first; see the note there.
func (r *Rule) Regex() *regexp.Regexp {
	if r.NameRegex == "" {
		return nil
	}
	if r.regex == nil {
		r.regex, _ = regexp.Compile(r.NameRegex)
	}
	return r.regex
}

// Compile eagerly compiles every rule's NameRegex on the calling goroutine.
// Rules are matched concurrently, so this must run once, single-threaded,
// before any concurrent Regex() reads to avoid a data race on the cache.
func (c *Config) Compile() {
	for i := range c.Rules {
		c.Rules[i].Regex()
	}
}

// Config is the fully resolved configuration (paths expanded to absolute).
type Config struct {
	Watch          []WatchDir
	GlobalExclude  []string
	SettleInterval time.Duration
	Rules          []Rule
}

// Paths returns just the watch-directory paths, for convenience.
func (c *Config) Paths() []string {
	out := make([]string, 0, len(c.Watch))
	for _, w := range c.Watch {
		out = append(out, w.Path)
	}
	return out
}

// GetConfigDir returns ~/.config/inotitidy, honoring XDG_CONFIG_HOME.
func GetConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "inotitidy")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "inotitidy")
}

// GetConfigPath returns the full path to config.yaml.
func GetConfigPath() string { return filepath.Join(GetConfigDir(), "config.yaml") }

// GetStatsPath returns the full path to the persisted stats file.
func GetStatsPath() string { return filepath.Join(GetConfigDir(), "stats.json") }

// GetHistoryPath returns the append-only move journal used for undo.
func GetHistoryPath() string { return filepath.Join(GetConfigDir(), "history.jsonl") }

// Load reads and resolves the config from the default path.
func Load() (*Config, error) { return LoadFromPath(GetConfigPath()) }

// LoadFromPath reads, upgrades, resolves and validates a config file.
func LoadFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg, err := parse(data)
	if err != nil {
		return nil, err
	}
	cfg.expand()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	cfg.Compile()
	return cfg, nil
}

// ---- serialization ----------------------------------------------------------

type rawWatch struct {
	Path      string `yaml:"path"`
	Recursive bool   `yaml:"recursive,omitempty"`
}

type rawRule struct {
	Name       string   `yaml:"name,omitempty"`
	Extensions []string `yaml:"extensions,omitempty"`
	NameGlob   string   `yaml:"name_glob,omitempty"`
	NameRegex  string   `yaml:"name_regex,omitempty"`
	MinSize    Size     `yaml:"min_size,omitempty"`
	MaxSize    Size     `yaml:"max_size,omitempty"`
	OlderThan  Duration `yaml:"older_than,omitempty"`
	NewerThan  Duration `yaml:"newer_than,omitempty"`
	ByContent  bool     `yaml:"by_content,omitempty"`
	Exclude    []string `yaml:"exclude,omitempty"`
	Action     string   `yaml:"action,omitempty"`
	Target     string   `yaml:"target,omitempty"`
	OnConflict string   `yaml:"on_conflict,omitempty"`
}

type rawConfig struct {
	Version        int        `yaml:"version,omitempty"`
	Watch          []rawWatch `yaml:"watch,omitempty"`
	GlobalExclude  []string   `yaml:"global_exclude,omitempty"`
	SettleInterval Duration   `yaml:"settle_interval,omitempty"`
	Rules          []rawRule  `yaml:"rules,omitempty"`

	// Legacy keys (v1).
	WatchDirectories []string `yaml:"watch_directories,omitempty"`
	ExcludeKeywords  []string `yaml:"exclude_keywords,omitempty"`
}

func parse(data []byte) (*Config, error) {
	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg := &Config{
		GlobalExclude:  raw.GlobalExclude,
		SettleInterval: raw.SettleInterval.D,
	}
	if cfg.SettleInterval == 0 {
		cfg.SettleInterval = DefaultSettleInterval
	}

	// Watch dirs: new format, else upgrade legacy watch_directories.
	for _, w := range raw.Watch {
		if w.Path != "" {
			cfg.Watch = append(cfg.Watch, WatchDir{Path: w.Path, Recursive: w.Recursive})
		}
	}
	for _, p := range raw.WatchDirectories {
		if p != "" {
			cfg.Watch = append(cfg.Watch, WatchDir{Path: p})
		}
	}

	// Legacy exclude_keywords fold into global excludes.
	cfg.GlobalExclude = append(cfg.GlobalExclude, raw.ExcludeKeywords...)

	for _, rr := range raw.Rules {
		rule := Rule{
			Name:       rr.Name,
			Extensions: normalizeExtensions(rr.Extensions),
			NameGlob:   rr.NameGlob,
			NameRegex:  rr.NameRegex,
			MinSize:    int64(rr.MinSize),
			MaxSize:    int64(rr.MaxSize),
			OlderThan:  rr.OlderThan.D,
			NewerThan:  rr.NewerThan.D,
			ByContent:  rr.ByContent,
			Exclude:    rr.Exclude,
			Action:     strings.ToLower(strings.TrimSpace(rr.Action)),
			Target:     rr.Target,
			OnConflict: strings.ToLower(strings.TrimSpace(rr.OnConflict)),
		}
		if rule.Action == "" {
			rule.Action = ActionMove
		}
		if rule.OnConflict == "" {
			rule.OnConflict = ConflictRename
		}
		cfg.Rules = append(cfg.Rules, rule)
	}

	return cfg, nil
}

// Save writes the config in the clean v2 format with home-relative paths.
func (c *Config) Save(path string) error {
	home, _ := os.UserHomeDir()
	collapse := func(p string) string {
		if home != "" && strings.HasPrefix(p, home) {
			return "~" + strings.TrimPrefix(p, home)
		}
		return p
	}

	raw := rawConfig{Version: 2}
	if c.SettleInterval != 0 && c.SettleInterval != DefaultSettleInterval {
		raw.SettleInterval = Duration{c.SettleInterval}
	}
	for _, w := range c.Watch {
		raw.Watch = append(raw.Watch, rawWatch{Path: collapse(w.Path), Recursive: w.Recursive})
	}
	raw.GlobalExclude = c.GlobalExclude
	for _, r := range c.Rules {
		raw.Rules = append(raw.Rules, rawRule{
			Name:       r.Name,
			Extensions: r.Extensions,
			NameGlob:   r.NameGlob,
			NameRegex:  r.NameRegex,
			MinSize:    Size(r.MinSize),
			MaxSize:    Size(r.MaxSize),
			OlderThan:  Duration{r.OlderThan},
			NewerThan:  Duration{r.NewerThan},
			ByContent:  r.ByContent,
			Exclude:    r.Exclude,
			Action:     r.Action,
			Target:     collapse(r.Target),
			OnConflict: r.OnConflict,
		})
	}

	out, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

// ---- normalization & validation --------------------------------------------

func (c *Config) expand() {
	home, _ := os.UserHomeDir()
	exp := func(p string) string {
		if p == "~" {
			return home
		}
		if strings.HasPrefix(p, "~/") {
			return filepath.Join(home, p[2:])
		}
		return p
	}
	for i := range c.Watch {
		c.Watch[i].Path = exp(c.Watch[i].Path)
	}
	for i := range c.Rules {
		c.Rules[i].Target = exp(c.Rules[i].Target)
	}
}

// Validate reports the first configuration problem in a human-readable form.
func (c *Config) Validate() error {
	if len(c.Watch) == 0 {
		return fmt.Errorf("no watch directories configured")
	}
	validAction := map[string]bool{ActionMove: true, ActionCopy: true, ActionTrash: true}
	validConflict := map[string]bool{ConflictRename: true, ConflictSkip: true, ConflictOverwrite: true, ConflictTrash: true}

	for i, r := range c.Rules {
		label := r.Name
		if label == "" {
			label = fmt.Sprintf("#%d", i+1)
		}
		if !validAction[r.Action] {
			return fmt.Errorf("rule %s: unknown action %q (use move, copy or trash)", label, r.Action)
		}
		if !validConflict[r.OnConflict] {
			return fmt.Errorf("rule %s: unknown on_conflict %q", label, r.OnConflict)
		}
		if r.Action != ActionTrash && strings.TrimSpace(r.Target) == "" {
			return fmt.Errorf("rule %s: target is required for action %q", label, r.Action)
		}
		if r.NameRegex != "" {
			if _, err := regexp.Compile(r.NameRegex); err != nil {
				return fmt.Errorf("rule %s: invalid name_regex: %w", label, err)
			}
		}
		if r.MinSize > 0 && r.MaxSize > 0 && r.MinSize > r.MaxSize {
			return fmt.Errorf("rule %s: min_size exceeds max_size", label)
		}
		if len(r.Extensions) == 0 && r.NameGlob == "" && r.NameRegex == "" &&
			r.MinSize == 0 && r.MaxSize == 0 && r.OlderThan == 0 && r.NewerThan == 0 {
			return fmt.Errorf("rule %s: has no conditions (would match every file)", label)
		}
	}
	return nil
}

func normalizeExtensions(in []string) []string {
	out := make([]string, 0, len(in))
	for _, e := range in {
		e = strings.ToLower(strings.TrimSpace(e))
		if e == "" || e == "." {
			continue
		}
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		out = append(out, e)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
