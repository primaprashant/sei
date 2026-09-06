package main

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

type browsePanel struct {
	label, path   string
	generation    uint64
	loading       bool
	entries       []skillEntry
	selectedName  string
	selected      int
	missing       bool
	err           error
	safetyChecked bool
	safetyErr     error
	root          resolvedRoot
}

type browseModel struct {
	config           config
	safetyGeneration uint64
	panels           []browsePanel // Library, configured globals, then corresponding locals.
	agents           int
	width, height    int
	focused          int
	pendingGlobal    bool
	showHelp         bool
	helpOffset       int
	nextOperation    uint64
	active           *mutationRequest
	pendingQuit      bool
	status           string
	exitError        error
}

type mutationRequest struct {
	id                uint64
	destination       panelID
	name, label, path string
	selected          int
	add               bool
}

func (r mutationRequest) target() string {
	if r.add {
		return fmt.Sprintf("Add %q to %s (%s)", r.name, r.label, r.path)
	}
	return fmt.Sprintf("Remove %q from %s (%s)", r.name, r.label, r.path)
}

type mutationResult struct {
	id  uint64
	err error
}

type startBrowseMsg struct{}

type rootSafetyMsg struct {
	generation uint64
	safety     rootSafety
}

// Construction only captures resolved configuration; Update owns scan state.
func newBrowseModel(cfg config) browseModel {
	m := browseModel{config: cfg, agents: len(cfg.Agents), width: 100, height: 30}
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
	case exitRequestMsg:
		if m.active != nil {
			m.pendingQuit = true
			return m, nil
		}
		return m, tea.Quit
	case startBrowseMsg:
		// Completion always refreshes all panels, coalescing refresh during work.
		if m.active != nil || m.pendingQuit {
			return m, nil
		}
		m.safetyGeneration++
		cfg, generation := m.config, m.safetyGeneration
		m.panels = append([]browsePanel(nil), m.panels...)
		commands := make([]tea.Cmd, len(m.panels))
		for i := range m.panels {
			p := &m.panels[i]
			p.generation++
			p.loading = true
			p.safetyChecked = false
			commands[i] = scanPanel(panelID(i), p.generation, p.path)
		}
		commands = append(commands, func() tea.Msg { return rootSafetyMsg{generation, resolveRoots(cfg)} })
		return m, tea.Batch(commands...)
	case mutationResult:
		if m.active == nil || msg.id != m.active.id {
			return m, nil
		}
		r := *m.active
		m.active = nil
		m.status = r.target() + ": complete"
		if msg.err != nil {
			m.status = r.target() + ": " + msg.err.Error()
		} else if !r.add {
			m.panels = append([]browsePanel(nil), m.panels...)
			p := &m.panels[r.destination]
			p.selectedName, p.selected = "", r.selected
		}
		if m.pendingQuit {
			if msg.err != nil {
				m.exitError = fmt.Errorf("%s: %w", r.target(), msg.err)
			}
			return m, tea.Quit
		}
		return m.Update(startBrowseMsg{})
	case rootSafetyMsg:
		if msg.generation != m.safetyGeneration || len(msg.safety.blocked) != len(m.panels) {
			return m, nil
		}
		m.panels = append([]browsePanel(nil), m.panels...)
		for i := range m.panels {
			m.panels[i].safetyChecked = true
			m.panels[i].safetyErr = msg.safety.blocked[i]
		}
	case scanResult:
		if msg.panel < 0 || int(msg.panel) >= len(m.panels) {
			return m, nil
		}
		p := m.panels[msg.panel]
		if msg.generation != p.generation || !p.loading {
			return m, nil
		}
		p.loading, p.missing, p.err = false, msg.missing, msg.err
		p.root = msg.root
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
			return m.Update(exitRequestMsg{})
		case "esc":
			m.showHelp, m.pendingGlobal = false, false
			return m, nil
		}
		if m.pendingQuit {
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
		if len(input) == 1 && m.focused == 0 {
			if slot := strings.IndexByte(addKeys, strings.ToLower(input)[0]); slot >= 0 && slot < m.agents {
				destination := 1 + m.agents + slot
				if input[0] >= 'A' && input[0] <= 'Z' {
					destination = 1 + slot
				}
				return m.startMutation(panelID(destination), true)
			}
		}
		switch input {
		case "X":
			return m.startMutation(panelID(m.focused), false)
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

func (m browseModel) startMutation(destination panelID, add bool) (tea.Model, tea.Cmd) {
	if m.active != nil || m.pendingQuit || m.showHelp || destination <= 0 || int(destination) >= len(m.panels) || m.width <= 0 || m.height <= 0 {
		return m, nil
	}
	p := m.panels[m.focused]
	if p.loading || p.err != nil || p.missing || p.selectedName == "" || p.selected < 0 || p.selected >= len(p.entries) {
		return m, nil
	}
	if p.entries[p.selected].blocked || p.entries[p.selected].name != p.selectedName {
		return m, nil
	}
	d := m.panels[destination]
	if !p.safetyChecked || p.safetyErr != nil || !d.safetyChecked || d.safetyErr != nil {
		m.status = "Mutation blocked: root safety is unchecked or unsafe; ? for details"
		return m, nil
	}
	m.nextOperation++
	r := mutationRequest{id: m.nextOperation, destination: destination, name: p.selectedName, label: d.label, path: d.path, selected: p.selected, add: add}
	m.active, m.status = &r, r.target()+": working"
	m.safetyGeneration++
	m.panels = append([]browsePanel(nil), m.panels...)
	for i := range m.panels {
		m.panels[i].generation++
	}
	cfg := m.config
	entry := p.entries[p.selected]
	return m, func() tea.Msg {
		if err := validateScannedSelection(cfg, r, p.root, d.root, entry); err != nil {
			return mutationResult{r.id, err}
		}
		if r.add {
			return mutationResult{r.id, addSkill(cfg, r.destination, r.name)}
		}
		return mutationResult{r.id, removeSkill(cfg, r.destination, r.name)}
	}
}
