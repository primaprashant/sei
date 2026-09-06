package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func mutationModel(t *testing.T) browseModel {
	t.Helper()
	cfg := rootFixture(t)
	for _, root := range []string{cfg.Library, cfg.Agents[0].Global, cfg.Agents[0].Local} {
		for _, name := range []string{"a", "b", "c"} {
			browseMkdir(t, filepath.Join(root, name, "nested"))
			writeTestFile(t, filepath.Join(root, name, "nested", "SKILL.md"), root+name)
		}
	}
	m := newBrowseModel(cfg)
	next, cmd := m.Update(startBrowseMsg{})
	m = finishMutationRefresh(t, next.(browseModel), cmd)
	m.focused = 1
	return m
}

func finishMutationRefresh(t *testing.T, m browseModel, cmd tea.Cmd) browseModel {
	t.Helper()
	if cmd == nil {
		t.Fatal("missing refresh command")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) != len(m.panels)+1 {
		t.Fatalf("refresh batch: %v", batch)
	}
	for _, scan := range batch {
		next, extra := m.Update(scan())
		if extra != nil {
			t.Fatal("scan scheduled unexpected work")
		}
		m = next.(browseModel)
	}
	return m
}

func TestRemoveFlow(t *testing.T) {
	for _, tc := range []struct {
		name            string
		panel, selected int
		want            string
	}{{"following", 1, 1, "c"}, {"preceding", 2, 2, "b"}} {
		t.Run(tc.name, func(t *testing.T) {
			m := mutationModel(t)
			m.focused = tc.panel
			p := &m.panels[tc.panel]
			p.selected, p.selectedName = tc.selected, p.entries[tc.selected].name
			target := filepath.Join(p.path, p.selectedName)
			library := removeSnapshot(t, m.config.Library)
			before := removeSnapshot(t, p.path)
			next, cmd := press(m, 'X')
			if cmd != nil || !reflect.DeepEqual(next, m) {
				t.Fatal("uppercase X changed actionable destination state")
			}
			assertRemoveSnapshot(t, p.path, before)
			assertRemoveSnapshot(t, m.config.Library, library)
			m, worker := press(next, 'x')
			if worker == nil || m.active == nil || m.status != m.active.target()+": working" {
				t.Fatalf("remove not started: %+v", m)
			}
			request := *m.active
			if request.id != 1 || request.destination != panelID(tc.panel) || request.selected != tc.selected || filepath.Join(request.path, request.name) != target || request.label != p.label {
				t.Fatalf("wrong captured request: %+v", request)
			}
			m, _ = press(m, tea.KeyUp)
			m, _ = press(m, '0')
			m, _ = press(m, tea.KeyDown)
			if m.focused != 0 || m.panels[0].selectedName != "b" || *m.active != request {
				t.Fatal("busy navigation changed captured target or was disabled")
			}
			if _, err := os.Stat(target); err != nil {
				t.Fatalf("remove ran before command execution: %v", err)
			}
			result := worker().(mutationResult)
			if result.id != request.id || result.err != nil {
				t.Fatalf("remove result: %+v", result)
			}
			setupAbsent(t, target)
			nextModel, refresh := m.Update(result)
			m = nextModel.(browseModel)
			if m.active != nil || m.status != request.target()+": complete" || m.exitError != nil {
				t.Fatalf("completion: %+v", m)
			}
			m = finishMutationRefresh(t, m, refresh)
			if m.panels[tc.panel].selectedName != tc.want || len(m.panels[tc.panel].entries) != 2 || m.panels[0].selectedName != "b" || m.focused != 0 {
				t.Fatalf("refresh selection: %+v", m.panels)
			}
			assertRemoveSnapshot(t, m.config.Library, library)
		})
	}
}

