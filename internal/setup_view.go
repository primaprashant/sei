package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func (m setupModel) View() tea.View {
	s := m.theme.styles()
	width := min(88, max(0, m.width))
	if width < 20 || m.height < 8 {
		return boundedView(s.warning.Render("Enlarge terminal to edit setup.")+"\nEsc / Ctrl+C cancel\n"+m.setupStatus(), m.width, m.height)
	}
	stage := "Edit → Review"
	if m.preview {
		stage = "Review configuration"
	}
	if m.details {
		stage = "Details"
	}
	if m.adding {
		stage = "Add agent"
	}
	if m.confirm {
		stage = "Replace configuration"
	}
	if m.busy {
		stage = "Checking configuration"
		if m.preview {
			stage = "Saving configuration"
		}
	}
	if m.pendingQuit {
		stage = "Finishing before exit"
	}
	title := s.accent.Render("sei setup") + "  " + s.section.Render(stage)
	body, anchor := m.setupBody(width, s)
	available := m.height - 6
	offset := m.offset
	if !m.preview && !m.details {
		offset = max(0, anchor-available+2)
	}
	offset = max(0, min(offset, len(body)-available))
	visible := append([]string(nil), body[offset:min(len(body), offset+available)]...)
	for len(visible) < available {
		visible = append(visible, "")
	}
	if !m.preview && !m.details && !m.adding && !m.confirm && available >= 6 {
		if offset > 0 {
			visible[0] = s.muted.Render("↑ Earlier fields")
		}
		if offset+available < len(body) {
			for i := len(visible) - 1; i >= 0; i-- {
				if strings.HasPrefix(ansi.Strip(visible[i]), "╰") {
					if i+1 < len(visible) {
						visible[i+1] = s.muted.Render("↓ More agents · Tab continues")
					}
					for j := i + 2; j < len(visible); j++ {
						visible[j] = ""
					}
					break
				}
			}
		}
	}
	helper := m.fieldHint()
	keys := "Tab/↑↓ field  Ctrl+U clear  Enter review  F1 details  Esc cancel"
	extra := "Ctrl+N add agent  Ctrl+X remove agent  Ctrl+K/J reorder"
	if m.preview || m.details {
		helper = fmt.Sprintf("Rows %d–%d / %d", offset+1, min(len(body), offset+available), len(body))
		keys = "Enter save  e edit  ↑↓ scroll  F1 details  Esc cancel"
		extra = "Copy replaces destination edits; removal is permanent. No undo."
		if m.details {
			keys = "↑↓ scroll  F1 back  Esc cancel setup"
			extra = ""
		}
	}
	if m.adding {
		helper = "Choose a preset or start with blank fields."
		keys = "1–9 preset  0 custom  Other keys back  Esc cancel setup"
		extra = ""
	}
	if m.confirm {
		helper = "Replace only the configuration shown above."
		keys = "y confirm  n back  Esc cancel"
		extra = ""
	}
	if width < 65 {
		switch {
		case m.details:
			keys, extra = "↑↓ scroll  F1 back", "Esc cancel setup"
		case m.confirm:
			keys, extra = "y confirm  n back", "Esc cancel"
		case m.adding:
			keys, extra = "1–9 preset · 0 custom", "Esc cancel · Other keys back"
		case m.preview:
			keys, extra = "Enter save  e edit", "Esc cancel  ↑↓ scroll"
		default:
			keys, extra = "Tab field  Enter review", "Esc cancel  F1 details"
		}
	}
	if m.busy {
		helper = ""
		keys = "Esc / Ctrl+C quit after work"
		extra = ""
	}
	if m.pendingQuit {
		helper, keys, extra = "", "", ""
	}
	statusStyle := s.danger
	if m.busy {
		statusStyle = s.section
	}
	if m.pendingQuit {
		statusStyle = s.warning
	}
	status := statusStyle.Render(ansi.Truncate(m.setupStatus(), width, "~"))
	if m.details && !m.busy {
		status = ""
	}
	lines := []string{title, ""}
	lines = append(lines, visible...)
	lines = append(lines, status, s.muted.Render(helper), s.accent.Render(keys), s.muted.Render(extra))
	margin := strings.Repeat(" ", max(0, (m.width-width)/2))
	for i := range lines {
		lines[i] = margin + ansi.Truncate(lines[i], width, "~")
	}
	return boundedView(strings.Join(lines, "\n"), m.width, m.height)
}

func (m setupModel) setupStatus() string {
	if m.pendingQuit {
		return "Exit requested; waiting for work"
	}
	if m.busy {
		if m.preview {
			return "Saving configuration; quit waits for completion."
		}
		return "Checking configuration; quit waits for completion."
	}
	if m.err != nil {
		return "Error: " + displayText(m.err.Error()) + " · F1 details"
	}
	return ""
}

func (m setupModel) fieldHint() string {
	if m.field == 0 {
		return "Source stays unchanged. Type appends; Backspace erases."
	}
	switch (m.field - 1) % 3 {
	case 0:
		return "Agent name: choose a unique label. Order determines shortcut slots."
	case 1:
		return "Global folder: absolute path or ~/; available across projects."
	default:
		return "Project folder: relative path inside the current project."
	}
}

