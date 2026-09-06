package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestSetupPresetPaths(t *testing.T) {
	want := []agentConfig{
		{"Claude Code", "~/.claude/skills", ".claude/skills"},
		{"Codex", "~/.agents/skills", ".agents/skills"},
		{"OpenCode", "~/.config/opencode/skills", ".opencode/skills"},
		{"Pi", "~/.pi/agent/skills", ".pi/skills"},
		{"Cursor", "~/.cursor/skills", ".cursor/skills"},
		{"Antigravity CLI", "~/.gemini/antigravity-cli/skills", ".agents/skills"},
		{"Crush", "~/.config/crush/skills", ".crush/skills"},
		{"GitHub Copilot CLI", "~/.copilot/skills", ".github/skills"},
		{"Cline CLI", "~/.cline/skills", ".cline/skills"},
	}
	if !reflect.DeepEqual(setupPresets, want) {
		t.Fatalf("preset paths changed: %+v", setupPresets)
	}
	for _, preset := range setupPresets {
		t.Run(preset.Name, func(t *testing.T) {
			cfg, project, path := saveConfigFixture(t)
			cfg.Library = "~/skill-library"
			cfg.Agents = []agentConfig{preset}
			resolved, err := validateSetupConfig(cfg, project, path)
			if err != nil {
				t.Fatal(err)
			}
			if resolved.Agents[0].Global != filepath.Join(os.Getenv("HOME"), preset.Global[2:]) || resolved.Agents[0].Local != filepath.Join(project, preset.Local) {
				t.Fatalf("unexpected destinations: %+v", resolved.Agents)
			}
			setupAbsent(t, path, resolved.Library, resolved.Agents[0].Global, resolved.Agents[0].Local)
		})
	}
	t.Run("shared Codex and Antigravity destination remains rejected", func(t *testing.T) {
		cfg, project, path := saveConfigFixture(t)
		cfg.Agents = []agentConfig{setupPresets[1], setupPresets[5]}
		if _, err := validateSetupConfig(cfg, project, path); err == nil || !strings.Contains(err.Error(), "overlap") {
			t.Fatalf("expected shared destination error, got %v", err)
		}
		setupAbsent(t, path, filepath.Join(project, ".agents"))
	})
}

