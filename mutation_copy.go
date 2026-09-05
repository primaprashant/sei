package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// A prepared source owns its root and metadata inventory, never file contents.
// Replacement can prepare before deletion, then reopen verified files while
// copying. A late source read failure may leave partial output.
type preparedSkill struct {
	root      *os.Root
	held      os.FileInfo
	name      string
	inventory []skillTreeEntry
}

// Per-call seams; before runs before validation, not after authorization.
type copySkillOps struct {
	before       func(string)
	copy         func(io.Writer, io.Reader) (int64, error)
	open         func(*os.Root, string, int, os.FileMode) (copySkillFile, error)
	beforeRemove func(string)
	remove       func(*os.Root, string) error
}

type copySkillFile interface {
	io.WriteCloser
	Stat() (os.FileInfo, error)
}

func addSkill(cfg config, destination panelID, name string) error {
	return addSkillWithOps(cfg, destination, name, copySkillOps{})
}

func addSkillWithOps(cfg config, destination panelID, name string, ops copySkillOps) (err error) {
	initial, err := revalidateSkillRoot(cfg, destination, name)
	if err != nil {
		return err
	}
	source, err := prepareSkillSource(cfg, name, ops)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, source.root.Close()) }()
	r, err := revalidateCopyDestination(cfg, destination, name, initial, 0)
	if err != nil {
		return err
	}
	if len(r.missing) == 0 {
		target, err := openSkillRoot(cfg, destination, name)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, target.Close()) }()
		if _, lookupErr := target.Lstat(name); lookupErr == nil {
			inventory, err := inventorySkill(target, name)
			if err != nil {
				return err
			}
			if err := replacementNames(target, source, inventory); err != nil {
				return err
			}
			err = removeSkillInventory(cfg, destination, name, target, r.ancestors[0].info, inventory, func(path string) error {
				if ops.beforeRemove != nil {
					ops.beforeRemove(path)
				}
				if _, err := revalidateCopyDestination(cfg, destination, name, initial, 0); err != nil {
					return err
				}
				return source.validate(cfg)
			}, ops.remove)
			if err != nil {
				return err
			}
		} else if !errors.Is(lookupErr, os.ErrNotExist) {
			return lookupErr
		}
	}
	return copyPreparedSkill(cfg, destination, source, initial, ops)
}

// Keep one destination observation across preparation, removal, and copying.
// Only components created by this copy may consume the original missing suffix.
// These checks reject observed changes, not arbitrary hostile-writer races.
func revalidateCopyDestination(cfg config, destination panelID, name string, initial resolvedRoot, created int) (resolvedRoot, error) {
	current, err := revalidateSkillRoot(cfg, destination, name)
	if err != nil {
		return resolvedRoot{}, err
	}
	if current.path != initial.path || len(current.missing) != len(initial.missing)-created || len(current.ancestors) != len(initial.ancestors)+created {
		return resolvedRoot{}, fmt.Errorf("destination resolution changed")
	}
	for i, ancestor := range initial.ancestors {
		now := current.ancestors[i+created]
		if now.path != ancestor.path || !os.SameFile(now.info, ancestor.info) {
			return resolvedRoot{}, fmt.Errorf("destination ancestor changed: %q", ancestor.path)
		}
	}
	return current, nil
}

// Check actual lookup behavior, not guessed case folding or writing probes.
// Missing directories (including file-to-directory swaps) provide no evidence
// about nested aliases: exclusive creation may discover those only after removal.
func replacementNames(target *os.Root, source *preparedSkill, inventory []skillTreeEntry) error {
	directories := map[string]bool{".": true}
	for _, entry := range inventory {
		directories[entry.path] = entry.info.IsDir()
	}
	for i := len(source.inventory) - 1; i >= 0; i-- {
		path := source.inventory[i].path
		parent := filepath.Dir(path)
		if !directories[parent] {
			continue
		}
		entries, err := readSkillDirectory(target, parent)
		if err != nil {
			return err
		}
		_, err = target.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		exact := false
		for _, entry := range entries {
			if entry.Name() == filepath.Base(path) {
				exact = true
			}
		}
		if !exact {
			return fmt.Errorf("replacement name %q aliases an existing entry", path)
		}
	}
	return nil
}

