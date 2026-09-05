package main

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

	"github.com/charmbracelet/x/term"
	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

func TestPTYFirstRunSetup(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "sei")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	build.WaitDelay = 5 * time.Second
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	for _, scenario := range []string{"cancel-edit", "cancel-preview", "complete-restart"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, "project"), 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "config", "sei.json")
			for attempt := 0; attempt < 2; attempt++ {
				// A second launch proves cancellation still starts setup, while a
				// completed first run loads the persisted config directly into browse.
				restart := attempt == 1 && scenario == "complete-restart"
				screen := runSetupPTY(t, binary, root, path, scenario, restart)
				if restart && strings.Contains(screen, "sei setup") {
					t.Fatal("restart unexpectedly entered setup")
				}
				if scenario != "complete-restart" {
					setupAbsent(t, filepath.Dir(path))
				} else {
					cfg, missing, err := loadConfig(path)
					if err != nil || missing || cfg.Library != "~/library" || !reflect.DeepEqual(cfg.Agents, setupPresets) {
						t.Fatalf("persisted first-run config: %+v, %v, %v", cfg, missing, err)
					}
				}
				for _, dir := range []string{"library", ".claude", ".agents", ".opencode", ".config/opencode", "Library", "project/.claude", "project/.agents", "project/.opencode"} {
					setupAbsent(t, filepath.Join(root, dir))
				}
			}
		})
	}
}

func runSetupPTY(t *testing.T, binary, root, path, scenario string, restart bool) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "--config", path)
	cmd.Dir = filepath.Join(root, "project")
	cmd.Env = []string{"HOME=" + root, "XDG_CONFIG_HOME=" + filepath.Join(root, ".config"), "TERM=xterm-256color", "NO_COLOR=1", "PATH=" + os.Getenv("PATH")}
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
	if err := pty.Setsize(master, &pty.Winsize{Rows: 40, Cols: 200}); err != nil {
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
	await := func(text string) {
		t.Helper()
		for !strings.Contains(screen.String(), text) {
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
	if restart {
		await("Read-only configured folders")
		send("q")
	} else {
		await("Enter preview")
		raw, err := term.GetState(slave.Fd())
		if err != nil || reflect.DeepEqual(before, raw) {
			t.Fatalf("setup did not enter raw mode: %v", err)
		}
		send("~/library")
		await("~/library")
		if scenario == "cancel-edit" {
			send("\x1b")
		} else {
			send("\r")
			await("Enter save")
			await("focus 3 / g3")
			setupAbsent(t, filepath.Dir(path), filepath.Join(root, "library"))
			if scenario == "cancel-preview" {
				send("\x1b")
			} else {
				send("\r")
				await("Read-only configured folders")
				send("q")
			}
		}
	}
waiting:
	for {
		select {
		case chunk, ok := <-chunks:
			if !ok {
				chunks = nil
			} else {
				screen.WriteString(chunk)
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
			}
		case <-ctx.Done():
			t.Fatal("timeout draining PTY")
		}
	}
	if ctx.Err() != nil || waitErr != nil || stderr.Len() != 0 {
		t.Fatalf("exit: %v, stderr=%q, screen=%s", waitErr, stderr.String(), screen.String())
	}
	for _, pair := range [][2]string{{"\x1b[?1049h", "\x1b[?1049l"}, {"\x1b[?25l", "\x1b[?25h"}} {
		if strings.LastIndex(screen.String(), pair[0]) < 0 || strings.LastIndex(screen.String(), pair[1]) <= strings.LastIndex(screen.String(), pair[0]) {
			t.Errorf("missing terminal restoration %q", pair)
		}
	}
	if scenario != "complete-restart" && strings.Contains(screen.String(), "Read-only configured folders") {
		t.Fatal("cancel entered browse")
	}
	return screen.String()
}
