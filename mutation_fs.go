package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// skillTreeEntry is a postorder inventory of ordinary directories and files.
type skillTreeEntry struct {
	path string
	info os.FileInfo
}

func readSkillDirectory(root *os.Root, path string) ([]os.DirEntry, error) {
	f, err := root.Open(path)
	if err != nil {
		return nil, err
	}
	entries, err := f.ReadDir(-1)
	return entries, errors.Join(err, f.Close())
}

func inventorySkill(root *os.Root, name string) ([]skillTreeEntry, error) {
	var inventory []skillTreeEntry
	var visit func(string) error
	visit = func(path string) error {
		info, err := root.Lstat(path)
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported skill entry %q (%s)", path, info.Mode())
		}
		if info.IsDir() {
			entries, err := readSkillDirectory(root, path)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if err := visit(filepath.Join(path, entry.Name())); err != nil {
					return err
				}
			}
		}
		inventory = append(inventory, skillTreeEntry{path, info})
		return nil
	}
	err := visit(name)
	return inventory, err
}

func removeSkill(cfg config, destination panelID, name string) error {
	return removeSkillObserved(cfg, destination, name, nil)
}

// beforeRemove is a per-call test seam, invoked before validation, never between
// the final checks and Remove. An error after deletion starts can be partial.
func removeSkillObserved(cfg config, destination panelID, name string, beforeRemove func(string)) error {
	root, err := openSkillRoot(cfg, destination, name)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	held, err := root.Stat(".")
	if err != nil {
		return err
	}
	exactName := func() error {
		entries, err := readSkillDirectory(root, ".")
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.Name() == name {
				return nil
			}
		}
		return fmt.Errorf("skill %q no longer exists with that exact name", name)
	}
	if err := exactName(); err != nil {
		return err
	}
	inventory, err := inventorySkill(root, name)
	if err != nil {
		return err
	}
	if !inventory[len(inventory)-1].info.IsDir() {
		return fmt.Errorf("skill %q is not an ordinary directory", name)
	}
	remaining := make(map[string]os.FileInfo, len(inventory))
	for _, entry := range inventory {
		remaining[entry.path] = entry.info
	}
	for _, next := range inventory {
		if beforeRemove != nil {
			beforeRemove(next.path)
		}
		resolved, err := revalidateSkillRoot(cfg, destination, name)
		if err != nil {
			return err
		}
		if len(resolved.missing) != 0 || !os.SameFile(held, resolved.ancestors[0].info) {
			return fmt.Errorf("destination changed during removal")
		}
		if err := exactName(); err != nil {
			return err
		}
		safety := resolveRoots(cfg)
		protected := append(safety.protected, safety.roots...)
		// Verify the entire remaining tree, ancestors first. This also catches
		// additions before any deletion, and never follows an observed link.
		for i := len(inventory) - 1; i >= 0; i-- {
			entry := inventory[i]
			if _, exists := remaining[entry.path]; !exists {
				continue
			}
			info, err := root.Lstat(entry.path)
			if err != nil {
				return err
			}
			if info.Mode().Type() != entry.info.Mode().Type() || !os.SameFile(info, entry.info) {
				return fmt.Errorf("skill entry %q changed during removal", entry.path)
			}
			if !info.IsDir() {
				continue
			}
			for _, p := range protected {
				if p.err != nil {
					return p.err
				}
				for _, ancestor := range p.ancestors {
					if os.SameFile(info, ancestor.info) {
						return fmt.Errorf("skill directory %q aliases a protected root or ancestor", entry.path)
					}
				}
			}
			children, err := readSkillDirectory(root, entry.path)
			if err != nil {
				return err
			}
			for _, child := range children {
				if _, exists := remaining[filepath.Join(entry.path, child.Name())]; !exists {
					return fmt.Errorf("unexpected child in %q: %q", entry.path, child.Name())
				}
			}
		}
		// Repeat the selected path's ancestry checks immediately before Remove.
		resolved, err = revalidateSkillRoot(cfg, destination, name)
		if err != nil {
			return err
		}
		if len(resolved.missing) != 0 || !os.SameFile(held, resolved.ancestors[0].info) {
			return fmt.Errorf("destination changed before removal")
		}
		var ancestors []string
		for path := next.path; path != "."; path = filepath.Dir(path) {
			ancestors = append(ancestors, path)
		}
		for i := len(ancestors) - 1; i >= 0; i-- {
			path := ancestors[i]
			info, err := root.Lstat(path)
			if err != nil {
				return err
			}
			expected := remaining[path]
			if info.Mode().Type() != expected.Mode().Type() || !os.SameFile(info, expected) {
				return fmt.Errorf("skill entry %q changed before removal", path)
			}
		}
		if err := root.Remove(next.path); err != nil {
			return fmt.Errorf("remove %q: %w", next.path, err)
		}
		delete(remaining, next.path)
	}
	return nil
}
