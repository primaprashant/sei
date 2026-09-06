package app

import (
	"errors"
	"io"
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

func TestReplaceSkill(t *testing.T) {
	for _, destination := range []panelID{1, 2} {
		cfg := rootFixture(t)
		base := cfg.Agents[0].Global
		if destination == 2 {
			base = cfg.Agents[0].Local
		}
		source, target := cfg.Library+"/skill", base+"/skill"
		browseMkdir(t, source+"/was-file/empty")
		writeTestFile(t, source+"/was-dir", "new")
		writeTestFile(t, source+"/same", "identical")
		browseMkdir(t, target+"/was-dir")
		writeTestFile(t, target+"/was-dir/old", "old")
		writeTestFile(t, target+"/was-file", "old")
		writeTestFile(t, target+"/same", "identical")
		library := removeSnapshot(t, cfg.Library)
		for range 2 {
			writeTestFile(t, target+"/destination-only", "delete even when identical")
			if err := addSkill(cfg, destination, "skill"); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(target + "/destination-only"); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("old output retained: %v", err)
			}
			if data, err := os.ReadFile(target + "/was-dir"); err != nil || string(data) != "new" {
				t.Fatalf("directory-to-file replacement: %q %v", data, err)
			}
			if info, err := os.Stat(target + "/was-file/empty"); err != nil || !info.IsDir() {
				t.Fatalf("file-to-directory replacement: %v", err)
			}
			assertRemoveSnapshot(t, cfg.Library, library)
		}
	}
}

func TestReplacePreflight(t *testing.T) {
	for _, kind := range []string{"source read", "target link", "target FIFO", "nested case alias", "source change", "target addition", "target bytes", "target swap", "source late change", "target late addition"} {
		t.Run(kind, func(t *testing.T) {
			cfg := rootFixture(t)
			source, target := cfg.Library+"/skill", cfg.Agents[0].Global+"/skill"
			browseMkdir(t, source+"/nested")
			writeTestFile(t, source+"/nested/file", "source")
			browseMkdir(t, target+"/nested")
			writeTestFile(t, target+"/nested/file", "old bytes")
			writeTestFile(t, target+"/keep", "keep")
			ops := copySkillOps{}
			switch kind {
			case "source read":
				ops.copy = func(io.Writer, io.Reader) (int64, error) { return 0, io.ErrUnexpectedEOF }
			case "target link":
				if err := os.Symlink(source, target+"/nested/link"); err != nil {
					t.Fatal(err)
				}
			case "target FIFO":
				if err := unix.Mkfifo(target+"/nested/fifo", 0o600); err != nil {
					t.Fatal(err)
				}
			case "nested case alias":
				if err := os.Rename(target+"/nested/file", target+"/nested/File"); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Lstat(target + "/nested/file"); errors.Is(err, os.ErrNotExist) {
					t.Skip("filesystem has distinct case names")
				}
			}
			before := removeSnapshot(t, target)
			library := removeSnapshot(t, cfg.Library)
			calls := 0
			ops.beforeRemove = func(string) {
				calls++
				late := kind == "source late change" || kind == "target late addition"
				if (!late && calls != 1) || (late && calls != 2) {
					return
				}
				switch kind {
				case "source change", "source late change":
					writeTestFile(t, source+"/nested/file", "changed source")
					library = removeSnapshot(t, cfg.Library)
				case "target addition", "target late addition":
					writeTestFile(t, target+"/new", "concurrent")
				case "target bytes":
					writeTestFile(t, target+"/nested/file", "changed target")
				case "target swap":
					if err := os.Rename(target, target+"-saved"); err != nil {
						t.Fatal(err)
					}
					browseMkdir(t, target)
					writeTestFile(t, target+"/new", "new tree")
				}
				before = removeSnapshot(t, target)
			}
			copies := 0
			if ops.copy == nil {
				ops.copy = func(w io.Writer, r io.Reader) (int64, error) {
					if w != io.Discard {
						copies++
					}
					return io.Copy(w, r)
				}
			}
			if err := addSkillWithOps(cfg, 1, "skill", ops); err == nil {
				t.Fatal("unsafe replacement accepted")
			}
			if copies != 0 {
				t.Fatal("copied despite incomplete deletion")
			}
			assertRemoveSnapshot(t, target, before)
			assertRemoveSnapshot(t, cfg.Library, library)
		})
	}
}

