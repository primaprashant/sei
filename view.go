package main

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func displayText(raw string) string {
	quoted := strconv.QuoteToGraphic(raw)
	return quoted[1 : len(quoted)-1]
}

func panelView(p browsePanel, library bool, width, height int) string {
	lines := []string{displayText(p.label), displayText(p.path)}
	switch {
	case p.loading:
		lines = append(lines, "Loading...")
	case p.err != nil:
		lines = append(lines, "Error: "+displayText(p.err.Error()))
	case p.missing && library:
		lines = append(lines, "Unavailable: library missing")
	case p.missing:
		lines = append(lines, "Not created")
	case len(p.entries) == 0:
		lines = append(lines, "Empty")
	default:
		available := max(1, height-2)
		visible := min(len(p.entries), available)
		if visible < len(p.entries) && visible > 1 {
			visible--
		}
		for _, entry := range p.entries[:visible] {
			marker := "  "
			if library && entry.name == p.selectedName {
				marker = "> "
			}
			label := displayText(entry.name)
			if entry.blocked {
				label = "[blocked: symlink] " + label
			}
			lines = append(lines, marker+label)
		}
		if visible < len(p.entries) {
			lines = append(lines, fmt.Sprintf("... %d more (navigation unavailable)", len(p.entries)-visible))
		}
	}
	for i, line := range lines {
		line = ansi.Truncate(line, width, "~")
		lines[i] = line + strings.Repeat(" ", max(0, width-ansi.StringWidth(line)))
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(lines, "\n")
}

func (m browseModel) View() tea.View {
	// Provisional grid, not a supported terminal-size or overflow contract.
	columns := min(3, m.agents)
	rows := (m.agents + columns - 1) / columns
	leftWidth := max(1, m.width/3)
	panelWidth := max(1, (m.width-leftWidth-3)/columns-1)
	panelHeight := max(4, (m.height-4)/(2*rows)-1)
	var rightRows []string
	for scope := range 2 {
		for row := range rows {
			var cells []string
			for col := range columns {
				i := row*columns + col
				if i < m.agents {
					if len(cells) > 0 {
						cells = append(cells, " ")
					}
					cells = append(cells, panelView(m.panels[1+scope*m.agents+i], false, panelWidth, panelHeight))
				}
			}
			rightRows = append(rightRows, lipgloss.JoinHorizontal(lipgloss.Top, cells...))
		}
	}
	right := strings.Join(rightRows, "\n\n")
	left := panelView(m.panels[0], true, leftWidth, lipgloss.Height(right))
	h := help.New()
	keys := h.ShortHelpView([]key.Binding{key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q / ctrl+c", "quit"))})
	v := tea.NewView("sei | Read-only configured folders\n" +
		lipgloss.JoinHorizontal(lipgloss.Top, left, " | ", right) +
		"\nNavigation, refresh, add/remove unavailable.\n" + keys)
	v.AltScreen = true
	return v
}
