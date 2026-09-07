package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

func TestPTYRemoveRestart(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "sei")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	build.Dir = repoPath(t, ".")
	build.WaitDelay = 5 * time.Second
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	for _, scope := range []string{"global", "local"} {
		t.Run(scope, func(t *testing.T) {
			root := t.TempDir()
			cfg := config{Library: filepath.Join(root, "skill-library"), Agents: []agentConfig{{Name: "Agent", Global: filepath.Join(root, "global"), Local: filepath.Join(root, "local")}}}
			for _, dir := range []string{cfg.Library, cfg.Agents[0].Global, cfg.Agents[0].Local} {
				for _, name := range []string{"remove-me", "survivor"} {
					browseMkdir(t, filepath.Join(dir, name, "nested"))
					writeTestFile(t, filepath.Join(dir, name, "nested", "SKILL.md"), "original\n\x00\xff")
				}
			}
			path := filepath.Join(root, "config.json")
			persisted := cfg
			persisted.Agents = []agentConfig{{Name: "Agent", Global: cfg.Agents[0].Global, Local: "local"}}
			if err := saveConfig(persisted, root, path, false); err != nil {
				t.Fatal(err)
			}
			library := removeSnapshot(t, cfg.Library)
			configBefore := removeSnapshot(t, path)
			other := cfg.Agents[0].Local
			if scope == "local" {
				other = cfg.Agents[0].Global
			}
			otherBefore := removeSnapshot(t, other)
			survivor := filepath.Join(root, scope, "survivor")
			survivorBefore := removeSnapshot(t, survivor)
			for _, restart := range []bool{false, true} {
				runMutationPTY(t, binary, root, path, scope, restart)
				setupAbsent(t, filepath.Join(root, scope, "remove-me"))
				assertRemoveSnapshot(t, cfg.Library, library)
				assertRemoveSnapshot(t, path, configBefore)
				assertRemoveSnapshot(t, other, otherBefore)
				assertRemoveSnapshot(t, survivor, survivorBefore)
			}
		})
	}
}

func runMutationPTY(t *testing.T, binary, root, path, scope string, restart bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "--config", path)
	cmd.Dir = root
	cmd.Env = []string{"HOME=" + root, "XDG_CONFIG_HOME=" + filepath.Join(root, ".config"), "TERM=xterm-256color", "NO_COLOR=1", "PATH=" + t.TempDir()}
	cmd.WaitDelay = 5 * time.Second
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, f := range []*os.File{master, slave} {
			if err := f.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
				t.Error(err)
			}
		}
	}()
	if err := pty.Setsize(master, &pty.Winsize{Rows: 40, Cols: 240}); err != nil {
		t.Fatal(err)
	}
	fd := int(master.Fd())
	if err := unix.SetNonblock(fd, true); err != nil {
		t.Fatal(err)
	}
	before, err := term.GetState(slave.Fd())
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, &stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	wait := make(chan struct{})
	var waitErr error
	go func() { waitErr = cmd.Wait(); close(wait) }()
	defer func() { cancel(); <-wait }()
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
	terminal := newPTYScreen(t, 240, 40)
	await := func(text string) {
		t.Helper()
		for !ptyScreenMatches(terminal, text) {
			select {
			case chunk, ok := <-chunks:
				if !ok {
					t.Fatalf("PTY closed waiting for %q: %s", text, screen.String())
				}
				screen.WriteString(chunk)
				if _, err := terminal.WriteString(chunk); err != nil {
					t.Fatal(err)
				}
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
	await("PROJECT")
	await("Ready")
	await("> remove-me")
	raw, err := term.GetState(slave.Fd())
	if err != nil || reflect.DeepEqual(before, raw) {
		t.Fatalf("terminal not raw: %v", err)
	}
	// Help reports the focused target without depending on terminal row diffs.
	if scope == "global" {
		send("g1")
	} else {
		send("1")
	}
	send("?")
	await("Help ")
	await("Root relations checked; every mutation revalidates.")
	if restart {
		await("Selected name: survivor")
	} else {
		await("Selected name: remove-me")
		send("?")
		await("PROJECT")
		before := removeSnapshot(t, filepath.Join(root, scope))
		// Opening help after X confirms input was processed without relying on a delay.
		send("X?")
		await("Selected name: remove-me")
		await("Root relations checked; every mutation revalidates.")
		assertRemoveSnapshot(t, filepath.Join(root, scope), before)
		send("?")
		await("PROJECT")
		send("x")
		await("Removed remove-me")
		send("?")
		await("Selected name: survivor")
	}
	send("q")
waiting:
	for {
		select {
		case chunk, ok := <-chunks:
			if !ok {
				chunks = nil
			} else {
				screen.WriteString(chunk)
				if _, err := terminal.WriteString(chunk); err != nil {
					t.Fatal(err)
				}
			}
		case <-wait:
			break waiting
		case <-ctx.Done():
			t.Fatalf("process timeout: %s", screen.String())
		}
	}
	after, err := term.GetState(slave.Fd())
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Errorf("terminal modes not restored: %v", err)
	}
	if err := slave.Close(); err != nil {
		t.Fatal(err)
	}
	for chunks != nil {
		select {
		case chunk, ok := <-chunks:
			if !ok {
				chunks = nil
			} else {
				screen.WriteString(chunk)
				if _, err := terminal.WriteString(chunk); err != nil {
					t.Fatal(err)
				}
			}
		case <-ctx.Done():
			t.Fatal("timeout draining PTY")
		}
	}
	if ctx.Err() != nil || waitErr != nil || stderr.Len() != 0 {
		t.Fatalf("exit: %v, stderr=%q", waitErr, stderr.String())
	}
	for _, pair := range [][2]string{{"\x1b[?1049h", "\x1b[?1049l"}, {"\x1b[?25l", "\x1b[?25h"}} {
		if strings.LastIndex(screen.String(), pair[0]) < 0 || strings.LastIndex(screen.String(), pair[1]) <= strings.LastIndex(screen.String(), pair[0]) {
			t.Errorf("missing terminal restoration %q", pair)
		}
	}
	leave := strings.LastIndex(screen.String(), "\x1b[?1049l")
	if leave >= 0 && strings.TrimSpace(ansi.Strip(screen.String()[leave+len("\x1b[?1049l"):])) != "" {
		t.Error("rendered after alternate-screen exit")
	}
}
