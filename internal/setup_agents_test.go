package app

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestSetupAgents(t *testing.T) {
	ctrl := func(code rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl} }
	text := func(s string) tea.KeyPressMsg { return tea.KeyPressMsg{Code: rune(s[0]), Text: s} }
	t.Run("library focus does not remove or reorder agents", func(t *testing.T) {
		m := newSetupModel("", "")
		before := append([]agentConfig(nil), m.cfg.Agents...)
		for _, key := range []rune{'d', 'k', 'j'} {
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
		m, _ = setupUpdate(t, m, ctrl('a'))
		m, _ = setupUpdate(t, m, text("6"))
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
			m, _ = setupUpdate(t, m, ctrl('a'))
			if !m.adding || !strings.Contains(m.View().Content, "5 Cursor | 6 Custom") {
				t.Fatal("missing chooser")
			}
			before := m.cfg.Agents
			m, cmd := setupUpdate(t, m, text(fmt.Sprint(i+1)))
			if cmd != nil || m.adding || m.field != 4 || len(m.cfg.Agents) != 2 || m.cfg.Agents[1] != preset || len(before) != 1 {
				t.Fatalf("preset %d: %+v", i+1, m)
			}
			m, _ = setupUpdate(t, m, ctrl('a'))
			m, _ = setupUpdate(t, m, text(fmt.Sprint(i+1)))
			if m.err == nil || !strings.Contains(m.err.Error(), "duplicate") || len(m.cfg.Agents) != 2 {
				t.Fatalf("duplicate accepted: %+v", m)
			}
		}
		for _, key := range []tea.KeyPressMsg{text("0"), text("7"), tea.KeyPressMsg{Code: tea.KeyEnter}} {
			m := newSetupModel("", "")
			before := m.cfg
			m, _ = setupUpdate(t, m, ctrl('a'))
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
		m, _ = setupUpdate(t, m, ctrl('d'))
		if len(m.cfg.Agents) != 1 || m.err == nil || m.field != 3 {
			t.Fatal("removed last agent")
		}
		for i := 2; i <= 9; i++ {
			m, _ = setupUpdate(t, m, ctrl('a'))
			m, _ = setupUpdate(t, m, text("6"))
			if m.field != 1+3*(i-1) || m.cfg.Agents[i-1] != (agentConfig{}) {
				t.Fatal("custom not blank or focused")
			}
			for _, value := range []string{fmt.Sprintf("Custom%d", i), fmt.Sprintf("~/custom%d", i), fmt.Sprintf(".custom%d/skills", i)} {
				m, _ = setupUpdate(t, m, text(value))
				m, _ = setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
			}
		}
		before := append([]agentConfig(nil), m.cfg.Agents...)
		m, _ = setupUpdate(t, m, ctrl('a'))
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
			mapping := fmt.Sprintf("%d %s | local %c / global %c | focus %d / g%d", i+1, a.Name, "abcdefhio"[i], "ABCDEFHIO"[i], i+1, i+1)
			if !strings.Contains(m.View().Content, mapping) || !strings.Contains(m.View().Content, m.resolved.Agents[i].Global) || !strings.Contains(m.View().Content, m.resolved.Agents[i].Local) {
				t.Fatalf("missing reordered mapping/paths: %s", mapping)
			}
			setupAbsent(t, m.resolved.Agents[i].Global, m.resolved.Agents[i].Local)
		}
		m, _ = setupUpdate(t, m, text("e"))
		m.field = 27
		for len(m.cfg.Agents) > 1 {
			old := append([]agentConfig(nil), m.cfg.Agents...)
			m, _ = setupUpdate(t, m, ctrl('d'))
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
				if cmd != nil || !m.confirm || m.busy || !strings.Contains(m.View().Content, "y confirm / n back") {
					t.Fatal("missing separate confirmation")
				}
				for _, key := range []tea.KeyPressMsg{{Code: tea.KeyEnter}, {Code: 'e', Text: "e"}, {Code: 'a', Mod: tea.ModCtrl}} {
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
