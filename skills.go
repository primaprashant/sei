package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
)

type skillEntry struct {
	name    string // Raw filesystem name, never display-escaped.
	blocked bool
}

type panelID int

type scanResult struct {
	panel      panelID
	generation uint64
	entries    []skillEntry
	missing    bool
	err        error
}

func scanPanel(id panelID, generation uint64, path string) tea.Cmd {
	return func() tea.Msg {
		result := scanResult{panel: id, generation: generation}
		result.entries, result.missing, result.err = scanFolder(path)
		return result
	}
}

func scanFolder(path string) ([]skillEntry, bool, error) {
	// Inspect raw prefixes: cleaning link/.. changes traversal, and ENOENT
	// alone cannot distinguish an absent directory from a dangling link.
	prefix := ""
	for _, component := range strings.Split(path, string(filepath.Separator)) {
		prefix += component
		if prefix == "" {
			prefix = string(filepath.Separator)
			continue
		}
		info, err := os.Lstat(prefix)
		if errors.Is(err, os.ErrNotExist) {
			return nil, true, nil
		}
		if err != nil {
			return nil, false, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			info, err = os.Stat(prefix)
			if err != nil {
				return nil, false, err
			}
		}
		if !info.IsDir() {
			return nil, false, fmt.Errorf("%q is not a directory", prefix)
		}
		prefix += string(filepath.Separator)
	}
	folder, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	entries, readErr := folder.ReadDir(-1)
	closeErr := folder.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, false, err
	}
	var skills []skillEntry
	for _, entry := range entries {
		info, err := entry.Info() // Lstat semantics: never follow a skill link.
		if err != nil {
			return nil, false, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			skills = append(skills, skillEntry{name: entry.Name(), blocked: true})
		} else if info.IsDir() {
			skills = append(skills, skillEntry{name: entry.Name()})
		}
	}
	slices.SortFunc(skills, func(a, b skillEntry) int { return strings.Compare(a.name, b.name) })
	return skills, false, nil
}
