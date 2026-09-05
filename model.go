package main

import tea "charm.land/bubbletea/v2"

type browsePanel struct {
	label, path  string
	generation   uint64
	loading      bool
	entries      []skillEntry
	selectedName string
	selected     int
	missing      bool
	err          error
}

type browseModel struct {
	panels        []browsePanel // Library, configured globals, then corresponding locals.
	agents        int
	width, height int
	focused       int
	pendingGlobal bool
	showHelp      bool
	helpOffset    int
}

type startBrowseMsg struct{}

// Construction only captures resolved configuration; Update owns scan state.
func newBrowseModel(cfg config) browseModel {
	m := browseModel{agents: len(cfg.Agents), width: 100, height: 30}
	m.panels = append(m.panels, browsePanel{label: "Library", path: cfg.Library, loading: true})
	for _, agent := range cfg.Agents {
		m.panels = append(m.panels, browsePanel{label: agent.Name + " / Global", path: agent.Global, loading: true})
	}
	for _, agent := range cfg.Agents {
		m.panels = append(m.panels, browsePanel{label: agent.Name + " / Local", path: agent.Local, loading: true})
	}
	return m
}

func (browseModel) Init() tea.Cmd {
	return func() tea.Msg { return startBrowseMsg{} }
}

func (m browseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case startBrowseMsg:
		m.panels = append([]browsePanel(nil), m.panels...)
		commands := make([]tea.Cmd, len(m.panels))
		for i := range m.panels {
			p := &m.panels[i]
			p.generation++
			p.loading = true
			commands[i] = scanPanel(panelID(i), p.generation, p.path)
		}
		return m, tea.Batch(commands...)
	case scanResult:
		if msg.panel < 0 || int(msg.panel) >= len(m.panels) {
			return m, nil
		}
		p := m.panels[msg.panel]
		if msg.generation != p.generation || !p.loading {
			return m, nil
		}
		p.loading, p.missing, p.err = false, msg.missing, msg.err
		oldName := p.selectedName
		p.entries, p.selectedName = nil, ""
		if !msg.missing && msg.err == nil {
			p.entries = msg.entries
			if len(p.entries) > 0 {
				p.selected = min(p.selected, len(p.entries)-1)
				for i, entry := range p.entries {
					if entry.name == oldName {
						p.selected = i
						break
					}
				}
				p.selectedName = p.entries[p.selected].name
			}
		}
		if len(p.entries) == 0 {
			p.selected = 0
		}
		m.panels = append([]browsePanel(nil), m.panels...)
		m.panels[msg.panel] = p
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.PasteMsg, tea.PasteStartMsg, tea.PasteEndMsg:
		return m, nil
	case tea.KeyPressMsg:
		input := msg.String()
		switch input {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.showHelp, m.pendingGlobal = false, false
			return m, nil
		}
		if m.pendingGlobal {
			m.pendingGlobal = false
			if len(input) == 1 && input[0] >= '1' && int(input[0]-'0') <= m.agents {
				m.focused = int(input[0] - '0')
				m.helpOffset = 0
			}
			return m, nil // Invalid continuations are consumed, never replayed.
		}
		switch input {
		case "g":
			m.pendingGlobal = true
		case "?":
			m.showHelp = !m.showHelp
			m.helpOffset = 0
		case "r":
			return m.Update(startBrowseMsg{})
		case "0":
			m.focused, m.helpOffset = 0, 0
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			if slot := int(input[0] - '0'); slot <= m.agents {
				m.focused, m.helpOffset = m.agents+slot, 0
			}
		case "up", "down":
			delta := 1
			if input == "up" {
				delta = -1
			}
			if m.showHelp {
				m.helpOffset = max(0, min(m.helpOffset+delta, len(m.helpLines())-m.helpHeight()))
				return m, nil
			}
			p := m.panels[m.focused]
			if len(p.entries) > 0 {
				p.selected = max(0, min(p.selected+delta, len(p.entries)-1))
				p.selectedName = p.entries[p.selected].name
				m.panels = append([]browsePanel(nil), m.panels...)
				m.panels[m.focused] = p
			}
		}
	}
	return m, nil
}
