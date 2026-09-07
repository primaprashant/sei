package app

import (
	"bytes"
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

func populatedStatsModel() statsModel {
	return statsModel{loaded: true, width: 80, height: 24, since: "2026-03-12", summary: statsSummary{
		copies: 934, removals: 314, activeDays: 86, monthActions: 42,
		allTime: []skillCount{{"code-review", 126}, {"debugging", 94}, {"writing", 71}, {"testing", 58}, {"research", 43}},
		recent:  []skillCount{{"debugging", 18}, {"testing", 12}, {"code-review", 9}},
	}}
}

func TestStatsViewSnapshots(t *testing.T) {
	var corpus strings.Builder
	for _, scenario := range []string{"populated", "desktop", "odd-width", "empty", "loading", "removals-only", "undersized", "long-names", "large-counts"} {
		m := populatedStatsModel()
		switch scenario {
		case "desktop":
			m.width, m.height = 110, 30
		case "odd-width":
			m.width = 81
		case "empty", "loading":
			m.since, m.summary, m.loaded = "", statsSummary{}, scenario == "empty"
		case "removals-only":
			m.summary = statsSummary{removals: 12, activeDays: 3, monthActions: 4}
		case "undersized":
			m.width, m.height = 79, 23
		case "long-names":
			m.summary.allTime = []skillCount{{strings.Repeat("界", 50), 1234}, {"bad\xff\x1b[2J\r\nname", 2}}
		case "large-counts":
			m.summary.copies, m.summary.removals, m.summary.monthActions = math.MaxInt64-1, 1, math.MaxInt64
			m.summary.allTime = []skillCount{{"code-review", math.MaxInt64 - 1}}
		}
		view := m.View()
		assertViewBounds(t, view.Content, m.width, m.height)
		if !view.AltScreen {
			t.Fatal("stats did not use alternate screen")
		}
		fmt.Fprintf(&corpus, "\n=== %s (%dx%d) ===\n", scenario, m.width, m.height)
		for _, line := range strings.Split(view.Content, "\n") {
			fmt.Fprintln(&corpus, strconv.QuoteToASCII(strings.TrimRight(line, " ")))
		}
	}
	const path = "testdata/stats.golden"
	if os.Getenv("SEI_TEST_UPDATE_VIEWS") == "1" {
		if err := os.WriteFile(path, []byte(corpus.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != corpus.String() {
		t.Fatalf("stats snapshot mismatch; review with SEI_TEST_UPDATE_VIEWS=1 go test -run '^TestStatsViewSnapshots$' ./internal\n%s", corpus.String())
	}
}

func TestStatsViewThemesAndKeys(t *testing.T) {
	var dark, light string
	for _, profile := range []string{"dark", "light", "no-color"} {
		m := populatedStatsModel()
		bg := color.Black
		if profile == "light" {
			bg = color.White
		}
		messages := []tea.Msg{tea.ColorProfileMsg{Profile: colorprofile.TrueColor}, tea.BackgroundColorMsg{Color: bg}}
		if profile == "no-color" {
			messages = append(messages, tea.EnvMsg{"NO_COLOR=1"})
		}
		for _, msg := range messages {
			next, _ := m.Update(msg)
			m = next.(statsModel)
		}
		view := m.View().Content
		if strings.ContainsRune(view, '\x1b') != (profile != "no-color") {
			t.Fatal("incorrect color profile")
		}
		for _, want := range []string{"1,248", "934 copies", "314 removals", "86", "active days", "42", "actions this month", "All time", "Last 30 days", "q / Ctrl+C quit"} {
			if !strings.Contains(ansi.Strip(view), want) {
				t.Fatalf("missing %q", want)
			}
		}
		switch profile {
		case "dark":
			dark = view
		case "light":
			light = view
		}
		if dir := os.Getenv("SEI_TEST_CAPTURES"); dir != "" {
			for _, size := range [][2]int{{80, 24}, {110, 30}} {
				m.width, m.height = size[0], size[1]
				name := fmt.Sprintf("stats-%s-%dx%d.ansi", profile, m.width, m.height)
				if err := os.WriteFile(filepath.Join(dir, name), []byte(m.View().Content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
		}
		m.summary.allTime = []skillCount{{"\x1b[2J\r\n\u202e\xff", 1}}
		for _, size := range [][2]int{{80, 24}, {100, 30}, {79, 23}, {0, 0}} {
			next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			m = next.(statsModel)
			view := m.View().Content
			assertViewBounds(t, view, size[0], size[1])
			for _, r := range ansi.Strip(view) {
				if (unicode.IsControl(r) && r != '\n') || unicode.Is(unicode.Cf, r) {
					t.Fatal("unsafe filename displayed")
				}
			}
			for _, msg := range []tea.Msg{tea.KeyPressMsg{Code: 'q'}, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}, exitRequestMsg{}} {
				_, cmd := m.Update(msg)
				if cmd == nil {
					t.Fatal("quit ignored")
				}
				if _, ok := cmd().(tea.QuitMsg); !ok {
					t.Fatal("quit not returned")
				}
			}
		}
	}
	if dark == light {
		t.Fatal("light and dark themes identical")
	}
}

func TestStatsRankingAlignment(t *testing.T) {
	for _, items := range [][]skillCount{
		{{"code-review", 1234}, {strings.Repeat("界", 30), 42}, {"bad\x1b\r\nname", 1}},
		{{"large-count", math.MaxInt64}, {"small-count", 1}},
	} {
		for _, width := range []int{37, 38, 49, 50} {
			view := statsRanking("All time", "Ranked by copies", items, width, true, uiStyles{}, uiStyles{}.accent)
			var countEdge int
			for _, line := range strings.Split(view, "\n") {
				if ansi.StringWidth(line) != width {
					t.Fatalf("misaligned border at width %d: %q", width, line)
				}
				if strings.Contains(line, "COPIES") {
					countEdge = ansi.StringWidth(strings.Split(line, "COPIES")[0]) + len("COPIES")
				}
				for i, item := range items {
					if !strings.HasPrefix(line, fmt.Sprintf("│ %d  ", i+1)) {
						continue
					}
					content := strings.TrimRight(strings.TrimSuffix(line, "│"), " ")
					if !strings.HasSuffix(content, statsNumber(item.count)) || ansi.StringWidth(content) != countEdge {
						t.Fatalf("count lost or misaligned: %q", line)
					}
				}
			}
		}
	}
}

func TestStatsBarProportions(t *testing.T) {
	for _, test := range []struct {
		count, total int64
		width, want  int
	}{
		{0, 0, 8, 0}, {0, 10, 8, 0}, {10, 10, 8, 8},
		{5, 10, 8, 4}, {1, 1000, 8, 1}, {1, 1, 0, 0},
		{math.MaxInt64 / 2, math.MaxInt64, 8, 4},
		{math.MaxInt64, math.MaxInt64, 8, 8},
	} {
		if got := statsBarCells(test.count, test.total, test.width); got != test.want {
			t.Errorf("bar(%d, %d, %d) = %d, want %d", test.count, test.total, test.width, got, test.want)
		}
	}
}

func TestStatsReadOnlyLoading(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "absent", "config.json.stats.json")
	m := statsModel{path: path}
	next, cmd := m.Update(m.Init()())
	m = next.(statsModel)
	if cmd != nil || !m.loaded || m.since != "" || m.fatal != nil {
		t.Fatalf("missing history: %+v", m)
	}
	setupAbsent(t, filepath.Join(root, "absent"))
	path = filepath.Join(root, "config.json.stats.json")
	writeTestFile(t, path, sampleStats)
	before := removeSnapshot(t, root)
	m.path = path
	next, cmd = m.Update(m.Init()())
	m = next.(statsModel)
	if cmd != nil || m.summary.copies != 6 || m.summary.removals != 2 || m.since != "2026-09-07" {
		t.Fatalf("loaded history: %+v", m)
	}
	if _, cmd := m.Update(tea.PasteMsg{Content: "qx"}); cmd != nil {
		t.Fatal("stats accepted paste")
	}
	assertRemoveSnapshot(t, root, before)
	writeTestFile(t, path, "broken")
	next, cmd = m.Update(m.Init()())
	if next.(statsModel).fatal == nil || cmd == nil {
		t.Fatal("read failure was hidden")
	}
}

func TestStatsCLIArguments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	writeTestFile(t, path, "invalid config should not be loaded by stats")
	for _, test := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"--config", path, "stats"}, 1, "requires terminal"},
		{[]string{"--config", path, "--project", path + ".missing", "stats"}, 1, "requires terminal"},
		{[]string{"stats", "--help"}, 2, "unexpected argument"},
		{[]string{"stats", "extra"}, 2, "unexpected argument"},
		{[]string{"--help", "stats"}, 0, "sei stats"},
	} {
		var out, stderr bytes.Buffer
		code := Run("test", test.args, strings.NewReader(""), &out, &stderr)
		if code != test.code || !strings.Contains(out.String()+stderr.String(), test.want) {
			t.Fatalf("args %v: code=%d out=%s stderr=%s", test.args, code, &out, &stderr)
		}
	}
}
