package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestStaleSelection(t *testing.T) {
	for _, add := range []bool{false, true} {
		for _, change := range []string{"disappear", "directory", "file", "link", "root", "alias", "destination root", "destination alias", "missing ancestor", "missing alias"} {
			if !add && (change == "missing ancestor" || change == "missing alias") {
				continue
			}
			t.Run(map[bool]string{true: "add", false: "remove"}[add]+"/"+change, func(t *testing.T) {
				m := mutationModel(t)
				if add {
					m.focused = 0
				}
				selected := m.panels[m.focused].path
				changedRoot := selected
				if change == "destination root" || change == "destination alias" || change == "missing alias" {
					changedRoot = m.config.Agents[0].Global
				}
				if change == "alias" || change == "destination alias" || change == "missing alias" {
					alias := changedRoot + "-alias"
					if err := os.Symlink(changedRoot, alias); err != nil {
						t.Fatal(err)
					}
					if add && change == "alias" {
						m.config.Library = alias
					} else {
						m.config.Agents[0].Global = alias
					}
				}
				if change == "missing ancestor" || change == "missing alias" {
					m.config.Agents[0].Global += "/new/deep"
				}
				cfg := m.config
				m = newBrowseModel(cfg)
				next, refresh := m.Update(startBrowseMsg{})
				m = finishMutationRefresh(t, next.(browseModel), refresh)
				key := rune('A')
				if !add {
					m.focused, key = 1, 'X'
				}
				// Capture the raw selection, then change disk before executing work.
				m, worker := press(m, key)
				if worker == nil {
					t.Fatal("no mutation command")
				}
				switch change {
				case "disappear", "directory", "file", "link":
					if err := os.Rename(selected+"/a", selected+"/saved"); err != nil {
						t.Fatal(err)
					}
					switch change {
					case "directory":
						browseMkdir(t, selected+"/a")
					case "file":
						writeTestFile(t, selected+"/a", "replacement")
					case "link":
						if err := os.Symlink(selected+"/saved", selected+"/a"); err != nil {
							t.Fatal(err)
						}
					}
				case "root", "destination root":
					if err := os.Rename(changedRoot, changedRoot+"-saved"); err != nil {
						t.Fatal(err)
					}
					browseMkdir(t, changedRoot+"/a")
				case "alias", "destination alias", "missing alias":
					other := changedRoot + "-other"
					browseMkdir(t, other+"/a")
					if err := os.Remove(changedRoot + "-alias"); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(other, changedRoot+"-alias"); err != nil {
						t.Fatal(err)
					}
				case "missing ancestor":
					browseMkdir(t, filepath.Dir(cfg.Agents[0].Global))
				}
				base := filepath.Dir(cfg.Home)
				before := removeSnapshot(t, base)
				result := worker().(mutationResult)
				if result.err == nil {
					t.Fatal("stale selection accepted")
				}
				assertRemoveSnapshot(t, base, before)
				next, refresh = m.Update(result)
				m = finishMutationRefresh(t, next.(browseModel), refresh)
				if m.active != nil || m.status == "" {
					t.Fatal("failure did not refresh/report")
				}
			})
		}
	}
}

func TestScanGeneration(t *testing.T) {
	m := mutationModel(t)
	oldScan := scanPanel(1, m.panels[1].generation, m.panels[1].path)()
	oldSafety := rootSafetyMsg{m.safetyGeneration, resolveRoots(m.config)}
	m, worker := press(m, 'X')
	for range 3 {
		for _, msg := range []tea.Msg{startBrowseMsg{}, oldScan, oldSafety, mutationResult{id: 99}} {
			next, cmd := m.Update(msg)
			if cmd != nil || !reflect.DeepEqual(next, m) {
				t.Fatalf("busy message changed state: %T", msg)
			}
		}
	}
	result := worker().(mutationResult)
	if result.err != nil {
		t.Fatal(result.err)
	}
	next, refresh := m.Update(result)
	m = finishMutationRefresh(t, next.(browseModel), refresh)
	status := m.status
	for _, msg := range []tea.Msg{oldScan, oldSafety, result} {
		next, cmd := m.Update(msg)
		if cmd != nil || !reflect.DeepEqual(next, m) {
			t.Fatalf("late message changed state: %T", msg)
		}
	}
	if m.panels[1].selectedName != "b" || len(m.panels[1].entries) != 2 || m.status != status {
		t.Fatal("deleted row resurrected or outcome erased")
	}
	m, worker = press(m, 'X')
	if worker == nil || m.active.id != result.id+1 {
		t.Fatal("next operation not independent")
	}
	next, cmd := m.Update(result)
	if cmd != nil || !reflect.DeepEqual(next, m) {
		t.Fatal("previous completion finished newer operation")
	}
}

