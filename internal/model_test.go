package app

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func browseMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
}

func TestBrowseTree(t *testing.T) {
	root := t.TempDir()
	names := []string{"z", "A", ".hidden", ".github", ".git-extra", "without-manifest", "nested", "\x1b[31mred", "line\nbreak", "tab\tname", "\x7f\u009b", "界", "combining-e\u0301", "accent-é", "\u202ename", `literal\n`}
	for _, name := range names {
		browseMkdir(t, filepath.Join(root, name))
	}
	browseMkdir(t, filepath.Join(root, ".git"))
	writeTestFile(t, filepath.Join(root, ".git", "config"), "repository metadata")
	browseMkdir(t, filepath.Join(root, "nested", "not-a-top-level-skill"))
	writeTestFile(t, filepath.Join(root, "SKILL.md"), "ignored loose file")
	writeTestFile(t, filepath.Join(root, "nested", "script.sh"), "must not execute")
	for name, target := range map[string]string{"dir-link": "nested", "file-link": "SKILL.md", "dangling-link": "absent"} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	slices.Sort(names)
	entries, missing, err := scanFolder(root)
	if err != nil || missing || len(entries) != len(names) {
		t.Fatalf("scan = %+v, missing %v, error %v", entries, missing, err)
	}
	for i, entry := range entries {
		if entry.name != names[i] || entry.blocked != strings.HasSuffix(entry.name, "-link") {
			t.Errorf("entry %d = %+v, want raw %q with correct link status", i, entry, names[i])
		}
	}
	m := newBrowseModel(config{Library: root, Agents: []agentConfig{{Name: "Example", Global: root, Local: root}}})
	updated, cmd := m.Update(m.Init()())
	m = updated.(browseModel)
	result := cmd().(tea.BatchMsg)[0]().(scanResult)
	updated, _ = m.Update(result)
	m = updated.(browseModel)
	if m.panels[0].selectedName != names[0] || m.panels[0].selectedName == displayText(names[0]) {
		t.Fatalf("raw selected control name lost: %q", m.panels[0].selectedName)
	}
	for _, entry := range m.panels[0].entries {
		if _, err := os.Lstat(filepath.Join(root, entry.name)); err != nil {
			t.Fatalf("raw name no longer addresses source: %q: %v", entry.name, err)
		}
	}
	if data, err := os.ReadFile(filepath.Join(root, ".git", "config")); err != nil || string(data) != "repository metadata" {
		t.Fatalf("repository metadata changed: %q, %v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "nested", "script.sh")); err != nil || string(data) != "must not execute" {
		t.Fatalf("source content changed: %q, %v", data, err)
	}
}

func TestBrowseRoots(t *testing.T) {
	for _, kind := range []string{"empty", "missing", "missing parents", "file", "file ancestor", "dangling", "dangling ancestor", "loop", "root alias", "raw link dotdot", "unreadable", "unsearchable ancestor"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "folder")
			wantMissing, wantError := false, false
			switch kind {
			case "empty":
				browseMkdir(t, path)
			case "missing", "missing parents":
				wantMissing = true
				if kind == "missing parents" {
					path += "/a/b"
				}
			case "file", "file ancestor":
				writeTestFile(t, path, "not a directory")
				wantError = true
				if kind == "file ancestor" {
					path += "/child"
				}
			case "dangling", "dangling ancestor", "loop", "root alias", "raw link dotdot":
				target := filepath.Join(root, "absent")
				wantError = true
				if kind == "loop" {
					target = path
				}
				if kind == "root alias" || kind == "raw link dotdot" {
					target = filepath.Join(root, "real", "child")
					browseMkdir(t, target)
					wantError = false
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
				if kind == "dangling ancestor" {
					path += "/child"
				}
				if kind == "raw link dotdot" {
					path += "/../child"
				}
			case "unreadable", "unsearchable ancestor":
				browseMkdir(t, path)
				protected := path
				if kind == "unsearchable ancestor" {
					protected = root
				}
				if err := os.Chmod(protected, 0); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Chmod(protected, 0o700); err != nil {
						t.Error(err)
					}
				})
				if _, err := os.ReadDir(path); err == nil {
					t.Skip("current user can read/search mode-000 directories (e.g. root)")
				}
				wantError = true
			}
			entries, missing, err := scanFolder(path)
			if missing != wantMissing || (err != nil) != wantError || len(entries) != 0 {
				t.Fatalf("scan %q = %+v, missing %v, error %v; want missing %v, error %v", path, entries, missing, err, wantMissing, wantError)
			}
			if wantMissing {
				if _, err := os.Lstat(filepath.Join(root, "folder")); !os.IsNotExist(err) {
					t.Fatalf("scan created destination: %v", err)
				}
			}
		})
	}
}

