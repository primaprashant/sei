package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"golang.org/x/sys/unix"
)

func TestAddSkill(t *testing.T) {
	for _, name := range []string{"skill", ".hidden", "with spaces", "raw\xff", "back\\slash"} {
		for _, destination := range []panelID{1, 2} {
			t.Run(name+strconv.Itoa(int(destination)), func(t *testing.T) {
				cfg := rootFixture(t)
				base := cfg.Agents[0].Global
				if destination == 2 {
					base = cfg.Agents[0].Local
				}
				browseMkdir(t, cfg.Library+"/"+name+"/nested/empty")
				contents := map[string]string{"SKILL.md": "not parsed\x00\xff\r\n", "nested/.dot": "dot", "nested/back\\slash": "raw", "nested/raw\xfe": "raw filename", ".exec": "#!/bin/sh\n", "empty": ""}
				for path, data := range contents {
					writeTestFile(t, cfg.Library+"/"+name+"/"+path, data)
				}
				if err := os.Chmod(cfg.Library+"/"+name+"/.exec", 0o751); err != nil {
					t.Fatal(err)
				}
				browseMkdir(t, base+"/other")
				writeTestFile(t, base+"/other/keep", "other")
				library := removeSnapshot(t, cfg.Library)
				other := removeSnapshot(t, base+"/other")
				if err := addSkill(cfg, destination, name); err != nil {
					t.Fatal(err)
				}
				for path, want := range contents {
					got, err := os.ReadFile(base + "/" + name + "/" + path)
					if err != nil || string(got) != want {
						t.Fatalf("copy %q: %q, %v", path, got, err)
					}
					info, err := os.Stat(base + "/" + name + "/" + path)
					if err != nil || os.SameFile(info, library[cfg.Library+"/"+name+"/"+path].info) {
						t.Fatalf("not an independent file: %v", err)
					}
				}
				if info, err := os.Stat(base + "/" + name + "/nested/empty"); err != nil || !info.IsDir() {
					t.Fatalf("empty directory: %v", err)
				}
				writeTestFile(t, base+"/"+name+"/nested/.dot", "destination changed")
				assertRemoveSnapshot(t, cfg.Library, library)
				assertRemoveSnapshot(t, base+"/other", other)
				before := removeSnapshot(t, base)
				if err := addSkill(cfg, destination, name); err == nil {
					t.Fatal("existing ordinary directory accepted")
				}
				assertRemoveSnapshot(t, base, before)
			})
		}
	}
	t.Run("missing roots and configured links", func(t *testing.T) {
		cfg := rootFixture(t)
		browseMkdir(t, cfg.Library+"/skill/nested")
		writeTestFile(t, cfg.Library+"/skill/nested/file", "bytes")
		libraryPath := cfg.Library
		library := removeSnapshot(t, cfg.Library)
		alias := cfg.Library + "-alias"
		if err := os.Symlink(cfg.Library, alias); err != nil {
			t.Fatal(err)
		}
		cfg.Library = alias
		destinationAlias := cfg.Agents[0].Global + "-alias"
		if err := os.Symlink(cfg.Agents[0].Global, destinationAlias); err != nil {
			t.Fatal(err)
		}
		cfg.Agents[0].Global = destinationAlias
		cfg.Agents[0].Global += "/new/deep"
		cfg.Agents[0].Local += "/new/deep"
		for _, destination := range []panelID{1, 2} {
			if err := addSkill(cfg, destination, "skill"); err != nil {
				t.Fatal(err)
			}
		}
		assertRemoveSnapshot(t, libraryPath, library)
	})
	t.Run("distinct case names", func(t *testing.T) {
		cfg := rootFixture(t)
		browseMkdir(t, cfg.Library+"/skill/nested")
		writeTestFile(t, cfg.Library+"/skill/nested/File", "upper")
		if _, err := os.Stat(cfg.Library + "/skill/nested/file"); !errors.Is(err, os.ErrNotExist) {
			t.Skip("filesystem aliases case variants")
		}
		writeTestFile(t, cfg.Library+"/skill/nested/file", "lower")
		browseMkdir(t, cfg.Agents[0].Global+"/Skill")
		other := removeSnapshot(t, cfg.Agents[0].Global+"/Skill")
		if err := addSkill(cfg, 1, "skill"); err != nil {
			t.Fatal(err)
		}
		for path, want := range map[string]string{"File": "upper", "file": "lower"} {
			got, err := os.ReadFile(cfg.Agents[0].Global + "/skill/nested/" + path)
			if err != nil || string(got) != want {
				t.Fatalf("distinct name %q: %q, %v", path, got, err)
			}
		}
		assertRemoveSnapshot(t, cfg.Agents[0].Global+"/Skill", other)
	})
	t.Run("source changes while reading", func(t *testing.T) {
		for _, kind := range []string{"replacement", "internal link", "close failure"} {
			t.Run(kind, func(t *testing.T) {
				cfg := rootFixture(t)
				browseMkdir(t, cfg.Library+"/skill/nested")
				writeTestFile(t, cfg.Library+"/skill/nested/file", "original")
				base := cfg.Agents[0].Global
				cfg.Agents[0].Global += "/missing/deep"
				before := removeSnapshot(t, base)
				err := addSkillWithOps(cfg, 1, "skill", copySkillOps{copy: func(w io.Writer, r io.Reader) (int64, error) {
					n, err := io.Copy(w, r)
					if err != nil {
						return n, err
					}
					if kind == "close failure" {
						return n, r.(*os.File).Close()
					}
					path := cfg.Library + "/skill/nested"
					if err := os.Rename(path, cfg.Library+"/saved"); err != nil {
						t.Fatal(err)
					}
					if kind == "internal link" {
						if err := os.Symlink(cfg.Library+"/saved", path); err != nil {
							t.Fatal(err)
						}
					} else {
						browseMkdir(t, path)
						writeTestFile(t, path+"/file", "replacement")
					}
					return n, nil
				}})
				if err == nil {
					t.Fatal("source change or close failure accepted")
				}
				assertRemoveSnapshot(t, base, before)
			})
		}
	})
	t.Run("preflight leaves no output", func(t *testing.T) {
		for _, kind := range []string{"missing source", "missing library", "top file", "top link", "nested link", "in-root link", "dangling link", "FIFO", "unreadable directory", "unreadable file", "read error", "protected source", "overlap", "protected target", "unknown root"} {
			t.Run(kind, func(t *testing.T) {
				cfg := rootFixture(t)
				base := cfg.Agents[0].Global
				cfg.Agents[0].Global += "/missing/deep"
				source := cfg.Library + "/skill"
				browseMkdir(t, source+"/z/nested")
				writeTestFile(t, source+"/a", "must be readable before output")
				ops := copySkillOps{}
				switch kind {
				case "missing source":
					if err := os.Rename(source, source+"-saved"); err != nil {
						t.Fatal(err)
					}
				case "missing library":
					cfg.Library += "/absent"
				case "top file", "top link":
					if err := os.Rename(source, source+"-saved"); err != nil {
						t.Fatal(err)
					}
					if kind == "top file" {
						writeTestFile(t, source, "file")
					} else if err := os.Symlink(source+"-saved", source); err != nil {
						t.Fatal(err)
					}
				case "nested link", "in-root link", "dangling link":
					to := cfg.Home
					switch kind {
					case "in-root link":
						to = source + "/a"
					case "dangling link":
						to += "/absent"
					}
					if err := os.Symlink(to, source+"/z/nested/bad"); err != nil {
						t.Fatal(err)
					}
				case "FIFO":
					if err := unix.Mkfifo(source+"/z/nested/bad", 0o600); err != nil {
						t.Fatal(err)
					}
				case "unreadable directory", "unreadable file":
					if os.Geteuid() == 0 {
						t.Skip("root bypasses permission restrictions")
					}
					path := source + "/a"
					if kind == "unreadable directory" {
						path = source + "/z"
					}
					if err := os.Chmod(path, 0); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = os.Chmod(path, 0o755) })
				case "read error":
					ops.copy = func(io.Writer, io.Reader) (int64, error) { return 0, errors.New("injected read failure") }
				case "protected source":
					cfg.Home = source + "/z/nested"
				case "overlap":
					cfg.Agents[0].Global = source + "/output"
				case "protected target":
					cfg.Home = cfg.Agents[0].Global + "/skill/child"
				case "unknown root":
					cfg.Agents[0].Local = source + "/a/invalid"
				}
				before := removeSnapshot(t, base)
				if err := addSkillWithOps(cfg, 1, "skill", ops); err == nil {
					t.Fatal("invalid source or target accepted")
				}
				assertRemoveSnapshot(t, base, before)
			})
		}
	})
	t.Run("invalid selection", func(t *testing.T) {
		cfg := rootFixture(t)
		browseMkdir(t, cfg.Library+"/skill")
		before := removeSnapshot(t, cfg.Agents[0].Global)
		for _, name := range []string{"", ".", "..", "a/b", "/skill", "a\x00b", "Skill"} {
			if err := addSkill(cfg, 1, name); err == nil {
				t.Errorf("accepted %q", name)
			}
		}
		for _, destination := range []panelID{-1, 0, 3, 99} {
			if err := addSkill(cfg, destination, "skill"); err == nil {
				t.Errorf("accepted destination %d", destination)
			}
		}
		assertRemoveSnapshot(t, cfg.Agents[0].Global, before)
	})
	t.Run("existing targets", func(t *testing.T) {
		for _, kind := range []string{"directory", "file", "link", "dangling", "FIFO", "case alias"} {
			t.Run(kind, func(t *testing.T) {
				cfg := rootFixture(t)
				browseMkdir(t, cfg.Library+"/skill")
				target := cfg.Agents[0].Global + "/skill"
				switch kind {
				case "directory":
					browseMkdir(t, target)
				case "file":
					writeTestFile(t, target, "keep")
				case "link", "dangling":
					to := cfg.Library
					if kind == "dangling" {
						to += "/absent"
					}
					if err := os.Symlink(to, target); err != nil {
						t.Fatal(err)
					}
				case "FIFO":
					if err := unix.Mkfifo(target, 0o600); err != nil {
						t.Fatal(err)
					}
				case "case alias":
					browseMkdir(t, cfg.Agents[0].Global+"/Skill")
					if _, err := os.Lstat(target); errors.Is(err, os.ErrNotExist) {
						t.Skip("filesystem has distinct case-sensitive names")
					}
				}
				before := removeSnapshot(t, cfg.Agents[0].Global)
				library := removeSnapshot(t, cfg.Library)
				if err := addSkill(cfg, 1, "skill"); err == nil {
					t.Fatal("existing target accepted")
				}
				assertRemoveSnapshot(t, cfg.Agents[0].Global, before)
				assertRemoveSnapshot(t, cfg.Library, library)
			})
		}
	})
}

