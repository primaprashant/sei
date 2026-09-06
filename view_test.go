package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestLayout(t *testing.T) {
	for count := 1; count <= 9; count++ {
		for _, size := range [][2]int{{80, 24}, {100, 30}, {143, 35}, {148, 39}, {180, 60}} {
			m := navigationModel(count)
			m.width, m.height = size[0], size[1]
			for i := range m.panels {
				m.panels[i].safetyChecked = true
				if i > 0 {
					m.panels[i].label = "Claude Code / " + strings.Split(m.panels[i].label, " / ")[1]
				}
			}
			for id := range m.panels {
				m.focused = id
				view := ansi.Strip(m.View().Content)
				lines := strings.Split(view, "\n")
				if len(lines) > m.height {
					t.Fatalf("%d agents %v: height %d", count, size, len(lines))
				}
				for _, line := range lines {
					if ansi.StringWidth(line) > m.width {
						t.Fatalf("%d agents %v: overflow %q", count, size, line)
					}
				}
				for _, want := range []string{"* " + m.panels[id].label, m.labeledPanel(id).hint, "> a", "Agents ", "Ready"} {
					if !strings.Contains(view, want) {
						t.Fatalf("%d agents %v focus %d missing %q:\n%s", count, size, id, want, view)
					}
				}
				if strings.Index(view, " / Local") > strings.Index(view, " / Global") || strings.Contains(view, "Root relations checked") {
					t.Fatal("scope order or quiet status regressed")
				}
				if id > 0 {
					slot := (id-1)%count + 1
					var first, last, total int
					if _, err := fmt.Sscanf(lines[0], "sei | Configured folders | Agents %d-%d / %d", &first, &last, &total); err != nil || slot < first || slot > last || total != count {
						t.Fatalf("focused slot hidden: %q", lines[0])
					}
				}
			}
		}
	}
}

func TestLayoutPaths(t *testing.T) {
	m := navigationModel(1)
	m.config.Home, m.config.Project = "/home/test", "/project"
	m.panels[0].path, m.panels[1].path, m.panels[2].path = "/home/test/library", "/home/test/.claude/skills", "/project/.claude/skills"
	for id, want := range []string{"~/library", "~/.claude/skills", ".claude/skills"} {
		before := m.panels[id]
		if got := m.labeledPanel(id).path; got != want || !reflect.DeepEqual(before, m.panels[id]) {
			t.Fatalf("compact path %q want %q; raw panel changed", got, want)
		}
		m.focused = id
		if !strings.Contains(strings.Join(m.helpLines(), ""), "Root path: "+before.path) {
			t.Fatal("full path lost in help")
		}
	}
	for _, path := range []string{"/home/testing/skills", "/elsewhere/skills"} {
		m.panels[1].path = path
		if m.labeledPanel(1).path != path {
			t.Fatal("unrelated path shortened")
		}
	}
}

func TestLayoutBounds(t *testing.T) {
	for count := 1; count <= 9; count++ {
		m := navigationModel(count)
		for _, width := range []int{80, 100, 143, 148, 180} {
			for height := 24; height <= 65; height++ {
				m.width, m.height = width, height
				for _, id := range []int{0, count, 2 * count} {
					m.focused = id
					lines := strings.Split(m.View().Content, "\n")
					if len(lines) > height {
						t.Fatalf("count=%d %dx%d focus=%d: got %d lines\n%s", count, width, height, id, len(lines), m.View().Content)
					}
				}
			}
		}
	}
}

func TestLayoutHelp(t *testing.T) {
	m := navigationModel(9)
	m.width, m.height, m.showHelp, m.pendingGlobal = 80, 24, true, true
	for _, state := range []string{"idle", "busy", "quit"} {
		if state != "idle" {
			m.active = &mutationRequest{id: 1}
		}
		m.pendingQuit = state == "quit"
		for _, offset := range []int{0, 20, 1000} {
			m.helpOffset = offset
			view := m.View().Content
			lines := strings.Split(view, "\n")
			if len(lines) > m.height || !strings.Contains(view, "g pending") || !strings.Contains(view, "Esc cancel") {
				t.Fatalf("help footer hidden: %s", view)
			}
			for _, line := range lines {
				if ansi.StringWidth(line) > m.width {
					t.Fatalf("help overflow: %q", line)
				}
			}
		}
	}
}

