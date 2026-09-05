package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func configSaveTestIO() configSaveIO {
	return configSaveIO{
		create: func(root *os.Root, name string) (*os.File, error) {
			return root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		},
		write:  (*os.File).Write,
		sync:   (*os.File).Sync,
		close:  (*os.File).Close,
		rename: (*os.Root).Rename,
	}
}

// Snapshot directories, regular bytes, links and identities without following
// skill-internal symlinks. Reading files must not make atime part of the oracle.
func configSaveSnapshot(t *testing.T, paths ...string) func() {
	t.Helper()
	type entry struct {
		info os.FileInfo
		data string
	}
	read := func() map[string]entry {
		result := make(map[string]entry)
		for _, path := range paths {
			err := filepath.WalkDir(path, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				info, err := d.Info()
				if err != nil {
					return err
				}
				e := entry{info: info}
				if info.Mode().IsRegular() {
					data, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					e.data = string(data)
				} else if info.Mode()&os.ModeSymlink != 0 {
					e.data, err = os.Readlink(path)
					if err != nil {
						return err
					}
				}
				result[path] = e
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		return result
	}
	before := read()
	return func() {
		t.Helper()
		after := read()
		if len(before) != len(after) {
			t.Fatalf("snapshot entry count changed: %d -> %d", len(before), len(after))
		}
		for path, a := range before {
			b, ok := after[path]
			if !ok || a.data != b.data || !os.SameFile(a.info, b.info) || a.info.Mode() != b.info.Mode() || a.info.Size() != b.info.Size() || !a.info.ModTime().Equal(b.info.ModTime()) {
				t.Errorf("save changed snapshot: %s", path)
			}
		}
	}
}

func populateConfigSaveRoots(t *testing.T, cfg config, project string) []string {
	t.Helper()
	resolved, err := resolveConfigPaths(cfg, project)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{resolved.Library, resolved.Agents[0].Global, resolved.Agents[0].Local}
	for _, path := range paths {
		if err := os.MkdirAll(path+"/skill/nested", 0o700); err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, path+"/skill/SKILL.md", "original skill\n")
		writeTestFile(t, path+"/skill/nested/payload", "\x00payload\xff")
		if err := os.Symlink("nested/payload", path+"/skill/link"); err != nil {
			t.Fatal(err)
		}
	}
	return paths
}

func assertConfigSaveNoTemp(t *testing.T, base string) {
	t.Helper()
	if err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err == nil && strings.HasPrefix(d.Name(), ".sei-config-") {
			t.Errorf("temporary output remains: %s", path)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func TestConfigSaveFailure(t *testing.T) {
	// config contains only strings and slices of string-only structs, with no
	// custom marshaler, interfaces, cycles or floats. Marshal cannot return an
	// error for this schema; invalid UTF-8 is tested before JSON can replace it.
	for _, existing := range []bool{false, true} {
		for _, stage := range []string{"validation", "invalid utf8", "create", "write", "shortwrite", "sync", "close", "rename"} {
			name := "new/" + stage
			if existing {
				name = "replace/" + stage
			}
			t.Run(name, func(t *testing.T) {
				cfg, project, path := saveConfigFixture(t)
				paths := populateConfigSaveRoots(t, cfg, project)
				if existing {
					if err := saveConfig(cfg, project, path, false); err != nil {
						t.Fatal(err)
					}
					paths = append(paths, path)
				}
				unchanged := configSaveSnapshot(t, paths...)
				cfg.Agents[0].Name = "Replacement"
				output := configSaveTestIO()
				injected := errors.New("injected " + stage)
				want := injected
				var file *os.File
				create := output.create
				output.create = func(root *os.Root, name string) (*os.File, error) {
					var err error
					file, err = create(root, name)
					return file, err
				}
				closed, synced, renamed := false, false, false
				output.close = func(f *os.File) error {
					closed = true
					err := f.Close()
					if stage == "close" {
						return errors.Join(err, injected)
					}
					return err
				}
				output.sync = func(f *os.File) error {
					synced = true
					if stage == "sync" {
						return injected
					}
					return f.Sync()
				}
				output.rename = func(*os.Root, string, string) error {
					renamed = true
					return injected
				}
				switch stage {
				case "validation":
					cfg.Library, want = "", nil
				case "invalid utf8":
					cfg.Library, want = "~/bad\xff", nil
				case "create":
					output.create = func(*os.Root, string) (*os.File, error) { return nil, injected }
				case "write", "shortwrite":
					output.write = func(f *os.File, data []byte) (int, error) {
						n, err := f.Write(data[:len(data)/2])
						if stage == "write" {
							err = errors.Join(err, injected)
						}
						return n, err
					}
					if stage == "shortwrite" {
						want = io.ErrShortWrite
					}
				}
				err := saveConfigWithIO(cfg, project, path, existing, output)
				if err == nil || (want != nil && !errors.Is(err, want)) {
					t.Fatalf("save error = %v, want %v", err, want)
				}
				if renamed != (stage == "rename") {
					t.Fatalf("unexpected commit attempt: %v", renamed)
				}
				if file != nil {
					if _, err := file.Stat(); !closed || !errors.Is(err, os.ErrClosed) {
						t.Fatalf("temporary file not closed: %v", err)
					}
				}
				if (stage == "write" || stage == "shortwrite") && synced {
					t.Fatal("synced after failed write")
				}
				unchanged()
				if !existing {
					if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("failed save created config: %v", err)
					}
					if stage == "validation" || stage == "invalid utf8" {
						if _, err := os.Lstat(filepath.Dir(filepath.Dir(path))); !errors.Is(err, os.ErrNotExist) {
							t.Fatalf("validation created parents: %v", err)
						}
					}
				}
				assertConfigSaveNoTemp(t, filepath.Dir(project))
			})
		}
	}
}

func TestConfigSaveSafety(t *testing.T) {
	for _, change := range []string{"config replaced", "config symlink", "parent retarget", "root retarget", "old root retarget", "temp replaced", "temp symlink", "temp edited", "temp chmod"} {
		t.Run(change, func(t *testing.T) {
			cfg, project, path := saveConfigFixture(t)
			home := os.Getenv("HOME")
			paths := populateConfigSaveRoots(t, cfg, project)
			if err := os.Symlink(home+"/library", home+"/root-alias"); err != nil {
				t.Fatal(err)
			}
			cfg.Library = home + "/root-alias"
			if err := saveConfig(cfg, project, path, false); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Dir(path), home+"/parent-alias"); err != nil {
				t.Fatal(err)
			}
			physical := path
			path = home + "/parent-alias/config.json"
			if change == "old root retarget" {
				cfg.Library = home + "/new-library"
			}
			cfg.Agents[0].Name = "Replacement"
			unchanged := configSaveSnapshot(t, paths...)
			original := configSaveSnapshot(t, physical)
			var external func()
			changed := false
			output := configSaveTestIO()
			output.close = func(f *os.File) error {
				if err := f.Close(); err != nil {
					return err
				}
				changed = true
				switch change {
				case "config replaced", "config symlink":
					if err := os.Rename(physical, physical+".original"); err != nil {
						t.Fatal(err)
					}
					if change == "config replaced" {
						writeTestFile(t, physical, "external replacement")
					} else if err := os.Symlink(paths[0]+"/skill/SKILL.md", physical); err != nil {
						t.Fatal(err)
					}
					external = configSaveSnapshot(t, physical, physical+".original")
				case "parent retarget":
					if err := os.Remove(home + "/parent-alias"); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(paths[1], home+"/parent-alias"); err != nil {
						t.Fatal(err)
					}
				case "root retarget", "old root retarget":
					if err := os.Remove(home + "/root-alias"); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(filepath.Dir(physical), home+"/root-alias"); err != nil {
						t.Fatal(err)
					}
				case "temp replaced", "temp symlink", "temp edited", "temp chmod":
					entries, err := os.ReadDir(filepath.Dir(physical))
					if err != nil {
						t.Fatal(err)
					}
					for _, e := range entries {
						if !strings.HasPrefix(e.Name(), ".sei-config-") {
							continue
						}
						temp := filepath.Join(filepath.Dir(physical), e.Name())
						if change == "temp chmod" {
							if err := os.Chmod(temp, 0o644); err != nil {
								t.Fatal(err)
							}
							continue
						}
						if change != "temp edited" {
							// Keep the old inode allocated to make replacement deterministic.
							if err := os.Rename(temp, physical+".held-temp"); err != nil {
								t.Fatal(err)
							}
						}
						if change == "temp symlink" {
							if err := os.Symlink(paths[0]+"/skill/SKILL.md", temp); err != nil {
								t.Fatal(err)
							}
						} else {
							writeTestFile(t, temp, "changed temporary bytes")
						}
					}
				}
				return nil
			}
			output.rename = func(*os.Root, string, string) error {
				t.Fatal("committed after observed change")
				return nil
			}
			if err := saveConfigWithIO(cfg, project, path, true, output); err == nil {
				t.Fatal("observed change accepted")
			}
			if !changed {
				t.Fatal("save failed before the observation hook")
			}
			unchanged()
			if external != nil {
				external()
			} else {
				original()
			}
			assertConfigSaveNoTemp(t, filepath.Dir(project))
		})
	}

	for _, root := range []string{"library", "global", "local"} {
		for _, alias := range []bool{false, true} {
			name := root
			if alias {
				name += " alias"
			}
			t.Run("original placement/"+name, func(t *testing.T) {
				cfg, project, _ := saveConfigFixture(t)
				paths := populateConfigSaveRoots(t, cfg, project)
				index := map[string]int{"library": 0, "global": 1, "local": 2}[root]
				dir := paths[index]
				if alias {
					link := os.Getenv("HOME") + "/alias"
					if err := os.Symlink(dir, link); err != nil {
						t.Fatal(err)
					}
					dir = link
				}
				path := dir + "/config.json"
				data, err := json.Marshal(cfg)
				if err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, path, string(data))
				unchanged := configSaveSnapshot(t, paths...)
				cfg.Library = "~/replacement-library"
				cfg.Agents = []agentConfig{{Name: "New", Global: "~/replacement-global", Local: "replacement-local"}}
				output := configSaveTestIO()
				output.create = func(*os.Root, string) (*os.File, error) {
					t.Fatal("created temp inside original managed root")
					return nil, nil
				}
				if err := saveConfigWithIO(cfg, project, path, true, output); err == nil {
					t.Fatal("editing away original roots permitted unsafe save")
				}
				unchanged()
				assertConfigSaveNoTemp(t, filepath.Dir(project))
			})
		}
	}

	for _, root := range []string{"library", "global", "local"} {
		t.Run("config ancestor/"+root, func(t *testing.T) {
			cfg, project, path := saveConfigFixture(t)
			switch root {
			case "library":
				cfg.Library = path + "/library"
			case "global":
				cfg.Agents[0].Global = path + "/global"
			case "local":
				path = project + "/new/config.json"
				cfg.Agents[0].Local = "new/config.json/local"
			}
			if err := saveConfig(cfg, project, path, false); err == nil {
				t.Fatal("config ancestor of managed root accepted")
			}
			if _, err := os.Lstat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unsafe save created parents: %v", err)
			}
		})
	}
}
