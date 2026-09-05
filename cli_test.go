package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestCLI(t *testing.T) {
	isolateConfigHome(t)
	path, err := configLocation(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, validConfig)
	for _, tt := range []struct {
		name string
		args []string
		code int
		out  string
		err  string
	}{
		{"help", []string{"--help"}, 0, "Usage: sei", ""},
		{"short help", []string{"-h"}, 0, "Usage: sei", ""},
		{"version", []string{"--version"}, 0, "sei dev\n", ""},
		{"unknown flag", []string{"--unknown"}, 2, "", "flag provided but not defined"},
		{"unknown command", []string{"unknown"}, 2, "", "unexpected argument"},
		{"unimplemented setup", []string{"setup"}, 1, "", "setup is not implemented"},
		{"flags before setup", []string{"--config", path, "--project", ".", "setup"}, 1, "", "setup is not implemented"},
		{"flag after setup", []string{"setup", "--help"}, 2, "", "unexpected argument"},
		{"extra setup argument", []string{"setup", "extra"}, 2, "", "unexpected argument"},
		{"no headless add", []string{"add"}, 2, "", "unexpected argument"},
		{"no headless remove", []string{"remove"}, 2, "", "unexpected argument"},
		{"missing config value", []string{"--config"}, 2, "", "flag needs an argument"},
		{"missing project value", []string{"--project"}, 2, "", "flag needs an argument"},
		{"empty config", []string{"--config="}, 2, "", "must not be empty"},
		{"empty project", []string{"--project="}, 2, "", "must not be empty"},
		{"positional after help", []string{"--help", "extra"}, 2, "", "unexpected argument"},
		{"flag after positional", []string{"extra", "--version"}, 2, "", "unexpected argument"},
		{"invalid boolean", []string{"--version=maybe"}, 2, "", "invalid boolean value"},
		{"non terminal", nil, 1, "", "requires terminal stdin and stdout"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, strings.NewReader(""), &stdout, &stderr); got != tt.code {
				t.Fatalf("status = %d, want %d; stderr: %s", got, tt.code, &stderr)
			}
			for _, stream := range []struct{ got, want string }{
				{stdout.String(), tt.out}, {stderr.String(), tt.err},
			} {
				if (stream.want == "" && stream.got != "") || !strings.Contains(stream.got, stream.want) {
					t.Errorf("output = %q, want %q", stream.got, stream.want)
				}
			}
		})
	}
}

func TestCLIConfig(t *testing.T) {
	isolateConfigHome(t)
	for _, kind := range []string{"valid", "missing", "missing parent", "malformed", "invalid paths", "directory", "file ancestor", "unreadable", "unreadable ancestor", "dangling", "dangling ancestor", "valid symlink", "symlink loop"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "config.json")
			want := "requires terminal stdin and stdout"
			setupAllowed := false
			switch kind {
			case "valid":
				writeTestFile(t, path, validConfig)
			case "missing", "missing parent":
				if kind == "missing parent" {
					path = filepath.Join(root, "absent", "config.json")
				}
				want, setupAllowed = "requires terminal stdin and stdout", true
			case "malformed":
				writeTestFile(t, path, `{"Library":null}`)
				want = "parse config"
			case "invalid paths":
				writeTestFile(t, path, strings.Replace(validConfig, ".local/skills", "../escape", 1))
				want = "must be relative"
			case "directory":
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
				want = "read config"
			case "file ancestor":
				writeTestFile(t, path, "file")
				path += "/config.json"
				want = "read config"
			case "unreadable", "unreadable ancestor":
				writeTestFile(t, path, validConfig)
				protected := path
				if kind == "unreadable ancestor" {
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
				if _, err := os.ReadFile(path); err == nil {
					t.Skip("current user can read mode-000 files")
				}
				want = "read config"
			case "dangling", "dangling ancestor":
				if err := os.Symlink(filepath.Join(root, "absent"), path); err != nil {
					t.Fatal(err)
				}
				if kind == "dangling ancestor" {
					path += "/config.json"
				}
				want = "resolve config link"
			case "valid symlink":
				target := filepath.Join(root, "target.json")
				writeTestFile(t, target, validConfig)
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			case "symlink loop":
				if err := os.Symlink(path, path); err != nil {
					t.Fatal(err)
				}
				want = "read config"
			}
			before, beforeErr := os.ReadFile(path)
			for _, explicit := range []bool{false, true} {
				args := []string{"--config", path, "--project", root}
				diagnostic := want
				if explicit {
					args = append(args, "setup")
					if kind == "valid" || kind == "valid symlink" || setupAllowed {
						diagnostic = "setup is not implemented"
					}
				}
				var stdout, stderr bytes.Buffer
				code := run(args, strings.NewReader(""), &stdout, &stderr)
				if code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), diagnostic) {
					t.Fatalf("explicit=%v: status %d, stdout %q, stderr %q; want %q", explicit, code, &stdout, &stderr, diagnostic)
				}
				if !setupAllowed && diagnostic != "setup is not implemented" && strings.Contains(stderr.String(), "setup") {
					t.Fatalf("existing invalid config dispatched setup: %s", &stderr)
				}
			}
			after, afterErr := os.ReadFile(path)
			if !bytes.Equal(before, after) || (beforeErr == nil) != (afterErr == nil) {
				t.Fatal("configuration changed")
			}
			if kind == "missing parent" {
				if _, err := os.Lstat(filepath.Dir(path)); !os.IsNotExist(err) {
					t.Fatalf("missing parent created: %v", err)
				}
			}
		})
	}
}