func TestResize(t *testing.T) {
	for _, add := range []bool{false, true} {
		for _, size := range [][2]int{{79, 24}, {80, 23}, {0, 0}, {-1, -1}} {
			m := mutationModel(t)
			key := 'x'
			if add {
				m.focused, key = 0, 'a'
			}
			before := removeSnapshot(t, m.config.Home)
			next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			m = next.(browseModel)
			m, cmd := press(m, key)
			if cmd != nil || m.active != nil || m.nextOperation != 0 {
				t.Fatal("undersized window started work")
			}
			if size[0] > 0 && !strings.Contains(m.View().Content, "Resize to at least 80x24") {
				t.Fatal("missing resize warning")
			}
			assertRemoveSnapshot(t, m.config.Home, before)
			if _, quit := press(m, 'q'); quit == nil {
				t.Fatal("undersized quit ignored")
			}
			next, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			m, worker := press(next.(browseModel), key)
			if worker == nil || m.active == nil {
				t.Fatal("minimum-size action unavailable")
			}
			request := *m.active
			for _, size := range [][2]int{{79, 24}, {148, 39}, {80, 23}} {
				next, cmd = m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
				m = next.(browseModel)
				m, extra := press(m, key)
				if cmd != nil || extra != nil || m.active == nil || *m.active != request || m.nextOperation != 1 || !strings.Contains(m.View().Content, "Working; q waits for completion") {
					t.Fatal("resize canceled or duplicated active work")
				}
			}
			result := worker().(mutationResult)
			if result.err != nil {
				t.Fatal(result.err)
			}
			next, refresh := m.Update(result)
			m = finishMutationRefresh(t, next.(browseModel), refresh)
			if _, cmd := press(m, key); cmd != nil {
				t.Fatal("completion reenabled undersized mutation")
			}
			next, _ = m.Update(tea.WindowSizeMsg{Width: 143, Height: 35})
			m = next.(browseModel)
			if strings.Contains(m.View().Content, "Resize to") || m.active != nil || m.nextOperation != 1 {
				t.Fatal("resize after completion lost state")
			}
		}
	}
}

