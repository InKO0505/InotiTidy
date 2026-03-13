package config

import (
	"os"
	"path/filepath"
	"strings"
	"gopkg.in/yaml.v3"
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
	
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	
	for i, dir := range cfg.WatchDirs {
		cfg.WatchDirs[i] = strings.Replace(dir, "~", home, 1)
	}
	for i, rule := range cfg.Rules {
		cfg.Rules[i].Target = strings.Replace(rule.Target, "~", home, 1)
	}

	return &cfg, nil
}