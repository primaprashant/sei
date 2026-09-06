package app

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// Limit actual reads before returning the injected failure; io.Copy still drives
// production output writes and both real files are closed by the copy operation.
type operationFailureReader struct {
	reader io.Reader
	err    error
}

func (r operationFailureReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if errors.Is(err, io.EOF) {
		err = r.err
	}
	return n, err
}

func TestOperationFailure(t *testing.T) {
	for _, replace := range []bool{false, true} {
		for _, kind := range []string{"preflight read", "read", "write", "close", "permission", "removal"} {
			if kind == "removal" && !replace {
				continue
			}
			name := "fresh/" + kind
			if replace {
				name = "replace/" + kind
			}
			t.Run(name, func(t *testing.T) {
				cfg := rootFixture(t)
				source, target := cfg.Library+"/skill", cfg.Agents[0].Global+"/skill"
				browseMkdir(t, source)
				writeTestFile(t, source+"/file", "source bytes")
				if replace {
					browseMkdir(t, target)
					writeTestFile(t, target+"/old-a", "old a")
					writeTestFile(t, target+"/old-b", "old b")
				}
				library := removeSnapshot(t, cfg.Library)
				other := removeSnapshot(t, cfg.Agents[0].Local)
				before := removeSnapshot(t, cfg.Agents[0].Global)
				fault := errors.New("injected " + kind)
				if kind == "permission" {
					fault = os.ErrPermission // Does not depend on uid or host ACLs.
				}
				injected, copies, removes := false, 0, 0
				var output *os.File
				var inputs []*os.File
				ops := copySkillOps{
					copy: func(w io.Writer, r io.Reader) (int64, error) {
						inputs = append(inputs, r.(*os.File))
						if w != io.Discard {
							copies++
						}
						if (kind == "preflight read" && w == io.Discard) || (kind == "read" && w != io.Discard) {
							injected = true
							r = operationFailureReader{io.LimitReader(r, 2), fault}
						}
						return io.Copy(w, r)
					},
					open: func(root *os.Root, path string, flags int, mode os.FileMode) (copySkillFile, error) {
						if kind == "permission" {
							injected = true
							return nil, &os.PathError{Op: "open", Path: path, Err: fault}
						}
						f, err := root.OpenFile(path, flags, mode)
						if err != nil {
							return nil, err
						}
						output = f
						if kind == "write" || kind == "close" {
							injected = true
						}
						return replaceFailureFile{f, kind, fault}, nil
					},
					remove: func(root *os.Root, path string) error {
						removes++
						if kind == "removal" && removes == 2 {
							injected = true
							return fault
						}
						return root.Remove(path)
					},
				}
				err := addSkillWithOps(cfg, 1, "skill", ops)
				if !injected || !errors.Is(err, fault) {
					t.Fatalf("fault not reached/retained: injected=%v err=%v", injected, err)
				}
				if kind != "removal" && !strings.Contains(err.Error(), filepath.Join("skill", "file")) {
					t.Fatalf("failure lost file context: %v", err)
				}
				for _, f := range append(inputs, output) {
					if f != nil {
						if _, err := f.Stat(); !errors.Is(err, os.ErrClosed) {
							t.Fatalf("file not closed: %v", err)
						}
					}
				}
				switch kind {
				case "preflight read":
					assertRemoveSnapshot(t, cfg.Agents[0].Global, before)
					if copies != 0 || removes != 0 {
						t.Fatal("preflight failure changed destination")
					}
				case "removal":
					if copies != 0 || len(removeSnapshot(t, cfg.Agents[0].Global)) != len(before)-1 {
						t.Fatal("partial deletion lost or copy started")
					}
				default:
					setupAbsent(t, target+"/old-a", target+"/old-b")
					if kind == "permission" {
						setupAbsent(t, target+"/file")
						if entries, err := os.ReadDir(target); err != nil || len(entries) != 0 {
							t.Fatalf("empty partial directory: %v, %v", entries, err)
						}
					} else {
						want := "so"
						if kind == "close" {
							want = "source bytes"
						}
						if data, err := os.ReadFile(target + "/file"); err != nil || string(data) != want {
							t.Fatalf("retained output=%q err=%v, want %q", data, err, want)
						}
					}
				}
				assertRemoveSnapshot(t, cfg.Library, library)
				if err := addSkill(cfg, 1, "skill"); err != nil {
					t.Fatalf("retry partial/missing output: %v", err)
				}
				if data, err := os.ReadFile(target + "/file"); err != nil || string(data) != "source bytes" {
					t.Fatalf("retry contents: %q, %v", data, err)
				}
				if err := removeSkill(cfg, 1, "skill"); err != nil {
					t.Fatalf("remove recovered skill: %v", err)
				}
				setupAbsent(t, target)
				assertRemoveSnapshot(t, cfg.Library, library)
				assertRemoveSnapshot(t, cfg.Agents[0].Local, other)
			})
		}
	}
}