func TestViewSnapshots(t *testing.T) {
	var corpus strings.Builder
	corpus.WriteString("Rendered frames: right padding removed; Go escapes encode Unicode/backslashes.\nCell bounds are asserted before encoding. Not terminal screenshots.\n")
	for _, scenario := range []string{"desktop-empty", "minimum-mixed", "busy-help", "busy-quit", "undersized", "nine-agent-focus"} {
		m := navigationModel(3)
		m.width, m.height = 143, 35
		m.config.Home, m.config.Project = "/home/example", "/project"
		m.panels[0].path = "/home/example/library"
		for i, name := range []string{"Claude Code", "Codex", "OpenCode"} {
			for _, id := range []int{1 + i, 4 + i} {
				p := &m.panels[id]
				scope, path := "Global", fmt.Sprintf("/home/example/.agent%d/skills", i+1)
				if id > 3 {
					scope, path = "Local", fmt.Sprintf("/project/.agent%d/skills", i+1)
				}
				p.label, p.path = name+" / "+scope, path
				p.entries, p.selectedName, p.missing = nil, "", true
			}
		}
		for i := range m.panels {
			m.panels[i].safetyChecked = true
		}
		m.panels[0].entries = []skillEntry{{name: ".dot"}, {name: "e\u0301-\u754c-wide"}, {name: strings.Repeat("long-name-", 6)}, {name: "look-safe\x1b[2J\r\nspoof"}, {name: "linked", blocked: true}}
		m.panels[0].selectedName = ".dot"
		switch scenario {
		case "minimum-mixed":
			m.width, m.height = 80, 24
			m.panels[0].selected, m.panels[0].selectedName = 3, m.panels[0].entries[3].name
			m.panels[4].missing = false // Distinguish empty from absent.
			m.panels[1].err = errors.New("denied\x1b]52;c;payload\a")
			m.panels[1].safetyErr = errors.New("unverifiable root")
		case "busy-help", "busy-quit":
			m.width, m.height = 80, 24
			m.active = &mutationRequest{id: 1, name: "look-safe\x1b[2J\r\nspoof", label: "Claude Code / Local", path: "/project/" + strings.Repeat("long-path/", 8), add: true}
			m.status = m.active.target() + ": working"
			m.showHelp = scenario == "busy-help"
			m.pendingQuit, m.pendingGlobal = !m.showHelp, m.showHelp
			if m.showHelp {
				m.helpOffset = len(m.helpLines()) // Snapshot the result/footer end.
			}
		case "undersized":
			m.width, m.height, m.pendingQuit = 79, 23, true
			m.active = &mutationRequest{id: 1}
		case "nine-agent-focus":
			m = navigationModel(9)
			m.width, m.height, m.focused = 80, 24, 18
			for i := range m.panels {
				m.panels[i].safetyChecked = true
			}
		}
		view := ansi.Strip(m.View().Content)
		lines := strings.Split(view, "\n")
		if len(lines) > m.height {
			t.Fatalf("%s height overflow", scenario)
		}
		fmt.Fprintf(&corpus, "\n=== %s (%dx%d) ===\n", scenario, m.width, m.height)
		for _, line := range lines {
			if ansi.StringWidth(line) > m.width {
				t.Fatalf("%s width overflow: %q", scenario, line)
			}
			quoted := strconv.QuoteToASCII(strings.TrimRight(line, " "))
			corpus.WriteString(quoted[1:len(quoted)-1] + "\n")
		}
	}
	const path = "testdata/views.golden"
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
		t.Fatalf("view snapshot mismatch; review with SEI_TEST_UPDATE_VIEWS=1 go test -run '^TestViewSnapshots$' .\n%s", corpus.String())
	}
}

func TestDisplaySafety(t *testing.T) {
	var controls strings.Builder
	for r := rune(0); r <= 0x9f; r++ {
		if unicode.IsControl(r) {
			controls.WriteRune(r)
		}
	}
	for _, raw := range []string{controls.String(), "\x1b]52;c;clipboard\a\x1b]8;;https://example.invalid\x1b\\link", "\u202e\u2066\u200b\u2028\u2029", "\u0301leading-\u754c-e\u0301", "bad\xff", "quote\"literal\\n", strings.Repeat("long \u754c/", 80)} {
		safe := displayText(raw)
		decoded, err := strconv.Unquote(`"` + safe + `"`)
		if err != nil || decoded != raw {
			t.Fatalf("display escaping changed identity: %q", raw)
		}
		m := navigationModel(1)
		m.width, m.height = 80, 24
		m.panels[0].label, m.panels[0].path, m.panels[0].selectedName = raw, raw, raw
		m.panels[0].entries = []skillEntry{{name: raw}}
		m.panels[0].err, m.panels[0].safetyErr, m.status = errors.New(raw), errors.New(raw), raw
		m.panels[0].safetyChecked = true
		for _, help := range []bool{false, true} {
			m.showHelp = help
			view := m.View().Content
			if !utf8.ValidString(view) {
				t.Fatal("invalid UTF-8 in view")
			}
			for _, r := range view {
				if (unicode.IsControl(r) && r != '\n') || unicode.Is(unicode.Cf, r) {
					t.Fatalf("unsafe displayed rune %U for %q: %q", r, raw, view)
				}
			}
			for _, line := range strings.Split(view, "\n") {
				if ansi.StringWidth(line) > m.width {
					t.Fatalf("unsafe cell width: %q", line)
				}
			}
		}
		full := strings.Join(m.helpLines(), "")
		for _, field := range []string{"Focused: ", "Root path: ", "Selected name: ", "Error: ", "Root safety blocked: ", "Result: "} {
			if !strings.Contains(full, field+safe) {
				t.Fatalf("full %s not inspectable", field)
			}
		}
		if m.panels[0].selectedName != raw || m.panels[0].path != raw || m.panels[0].label != raw {
			t.Fatal("view rewrote raw model state")
		}
	}
}