func TestAddSkillStreaming(t *testing.T) {
	for _, kind := range []string{"success", "late read failure", "source close failure", "source edit"} {
		t.Run(kind, func(t *testing.T) {
			cfg := rootFixture(t)
			browseMkdir(t, cfg.Library+"/skill")
			writeTestFile(t, cfg.Library+"/skill/file", "original")
			library := removeSnapshot(t, cfg.Library)
			before := removeSnapshot(t, cfg.Agents[0].Global)
			var prepared, source *os.File
			var output copySkillFile
			preflightCalls, copyCalls := 0, 0
			err := addSkillWithOps(cfg, 1, "skill", copySkillOps{copy: func(w io.Writer, r io.Reader) (int64, error) {
				if w == io.Discard {
					preflightCalls++
					prepared = r.(*os.File)
					assertRemoveSnapshot(t, cfg.Agents[0].Global, before)
					return io.Copy(w, r)
				}
				copyCalls++
				source = r.(*os.File)
				output = w.(copySkillFile)
				if _, err := prepared.Stat(); !errors.Is(err, os.ErrClosed) || source == prepared {
					t.Fatalf("source was not closed and reopened: %v", err)
				}
				if kind == "late read failure" {
					n, err := io.CopyN(w, r, 3)
					return n, errors.Join(err, io.ErrUnexpectedEOF)
				}
				n, err := io.Copy(w, r)
				switch kind {
				case "source close failure":
					err = errors.Join(err, source.Close())
				case "source edit":
					writeTestFile(t, cfg.Library+"/skill/file", "changed source")
				}
				return n, err
			}})
			if (err == nil) != (kind == "success") || preflightCalls != 1 || copyCalls != 1 {
				t.Fatalf("result %v, preflight=%d copy=%d", err, preflightCalls, copyCalls)
			}
			if kind == "late read failure" && !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("lost read failure: %v", err)
			}
			if _, err := source.Stat(); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("source not closed: %v", err)
			}
			if _, err := output.Stat(); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("output not closed: %v", err)
			}
			want := "original"
			if kind == "late read failure" {
				want = "ori"
			}
			got, readErr := os.ReadFile(cfg.Agents[0].Global + "/skill/file")
			if readErr != nil || string(got) != want {
				t.Fatalf("streamed output: %q, %v", got, readErr)
			}
			if kind != "source edit" {
				assertRemoveSnapshot(t, cfg.Library, library)
			}
		})
	}
}

