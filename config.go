package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ErrRuleNotFound = errors.New("rule not found")

// Config holds the loaded jaqlom.json settings.
type Config struct {
	Rules []Rule `json:"rules"`
}

// Rule defines how a single file extension should be rendered.
type Rule struct {
	Ext     string            `json:"ext"`
	URL     string            `json:"url,omitempty"`
	Imports map[string]string `json:"imports,omitempty"`
	CSS     []string          `json:"css,omitempty"`
	Style   string            `json:"style,omitempty"`
	Render  string            `json:"render"`
}

func (c Config) ruleForPath(path string) (Rule, error) {
	return c.ruleForExt(strings.TrimPrefix(filepath.Ext(path), "."))
}

func (c Config) ruleForExt(ext string) (Rule, error) {
	normalized := strings.ToLower(strings.TrimPrefix(ext, "."))
	for _, rule := range c.Rules {
		ruleExt := strings.ToLower(strings.TrimPrefix(rule.Ext, "."))
		if normalized == ruleExt {
			return rule, nil
		}
	}

	return Rule{}, fmt.Errorf("%w: %s", ErrRuleNotFound, normalized)
}

func loadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config %q: %w", path, err)
	}

	return cfg, nil
}
