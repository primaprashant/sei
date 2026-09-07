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
	"testing"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

func TestPTYStats(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "sei")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = repoPath(t, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	cfg := statsFixture(t)
	raw := cfg
	raw.Agents = append([]agentConfig(nil), cfg.Agents...)
	raw.Agents[0].Local = "local"
	// The browser needs a real config; stats should still work after its removal.
	writeTestFile(t, cfg.ConfigPath, fmt.Sprintf(`{"library":%q,"agents":[{"name":"Example","global":%q,"local":%q}]}`, raw.Library, raw.Agents[0].Global, raw.Agents[0].Local))
	browseMkdir(t, filepath.Join(cfg.Library, "example"))
	writeTestFile(t, filepath.Join(cfg.Library, "example", "SKILL.md"), "inert fixture")
	before := configSaveSnapshot(t, cfg.Library, cfg.ConfigPath)
	for i, action := range []string{"a", "a", "1x"} {
		t.Run(fmt.Sprintf("action-%d", i), func(t *testing.T) {
			want := "Copied example"
			if action == "1x" {
				want = "Removed example"
			}
			testStatsPTY(t, binary, cfg, nil, "Ready", action, want)
		})
	}
	before()
	history, err := loadStats(statsPath(cfg.ConfigPath))
	if err != nil {
		t.Fatal(err)
	}
	if summary := history.summarize(time.Now()); summary.copies != 2 || summary.removals != 1 {
		t.Fatalf("CLI counts: %+v", summary)
	}
	if err := os.Remove(cfg.ConfigPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(cfg.Library, cfg.Library+"-moved"); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"recorded", "empty"} {
		t.Run(scenario, func(t *testing.T) {
			chosen := cfg
			want := "2 copies"
			if scenario == "empty" {
				chosen.ConfigPath += ".other"
				want = "No activity yet."
			}
			before := removeSnapshot(t, cfg.Home)
			testStatsPTY(t, binary, chosen, []string{"--project", "/missing-stats-project", "stats"}, want, "", "Last 30 days")
			assertRemoveSnapshot(t, cfg.Home, before)
		})
	}
}

func testStatsPTY(t *testing.T, binary string, cfg config, args []string, initial, keys, expected string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, append([]string{"--config", cfg.ConfigPath}, args...)...)
	cmd.Dir = cfg.Project
	cmd.Env = []string{"HOME=" + cfg.Home, "XDG_CONFIG_HOME=" + filepath.Join(cfg.Home, ".config"), "TERM=xterm-256color", "NO_COLOR=1"}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	master, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 80, Rows: 24})
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
	screen := newPTYScreen(t, 80, 24)
	await := func(want string) {
		t.Helper()
		for !ptyScreenMatches(screen, want) {
			select {
			case chunk, ok := <-chunks:
				if !ok {
					t.Fatalf("PTY closed waiting for %q: %s", want, screen.String())
				}
				if _, err := screen.WriteString(chunk); err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatalf("timeout waiting for %q: %s", want, screen.String())
			}
		}
	}
	await(initial)
	if _, err := io.WriteString(master, keys); err != nil {
		t.Fatal(err)
	}
	await(expected)
	if dir := os.Getenv("SEI_TEST_CAPTURES"); dir != "" && len(args) > 0 {
		name := strings.ReplaceAll(t.Name(), "/", "-")
		if err := os.WriteFile(filepath.Join(dir, name+".ansi"), []byte(screen.Render()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := io.WriteString(master, "q"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("quit timed out")
	}
	if waitErr != nil || stderr.Len() != 0 {
		t.Fatalf("exit: %v, %s", waitErr, &stderr)
	}
}