func TestMutationGuard(t *testing.T) {
	for _, kind := range []string{"uppercase", "library", "empty", "blocked", "loading", "error", "missing", "unchecked", "unsafe", "help", "paste", "g", "size", "selection"} {
		t.Run(kind, func(t *testing.T) {
			m := mutationModel(t)
			before := removeSnapshot(t, m.panels[1].path)
			msg := tea.Msg(tea.KeyPressMsg{Code: 'x'})
			p := &m.panels[1]
			switch kind {
			case "uppercase":
				msg = tea.KeyPressMsg{Code: 'X'}
			case "library":
				m.focused = 0
			case "empty":
				p.entries, p.selectedName = nil, ""
			case "blocked":
				p.entries[0].blocked = true
			case "loading":
				p.loading = true
			case "error":
				p.err = errors.New("scan failed")
			case "missing":
				p.missing = true
			case "unchecked":
				p.safetyChecked = false
			case "unsafe":
				p.safetyErr = errors.New("overlap")
			case "help":
				m, _ = press(m, '?')
			case "paste":
				msg = tea.PasteMsg{Content: "xX"}
			case "g":
				m, _ = press(m, 'g')
			case "size":
				m.width = 0
			case "selection":
				p.selectedName = "not-selected"
			}
			next, cmd := m.Update(msg)
			m = next.(browseModel)
			if cmd != nil || m.active != nil || m.nextOperation != 0 || m.pendingGlobal {
				t.Fatalf("guard failed: %+v", m)
			}
			if (kind == "unchecked" || kind == "unsafe") && !strings.Contains(m.status, "Mutation blocked:") {
				t.Fatal("missing safety feedback")
			}
			assertRemoveSnapshot(t, m.panels[1].path, before)
		})
	}
	t.Run("busy and stale", func(t *testing.T) {
		m := mutationModel(t)
		// A different panel can still have a scan in flight when remove starts.
		m.panels[0].loading = true
		oldScan := scanResult{panel: 0, generation: m.panels[0].generation, missing: true}
		oldSafety := rootSafetyMsg{m.safetyGeneration, rootSafety{blocked: make([]error, len(m.panels))}}
		m, worker := press(m, 'x')
		if worker == nil || m.active == nil {
			t.Fatal("missing worker")
		}
		if m.safetyGeneration != oldSafety.generation+1 || m.panels[0].generation != oldScan.generation+1 {
			t.Fatal("mutation did not invalidate in-flight scans and safety")
		}
		for _, msg := range []tea.Msg{tea.KeyPressMsg{Code: 'x'}, tea.KeyPressMsg{Code: 'r'}, startBrowseMsg{}, oldScan, oldSafety, mutationResult{id: 0}, mutationResult{id: m.active.id + 1}, tea.PasteStartMsg{}, tea.PasteMsg{Content: "xXrq"}, tea.PasteEndMsg{}} {
			next, cmd := m.Update(msg)
			if cmd != nil || !reflect.DeepEqual(next, m) {
				t.Fatalf("busy/stale message changed state: %#v", msg)
			}
		}
		result := worker().(mutationResult)
		if result.err != nil {
			t.Fatal(result.err)
		}
		next, refresh := m.Update(result)
		m = next.(browseModel)
		for i, p := range m.panels {
			if !p.loading || p.safetyChecked || p.generation != 3 {
				t.Fatalf("panel %d not refreshed: %+v", i, p)
			}
		}
		for _, msg := range []tea.Msg{oldScan, oldSafety, result} {
			next, cmd := m.Update(msg)
			if cmd != nil || !reflect.DeepEqual(next, m) {
				t.Fatalf("stale completion accepted: %#v", msg)
			}
		}
		m = finishMutationRefresh(t, m, refresh)
		m, worker = press(m, 'x')
		if worker == nil || m.active == nil || m.active.id != 2 {
			t.Fatal("operation ID not advanced")
		}
		next, cmd := m.Update(result)
		if cmd != nil || !reflect.DeepEqual(next, m) {
			t.Fatal("old operation completed new work")
		}
	})
	t.Run("failure refresh", func(t *testing.T) {
		m := mutationModel(t)
		m, _ = press(m, tea.KeyDown)
		m, worker := press(m, 'x')
		if worker == nil || m.active == nil {
			t.Fatal("missing worker")
		}
		request := *m.active
		if err := os.Symlink("absent", filepath.Join(request.path, request.name, "unsafe")); err != nil {
			t.Fatal(err)
		}
		before := removeSnapshot(t, request.path)
		result := worker().(mutationResult)
		if result.err == nil {
			t.Fatal("unsafe tree removed")
		}
		next, refresh := m.Update(result)
		m = finishMutationRefresh(t, next.(browseModel), refresh)
		if m.active != nil || m.exitError != nil || m.pendingQuit || m.panels[1].selectedName != "b" || m.status != request.target()+": "+result.err.Error() {
			t.Fatalf("failure state: %+v", m)
		}
		assertRemoveSnapshot(t, request.path, before)
	})
	for _, fail := range []bool{false, true} {
		for _, quit := range []string{"q", "ctrl-c", "signal"} {
			t.Run(quit+map[bool]string{false: "/success", true: "/failure"}[fail], func(t *testing.T) {
				m := mutationModel(t)
				m, worker := press(m, 'x')
				if worker == nil || m.active == nil {
					t.Fatal("missing worker")
				}
				request := *m.active
				msg := tea.Msg(exitRequestMsg{})
				if quit == "q" {
					msg = tea.KeyPressMsg{Code: 'q'}
				}
				if quit == "ctrl-c" {
					msg = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
				}
				for range 2 {
					next, cmd := m.Update(msg)
					m = next.(browseModel)
					if cmd != nil || !m.pendingQuit || m.active == nil {
						t.Fatal("quit did not wait")
					}
				}
				for _, code := range []rune{'x', 'r', '0', tea.KeyDown} {
					next, cmd := press(m, code)
					if cmd != nil || !reflect.DeepEqual(next, m) {
						t.Fatal("input accepted while quitting")
					}
				}
				if fail {
					if err := os.Symlink("absent", filepath.Join(request.path, request.name, "unsafe")); err != nil {
						t.Fatal(err)
					}
				}
				result := worker().(mutationResult)
				if (result.err != nil) != fail {
					t.Fatalf("worker error: %v", result.err)
				}
				next, cmd := m.Update(result)
				m = next.(browseModel)
				if cmd == nil {
					t.Fatal("completion did not quit")
				}
				if _, ok := cmd().(tea.QuitMsg); !ok || m.active != nil || !m.pendingQuit {
					t.Fatal("completion scheduled work instead of quit")
				}
				if fail {
					if !errors.Is(m.exitError, result.err) || !strings.Contains(m.exitError.Error(), request.target()) || m.status != request.target()+": "+result.err.Error() {
						t.Fatalf("lost exit failure: %v", m.exitError)
					}
				} else if m.exitError != nil || m.status != request.target()+": complete" {
					t.Fatalf("success exit: %v", m.exitError)
				}
			})
		}
	}
}
