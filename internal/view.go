package app

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const minimumWidth, minimumHeight = 80, 24

func displayText(raw string) string {
	quoted := strconv.QuoteToGraphic(raw)
	return quoted[1 : len(quoted)-1]
}

func (s uiStyles) panelView(p browsePanel, library, focused bool, width, height int) string {
	title := strings.TrimSuffix(strings.TrimSuffix(p.label, " / Project"), " / Global")
	lines := []string{}
	if library {
		lines = append(lines, s.muted.Render(displayText(p.path)))
	} else if p.hint != "" {
		lines = append(lines, s.accent.Render(p.hint))
	}
	if !p.safetyChecked {
		lines = append(lines, s.warning.Render("Checking folder safety"))
	} else if p.safetyErr != nil {
		lines = append(lines, s.warning.Render("Blocked folder · ? details"))
	}
	footer := fmt.Sprintf("%d skills", len(p.entries))
	if len(p.entries) == 1 {
		footer = "1 skill"
	}
	switch {
	case p.loading:
		lines = append(lines, s.muted.Render("Loading..."))
	case p.err != nil:
		lines = append(lines, s.danger.Render("Error: "+displayText(p.err.Error())))
	case p.missing && library:
		lines = append(lines, s.warning.Render("Library folder missing"))
	case p.missing:
		lines = append(lines, s.muted.Render("Folder not created"))
	case len(p.entries) == 0:
		lines = append(lines, s.muted.Render("No skills yet"))
	default:
		available := max(0, height-2-len(lines))
		visible := min(len(p.entries), available)
		start := max(0, p.selected-visible+1)
		if visible > 0 {
			for _, entry := range p.entries[start : start+visible] {
				marker, style := "  ", lipgloss.NewStyle()
				label := displayText(entry.name)
				if entry.blocked {
					label = "[blocked] " + label
					style = s.warning
				}
				if entry.name == p.selectedName && focused {
					marker, style = "> ", s.selected
				}
				lines = append(lines, style.Render(fit(marker+label, width-4)))
			}
			if visible < len(p.entries) {
				footer = fmt.Sprintf("%d–%d / %d", start+1, start+visible, len(p.entries))
			}
		}
	}
	if library {
		footer = "focus 0 · " + footer
	}
	return s.frame(displayText(title), footer, lines, width, height, focused)
}

// Agent IDs remain globals then projects; only their presentation is reordered.
func (m browseModel) layout() (left, columns, rows, visible, first int) {
	left = min(38, max(26, m.width/3))
	columns = min(max(1, (m.width-left-1)/30), 3, m.agents)
	rows = (m.agents + columns - 1) / columns
	visible = min(rows, max(1, (m.height-7)/14))
	if m.focused > 0 {
		first = max(0, (m.focused-1)%m.agents/columns-visible+1)
	}
	return
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
	s := m.theme.styles()
	leftWidth, columns, rows, visibleRows, firstRow := m.layout()
	rightWidth := m.width - leftWidth - 1
	panelWidth := (rightWidth - columns + 1) / columns
	workspaceHeight := m.height - 5
	panelHeight := (workspaceHeight - 2) / (2 * visibleRows)
	var rightRows []string
	for _, scope := range []int{1, 0} {
		label := "GLOBAL"
		if scope == 1 {
			label = "PROJECT"
		}
		rightRows = append(rightRows, s.section.Render(label))
		for row := firstRow; row < min(rows, firstRow+visibleRows); row++ {
			var cells []string
			for col := range columns {
				i := row*columns + col
				if i < m.agents {
					if len(cells) > 0 {
						cells = append(cells, " ")
					}
					id := 1 + scope*m.agents + i
					cells = append(cells, s.panelView(m.labeledPanel(id), false, id == m.focused, panelWidth, panelHeight))
				}
			}
			rightRows = append(rightRows, lipgloss.JoinHorizontal(lipgloss.Top, cells...))
		}
	}
	right := strings.Join(rightRows, "\n")
	left := s.panelView(m.labeledPanel(0), true, m.focused == 0, leftWidth, workspaceHeight)
	workspace := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	// Keep header context and the status/controls anchored as lists change.
	agentRange := fmt.Sprintf("Agents %d-%d / %d", firstRow*columns+1, min(m.agents, (firstRow+visibleRows)*columns), m.agents)
	header := s.accent.Render("sei") + "  " + s.muted.Render(ansi.Truncate(displayText(m.config.Project), max(0, m.width-len(agentRange)-8), "~"))
	header = fit(header, m.width-len(agentRange)-1) + " " + s.muted.Render(agentRange)
	p := m.labeledPanel(m.focused)
	detail := displayText(p.label) + " · " + displayText(p.path)
	if p.selectedName != "" {
		detail += " · " + displayText(p.selectedName)
	}
	status, style := m.browserStatus(s)
	controls := "↑↓ select  0 library  1-9 project  g global  r refresh  ? help  q quit"
	if m.focused > 0 {
		controls = "↑↓ select  0 library  x remove permanently  g global  ? help  q quit"
	}
	if m.pendingGlobal {
		controls = "g pending: 1-9 global, Esc cancel | ? help | q quit"
	}
	return boundedView(header+"\n"+workspace+"\n"+s.muted.Render(ansi.Truncate(detail, m.width, "~"))+"\n"+
		style.Render(ansi.Truncate(status, m.width, "~"))+"\n"+s.muted.Render(controls)+"\n"+s.warning.Render(m.operationHint()), m.width, m.height)
}

func (m browseModel) operationHint() string {
	if m.pendingQuit {
		return "Exit requested; waiting for work"
	}
	if m.active != nil {
		return "Working; q waits for completion"
	}
	if m.focused == 0 {
		return "Copy using panel keys · Replaces existing folders; no undo"
	}
	return "Removal is permanent · No confirmation or undo"
}

func (m browseModel) browserStatus(s uiStyles) (string, lipgloss.Style) {
	if m.active != nil {
		verb := "Copying"
		if !m.active.add {
			verb = "Removing"
		}
		return verb + " " + displayText(m.active.name) + " → " + displayText(m.active.label), s.section
	}
	if m.lastResult != nil {
		verb, style := "Copied", s.success
		if !m.lastResult.add {
			verb = "Removed"
		}
		if m.statusFailed {
			verb, style = "Failed", s.danger
		}
		return verb + " " + displayText(m.lastResult.name) + " → " + displayText(m.lastResult.label) + " · ? details", style
	}
	if m.status != "" {
		style := s.success
		if m.statusFailed {
			style = s.danger
		}
		return displayText(m.status), style
	}

	for _, p := range m.panels {
		if p.err != nil || p.safetyErr != nil {
			return "Check panel errors; ? for details", s.warning
		}
		if p.loading || !p.safetyChecked {
			return "Checking folders...", s.muted
		}
	}
	return "Ready", s.muted
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
	p.hint = "focus 0"
	if id > 0 {
		slot := (id - 1) % m.agents
		shortcut := strconv.Itoa(slot + 1)
		add := string(addKeys[slot])
		if id <= m.agents {
			shortcut = "g " + shortcut
			add = strings.ToUpper(add)
		}
		p.hint = "focus " + shortcut
		if m.focused == 0 {
			p.hint += " · copy " + add
		}
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