func TestSetupAgents(t *testing.T) {
	ctrl := func(code rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl} }
	text := func(s string) tea.KeyPressMsg { return tea.KeyPressMsg{Code: rune(s[0]), Text: s} }
	t.Run("old shortcuts are ignored and plain letters remain editable", func(t *testing.T) {
		m := newSetupModel("", "")
		m.field = 1
		before := m
		for _, code := range []rune{'a', 'd'} {
			next, cmd := setupUpdate(t, m, ctrl(code))
			if cmd != nil || !reflect.DeepEqual(next, before) {
				t.Fatalf("Ctrl+%c changed setup", code)
			}
		}
		for _, letter := range []string{"n", "x"} {
			m, _ = setupUpdate(t, m, text(letter))
		}
		if m.adding || len(m.cfg.Agents) != len(before.cfg.Agents) || m.cfg.Agents[0].Name != before.cfg.Agents[0].Name+"nx" {
			t.Fatal("plain n/x did not edit the field")
		}
	})
	t.Run("library focus does not remove or reorder agents", func(t *testing.T) {
		m := newSetupModel("", "")
		before := append([]agentConfig(nil), m.cfg.Agents...)
		for _, key := range []rune{'x', 'k', 'j'} {
			m, cmd := setupUpdate(t, m, ctrl(key))
			if cmd != nil || m.field != 0 || !reflect.DeepEqual(m.cfg.Agents, before) {
				t.Fatal("library focus changed agents")
			}
		}
	})
	t.Run("custom duplicate name fails preview validation", func(t *testing.T) {
		cfg, project, path := saveConfigFixture(t)
		m := newSetupModel(project, path)
		m.cfg = cfg
		m, _ = setupUpdate(t, m, ctrl('n'))
		m, _ = setupUpdate(t, m, text("0"))
		for _, value := range []string{cfg.Agents[0].Name, "~/other", ".other/skills"} {
			m, _ = setupUpdate(t, m, text(value))
			m, _ = setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
		}
		m, cmd := setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
		m, cmd = setupUpdate(t, m, cmd())
		if cmd != nil || m.preview || m.busy || m.err == nil || !strings.Contains(m.err.Error(), "duplicate") {
			t.Fatalf("custom duplicate accepted: %+v", m)
		}
		setupAbsent(t, path, os.Getenv("HOME")+"/other", project+"/.other")
	})
	t.Run("chooser presets duplicate and dismissal", func(t *testing.T) {
		for i, preset := range setupPresets {
			m := newSetupModel("", "")
			m.cfg.Agents = []agentConfig{{Name: "Other"}}
			m, _ = setupUpdate(t, m, ctrl('n'))
			if !m.adding || !strings.Contains(m.View().Content, "5  Cursor") || !strings.Contains(m.View().Content, "0  Custom") {
				t.Fatal("missing chooser")
			}
			before := m.cfg.Agents
			m, cmd := setupUpdate(t, m, text(fmt.Sprint(i+1)))
			if cmd != nil || m.adding || m.field != 4 || len(m.cfg.Agents) != 2 || m.cfg.Agents[1] != preset || len(before) != 1 {
				t.Fatalf("preset %d: %+v", i+1, m)
			}
			m, _ = setupUpdate(t, m, ctrl('n'))
			m, _ = setupUpdate(t, m, text(fmt.Sprint(i+1)))
			if m.err == nil || !strings.Contains(m.err.Error(), "duplicate") || len(m.cfg.Agents) != 2 {
				t.Fatalf("duplicate accepted: %+v", m)
			}
		}
		for _, key := range []tea.KeyPressMsg{text("x"), text("10"), tea.KeyPressMsg{Code: tea.KeyEnter}} {
			m := newSetupModel("", "")
			before := m.cfg
			m, _ = setupUpdate(t, m, ctrl('n'))
			m, cmd := setupUpdate(t, m, key)
			if cmd != nil || m.adding || !reflect.DeepEqual(m.cfg, before) {
				t.Fatal("invalid chooser input changed config")
			}
		}
	})
	t.Run("custom paths reorder mapping and bounds", func(t *testing.T) {
		cfg, project, path := saveConfigFixture(t)
		m := newSetupModel(project, path)
		m.cfg = cfg
		m.field = 3
		m, _ = setupUpdate(t, m, ctrl('x'))
		if len(m.cfg.Agents) != 1 || m.err == nil || m.field != 3 {
			t.Fatal("removed last agent")
		}
		for i := 2; i <= 9; i++ {
			m, _ = setupUpdate(t, m, ctrl('n'))
			m, _ = setupUpdate(t, m, text("0"))
			if m.field != 1+3*(i-1) || m.cfg.Agents[i-1] != (agentConfig{}) {
				t.Fatal("custom not blank or focused")
			}
			for _, value := range []string{fmt.Sprintf("Custom%d", i), fmt.Sprintf("~/custom%d", i), fmt.Sprintf(".custom%d/skills", i)} {
				m, _ = setupUpdate(t, m, text(value))
				m, _ = setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
			}
		}
		before := append([]agentConfig(nil), m.cfg.Agents...)
		m, _ = setupUpdate(t, m, ctrl('n'))
		if m.adding || m.err == nil || !reflect.DeepEqual(m.cfg.Agents, before) {
			t.Fatal("allowed tenth agent")
		}
		m.field = 27
		m, _ = setupUpdate(t, m, ctrl('j'))
		if m.field != 27 || !reflect.DeepEqual(m.cfg.Agents, before) {
			t.Fatal("reorder escaped bottom")
		}
		prior := m
		m, _ = setupUpdate(t, m, ctrl('k'))
		if m.field != 24 || m.cfg.Agents[7] != before[8] || m.cfg.Agents[8] != before[7] || !reflect.DeepEqual(prior.cfg.Agents, before) {
			t.Fatal("move up lost field, order, or model isolation")
		}
		m, _ = setupUpdate(t, m, ctrl('j'))
		if m.field != 27 || !reflect.DeepEqual(m.cfg.Agents, before) {
			t.Fatal("move down did not restore order")
		}
		m.field = 1
		m, _ = setupUpdate(t, m, ctrl('k'))
		if m.field != 1 || !reflect.DeepEqual(m.cfg.Agents, before) {
			t.Fatal("reorder escaped top")
		}
		m, _ = setupUpdate(t, m, ctrl('j'))
		m, cmd := setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
		m, _ = setupUpdate(t, m, cmd())
		if !m.preview || m.err != nil {
			t.Fatalf("custom validation: %v", m.err)
		}
		m.height = 100
		for i, a := range m.cfg.Agents {
			mapping := fmt.Sprintf("focus %d / g%d · copy %c / %c", i+1, i+1, addKeys[i], strings.ToUpper(string(addKeys[i]))[0])
			view := m.View().Content
			if !strings.Contains(view, displayText(a.Name)) || !strings.Contains(view, mapping) {
				t.Fatalf("missing mapping/name: %s", mapping)
			}
			for _, path := range []string{"Global: " + displayText(m.resolved.Agents[i].Global), "Project: " + displayText(m.resolved.Agents[i].Local)} {
				for _, line := range strings.Split(ansi.Hardwrap(path, m.width-4, true), "\n") {
					if !strings.Contains(view, line) {
						t.Fatalf("missing resolved path segment: %q", line)
					}
				}
			}
			setupAbsent(t, m.resolved.Agents[i].Global, m.resolved.Agents[i].Local)
		}
		m, _ = setupUpdate(t, m, text("e"))
		m.field = 27
		for len(m.cfg.Agents) > 1 {
			old := append([]agentConfig(nil), m.cfg.Agents...)
			m, _ = setupUpdate(t, m, ctrl('x'))
			if !reflect.DeepEqual(m.cfg.Agents, old[:len(old)-1]) || m.field != 3*len(m.cfg.Agents) {
				t.Fatal("remove did not preserve order/clamp focus")
			}
		}
		setupAbsent(t, path, os.Getenv("HOME")+"/library")
	})
}