func TestBrowseCommandsAndGenerations(t *testing.T) {
	root := t.TempDir()
	cfg := config{Library: filepath.Join(root, "missing-library")}
	for i := range 3 {
		cfg.Agents = append(cfg.Agents, agentConfig{fmt.Sprintf("Agent %d", i), filepath.Join(root, fmt.Sprintf("global%d", i)), filepath.Join(root, fmt.Sprintf("local%d", i))})
	}
	m := newBrowseModel(cfg)
	updated, cmd := m.Update(m.Init()())
	m = updated.(browseModel)
	old := cmd().(tea.BatchMsg)
	if len(old) != 8 {
		t.Fatalf("got %d independent commands", len(old))
	}
	// Trees appear only after construction, Init, Update, View and command creation.
	_ = m.View()
	browseMkdir(t, filepath.Join(cfg.Agents[0].Global, "global-only"))
	browseMkdir(t, filepath.Join(cfg.Agents[2].Local, "local-only"))
	writeTestFile(t, cfg.Agents[1].Global, "invalid root")
	oldLibrary := old[0]().(scanResult)
	updated, cmd = m.Update(startBrowseMsg{})
	m = updated.(browseModel)
	newCommands := cmd().(tea.BatchMsg)
	for _, id := range []int{6, 2, 4, 1, 5, 3, 0} {
		result := newCommands[id]().(scanResult)
		if result.panel != panelID(id) || result.generation != 2 {
			t.Fatalf("wrong captured identity: %+v", result)
		}
		if id == 0 {
			// Stale and impossible future results must not finish this pending scan.
			for _, stale := range []scanResult{oldLibrary, {panel: 0, generation: 3}, {panel: -1, generation: 2}, {panel: 99, generation: 2}} {
				before := m
				updated, _ = m.Update(stale)
				m = updated.(browseModel)
				if !reflect.DeepEqual(before, m) {
					t.Fatalf("rejected result changed model: %+v", stale)
				}
			}
		}
		updated, _ = m.Update(result)
		m = updated.(browseModel)
	}
	if !m.panels[0].missing || m.panels[2].err == nil || !m.panels[4].missing || m.panels[1].selectedName != "global-only" || m.panels[6].selectedName != "local-only" {
		t.Fatalf("panels not independent: %+v", m.panels)
	}
	before := m
	for _, result := range []scanResult{old[1]().(scanResult), {panel: 1, generation: 2, missing: true}} {
		updated, _ = m.Update(result)
		m = updated.(browseModel)
		if !reflect.DeepEqual(before, m) {
			t.Fatal("stale or duplicate completion overwrote finished panel")
		}
	}
	for _, input := range "abcdefhioABCDEFHIOXx" {
		updated, cmd = m.Update(tea.KeyPressMsg{Code: input})
		if cmd != nil || !reflect.DeepEqual(updated, m) {
			t.Fatalf("mutation key %q changed model before safety checks completed", input)
		}
	}
	m.width = 143 // Show all three agents.
	view := m.View().Content
	for _, text := range []string{"Library folder missing", "Folder not created", "Error:", "global-only", "local-only"} {
		if !strings.Contains(view, text) {
			t.Errorf("view missing %q", text)
		}
	}
	if _, err := os.Lstat(cfg.Agents[0].Local); !os.IsNotExist(err) {
		t.Fatalf("missing local created: %v", err)
	}
	// An invalid (rather than merely absent) library also leaves destinations usable.
	writeTestFile(t, cfg.Library, "invalid library root")
	updated, cmd = m.Update(startBrowseMsg{})
	m = updated.(browseModel)
	for _, scan := range cmd().(tea.BatchMsg) {
		updated, _ = m.Update(scan())
		m = updated.(browseModel)
	}
	if m.panels[0].err == nil || m.panels[0].missing || m.panels[6].selectedName != "local-only" {
		t.Fatalf("invalid library interfered with destinations: %+v", m.panels)
	}
}

func TestBrowseDisplay(t *testing.T) {
	for _, raw := range []string{"\x1b[31m\n\r\t\x00\x7f\u009b\u202e", "界界界", "e\u0301e\u0301", "bad\xff", `literal\n`} {
		safe := displayText(raw)
		if !utf8.ValidString(safe) || strings.ContainsAny(safe, "\x1b\n\r\t\x00\x7f\u009b\u202e") {
			t.Fatalf("unsafe text: %q", safe)
		}
		decoded, err := strconv.Unquote(`"` + safe + `"`)
		if err != nil || decoded != raw {
			t.Fatalf("escaping lost identity: %q => %q, %v", raw, decoded, err)
		}
		p := browsePanel{label: raw, path: raw, selectedName: raw, entries: []skillEntry{{name: raw, blocked: true}}}
		for width := 1; width <= 40; width++ {
			for _, line := range strings.Split((uiStyles{}).panelView(p, true, true, width, 4), "\n") {
				if ansi.StringWidth(line) != width || !utf8.ValidString(line) || strings.ContainsRune(line, '\x1b') {
					t.Fatalf("width %d: unsafe or wrong cell width: %q (%d)", width, line, ansi.StringWidth(line))
				}
			}
		}
		p.err = fmt.Errorf("read %s", raw)
		if view := (uiStyles{}).panelView(p, true, true, 100, 6); strings.ContainsAny(view, "\x1b\r\t\x00\x7f\u009b\u202e") || !strings.Contains(view, "Error:") {
			t.Fatalf("unsafe error display: %q", view)
		}
	}
	if displayText("界e\u0301") != "界e\u0301" {
		t.Fatal("printable Unicode unnecessarily escaped")
	}
	for _, count := range []int{1, 3, 9} {
		cfg := config{Library: "/library"}
		for i := range count {
			cfg.Agents = append(cfg.Agents, agentConfig{fmt.Sprintf("Agent%d", i), fmt.Sprintf("/global%d", i), fmt.Sprintf("/local%d", i)})
		}
		m := newBrowseModel(cfg)
		m.width, m.height = 180, 60
		view := m.View().Content
		libraryAt, projectAt, globalAt := strings.Index(view, "LIBRARY"), strings.Index(view, "PROJECT"), strings.Index(view, "GLOBAL")
		if libraryAt < 0 || projectAt < libraryAt || globalAt < projectAt {
			t.Fatal("scope order lost")
		}
		for i := range count {
			name := fmt.Sprintf("Agent%d", i)
			if !strings.Contains(view[projectAt:globalAt], name) || !strings.Contains(view[globalAt:], name) {
				t.Fatalf("missing agent %s", name)
			}
		}
	}
}