func TestViewProfiles(t *testing.T) {
	m := navigationModel(3)
	m.width, m.height = 80, 24
	for _, profile := range []string{"light", "dark", "no-color"} {
		t.Run(profile, func(t *testing.T) {
			t.Setenv("NO_COLOR", "")
			t.Setenv("COLORFGBG", "0;15")
			if profile == "dark" {
				t.Setenv("COLORFGBG", "15;0")
			}
			if profile == "no-color" {
				t.Setenv("NO_COLOR", "1")
			}
			view := m.View().Content
			if strings.ContainsRune(view, '\x1b') {
				t.Fatal("text presentation depends on fixed colors or control sequences")
			}
			for _, want := range []string{"* Library", "> a", "focus 1 | add a", "Local", "Global", "q / ctrl+c quit"} {
				if !strings.Contains(view, want) {
					t.Fatalf("%s missing no-color cue %q", profile, want)
				}
			}
		})
	}
}

func TestDisplaySafetyRawNames(t *testing.T) {
	for _, raw := range []string{"line\nname", "look-safe\x1b[2J", "\u202ehidden", "wide-\u754c-e\u0301", ".dot", "bad\xff"} {
		t.Run(displayText(raw), func(t *testing.T) {
			cfg := rootFixture(t)
			if !utf8.ValidString(raw) {
				requireInvalidUTF8Names(t, cfg.Library)
			}
			writeTestFilePath := func(path, text string) {
				t.Helper()
				browseMkdir(t, filepath.Dir(path))
				writeTestFile(t, path, text)
			}
			writeTestFilePath(filepath.Join(cfg.Library, raw, "file"), "source bytes")
			base := cfg.Agents[0].Local
			decoy := displayText(raw)
			if decoy == raw {
				decoy = "other"
			}
			writeTestFilePath(filepath.Join(base, decoy, "file"), "untouched decoy")
			library, other := removeSnapshot(t, cfg.Library), removeSnapshot(t, filepath.Join(base, decoy))
			m := newBrowseModel(cfg)
			next, refresh := m.Update(m.Init()())
			m = finishMutationRefresh(t, next.(browseModel), refresh)
			for _, key := range []rune{'a', 'a', 'x'} {
				if key == 'x' {
					m.focused = m.agents + 1
					p := &m.panels[m.focused]
					for i, e := range p.entries {
						if e.name == raw {
							p.selected, p.selectedName = i, raw
							break
						}
					}
				}
				if strings.ContainsRune(m.View().Content, '\x1b') {
					t.Fatal("raw filename emitted terminal controls in browse")
				}
				m, _ = press(m, '?')
				if strings.ContainsRune(m.View().Content, '\x1b') {
					t.Fatal("raw filename emitted terminal controls in help")
				}
				if !strings.Contains(strings.Join(m.helpLines(), ""), "Selected name: "+displayText(raw)) {
					t.Fatal("help lost exact raw target")
				}
				m, _ = press(m, '?')
				p := m.panels[m.focused]
				if p.selectedName != raw || p.entries[p.selected].name != raw {
					t.Fatal("rendering changed raw selected/entry name")
				}
				var worker tea.Cmd
				m, worker = press(m, key)
				if worker == nil || m.active.name != raw {
					t.Fatal("escaped spelling substituted for raw action name")
				}
				result := worker().(mutationResult)
				if result.err != nil {
					t.Fatal(result.err)
				}
				next, refresh = m.Update(result)
				m = finishMutationRefresh(t, next.(browseModel), refresh)
				if key == 'a' {
					data, err := os.ReadFile(filepath.Join(base, raw, "file"))
					if err != nil || string(data) != "source bytes" {
						t.Fatalf("wrong copied target: %q %v", data, err)
					}
					writeTestFile(t, filepath.Join(base, raw, "file"), "local edit for replacement/removal")
				}
				assertRemoveSnapshot(t, cfg.Library, library)
				assertRemoveSnapshot(t, filepath.Join(base, decoy), other)
			}
			setupAbsent(t, filepath.Join(base, raw))
		})
	}
}
