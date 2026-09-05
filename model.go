package main

import tea "charm.land/bubbletea/v2"

type browsePanel struct {
	label, path  string
	generation   uint64
	loading      bool
	entries      []skillEntry
	selectedName string
	missing      bool
	err          error
}

type browseModel struct {
	panels        []browsePanel // Library, configured globals, then corresponding locals.
	agents        int
	width, height int
}

type startBrowseMsg struct{}

// Construction only captures resolved configuration; Update owns scan state.
func newBrowseModel(cfg config) browseModel {
	m := browseModel{agents: len(cfg.Agents), width: 100, height: 30}
	m.panels = append(m.panels, browsePanel{label: "Library [focus]", path: cfg.Library, loading: true})
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
		p.entries, p.selectedName = nil, ""
		if !msg.missing && msg.err == nil {
			p.entries = msg.entries
			if len(p.entries) > 0 {
				p.selectedName = p.entries[0].name
			}
		}
		m.panels = append([]browsePanel(nil), m.panels...)
		m.panels[msg.panel] = p
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}