func TestErrorPersistence(t *testing.T) {
	for _, add := range []bool{false, true} {
		t.Run(map[bool]string{false: "remove", true: "add"}[add], func(t *testing.T) {
			m := mutationModel(t)
			if add {
				m.focused = 0
			}
			next, _ := m.startMutation(1, add)
			m = next.(browseModel)
			r := *m.active
			library := removeSnapshot(t, m.config.Library)
			fault := errors.New("disk failure\x1b[31m")
			// Real partial removal, driven synchronously at a known entry count.
			calls := 0
			ops := copySkillOps{remove: func(root *os.Root, path string) error {
				calls++
				if calls == 2 {
					return fault
				}
				return root.Remove(path)
			}}
			var err error
			if add {
				err = addSkillWithOps(m.config, r.destination, r.name, ops)
			} else {
				root, openErr := openSkillRoot(m.config, r.destination, r.name)
				if openErr != nil {
					t.Fatal(openErr)
				}
				inventory, inventoryErr := inventorySkill(root, r.name)
				if inventoryErr != nil {
					t.Fatal(inventoryErr)
				}
				held, statErr := root.Stat(".")
				if statErr != nil {
					t.Fatal(statErr)
				}
				err = removeSkillInventory(m.config, r.destination, r.name, root, held, inventory, nil, ops.remove)
				err = errors.Join(err, root.Close())
			}
			if !errors.Is(err, fault) {
				t.Fatalf("partial failure: %v", err)
			}
			m, _ = press(m, '1') // Completion must use the captured global, not local focus.
			next, refresh := m.Update(mutationResult{r.id, err})
			m = next.(browseModel)
			want := r.target() + ": " + err.Error()
			if m.status != want || refresh == nil || !m.panels[1].loading {
				t.Fatalf("failure completion: %q", m.status)
			}
			// Deliver a deterministic failed destination scan; other scans remain real.
			for _, scan := range refresh().(tea.BatchMsg) {
				msg := scan()
				if result, ok := msg.(scanResult); ok && result.panel == 1 {
					result.err = os.ErrPermission
					msg = result // Even a result carrying entries must not expose them.
				}
				next, _ = m.Update(msg)
				m = next.(browseModel)
			}
			p := m.panels[1]
			if p.err == nil || p.missing || p.loading || len(p.entries) != 0 || p.selectedName != "" {
				t.Fatalf("failed refresh retained actionable/stale listing: %+v", p)
			}
			for _, key := range []rune{tea.KeyDown, tea.KeyUp, '0', 'g', '1', '?', tea.KeyDown, tea.KeyEsc} {
				m, _ = press(m, key)
				if m.status != want {
					t.Fatalf("navigation %q erased error: %q", key, m.status)
				}
			}
			m.width = 1000
			if view := m.View().Content; !strings.Contains(view, displayText(want)) || strings.Contains(view, "\x1b[31m") {
				t.Fatal("captured diagnostic missing or not escaped")
			}
			if help := strings.Join(m.helpLines(), "\n"); !strings.Contains(help, displayText(want)) {
				t.Fatal("full failure diagnostic unavailable in help")
			}
			m, refresh = press(m, 'r')
			m = finishMutationRefresh(t, m, refresh)
			if m.status != want || m.panels[1].selectedName != r.name || m.panels[1].err != nil {
				t.Fatal("successful rescan hid error or failed to recover partial listing")
			}
			m.focused = 1
			m, worker := press(m, 'x')
			if worker == nil || m.status == want {
				t.Fatal("explicit removal did not supersede failure")
			}
			result := worker().(mutationResult)
			if result.err != nil {
				t.Fatalf("remove partial output through UI: %v", result.err)
			}
			next, refresh = m.Update(result)
			m = finishMutationRefresh(t, next.(browseModel), refresh)
			if len(m.panels[1].entries) != 2 || !strings.HasSuffix(m.status, ": complete") {
				t.Fatal("recovery result not refreshed")
			}
			setupAbsent(t, filepath.Join(r.path, r.name))
			assertRemoveSnapshot(t, m.config.Library, library)
		})
	}
}

