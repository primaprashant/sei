package app

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

func TestDuplicateProjectSetup(t *testing.T) {
	_, _, path := saveConfigFixture(t)
	home := os.Getenv("HOME")
	m := newSetupModel(home, path)
	m.cfg.Library = "~/skill-library"
	m, cmd := setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m, _ = setupUpdate(t, m, cmd())
	if m.err != nil || !m.preview || !m.duplicateProject[4] || !m.duplicateProject[5] || m.duplicateProject[6] {
		t.Fatalf("default setup at home: %+v", m)
	}
	body, _ := m.setupBody(88, m.theme.styles())
	if !strings.Contains(strings.Join(body, "\n"), "Project disabled here:") {
		t.Fatal("preview lacks explanation")
	}
	setupAbsent(t, path, home+"/.claude", home+"/.agents", home+"/skill-library")
	m, cmd = setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m, quit := setupUpdate(t, m, cmd())
	if m.err != nil || !m.saved {
		t.Fatalf("save: %+v", m)
	}
	setupQuit(t, quit)
	cfg, missing, err := loadConfig(path)
	if err != nil || missing || cfg.Agents[0].Local != ".claude/skills" {
		t.Fatalf("saved config: %+v, %v", cfg, err)
	}
	setupAbsent(t, home+"/.claude", home+"/.agents", home+"/skill-library")
}

func TestDuplicateProjectMutations(t *testing.T) {
	for _, kind := range []string{"existing", "missing", "alias", "missing alias"} {
		t.Run(kind, func(t *testing.T) {
			cfg := rootFixture(t)
			cfg.Agents[0].Global = cfg.Agents[0].Local
			if strings.Contains(kind, "missing") {
				cfg.Agents[0].Global += "/new/skills"
				cfg.Agents[0].Local = cfg.Agents[0].Global
			}
			if strings.Contains(kind, "alias") {
				if err := os.Symlink("local", cfg.Project+"/alias"); err != nil {
					t.Fatal(err)
				}
				cfg.Agents[0].Local = strings.Replace(cfg.Agents[0].Global, "/local", "/alias", 1)
			}
			browseMkdir(t, cfg.Library+"/example")
			writeTestFile(t, cfg.Library+"/example/SKILL.md", "original")
			s := resolveRoots(cfg)
			if !s.duplicateProject[2] || s.blocked[1] != nil || s.blocked[2] != nil {
				t.Fatalf("safety: %+v", s)
			}
			for _, mutate := range []func(config, panelID, string) error{addSkill, removeSkill} {
				if err := mutate(cfg, 2, "example"); err == nil || !strings.Contains(err.Error(), duplicateProjectReason) {
					t.Fatalf("project operation accepted: %v", err)
				}
			}
			if err := addSkill(cfg, 1, "example"); err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, cfg.Library+"/example/SKILL.md", "replacement")
			if err := addSkill(cfg, 1, "example"); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(cfg.Agents[0].Global + "/example/SKILL.md")
			if err != nil || string(data) != "replacement" {
				t.Fatalf("global replacement: %q, %v", data, err)
			}
			if err := removeSkill(cfg, 1, "example"); err != nil {
				t.Fatal(err)
			}
			setupAbsent(t, cfg.Agents[0].Global+"/example")
		})
	}
}

func TestDuplicateProjectStillRejectsUnsafeConfig(t *testing.T) {
	for _, kind := range []string{"library", "nested", "other agent", "config", "containment"} {
		t.Run(kind, func(t *testing.T) {
			cfg, _, path := saveConfigFixture(t)
			home := os.Getenv("HOME")
			cfg.Agents[0].Global, cfg.Agents[0].Local = "~/skills", "skills"
			switch kind {
			case "library":
				cfg.Library = "~/skills"
			case "nested":
				cfg.Agents[0].Local = "skills/nested"
			case "other agent":
				cfg.Agents = append(cfg.Agents, agentConfig{"Other", "~/skills", "other"})
			case "config":
				path = home + "/skills/config.json"
			case "containment":
				cfg.Agents[0].Global = home + "-outside"
				browseMkdir(t, cfg.Agents[0].Global)
				if err := os.Symlink(cfg.Agents[0].Global, home+"/skills"); err != nil {
					t.Fatal(err)
				}
			}
			if err := saveConfig(cfg, home, path, false); err == nil {
				t.Fatal("unsafe setup accepted")
			}
			setupAbsent(t, path)
		})
	}
}

func TestDuplicateProjectBrowserAndRefresh(t *testing.T) {
	m := mutationModel(t)
	m.config.Agents[0].Global = m.config.Agents[0].Local
	m.panels[1].path = m.config.Agents[0].Global
	next, cmd := m.Update(startBrowseMsg{})
	m = finishMutationRefresh(t, next.(browseModel), cmd)
	for _, add := range []bool{false, true} {
		m.focused = 2
		if add {
			m.focused = 0
		}
		next, cmd = m.startMutation(2, add)
		if cmd != nil || !strings.Contains(next.(browseModel).status, duplicateProjectReason) {
			t.Fatal("project shortcut not blocked")
		}
	}
	for _, theme := range []uiTheme{{}, {profile: colorprofile.TrueColor}, {profile: colorprofile.TrueColor, light: true}} {
		m.theme, m.focused = theme, 2
		view := ansi.Strip(m.View().Content)
		assertViewBounds(t, view, m.width, m.height)
		if !strings.Contains(view, "Disabled · same as global") || strings.Contains(view, "x remove permanently") || !strings.Contains(view, duplicateProjectReason) {
			t.Fatalf("disabled view:\n%s", view)
		}
		if !strings.Contains(strings.Join(m.helpLines(), "\n"), "Global destination:") {
			t.Fatal("missing global destination")
		}
		m.focused = 0
		if strings.Contains(m.labeledPanel(2).hint, "copy") || !strings.Contains(m.labeledPanel(1).hint, "copy A") {
			t.Fatal("wrong copy hints")
		}
	}
	m.focused = 0
	next, cmd = m.startMutation(1, true)
	if cmd == nil || next.(browseModel).active == nil {
		t.Fatal("global shortcut blocked")
	}
	// A newly introduced alias must block even without refreshing the displayed state.
	cfg := rootFixture(t)
	if err := os.Remove(cfg.Agents[0].Local); err != nil {
		t.Fatal(err)
	}
	cfg.Agents[0].Global = cfg.Project + "/shared"
	browseMkdir(t, cfg.Agents[0].Global)
	if err := os.Symlink("shared", cfg.Agents[0].Local); err != nil {
		t.Fatal(err)
	}
	if _, err := revalidateSkillRoot(cfg, 2, "a"); err == nil {
		t.Fatal("fresh duplicate accepted")
	}
	// Refresh can both clear and restore the disabled state; stale results are ignored.
	old := rootSafetyMsg{m.safetyGeneration, resolveRoots(m.config)}
	m.config.Agents[0].Global = m.config.Home + "/separate"
	next, cmd = m.Update(startBrowseMsg{})
	m = finishMutationRefresh(t, next.(browseModel), cmd)
	next, _ = m.Update(old)
	m = next.(browseModel)
	if m.panels[2].duplicateProject {
		t.Fatal("stale duplicate restored")
	}
	m.config.Agents[0].Global = m.config.Agents[0].Local
	next, cmd = m.Update(startBrowseMsg{})
	m = finishMutationRefresh(t, next.(browseModel), cmd)
	if !m.panels[2].duplicateProject {
		t.Fatal("duplicate not restored")
	}
}
