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
	"github.com/charmbracelet/x/ansi"
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
	width := min(m.width-4, 100)
	since := "Local skill activity"
	if date, err := time.Parse(time.DateOnly, m.since); err == nil {
		since = "Since " + date.Format("02 Jan 2006")
	}
	lines := []string{
		statsEdges(s.brand.Render(" sei ")+" "+s.title.Render("stats"), s.muted.Render(since), width),
		s.muted.Render("Your skills, in motion."),
		"",
		m.statsOverview(width, s),
		"",
		s.sectionLine("MOST COPIED", "Your go-to skills", width),
		"",
	}
	left := (width - 2) / 2
	lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top,
		statsRanking("All time", "Ranked by copies", m.summary.allTime, left, m.loaded, s, s.accent),
		"  ",
		statsRanking("Last 30 days", "Includes today", m.summary.recent, width-left-2, m.loaded, s, s.section),
	))
	message := "Copies and removals recorded by sei"
	if !m.loaded {
		message = "Loading stats..."
	} else if m.since == "" {
		message = "No activity yet. Copy or remove a skill to get started."
	}
	lines = append(lines, "", statsEdges(s.shortcuts("q / Ctrl+C", "quit"), s.muted.Render(message), width))
	lines = strings.Split(strings.Join(lines, "\n"), "\n")
	for i := range lines {
		lines[i] = fit(lines[i], width)
	}
	return boundedView(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, strings.Join(lines, "\n")), m.width, m.height)
}

// Keep both edges anchored in terminal cells, including odd widths and wide names.
func statsEdges(left, right string, width int) string {
	return fit(left, width-ansi.StringWidth(right)-1) + " " + right
}

func (m statsModel) statsOverview(width int, s uiStyles) string {
	inner := width - 4
	first, second := inner/2, inner/4
	values := []string{statsNumber(m.summary.copies + m.summary.removals), statsNumber(int64(m.summary.activeDays)), statsNumber(m.summary.monthActions)}
	copies, removals := statsNumber(m.summary.copies), statsNumber(m.summary.removals)
	if !m.loaded {
		values, copies, removals = []string{"—", "—", "—"}, "—", "—"
	}
	// Give unusually large counters more room without shortening their values.
	first = max(first, len(values[0])+2)
	second = max(second, len(values[1])+2)
	first = min(first, inner-second-max(18, len(values[2])))
	columns := func(a, b, c string) string { return fit(a, first) + fit(b, second) + c }
	total := m.summary.copies + m.summary.removals
	filled := statsBarCells(m.summary.copies, total, inner)
	bar := s.accent.Render(strings.Repeat("━", filled)) + s.warning.Render(strings.Repeat("╌", inner-filled))
	if total == 0 || !m.loaded {
		bar = s.border.Render(strings.Repeat("─", inner))
	}
	lines := []string{
		columns(s.accent.Render(values[0]), s.title.Render(values[1]), s.title.Render(values[2])),
		columns(s.muted.Render("skill actions"), s.muted.Render("active days"), s.muted.Render("actions this month")),
		"",
		bar,
		statsEdges(s.accent.Render("↑ "+copies+" copies"), s.warning.Render("× "+removals+" removals"), inner),
	}
	return s.frame("Activity", "All time", lines, width, 7, false)
}

func statsRanking(title, footer string, items []skillCount, width int, loaded bool, s uiStyles, accent lipgloss.Style) string {
	inner := width - 4
	countWidth := len("COPIES")
	for _, item := range items {
		countWidth = max(countWidth, len(statsNumber(item.count)))
	}
	barWidth := min(8, max(0, inner-countWidth-17))
	nameWidth := inner - countWidth - 5
	if barWidth > 0 {
		nameWidth -= barWidth + 2
	}
	lines := []string{s.muted.Render(statsEdges("   SKILL", "COPIES", inner)), ""}
	for i := range 5 {
		line := ""
		if i < len(items) && loaded {
			item := items[i]
			nameStyle := s.title
			if i == 0 {
				nameStyle = accent
			}
			line = s.muted.Render(fmt.Sprintf("%d  ", i+1)) + nameStyle.Render(fit(displayText(item.name), nameWidth)) + "  "
			if barWidth > 0 {
				filled := statsBarCells(item.count, items[0].count, barWidth)
				line += accent.Render(strings.Repeat("━", filled)) + s.border.Render(strings.Repeat("─", barWidth-filled)) + "  "
			}
			line += accent.Render(lipgloss.PlaceHorizontal(countWidth, lipgloss.Right, statsNumber(item.count)))
		} else if i == 1 && (len(items) == 0 || !loaded) {
			line = "No copies in this period"
			if !loaded {
				line = "Loading..."
			}
			line = s.muted.Render(lipgloss.PlaceHorizontal(inner, lipgloss.Center, line))
		}
		lines = append(lines, line)
	}
	s.title = accent
	return s.frame(title, footer, lines, width, 9, false)
}

func statsBarCells(count, total int64, width int) int {
	if count <= 0 || total <= 0 || width <= 0 {
		return 0
	}
	// Float division avoids overflowing when a persisted counter approaches int64's limit.
	return min(width, max(1, int(float64(count)/float64(total)*float64(width)+0.5)))
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
