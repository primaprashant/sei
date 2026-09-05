package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"golang.org/x/sys/unix"
)

// Include identities and modes as well as bytes, so unlink/recreate is visible.
type removeSnapshotEntry struct {
	info os.FileInfo
	data string
}

func removeSnapshot(t *testing.T, path string) map[string]removeSnapshotEntry {
	t.Helper()
	result := make(map[string]removeSnapshotEntry)
	err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		entry := removeSnapshotEntry{info: info}
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			entry.data = string(data)
		} else if info.Mode()&os.ModeSymlink != 0 {
			entry.data, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}
		result[path] = entry
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertRemoveSnapshot(t *testing.T, path string, want map[string]removeSnapshotEntry) {
	t.Helper()
	got := removeSnapshot(t, path)
	if len(got) != len(want) {
		t.Fatalf("snapshot entry count changed for %s", path)
	}
	for path, before := range want {
		after, ok := got[path]
		if !ok || !os.SameFile(before.info, after.info) || before.info.Mode() != after.info.Mode() || before.data != after.data {
			t.Fatalf("snapshot changed: %s", path)
		}
	}
}

func requireInvalidUTF8Names(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, "raw\xff")
	if err := os.Mkdir(path, 0o700); err != nil {
		if errors.Is(err, unix.EILSEQ) {
			t.Skipf("filesystem rejects invalid UTF-8 names: %v", err)
		}
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveSkill(t *testing.T) {
	for _, name := range []string{"skill", ".hidden", "with spaces", "back\\slash"} {
		t.Run("success/"+name, func(t *testing.T) {
			cfg := rootFixture(t)
			for _, destination := range []panelID{1, 2} {
				base := cfg.Agents[0].Global
				if destination == 2 {
					base = cfg.Agents[0].Local
				}
				browseMkdir(t, base+"/"+name+"/nested/empty")
				writeTestFile(t, base+"/"+name+"/nested/.dot", "destination")
				browseMkdir(t, base+"/other")
				writeTestFile(t, base+"/other/keep", "other")
				browseMkdir(t, cfg.Library+"/"+name)
				writeTestFile(t, cfg.Library+"/"+name+"/source", "source")
				library := removeSnapshot(t, cfg.Library)
				other := removeSnapshot(t, base+"/other")
				if err := removeSkill(cfg, destination, name); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Lstat(base + "/" + name); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("target remains: %v", err)
				}
				assertRemoveSnapshot(t, cfg.Library, library)
				assertRemoveSnapshot(t, base+"/other", other)
			}
		})
	}
	t.Run("raw root", func(t *testing.T) {
		cfg := rootFixture(t)
		requireInvalidUTF8Names(t, cfg.Library)
		name := "raw\xff"
		browseMkdir(t, cfg.Library+"/"+name)
		writeTestFile(t, cfg.Library+"/"+name+"/source", "source")
		library := removeSnapshot(t, cfg.Library)
		for _, base := range []string{cfg.Agents[0].Global, cfg.Agents[0].Local} {
			requireInvalidUTF8Names(t, base)
		}
		for i, base := range []string{cfg.Agents[0].Global, cfg.Agents[0].Local} {
			browseMkdir(t, base+"/"+name+"/nested/empty")
			writeTestFile(t, base+"/"+name+"/nested/.dot", "destination")
			browseMkdir(t, base+"/other")
			writeTestFile(t, base+"/other/keep", "other")
			other := removeSnapshot(t, base+"/other")
			if err := removeSkill(cfg, panelID(i+1), name); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(base + "/" + name); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("target remains: %v", err)
			}
			assertRemoveSnapshot(t, cfg.Library, library)
			assertRemoveSnapshot(t, base+"/other", other)
		}
	})
	t.Run("preflight", func(t *testing.T) {
		for _, kind := range []string{"top file", "top link", "nested link", "dangling link", "FIFO", "library link", "home link", "project link"} {
			t.Run(kind, func(t *testing.T) {
				cfg := rootFixture(t)
				target := cfg.Agents[0].Global + "/skill"
				external := t.TempDir()
				writeTestFile(t, external+"/keep", "external")
				writeTestFile(t, cfg.Library+"/keep", "library")
				switch kind {
				case "top file":
					writeTestFile(t, target, "file")
				case "top link":
					if err := os.Symlink(external, target); err != nil {
						t.Fatal(err)
					}
				default:
					browseMkdir(t, target+"/z/nested")
					writeTestFile(t, target+"/a", "must survive complete preflight")
					linkTarget := external
					switch kind {
					case "dangling link":
						linkTarget += "/absent"
					case "library link":
						linkTarget = cfg.Library
					case "home link":
						linkTarget = cfg.Home
					case "project link":
						linkTarget = cfg.Project
					}
					var err error
					if kind == "FIFO" {
						err = unix.Mkfifo(target+"/z/nested/bad", 0o600)
					} else {
						err = os.Symlink(linkTarget, target+"/z/nested/bad")
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				before := removeSnapshot(t, cfg.Agents[0].Global)
				library := removeSnapshot(t, cfg.Library)
				outside := removeSnapshot(t, external)
				if err := removeSkill(cfg, 1, "skill"); err == nil {
					t.Fatal("unsafe target accepted")
				}
				assertRemoveSnapshot(t, cfg.Agents[0].Global, before)
				assertRemoveSnapshot(t, cfg.Library, library)
				assertRemoveSnapshot(t, external, outside)
			})
		}
	})
	t.Run("invalid selection", func(t *testing.T) {
		cfg := rootFixture(t)
		browseMkdir(t, cfg.Agents[0].Global+"/other")
		before := removeSnapshot(t, cfg.Agents[0].Global)
		for _, name := range []string{"", ".", "..", "a/b", "/other", "a\x00b", "absent"} {
			if err := removeSkill(cfg, 1, name); err == nil {
				t.Errorf("accepted %q", name)
			}
		}
		for _, destination := range []panelID{-1, 0, 3, 99} {
			if err := removeSkill(cfg, destination, "other"); err == nil {
				t.Errorf("accepted destination %d", destination)
			}
		}
		assertRemoveSnapshot(t, cfg.Agents[0].Global, before)
	})
	t.Run("exact case", func(t *testing.T) {
		cfg := rootFixture(t)
		base := cfg.Agents[0].Global
		browseMkdir(t, base+"/Skill")
		before := removeSnapshot(t, base)
		_, aliasErr := os.Stat(base + "/skill")
		t.Logf("actual filesystem case alias lookup: %v", aliasErr == nil)
		if err := removeSkill(cfg, 1, "skill"); err == nil {
			t.Fatal("non-exact spelling accepted")
		}
		assertRemoveSnapshot(t, base, before)
		if errors.Is(aliasErr, os.ErrNotExist) {
			browseMkdir(t, base+"/skill")
			if err := removeSkill(cfg, 1, "skill"); err != nil {
				t.Fatal(err)
			}
			assertRemoveSnapshot(t, base, before)
		}
		if err := removeSkill(cfg, 1, "Skill"); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("protected roots", func(t *testing.T) {
		for _, kind := range []string{"home", "project", "library", "home ancestor", "project ancestor", "library ancestor", "home alias", "library alias"} {
			t.Run(kind, func(t *testing.T) {
				cfg := rootFixture(t)
				target := cfg.Agents[0].Global + "/skill"
				browseMkdir(t, target+"/nested")
				writeTestFile(t, target+"/keep", "protected")
				switch kind {
				case "home":
					cfg.Home = target
				case "project":
					cfg.Project = target
				case "library":
					cfg.Library = target
				case "home ancestor":
					cfg.Home = target + "/nested"
				case "project ancestor":
					cfg.Project = target + "/nested"
				case "library ancestor":
					cfg.Library = target + "/nested"
				case "home alias", "library alias":
					alias := cfg.Home + "/alias"
					if err := os.Symlink(target+"/nested", alias); err != nil {
						t.Fatal(err)
					}
					if kind == "home alias" {
						cfg.Home = alias
					} else {
						cfg.Library = alias
					}
				}
				before := removeSnapshot(t, target)
				if err := removeSkill(cfg, 1, "skill"); err == nil {
					t.Fatal("protected root accepted")
				}
				assertRemoveSnapshot(t, target, before)
			})
		}
	})
	t.Run("unavailable library", func(t *testing.T) {
		for _, kind := range []string{"missing", "unreadable", "unknown"} {
			t.Run(kind, func(t *testing.T) {
				cfg := rootFixture(t)
				browseMkdir(t, cfg.Agents[0].Global+"/skill")
				writeTestFile(t, cfg.Library+"/keep", "library")
				libraryPath := cfg.Library
				before := removeSnapshot(t, libraryPath)
				switch kind {
				case "missing":
					cfg.Library += "/absent/deep"
				case "unreadable":
					if err := os.Chmod(libraryPath, 0o111); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = os.Chmod(libraryPath, 0o755) })
				case "unknown":
					cfg.Library += "/keep/invalid"
				}
				err := removeSkill(cfg, 1, "skill")
				if (err != nil) != (kind == "unknown") {
					t.Fatalf("unexpected removal result: %v", err)
				}
				if kind == "unreadable" {
					if err := os.Chmod(libraryPath, before[libraryPath].info.Mode().Perm()); err != nil {
						t.Fatal(err)
					}
				}
				assertRemoveSnapshot(t, libraryPath, before)
			})
		}
	})
}

func TestRemoveSkillObservedChanges(t *testing.T) {
	for _, kind := range []string{"top replacement", "file replacement", "directory replacement", "in-root link", "external link", "new child", "missing child", "root replacement", "root retarget", "protected retarget", "during deletion"} {
		t.Run(kind, func(t *testing.T) {
			cfg := rootFixture(t)
			base := cfg.Agents[0].Global
			target := base + "/skill"
			browseMkdir(t, target+"/nested")
			writeTestFile(t, target+"/nested/file", "original")
			writeTestFile(t, target+"/nested/second", "second")
			browseMkdir(t, base+"/other")
			writeTestFile(t, base+"/other/file", "other")
			writeTestFile(t, cfg.Library+"/file", "source")
			library := removeSnapshot(t, cfg.Library)
			other := removeSnapshot(t, base+"/other")
			if kind == "root retarget" {
				cfg.Agents[0].Global = base + "-alias"
				if err := os.Symlink(base, cfg.Agents[0].Global); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "protected retarget" {
				alias := cfg.Home + "-alias"
				if err := os.Symlink(cfg.Home, alias); err != nil {
					t.Fatal(err)
				}
				cfg.Home = alias
			}
			calls := 0
			var changed map[string]removeSnapshotEntry
			var savedRoot map[string]removeSnapshotEntry
			err := removeSkillObserved(cfg, 1, "skill", func(_ string) {
				calls++
				if calls != 1 && (kind != "during deletion" || calls != 2) {
					return
				}
				if kind == "during deletion" && calls == 1 {
					return
				}
				rename := func(from, to string) {
					t.Helper()
					if err := os.Rename(from, to); err != nil {
						t.Fatal(err)
					}
				}
				link := func(to, path string) {
					t.Helper()
					if err := os.Symlink(to, path); err != nil {
						t.Fatal(err)
					}
				}
				switch kind {
				case "top replacement":
					rename(target, base+"/saved")
					browseMkdir(t, target)
				case "file replacement":
					rename(target+"/nested/file", base+"/saved")
					writeTestFile(t, target+"/nested/file", "replacement")
				case "directory replacement":
					rename(target+"/nested", base+"/saved")
					browseMkdir(t, target+"/nested")
				case "in-root link", "external link", "during deletion":
					rename(target+"/nested", base+"/saved")
					to := base + "/other"
					if kind == "external link" {
						to = cfg.Library
					}
					link(to, target+"/nested")
				case "new child":
					writeTestFile(t, target+"/new", "unexpected")
				case "missing child":
					rename(target+"/nested/file", base+"/saved")
				case "root replacement":
					rename(base, base+"-saved")
					savedRoot = removeSnapshot(t, base+"-saved")
					browseMkdir(t, target+"/nested")
				case "root retarget":
					if err := os.Remove(cfg.Agents[0].Global); err != nil {
						t.Fatal(err)
					}
					link(cfg.Library, cfg.Agents[0].Global)
				case "protected retarget":
					if err := os.Remove(cfg.Home); err != nil {
						t.Fatal(err)
					}
					link(target+"/nested", cfg.Home)
				}
				changed = removeSnapshot(t, base)
			})
			if err == nil || changed == nil {
				t.Fatalf("change not refused: %v (calls %d)", err, calls)
			}
			assertRemoveSnapshot(t, base, changed)
			assertRemoveSnapshot(t, cfg.Library, library)
			if savedRoot != nil {
				assertRemoveSnapshot(t, base+"-saved", savedRoot)
			}
			if kind != "root replacement" {
				assertRemoveSnapshot(t, base+"/other", other)
			}
		})
	}
}

func TestRemoveSkillInventory(t *testing.T) {
	cfg := rootFixture(t)
	base := cfg.Agents[0].Global
	browseMkdir(t, base+"/skill/nested")
	writeTestFile(t, base+"/skill/nested/.file", "contents")
	root, err := os.OpenRoot(base)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	entries, err := inventorySkill(root, "skill")
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, entry := range entries {
		paths = append(paths, entry.path)
		info, err := os.Lstat(filepath.Join(base, entry.path))
		if err != nil || !os.SameFile(info, entry.info) {
			t.Fatalf("missing inventory identity: %v", err)
		}
	}
	if !reflect.DeepEqual(paths, []string{"skill/nested/.file", "skill/nested", "skill"}) {
		t.Fatalf("not postorder: %v", paths)
	}
}
