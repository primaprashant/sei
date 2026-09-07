package app

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
)

type skillEntry struct {
	name    string // Raw filesystem name, never display-escaped.
	blocked bool
	info    os.FileInfo
}

type panelID int

type scanResult struct {
	panel      panelID
	generation uint64
	entries    []skillEntry
	missing    bool
	err        error
	root       resolvedRoot
}

func scanPanel(id panelID, generation uint64, path string) tea.Cmd {
	return func() tea.Msg {
		result := scanResult{panel: id, generation: generation}
		result.root = resolveRoot(path)
		result.entries, result.missing, result.err = scanResolvedFolder(result.root)
		if result.err == nil && !sameResolvedRoot(result.root, resolveRoot(path), 0) {
			result.entries, result.err = nil, fmt.Errorf("root changed during scan")
		}
		return result
	}
}

func scanFolder(path string) ([]skillEntry, bool, error) {
	return scanResolvedFolder(resolveRoot(path))
}

func validateScannedSelection(cfg config, r mutationRequest, selectedRoot, destinationRoot resolvedRoot, entry skillEntry) error {
	destination, err := revalidateSkillRoot(cfg, r.destination, r.name)
	if err != nil {
		return err
	}
	if !sameResolvedRoot(destinationRoot, destination, 0) {
		return fmt.Errorf("destination root changed since scan; refresh and retry")
	}
	path := r.path
	if r.add {
		path = cfg.Library
	}
	current := resolveRoot(path)
	if !sameResolvedRoot(selectedRoot, current, 0) {
		return fmt.Errorf("selected root changed since scan; refresh and retry")
	}
	entries, _, err := scanResolvedFolder(current)
	if err != nil {
		return err
	}
	for _, now := range entries {
		if now.name == r.name {
			if entry.info != nil && now.info.Mode().Type() == entry.info.Mode().Type() && os.SameFile(now.info, entry.info) {
				return nil
			}
			return fmt.Errorf("selected entry %q changed since scan; refresh and retry", r.name)
		}
	}
	return fmt.Errorf("selected entry %q disappeared since scan; refresh and retry", r.name)
}

func scanResolvedFolder(r resolvedRoot) ([]skillEntry, bool, error) {
	if r.err != nil {
		return nil, false, r.err
	}
	if len(r.missing) != 0 {
		return nil, true, nil
	}
	folder, err := os.OpenRoot(r.path)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = folder.Close() }()
	held, err := folder.Stat(".")
	if err != nil || !os.SameFile(held, r.ancestors[0].info) {
		return nil, false, errors.Join(err, fmt.Errorf("root changed while scanning"))
	}
	entries, err := readSkillDirectory(folder, ".")
	if err != nil {
		return nil, false, err
	}
	var skills []skillEntry
	for _, entry := range entries {
		// Repository metadata is never a top-level skill.
		if entry.Name() == ".git" {
			continue
		}
		info, err := folder.Lstat(entry.Name())
		if err != nil {
			return nil, false, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			skills = append(skills, skillEntry{name: entry.Name(), blocked: true, info: info})
		} else if info.IsDir() {
			skills = append(skills, skillEntry{name: entry.Name(), info: info})
		}
	}
	slices.SortFunc(skills, func(a, b skillEntry) int { return strings.Compare(a.name, b.name) })
	return skills, false, nil
}
