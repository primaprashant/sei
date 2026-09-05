package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
	for _, scenario := range []string{"cancel-edit", "cancel-preview", "complete-restart", "explicit-new", "explicit-one", "explicit-nine", "explicit-refuse"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, "project"), 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "config", "sei.json")
			explicit := strings.HasPrefix(scenario, "explicit-")
			want := config{Library: "~/library", Agents: append([]agentConfig(nil), setupPresets[:3]...)}
			if explicit && scenario != "explicit-new" {
				want.Agents = nil
				count := 1
				if scenario == "explicit-nine" {
					count = 9
				}
				for i := 1; i <= count; i++ {
					want.Agents = append(want.Agents, agentConfig{fmt.Sprintf("Existing%d", i), fmt.Sprintf("~/global%d", i), fmt.Sprintf(".local%d/skills", i)})
				}
				if err := saveConfig(want, filepath.Join(root, "project"), path, false); err != nil {
					t.Fatal(err)
				}
			}
			var before []byte
			if scenario == "explicit-refuse" {
				var err error
				before, err = os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
			}
			attempts := 2
			if explicit {
				attempts = 1
			}
			for attempt := 0; attempt < attempts; attempt++ {
				// A second launch proves cancellation still starts setup, while a
				// completed first run loads the persisted config directly into browse.
				restart := attempt == 1 && scenario == "complete-restart"
				screen := runSetupPTY(t, binary, root, path, scenario, restart)
				if restart && strings.Contains(screen, "sei setup") {
					t.Fatal("restart unexpectedly entered setup")
				}
				if scenario != "complete-restart" && !explicit {
					setupAbsent(t, filepath.Dir(path))
				} else {
					cfg, missing, err := loadConfig(path)
					if explicit && scenario != "explicit-new" && scenario != "explicit-refuse" {
						want.Library += "-edited"
					}
					if err != nil || missing || !reflect.DeepEqual(cfg, want) {
						t.Fatalf("persisted first-run config: %+v, %v, %v", cfg, missing, err)
					}
				}
				if scenario == "explicit-refuse" {
					after, err := os.ReadFile(path)
					if err != nil || !bytes.Equal(before, after) {
						t.Fatal("PTY refusal changed config")
					}
				}
				setupAbsent(t, filepath.Join(root, "library-edited"))
				for i := 1; i <= 9; i++ {
					setupAbsent(t, filepath.Join(root, fmt.Sprintf("global%d", i)), filepath.Join(root, "project", fmt.Sprintf(".local%d", i)))
				}
				for _, dir := range []string{"library", ".claude", ".agents", ".opencode", ".config/opencode", "Library", "project/.claude", "project/.agents", "project/.opencode"} {
					setupAbsent(t, filepath.Join(root, dir))
				}
			}
		})
	}
}

func TestConfigSaveFailurePTY(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "sei")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	build.WaitDelay = 5 * time.Second
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	root := t.TempDir()
	paths := []string{"library", ".claude/skills", ".agents/skills", ".config/opencode/skills", "project/.claude/skills", "project/.agents/skills", "project/.opencode/skills"}
	for _, path := range paths {
		dir := filepath.Join(root, path, "existing")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("original skill\n\x00\xff"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Include directory entries and file bytes so additions and removals also fail.
	snapshot := func() map[string]string {
		t.Helper()
		entries := make(map[string]string)
		for _, path := range paths {
			if err := filepath.WalkDir(filepath.Join(root, path), func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				entries[path] = "directory"
				if !d.IsDir() {
					data, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					entries[path] = string(data)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		}
		return entries
	}
	before := snapshot()
	path := filepath.Join(root, "config", "sei.json")
	runSetupPTY(t, binary, root, path, "save-failure", false)
	if after := snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatalf("save failure changed skills: before=%q after=%q", before, after)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "concurrent config\n\x1b[31m\r\x00\xff" {
		t.Fatalf("competing config changed: %q, %v", data, err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 || entries[0].Name() != "sei.json" {
		t.Fatalf("unexpected config directory contents: %v, %v", entries, err)
	}
}

func runSetupPTY(t *testing.T, binary, root, path, scenario string, restart bool) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	args := []string{"--config", path}
	explicit := strings.HasPrefix(scenario, "explicit-")
	if explicit {
		args = append(args, "setup")
	}
	cmd := exec.CommandContext(ctx, binary, args...)
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
	awaitFrom := 0
	await := func(text string) {
		t.Helper()
		for !strings.Contains(screen.String()[awaitFrom:], text) {
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
		await("Configured folders")
		send("q")
	} else {
		await("Enter preview")
		raw, err := term.GetState(slave.Fd())
		if err != nil || reflect.DeepEqual(before, raw) {
			t.Fatalf("setup did not enter raw mode: %v", err)
		}
		if explicit && scenario != "explicit-new" {
			await("Library: ~/library")
			await("Name: Existing1")
			send("-edited")
			await("-edited")
		} else {
			send("~/library")
			await("~/library")
		}
		if scenario == "cancel-edit" {
			send("\x1b")
		} else {
			send("\r")
			await("Enter save")
			slot := 3
			if explicit && scenario != "explicit-new" {
				slot = 1
			}
			if scenario == "explicit-nine" {
				slot = 9
			}
			await(fmt.Sprintf("focus %d / g%d", slot, slot))
			if scenario != "save-failure" {
				setupAbsent(t, filepath.Join(root, "library"))
			}
			if !explicit || scenario == "explicit-new" {
				setupAbsent(t, filepath.Dir(path))
			}
			if scenario == "cancel-preview" {
				send("\x1b")
			} else {
				if scenario == "save-failure" {
					// The preview observed absence; publish a competing file before save.
					if err := os.Mkdir(filepath.Dir(path), 0o700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte("concurrent config\n\x1b[31m\r\x00\xff"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				send("\r")
				if explicit {
					if scenario != "explicit-new" {
						await("Replace existing configuration?")
						if scenario == "explicit-refuse" {
							awaitFrom = screen.Len()
							send("n")
							await("Enter save")
							awaitFrom = 0
							send("\x1b")
						} else {
							send("y")
						}
					}
				} else if scenario != "save-failure" {
					await("Configured folders")
					send("q")
				}
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
	if scenario == "save-failure" {
		var exitErr *exec.ExitError
		if ctx.Err() != nil || !errors.As(waitErr, &exitErr) || exitErr.ExitCode() != 1 {
			t.Fatalf("save failure exit: %v, stderr=%q, screen=%s", waitErr, stderr.String(), screen.String())
		}
		if stderr.String() != "sei: setup: config already exists: file already exists\n" || strings.ContainsAny(stderr.String(), "\x1b\r\x00") {
			t.Fatalf("missing or unsafe final save diagnostic: %q", stderr.String())
		}
	} else if ctx.Err() != nil || waitErr != nil || stderr.Len() != 0 {
		t.Fatalf("exit: %v, stderr=%q, screen=%s", waitErr, stderr.String(), screen.String())
	}
	for _, pair := range [][2]string{{"\x1b[?1049h", "\x1b[?1049l"}, {"\x1b[?25l", "\x1b[?25h"}} {
		if strings.LastIndex(screen.String(), pair[0]) < 0 || strings.LastIndex(screen.String(), pair[1]) <= strings.LastIndex(screen.String(), pair[0]) {
			t.Errorf("missing terminal restoration %q", pair)
		}
	}
	if scenario != "complete-restart" && strings.Contains(screen.String(), "Configured folders") {
		t.Fatal("cancel entered browse")
	}
	return screen.String()
}
