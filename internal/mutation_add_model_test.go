package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestAddFlow(t *testing.T) {
	// Keep the expected bindings independent of addKeys so remapping is detected.
	for slot, key := range "abcdefhio" {
		for _, global := range []bool{false, true} {
			key := key
			if global {
				key -= 'a' - 'A'
			}
			t.Run(string(key), func(t *testing.T) {
				cfg := rootFixture(t)
				cfg.Agents = nil
				for i := range 9 {
					cfg.Agents = append(cfg.Agents, agentConfig{
						Name:   fmt.Sprintf("Agent%d", i+1),
						Global: filepath.Join(cfg.Home, fmt.Sprintf("global%d", i+1)),
						Local:  filepath.Join(cfg.Project, fmt.Sprintf("local%d", i+1)),
					})
				}
				for _, name := range []string{"a", "b", "c"} {
					browseMkdir(t, filepath.Join(cfg.Library, name, "nested", "empty"))
					writeTestFile(t, filepath.Join(cfg.Library, name, "nested", "SKILL.md"), name+"\x00\xff\r\n")
				}
				for _, agent := range cfg.Agents {
					for _, path := range []string{agent.Global, agent.Local} {
						browseMkdir(t, filepath.Join(path, "z"))
						writeTestFile(t, filepath.Join(path, "z", "keep"), path)
					}
				}
				configPath := filepath.Join(cfg.Home, "config.json")
				persisted := cfg
				persisted.Agents = append([]agentConfig(nil), cfg.Agents...)
				for i := range persisted.Agents {
					persisted.Agents[i].Local = filepath.Base(persisted.Agents[i].Local)
				}
				if err := saveConfig(persisted, cfg.Home, configPath, false); err != nil {
					t.Fatal(err)
				}
				configBefore := removeSnapshot(t, configPath)
				m := newBrowseModel(cfg)
				next, refresh := m.Update(startBrowseMsg{})
				m = finishMutationRefresh(t, next.(browseModel), refresh)
				before := make([]map[string]removeSnapshotEntry, len(m.panels))
				for i, p := range m.panels {
					if !p.safetyChecked || p.safetyErr != nil || p.err != nil || p.loading {
						t.Fatalf("fixture panel %d not ready and safe: %+v", i, p)
					}
					before[i] = removeSnapshot(t, p.path)
				}
				m, _ = press(m, tea.KeyDown)
				destination := 10 + slot
				wantPath, wantLabel := cfg.Agents[slot].Local, cfg.Agents[slot].Name+" / Local"
				if global {
					destination = 1 + slot
					wantPath, wantLabel = cfg.Agents[slot].Global, cfg.Agents[slot].Name+" / Global"
				}
				m, worker := press(m, key)
				survivor := removeSnapshot(t, filepath.Join(wantPath, "z"))
				want := mutationRequest{id: 1, destination: panelID(destination), name: "b", label: wantLabel, path: wantPath, selected: 1, add: true}
				if worker == nil || m.active == nil || *m.active != want || m.nextOperation != 1 || m.status != want.target()+": working" {
					t.Fatalf("wrong captured add: %+v", m)
				}
				for i, p := range m.panels {
					assertRemoveSnapshot(t, p.path, before[i])
				}
				// Exercise unchanged source, source navigation, and destination focus.
				switch slot % 3 {
				case 1:
					m, _ = press(m, tea.KeyDown)
				case 2:
					if global {
						m, _ = press(m, 'g')
					}
					m, _ = press(m, rune('1'+slot))
				}
				focus, sourceName := m.focused, m.panels[0].selectedName
				if *m.active != want {
					t.Fatal("navigation changed captured add")
				}
				result := worker().(mutationResult)
				if result.id != want.id || result.err != nil {
					t.Fatalf("copy: %+v", result)
				}
				next, refresh = m.Update(result)
				m = finishMutationRefresh(t, next.(browseModel), refresh)
				if m.active != nil || m.exitError != nil || m.pendingQuit || m.status != want.target()+": complete" || m.focused != focus || m.panels[0].selectedName != sourceName || m.panels[destination].selectedName != "z" || m.panels[destination].selected != 1 || len(m.panels[destination].entries) != 2 {
					t.Fatalf("completion lost selection/focus: %+v", m)
				}
				for i, p := range m.panels {
					if i != destination {
						assertRemoveSnapshot(t, p.path, before[i])
					}
				}
				assertRemoveSnapshot(t, configPath, configBefore)
				assertRemoveSnapshot(t, filepath.Join(wantPath, "z"), survivor)
				data, err := os.ReadFile(filepath.Join(wantPath, "b", "nested", "SKILL.md"))
				if err != nil || string(data) != "b\x00\xff\r\n" {
					t.Fatalf("wrong copy destination/content: %q, %v", data, err)
				}
				if info, err := os.Stat(filepath.Join(wantPath, "b", "nested", "empty")); err != nil || !info.IsDir() {
					t.Fatalf("missing empty directory: %v", err)
				}
			})
		}
	}
}