func prepareSkillSource(cfg config, name string, ops copySkillOps) (_ *preparedSkill, err error) {
	if err := validateSkillName(name); err != nil {
		return nil, err
	}
	r := resolveRoot(cfg.Library)
	if r.err != nil {
		return nil, r.err
	}
	if len(r.missing) != 0 {
		return nil, fmt.Errorf("source root does not exist")
	}
	root, err := os.OpenRoot(r.path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, root.Close())
		}
	}()
	s := &preparedSkill{root: root, held: r.ancestors[0].info, name: name}
	entries, err := readSkillDirectory(root, ".")
	if err != nil {
		return nil, err
	}
	exact := false
	for _, entry := range entries {
		exact = exact || entry.Name() == name
	}
	if !exact {
		return nil, fmt.Errorf("source %q does not exist with that exact name", name)
	}
	s.inventory, err = inventorySkill(root, name)
	if err != nil {
		return nil, err
	}
	if !s.inventory[len(s.inventory)-1].info.IsDir() {
		return nil, fmt.Errorf("source %q is not an ordinary directory", name)
	}
	stream := ops.copy
	if stream == nil {
		stream = io.Copy
	}
	for _, entry := range s.inventory {
		if err := s.validate(cfg); err != nil {
			return nil, err
		}
		if entry.info.IsDir() {
			continue
		}
		f, err := root.Open(entry.path)
		if err != nil {
			return nil, err
		}
		info, statErr := f.Stat()
		if statErr != nil || !sameCopyEntry(info, entry.info) {
			return nil, errors.Join(fmt.Errorf("source changed while opening %q", entry.path), statErr, f.Close())
		}
		n, readErr := stream(io.Discard, f)
		info, statErr = f.Stat()
		closeErr := f.Close()
		if err := errors.Join(readErr, statErr, closeErr); err != nil {
			return nil, err
		}
		if !sameCopyEntry(info, entry.info) || n != entry.info.Size() {
			return nil, fmt.Errorf("source changed while reading %q", entry.path)
		}
	}
	if err := s.validate(cfg); err != nil {
		return nil, err
	}
	return s, nil
}

func sameCopyEntry(a, b os.FileInfo) bool {
	return a != nil && b != nil && os.SameFile(a, b) && a.Mode() == b.Mode() &&
		(a.IsDir() || (a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())))
}

func (s *preparedSkill) validate(cfg config) error {
	safety := resolveRoots(cfg)
	r := safety.roots[0]
	if r.err != nil {
		return r.err
	}
	if len(r.missing) != 0 || !os.SameFile(s.held, r.ancestors[0].info) {
		return fmt.Errorf("source root changed")
	}
	held, err := s.root.Stat(".")
	if err != nil || !os.SameFile(s.held, held) {
		return errors.Join(err, fmt.Errorf("source root identity changed"))
	}
	children := make(map[string]os.FileInfo, len(s.inventory))
	for _, entry := range s.inventory {
		children[entry.path] = entry.info
	}
	// Ancestors first, so no observed internal link is traversed.
	for i := len(s.inventory) - 1; i >= 0; i-- {
		entry := s.inventory[i]
		info, err := s.root.Lstat(entry.path)
		if err != nil || !sameCopyEntry(info, entry.info) {
			return errors.Join(err, fmt.Errorf("source entry changed: %q", entry.path))
		}
		if !info.IsDir() {
			continue
		}
		if err := copyDirectorySafety(info, safety); err != nil {
			return err
		}
		entries, err := readSkillDirectory(s.root, entry.path)
		if err != nil {
			return err
		}
		for _, child := range entries {
			if children[filepath.Join(entry.path, child.Name())] == nil {
				return fmt.Errorf("unexpected source child %q", child.Name())
			}
		}
	}
	entries, err := readSkillDirectory(s.root, ".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == s.name {
			return nil
		}
	}
	return fmt.Errorf("source exact name changed")
}

func copyDirectorySafety(info os.FileInfo, safety rootSafety) error {
	for _, p := range append(safety.protected, safety.roots...) {
		if p.err != nil {
			return p.err
		}
		for _, ancestor := range p.ancestors {
			if os.SameFile(info, ancestor.info) {
				return fmt.Errorf("copy directory aliases a protected root or ancestor")
			}
		}
	}
	return nil
}

