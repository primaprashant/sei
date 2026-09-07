package app

import (
	"bytes"
	"fmt"
	"image/color"
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
	for _, scenario := range []string{"populated", "desktop", "empty", "loading", "undersized", "long-names"} {
		m := populatedStatsModel()
		switch scenario {
		case "desktop":
			m.width, m.height = 110, 30
		case "empty", "loading":
			m.since, m.summary, m.loaded = "", statsSummary{}, scenario == "empty"
		case "undersized":
			m.width, m.height = 79, 23
		case "long-names":
			m.summary.allTime = []skillCount{{strings.Repeat("界", 50), 1234}, {"bad\xff\x1b[2J\r\nname", 2}}
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
		for _, want := range []string{"1,248", "934 copies", "314 removals", "86 active days", "42 actions this month", "All time", "Last 30 days", "q / Ctrl+C quit"} {
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
