package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestResolveOptionsDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	got, err := resolveOptions(nil, dir)
	if err != nil {
		t.Fatalf("resolveOptions(nil, %q) returned error: %v", dir, err)
	}

	want := Options{
		Port:       8080,
		RootDir:    dir,
		ConfigPath: filepath.Join(dir, "jaqlom.json"),
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("resolveOptions(nil, %q) mismatch (-want +got):\n%s", dir, diff)
	}
}

func TestResolveOptionsOverrides(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "custom.json")

	got, err := resolveOptions([]string{"-port", "9090", "-config", configPath, "docs"}, dir)
	if err != nil {
		t.Fatalf("resolveOptions returned error: %v", err)
	}

	want := Options{
		Port:       9090,
		RootDir:    filepath.Join(dir, "docs"),
		ConfigPath: configPath,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("resolveOptions(...) mismatch (-want +got):\n%s", diff)
	}
}

func TestResolveOptionsRelativeConfigPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	got, err := resolveOptions([]string{"-config", "configs/jaqlom.json", "docs"}, dir)
	if err != nil {
		t.Fatalf("resolveOptions returned error: %v", err)
	}

	want := filepath.Join(dir, "configs", "jaqlom.json")
	if got.ConfigPath != want {
		t.Fatalf("resolveOptions config path = %q, want %q", got.ConfigPath, want)
	}
}

func TestResolveOptionsAssetsDir(t *testing.T) {
	t.Parallel()

	cwd := filepath.Join(string(filepath.Separator), "tmp", "work")
	absPath := filepath.Join(string(filepath.Separator), "abs", "path")

	testCases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "absolute",
			args: []string{"-assets", absPath},
			want: absPath,
		},
		{
			name: "relative",
			args: []string{"-assets", "relative/path"},
			want: filepath.Join(cwd, "relative", "path"),
		},
		{
			name: "unset",
			args: nil,
			want: "",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveOptions(tc.args, cwd)
			if err != nil {
				t.Fatalf("resolveOptions(%v, %q) returned error: %v", tc.args, cwd, err)
			}
			if got.AssetsDir != tc.want {
				t.Fatalf("resolveOptions(%v, %q) AssetsDir = %q, want %q", tc.args, cwd, got.AssetsDir, tc.want)
			}
		})
	}
}

func TestResolveOptionsAssetsDirWithOtherFlags(t *testing.T) {
	t.Parallel()

	cwd := filepath.Join(string(filepath.Separator), "tmp", "work")
	configPath := filepath.Join(cwd, "custom.json")
	assetsDir := filepath.Join(cwd, "assets")

	got, err := resolveOptions([]string{"-port", "9090", "-config", configPath, "-assets", "assets", "docs"}, cwd)
	if err != nil {
		t.Fatalf("resolveOptions returned error: %v", err)
	}

	want := Options{
		Port:       9090,
		RootDir:    filepath.Join(cwd, "docs"),
		ConfigPath: configPath,
		AssetsDir:  assetsDir,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("resolveOptions(...) mismatch (-want +got):\n%s", diff)
	}
}

func TestServeReturnsErrorForMissingAssetsDir(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	assetsDir := filepath.Join(rootDir, "missing-assets")

	var stderr bytes.Buffer
	err := serve(Options{Port: -1, RootDir: rootDir, AssetsDir: assetsDir}, Config{}, &stderr)
	if err == nil {
		t.Fatalf("serve(...) error = nil, want non-nil")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("serve(...) error = %v, want not-exist error", err)
	}
	wantMessage := []byte("assets directory not found: " + assetsDir)
	if !bytes.Contains(stderr.Bytes(), wantMessage) {
		t.Fatalf("serve(...) stderr = %q, want message containing %q", stderr.String(), wantMessage)
	}
}

func TestServeWarnsWhenConfigHasLocalPathsWithoutAssetsDir(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	cfg := Config{Rules: []Rule{{Ext: "md", URL: "/assets/marked.min.js", Render: "return content"}}}

	var stderr bytes.Buffer
	serve(Options{Port: -1, RootDir: rootDir}, cfg, &stderr) //nolint:errcheck

	if !bytes.Contains(stderr.Bytes(), []byte("warning")) {
		t.Fatalf("serve(...) stderr = %q, want warning about local asset paths", stderr.String())
	}
}

func TestServeNoWarnWhenAssetsDirIsSet(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	assetsDir := t.TempDir()
	cfg := Config{Rules: []Rule{{Ext: "md", URL: "/assets/marked.min.js", Render: "return content"}}}

	var stderr bytes.Buffer
	serve(Options{Port: -1, RootDir: rootDir, AssetsDir: assetsDir}, cfg, &stderr) //nolint:errcheck

	if bytes.Contains(stderr.Bytes(), []byte("warning")) {
		t.Fatalf("serve(...) stderr = %q, want no warning when -assets is set", stderr.String())
	}
}

func TestServeSkipsAssetsDirValidationWhenUnset(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()

	var stderr bytes.Buffer
	err := serve(Options{Port: -1, RootDir: rootDir}, Config{}, &stderr)
	if err == nil {
		t.Fatalf("serve(...) error = nil, want non-nil")
	}
	if bytes.Contains(stderr.Bytes(), []byte("assets directory not found")) {
		t.Fatalf("serve(...) stderr = %q, want no assets directory error", stderr.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("failed to listen")) {
		t.Fatalf("serve(...) stderr = %q, want listen error", stderr.String())
	}
}

func TestRunLoadsConfig(t *testing.T) {
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

	var stderr bytes.Buffer
	opts, cfg, err := run(nil, dir, &stderr)
	if err != nil {
		t.Fatalf("run(nil, %q) returned error: %v", dir, err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("run(nil, %q) wrote stderr %q, want empty", dir, stderr.String())
	}
	if opts.ConfigPath != configPath {
		t.Fatalf("run(nil, %q) config path = %q, want %q", dir, opts.ConfigPath, configPath)
	}
	if len(cfg.Rules) != 1 || cfg.Rules[0].Ext != "md" {
		t.Fatalf("run(nil, %q) config = %#v, want md rule", dir, cfg)
	}
}

func TestRunMissingConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	var stderr bytes.Buffer
	_, _, err := run(nil, dir, &stderr)
	if err == nil {
		t.Fatalf("run(nil, %q) error = nil, want non-nil", dir)
	}
	if stderr.Len() == 0 {
		t.Fatalf("run(nil, %q) stderr is empty, want error message", dir)
	}
}

func TestRunInvalidConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "jaqlom.json"), `{"rules":[`)

	var stderr bytes.Buffer
	_, _, err := run(nil, dir, &stderr)
	if err == nil {
		t.Fatalf("run(nil, %q) error = nil, want non-nil", dir)
	}
	if stderr.Len() == 0 {
		t.Fatalf("run(nil, %q) stderr is empty, want error message", dir)
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) returned error: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) returned error: %v", path, err)
	}
}
