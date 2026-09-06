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

func setupUpdate(t *testing.T, m setupModel, msg tea.Msg) (setupModel, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	return next.(setupModel), cmd
}

func setupQuit(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("missing quit command")
	}
	if msg := cmd(); reflect.TypeOf(msg) != reflect.TypeOf(tea.QuitMsg{}) {
		t.Fatalf("expected quit, got %T", msg)
	}
}

func setupAbsent(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("unexpected write to %q: %v", path, err)
		}
	}
}

func TestSetupDefaultsAndEditing(t *testing.T) {
	m := newSetupModel("/project", "/config.json")
	want := []agentConfig{{"Claude Code", "~/.claude/skills", ".claude/skills"}, {"Codex", "~/.agents/skills", ".agents/skills"}, {"OpenCode", "~/.config/opencode/skills", ".opencode/skills"}}
	if m.Init() != nil || m.cfg.Library != "" || !reflect.DeepEqual(m.cfg.Agents, want) || m.field != 0 || m.height != 24 || m.project != "/project" || m.path != "/config.json" || m.preview || m.busy {
		t.Fatalf("unexpected defaults: %+v", m)
	}
	for field := 0; field < 10; field++ {
		before := m
		m, _ = setupUpdate(t, m, tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
		m, _ = setupUpdate(t, m, tea.KeyPressMsg{Code: 'x', Text: "value\u754c"})
		m, _ = setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})
		if *m.value() != "value" || m.field != field {
			t.Fatalf("field %d edit: %+v", field, m)
		}
		if field > 0 && !reflect.DeepEqual(before.cfg.Agents, want) {
			t.Fatal("Update mutated prior model agents")
		}
		want = append([]agentConfig(nil), m.cfg.Agents...)
		m, _ = setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	}
	if m.field != 0 || !reflect.DeepEqual(newSetupModel("", "").cfg.Agents, setupPresets[:3]) || setupPresets[0].Name != "Claude Code" {
		t.Fatal("navigation did not wrap or presets were mutated")
	}
	for _, key := range []tea.KeyPressMsg{{Code: tea.KeyTab, Mod: tea.ModShift}, {Code: tea.KeyDown}, {Code: tea.KeyUp}} {
		m, _ = setupUpdate(t, m, key)
	}
	if m.field != 9 {
		t.Fatalf("reverse navigation: %d", m.field)
	}
	for _, msg := range []tea.Msg{tea.PasteStartMsg{}, tea.PasteMsg{Content: "unsafe\n"}, tea.PasteEndMsg{}, tea.KeyPressMsg{Code: 'x', Text: "bad\n"}, tea.KeyPressMsg{Code: 'x', Text: "x", Mod: tea.ModAlt}} {
		before := m
		var cmd tea.Cmd
		m, cmd = setupUpdate(t, m, msg)
		if cmd != nil || !reflect.DeepEqual(m, before) {
			t.Fatalf("ignored input %T changed model", msg)
		}
	}
	m, _ = setupUpdate(t, m, tea.WindowSizeMsg{Height: 13, Width: 40})
	if v := m.View(); !v.AltScreen || !strings.Contains(v.Content, "> Local: value") {
		t.Fatalf("focused field not visible: %s", v.Content)
	}
	m, _ = setupUpdate(t, m, tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	m, _ = setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	if *m.value() != "" {
		t.Fatal("backspace on empty field changed it")
	}
}

