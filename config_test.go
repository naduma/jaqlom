package main

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "jaqlom.json")
	writeTestFile(t, configPath, `{
  "rules": [
    {
      "ext": "md",
      "url": "https://cdn.example.test/marked.js",
      "render": "return content"
    }
  ]
}`)

	got, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("loadConfig(%q) returned error: %v", configPath, err)
	}

	want := Config{
		Rules: []Rule{
			{
				Ext:    "md",
				URL:    "https://cdn.example.test/marked.js",
				Render: "return content",
			},
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("loadConfig(%q) mismatch (-want +got):\n%s", configPath, diff)
	}
}

func TestLoadConfigInvalidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "jaqlom.json")
	writeTestFile(t, configPath, `{"rules":[`)

	_, err := loadConfig(configPath)
	if err == nil {
		t.Fatalf("loadConfig(%q) error = nil, want non-nil", configPath)
	}
}

func TestConfigRuleForPath(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Rules: []Rule{
			{
				Ext:     "md",
				URL:     "https://cdn.example.test/marked.js",
				Imports: map[string]string{"default": "marked"},
				CSS:     []string{"https://cdn.example.test/markdown.css"},
				Style:   ".markdown-body { max-width: 48rem; }",
				Render:  "return content",
			},
		},
	}

	got, err := cfg.ruleForPath("docs/guide.md")
	if err != nil {
		t.Fatalf("Config.ruleForPath(%q) returned error: %v", "docs/guide.md", err)
	}

	want := Rule{
		Ext:     "md",
		URL:     "https://cdn.example.test/marked.js",
		Imports: map[string]string{"default": "marked"},
		CSS:     []string{"https://cdn.example.test/markdown.css"},
		Style:   ".markdown-body { max-width: 48rem; }",
		Render:  "return content",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Config.ruleForPath(%q) mismatch (-want +got):\n%s", "docs/guide.md", diff)
	}
}

func TestConfigRuleForPathMissing(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Rules: []Rule{
			{
				Ext:    "md",
				URL:    "https://cdn.example.test/marked.js",
				Render: "return content",
			},
		},
	}

	_, err := cfg.ruleForPath("docs/guide.txt")
	if !errors.Is(err, ErrRuleNotFound) {
		t.Fatalf("Config.ruleForPath(%q) error = %v, want ErrRuleNotFound", "docs/guide.txt", err)
	}
}

func TestConfigRuleForExt(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Rules: []Rule{
			{
				Ext:    "md",
				URL:    "https://cdn.example.test/marked.js",
				Render: "return content",
			},
		},
	}

	got, err := cfg.ruleForExt("md")
	if err != nil {
		t.Fatalf("Config.ruleForExt(%q) returned error: %v", "md", err)
	}

	if got.Ext != "md" {
		t.Fatalf("Config.ruleForExt(%q) = %q, want %q", "md", got.Ext, "md")
	}

	gotUpper, err := cfg.ruleForExt(".MD")
	if err != nil {
		t.Fatalf("Config.ruleForExt(%q) returned error: %v", ".MD", err)
	}

	if gotUpper.Ext != "md" {
		t.Fatalf("Config.ruleForExt(%q) = %q, want %q", ".MD", gotUpper.Ext, "md")
	}
}

func TestConfigRuleForPathWithDotPrefixedRuleExt(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Rules: []Rule{
			{
				Ext:    ".md",
				URL:    "https://cdn.example.test/marked.js",
				Render: "return content",
			},
		},
	}

	got, err := cfg.ruleForPath("docs/guide.md")
	if err != nil {
		t.Fatalf("Config.ruleForPath(%q) returned error: %v", "docs/guide.md", err)
	}
	if got.Ext != ".md" {
		t.Fatalf("Config.ruleForPath(%q) = %q, want %q", "docs/guide.md", got.Ext, ".md")
	}

	gotUpper, err := cfg.ruleForPath("docs/GUIDE.MD")
	if err != nil {
		t.Fatalf("Config.ruleForPath(%q) returned error: %v", "docs/GUIDE.MD", err)
	}
	if gotUpper.Ext != ".md" {
		t.Fatalf("Config.ruleForPath(%q) = %q, want %q", "docs/GUIDE.MD", gotUpper.Ext, ".md")
	}
}

func TestConfigRuleForPathUpperCase(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Rules: []Rule{
			{
				Ext:    "md",
				URL:    "https://cdn.example.test/marked.js",
				Render: "return content",
			},
		},
	}

	got, err := cfg.ruleForPath("docs/GUIDE.MD")
	if err != nil {
		t.Fatalf("Config.ruleForPath(%q) returned error: %v", "docs/GUIDE.MD", err)
	}

	if got.Ext != "md" {
		t.Fatalf("Config.ruleForPath(%q) = %q, want %q", "docs/GUIDE.MD", got.Ext, "md")
	}
}
