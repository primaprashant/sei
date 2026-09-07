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
	if p.safetyChecked && p.duplicateProject {
		s.title, s.accent, s.selected = s.muted, s.muted, s.muted
	}
	title := p.label
	if !library {
		if strings.HasSuffix(title, " / Project") {
			title = strings.TrimSuffix(title, " / Project")
		} else {
			title = strings.TrimSuffix(title, " / Global")
		}
	}
	lines := []string{}
	if library {
		lines = append(lines, s.muted.Render(displayText(p.path)))
	} else if p.hint != "" {
		lines = append(lines, s.muted.Render(p.hint))
	}
	if !p.safetyChecked {
		lines = append(lines, s.warning.Render("Checking folder safety"))
	} else if p.safetyErr != nil {
		lines = append(lines, s.warning.Render("Blocked folder · ? details"))
	}
	if p.safetyChecked && p.duplicateProject {
		lines = append(lines, s.muted.Render("Disabled · same as global"))
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
				if p.safetyChecked && p.duplicateProject {
					style = s.muted
				}
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
		v := tea.NewView(m.theme.styles().warning.Render(strings.Join(lines[:min(len(lines), max(0, m.height))], "\n")))
		v.AltScreen = true
		return v
	}
	if m.showHelp {
		return m.helpView()
	}

	s := m.theme.styles()
	leftWidth, columns, rows, visibleRows, firstRow := m.layout()
	rightWidth := m.width - leftWidth - 1
	panelWidth := (rightWidth - columns + 1) / columns
	workspaceHeight := m.height - 5
	panelHeight := (workspaceHeight - 2) / (2 * visibleRows)
	var rightRows []string
	for _, scope := range []int{1, 0} {
		label, detail := "GLOBAL", "all projects"
		if scope == 1 {
			label, detail = "PROJECT", "this project"
		}
		rightRows = append(rightRows, s.sectionLine(label, detail, rightWidth))
		for row := firstRow; row < min(rows, firstRow+visibleRows); row++ {
			height := panelHeight
			if (1-scope)*visibleRows+row-firstRow < (workspaceHeight-2)%(2*visibleRows) {
				height++
			}
			var cells []string
			for col := range columns {
				i := row*columns + col
				if i < m.agents {
					if len(cells) > 0 {
						cells = append(cells, " ")
					}
					id := 1 + scope*m.agents + i
					width := panelWidth
					if col < (rightWidth-columns+1)%columns {
						width++
					}
					cells = append(cells, s.panelView(m.labeledPanel(id), false, id == m.focused, width, height))
				}
			}
			rightRows = append(rightRows, lipgloss.JoinHorizontal(lipgloss.Top, cells...))
		}
	}
	right := strings.Join(rightRows, "\n")
	left := s.sectionLine("LIBRARY", "source", leftWidth) + "\n" + s.panelView(m.labeledPanel(0), true, m.focused == 0, leftWidth, workspaceHeight-1)
	workspace := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	// Keep header context and the status/controls anchored as lists change.
	agentRange := fmt.Sprintf("Agents %d-%d / %d", firstRow*columns+1, min(m.agents, (firstRow+visibleRows)*columns), m.agents)
	header := s.brand.Render(" sei ") + "  " + s.muted.Render(ansi.Truncate(displayText(m.config.Project), max(0, m.width-len(agentRange)-8), "~"))
	header = fit(header, m.width-len(agentRange)-1) + " " + s.muted.Render(agentRange)
	p := m.labeledPanel(m.focused)
	detail := displayText(p.label) + " · " + displayText(p.path)
	if p.selectedName != "" {
		detail += " · " + displayText(p.selectedName)
	}
	status, style := m.browserStatus(s)
	controls := s.shortcuts("↑↓", "select", "Tab", "panel", "0", "library", "1-9", "project", "g", "global", "?", "help", "q", "quit")
	if m.focused > 0 && (!p.safetyChecked || !p.duplicateProject) {
		controls = s.shortcuts("↑↓", "select", "Tab", "panel", "0", "library") + "  " + s.danger.Render("x remove permanently") + "  " + s.shortcuts("?", "help", "q", "quit")
	}
	if m.active != nil {
		controls = s.shortcuts("↑↓", "select", "Tab", "panel", "?", "help", "q", "quit")
	}
	if m.pendingGlobal {
		controls = s.warning.Render("g pending: ") + s.shortcuts("1-9", "global", "Esc", "cancel", "q", "quit")
	}
	if m.pendingQuit {
		controls = ""
	}

	return boundedView(header+"\n"+workspace+"\n"+s.muted.Render(ansi.Truncate(detail, m.width, "~"))+"\n"+
		style.Render(ansi.Truncate("● "+status, m.width, "~"))+"\n"+controls+"\n"+s.muted.Render(m.operationHint()), m.width, m.height)
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
	if p := m.panels[m.focused]; p.safetyChecked && p.duplicateProject {
		return duplicateProjectReason
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
		if m.statsWarning != nil {
			return verb + " " + displayText(m.lastResult.name) + " · stats not saved; ? details", s.warning
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
	for _, p := range m.panels {
		if p.duplicateProject {
			return "Duplicate project panels disabled; use global panels", s.muted
		}
	}
	return "Ready", s.success
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
		if m.focused == 0 && m.active == nil && !m.pendingQuit && (!p.safetyChecked || !p.duplicateProject) {
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

func (m browseModel) helpView() tea.View {
	s := m.theme.styles()
	lines := m.helpLines()
	offset := max(0, min(m.helpOffset, len(lines)-m.helpHeight()))
	visible := append([]string(nil), lines[offset:min(len(lines), offset+m.helpHeight())]...)
	for i, line := range visible {
		switch {
		case line == "SELECTION AND PATHS" || line == "NAVIGATION" || line == "COPY AND REMOVE" || line == "ERRORS AND BEHAVIOR":
			visible[i] = s.section.Render(line)
		case strings.HasPrefix(line, "Error:") || strings.HasPrefix(line, "Root safety blocked:"):
			visible[i] = s.danger.Render(line)
		case strings.HasPrefix(line, "Selected folder blocked:"):
			visible[i] = s.warning.Render(line)
		case strings.HasPrefix(line, "Result:"):
			style := s.success
			if m.statusFailed {
				style = s.danger
			}
			visible[i] = style.Render(line)
		case strings.HasPrefix(line, "Root path:"):
			visible[i] = s.muted.Render(line)
		}
	}
	for len(visible) < m.helpHeight() {
		visible = append(visible, "")
	}
	quit := "q quit"
	if m.pendingQuit {
		quit = "Exit requested; waiting for work"
	} else if m.active != nil {
		quit = "Working; q waits for completion"
	}
	return boundedView(s.accent.Render("sei | Help")+"\n"+strings.Join(visible, "\n")+
		"\n"+s.muted.Render(fmt.Sprintf("Help %d/%d | up/down scroll | ?/Esc close |", offset+1, len(lines)))+"\n"+s.warning.Render(quit+m.sequenceHint()), m.width, m.height)
}

func (m browseModel) helpHeight() int { return max(1, m.height-3) }

func (m browseModel) helpLines() []string {
	p := m.panels[m.focused]
	text := "SELECTION AND PATHS\nFocused: " + displayText(p.label) + "\nRoot path: " + displayText(p.path) +
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
	if p.selected >= 0 && p.selected < len(p.entries) && p.entries[p.selected].blocked {
		text += "\nSelected folder blocked: symlink"
	}
	if p.safetyChecked && p.duplicateProject {
		global := m.panels[m.focused-m.agents]
		text += "\nProject disabled: " + duplicateProjectReason + "\nGlobal destination: " + displayText(global.label) + " (" + displayText(global.path) + ")"
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
	text += "\n\nNAVIGATION\n0 library · 1–9 project · g then 1–9 global · Up/Down select\nTab / Shift+Tab cycles panels forward / backward\nr refresh · ? help · Esc close/cancel · q/Ctrl+C quit\nUnconfigured slots do nothing. g has no timeout; invalid keys cancel it.\n\nCOPY AND REMOVE\nUse these keys from the library to copy a skill:"

	for i := range m.agents {
		project := m.panels[1+m.agents+i]
		label := displayText(project.label)
		if project.safetyChecked && project.duplicateProject {
			label += " (disabled; use global)"
		}
		text += fmt.Sprintf("\n%c: %s; %c: %s", addKeys[i], label, strings.ToUpper(string(addKeys[i]))[0], displayText(m.panels[1+i].label))
	}
	text += "\n\nERRORS AND BEHAVIOR\nCopy always replaces the entire destination, including local edits.\nx permanently removes a destination skill. No confirmation or undo.\nNo trash, backup, or rollback. Failures may leave missing or partial output.\nRetry copy or remove the partial skill after inspecting the destination.\nWhile working: navigation stays available; refresh and quit wait.\nAdditional copy/remove keys are ignored during work. Paste is ignored.\n" + ansi.Wrap(sharedDiscovery, max(1, m.width), "")

	if m.status != "" {
		text += "\nResult: " + displayText(m.status)
	}
	if m.statsWarning != nil {
		text += "\nStats warning: " + displayText(m.statsWarning.Error())
	}
	if m.pendingQuit {
		text += "\nExit requested; waiting for work"
	}
	return strings.Split(ansi.Hardwrap(text, max(1, m.width), true), "\n")
}