// Only this phase creates output. Errors after creation may leave a partial
// fresh tree; never remove or merge an existing entry, including lookup aliases.
func copyPreparedSkill(cfg config, destination panelID, s *preparedSkill, initial resolvedRoot, ops copySkillOps) (err error) {
	r, err := revalidateCopyDestination(cfg, destination, s.name, initial, 0)
	if err != nil {
		return err
	}
	anchor, err := os.OpenRoot(r.ancestor)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, anchor.Close()) }()
	type heldDirectory struct {
		path string
		root *os.Root
		info os.FileInfo
	}
	dirs := []heldDirectory{{r.ancestor, anchor, r.ancestors[0].info}}
	defer func() {
		for _, dir := range dirs[1:] {
			err = errors.Join(err, dir.root.Close())
		}
	}()
	created := make(map[string]os.FileInfo)
	createdComponents := 0
	var target *os.Root
	check := func() error {
		_, err := revalidateCopyDestination(cfg, destination, s.name, initial, createdComponents)
		if err != nil {
			return err
		}
		if err := s.validate(cfg); err != nil {
			return err
		}
		for _, dir := range dirs {
			pathInfo, err := os.Lstat(dir.path)
			if err != nil || !pathInfo.IsDir() || !os.SameFile(pathInfo, dir.info) {
				return errors.Join(err, fmt.Errorf("destination directory path changed: %q", dir.path))
			}
			resolved := resolveRoot(dir.path)
			if resolved.err != nil || len(resolved.missing) != 0 || !os.SameFile(dir.info, resolved.ancestors[0].info) {
				return fmt.Errorf("held destination directory changed: %q", dir.path)
			}
			info, err := dir.root.Stat(".")
			if err != nil || !os.SameFile(info, dir.info) {
				return errors.Join(err, fmt.Errorf("held destination identity changed"))
			}
		}
		if target != nil {
			// Validate directories first; map iteration must not traverse a
			// replaced ancestor before observing the replacement.
			for i := len(s.inventory) - 1; i >= 0; i-- {
				path := s.inventory[i].path
				expected := created[path]
				if expected == nil {
					continue
				}
				info, err := target.Lstat(path)
				if err != nil || !sameCopyEntry(info, expected) {
					return errors.Join(err, fmt.Errorf("output changed: %q", path))
				}
				if !info.IsDir() {
					continue
				}
				if err := copyDirectorySafety(info, resolveRoots(cfg)); err != nil {
					return err
				}
				entries, err := readSkillDirectory(target, path)
				if err != nil {
					return err
				}
				for _, child := range entries {
					if created[filepath.Join(path, child.Name())] == nil {
						return fmt.Errorf("unexpected output child %q", child.Name())
					}
				}
			}
		}
		return nil
	}
	before := func(path string) error {
		if ops.before != nil {
			ops.before(path)
		}
		return check()
	}
	if err := before(r.path); err != nil {
		return err
	}
	for _, component := range r.missing {
		parent := dirs[len(dirs)-1]
		if err := before(filepath.Join(parent.path, component)); err != nil {
			return err
		}
		if err := parent.root.Mkdir(component, 0o777); err != nil {
			return err
		}
		info, err := parent.root.Lstat(component)
		if err != nil || !info.IsDir() {
			return errors.Join(err, fmt.Errorf("created destination is not a directory"))
		}
		next, err := parent.root.OpenRoot(component)
		if err != nil {
			return err
		}
		dirs = append(dirs, heldDirectory{filepath.Join(parent.path, component), next, info})
		createdComponents++
		if err := check(); err != nil {
			return err
		}
	}
	target = dirs[len(dirs)-1].root
	stream := ops.copy
	if stream == nil {
		stream = io.Copy
	}
	open := ops.open
	if open == nil {
		open = func(root *os.Root, path string, flags int, mode os.FileMode) (copySkillFile, error) {
			return root.OpenFile(path, flags, mode)
		}
	}
	for i := len(s.inventory) - 1; i >= 0; i-- {
		entry := s.inventory[i]
		if err := before(entry.path); err != nil {
			return err
		}
		if _, err := target.Lstat(entry.path); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(err, fmt.Errorf("output %q already exists or cannot be inspected", entry.path))
		}
		if entry.info.IsDir() {
			if err := target.Mkdir(entry.path, 0o777); err != nil {
				return err
			}
		} else {
			source, err := s.root.Open(entry.path)
			if err != nil {
				return err
			}
			sourceInfo, statErr := source.Stat()
			if statErr != nil || !sameCopyEntry(sourceInfo, entry.info) {
				return errors.Join(statErr, fmt.Errorf("source changed while reopening %q", entry.path), source.Close())
			}
			if err := check(); err != nil {
				return errors.Join(err, source.Close())
			}
			f, err := open(target, entry.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666|entry.info.Mode().Perm()&0o111)
			if err != nil {
				return errors.Join(err, source.Close())
			}
			opened, statErr := f.Stat()
			pathInfo, pathErr := target.Lstat(entry.path)
			if statErr != nil || pathErr != nil || !opened.Mode().IsRegular() || !sameCopyEntry(opened, pathInfo) {
				return errors.Join(statErr, pathErr, fmt.Errorf("output identity changed while opening %q", entry.path), source.Close(), f.Close())
			}
			created[entry.path] = opened
			if err := check(); err != nil {
				return errors.Join(err, source.Close(), f.Close())
			}
			n, copyErr := stream(f, source)
			sourceInfo, sourceStatErr := source.Stat()
			written, statErr := f.Stat()
			if err := errors.Join(copyErr, sourceStatErr, statErr, source.Close(), f.Close()); err != nil {
				return err
			}
			if !sameCopyEntry(sourceInfo, entry.info) || n != entry.info.Size() {
				return fmt.Errorf("source changed while copying %q", entry.path)
			}
			if !os.SameFile(opened, written) || written.Size() != n {
				return fmt.Errorf("output changed while writing %q", entry.path)
			}
			created[entry.path] = written
		}
		info, err := target.Lstat(entry.path)
		if err != nil || info.IsDir() != entry.info.IsDir() || (!info.IsDir() && !info.Mode().IsRegular()) {
			return errors.Join(err, fmt.Errorf("unexpected created output %q", entry.path))
		}
		if expected := created[entry.path]; expected != nil && !sameCopyEntry(info, expected) {
			return fmt.Errorf("output identity changed after closing %q", entry.path)
		}
		created[entry.path] = info
		if info.IsDir() {
			held, err := target.OpenRoot(entry.path)
			if err != nil {
				return err
			}
			dirs = append(dirs, heldDirectory{filepath.Join(r.path, entry.path), held, info})
		}
		if err := check(); err != nil {
			return err
		}
	}
	return nil
}
