package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadFileWithinRoot(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	path := filepath.Join(rootDir, "docs", "guide.md")
	writeTestFile(t, path, "# guide")

	got, err := readFile(rootDir, "docs/guide.md")
	if err != nil {
		t.Fatalf("readFile(%q, %q) returned error: %v", rootDir, "docs/guide.md", err)
	}
	if string(got) != "# guide" {
		t.Fatalf("readFile(%q, %q) = %q, want %q", rootDir, "docs/guide.md", string(got), "# guide")
	}
}

func TestReadFileRejectsPathTraversal(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	writeTestFile(t, filepath.Join(rootDir, "docs", "guide.md"), "# guide")

	_, err := readFile(rootDir, "../../etc/passwd")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("readFile(%q, %q) error = %v, want ErrForbidden", rootDir, "../../etc/passwd", err)
	}
}

func TestReadFileMissingFile(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()

	_, err := readFile(rootDir, "docs/missing.md")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("readFile(%q, %q) error = %v, want ErrNotFound", rootDir, "docs/missing.md", err)
	}
}

func TestReadFileRejectsSymlinkPathTraversal(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	writeTestFile(t, outsideFile, "secret")

	// docs/ inside rootDir points outside
	linkPath := filepath.Join(rootDir, "docs", "link.txt")
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) returned error: %v", filepath.Dir(linkPath), err)
	}
	if err := os.Symlink(outsideFile, linkPath); err != nil {
		t.Fatalf("os.Symlink(%q, %q) returned error: %v", outsideFile, linkPath, err)
	}

	_, err := readFile(rootDir, "docs/link.txt")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("readFile(%q, %q) error = %v, want ErrForbidden", rootDir, "docs/link.txt", err)
	}
}

func TestReadFileAllowsValidSymlinkInsideRoot(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	realFile := filepath.Join(rootDir, "docs", "real.md")
	writeTestFile(t, realFile, "real content")

	linkPath := filepath.Join(rootDir, "docs", "link.md")
	if err := os.Symlink("real.md", linkPath); err != nil {
		t.Fatalf("os.Symlink(%q, %q) returned error: %v", "real.md", linkPath, err)
	}

	got, err := readFile(rootDir, "docs/link.md")
	if err != nil {
		t.Fatalf("readFile(%q, %q) returned error: %v", rootDir, "docs/link.md", err)
	}
	if string(got) != "real content" {
		t.Fatalf("readFile(%q, %q) = %q, want %q", rootDir, "docs/link.md", string(got), "real content")
	}
}

func TestReadFileRejectsSymlinkedParentOutsideRoot(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	outsideDir := t.TempDir()

	if err := os.Symlink(outsideDir, filepath.Join(rootDir, "docs")); err != nil {
		t.Fatalf("os.Symlink(...) returned error: %v", err)
	}

	_, err := readFile(rootDir, "docs/missing.txt")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("readFile(%q, %q) error = %v, want ErrForbidden", rootDir, "docs/missing.txt", err)
	}
}

func TestReadFileRejectsSymlinkAncestorOutsideRootWithNestedMissingPath(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	outsideDir := t.TempDir()

	if err := os.Symlink(outsideDir, filepath.Join(rootDir, "docs")); err != nil {
		t.Fatalf("os.Symlink(...) returned error: %v", err)
	}

	_, err := readFile(rootDir, "docs/nested/missing.txt")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("readFile(%q, %q) error = %v, want ErrForbidden", rootDir, "docs/nested/missing.txt", err)
	}
}

