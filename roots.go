package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type rootAncestor struct {
	path string
	info os.FileInfo
}

type resolvedRoot struct {
	path      string
	ancestor  string
	missing   []string
	ancestors []rootAncestor
	err       error
}

// resolveRoot traverses raw components before doing any lexical normalization.
// A missing suffix containing .. is unprovable until its directories exist.
func resolveRoot(raw string) (r resolvedRoot) {
	if !filepath.IsAbs(raw) || strings.ContainsRune(raw, 0) {
		r.err = fmt.Errorf("root %q must be absolute and contain no NUL", raw)
		return r
	}
	current := string(filepath.Separator)
	for _, component := range strings.Split(raw, string(filepath.Separator)) {
		if component == "" {
			continue
		}
		if len(r.missing) > 0 {
			if component == ".." {
				r.err = fmt.Errorf("cannot traverse .. after missing directory in %q", raw)
				return r
			}
			if component != "." {
				r.missing = append(r.missing, component)
			}
			continue
		}
		candidate := current + "/" + component
		info, err := os.Lstat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			r.missing = append(r.missing, component)
			continue
		}
		if err != nil {
			r.err = err
			return r
		}
		if info.Mode()&os.ModeSymlink != 0 {
			// Stat distinguishes a dangling link from an ordinary absent suffix.
			info, err = os.Stat(candidate)
			if err == nil {
				candidate, err = filepath.EvalSymlinks(candidate)
			}
			if err != nil {
				r.err = err
				return r
			}
		}
		if !info.IsDir() {
			r.err = fmt.Errorf("%q is not a directory", candidate)
			return r
		}
		current = filepath.Clean(candidate)
	}
	r.ancestor = current
	r.path = current
	for _, component := range r.missing {
		r.path = filepath.Join(r.path, component)
	}
	for path := current; ; path = filepath.Dir(path) {
		info, err := os.Stat(path)
		if err != nil {
			r.err = err
			return r
		}
		if !info.IsDir() {
			r.err = fmt.Errorf("ancestor %q is not a directory", path)
			return r
		}
		r.ancestors = append(r.ancestors, rootAncestor{path, info})
		if path == filepath.Dir(path) {
			break
		}
	}
	return r
}

func pathWithin(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// rootWithin supplements component comparisons with existing ancestor identity.
// Relative tails at an identical ancestor also cover case and mount aliases.
func rootWithin(parent, child resolvedRoot) bool {
	if pathWithin(parent.path, child.path) {
		return true
	}
	for _, a := range parent.ancestors {
		for _, b := range child.ancestors {
			if os.SameFile(a.info, b.info) {
				p, _ := filepath.Rel(a.path, parent.path)
				c, _ := filepath.Rel(b.path, child.path)
				if pathWithin(p, c) {
					return true
				}
			}
		}
	}
	return false
}

type rootSafety struct {
	roots     []resolvedRoot // Same order as browser panels.
	blocked   []error
	protected []resolvedRoot // Home, project, and the active config's locations.
}

// Represent a protected file using its physical parent and final component;
// resolveRoot itself intentionally accepts directories only.
func protectedConfigFile(path string) resolvedRoot {
	dir, name := filepath.Split(path)
	r := resolveRoot(dir)
	if r.err == nil {
		r.path = filepath.Join(r.path, name)
	}
	return r
}

// resolveRoots is a fresh observation, never a mutation authorization token.
// Unknown relationships block only operations which depend on those boundaries.
func resolveRoots(cfg config) rootSafety {
	paths := []string{cfg.Library}
	for _, a := range cfg.Agents {
		paths = append(paths, a.Global)
	}
	for _, a := range cfg.Agents {
		paths = append(paths, a.Local)
	}
	s := rootSafety{roots: make([]resolvedRoot, len(paths)), blocked: make([]error, len(paths))}
	for i, path := range paths {
		s.roots[i] = resolveRoot(path)
		s.blocked[i] = s.roots[i].err
	}
	s.protected = []resolvedRoot{resolveRoot(cfg.Home), resolveRoot(cfg.Project)}
	if cfg.ConfigPath != "" {
		s.protected = append(s.protected, protectedConfigFile(cfg.ConfigPath))
		info, err := os.Lstat(cfg.ConfigPath)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			target, err := filepath.EvalSymlinks(cfg.ConfigPath)
			if err != nil {
				s.protected = append(s.protected, resolvedRoot{err: err})
			} else {
				s.protected = append(s.protected, protectedConfigFile(target))
			}
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			s.protected = append(s.protected, resolvedRoot{err: err})
		}
	}
	for i := 1; i < len(paths); i++ {
		for _, p := range s.protected {
			if p.err != nil {
				s.blocked[i] = errors.Join(s.blocked[i], fmt.Errorf("protected root safety is unprovable: %w", p.err))
			}
		}
		for j := 0; j < len(paths); j++ {
			if i == j {
				continue
			}
			a, b := s.roots[i], s.roots[j]
			if b.err != nil {
				s.blocked[i] = errors.Join(s.blocked[i], fmt.Errorf("cannot establish relationship to %q: %w", paths[j], b.err))
			} else if a.err == nil && (rootWithin(a, b) || rootWithin(b, a)) {
				s.blocked[i] = errors.Join(s.blocked[i], fmt.Errorf("root overlaps %q", paths[j]))
			}
		}
		if i > len(cfg.Agents) {
			project := s.protected[1]
			if project.err != nil || s.roots[i].err != nil || !pathWithin(cfg.Project, paths[i]) || !rootWithin(project, s.roots[i]) {
				s.blocked[i] = errors.Join(s.blocked[i], fmt.Errorf("local root containment is unprovable"))
			}
		}
	}
	return s
}

