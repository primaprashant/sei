package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestCLI(t *testing.T) {
	// Neither help nor version should consult configuration, even when its
	// platform-native location contains an invalid path.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "relative-invalid-config")
	for _, tt := range []struct {
		name string
		args []string
		code int
		out  string
		err  string
	}{
		{"help", []string{"--help"}, 0, "Usage: sei", ""},
		{"version", []string{"--version"}, 0, "sei dev\n", ""},
		{"unknown flag", []string{"--unknown"}, 2, "", "flag provided but not defined"},
		{"unknown command", []string{"unknown"}, 2, "", "unexpected argument"},
		{"unimplemented setup", []string{"setup"}, 2, "", "unexpected argument"},
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

func TestShell(t *testing.T) {
	m := shellModel{}
	if m.Init() != nil {
		t.Fatal("shell must not start filesystem commands")
	}
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
	if !v.AltScreen || !strings.Contains(v.Content, "No folders are read or changed") || !strings.Contains(v.Content, "quit") {
		t.Fatalf("unexpected shell view: %+v", v)
	}
}
