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

const minimumWidth, minimumHeight = 80, 24

func displayText(raw string) string {
	quoted := strconv.QuoteToGraphic(raw)
	return quoted[1 : len(quoted)-1]
}

func panelView(p browsePanel, library bool, width, height int) string {
	lines := []string{displayText(p.label)}
	if p.hint != "" {
		lines = append(lines, p.hint)
	}
	lines = append(lines, displayText(p.path))
	if !p.safetyChecked {
		lines = append(lines, "Root safety: checking")
	} else if p.safetyErr != nil {
		lines = append(lines, "Root safety: blocked (? reason)")
	}
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
		available := max(1, height-len(lines))
		visible := min(len(p.entries), available)
		if visible < len(p.entries) && visible > 1 {
			visible--
		}
		start := max(0, p.selected-visible+1)
		for _, entry := range p.entries[start : start+visible] {
			marker := "  "
			if entry.name == p.selectedName {
				marker = "> "
			}
			label := displayText(entry.name)
			if entry.blocked {
				label = "[blocked: symlink] " + label
			}
			lines = append(lines, marker+label)
		}
		if visible < len(p.entries) {
			lines = append(lines, fmt.Sprintf("Rows %d-%d / %d", start+1, start+visible, len(p.entries)))
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
	if m.width < minimumWidth || m.height < minimumHeight {
		text := fmt.Sprintf("Resize to at least %dx%d (current %dx%d).\nNew mutations disabled; q / ctrl+c quit.", minimumWidth, minimumHeight, m.width, m.height)
		if m.pendingQuit {
			text += "\nExit requested; waiting for work"
		} else if m.active != nil {
			text += "\nWorking; q waits for completion"
		}
		lines := strings.Split(ansi.Hardwrap(text, max(1, m.width), true), "\n")
		v := tea.NewView(strings.Join(lines[:min(len(lines), max(0, m.height))], "\n"))
		v.AltScreen = true
		return v
	}
	if m.showHelp {
		lines := m.helpLines()
		offset := max(0, min(m.helpOffset, len(lines)-m.helpHeight()))
		quit := "q quit"
		if m.pendingQuit {
			quit = "Exit requested; waiting for work"
		} else if m.active != nil {
			quit = "Working; q waits for completion"
		}
		v := tea.NewView(strings.Join(lines[offset:min(len(lines), offset+m.helpHeight())], "\n") +
			fmt.Sprintf("\nHelp %d/%d | up/down scroll | ?/Esc close |\n%s%s", offset+1, len(lines), quit, m.sequenceHint()))
		v.AltScreen = true
		return v
	}
	leftWidth := min(38, max(26, m.width/3))
	columns := min(max(1, (m.width-leftWidth-2)/30), 3, m.agents)
	rows := (m.agents + columns - 1) / columns
	visibleRows := min(rows, max(1, (m.height-7)/17))
	firstRow := 0
	if m.focused > 0 {
		firstRow = max(0, (m.focused-1)%m.agents/columns-visibleRows+1)
	}
	panelWidth := max(1, (m.width-leftWidth-3)/columns-1)
	panelHeight := max(4, (m.height-7)/(2*visibleRows)-1)
	var rightRows []string
	// Keep target IDs unchanged: presentation order is local, then global.
	for _, scope := range []int{1, 0} {
		for row := firstRow; row < min(rows, firstRow+visibleRows); row++ {
			var cells []string
			for col := range columns {
				i := row*columns + col
				if i < m.agents {
					if len(cells) > 0 {
						cells = append(cells, " ")
					}
					id := 1 + scope*m.agents + i
					cells = append(cells, panelView(m.labeledPanel(id), false, panelWidth, panelHeight))
				}
			}
			rightRows = append(rightRows, lipgloss.JoinHorizontal(lipgloss.Top, cells...))
		}
	}
	right := strings.Join(rightRows, "\n\n")
	left := panelView(m.labeledPanel(0), true, leftWidth, lipgloss.Height(right))
	status := m.status
	if status == "" {
		status = "Ready"
		for _, p := range m.panels {
			if p.loading || !p.safetyChecked {
				status = "Checking folders..."
				break
			}
			if p.err != nil || p.safetyErr != nil {
				status = "Check panel errors; ? for details"
			}
		}
	}
	h := help.New()
	state := h.ShortHelpView([]key.Binding{key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q / ctrl+c", "quit"))})
	if m.pendingQuit {
		state = "Exit requested; waiting for work"
	} else if m.active != nil {
		state = "Working; q waits for completion"
	}
	if m.pendingGlobal {
		state += m.sequenceHint()
	}
	v := tea.NewView(fmt.Sprintf("sei | Configured folders | Agents %d-%d / %d\n", firstRow*columns+1, min(m.agents, (firstRow+visibleRows)*columns), m.agents) +
		lipgloss.JoinHorizontal(lipgloss.Top, left, " | ", right) +
		"\n" + ansi.Truncate("Focused: "+displayText(m.panels[m.focused].label)+" | Selected: "+displayText(m.panels[m.focused].selectedName), m.width, "~") +
		"\n0 library | 1-9 local | g 1-9 global | up/down | r refresh | ? full targets/help" +
		"\nAdd from library using header keys; x permanently removes." +
		"\n" + ansi.Truncate(displayText(status), m.width, "~") +
		"\n" + state)
	v.AltScreen = true
	return v
}

const addKeys = "abcdefhio"

func (m browseModel) sequenceHint() string {
	if m.pendingGlobal {
		return " | g pending: 1-9 global, Esc cancel"
	}
	return ""
}

func (m browseModel) labeledPanel(id int) browsePanel {
	p := m.panels[id]
	p.hint = "focus 0 | ? full target"
	if id > 0 {
		slot := (id - 1) % m.agents
		shortcut := strconv.Itoa(slot + 1)
		add := string(addKeys[slot])
		if id <= m.agents {
			shortcut = "g " + shortcut
			add = strings.ToUpper(add)
		}
		p.hint = "focus " + shortcut + " | add " + add
	}
	base, prefix := m.config.Home, "~"
	if id > m.agents {
		base, prefix = m.config.Project, "."
	}
	if base != "" && strings.HasPrefix(p.path, strings.TrimRight(base, "/")+"/") {
		p.path = prefix + "/" + strings.TrimPrefix(p.path, strings.TrimRight(base, "/")+"/")
		if prefix == "." {
			p.path = strings.TrimPrefix(p.path, "./")
		}
	}
	if id == m.focused {
		p.label = "* " + p.label
	}
	return p
}

func (m browseModel) helpHeight() int { return max(1, m.height-2) }

func (m browseModel) helpLines() []string {
	p := m.panels[m.focused]
	text := "sei | Help\nFocused: " + displayText(p.label) + "\nRoot path: " + displayText(p.path) +
		"\nSelected name: " + displayText(p.selectedName)
	if p.selectedName == "" {
		text += "(none)"
	}
	switch {
	case p.loading:
		text += "\nListing: loading"
	case p.err != nil:
		text += "\nListing: error"
	case p.missing:
		text += "\nListing: not created"
	default:
		text += fmt.Sprintf("\nListing: ready (%d entries)", len(p.entries))
	}
	if p.err != nil {
		text += "\nError: " + displayText(p.err.Error())
	}
	if !p.safetyChecked {
		text += "\nRoot safety: checking"
	} else if p.safetyErr != nil {
		text += "\nRoot safety blocked: " + displayText(p.safetyErr.Error())
	} else {
		text += "\nRoot relations checked; every mutation revalidates."
	}
	text += "\n0 library; 1-9 local; g then 1-9 global. Unconfigured slots do nothing.\nUp/Down clamp selection; in help scroll. r refreshes listings, not config.\ng has no timeout; invalid continuation is consumed. Esc cancels/closes; q/Ctrl+C quit. Paste ignored.\nAdd from library only (existing same-named targets are deleted first, then copied):"
	for i := range m.agents {
		text += fmt.Sprintf("\n%c: %s; %c: %s", addKeys[i], displayText(m.panels[1+m.agents+i].label), strings.ToUpper(string(addKeys[i]))[0], displayText(m.panels[1+i].label))
	}
	text += "\nReplacement loses local edits and destination-only files, even if content seems identical. Not merge or sync.\nx: permanently remove from destination only; X does nothing. No confirmation, trash, backup, or undo.\nCopy failure may leave a missing or partial destination; no rollback. Retry add or remove the partial skill.\nWhile working, navigation remains available; extra mutations are ignored and refresh waits. Quit waits for completion.\nOther agents may also load skills from these folders. sei shows configured folder contents, not everything an agent discovers or has loaded."
	if m.status != "" {
		text += "\nResult: " + displayText(m.status)
	}
	if m.pendingQuit {
		text += "\nExit requested; waiting for work"
	}
	return strings.Split(ansi.Hardwrap(text, max(1, m.width), true), "\n")
}