func TestCLIHelpWithoutConfig(t *testing.T) {
	isolateConfigHome(t)
	path, err := configLocation(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, "malformed")
	for _, option := range []string{"--help", "-h", "--version"} {
		for _, args := range [][]string{{option}, {"--config", path, "--project", "/missing-project", option, "setup"}} {
			var stdout, stderr bytes.Buffer
			if code := run(args, strings.NewReader(""), &stdout, &stderr); code != 0 || stdout.Len() == 0 || stderr.Len() != 0 {
				t.Fatalf("%v: status %d, stdout %q, stderr %q", args, code, &stdout, &stderr)
			}
		}
	}
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "relative-invalid")
	for _, option := range []string{"--help", "--version"} {
		var stdout, stderr bytes.Buffer
		if code := run([]string{option}, strings.NewReader(""), &stdout, &stderr); code != 0 || stderr.Len() != 0 {
			t.Fatalf("%s: status %d, stderr %q", option, code, &stderr)
		}
	}
}

func TestCLIProjectAndOverrides(t *testing.T) {
	isolateConfigHome(t)
	root := t.TempDir()
	native, err := configLocation(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(native), 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, native, "malformed native config must not merge")
	writeTestFile(t, filepath.Join(root, "custom.json"), validConfig)
	writeTestFile(t, filepath.Join(root, "file"), "not a directory")
	t.Chdir(root)
	launch, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ project, want string }{{".", "requires terminal"}, {"missing", "inspect project"}, {"file", "not a directory"}} {
		var stdout, stderr bytes.Buffer
		code := run([]string{"--config", "custom.json", "--project", tt.project}, strings.NewReader(""), &stdout, &stderr)
		if code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), tt.want) {
			t.Fatalf("project %q: status %d, stderr %q", tt.project, code, &stderr)
		}
		cwd, err := os.Getwd()
		if err != nil || cwd != launch {
			t.Fatalf("cwd changed: %q, %v", cwd, err)
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestCLIOutputFailure(t *testing.T) {
	var stderr bytes.Buffer
	if got := run([]string{"--version"}, strings.NewReader(""), failingWriter{}, &stderr); got != 1 {
		t.Fatalf("status = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "write output") {
		t.Fatalf("missing output diagnostic: %s", &stderr)
	}
}

func TestBrowseKeys(t *testing.T) {
	m := newBrowseModel(config{Library: t.TempDir(), Agents: []agentConfig{{Name: "Example"}}})
	for _, msg := range []tea.Msg{
		tea.KeyPressMsg{Code: 'q'},
		tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl},
	} {
		_, cmd := m.Update(msg)
		if cmd == nil {
			t.Fatalf("quit key %v did not produce a command", msg)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("quit key %v did not quit", msg)
		}
	}
	for _, msg := range []tea.Msg{
		tea.KeyPressMsg{Code: 'a'}, tea.KeyPressMsg{Code: 'X'}, tea.PasteMsg{Content: "q"},
		tea.WindowSizeMsg{Width: 80, Height: 24},
	} {
		if _, cmd := m.Update(msg); cmd != nil {
			t.Fatalf("unexpected command for %v", msg)
		}
	}
	v := m.View()
	if !v.AltScreen || !strings.Contains(v.Content, "Read-only configured folders") || !strings.Contains(v.Content, "quit") {
		t.Fatalf("unexpected browser view: %+v", v)
	}
}
