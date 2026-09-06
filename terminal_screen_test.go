package main

import (
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

func performanceScreenMatches(screen *vt.Emulator, wants ...string) bool {
	text := strings.ReplaceAll(screen.String(), "\n", "")
	for _, want := range wants {
		// String trims blank cells at row ends, including a real space that
		// lands at a hard-wrap boundary. Apply that same loss to expectations.
		lines := strings.Split(ansi.Hardwrap(want, screen.Width(), true), "\n")
		for i := range lines {
			lines[i] = strings.TrimRight(lines[i], " ")
		}
		if !strings.Contains(text, strings.Join(lines, "")) {
			return false
		}
	}
	return true
}

func newPTYScreen(t *testing.T, width, height int) *vt.Emulator {
	t.Helper()
	screen := vt.NewEmulator(width, height)
	done := make(chan error, 1)
	// Consume query responses without changing the PTY's capability profile.
	go func() { _, err := io.Copy(io.Discard, screen); done <- err }()
	t.Cleanup(func() {
		// Unblock/join Read before Close changes the emulator's closed flag.
		if err := screen.InputPipe().(io.Closer).Close(); err != nil {
			t.Error(err)
		}
		if err := <-done; err != nil {
			t.Error(err)
		}
		if err := screen.Close(); err != nil {
			t.Error(err)
		}
	})
	return screen
}

func TestPerformanceScreen(t *testing.T) {
	screen := newPTYScreen(t, 80, 24)
	write := func(text string) {
		t.Helper()
		// Exercise parser state retained across split CSI/UTF-8 chunks too.
		for i := range len(text) {
			if _, err := screen.WriteString(text[i : i+1]); err != nil {
				t.Fatal(err)
			}
		}
	}
	write("\x1b[?2026$p\x1b[?1049h\x1b[H\x1b[2JResult: complete\r\nRoot checked.\r\nwide \u754c")
	write("\x1b[1;1HResult: complet") // Last e is retained, not emitted again.
	if !strings.Contains(screen.String(), "Result: complete") || !strings.Contains(screen.String(), "wide \u754c") {
		t.Fatalf("lost retained/Unicode cells: %q", screen.String())
	}
	write("\x1b[2;1H\x1b[L")
	if !strings.Contains(screen.String(), "Root checked.") {
		t.Fatal("insert-line lost existing cells")
	}
	write("\x1b[H\x1b[2JResult: working")
	if strings.Contains(screen.String(), "complete") || representativeHelpMatches(screen.String(), "Result: complete") {
		t.Fatal("stale result survived clear")
	}
	write("\x1b[HResult: complete\x1b[K\r\nHelp 1/1")
	if !representativeHelpMatches(screen.String(), "Result: completeHelp 1/1") {
		t.Fatal("exact current-screen completion missing")
	}
	result := "Result: " + strings.Repeat("x", 70) + ": complete\nHelp 1/2"
	write("\x1b[H\x1b[2J" + strings.ReplaceAll(ansi.Hardwrap(result, 80, true), "\n", "\r\n"))
	if !performanceScreenMatches(screen, result) || performanceScreenMatches(screen, strings.Replace(result, "complete", "working", 1)) {
		t.Fatal("space at wrap boundary confused current-screen result")
	}
	write("\x1b[?1049l")
	if strings.Contains(screen.String(), "Result:") {
		t.Fatal("alternate-screen result leaked into restored screen")
	}
}
