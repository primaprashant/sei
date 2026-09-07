package app

import (
	"fmt"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
)

const sharedDiscovery = "Other agents may also load skills from these folders. sei shows configured folder contents, not everything an agent discovers or has loaded."

var setupPresets = []agentConfig{
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

type setupResult struct {
	duplicateProject []bool
	resolved         config
	err              error
	saved            bool
}

type setupModel struct {
	duplicateProject                  []bool
	cfg, resolved                     config
	project, path                     string
	field, offset, width, height      int
	theme                             uiTheme
	details                           bool
	preview, busy, pendingQuit, saved bool
	replace, confirm, adding          bool
	err, fatal                        error
}

func newSetupModel(project, path string) setupModel {
	return setupModel{cfg: config{Agents: append([]agentConfig(nil), setupPresets[:3]...)}, project: project, path: path, width: 80, height: 24}
}

func (setupModel) Init() tea.Cmd { return nil }

func (m *setupModel) value() *string {
	if m.field == 0 {
		return &m.cfg.Library
	}
	a := &m.cfg.Agents[(m.field-1)/3]
	switch (m.field - 1) % 3 {
	case 0:
		return &a.Name
	case 1:
		return &a.Global
	default:
		return &a.Local
	}
}

func (m setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := m.theme.update(msg); handled {
		return m, cmd
	}
	switch msg := msg.(type) {
	case exitRequestMsg:
		if m.busy {
			m.pendingQuit = true
			return m, nil
		}
		return m, tea.Quit
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case setupResult:
		m.busy = false
		m.err, m.resolved, m.saved = msg.err, msg.resolved, msg.saved
		m.duplicateProject = msg.duplicateProject
		if msg.saved || (msg.err != nil && m.preview) {
			m.fatal = msg.err
			return m, tea.Quit
		}
		if m.pendingQuit {
			return m, tea.Quit
		}
		m.preview = msg.err == nil
		m.offset = 0
	case tea.PasteMsg, tea.PasteStartMsg, tea.PasteEndMsg:
		return m, nil
	case tea.KeyPressMsg:
		key := msg.String()
		if key == "ctrl+c" || key == "esc" {
			return m.Update(exitRequestMsg{})
		}
		if m.busy {
			return m, nil
		}
		if m.details {
			switch key {
			case "f1":
				m.details = false
				m.offset = 0
			case "up":
				m.offset = max(0, m.offset-1)
			case "down":
				m.offset++
			}
			return m, nil
		}
		if key == "f1" && !m.confirm && !m.adding {
			m.details = true
			m.offset = 0
			return m, nil
		}

		if m.confirm {
			if key == "n" {
				m.confirm = false
				return m, nil
			}
			if key != "y" {
				return m, nil
			}
			m.confirm = false
			m.busy = true
			return m, func() tea.Msg {
				return setupResult{resolved: m.resolved, err: saveConfig(m.cfg, m.project, m.path, true), saved: true}
			}
		}
		if m.adding {
			m.adding = false
			if len(key) != 1 || key[0] < '0' || key[0] > '9' {
				return m, nil
			}
			a := agentConfig{}
			if key != "0" {
				a = setupPresets[int(key[0]-'1')]
			}
			for _, existing := range m.cfg.Agents {
				if a.Name != "" && existing.Name == a.Name {
					m.err = fmt.Errorf("duplicate agent name %q", a.Name)
					return m, nil
				}
			}
			m.cfg.Agents = append(append([]agentConfig(nil), m.cfg.Agents...), a)
			m.field = 1 + 3*(len(m.cfg.Agents)-1)
			return m, nil
		}
		if m.preview {
			switch key {
			case "e":
				m.preview = false
				m.offset = 0
			case "up":
				m.offset = max(0, m.offset-1)
			case "down":
				m.offset++
			case "enter":
				if m.replace {
					m.confirm = true
					return m, nil
				}
				m.busy = true
				cfg, project, path := m.cfg, m.project, m.path
				return m, func() tea.Msg {
					return setupResult{resolved: m.resolved, err: saveConfig(cfg, project, path, false), saved: true}
				}
			}
			return m, nil
		}
		m.cfg.Agents = append([]agentConfig(nil), m.cfg.Agents...)
		switch key {
		case "ctrl+n":
			if len(m.cfg.Agents) == 9 {
				m.err = fmt.Errorf("agents must contain 1-9 entries")
			} else {
				m.adding = true
			}
		case "ctrl+x":
			if len(m.cfg.Agents) == 1 {
				m.err = fmt.Errorf("at least one agent is required")
			} else if m.field > 0 {
				i := (m.field - 1) / 3
				m.cfg.Agents = append(m.cfg.Agents[:i], m.cfg.Agents[i+1:]...)
				m.field = min(m.field, 3*len(m.cfg.Agents))
			}
		case "ctrl+k", "ctrl+j":
			if m.field > 0 {
				i := (m.field - 1) / 3
				j := i + 1
				if key == "ctrl+k" {
					j = i - 1
				}
				if j >= 0 && j < len(m.cfg.Agents) {
					m.cfg.Agents[i], m.cfg.Agents[j] = m.cfg.Agents[j], m.cfg.Agents[i]
					m.field += 3 * (j - i)
				}
			}
		case "tab", "down":
			m.field = (m.field + 1) % (1 + 3*len(m.cfg.Agents))
		case "shift+tab", "up":
			m.field = (m.field + 3*len(m.cfg.Agents)) % (1 + 3*len(m.cfg.Agents))
		case "enter":
			m.busy = true
			cfg, project, path := m.cfg, m.project, m.path
			return m, func() tea.Msg {
				resolved, err := validateSetupConfig(cfg, project, path)
				result := setupResult{resolved: resolved, err: err}
				if err == nil {
					result.duplicateProject = resolveRoots(resolved).duplicateProject
				}
				return result
			}
		case "ctrl+u":
			*m.value() = ""
		case "backspace":
			runes := []rune(*m.value())
			if len(runes) > 0 {
				*m.value() = string(runes[:len(runes)-1])
			}
		default:
			if msg.Mod == 0 || msg.Mod == tea.ModShift {
				if msg.Text != "" && strings.IndexFunc(msg.Text, unicode.IsControl) == -1 {
					*m.value() += msg.Text
				}
			}
		}
	}
	return m, nil
}
