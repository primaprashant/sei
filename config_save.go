package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"
)

// validateSetupConfig accepts persisted (not already resolved) paths and does no
// writes. Callers can use the returned paths for the setup preview/browser.
func validateSetupConfig(cfg config, project, path string) (config, error) {
	values := []string{cfg.Library}
	for _, a := range cfg.Agents {
		values = append(values, a.Name, a.Global, a.Local)
	}
	for _, value := range values {
		if !utf8.ValidString(value) {
			return config{}, fmt.Errorf("configuration contains invalid UTF-8")
		}
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return config{}, err
	}
	cfg, err = parseConfig(data)
	if err != nil {
		return config{}, err
	}
	p := resolveRoot(project)
	if p.err != nil {
		return config{}, fmt.Errorf("project: %w", p.err)
	}
	if len(p.missing) != 0 {
		return config{}, fmt.Errorf("project must exist")
	}
	cfg, err = resolveConfigPaths(cfg, project)
	if err != nil {
		return config{}, err
	}
	_, _, _, err = configSaveLocation(cfg, path)
	if err != nil {
		return config{}, err
	}
	return cfg, nil
}

// configSaveLocation is a fresh observation, not permission for a later write.
func configSaveLocation(cfg config, path string) (resolvedRoot, string, os.FileInfo, error) {
	fail := func(err error) (resolvedRoot, string, os.FileInfo, error) {
		return resolvedRoot{}, "", nil, err
	}
	if !filepath.IsAbs(path) {
		return fail(fmt.Errorf("config path must be absolute"))
	}
	// Split, unlike Dir, preserves link/.. traversal in the parent.
	dir, name := filepath.Split(path)
	if err := validateSkillName(name); err != nil {
		return fail(fmt.Errorf("config filename: %w", err))
	}
	parent := resolveRoot(dir)
	if parent.err != nil {
		return fail(fmt.Errorf("config parent: %w", parent.err))
	}
	s := resolveRoots(cfg)
	for i, err := range s.blocked {
		if err != nil {
			return fail(fmt.Errorf("managed root %d: %w", i, err))
		}
	}
	location := parent
	location.path = filepath.Join(parent.path, name)
	for _, managed := range s.roots {
		if rootWithin(managed, location) {
			return fail(fmt.Errorf("config must not be inside managed root %q", managed.path))
		}
	}
	info, err := os.Lstat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fail(fmt.Errorf("inspect config: %w", err))
	}
	if info != nil && !info.Mode().IsRegular() {
		return fail(fmt.Errorf("config must be a regular file, not a symlink or special file"))
	}
	return parent, name, info, nil
}

// checkConfigParent detects changed physical paths and previously observed
// ancestor identities, while permitting our own missing-parent creation.
func checkConfigParent(before, after resolvedRoot) error {
	if before.path != after.path {
		return fmt.Errorf("config parent changed")
	}
	for _, a := range before.ancestors {
		found := false
		for _, b := range after.ancestors {
			if a.path == b.path && os.SameFile(a.info, b.info) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("config ancestor %q changed", a.path)
		}
	}
	return nil
}

// saveConfig persists the original path spellings. An error always precedes the
// rename commit; it never indicates that a committed save should be retried.
// External writers are not locked out, but observed changes abort the save.
func saveConfig(cfg config, project, path string, replace bool) error {
	resolved, err := validateSetupConfig(cfg, project, path)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if _, err := parseConfig(data); err != nil {
		return err
	}
	parent, name, original, err := configSaveLocation(resolved, path)
	if err != nil {
		return err
	}
	if original != nil && !replace {
		return fmt.Errorf("config already exists: %w", os.ErrExist)
	}
	root, err := os.OpenRoot(parent.ancestor)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	info, err := root.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(info, parent.ancestors[0].info) {
		return fmt.Errorf("config ancestor changed while opening")
	}
	for _, component := range parent.missing {
		fresh, _, _, err := configSaveLocation(resolved, path)
		if err != nil {
			return err
		}
		if err := checkConfigParent(parent, fresh); err != nil {
			return err
		}
		if err := root.Mkdir(component, 0o700); err != nil {
			return fmt.Errorf("create config parent: %w", err)
		}
		created, err := root.Lstat(component)
		if err != nil {
			return err
		}
		if !created.IsDir() || created.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("created config parent changed")
		}
		next, err := root.OpenRoot(component)
		if err != nil {
			return err
		}
		_ = root.Close()
		root = next
		info, err = root.Stat(".")
		if err != nil {
			return err
		}
		if !os.SameFile(info, created) {
			return fmt.Errorf("config parent changed while opening")
		}
		fresh, _, _, err = configSaveLocation(resolved, path)
		if err != nil {
			return err
		}
		if err := checkConfigParent(parent, fresh); err != nil {
			return err
		}
		if !os.SameFile(info, fresh.ancestors[0].info) {
			return fmt.Errorf("created config parent moved")
		}
		parent = fresh
	}
	// Keep the original target identity (including absence) through the entire
	// operation. Size/time/mode also catch ordinary in-place edits of that inode.
	check := func() error {
		fresh, _, current, err := configSaveLocation(resolved, path)
		if err != nil {
			return err
		}
		if err := checkConfigParent(parent, fresh); err != nil {
			return err
		}
		held, err := root.Stat(".")
		if err != nil {
			return err
		}
		if len(fresh.missing) != 0 || !os.SameFile(held, fresh.ancestors[0].info) {
			return fmt.Errorf("config parent changed")
		}
		if (original == nil) != (current == nil) {
			return fmt.Errorf("config existence changed")
		}
		if original != nil && (!os.SameFile(original, current) || original.Size() != current.Size() || original.Mode() != current.Mode() || !original.ModTime().Equal(current.ModTime())) {
			return fmt.Errorf("config changed")
		}
		return nil
	}
	if err := check(); err != nil {
		return err
	}
	if original != nil {
		old, err := root.ReadFile(name)
		if err != nil {
			return err
		}
		if _, err := parseConfig(old); err != nil {
			return fmt.Errorf("existing config: %w", err)
		}
	}
	temp := ".sei-config-" + rand.Text()
	file, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = root.Remove(temp) }()
	_, writeErr := file.Write(data)
	var syncErr error
	if writeErr == nil {
		syncErr = file.Sync()
	}
	tempInfo, statErr := file.Stat()
	if err := errors.Join(writeErr, syncErr, statErr, file.Close()); err != nil {
		return err
	}
	currentTemp, err := root.Lstat(temp)
	if err != nil {
		return err
	}
	if !currentTemp.Mode().IsRegular() || !os.SameFile(tempInfo, currentTemp) || tempInfo.Size() != currentTemp.Size() || !tempInfo.ModTime().Equal(currentTemp.ModTime()) {
		return fmt.Errorf("temporary config changed")
	}
	if err := check(); err != nil {
		return err
	}
	return root.Rename(temp, name)
}
