package app

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

// The zero value is monochrome, also useful for deterministic view snapshots.
// Terminal capability messages choose colors without doing I/O in View.
type uiTheme struct {
	profile                         colorprofile.Profile
	light, noColor, backgroundKnown bool
}

func (t *uiTheme) update(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.ColorProfileMsg:
		t.profile = msg.Profile
		return true, tea.RequestBackgroundColor
	case tea.BackgroundColorMsg:
		t.light = !msg.IsDark()
		t.backgroundKnown = true
	case tea.EnvMsg:
		t.noColor = msg.Getenv("NO_COLOR") != ""
		parts := strings.Split(msg.Getenv("COLORFGBG"), ";")
		if n, err := strconv.Atoi(parts[len(parts)-1]); err == nil && !t.backgroundKnown {
			t.light = n == 7 || n >= 9 && n <= 15
		}
	default:
		return false, nil
	}
	return true, nil
}

type uiStyles struct {
	title, muted, accent, section, selected, success, warning, danger lipgloss.Style
	border, brand, key                                                lipgloss.Style
}

func (t uiTheme) styles() uiStyles {
	if t.noColor || t.profile <= colorprofile.ASCII {
		return uiStyles{}
	}
	violet, cyan, gray := "#B4A4F4", "#7DCBD4", "#9A9AA8"
	green, amber, red, ink := "#91C99C", "#E5BE7A", "#F08D98", "#191923"
	border, surface := "#505064", "#2B2938"
	if t.light {
		violet, cyan, gray = "#6545AA", "#176B7B", "#626273"
		green, amber, red, ink = "#28723D", "#8A5A0A", "#B02D43", "#FFFFFF"
		border, surface = "#B8B5C5", "#ECE8F4"
	}
	fg := func(c string) lipgloss.Style { return lipgloss.NewStyle().Foreground(lipgloss.Color(c)) }
	return uiStyles{
		title: lipgloss.NewStyle().Bold(true), muted: fg(gray), accent: fg(violet).Bold(true),
		section: fg(cyan).Bold(true), success: fg(green), warning: fg(amber), danger: fg(red),
		selected: fg(ink).Background(lipgloss.Color(violet)).Bold(true),
		border:   fg(border), brand: fg(ink).Background(lipgloss.Color(violet)).Bold(true),
		key: fg(violet).Background(lipgloss.Color(surface)).Bold(true),
	}
}

func fit(text string, width int) string {
	width = max(0, width)
	text = ansi.Truncate(text, width, "~")
	return text + strings.Repeat(" ", max(0, width-ansi.StringWidth(text)))
}

// frame owns its exact cell dimensions, including borders and horizontal padding.
func (s uiStyles) frame(title, footer string, lines []string, width, height int, focused bool) string {
	if width < 4 || height < 3 {
		out := make([]string, max(0, height))
		for i := range out {
			line := ""
			if i < len(lines) {
				line = lines[i]
			}
			out[i] = fit(line, width)
		}
		return strings.Join(out, "\n")
	}
	border, heading := s.border, s.title
	if focused {
		border, heading = s.accent, s.accent
		title = "* " + title
	}
	edge := func(left, label, right string, textStyle lipgloss.Style) string {
		label = ansi.Truncate(" "+label+" ", width-2, "~")
		return border.Render(left) + textStyle.Render(label) + border.Render(strings.Repeat("─", width-2-ansi.StringWidth(label))+right)
	}
	out := []string{edge("╭", title, "╮", heading)}
	for i := range height - 2 {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		out = append(out, border.Render("│")+" "+fit(line, width-4)+" "+border.Render("│"))
	}
	out = append(out, edge("╰", footer, "╯", s.muted))
	return strings.Join(out, "\n")
}

func boundedView(text string, width, height int) tea.View {
	lines := strings.Split(text, "\n")
	lines = lines[:min(len(lines), max(0, height))]
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], max(0, width), "~")
	}
	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	return v
}

func (s uiStyles) shortcuts(pairs ...string) string {
	var hints []string
	for i := 0; i+1 < len(pairs); i += 2 {
		hints = append(hints, s.key.Render(pairs[i])+" "+s.muted.Render(pairs[i+1]))
	}
	return strings.Join(hints, "  ")
}

func (s uiStyles) sectionLine(label, detail string, width int) string {
	text := s.section.Render(label) + "  " + s.muted.Render(detail) + " "
	return text + s.border.Render(strings.Repeat("─", max(0, width-ansi.StringWidth(text))))
}