func TestAddSkillObservedChanges(t *testing.T) {
	for _, kind := range []string{"source file", "source child", "source link", "source root", "destination root", "destination child", "protected retarget", "created component", "created target"} {
		t.Run(kind, func(t *testing.T) {
			cfg := rootFixture(t)
			base := cfg.Agents[0].Global
			browseMkdir(t, cfg.Library+"/skill/nested")
			writeTestFile(t, cfg.Library+"/skill/nested/file", "original")
			if kind == "created component" {
				cfg.Agents[0].Global += "/new/deep"
			}
			if kind == "protected retarget" {
				alias := cfg.Home + "-alias"
				if err := os.Symlink(cfg.Home, alias); err != nil {
					t.Fatal(err)
				}
				cfg.Home = alias
			}
			changed := false
			var before map[string]removeSnapshotEntry
			err := addSkillWithOps(cfg, 1, "skill", copySkillOps{before: func(path string) {
				if changed || (kind == "created component" && path != base+"/new/deep") || (kind == "created target" && path != "skill/nested") {
					return
				}
				// The initial whole-root hook precedes creation of new/deep.
				if kind == "created component" {
					if _, err := os.Stat(base + "/new"); errors.Is(err, os.ErrNotExist) {
						return
					}
				}
				changed = true
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
				case "source file":
					writeTestFile(t, cfg.Library+"/skill/nested/file", "changed bytes")
				case "source child":
					writeTestFile(t, cfg.Library+"/skill/new", "new")
				case "source link":
					rename(cfg.Library+"/skill/nested", cfg.Library+"/saved")
					link(cfg.Library+"/saved", cfg.Library+"/skill/nested")
				case "source root":
					rename(cfg.Library, cfg.Library+"-saved")
					browseMkdir(t, cfg.Library+"/skill")
				case "destination root":
					rename(base, base+"-saved")
					browseMkdir(t, base)
				case "destination child":
					browseMkdir(t, base+"/skill")
					writeTestFile(t, base+"/skill/keep", "existing")
				case "protected retarget":
					if err := os.Remove(cfg.Home); err != nil {
						t.Fatal(err)
					}
					link(cfg.Library+"/skill/nested", cfg.Home)
				case "created component":
					rename(base+"/new", base+"/saved")
					link(cfg.Library, base+"/new")
				case "created target":
					rename(base+"/skill", base+"/saved")
					link(cfg.Library+"/skill", base+"/skill")
				}
				before = removeSnapshot(t, filepath.Dir(base))
			}})
			if err == nil || !changed {
				t.Fatalf("change not rejected: %v, changed=%v", err, changed)
			}
			assertRemoveSnapshot(t, filepath.Dir(base), before)
		})
	}
}

