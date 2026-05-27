package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrForbidden = errors.New("forbidden")
	ErrNotFound  = errors.New("not found")
)

func readFile(rootDir string, relPath string) ([]byte, error) {
	resolvedPath, err := resolvePath(rootDir, relPath)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, relPath)
		}
		return nil, fmt.Errorf("read file %q: %w", relPath, err)
	}

	return data, nil
}

func resolvePath(rootDir string, relPath string) (string, error) {
	canonicalRoot, err := filepath.EvalSymlinks(rootDir)
	if err != nil {
		return "", fmt.Errorf("resolve root dir %q: %w", rootDir, err)
	}

	candidate := filepath.Join(canonicalRoot, filepath.Clean(relPath))

	for current := candidate; ; current = filepath.Dir(current) {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			canonicalPath := filepath.Join(resolved, strings.TrimPrefix(candidate, current))
			relativeToRoot, err := filepath.Rel(canonicalRoot, canonicalPath)
			if err != nil {
				return "", fmt.Errorf("compare file path %q to root %q: %w", relPath, rootDir, err)
			}
			if relativeToRoot == ".." || strings.HasPrefix(relativeToRoot, ".."+string(filepath.Separator)) {
				return "", fmt.Errorf("%w: %s", ErrForbidden, relPath)
			}
			return candidate, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("resolve file path %q: %w", relPath, err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("resolve file path %q: %w", relPath, err)
		}
	}
}
