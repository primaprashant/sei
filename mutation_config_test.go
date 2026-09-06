package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestMutationConfigProtection(t *testing.T) {
	for _, kind := range []string{"direct", "parent alias", "file alias", "new project"} {
		t.Run(kind, func(t *testing.T) {
			cfg := rootFixture(t)
			base := cfg.Agents[0].Local
			if kind == "new project" {
				cfg.Project = filepath.Join(cfg.Project, "other-project")
				base = filepath.Join(cfg.Project, "skills")
				cfg.Agents[0].Local = base
			}
			browseMkdir(t, filepath.Join(base, "skill"))
			browseMkdir(t, filepath.Join(cfg.Library, "skill"))
			writeTestFile(t, filepath.Join(cfg.Library, "skill", "source"), "source")
			cfg.ConfigPath = filepath.Join(base, "skill", "config.json")
			writeTestFile(t, cfg.ConfigPath, "active config bytes")
			if kind == "parent alias" {
				alias := filepath.Join(cfg.Home, "config-parent")
				if err := os.Symlink(filepath.Join(base, "skill"), alias); err != nil {
					t.Fatal(err)
				}
				cfg.ConfigPath = filepath.Join(alias, "config.json")
			}
			if kind == "file alias" {
				alias := filepath.Join(cfg.Home, "config-link")
				if err := os.Symlink(cfg.ConfigPath, alias); err != nil {
					t.Fatal(err)
				}
				cfg.ConfigPath = alias
			}
			before, library := removeSnapshot(t, base), removeSnapshot(t, cfg.Library)
			for _, operation := range []func(config, panelID, string) error{removeSkill, addSkill} {
				if err := operation(cfg, 2, "skill"); err == nil {
					t.Fatal("active configuration was not protected")
				}
				assertRemoveSnapshot(t, base, before)
				assertRemoveSnapshot(t, cfg.Library, library)
			}
			browseMkdir(t, filepath.Join(base, "unrelated"))
			if err := removeSkill(cfg, 2, "unrelated"); err != nil {
				t.Fatalf("unrelated skill blocked: %v", err)
			}
			data, err := json.Marshal(cfg)
			if err != nil || strings.Contains(string(data), "ConfigPath") || strings.Contains(string(data), "config.json") {
				t.Fatalf("runtime config location persisted: %s, %v", data, err)
			}
		})
	}
}

func TestMutationHelpPendingQuit(t *testing.T) {
	m := mutationModel(t)
	m.focused = 1
	m, worker := press(m, 'x')
	if worker == nil {
		t.Fatal("no worker")
	}
	m, _ = press(m, '?')
	m.height = 5
	m, _ = press(m, 'q')
	if !m.pendingQuit || !strings.Contains(m.View().Content, "Exit requested; waiting for work") {
		t.Fatal("pending quit hidden in help")
	}
	next, cmd := m.Update(worker())
	if next.(browseModel).active != nil || cmd == nil {
		t.Fatal("quit did not wait for completion")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("not a quit command")
	}
}
