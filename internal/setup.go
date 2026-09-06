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
}

type setupResult struct {
	resolved config
	err      error
	saved    bool
}

type setupModel struct {
	cfg, resolved                     config
	project, path                     string
	field, offset, height             int
	preview, busy, pendingQuit, saved bool
	replace, confirm, adding          bool
	err, fatal                        error
}

func newSetupModel(project, path string) setupModel {
	return setupModel{cfg: config{Agents: append([]agentConfig(nil), setupPresets[:3]...)}, project: project, path: path, height: 24}
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
	switch msg := msg.(type) {
	case exitRequestMsg:
		if m.busy {
			m.pendingQuit = true
			return m, nil
		}
		return m, tea.Quit
	case tea.WindowSizeMsg:
		m.height = msg.Height
	case setupResult:
		m.busy = false
		m.err, m.resolved, m.saved = msg.err, msg.resolved, msg.saved
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
			if len(key) != 1 || key[0] < '1' || key[0] > '6' {
				return m, nil
			}
			a := agentConfig{}
			if key != "6" {
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
		case "ctrl+a":
			if len(m.cfg.Agents) == 9 {
				m.err = fmt.Errorf("agents must contain 1-9 entries")
			} else {
				m.adding = true
			}
		case "ctrl+d":
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
				return setupResult{resolved: resolved, err: err}
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

func (m setupModel) View() tea.View {
	lines := []string{"sei setup", "Config: " + displayText(m.path)}
	if m.confirm {
		v := tea.NewView(strings.Join(append(lines, "Replace existing configuration? y confirm / n back / Esc cancel"), "\n"))
		v.AltScreen = true
		return v
	}
	if m.adding {
		v := tea.NewView(strings.Join(append(lines, "Add agent: 1 Claude Code | 2 Codex | 3 OpenCode | 4 Pi | 5 Cursor | 6 Custom", "Other keys return; Esc cancels setup."), "\n"))
		v.AltScreen = true
		return v
	}
	if m.preview {
		lines = append(lines, "Preview | Enter save | e edit | Up/Down scroll | Esc cancel", "Library: "+displayText(m.resolved.Library))
		for i, a := range m.resolved.Agents {
			lines = append(lines, fmt.Sprintf("%d %s | local %c / global %c | focus %d / g%d", i+1, displayText(a.Name), "abcdefhio"[i], "ABCDEFHIO"[i], i+1, i+1), "  Global: "+displayText(a.Global), "  Local: "+displayText(a.Local))
		}
	} else {
		lines = append(lines, "Tab/Up/Down field | type appends | Backspace | Ctrl+U clear | Enter preview | Esc cancel")
		lines = append(lines, "Ctrl+A add agent | Ctrl+D remove selected agent | Ctrl+K/J move agent up/down")
		fields := []string{"Library: " + displayText(m.cfg.Library)}
		for _, a := range m.cfg.Agents {
			fields = append(fields, "Name: "+displayText(a.Name), "Global: "+displayText(a.Global), "Local: "+displayText(a.Local))
		}
		start := max(0, m.field-max(1, m.height-12)+1)
		for i := start; i < len(fields) && i < start+max(1, m.height-12); i++ {
			marker := "  "
			if i == m.field {
				marker = "> "
			}
			lines = append(lines, marker+fields[i])
		}
	}
	lines = append(lines, "Replacement loses local edits and destination-only files: delete first, then copy.", "Deletion is permanent. No confirmation, trash, backup, or rollback; failures may leave partial output.", sharedDiscovery)
	if m.err != nil {
		lines = append(lines, "Error: "+displayText(m.err.Error()))
	}
	if m.busy {
		lines = append(lines, "Working; quit waits for completion.")
	}
	if m.pendingQuit {
		lines = append(lines, "Exit requested; waiting.")
	}
	if m.preview {
		start := min(m.offset, max(0, len(lines)-max(1, m.height-1)))
		lines = lines[start:min(len(lines), start+max(1, m.height-1))]
	}
	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	return v
}
