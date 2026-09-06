package app

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

func TestPTYAddRestart(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "sei")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	build.Dir = repoPath(t, ".")
	build.WaitDelay = 5 * time.Second
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	for _, scope := range []string{"local", "global"} {
		t.Run(scope, func(t *testing.T) {
			root := t.TempDir()
			cfg := config{Library: filepath.Join(root, "library"), Agents: []agentConfig{{Name: "Agent", Global: filepath.Join(root, "global"), Local: "local"}}}
			browseMkdir(t, filepath.Join(cfg.Library, "add-me", "nested", "empty"))
			writeTestFile(t, filepath.Join(cfg.Library, "add-me", "nested", "SKILL.md"), "original\n\x00\xff")
			for _, dir := range []string{"local", "global"} {
				browseMkdir(t, filepath.Join(root, dir, "survivor"))
				writeTestFile(t, filepath.Join(root, dir, "survivor", "keep"), dir)
			}
			path := filepath.Join(root, "config.json")
			if err := saveConfig(cfg, root, path, false); err != nil {
				t.Fatal(err)
			}
			library := removeSnapshot(t, cfg.Library)
			configBefore := removeSnapshot(t, path)
			other := filepath.Join(root, "global")
			if scope == "global" {
				other = filepath.Join(root, "local")
			}
			otherBefore := removeSnapshot(t, other)
			survivor := filepath.Join(root, scope, "survivor")
			survivorBefore := removeSnapshot(t, survivor)
			target := filepath.Join(root, scope, "add-me")
			setupAbsent(t, target)
			for _, restart := range []bool{false, true} {
				runAddPTY(t, binary, root, path, scope, restart)
				data, err := os.ReadFile(filepath.Join(target, "nested", "SKILL.md"))
				if err != nil || string(data) != "original\n\x00\xff" {
					t.Fatalf("copied bytes: %q, %v", data, err)
				}
				if info, err := os.Stat(filepath.Join(target, "nested", "empty")); err != nil || !info.IsDir() {
					t.Fatalf("copied empty directory: %v", err)
				}
				assertRemoveSnapshot(t, cfg.Library, library)
				assertRemoveSnapshot(t, path, configBefore)
				assertRemoveSnapshot(t, other, otherBefore)
				assertRemoveSnapshot(t, survivor, survivorBefore)
			}
		})
	}
}

func runAddPTY(t *testing.T, binary, root, path, scope string, restart bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "--config", path)
	cmd.Dir = root
	cmd.Env = []string{"HOME=" + root, "XDG_CONFIG_HOME=" + filepath.Join(root, ".config"), "TERM=xterm-256color", "NO_COLOR=1", "PATH=" + t.TempDir()}
	cmd.WaitDelay = 5 * time.Second
	master, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 40, Cols: 240})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = master.Close() }()
	wait := make(chan struct{})
	var waitErr error
	go func() { waitErr = cmd.Wait(); close(wait) }()
	defer func() { cancel(); <-wait }()
	fd := int(master.Fd())
	if err := unix.SetNonblock(fd, true); err != nil {
		t.Fatal(err)
	}
	chunks := make(chan string)
	readerCtx, stopReader := context.WithCancel(ctx)
	readerDone := make(chan struct{})
	var readerErr error
	go func() {
		defer close(readerDone)
		defer close(chunks)
		readerErr = readLifecyclePTY(readerCtx, fd, chunks)
	}()
	defer func() {
		stopReader()
		<-readerDone
		if readerErr != nil && !errors.Is(readerErr, context.Canceled) {
			t.Errorf("PTY reader: %v", readerErr)
		}
	}()
	var screen strings.Builder
	from := 0
	await := func(text string) {
		t.Helper()
		for !strings.Contains(ansi.Strip(screen.String()[from:]), text) {
			select {
			case chunk, ok := <-chunks:
				if !ok {
					t.Fatalf("PTY closed waiting for %q: %s", text, screen.String())
				}
				screen.WriteString(chunk)
			case <-ctx.Done():
				t.Fatalf("timeout waiting for %q: %s", text, screen.String())
			}
		}
	}
	send := func(text string) {
		t.Helper()
		if _, err := io.WriteString(master, text); err != nil {
			t.Fatal(err)
		}
	}
	await("Configured folders")
	await("Ready")
	await("> add-me")
	if !restart {
		from = screen.Len()
		if scope == "global" {
			send("A")
		} else {
			send("a")
		}
		await("complete")
		from = screen.Len()
		send("?")
		await("Root relations checked; every mutation revalidates")
		await("Selected name: add-me")
		await("Focused: Library")
		from = screen.Len()
		send("?")
		await("Configured folders")
	}
	if scope == "global" {
		send("g1")
	} else {
		send("1")
	}
	from = screen.Len()
	send("?")
	await("Root relations checked; every mutation revalidates")
	if restart {
		await("Selected name: add-me")
	} else {
		await("Selected name: survivor")
	}
	send("q")
	for chunks != nil {
		select {
		case chunk, ok := <-chunks:
			if !ok {
				chunks = nil
			} else {
				screen.WriteString(chunk)
			}
		case <-ctx.Done():
			t.Fatalf("timeout draining PTY: %s", screen.String())
		}
	}
	select {
	case <-wait:
	case <-ctx.Done():
		t.Fatal("process timeout")
	}
	if ctx.Err() != nil || waitErr != nil {
		t.Fatalf("exit: %v, context: %v\n%s", waitErr, ctx.Err(), screen.String())
	}
	for _, pair := range [][2]string{{"\x1b[?1049h", "\x1b[?1049l"}, {"\x1b[?25l", "\x1b[?25h"}} {
		if strings.LastIndex(screen.String(), pair[0]) < 0 || strings.LastIndex(screen.String(), pair[1]) <= strings.LastIndex(screen.String(), pair[0]) {
			t.Errorf("missing terminal restoration %q", pair)
		}
	}
}