func TestReplaceSkillHardlinks(t *testing.T) {
	cfg := rootFixture(t)
	source, target := cfg.Library+"/skill", cfg.Agents[0].Global+"/skill"
	browseMkdir(t, source+"/nested")
	browseMkdir(t, target+"/nested")
	writeTestFile(t, source+"/nested/file", "source")
	writeTestFile(t, source+"/nested/other", "other")
	writeTestFile(t, target+"/nested/file", "old bytes")
	if err := os.Link(target+"/nested/file", target+"/nested/other"); err != nil {
		t.Fatal(err)
	}
	library := removeSnapshot(t, cfg.Library)
	if err := addSkill(cfg, 1, "skill"); err != nil {
		t.Fatal(err)
	}
	var first os.FileInfo
	for name, want := range map[string]string{"file": "source", "other": "other"} {
		data, err := os.ReadFile(target + "/nested/" + name)
		if err != nil || string(data) != want {
			t.Fatalf("replacement %q: %q, %v", name, data, err)
		}
		info, err := os.Stat(target + "/nested/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if os.SameFile(info, library[source+"/nested/"+name].info) || (first != nil && os.SameFile(first, info)) {
			t.Fatal("replacement output is not independent")
		}
		first = info
	}
	assertRemoveSnapshot(t, cfg.Library, library)
}

func TestReplacePreflightDestinationObservation(t *testing.T) {
	for _, stage := range []string{"prepare", "prepare missing", "before remove", "after remove", "before copy"} {
		t.Run(stage, func(t *testing.T) {
			cfg := rootFixture(t)
			a, b := cfg.Agents[0].Global, t.TempDir()
			alias := a + "-alias"
			if err := os.Symlink(a, alias); err != nil {
				t.Fatal(err)
			}
			cfg.Agents[0].Global = alias
			browseMkdir(t, cfg.Library+"/skill")
			writeTestFile(t, cfg.Library+"/skill/file", "source")
			writeTestFile(t, a+"/sentinel", "A")
			writeTestFile(t, b+"/sentinel", "B")
			if stage == "prepare missing" {
				cfg.Agents[0].Global += "/new/deep"
			} else {
				browseMkdir(t, a+"/skill")
				writeTestFile(t, a+"/skill/old", "old A")
				if stage == "prepare" {
					browseMkdir(t, b+"/skill")
					writeTestFile(t, b+"/skill/old", "old B")
				}
			}
			beforeA, beforeB := removeSnapshot(t, a), removeSnapshot(t, b)
			library := removeSnapshot(t, cfg.Library)
			changed, copies := false, 0
			retarget := func() {
				if changed {
					return
				}
				changed = true
				if err := os.Remove(alias); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(b, alias); err != nil {
					t.Fatal(err)
				}
			}
			ops := copySkillOps{
				copy: func(w io.Writer, r io.Reader) (int64, error) {
					if w == io.Discard {
						if stage == "prepare" || stage == "prepare missing" {
							retarget()
						}
					} else {
						copies++
					}
					return io.Copy(w, r)
				},
				beforeRemove: func(string) {
					if stage == "before remove" {
						retarget()
					}
				},
				remove: func(root *os.Root, path string) error {
					if err := root.Remove(path); err != nil {
						return err
					}
					if path == "skill" && stage == "after remove" {
						retarget()
					}
					return nil
				},
				before: func(string) {
					if stage == "before copy" {
						retarget()
					}
				},
			}
			if err := addSkillWithOps(cfg, 1, "skill", ops); err == nil {
				t.Fatal("destination retarget accepted")
			}
			if !changed || copies != 0 {
				t.Fatalf("changed=%v copies=%d", changed, copies)
			}
			assertRemoveSnapshot(t, b, beforeB)
			assertRemoveSnapshot(t, cfg.Library, library)
			if stage == "after remove" || stage == "before copy" {
				if _, err := os.Lstat(a + "/skill"); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("deleted A should remain missing: %v", err)
				}
				delete(beforeA, a+"/skill/old")
				delete(beforeA, a+"/skill")
			}
			assertRemoveSnapshot(t, a, beforeA)
		})
	}
}

func TestReplacePreflightDestinationAncestors(t *testing.T) {
	for _, kind := range []string{"ancestor replaced", "missing appeared"} {
		t.Run(kind, func(t *testing.T) {
			cfg := rootFixture(t)
			base := cfg.Agents[0].Global
			cfg.Agents[0].Global += "/parent/root"
			browseMkdir(t, cfg.Library+"/skill")
			writeTestFile(t, cfg.Library+"/skill/file", "source")
			if kind == "ancestor replaced" {
				browseMkdir(t, cfg.Agents[0].Global+"/skill")
				writeTestFile(t, cfg.Agents[0].Global+"/skill/old", "old")
			}
			library := removeSnapshot(t, cfg.Library)
			var before map[string]removeSnapshotEntry
			err := addSkillWithOps(cfg, 1, "skill", copySkillOps{copy: func(w io.Writer, r io.Reader) (int64, error) {
				if w != io.Discard {
					t.Fatal("copy followed changed destination observation")
				}
				if kind == "ancestor replaced" {
					if err := os.Rename(base+"/parent", base+"/saved"); err != nil {
						t.Fatal(err)
					}
					browseMkdir(t, base+"/parent")
					// Preserve the root inode while changing an existing ancestor.
					if err := os.Rename(base+"/saved/root", base+"/parent/root"); err != nil {
						t.Fatal(err)
					}
				} else {
					browseMkdir(t, cfg.Agents[0].Global)
				}
				before = removeSnapshot(t, base)
				return io.Copy(w, r)
			}})
			if err == nil {
				t.Fatal("changed destination ancestors accepted")
			}
			assertRemoveSnapshot(t, base, before)
			assertRemoveSnapshot(t, cfg.Library, library)
		})
	}
}