func TestStaleSelectionRawName(t *testing.T) {
	m := mutationModel(t)
	base := m.config.Agents[0].Global
	raw := "a\n\x1b[31m"
	if err := os.Rename(base+"/a", base+"/"+raw); err != nil {
		t.Fatal(err)
	}
	next, refresh := m.Update(startBrowseMsg{})
	m = finishMutationRefresh(t, next.(browseModel), refresh)
	library := removeSnapshot(t, m.config.Library)
	other := removeSnapshot(t, base+"/b")
	m, worker := press(m, 'X')
	if worker == nil || m.active.name != raw {
		t.Fatal("raw name was not captured")
	}
	if result := worker().(mutationResult); result.err != nil {
		t.Fatal(result.err)
	}
	setupAbsent(t, base+"/"+raw)
	assertRemoveSnapshot(t, m.config.Library, library)
	assertRemoveSnapshot(t, base+"/b", other)
}

func TestObservedChangeMissingAncestor(t *testing.T) {
	for _, stage := range []string{"preflight", "creation"} {
		for _, link := range []bool{false, true} {
			t.Run(stage+map[bool]string{true: "/link", false: "/directory"}[link], func(t *testing.T) {
				cfg := rootFixture(t)
				base := cfg.Agents[0].Global
				cfg.Agents[0].Global += "/new/deep"
				browseMkdir(t, cfg.Library+"/skill")
				writeTestFile(t, cfg.Library+"/skill/file", "source")
				other := base + "-other"
				browseMkdir(t, other)
				var before map[string]removeSnapshotEntry
				change := func() {
					if before != nil {
						return
					}
					if link {
						if err := os.Symlink(other, base+"/new"); err != nil {
							t.Fatal(err)
						}
					} else {
						browseMkdir(t, base+"/new")
					}
					before = removeSnapshot(t, filepath.Dir(base))
				}
				ops := copySkillOps{
					copy: func(w io.Writer, r io.Reader) (int64, error) {
						if stage == "preflight" {
							change()
						}
						return io.Copy(w, r)
					},
					before: func(string) {
						if stage == "creation" {
							change()
						}
					},
				}
				if err := addSkillWithOps(cfg, 1, "skill", ops); err == nil || before == nil {
					t.Fatalf("new ancestor not rejected: %v", err)
				}
				assertRemoveSnapshot(t, filepath.Dir(base), before)
			})
		}
	}
}

// macOS CI must exercise real cross-filesystem lookup, not a folded-name mock.
func caseSensitiveSource(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	if runtime.GOOS != "darwin" {
		return base
	}
	image, mount := base+"/source.dmg", base+"/mount"
	browseMkdir(t, mount)
	for _, args := range [][]string{
		{"create", "-size", "64m", "-fs", "HFSX", "-volname", "sei-test", image},
		{"attach", "-nobrowse", "-mountpoint", mount, image},
	} {
		if output, err := exec.Command("hdiutil", args...).CombinedOutput(); err != nil {
			t.Fatalf("case-sensitive test volume: %v\n%s", err, output)
		}
	}
	t.Cleanup(func() {
		if output, err := exec.Command("hdiutil", "detach", mount).CombinedOutput(); err != nil {
			t.Errorf("detach test volume: %v\n%s", err, output)
		}
	})
	return mount
}

