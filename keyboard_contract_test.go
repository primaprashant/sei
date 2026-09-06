package main

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestKeyContract(t *testing.T) {
	for count := 1; count <= 9; count++ {
		for slot, local := range "abcdefhio" {
			for _, global := range []bool{false, true} {
				t.Run(fmt.Sprintf("agents=%d/slot=%d/global=%t", count, slot+1, global), func(t *testing.T) {
					m := navigationModel(count)
					for i := range m.panels {
						m.panels[i].safetyChecked = true
					}
					key, destination := local, count+slot+1
					focus := m
					if global {
						key -= 'a' - 'A'
						destination = slot + 1
						focus, _ = press(focus, 'g')
					}
					focus, cmd := press(focus, rune('1'+slot))
					wantFocus := 0
					if slot < count {
						wantFocus = destination
					}
					if cmd != nil || focus.focused != wantFocus || focus.pendingGlobal {
						t.Fatalf("focus: %+v", focus)
					}
					focus, _ = press(focus, '0')
					if focus.focused != 0 {
						t.Fatal("0 did not focus library")
					}
					for _, context := range []string{"library", "global", "local", "empty", "blocked"} {
						t.Run(context, func(t *testing.T) {
							m := m
							m.panels = append([]browsePanel(nil), m.panels...)
							switch context {
							case "global":
								m.focused = 1
							case "local":
								m.focused = count + 1
							case "empty":
								m.panels[0].entries, m.panels[0].selectedName = nil, ""
							case "blocked":
								m.panels[0].entries = []skillEntry{{name: "a", blocked: true}}
							}
							next, cmd := press(m, key)
							if context != "library" || slot >= count {
								if cmd != nil || !reflect.DeepEqual(next, m) {
									t.Fatal("ineligible add changed state")
								}
								return
							}
							want := mutationRequest{id: 1, destination: panelID(destination), name: "a", label: m.panels[destination].label, path: m.panels[destination].path, add: true}
							if cmd == nil || next.active == nil || *next.active != want {
								t.Fatalf("key %c captured %+v, want %+v", key, next.active, want)
							}
						})
					}
				})
			}
		}
	}
}

func TestInputPrecedence(t *testing.T) {
	for _, busy := range []bool{false, true} {
		for _, help := range []bool{false, true} {
			t.Run(fmt.Sprintf("busy=%t/help=%t", busy, help), func(t *testing.T) {
				m := mutationModel(t)
				var worker tea.Cmd
				if busy {
					m, worker = press(m, 'X')
				}
				m, _ = press(m, '0')
				if help {
					m, _ = press(m, '?')
				}
				m, _ = press(m, 'g')
				for _, msg := range []tea.Msg{struct{}{}, tea.WindowSizeMsg{Width: m.width, Height: m.height}, tea.PasteStartMsg{}, tea.PasteMsg{Content: "\x1baAXrq?g9\x03"}, tea.PasteEndMsg{}} {
					next, cmd := m.Update(msg)
					if cmd != nil || !reflect.DeepEqual(next, m) {
						t.Fatalf("non-key changed pending sequence: %T", msg)
					}
				}
				for _, key := range "0gabcdefhioABCDEFHIOXxr?9" {
					next, cmd := press(m, key)
					want := m
					want.pendingGlobal = false
					if cmd != nil || !reflect.DeepEqual(next, want) {
						t.Fatalf("continuation %c not consumed", key)
					}
				}
				focused, cmd := press(m, '1')
				if cmd != nil || focused.focused != 1 || focused.pendingGlobal || focused.showHelp != help {
					t.Fatal("global focus lost to help/busy")
				}
				escaped, cmd := press(m, tea.KeyEscape)
				if cmd != nil || escaped.showHelp || escaped.pendingGlobal {
					t.Fatal("Esc did not clear help and sequence")
				}
				again, cmd := press(escaped, tea.KeyEscape)
				if cmd != nil || !reflect.DeepEqual(again, escaped) {
					t.Fatal("idle Esc changed state")
				}
				for _, quit := range []tea.KeyPressMsg{{Code: 'q'}, {Code: 'c', Mod: tea.ModCtrl}} {
					next, cmd := m.Update(quit)
					quitting := next.(browseModel)
					if !busy {
						if cmd == nil {
							t.Fatal("pending sequence swallowed quit")
						}
						if _, ok := cmd().(tea.QuitMsg); !ok {
							t.Fatal("idle quit did not exit")
						}
						continue
					}
					if cmd != nil || !quitting.pendingQuit || quitting.active != m.active {
						t.Fatal("busy quit did not wait")
					}
					for _, key := range "abcdefhioABCDEFHIOXr?g19" {
						next, cmd := press(quitting, key)
						if cmd != nil || !reflect.DeepEqual(next, quitting) {
							t.Fatalf("pending quit accepted %c", key)
						}
					}
				}
				if !busy && !help {
					return
				}
				m.pendingGlobal = false
				for _, focus := range []rune{'0', '1'} {
					m, _ = press(m, focus)
					for _, key := range "abcdefhioABCDEFHIOXx" {
						next, cmd := press(m, key)
						if cmd != nil || !reflect.DeepEqual(next, m) {
							t.Fatalf("help/busy accepted %c", key)
						}
					}
				}
				if busy {
					for range 3 {
						next, cmd := press(m, 'r')
						if cmd != nil || !reflect.DeepEqual(next, m) {
							t.Fatal("busy refresh was not deferred")
						}
					}
					m, _ = press(m, '0')
					m, _ = press(m, 'g')
					result := worker().(mutationResult)
					if result.err != nil {
						t.Fatal(result.err)
					}
					next, refresh := m.Update(result)
					m = finishMutationRefresh(t, next.(browseModel), refresh)
					if m.active != nil || m.nextOperation != 1 || !m.pendingGlobal || m.showHelp != help || len(m.panels[1].entries) != 2 || len(m.panels[2].entries) != 3 {
						t.Fatal("busy input replayed after refresh")
					}
					consumed, cmd := press(m, 'a')
					m.pendingGlobal = false
					if cmd != nil || !reflect.DeepEqual(consumed, m) {
						t.Fatal("sequence started busy replayed continuation after completion")
					}
				}
			})
		}
	}
}

