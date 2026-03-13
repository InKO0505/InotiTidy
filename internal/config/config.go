package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Rule struct {
	Extensions []string `yaml:"extensions"`
	Target     string   `yaml:"target"`
}

type Config struct {
	WatchDirs []string `yaml:"watch_directories"`
	Excludes  []string `yaml:"exclude_keywords"`
	Rules     []Rule   `yaml:"rules"`
}

func Load() (*Config, error) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".config/inotitidy/config.yaml")
	return LoadFromPath(path)
}

func LoadFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg, err := parseYAMLConfig(string(data))
	if err != nil {
		return nil, err
	}

	home, _ := os.UserHomeDir()
	for i, dir := range cfg.WatchDirs {
		cfg.WatchDirs[i] = strings.Replace(dir, "~", home, 1)
	}
	for i, rule := range cfg.Rules {
		cfg.Rules[i].Target = strings.Replace(rule.Target, "~", home, 1)
	}

	return cfg, nil
}

func parseYAMLConfig(src string) (*Config, error) {
	cfg := &Config{}
	section := ""
	var currentRule *Rule

	scanner := bufio.NewScanner(strings.NewReader(src))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		switch trimmed {
		case "watch_directories:", "exclude_keywords:", "rules:":
			section = strings.TrimSuffix(trimmed, ":")
			if section != "rules" {
				currentRule = nil
			}
			continue
		}

		switch section {
		case "watch_directories":
			if strings.HasPrefix(trimmed, "- ") {
				cfg.WatchDirs = append(cfg.WatchDirs, unquote(strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))))
			}
		case "exclude_keywords":
			if strings.HasPrefix(trimmed, "- ") {
				cfg.Excludes = append(cfg.Excludes, unquote(strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))))
			}
		case "rules":
			if strings.HasPrefix(trimmed, "- extensions:") {
				r := Rule{}
				value := strings.TrimSpace(strings.TrimPrefix(trimmed, "- extensions:"))
				r.Extensions = parseInlineList(value)
				cfg.Rules = append(cfg.Rules, r)
				currentRule = &cfg.Rules[len(cfg.Rules)-1]
				continue
			}
			if currentRule != nil && strings.HasPrefix(trimmed, "target:") {
				currentRule.Target = unquote(strings.TrimSpace(strings.TrimPrefix(trimmed, "target:")))
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(cfg.WatchDirs) == 0 {
		return nil, fmt.Errorf("watch_directories is required")
	}
	return cfg, nil
}

func parseInlineList(value string) []string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "[")
	value = strings.TrimSuffix(value, "]")
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, p := range parts {
		item := unquote(strings.TrimSpace(p))
		if item != "" {
			items = append(items, strings.ToLower(item))
		}
	}
	return items
}

func unquote(v string) string {
	v = strings.TrimSpace(v)
	v = strings.Trim(v, "\"")
	v = strings.Trim(v, "'")
	return v
}
