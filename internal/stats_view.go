package app

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"
)

type statsModel struct {
	path          string
	theme         uiTheme
	width, height int
	loaded        bool
	since         string
	summary       statsSummary
	fatal         error
}

type statsLoadedMsg struct {
	history statsHistory
	now     time.Time
	err     error
}

func (m statsModel) Init() tea.Cmd {
	return func() tea.Msg {
		history, err := loadStats(m.path)
		return statsLoadedMsg{history, time.Now(), err}
	}
}

func (m statsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := m.theme.update(msg); handled {
		return m, cmd
	}
	switch msg := msg.(type) {
	case exitRequestMsg:
		return m, tea.Quit
	case tea.KeyPressMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case statsLoadedMsg:
		m.loaded, m.fatal = true, msg.err
		if msg.err != nil {
			return m, tea.Quit
		}
		m.since, m.summary = msg.history.TrackingStarted, msg.history.summarize(msg.now)
	}
	return m, nil
}

func (m statsModel) View() tea.View {
	s := m.theme.styles()
	if m.width < minimumWidth || m.height < minimumHeight {
		return boundedView(s.accent.Render("sei | Stats")+"\nResize to at least 80x24\nq quit", m.width, m.height)
	}
	width := min(m.width, 100)
	inner, column := width-4, (width-8)/2
	center := func(text string) string { return lipgloss.PlaceHorizontal(inner, lipgloss.Center, text) }
	pair := func(left, right string) string { return fit(left, column) + "    " + fit(right, column) }
	since := ""
	if date, err := time.Parse(time.DateOnly, m.since); err == nil {
		since = "Since " + date.Format("02 Jan 2006")
	}
	lines := []string{
		s.muted.Render(lipgloss.PlaceHorizontal(inner, lipgloss.Right, since)),
		"",
		center(s.accent.Render(statsNumber(m.summary.copies + m.summary.removals))),
		center(s.title.Render("skill actions")),
		"",
		pair(centerStat(statsNumber(m.summary.copies)+" copies", column), centerStat(statsNumber(m.summary.removals)+" removals", column)),
		pair(centerStat(statsNumber(int64(m.summary.activeDays))+" active days", column), centerStat(statsNumber(m.summary.monthActions)+" actions this month", column)),
		"",
		"",
		pair(s.section.Render("Most copied · All time"), s.section.Render("Most copied · Last 30 days")),
		"",
	}
	for i := range 5 {
		lines = append(lines, pair(statsRank(m.summary.allTime, i, column), statsRank(m.summary.recent, i, column)))
	}
	lines = append(lines, "")
	message := "Copies and removals recorded by sei · Last 30 days includes today"
	if !m.loaded {
		message = "Loading stats..."
	} else if m.since == "" {
		message = "No activity yet. Copy or remove a skill to get started."
	}
	lines = append(lines, center(s.muted.Render(message)))
	frame := s.frame("sei | Stats", "q / Ctrl+C quit", lines, width, 24, false)
	return boundedView(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, frame), m.width, m.height)
}

func centerStat(text string, width int) string {
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, text)
}

func statsRank(items []skillCount, index, width int) string {
	if index >= len(items) {
		if index == 0 {
			return "No copies in this period"
		}
		return ""
	}
	item := items[index]
	count := statsNumber(item.count)
	return fit(displayText(item.name), width-len(count)-2) + "  " + count
}

func statsNumber(n int64) string {
	text := strconv.FormatInt(n, 10)
	var parts []string
	for len(text) > 3 {
		parts = append([]string{text[len(text)-3:]}, parts...)
		text = text[:len(text)-3]
	}
	return strings.Join(append([]string{text}, parts...), ",")
}

func runStats(configPath string, stdin io.Reader, stdout, stderr io.Writer) int {
	input, inputOK := stdin.(interface{ Fd() uintptr })
	output, outputOK := stdout.(interface{ Fd() uintptr })
	if !inputOK || !outputOK || !term.IsTerminal(input.Fd()) || !term.IsTerminal(output.Fd()) {
		_, _ = fmt.Fprintln(stderr, "sei: interactive mode requires terminal stdin and stdout; use --help or --version")
		return 1
	}
	final, err := runLifecycle(statsModel{path: statsPath(configPath), width: 100, height: 30}, stdin, stdout)
	result, _ := final.(statsModel)
	if err = errors.Join(err, result.fatal); err != nil {
		_, _ = fmt.Fprintf(stderr, "sei: stats: %s\n", displayText(err.Error()))
		return 1
	}
	return 0
}