func TestCopyPermissions(t *testing.T) {
	if mask := os.Getenv("SEI_COPY_TEST_UMASK"); mask != "" {
		value, err := strconv.ParseInt(mask, 8, 32)
		if err != nil {
			t.Fatal(err)
		}
		cfg := rootFixture(t)
		browseMkdir(t, cfg.Library+"/skill/nested")
		for _, mode := range []os.FileMode{0o400, 0o644, 0o751, 0o710, 0o777} {
			path := cfg.Library + "/skill/nested/" + strconv.FormatUint(uint64(mode), 8)
			writeTestFile(t, path, "contents")
			if err := os.Chmod(path, mode); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Chmod(cfg.Library+"/skill/nested", 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(cfg.Library+"/skill/nested", 0o700) })
		base := cfg.Agents[0].Global
		cfg.Agents[0].Global += "/new/deep"
		library := removeSnapshot(t, cfg.Library)
		unix.Umask(int(value))
		if err := addSkill(cfg, 1, "skill"); err != nil {
			t.Fatal(err)
		}
		for path, entry := range removeSnapshot(t, base+"/new") {
			want := os.FileMode(0o777)
			if !entry.info.IsDir() {
				mode, err := strconv.ParseUint(filepath.Base(path), 8, 32)
				if err != nil {
					t.Fatal(err)
				}
				want = 0o666 | os.FileMode(mode)&0o111
			}
			want &^= os.FileMode(value)
			if entry.info.Mode().Perm() != want {
				t.Errorf("%s: mode %o, want %o", path, entry.info.Mode().Perm(), want)
			}
		}
		assertRemoveSnapshot(t, cfg.Library, library)
		return
	}
	for _, mask := range []string{"000", "022", "027", "077"} {
		t.Run(mask, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestCopyPermissions$", "-test.count=1")
			cmd.Env = append(os.Environ(), "SEI_COPY_TEST_UMASK="+mask)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("isolated umask %s: %v\n%s", mask, err, output)
			}
		})
	}
}