type replaceFailureFile struct {
	file *os.File
	kind string
	err  error
}

func (f replaceFailureFile) Stat() (os.FileInfo, error) { return f.file.Stat() }

func (f replaceFailureFile) Write(p []byte) (int, error) {
	if f.kind == "write" {
		n, err := f.file.Write(p[:2])
		return n, errors.Join(err, f.err)
	}
	return f.file.Write(p)
}

func (f replaceFailureFile) Close() error {
	err := f.file.Close()
	if f.kind == "close" {
		return errors.Join(err, f.err)
	}
	return err
}

func TestReplaceFailure(t *testing.T) {
	for _, kind := range []string{"remove first", "remove partial", "create", "write", "read", "close", "source close"} {
		t.Run(kind, func(t *testing.T) {
			cfg := rootFixture(t)
			source, target := cfg.Library+"/skill", cfg.Agents[0].Global+"/skill"
			browseMkdir(t, source)
			writeTestFile(t, source+"/file", "source bytes")
			browseMkdir(t, target)
			writeTestFile(t, target+"/old-a", "old a")
			writeTestFile(t, target+"/old-b", "old b")
			library, before := removeSnapshot(t, cfg.Library), removeSnapshot(t, target)
			injected := errors.New("injected " + kind)
			removes, copies, prepared := 0, 0, 0
			ops := copySkillOps{
				remove: func(root *os.Root, path string) error {
					if prepared != 1 {
						t.Fatal("deletion preceded full readable source preparation")
					}
					removes++
					if (kind == "remove first" && removes == 1) || (kind == "remove partial" && removes == 2) {
						return injected
					}
					return root.Remove(path)
				},
				open: func(root *os.Root, path string, flags int, mode os.FileMode) (copySkillFile, error) {
					if kind == "create" {
						return nil, injected
					}
					f, err := root.OpenFile(path, flags, mode)
					if err != nil {
						return nil, err
					}
					return replaceFailureFile{f, kind, injected}, nil
				},
				copy: func(w io.Writer, r io.Reader) (int64, error) {
					if w == io.Discard {
						prepared++
						assertRemoveSnapshot(t, target, before)
						return io.Copy(w, r)
					}
					copies++
					if kind == "read" {
						n, err := io.CopyN(w, r, 2)
						return n, errors.Join(err, injected)
					}
					n, err := io.Copy(w, r)
					if kind == "source close" {
						err = errors.Join(err, r.(*os.File).Close())
					}
					return n, err
				},
			}
			err := addSkillWithOps(cfg, 1, "skill", ops)
			if err == nil || (kind != "source close" && !errors.Is(err, injected)) {
				t.Fatalf("failure lost: %v", err)
			}
			assertRemoveSnapshot(t, cfg.Library, library)
			switch kind {
			case "remove first":
				assertRemoveSnapshot(t, target, before)
			case "remove partial":
				if len(removeSnapshot(t, target)) != len(before)-1 {
					t.Fatal("partial removal not retained")
				}
			default:
				for _, name := range []string{"old-a", "old-b"} {
					if _, err := os.Lstat(target + "/" + name); !errors.Is(err, os.ErrNotExist) {
						t.Fatal("old output retained")
					}
				}
				if kind != "create" {
					want := "source bytes"
					if kind == "read" || kind == "write" {
						want = "so"
					}
					if data, err := os.ReadFile(target + "/file"); err != nil || string(data) != want {
						t.Fatalf("partial output %q: %v", data, err)
					}
				}
			}
			if (kind == "remove first" || kind == "remove partial") && copies != 0 {
				t.Fatal("copy started before deletion completed")
			}
			if kind == "read" || kind == "remove partial" {
				if err := removeSkill(cfg, 1, "skill"); err != nil {
					t.Fatalf("remove partial tree: %v", err)
				}
			}
			if err := addSkill(cfg, 1, "skill"); err != nil {
				t.Fatalf("retry: %v", err)
			}
			if err := removeSkill(cfg, 1, "skill"); err != nil {
				t.Fatalf("remove: %v", err)
			}
			assertRemoveSnapshot(t, cfg.Library, library)
		})
	}
}