func TestAddGuards(t *testing.T) {
	for _, key := range "aA" {
		for _, kind := range []string{"global focus", "local focus", "empty", "blocked", "loading", "error", "missing", "unchecked source", "unsafe source", "unchecked destination", "unsafe destination", "selection", "negative selection", "past selection", "width", "height", "paste", "help", "g consumed", "busy", "pendingQuit"} {
			t.Run(string(key)+"/"+kind, func(t *testing.T) {
				m := mutationModel(t)
				m.focused = 0
				before := removeSnapshot(t, filepath.Dir(m.config.Library))
				msg := tea.Msg(tea.KeyPressMsg{Code: key})
				p := &m.panels[0]
				d := &m.panels[2]
				if key == 'A' {
					d = &m.panels[1]
				}
				switch kind {
				case "global focus":
					m.focused = 1
				case "local focus":
					m.focused = 2
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
				case "unchecked source":
					p.safetyChecked = false
				case "unsafe source":
					p.safetyErr = errors.New("overlap")
				case "unchecked destination":
					d.safetyChecked = false
				case "unsafe destination":
					d.safetyErr = errors.New("overlap")
				case "selection":
					p.selectedName = "wrong"
				case "negative selection":
					p.selected = -1
				case "past selection":
					p.selected = len(p.entries)
				case "width":
					m.width = 0
				case "height":
					m.height = 0
				case "paste":
					msg = tea.PasteMsg{Content: string(key)}
				case "help":
					m, _ = press(m, '?')
				case "g consumed":
					m, _ = press(m, 'g')
				case "busy", "pendingQuit":
					var worker tea.Cmd
					m, worker = press(m, key)
					if worker == nil || m.active == nil {
						t.Fatal("missing initial add")
					}
					if kind == "pendingQuit" {
						m, worker = press(m, 'q')
						if worker != nil || !m.pendingQuit {
							t.Fatal("quit did not wait for add")
						}
					}
				}
				want := m
				want.pendingGlobal = false
				if strings.HasPrefix(kind, "unchecked") || strings.HasPrefix(kind, "unsafe") {
					want.status = "Mutation blocked: root safety is unchecked or unsafe; ? for details"
				}
				next, cmd := m.Update(msg)
				if cmd != nil || !reflect.DeepEqual(next, want) {
					t.Fatalf("guard changed state: got %+v, want %+v", next, want)
				}
				assertRemoveSnapshot(t, filepath.Dir(m.config.Library), before)
			})
		}
	}
	for _, key := range "bcdefhioBCDEFHIO" {
		t.Run("unconfigured/"+string(key), func(t *testing.T) {
			m := mutationModel(t)
			m.focused = 0
			before := removeSnapshot(t, filepath.Dir(m.config.Library))
			next, cmd := press(m, key)
			if cmd != nil || !reflect.DeepEqual(next, m) {
				t.Fatalf("unconfigured slot accepted: %+v", next)
			}
			assertRemoveSnapshot(t, filepath.Dir(m.config.Library), before)
		})
	}
	for _, key := range "aA" {
		for _, late := range []bool{false, true} {
			t.Run(fmt.Sprintf("existing target/%c/late=%t", key, late), func(t *testing.T) {
				m := mutationModel(t)
				m.focused = 0
				m, _ = press(m, tea.KeyDown)
				d := 2
				if key == 'A' {
					d = 1
				}
				target := filepath.Join(m.panels[d].path, "b")
				source := filepath.Join(m.config.Library, "b")
				writeTestFile(t, filepath.Join(source, "nested", "SKILL.md"), "original\n\x00\xff\r\n")
				browseMkdir(t, filepath.Join(source, "nested", "empty"))
				if late {
					if err := os.RemoveAll(target); err != nil {
						t.Fatal(err)
					}
					next, refresh := m.Update(startBrowseMsg{})
					m = finishMutationRefresh(t, next.(browseModel), refresh)
				} else {
					writeTestFile(t, filepath.Join(target, "keep"), "destination only")
				}
				selected, selectedName := m.panels[0].selected, m.panels[0].selectedName
				roots := make([]map[string]removeSnapshotEntry, len(m.panels))
				for i, p := range m.panels {
					roots[i] = removeSnapshot(t, p.path)
				}
				siblings := make(map[string]map[string]removeSnapshotEntry)
				for _, name := range []string{"a", "c"} {
					path := filepath.Join(m.panels[d].path, name)
					siblings[path] = removeSnapshot(t, path)
				}
				before := removeSnapshot(t, filepath.Dir(m.config.Library))
				m, worker := press(m, key)
				if worker == nil || m.active == nil || !m.active.add {
					t.Fatal("missing add worker")
				}
				request := *m.active
				assertRemoveSnapshot(t, filepath.Dir(m.config.Library), before)
				if late {
					// Another writer creates the target after request capture.
					browseMkdir(t, filepath.Join(target, "nested"))
					writeTestFile(t, filepath.Join(target, "keep"), "external destination")
					writeTestFile(t, filepath.Join(target, "nested", "SKILL.md"), "edited destination")
				}
				result := worker().(mutationResult)
				if result.id != request.id || result.err != nil {
					t.Fatalf("existing target not replaced: %+v", result)
				}
				next, refresh := m.Update(result)
				m = finishMutationRefresh(t, next.(browseModel), refresh)
				if m.active != nil || m.exitError != nil || m.pendingQuit || m.focused != 0 || m.panels[0].selected != selected || m.panels[0].selectedName != selectedName || m.status != request.target()+": complete" {
					t.Fatalf("replacement completion: %+v", m)
				}
				for i, p := range m.panels {
					if i != d {
						assertRemoveSnapshot(t, p.path, roots[i])
					}
				}
				for path, snapshot := range siblings {
					assertRemoveSnapshot(t, path, snapshot)
				}
				setupAbsent(t, filepath.Join(target, "keep"))
				copied := removeSnapshot(t, target)
				original := removeSnapshot(t, source)
				if len(copied) != len(original) {
					t.Fatal("replacement tree differs from source")
				}
				for path, entry := range original {
					rel, err := filepath.Rel(source, path)
					if err != nil {
						t.Fatal(err)
					}
					got, ok := copied[filepath.Join(target, rel)]
					if !ok || got.data != entry.data || got.info.Mode().Type() != entry.info.Mode().Type() {
						t.Fatalf("replacement differs at %s", rel)
					}
				}
			})
		}
	}
}