func TestCaseCollision(t *testing.T) {
	cfg := rootFixture(t)
	base := cfg.Agents[0].Global
	writeTestFile(t, base+"/CaseProbe", "probe")
	_, err := os.Lstat(base + "/caseprobe")
	if errors.Is(err, os.ErrNotExist) {
		if runtime.GOOS == "darwin" {
			t.Fatal("native case-collision coverage requires a case-insensitive target volume")
		}
		t.Skip("target filesystem is case-sensitive; native macOS volume coverage required")
	}
	if err != nil {
		t.Fatal(err)
	}
	cfg.Library = caseSensitiveSource(t)
	t.Run("scanned case rename", func(t *testing.T) {
		browseMkdir(t, base+"/skill")
		writeTestFile(t, base+"/skill/keep", "old")
		m := newBrowseModel(cfg)
		next, refresh := m.Update(startBrowseMsg{})
		m = finishMutationRefresh(t, next.(browseModel), refresh)
		m.focused = 1
		m, worker := press(m, 'X')
		if worker == nil || m.active.name != "skill" {
			t.Fatal("missing captured removal")
		}
		if err := os.Rename(base+"/skill", base+"/Skill"); err != nil {
			t.Fatal(err)
		}
		before := removeSnapshot(t, base)
		if result := worker().(mutationResult); result.err == nil {
			t.Fatal("same-identity differently spelled selection accepted")
		}
		assertRemoveSnapshot(t, base, before)
	})
	for _, scenario := range []string{"top", "nested existing", "nested absent", "fresh"} {
		t.Run(scenario, func(t *testing.T) {
			cfg := cfg
			cfg.Agents = append([]agentConfig(nil), cfg.Agents...)
			cfg.Library += "/" + scenario
			cfg.Agents[0].Global += "/" + scenario
			base := cfg.Agents[0].Global
			browseMkdir(t, cfg.Library+"/skill/nested")
			writeTestFile(t, cfg.Library+"/skill/nested/A", "upper")
			writeTestFile(t, cfg.Library+"/skill/nested/a", "lower")
			upper, err := os.Stat(cfg.Library + "/skill/nested/A")
			if err != nil {
				t.Fatal(err)
			}
			lower, err := os.Stat(cfg.Library + "/skill/nested/a")
			if err != nil || os.SameFile(upper, lower) {
				t.Fatalf("source must support distinct case names: %v", err)
			}
			browseMkdir(t, base)
			if scenario == "top" {
				browseMkdir(t, base+"/Skill")
			} else if scenario != "fresh" {
				browseMkdir(t, base+"/skill")
				writeTestFile(t, base+"/skill/keep", "old")
				if scenario == "nested existing" {
					browseMkdir(t, base+"/skill/nested")
					writeTestFile(t, base+"/skill/nested/A", "old upper")
				}
			}
			before, library := removeSnapshot(t, base), removeSnapshot(t, cfg.Library)
			if err := addSkill(cfg, 1, "skill"); err == nil {
				t.Fatal("case collision accepted")
			}
			assertRemoveSnapshot(t, cfg.Library, library)
			if scenario == "top" || scenario == "nested existing" {
				assertRemoveSnapshot(t, base, before)
			} else {
				// With no lookup evidence before removal, exclusive creation must
				// stop at the second alias without overwriting the first output.
				children, err := os.ReadDir(base + "/skill/nested")
				if err != nil || len(children) != 1 {
					t.Fatalf("expected truthful partial output: %v %v", children, err)
				}
				data, err := os.ReadFile(base + "/skill/nested/" + children[0].Name())
				want := "upper"
				if children[0].Name() == "a" {
					want = "lower"
				}
				if err != nil || string(data) != want {
					t.Fatalf("first output overwritten: %q %v", data, err)
				}
				if err := removeSkill(cfg, 1, "skill"); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
