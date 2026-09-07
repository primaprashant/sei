package app

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

func assertViewBounds(t *testing.T, view string, width, height int) {
	t.Helper()
	if height > 0 && len(strings.Split(view, "\n")) > height {
		t.Fatalf("height overflow at %dx%d", width, height)
	}
	for _, line := range strings.Split(view, "\n") {
		if ansi.StringWidth(line) > max(0, width) {
			t.Fatalf("width overflow at %dx%d: %q", width, height, line)
		}
	}
}

func TestSetupViewBounds(t *testing.T) {
	for _, count := range []int{1, 3, 9} {
		m := newSetupModel("/project", "/config.json")
		m.cfg.Agents = nil
		for i := range count {
			m.cfg.Agents = append(m.cfg.Agents, agentConfig{Name: fmt.Sprintf("Agent%d", i+1), Global: "~/" + strings.Repeat("long-界/", 20), Local: ".agent/skills"})
		}
		m.cfg.Library = "~/" + strings.Repeat("long-path/", 20) + "END"
		m.resolved = m.cfg
		for _, size := range [][2]int{{80, 24}, {143, 35}, {40, 13}, {20, 8}, {10, 3}, {0, 0}} {
			m.width, m.height = size[0], size[1]
			for _, colored := range []bool{false, true} {
				m.theme = uiTheme{}
				if colored {
					m.theme.profile = colorprofile.TrueColor
				}
				for field := 0; field <= 3*count; field++ {
					m.field = field
					view := m.View().Content
					assertViewBounds(t, view, m.width, m.height)
					if m.width >= 20 && m.height >= 8 && !strings.Contains(ansi.Strip(view), "> ") {
						t.Fatalf("active field %d hidden at %v", field, size)
					}
				}
				for _, state := range []string{"preview", "details", "adding", "confirm", "error", "busy", "quit"} {
					n := m
					n.preview = state == "preview"
					n.details = state == "details"
					n.adding = state == "adding"
					n.confirm = state == "confirm"
					if state == "error" {
						n.err = errors.New(strings.Repeat("long error ", 30))
					}
					n.busy = state == "busy"
					n.pendingQuit = state == "quit"
					assertViewBounds(t, n.View().Content, n.width, n.height)
				}
			}
		}
	}
}

func TestSetupDetails(t *testing.T) {
	m := newSetupModel("/project", "/config.json")
	raw := strings.Repeat("long-界/", 30) + "\x1b[2J\nEND"
	m.cfg.Library = raw
	m.err = errors.New(raw)
	before := m.cfg.Library
	m, cmd := setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyF1})
	if cmd != nil || !m.details {
		t.Fatal("details did not open")
	}
	var seen strings.Builder
	for range 200 {
		lines := strings.Split(m.View().Content, "\n")
		seen.WriteString(lines[2])
		m, _ = setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	seen.WriteString(strings.Join(strings.Split(m.View().Content, "\n")[2:], ""))
	if !strings.Contains(seen.String(), displayText(raw)) || strings.ContainsRune(seen.String(), '\x1b') {
		t.Fatal("full escaped diagnostic unavailable")
	}
	m, _ = setupUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyF1})
	if m.details || m.cfg.Library != before || !strings.Contains(m.View().Content, `\nEND▏`) {
		t.Fatal("details changed value or active input tail is hidden")
	}
}

func TestSetupDefaultFieldsVisible(t *testing.T) {
	m := newSetupModel("/work/project", "/home/demo/.config/sei/config.json")
	m.width, m.height = 80, 24
	for _, field := range []int{0, 9} {
		m.field = field
		view := ansi.Strip(m.View().Content)
		for _, a := range m.cfg.Agents {
			for _, value := range []string{a.Name, a.Global, a.Local} {
				if !strings.Contains(view, value) {
					t.Fatalf("default field %q is hidden with focus %d", value, field)
				}
			}
		}
		if strings.Contains(view, "Earlier fields") || strings.Contains(view, "More agents") {
			t.Fatal("default setup unexpectedly needs scrolling")
		}
	}
}

func TestSetupSnapshots(t *testing.T) {
	var corpus strings.Builder
	for _, state := range []string{"edit", "last-agent", "chooser", "review", "review-small", "replace", "error", "saving", "quit", "details"} {
		m := newSetupModel("/work/project", "/home/demo/.config/sei/config.json")
		m.cfg.Library = "~/skill-library"
		m.resolved = m.cfg
		m.resolved.Library = "/home/demo/skill-library"
		m.resolved.Agents = append([]agentConfig(nil), m.cfg.Agents...)
		for i := range m.resolved.Agents {
			a := &m.resolved.Agents[i]
			a.Global = strings.Replace(a.Global, "~/", "/home/demo/", 1)
			a.Local = "/work/project/" + a.Local
		}
		switch state {
		case "last-agent":
			m.field = 9
		case "chooser":
			m.adding = true
		case "review":
			m.preview = true
			m.width, m.height = 100, 35
		case "review-small":
			m.preview = true
		case "replace":
			m.preview, m.confirm = true, true
		case "error":
			m.err = errors.New("library must be absolute or start with ~/")
		case "saving":
			m.preview, m.busy = true, true
		case "quit":
			m.preview, m.busy, m.pendingQuit = true, true, true
		case "details":
			m.details = true
		}
		view := m.View().Content
		assertViewBounds(t, view, m.width, m.height)
		fmt.Fprintf(&corpus, "\n=== %s (%dx%d) ===\n", state, m.width, m.height)
		for _, line := range strings.Split(view, "\n") {
			corpus.WriteString(strings.TrimRight(line, " ") + "\n")
		}
	}
	const path = "testdata/setup.golden"
	if os.Getenv("SEI_TEST_UPDATE_VIEWS") == "1" {
		if err := os.WriteFile(path, []byte(corpus.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != corpus.String() {
		t.Fatal("setup snapshots changed; review with SEI_TEST_UPDATE_VIEWS=1 go test -run '^TestSetupSnapshots$' ./internal")
	}
}