func TestSetupAsyncValidationAndSave(t *testing.T) {
	cfg, project, path := saveConfigFixture(t)
	m := newSetupModel(project, path)
	m.cfg.Library = cfg.Library
	raw := m.cfg
	m, cmd := setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil || !m.busy || m.preview || m.saved {
		t.Fatalf("validation not pending: %+v", m)
	}
	for _, msg := range []tea.Msg{tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}, tea.KeyPressMsg{Code: tea.KeyTab}, tea.KeyPressMsg{Code: tea.KeyEnter}, tea.KeyPressMsg{Code: 'x', Text: "x"}} {
		next, ignored := setupUpdate(t, m, msg)
		if ignored != nil || !reflect.DeepEqual(next, m) {
			t.Fatal("busy model accepted editing or duplicate work")
		}
	}
	setupAbsent(t, filepath.Dir(filepath.Dir(path)))
	result := cmd().(setupResult)
	if result.err != nil || result.saved || result.resolved.Library != os.Getenv("HOME")+"/library" {
		t.Fatalf("validation result: %+v", result)
	}
	setupAbsent(t, filepath.Dir(filepath.Dir(path)), result.resolved.Library)
	m, cmd = setupUpdate(t, m, result)
	if cmd != nil || m.busy || !m.preview || m.err != nil || !reflect.DeepEqual(m.cfg, raw) {
		t.Fatalf("preview state: %+v", m)
	}
	m, cmd = setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil || !m.busy || m.saved {
		t.Fatal("save did not remain pending")
	}
	setupAbsent(t, path)
	result = cmd().(setupResult)
	if result.err != nil || !result.saved {
		t.Fatalf("save result: %+v", result)
	}
	loaded, missing, err := loadConfig(path)
	if err != nil || missing || !reflect.DeepEqual(loaded, raw) {
		t.Fatalf("raw config round trip: %+v, %v, %v", loaded, missing, err)
	}
	m, cmd = setupUpdate(t, m, result)
	setupQuit(t, cmd)
	if !m.saved || m.busy || m.fatal != nil || !reflect.DeepEqual(m.resolved, result.resolved) {
		t.Fatalf("saved state: %+v", m)
	}
	setupAbsent(t, m.resolved.Library)
	for _, a := range m.resolved.Agents {
		setupAbsent(t, a.Global, a.Local)
	}
}

func TestSetupPreview(t *testing.T) {
	m := newSetupModel("/project", "/config\n.json")
	m.preview, m.height = true, 60
	m.resolved.Library = "/library"
	for i := 0; i < 9; i++ {
		m.resolved.Agents = append(m.resolved.Agents, agentConfig{fmt.Sprintf("Agent%d", i+1), fmt.Sprintf("/global/%d", i), fmt.Sprintf("/project/local/%d", i)})
	}
	view := m.View().Content
	for _, text := range []string{`Config: /config\n.json`, "Library: /library", "Replacement loses local edits and destination-only files: delete first, then copy.", "Deletion is permanent. No confirmation, trash, backup, or rollback; failures may leave partial output.", sharedDiscovery} {
		if !strings.Contains(view, text) {
			t.Errorf("preview missing %q: %s", text, view)
		}
	}
	for i, a := range m.resolved.Agents {
		for _, text := range []string{fmt.Sprintf("%d %s | local %c / global %c | focus %d / g%d", i+1, a.Name, "abcdefhio"[i], "ABCDEFHIO"[i], i+1, i+1), "Global: " + a.Global, "Local: " + a.Local} {
			if !strings.Contains(view, text) {
				t.Errorf("preview missing mapping/path %q", text)
			}
		}
	}
	m, _ = setupUpdate(t, m, tea.WindowSizeMsg{Height: 5})
	m, _ = setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	if m.offset != 0 {
		t.Fatal("scroll escaped top")
	}
	for i := 0; i < 100; i++ {
		m, _ = setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if view := m.View().Content; len(strings.Split(view, "\n")) > 4 || !strings.Contains(view, sharedDiscovery) {
		t.Fatalf("bottom scroll: %s", view)
	}
	m, cmd := setupUpdate(t, m, tea.KeyPressMsg{Code: 'e', Text: "e"})
	if cmd != nil || m.preview || m.offset != 0 {
		t.Fatalf("edit from preview: %+v", m)
	}
}

func TestSetupCancelNoWrites(t *testing.T) {
	for _, preview := range []bool{false, true} {
		for _, msg := range []tea.Msg{tea.KeyPressMsg{Code: tea.KeyEscape}, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}, exitRequestMsg{}} {
			t.Run(fmt.Sprintf("preview=%v/%T/%v", preview, msg, msg), func(t *testing.T) {
				cfg, project, path := saveConfigFixture(t)
				m := newSetupModel(project, path)
				m.cfg = cfg
				if preview {
					var cmd tea.Cmd
					m, cmd = setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
					m, _ = setupUpdate(t, m, cmd())
					if !m.preview {
						t.Fatal("validation failed")
					}
				}
				m, cmd := setupUpdate(t, m, msg)
				setupQuit(t, cmd)
				if m.saved || m.fatal != nil {
					t.Fatalf("cancel state: %+v", m)
				}
				setupAbsent(t, filepath.Dir(filepath.Dir(path)), os.Getenv("HOME")+"/library", os.Getenv("HOME")+"/global", project+"/.example")
			})
		}
	}
}

