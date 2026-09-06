package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

func TestPTYThemeAndNavigation(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "sei")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = repoPath(t, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	for _, profile := range []string{"dark", "light", "no-color"} {
		t.Run(profile, func(t *testing.T) {
			root := t.TempDir()
			project, library := filepath.Join(root, "project"), filepath.Join(root, "skill-library")
			browseMkdir(t, project)
			browseMkdir(t, filepath.Join(library, "example"))
			writeTestFile(t, filepath.Join(library, "example", "SKILL.md"), "Inert UI fixture")
			configPath := filepath.Join(root, "config.json")
			writeTestFile(t, configPath, fmt.Sprintf(`{"library":%q,"agents":[{"name":"Demo","global":%q,"local":".demo/skills"}]}`, library, filepath.Join(root, "global")))
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, "--config", configPath)
			cmd.Dir = project
			cmd.Env = []string{"HOME=" + root, "XDG_CONFIG_HOME=" + filepath.Join(root, "config"), "TERM=xterm-256color", "COLORTERM=truecolor", "COLORFGBG=15;0"}
			if profile == "no-color" {
				cmd.Env = append(cmd.Env, "NO_COLOR=nonempty")
			}
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			master, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 100, Rows: 30})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = master.Close() }()
			done := make(chan struct{})
			var waitErr error
			go func() { waitErr = cmd.Wait(); close(done) }()
			defer func() { cancel(); <-done }()
			if err := unix.SetNonblock(int(master.Fd()), true); err != nil {
				t.Fatal(err)
			}
			chunks := make(chan string)
			readerCtx, stop := context.WithCancel(ctx)
			readerDone := make(chan struct{})
			go func() {
				defer close(readerDone)
				defer close(chunks)
				_ = readLifecyclePTY(readerCtx, int(master.Fd()), chunks)
			}()
			defer func() { stop(); <-readerDone }()
			terminal := newPTYScreen(t, 100, 30)
			read := func() {
				t.Helper()
				select {
				case chunk, ok := <-chunks:
					if !ok {
						t.Fatalf("PTY closed: %s", terminal.String())
					}
					if _, err := terminal.WriteString(chunk); err != nil {
						t.Fatal(err)
					}
				case <-ctx.Done():
					t.Fatalf("timeout: %s", terminal.String())
				}
			}
			await := func(wants ...string) {
				t.Helper()
				for !performanceScreenMatches(terminal, wants...) {
					read()
				}
			}
			send := func(text string) {
				t.Helper()
				if _, err := io.WriteString(master, text); err != nil {
					t.Fatal(err)
				}
			}
			await("Ready", "> example", "Tab panel")
			bg := "0000/0000/0000"
			expected := lipgloss.Color("#B4A4F4")
			if profile == "light" {
				bg = "ffff/ffff/ffff"
				expected = lipgloss.Color("#6545AA")
			}
			// Exercise a real OSC reply, overriding the dark COLORFGBG fallback.
			send("\x1b]11;rgb:" + bg + "\x1b\\")
			correctColor := func() bool {
				for y, line := range strings.Split(terminal.String(), "\n") {
					if pos := strings.Index(line, "> example"); pos >= 0 {
						cell := terminal.CellAt(ansi.StringWidth(line[:pos]), y)
						if profile == "no-color" {
							return cell.Style.Bg == nil && cell.Style.Fg == nil
						}
						if cell.Style.Bg == nil {
							return false
						}
						r, g, b, _ := cell.Style.Bg.RGBA()
						er, eg, eb, _ := expected.RGBA()
						return r == er && g == eg && b == eb
					}
				}
				return false
			}
			for !correctColor() {
				read()
			}
			if profile == "no-color" {
				for y := 0; y < terminal.Height(); y++ {
					for x := 0; x < terminal.Width(); x++ {
						if cell := terminal.CellAt(x, y); cell != nil && (cell.Style.Fg != nil || cell.Style.Bg != nil) {
							t.Fatal("NO_COLOR left a colored cell")
						}
					}
				}
			}
			if dir := os.Getenv("SEI_TEST_CAPTURES"); dir != "" {
				if err := os.WriteFile(filepath.Join(dir, "ui-"+profile+".ansi"), []byte(terminal.Render()), 0600); err != nil {
					t.Fatal(err)
				}
			}
			send("\t")
			await("Demo / Project ·")
			send("\t")
			await("Demo / Global ·")
			send("\x1b[Z")
			await("Demo / Project ·")
			send("0?")
			await("sei | Help", "Focused: Library")
			send("\x1b")
			await("* Library", "PROJECT")
			for _, size := range [][2]int{{80, 24}, {79, 23}} {
				terminal.Resize(size[0], size[1])
				if err := pty.Setsize(master, &pty.Winsize{Cols: uint16(size[0]), Rows: uint16(size[1])}); err != nil {
					t.Fatal(err)
				}
				if err := cmd.Process.Signal(syscall.SIGWINCH); err != nil {
					t.Fatal(err)
				}
				if size[0] == 80 {
					await("Tab panel", "> example")
				} else {
					await("Resize to at least 80x24")
				}
			}
			send("q")
			select {
			case <-done:
			case <-ctx.Done():
				t.Fatal("quit timed out")
			}
			if waitErr != nil || stderr.Len() != 0 {
				t.Fatalf("exit: %v, %s", waitErr, stderr.String())
			}
		})
	}
}
