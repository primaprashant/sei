package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

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