func (m setupModel) setupBody(width int, s uiStyles) ([]string, int) {
	wrap := func(text string) []string { return strings.Split(ansi.Hardwrap(text, width-4, true), "\n") }
	if m.confirm {
		lines := append([]string{s.warning.Render("Replace existing configuration?"), ""}, wrap(displayText(m.path))...)
		return strings.Split(s.frame("Configuration", "y confirm · n back", lines, width, len(lines)+2, false), "\n"), 0
	}
	if m.adding {
		var lines []string
		for i, a := range setupPresets {
			lines = append(lines, s.accent.Render(fmt.Sprint(i+1))+"  "+displayText(a.Name))
		}
		lines = append(lines, s.accent.Render("0")+"  Custom")
		return strings.Split(s.frame("Add agent", "1–9 preset · 0 custom", lines, width, len(lines)+2, false), "\n"), 0
	}
	if m.details {
		lines := []string{s.section.Render("PATHS AND FIELD")}
		for _, text := range []string{"Config: " + displayText(m.path), "Project: " + displayText(m.project), "Current field: " + displayText(*m.value())} {
			lines = append(lines, strings.Split(ansi.Hardwrap(text, width, true), "\n")...)
		}
		for _, line := range strings.Split(ansi.Wrap(m.fieldHint(), width, ""), "\n") {
			lines = append(lines, s.muted.Render(line))
		}
		lines = append(lines, "")
		if m.err != nil {
			for _, line := range strings.Split(ansi.Hardwrap("Error: "+displayText(m.err.Error()), width, true), "\n") {
				lines = append(lines, s.danger.Render(line))
			}
			lines = append(lines, "")
		}
		lines = append(lines, s.section.Render("EDITING"))
		text := "Tab/Up/Down selects fields; typing appends; Backspace erases; Ctrl+U clears.\nCtrl+N adds an agent, Ctrl+X removes it, Ctrl+K/J changes its slot.\nEnter reviews resolved paths before saving. Recognized paste is ignored."
		lines = append(lines, strings.Split(ansi.Wrap(text, width, ""), "\n")...)
		lines = append(lines, "")
		for _, line := range strings.Split(ansi.Wrap(sharedDiscovery, width, ""), "\n") {
			lines = append(lines, s.muted.Render(line))
		}
		return lines, 0
	}

	var body []string
	anchor := 0
	appendFrame := func(title, footer string, lines []string, active bool) {
		body = append(body, strings.Split(s.frame(title, footer, lines, width, len(lines)+2, active), "\n")...)
		body = append(body, "")
	}
	if m.preview {
		for _, line := range wrap("Config: " + displayText(m.path)) {
			body = append(body, s.muted.Render(line))
		}
		body = append(body, "")
		appendFrame("Library", "source stays unchanged", wrap("Library: "+displayText(m.resolved.Library)), false)
		for i, a := range m.resolved.Agents {
			lines := wrap("Global: " + displayText(a.Global))
			lines = append(lines, wrap("Project: "+displayText(a.Local))...)
			appendFrame(fmt.Sprintf("%d %s", i+1, displayText(a.Name)), fmt.Sprintf("focus %d / g%d · copy %c / %c", i+1, i+1, addKeys[i], strings.ToUpper(string(addKeys[i]))[0]), lines, false)
		}
		for _, text := range []string{
			"Replacement loses local edits and destination-only files: delete first, then copy.",
			"Deletion is permanent. No confirmation, trash, backup, or rollback; failures may leave partial output.",
			sharedDiscovery,
		} {
			for _, line := range strings.Split(ansi.Wrap(text, width-4, ""), "\n") {
				style := s.warning
				if text == sharedDiscovery {
					style = s.muted
				}
				body = append(body, style.Render(line))
			}
		}
		return body, 0
	}
	field := func(id int, label, value string) string {
		prefix := "  " + fmt.Sprintf("%-9s", label+":")
		style := s.title.Bold(false)
		if m.field == id {
			prefix = "> " + fmt.Sprintf("%-9s", label+":")
			style = s.selected
		}
		value = displayText(value)
		if m.field == id {
			available := max(0, width-4-ansi.StringWidth(prefix)-1)
			if ansi.StringWidth(value) > available {
				value = ansi.TruncateLeft(value, ansi.StringWidth(value)-available+1, "~")
			}
			value += "▏"
		}
		return style.Render(fit(prefix+value, width-4))
	}
	appendFrame("Library", "source collection", []string{field(0, "Library", m.cfg.Library)}, m.field == 0)
	anchor = 1
	for i, a := range m.cfg.Agents {
		selected := m.field > 0 && (m.field-1)/3 == i
		if selected {
			anchor = len(body) + 1 + (m.field-1)%3
			if m.height-6 >= 6 {
				anchor = len(body) + 4
			}
		}
		appendFrame(fmt.Sprintf("%d · %s", i+1, displayText(a.Name)), fmt.Sprintf("Agent %d / %d", i+1, len(m.cfg.Agents)), []string{
			field(1+3*i, "Name", a.Name), field(2+3*i, "Global", a.Global), field(3+3*i, "Project", a.Local),
		}, selected)
	}
	return body, anchor
}