func TestOperationFailurePermission(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses mode-bit permissions; injected permission coverage still runs")
	}
	for _, add := range []bool{false, true} {
		t.Run(map[bool]string{false: "remove partial", true: "replace missing"}[add], func(t *testing.T) {
			cfg := rootFixture(t)
			base := cfg.Agents[0].Global
			target := base + "/skill"
			browseMkdir(t, cfg.Library+"/skill")
			writeTestFile(t, cfg.Library+"/skill/file", "source")
			browseMkdir(t, target)
			writeTestFile(t, target+"/old-a", "a")
			writeTestFile(t, target+"/old-b", "b")
			library := removeSnapshot(t, cfg.Library)
			denied := base
			info, err := os.Stat(denied)
			if err != nil {
				t.Fatal(err)
			}
			restore := func() {
				t.Helper()
				if err := os.Chmod(denied, info.Mode().Perm()); err != nil {
					t.Fatal(err)
				}
			}
			restored := false
			t.Cleanup(func() {
				if !restored {
					restore()
				}
			})
			deny := func() {
				t.Helper()
				if err := os.Chmod(denied, 0o555); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			if add {
				err = addSkillWithOps(cfg, 1, "skill", copySkillOps{before: func(string) {
					calls++
					if calls == 1 {
						setupAbsent(t, target) // Deletion finished before copy starts.
						deny()
					}
				}})
			} else {
				err = removeSkillObserved(cfg, 1, "skill", func(path string) {
					calls++
					if path == "skill" {
						deny()
					}
				})
			}
			if !errors.Is(err, os.ErrPermission) {
				t.Fatalf("expected actual permission failure, got %v", err)
			}
			restore()
			restored = true
			if add {
				setupAbsent(t, target)
			} else if entries, err := os.ReadDir(target); err != nil || len(entries) != 0 {
				t.Fatalf("partial remove: %v, %v", entries, err)
			}
			assertRemoveSnapshot(t, cfg.Library, library)
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

func TestUnavailableRoots(t *testing.T) {
	for _, kind := range []string{"missing library", "unreadable library", "unreadable destination", "missing destination", "empty destination"} {
		t.Run(kind, func(t *testing.T) {
			if strings.HasPrefix(kind, "unreadable") && os.Geteuid() == 0 {
				t.Skip("root bypasses mode-bit permissions; model failed-scan coverage still runs")
			}
			m := mutationModel(t)
			cfg := m.config
			library := removeSnapshot(t, cfg.Library)
			libraryPath := cfg.Library
			path, want := cfg.Agents[0].Global, "Empty"
			panel := 1
			switch kind {
			case "missing library":
				if err := os.Rename(cfg.Library, cfg.Library+"-saved"); err != nil {
					t.Fatal(err)
				}
				libraryPath += "-saved"
				library = removeSnapshot(t, libraryPath)
				panel, want = 0, "Unavailable: library missing"
			case "unreadable library", "unreadable destination":
				if kind == "unreadable library" {
					panel, path = 0, cfg.Library
				}
				info, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				// Search permission retains provable root identity/relationships,
				// but listings are unavailable. Do not weaken root safety checks.
				if err := os.Chmod(path, 0o111); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Chmod(path, info.Mode().Perm()); err != nil {
						t.Error(err)
					}
				})
				want = "Error:"
			case "missing destination", "empty destination":
				for _, name := range []string{"a", "b", "c"} {
					if err := removeSkill(cfg, 1, name); err != nil {
						t.Fatal(err)
					}
				}
				if kind == "missing destination" {
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
					want = "Not created"
				}
			}
			m, refresh := press(m, 'r')
			m = finishMutationRefresh(t, m, refresh)
			p := m.panels[panel]
			if view := panelView(p, panel == 0, 500, 10); !strings.Contains(view, want) {
				t.Fatalf("%s not distinguished: %s", kind, view)
			}
			if len(p.entries) != 0 || p.selectedName != "" || p.safetyErr != nil {
				t.Fatalf("unavailable/empty panel: %+v", p)
			}
			if panel == 0 {
				m.focused = 0
				if next, cmd := press(m, 'a'); cmd != nil || next.active != nil {
					t.Fatal("unavailable library allowed add")
				}
			}
			m.focused = panel
			if next, cmd := press(m, 'x'); cmd != nil || next.active != nil {
				t.Fatal("unavailable/empty panel allowed deletion")
			}
			// A supported destination skill remains removable without library reads.
			m.focused = 2
			m, worker := press(m, 'x')
			if worker == nil {
				t.Fatal("unaffected destination unusable")
			}
			result := worker().(mutationResult)
			if result.err != nil {
				t.Fatalf("safe deletion with unavailable panel: %v", result.err)
			}
			next, refresh := m.Update(result)
			m = finishMutationRefresh(t, next.(browseModel), refresh)
			if len(m.panels[2].entries) != 2 {
				t.Fatal("unaffected panel not refreshed")
			}
			if kind == "unreadable library" {
				// Restore permissions solely for the byte/identity snapshot assertion.
				if err := os.Chmod(cfg.Library, library[cfg.Library].info.Mode().Perm()); err != nil {
					t.Fatal(err)
				}
			}
			assertRemoveSnapshot(t, libraryPath, library)
			if kind == "missing destination" {
				setupAbsent(t, path)
			}
		})
	}
}
