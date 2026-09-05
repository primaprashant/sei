package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func navigationModel(count int) browseModel {
	cfg := config{Library: "/library"}
	for i := range count {
		cfg.Agents = append(cfg.Agents, agentConfig{fmt.Sprintf("Agent%d", i+1), fmt.Sprintf("/global%d", i+1), fmt.Sprintf("/local%d", i+1)})
	}
	m := newBrowseModel(cfg)
	for i := range m.panels {
		m.panels[i].loading = false
		m.panels[i].entries = []skillEntry{{name: "a"}, {name: "line\nb"}, {name: "z"}}
		m.panels[i].selectedName = "a"
	}
	return m
}

func press(m browseModel, code rune) (browseModel, tea.Cmd) {
	updated, cmd := m.Update(tea.KeyPressMsg{Code: code})
	return updated.(browseModel), cmd
}

func TestNavigation(t *testing.T) {
	for _, count := range []int{1, 3, 9} {
		m := navigationModel(count)
		for slot := 1; slot <= 9; slot++ {
			before := m.focused
			m, _ = press(m, rune('0'+slot))
			want := before
			if slot <= count {
				want = count + slot
			}
			if m.focused != want {
				t.Fatalf("local %d/%d: %d", slot, count, m.focused)
			}
			m, _ = press(m, 'g')
			m, _ = press(m, rune('0'+slot))
			if slot <= count {
				want = slot
			}
			if m.focused != want {
				t.Fatalf("global %d/%d: %d", slot, count, m.focused)
			}
		}
		for id := range m.panels {
			m.focused = id
			m, _ = press(m, tea.KeyUp)
			if m.panels[id].selected != 0 {
				t.Fatal("up wrapped")
			}
			for range 5 {
				m, _ = press(m, tea.KeyDown)
			}
			if m.panels[id].selectedName != "z" {
				t.Fatal("down did not clamp")
			}
			m, _ = press(m, tea.KeyUp)
			if m.panels[id].selectedName != "line\nb" {
				t.Fatal("lost raw name")
			}
			m.width, m.height = 100, 30
			view := m.View().Content
			if !strings.Contains(view, "> line\\nb") || !strings.Contains(view, "> [") {
				t.Fatalf("focused selection hidden: %s", view)
			}
		}
		m.panels[0].err = fmt.Errorf("persistent failure")
		m, _ = press(m, '0')
		if m.focused != 0 || m.panels[0].err == nil {
			t.Fatal("focus/error lost")
		}
		for _, p := range m.panels {
			if p.selectedName != "line\nb" {
				t.Fatal("selection memory lost")
			}
		}
		m.panels[0].entries, m.panels[0].selectedName = nil, ""
		m, _ = press(m, tea.KeyDown)
		if m.panels[0].selectedName != "" {
			t.Fatal("empty selection actionable")
		}
	}
}

func TestKeySequence(t *testing.T) {
	for _, continuation := range "0gabcdefhioABCDEFHIOXxr?9" {
		m := navigationModel(3)
		m, _ = press(m, 'g')
		before := m
		for _, msg := range []tea.Msg{tea.PasteStartMsg{}, tea.PasteMsg{Content: "1qXr?"}, tea.PasteEndMsg{}, tea.WindowSizeMsg{Width: m.width, Height: m.height}, struct{}{}} {
			updated, cmd := m.Update(msg)
			m = updated.(browseModel)
			if cmd != nil || !reflect.DeepEqual(before, m) {
				t.Fatal("pending sequence changed without a key")
			}
		}
		if !strings.Contains(m.View().Content, "g pending") {
			t.Fatal("pending g invisible")
		}
		m, cmd := press(m, continuation)
		before.pendingGlobal = false
		if cmd != nil || !reflect.DeepEqual(before, m) {
			t.Fatalf("continuation %q replayed", continuation)
		}
	}
	for _, help := range []bool{false, true} {
		m := navigationModel(9)
		m.showHelp, m.pendingGlobal = help, true
		closed, cmd := press(m, tea.KeyEscape)
		if cmd != nil || closed.showHelp || closed.pendingGlobal {
			t.Fatal("Esc failed")
		}
		for _, key := range []tea.KeyPressMsg{{Code: 'q'}, {Code: 'c', Mod: tea.ModCtrl}} {
			_, cmd := m.Update(key)
			if cmd == nil {
				t.Fatal("quit lost priority")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatal("not quit")
			}
		}
		m.pendingGlobal = false
		for _, key := range "abcdefhioABCDEFHIOXx" {
			updated, cmd := press(m, key)
			if cmd != nil || !reflect.DeepEqual(m, updated) {
				t.Fatalf("mutation %c enabled", key)
			}
		}
	}
}

func TestHelp(t *testing.T) {
	m := navigationModel(9)
	m.focused = 9
	m.panels[9].path = "/" + strings.Repeat("long path界/", 200) + "\x1b[31mENDPATH"
	m.panels[9].selectedName = strings.Repeat("long-name", 200) + "\nENDNAME"
	m.width, m.height = 42, 10
	m, _ = press(m, '?')
	var inspected strings.Builder
	for range len(m.helpLines()) {
		view := m.View().Content
		inspected.WriteString(strings.Split(view, "\n")[0])
		if strings.ContainsRune(view, '\x1b') {
			t.Fatal("unsafe help")
		}
		m, _ = press(m, tea.KeyDown)
	}
	// Include the final viewport, which remains clamped at the end.
	inspected.WriteString(strings.ReplaceAll(m.View().Content, "\n", ""))
	for _, want := range []string{displayText(m.panels[9].path), displayText(m.panels[9].selectedName), "Other agents may also load skills from these folders. sei shows configured folder contents, not everything an agent discovers or has loaded."} {
		if !strings.Contains(inspected.String(), want) {
			t.Fatalf("full help text not inspectable: %q", want)
		}
	}
	for i, key := range "abcdefhio" {
		text := strings.Join(m.helpLines(), "")
		want := fmt.Sprintf("%c: Agent%d / Local; %s: Agent%d / Global", key, i+1, strings.ToUpper(string(key)), i+1)
		if !strings.Contains(text, want) {
			t.Fatalf("wrong mapping: %s", want)
		}
	}
	m, _ = press(m, '?')
	if m.showHelp {
		t.Fatal("help did not toggle closed")
	}
}

func TestRefresh(t *testing.T) {
	for _, names := range [][]string{{"0", "a", "line\nb", "z"}, {"a", "z"}, {"a"}, nil} {
		m := navigationModel(3)
		for i := range m.panels {
			m.panels[i].selected, m.panels[i].selectedName = 1, "line\nb"
		}
		m.focused = 6
		m, cmd := press(m, 'r')
		if cmd == nil || len(cmd().(tea.BatchMsg)) != len(m.panels) {
			t.Fatal("refresh did not schedule scans")
		}
		var entries []skillEntry
		for _, name := range names {
			entries = append(entries, skillEntry{name: name})
		}
		for i := len(m.panels) - 1; i >= 0; i-- {
			updated, _ := m.Update(scanResult{panel: panelID(i), generation: m.panels[i].generation, entries: entries})
			m = updated.(browseModel)
		}
		want := ""
		if len(names) > 0 {
			want = names[min(1, len(names)-1)]
		}
		if len(names) == 4 {
			want = "line\nb"
		}
		for _, p := range m.panels {
			if p.selectedName != want {
				t.Fatalf("refresh %q: got %q want %q", names, p.selectedName, want)
			}
		}
		if m.focused != 6 {
			t.Fatal("refresh changed focus")
		}
	}
}