func TestSelectionPrimaryWorkflow(t *testing.T) {
	for _, count := range []int{1, 3, 9} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			cfg := rootFixture(t)
			cfg.Agents = nil
			for i := range count {
				cfg.Agents = append(cfg.Agents, agentConfig{Name: fmt.Sprintf("Agent%d", i+1), Global: filepath.Join(cfg.Home, fmt.Sprintf("global%d", i+1)), Local: fmt.Sprintf("local%d", i+1)})
			}
			for _, name := range []string{"a", "b", "c"} {
				browseMkdir(t, filepath.Join(cfg.Library, name))
				writeTestFile(t, filepath.Join(cfg.Library, name, "SKILL.md"), name)
			}
			path := filepath.Join(cfg.Home, "config.json")
			if err := saveConfig(cfg, cfg.Home, path, false); err != nil {
				t.Fatal(err)
			}
			configBefore, libraryBefore := removeSnapshot(t, path), removeSnapshot(t, cfg.Library)
			base := filepath.Dir(cfg.Home)
			expected := removeSnapshot(t, base)
			for _, restart := range []bool{false, true} {
				loaded, missing, err := loadConfig(path)
				if err != nil || missing {
					t.Fatalf("reload: missing=%t, %v", missing, err)
				}
				loaded, err = resolveConfigPaths(loaded, cfg.Project)
				if err != nil {
					t.Fatal(err)
				}
				m := newBrowseModel(loaded)
				next, refresh := m.Update(m.Init()())
				m = finishMutationRefresh(t, next.(browseModel), refresh)
				if m.focused != 0 || m.panels[0].selectedName != "a" || m.showHelp || m.pendingGlobal || m.nextOperation != 0 || m.status != "" {
					t.Fatal("initial state persisted session state")
				}
				destination := count * 2
				if restart {
					if m.panels[destination].selectedName != "a" || len(m.panels[destination].entries) != 1 {
						t.Fatal("filesystem changes did not survive restart")
					}
					assertRemoveSnapshot(t, base, expected)
					continue
				}
				for _, name := range []string{"a", "b", "c"} {
					var worker tea.Cmd
					m, worker = press(m, rune("abcdefhio"[count-1]))
					if worker == nil {
						t.Fatal("missing add")
					}
					result := worker().(mutationResult)
					if result.err != nil {
						t.Fatal(result.err)
					}
					next, refresh := m.Update(result)
					m = finishMutationRefresh(t, next.(browseModel), refresh)
					if m.focused != 0 || m.panels[0].selectedName != name {
						t.Fatal("add moved library selection")
					}
					m, _ = press(m, tea.KeyDown)
				}
				m, _ = press(m, rune('0'+count))
				m, _ = press(m, tea.KeyDown)
				for _, want := range []string{"c", "a"} {
					var worker tea.Cmd
					m, worker = press(m, 'X')
					if worker == nil {
						t.Fatal("missing remove")
					}
					result := worker().(mutationResult)
					if result.err != nil {
						t.Fatal(result.err)
					}
					next, refresh = m.Update(result)
					m = finishMutationRefresh(t, next.(browseModel), refresh)
					if m.panels[destination].selectedName != want || m.panels[0].selectedName != "c" {
						t.Fatal("delete lost following/preceding or independent selection")
					}
				}
				m, _ = press(m, '?')
				_, quit := press(m, 'q')
				if quit == nil {
					t.Fatal("quit ignored")
				}
				if _, ok := quit().(tea.QuitMsg); !ok {
					t.Fatal("quit scheduled work instead of exit")
				}
				assertRemoveSnapshot(t, path, configBefore)
				assertRemoveSnapshot(t, cfg.Library, libraryBefore)
				copied := removeSnapshot(t, m.panels[destination].path)
				if len(copied) != 3 || copied[filepath.Join(m.panels[destination].path, "a", "SKILL.md")].data != "a" {
					t.Fatal("unexpected surviving copy")
				}
				for path, entry := range copied {
					expected[path] = entry
				}
				assertRemoveSnapshot(t, base, expected)
			}
		})
	}
}
