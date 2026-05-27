package main

import (
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
)

func scanFiles(rootDir string, cfg Config) ([]string, error) {
	allowedExts := make(map[string]struct{}, len(cfg.Rules))
	for _, rule := range cfg.Rules {
		ext := strings.TrimPrefix(rule.Ext, ".")
		if ext == "" {
			continue
		}
		allowedExts["."+ext] = struct{}{}
	}

	var paths []string
	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		if _, ok := allowedExts[strings.ToLower(filepath.Ext(path))]; !ok {
			return nil
		}

		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}

		paths = append(paths, filepath.ToSlash(relPath))
		return nil
	})
	if err != nil {
		return nil, err
	}

	slices.Sort(paths)
	return paths, nil
}