func validateSkillName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\x00") || strings.ContainsRune(name, filepath.Separator) {
		return fmt.Errorf("skill name %q must be one nonempty filename component", name)
	}
	return nil
}

// revalidateSkillRoot must run in each future mutation command, and again after
// destination creation. It does not preflight a tree or authorize copy/removal.
func revalidateSkillRoot(cfg config, destination panelID, name string) (resolvedRoot, error) {
	if err := validateSkillName(name); err != nil {
		return resolvedRoot{}, err
	}
	s := resolveRoots(cfg)
	if destination <= 0 || int(destination) >= len(s.roots) {
		return resolvedRoot{}, fmt.Errorf("invalid destination")
	}
	if err := s.blocked[destination]; err != nil {
		return resolvedRoot{}, err
	}
	root := s.roots[destination]
	info, err := os.Lstat(root.path + "/" + name)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return resolvedRoot{}, err
	}
	if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
		return resolvedRoot{}, fmt.Errorf("skill %q is not an ordinary directory", name)
	}
	child := resolveRoot(root.path + "/" + name)
	if child.err != nil {
		return resolvedRoot{}, child.err
	}
	if rootWithin(child, s.roots[0]) {
		return resolvedRoot{}, fmt.Errorf("skill %q is the library or a library ancestor", name)
	}
	protected := append(s.protected, s.roots...)
	for _, p := range protected {
		if p.err != nil {
			return resolvedRoot{}, fmt.Errorf("protected root safety is unprovable: %w", p.err)
		}
		if rootWithin(child, p) {
			return resolvedRoot{}, fmt.Errorf("skill %q is a protected root or protected-root ancestor", name)
		}
	}
	return root, nil
}

// openSkillRoot returns an owned handle only for an existing, freshly validated
// destination. The caller must close it and still preflight the selected tree.
func openSkillRoot(cfg config, destination panelID, name string) (*os.Root, error) {
	r, err := revalidateSkillRoot(cfg, destination, name)
	if err != nil {
		return nil, err
	}
	if len(r.missing) != 0 {
		return nil, fmt.Errorf("destination is not created; revalidate after creation")
	}
	root, err := os.OpenRoot(r.path)
	if err != nil {
		return nil, err
	}
	info, err := root.Stat(".")
	if err == nil && !os.SameFile(info, r.ancestors[0].info) {
		err = fmt.Errorf("destination changed while opening")
	}
	if err != nil {
		return nil, errors.Join(err, root.Close())
	}
	return root, nil
}