func TestExplicitSetup(t *testing.T) {
	for _, outcome := range []string{"save", "refuse", "cancel-edit", "cancel-preview", "cancel-confirm"} {
		t.Run(outcome, func(t *testing.T) {
			cfg, project, path := saveConfigFixture(t)
			if err := saveConfig(cfg, project, path, false); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			m := newSetupModel(project, path)
			m.cfg, m.replace = cfg, true
			m, _ = setupUpdate(t, m, tea.KeyPressMsg{Code: 'x', Text: "-edited"})
			var cmd tea.Cmd
			if outcome != "cancel-edit" {
				m, cmd = setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
				m, _ = setupUpdate(t, m, cmd())
				if !m.preview || m.err != nil {
					t.Fatalf("preview: %+v", m)
				}
			}
			if outcome != "cancel-edit" && outcome != "cancel-preview" {
				m, cmd = setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
				if cmd != nil || !m.confirm || m.busy || !strings.Contains(m.View().Content, "y confirm · n back") {
					t.Fatal("missing separate confirmation")
				}
				for _, key := range []tea.KeyPressMsg{{Code: tea.KeyEnter}, {Code: 'e', Text: "e"}, {Code: 'n', Mod: tea.ModCtrl}, {Code: 'x', Mod: tea.ModCtrl}} {
					next, ignored := setupUpdate(t, m, key)
					if ignored != nil || !reflect.DeepEqual(next, m) {
						t.Fatal("confirmation accepted unrelated key")
					}
				}
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != string(before) {
				t.Fatal("wrote before confirmation")
			}
			if outcome == "save" {
				m, cmd = setupUpdate(t, m, tea.KeyPressMsg{Code: 'y', Text: "y"})
				if cmd == nil || !m.busy || m.confirm || m.saved {
					t.Fatal("save not pending")
				}
				m, cmd = setupUpdate(t, m, cmd())
				setupQuit(t, cmd)
				if !m.saved || m.fatal != nil {
					t.Fatalf("save: %+v", m)
				}
				loaded, missing, err := loadConfig(path)
				if err != nil || missing || !reflect.DeepEqual(loaded, m.cfg) {
					t.Fatalf("round trip: %+v %v", loaded, err)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var schema map[string]json.RawMessage
				if err := json.Unmarshal(data, &schema); err != nil {
					t.Fatal(err)
				}
				if len(schema) != 2 || schema["library"] == nil || schema["agents"] == nil {
					t.Fatalf("extra persisted state: %s", data)
				}
				var agents []map[string]json.RawMessage
				if err := json.Unmarshal(schema["agents"], &agents); err != nil {
					t.Fatal(err)
				}
				for _, a := range agents {
					if len(a) != 3 || a["name"] == nil || a["global"] == nil || a["local"] == nil {
						t.Fatalf("extra agent state: %s", data)
					}
				}
			} else {
				if outcome == "refuse" {
					m, cmd = setupUpdate(t, m, tea.KeyPressMsg{Code: 'n', Text: "n"})
					if cmd != nil || m.confirm || !m.preview || m.busy {
						t.Fatal("refusal did not return to preview")
					}
				}
				m, cmd = setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
				setupQuit(t, cmd)
				if m.saved || m.fatal != nil {
					t.Fatal("cancel saved or failed")
				}
				after, err := os.ReadFile(path)
				if err != nil || string(after) != string(before) {
					t.Fatal("cancel/refusal changed existing bytes")
				}
			}
			setupAbsent(t, os.Getenv("HOME")+"/library", os.Getenv("HOME")+"/library-edited", os.Getenv("HOME")+"/global", project+"/.example")
		})
	}
}