func TestSetupFailuresAndPendingQuit(t *testing.T) {
	t.Run("validation can be corrected", func(t *testing.T) {
		_, project, path := saveConfigFixture(t)
		m := newSetupModel(project, path)
		m.offset = 4
		m, cmd := setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
		m, cmd = setupUpdate(t, m, cmd())
		if cmd != nil || m.err == nil || m.fatal != nil || m.preview || m.busy || m.offset != 0 || !strings.Contains(m.View().Content, "Error:") {
			t.Fatalf("recoverable validation failure: %+v", m)
		}
		m, _ = setupUpdate(t, m, tea.KeyPressMsg{Code: '~', Text: "~/library"})
		m, cmd = setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
		m, cmd = setupUpdate(t, m, cmd())
		if cmd != nil || m.err != nil || !m.preview {
			t.Fatalf("retry: %+v", m)
		}
		setupAbsent(t, path)
	})
	t.Run("save failure is fatal and preserves competing config", func(t *testing.T) {
		cfg, project, path := saveConfigFixture(t)
		m := newSetupModel(project, path)
		m.cfg = cfg
		m, cmd := setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
		m, _ = setupUpdate(t, m, cmd())
		m, cmd = setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
		if err := saveConfig(cfg, project, path, false); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		result := cmd().(setupResult)
		if !result.saved || !errors.Is(result.err, os.ErrExist) {
			t.Fatalf("save failure: %+v", result)
		}
		m, cmd = setupUpdate(t, m, result)
		setupQuit(t, cmd)
		if m.busy || !errors.Is(m.fatal, os.ErrExist) {
			t.Fatalf("fatal state: %+v", m)
		}
		after, err := os.ReadFile(path)
		if err != nil || string(after) != string(before) {
			t.Fatalf("competing config changed: %v", err)
		}
	})
	for _, save := range []bool{false, true} {
		for _, fail := range []bool{false, true} {
			t.Run(fmt.Sprintf("pending save=%v/fail=%v", save, fail), func(t *testing.T) {
				cfg, project, path := saveConfigFixture(t)
				m := newSetupModel(project, path)
				m.cfg = cfg
				if save {
					var cmd tea.Cmd
					m, cmd = setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
					m, _ = setupUpdate(t, m, cmd())
				} else if fail {
					m.cfg.Library = ""
				}
				m, work := setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
				for _, msg := range []tea.Msg{exitRequestMsg{}, tea.KeyPressMsg{Code: tea.KeyEscape}, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}} {
					var cmd tea.Cmd
					m, cmd = setupUpdate(t, m, msg)
					if cmd != nil || !m.busy || !m.pendingQuit {
						t.Fatalf("quit did not wait: %+v", m)
					}
				}
				if !strings.Contains(m.View().Content, "Working; quit waits for completion.") || !strings.Contains(m.View().Content, "Exit requested; waiting.") {
					t.Fatal("missing pending status")
				}
				if save && fail {
					if err := os.MkdirAll(path, 0o700); err != nil {
						t.Fatal(err)
					}
				}
				result := work().(setupResult)
				if (result.err != nil) != fail {
					t.Fatalf("result: %+v", result)
				}
				m, cmd := setupUpdate(t, m, result)
				setupQuit(t, cmd)
				if m.busy || !m.pendingQuit || m.saved != save || (m.fatal != nil) != (save && fail) {
					t.Fatalf("completion: %+v", m)
				}
				if !save {
					setupAbsent(t, filepath.Dir(filepath.Dir(path)))
				}
				if save && !fail {
					if _, missing, err := loadConfig(path); err != nil || missing {
						t.Fatalf("pending save lost: %v", err)
					}
				}
			})
		}
	}
}
