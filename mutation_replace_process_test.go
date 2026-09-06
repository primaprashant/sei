package main

import (
	"context"
	"errors"
	"fmt"
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

func TestPTYReplaceWorkflow(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "sei")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	build.WaitDelay = 5 * time.Second
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	for _, scope := range []string{"local", "global"} {
		t.Run(scope, func(t *testing.T) {
			// Exercise launch-directory aliases on Linux too, as with macOS /var.
			physicalRoot, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			root := filepath.Join(t.TempDir(), "project-link")
			if err := os.Symlink(physicalRoot, root); err != nil {
				t.Fatal(err)
			}
			cfg := config{Library: filepath.Join(root, "library")}
			source := filepath.Join(cfg.Library, "replace-me")
			browseMkdir(t, filepath.Join(source, "nested", "empty"))
			const original = "original\n\x00\xff\r\n"
			writeTestFile(t, filepath.Join(source, "nested", "SKILL.md"), original)
			unchanged := make(map[string]map[string]removeSnapshotEntry)
			var targets []string
			for i := range 3 {
				agent := agentConfig{Name: fmt.Sprintf("Agent%d", i+1), Global: filepath.Join(root, fmt.Sprintf("global%d", i+1)), Local: fmt.Sprintf("local%d", i+1)}
				cfg.Agents = append(cfg.Agents, agent)
				for _, kind := range []string{"local", "global"} {
					dir := filepath.Join(root, fmt.Sprintf("%s%d", kind, i+1))
					survivor := filepath.Join(dir, "survivor")
					browseMkdir(t, survivor)
					writeTestFile(t, filepath.Join(survivor, "keep"), dir)
					if kind == scope {
						targets = append(targets, filepath.Join(dir, "replace-me"))
						unchanged[survivor] = removeSnapshot(t, survivor)
					} else {
						unchanged[dir] = removeSnapshot(t, dir)
					}
				}
			}
			// Keep the replacement as the surviving copy, proving persistence of
			// original bytes rather than the initially edited destination.
			browseMkdir(t, filepath.Join(targets[2], "nested"))
			writeTestFile(t, filepath.Join(targets[2], "nested", "SKILL.md"), "edited destination")
			writeTestFile(t, filepath.Join(targets[2], "destination-only"), "obsolete")
			path := filepath.Join(root, "config.json")
			if err := saveConfig(cfg, root, path, false); err != nil {
				t.Fatal(err)
			}
			// The child uses os.Getwd for local destinations; configured global
			// and library spellings are deliberately not canonicalized.
			runtimeConfig, err := resolveConfigPaths(cfg, physicalRoot)
			if err != nil {
				t.Fatal(err)
			}
			resultPath := func(i int) string {
				if scope == "global" {
					return runtimeConfig.Agents[i].Global
				}
				return runtimeConfig.Agents[i].Local
			}
			unchanged[cfg.Library] = removeSnapshot(t, cfg.Library)
			unchanged[path] = removeSnapshot(t, path)
			assertUnchanged := func(t *testing.T) {
				t.Helper()
				for path, before := range unchanged {
					assertRemoveSnapshot(t, path, before)
				}
			}
			assertCopy := func(t *testing.T, target string) {
				t.Helper()
				data, err := os.ReadFile(filepath.Join(target, "nested", "SKILL.md"))
				if err != nil || string(data) != original {
					t.Fatalf("copied bytes at %s: %q, %v", target, data, err)
				}
				if info, err := os.Stat(filepath.Join(target, "nested", "empty")); err != nil || !info.IsDir() {
					t.Fatalf("missing copied empty directory: %v", err)
				}
				if len(removeSnapshot(t, target)) != 4 {
					t.Fatalf("unexpected replacement contents at %s", target)
				}
				setupAbsent(t, filepath.Join(target, "destination-only"))
			}
			for _, restart := range []bool{false, true} {
				t.Run(fmt.Sprintf("restart=%t", restart), func(t *testing.T) {
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
						from = screen.Len()
						if _, err := io.WriteString(master, text); err != nil {
							t.Fatal(err)
						}
					}
					// Read a complete help redraw, not a history of terminal diffs.
					// If refresh is still pending, toggle views for a fresh acknowledgment.
					awaitHelp := func(texts ...string) {
						t.Helper()
						for {
							await("q quit")
							output := ansi.Strip(screen.String()[from:])
							ready := strings.Contains(output, "Root relations checked; every mutation revalidates.")
							for _, text := range texts {
								ready = ready && strings.Contains(output, text)
							}
							if ready {
								return
							}
							send("?")
							await("Configured folders")
							send("?")
						}
					}
					scopeLabel := strings.ToUpper(scope[:1]) + scope[1:]
					await("Configured folders")
					await("Ready")
					await("> replace-me")
					if !restart {
						for i, key := range "abc" {
							if scope == "global" {
								key -= 'a' - 'A'
							}
							send(string(key) + "?")
							awaitHelp("Focused: Library", "Selected name: replace-me",
								fmt.Sprintf(`Result: Add \"replace-me\" to Agent%d / %s (%s): complete`, i+1, scopeLabel, resultPath(i)))
							assertCopy(t, targets[i])
							assertUnchanged(t)
							send("?")
							await("Configured folders")
						}
					}
					for i := range 3 {
						focus := fmt.Sprint(i + 1)
						if scope == "global" {
							focus = "g" + focus
						}
						// Up selects the copied row even when fresh-add preserved survivor.
						send(focus + "\x1b[A?")
						selected := "replace-me"
						if restart && i < 2 {
							selected = "survivor"
						}
						awaitHelp(fmt.Sprintf("Focused: Agent%d / %s", i+1, scopeLabel), "Selected name: "+selected)
						send("?")
						await("Configured folders")
						if !restart && i < 2 {
							send("x?")
							awaitHelp("Selected name: survivor",
								fmt.Sprintf(`Result: Remove \"replace-me\" from Agent%d / %s (%s): complete`, i+1, scopeLabel, resultPath(i)))
							setupAbsent(t, targets[i])
							assertUnchanged(t)
							send("?")
							await("Configured folders")
						}
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
				})
				setupAbsent(t, targets[0])
				setupAbsent(t, targets[1])
				assertCopy(t, targets[2])
				assertUnchanged(t)
			}
		})
	}
}
