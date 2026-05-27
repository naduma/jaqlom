package main

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestScanFilesIncludesNestedMatchingFiles(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	writeTestFile(t, filepath.Join(rootDir, "docs", "guide.md"), "# guide")
	writeTestFile(t, filepath.Join(rootDir, "docs", "nested", "deep.adoc"), "= deep")
	writeTestFile(t, filepath.Join(rootDir, "notes.txt"), "ignore")

	cfg := Config{
		Rules: []Rule{
			{Ext: "md"},
			{Ext: "adoc"},
		},
	}

	got, err := scanFiles(rootDir, cfg)
	if err != nil {
		t.Fatalf("scanFiles(%q, %+v) returned error: %v", rootDir, cfg, err)
	}

	want := []string{
		"docs/guide.md",
		"docs/nested/deep.adoc",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("scanFiles(%q, %+v) mismatch (-want +got):\n%s", rootDir, cfg, diff)
	}
}

func TestScanFilesFiltersUnknownExtensions(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	writeTestFile(t, filepath.Join(rootDir, "README.md"), "# readme")
	writeTestFile(t, filepath.Join(rootDir, "image.png"), "png")
	writeTestFile(t, filepath.Join(rootDir, "script.js"), "console.log('x')")

	cfg := Config{
		Rules: []Rule{
			{Ext: "md"},
		},
	}

	got, err := scanFiles(rootDir, cfg)
	if err != nil {
		t.Fatalf("scanFiles(%q, %+v) returned error: %v", rootDir, cfg, err)
	}

	want := []string{"README.md"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("scanFiles(%q, %+v) mismatch (-want +got):\n%s", rootDir, cfg, diff)
	}
}

func TestScanFilesMatchesUpperCaseExtensions(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	writeTestFile(t, filepath.Join(rootDir, "README.MD"), "# readme")
	writeTestFile(t, filepath.Join(rootDir, "notes.TXT"), "ignore")

	cfg := Config{
		Rules: []Rule{
			{Ext: "md"},
		},
	}

	got, err := scanFiles(rootDir, cfg)
	if err != nil {
		t.Fatalf("scanFiles(%q, %+v) returned error: %v", rootDir, cfg, err)
	}

	want := []string{"README.MD"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("scanFiles(%q, %+v) mismatch (-want +got):\n%s", rootDir, cfg, diff)
	}
}

func TestScanFilesSortsNormalizedPaths(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	writeTestFile(t, filepath.Join(rootDir, "zeta", "last.md"), "# last")
	writeTestFile(t, filepath.Join(rootDir, "alpha.md"), "# alpha")
	writeTestFile(t, filepath.Join(rootDir, "beta", "middle.md"), "# middle")

	cfg := Config{
		Rules: []Rule{
			{Ext: "md"},
		},
	}

	got, err := scanFiles(rootDir, cfg)
	if err != nil {
		t.Fatalf("scanFiles(%q, %+v) returned error: %v", rootDir, cfg, err)
	}

	want := []string{
		"alpha.md",
		"beta/middle.md",
		"zeta/last.md",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("scanFiles(%q, %+v) mismatch (-want +got):\n%s", rootDir, cfg, diff)
	}
}
