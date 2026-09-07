package app

import (
	"bufio"
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
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

type mutationQuitProbe struct {
	browseModel
	ack, release *os.File
	completed    *atomic.Bool
	starts       int
	fail         bool
}

func (m mutationQuitProbe) report(event string) {
	if _, err := fmt.Fprintln(m.ack, event); err != nil {
		panic(err)
	}
}

func (m mutationQuitProbe) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	previous := m.browseModel
	next, cmd := previous.Update(msg)
	m.browseModel = next.(browseModel)
	if previous.active == nil && m.active != nil {
		// Exceed the PTY output buffer before acknowledging input, even on
		// Linux. The parent must keep draining output while waiting for acks.
		if _, err := io.WriteString(os.Stdout, strings.Repeat("\x1b[0m", 16*1024)); err != nil {
			panic(err)
		}
		m.starts++
		r, cfg := *m.active, m.config
		p, d := previous.panels[previous.focused], previous.panels[m.active.destination]
		// Replace only the mutation command to install existing per-call seams.
		// Input guards, captured scan validation, operations and results stay real.
		cmd = func() tea.Msg {
			if err := validateScannedSelection(cfg, r, p.root, d.root, p.entries[p.selected]); err != nil {
				return mutationResult{id: r.id, err: err}
			}
			gate := func() {
				m.report("started")
				var b [1]byte
				if _, err := io.ReadFull(m.release, b[:]); err != nil || b[0] != 'R' {
					panic(fmt.Sprintf("release: %q %v", b, err))
				}
			}
			var err error
			if r.add {
				var files []*os.File
				held := false
				err = addSkillWithOps(cfg, r.destination, r.name, copySkillOps{copy: func(w io.Writer, reader io.Reader) (int64, error) {
					if w == io.Discard || held || filepath.Base(reader.(*os.File).Name()) != "file" {
						return io.Copy(w, reader)
					}
					held = true
					files = append(files, reader.(*os.File), w.(*os.File))
					n, err := io.CopyN(w, reader, 2)
					if err != nil {
						return n, err
					}
					gate()
					for _, f := range files {
						if _, err := f.Stat(); err != nil {
							return n, fmt.Errorf("file closed while waiting: %w", err)
						}
					}
					if m.fail {
						return n, errors.New("injected copy failure\x1b[31m\nunsafe")
					}
					rest, err := io.Copy(w, reader)
					return n + rest, err
				}})
				for _, f := range files {
					if _, statErr := f.Stat(); !errors.Is(statErr, os.ErrClosed) {
						err = errors.Join(err, errors.New("copy file not closed after completion"))
					}
				}
			} else {
				calls := 0
				err = removeSkillObserved(cfg, r.destination, r.name, func(string) {
					calls++
					if calls == 2 {
						gate() // First postorder removal has already completed.
					}
				})
			}
			if !m.fail && err != nil {
				panic(err)
			}
			m.completed.Store(true)
			m.report("completed")
			return mutationResult{id: r.id, err: err}
		}
	}
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		m.report("key:" + msg.String())
	case exitRequestMsg:
		m.report("signal")
	case tea.WindowSizeMsg:
		if msg.Width == 79 || previous.width == 79 {
			m.report(fmt.Sprintf("resize:%dx%d", msg.Width, msg.Height))
		}
	}
	return m, cmd
}

type mutationQuitOutput struct {
	*lifecycleOutput
	completed, earlyCleanup *atomic.Bool
}

func (w mutationQuitOutput) Write(b []byte) (int, error) {
	if !w.completed.Load() && (bytes.Contains(b, []byte("\x1b[?1049l")) || bytes.Contains(b, []byte("\x1b[?25h"))) {
		w.earlyCleanup.Store(true)
	}
	return w.lifecycleOutput.Write(b)
}

// Only the test executable accepts this private wrapper invocation and pipe fds.
func TestPTYQuitDuringMutationProcess(t *testing.T) {
	if len(os.Args) < 4 || !strings.HasPrefix(os.Args[len(os.Args)-2], "--mutation-quit") {
		return
	}
	mode := os.Args[len(os.Args)-2]
	fail := mode != "--mutation-quit"
	cfg, _, err := loadConfig(os.Args[len(os.Args)-1])
	if err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err = resolveConfigPaths(cfg, cwd)
	if err != nil {
		t.Fatal(err)
	}
	ack, release := os.NewFile(3, "ack"), os.NewFile(4, "release")
	var completed, earlyCleanup atomic.Bool
	before, err := term.GetState(os.Stdin.Fd())
	if err != nil {
		t.Fatal(err)
	}
	output := &lifecycleOutput{File: os.Stdout}
	final, err := runLifecycle(mutationQuitProbe{browseModel: newBrowseModel(cfg), ack: ack, release: release, completed: &completed, fail: fail}, os.Stdin, mutationQuitOutput{output, &completed, &earlyCleanup})
	if err != nil {
		t.Fatal(err)
	}
	m := final.(mutationQuitProbe)
	wantFailure := mode == "--mutation-quit-failure"
	if !completed.Load() || earlyCleanup.Load() || m.starts != 1 || m.nextOperation != 1 || m.active != nil || m.pendingQuit != (mode != "--mutation-quit-recoverable") || (m.exitError != nil) != wantFailure {
		t.Fatalf("premature cleanup, canceled/replayed work or bad final state: completed=%v early=%v model=%+v", completed.Load(), earlyCleanup.Load(), m)
	}
	if err := errors.Join(ack.Close(), release.Close()); err != nil {
		t.Fatal(err)
	}
	os.Exit(browseExit(m.browseModel, nil, lifecycleDiagnostics{os.Stderr, before, output}))
}

func TestPTYQuitDuringMutation(t *testing.T) {
	testPTYMutationExit(t, false)
}

func TestPTYQuitFailure(t *testing.T) {
	testPTYMutationExit(t, true)
}

func testPTYMutationExit(t *testing.T, fail bool) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"copy", "delete"} {
		if fail && operation != "copy" {
			continue
		}
		quits := []string{"q", "ctrl-c", "repeated-SIGINT"}
		if fail {
			quits = append(quits, "later-q")
		}
		for _, quit := range quits {
			t.Run(operation+"/"+quit, func(t *testing.T) {
				root := t.TempDir()
				if quit == "later-q" {
					// Exercise diagnostic truncation even with short Linux temp paths.
					root = filepath.Join(root, strings.Repeat("long-path-", 16))
					browseMkdir(t, root)
				}
				root, err := filepath.EvalSymlinks(root)
				if err != nil {
					t.Fatal(err)
				}
				name := "active"
				local := "local"
				if fail {
					name += "\tunsafe"
					local += "\tpath"
				}
				cfg := config{Library: filepath.Join(root, "skill-library"), Agents: []agentConfig{{Name: "Agent", Global: filepath.Join(root, "global"), Local: local}}}
				for _, dir := range []string{"skill-library", "global", local} {
					for _, name := range []string{name, "survivor"} {
						browseMkdir(t, filepath.Join(root, dir, name, "nested"))
						writeTestFile(t, filepath.Join(root, dir, name, "nested", "file"), "original bytes\x00\xff")
						writeTestFile(t, filepath.Join(root, dir, name, ".hidden"), "hidden bytes")
					}
				}
				if operation == "copy" {
					if err := os.RemoveAll(filepath.Join(root, local, name)); err != nil {
						t.Fatal(err)
					}
				}
				path := filepath.Join(root, "config.json")
				if err := saveConfig(cfg, root, path, false); err != nil {
					t.Fatal(err)
				}
				library, global := removeSnapshot(t, cfg.Library), removeSnapshot(t, cfg.Agents[0].Global)
				configBefore := removeSnapshot(t, path)
				survivor := filepath.Join(root, local, "survivor")
				survivorBefore := removeSnapshot(t, survivor)
				target := filepath.Join(root, local, name)
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				mode := "--mutation-quit"
				if fail {
					mode += "-failure"
					if quit == "later-q" {
						mode = "--mutation-quit-recoverable"
					}
				}
				cmd := exec.CommandContext(ctx, self, "-test.run=^TestPTYQuitDuringMutationProcess$", "--", mode, path)
				cmd.Dir = root
				cmd.Env = []string{"HOME=" + root, "XDG_CONFIG_HOME=" + filepath.Join(root, ".config"), "TERM=xterm-256color", "NO_COLOR=1", "PATH=" + t.TempDir()}
				// Self-exec coverage output is disposable, never application stderr.
				cmd.Env = append(cmd.Env, "GOCOVERDIR="+t.TempDir())
				cmd.WaitDelay = 5 * time.Second
				master, slave, err := pty.Open()
				if err != nil {
					t.Fatal(err)
				}
				files := []*os.File{master, slave}
				defer func() {
					for _, f := range files {
						if err := f.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
							t.Error(err)
						}
					}
				}()
				ackR, ackW, err := os.Pipe()
				if err != nil {
					t.Fatal(err)
				}
				files = append(files, ackR, ackW)
				releaseR, releaseW, err := os.Pipe()
				if err != nil {
					t.Fatal(err)
				}
				files = append(files, releaseR, releaseW)
				cmd.ExtraFiles = []*os.File{ackW, releaseR}
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
				if err := errors.Join(ackW.Close(), releaseR.Close()); err != nil {
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
				from := 0
				await := func(text string) {
					t.Helper()
					// Help hard-wraps long paths/results; escaped newlines remain literal.
					for !strings.Contains(strings.NewReplacer("\r", "", "\n", "").Replace(ansi.Strip(screen.String()[from:])), text) {
						select {
						case chunk, ok := <-chunks:
							if !ok {
								t.Fatalf("PTY closed waiting for %q: %s", text, screen.String())
							}
							screen.WriteString(chunk)
						case <-wait:
							t.Fatalf("exited waiting for %q: %v stderr=%q", text, waitErr, stderr.String())
						case <-ctx.Done():
							t.Fatalf("timeout waiting for %q: %s", text, screen.String())
						}
					}
				}
				scanner := bufio.NewScanner(ackR)
				ack := func(want string) {
					t.Helper()
					if err := ackR.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
						t.Fatal(err)
					}
					// A redraw can fill the PTY buffer (especially on Darwin),
					// blocking Update before it can acknowledge the next event.
					// Read both streams until the bounded scan finishes.
					scanned := make(chan bool)
					go func() { scanned <- scanner.Scan() }()
					for {
						select {
						case ok := <-scanned:
							if !ok || scanner.Text() != want {
								t.Fatalf("ack=%q want=%q err=%v", scanner.Text(), want, scanner.Err())
							}
							return
						case chunk, ok := <-chunks:
							if !ok {
								chunks = nil
							} else {
								screen.WriteString(chunk)
							}
						}
					}
				}
				send := func(key, name string) {
					t.Helper()
					if _, err := io.WriteString(master, key); err != nil {
						t.Fatal(err)
					}
					ack("key:" + name)
				}
				await("Ready")
				await("> active")
				// Help confirms the actual target's scan and safety are ready.
				send("1", "1")
				send("?", "?")
				if operation == "copy" {
					await("Selected name: survivor")
				} else {
					await("Selected name: active")
				}
				await("Root relations checked; every mutation revalidates.")
				from = screen.Len()
				send("?", "?")
				await("PROJECT")
				if operation == "copy" {
					send("0", "0")
					send("a", "a")
				} else {
					send("x", "x")
				}
				ack("started")
				from = screen.Len()
				if err := pty.Setsize(master, &pty.Winsize{Rows: 24, Cols: 79}); err != nil {
					t.Fatal(err)
				}
				if err := cmd.Process.Signal(syscall.SIGWINCH); err != nil {
					t.Fatal(err)
				}
				ack("resize:79x24")
				await("Resize to at least 80x24")
				await("Working; q waits for completion")
				for _, key := range []string{"0", "a", "1", "x"} {
					send(key, key)
				}
				// Restore the original focus before the existing navigation proof.
				if operation == "copy" {
					send("0", "0")
				}
				from = screen.Len()
				if err := pty.Setsize(master, &pty.Winsize{Rows: 40, Cols: 240}); err != nil {
					t.Fatal(err)
				}
				if err := cmd.Process.Signal(syscall.SIGWINCH); err != nil {
					t.Fatal(err)
				}
				ack("resize:240x40")
				await("PROJECT")
				from = screen.Len()
				send("\x1b[B", "down")
				send("?", "?")
				await("Selected name: survivor")
				// Busy text was verified in the resize frame above. The renderer
				// may retain that same line while scrolling into help.
				from = screen.Len()
				send("?", "?")
				await("PROJECT")
				// Exercise valid add and remove contexts while busy, then change focus.
				for _, key := range []string{"0", "a", "A", "1", "x", "r", "g", "1"} {
					send(key, key)
				}
				from = screen.Len()
				send("?", "?")
				await("Focused: Agent / Global")
				from = screen.Len()
				send("?", "?")
				await("PROJECT")
				switch quit {
				case "later-q":
					// Release and observe the recoverable error before requesting quit.
				case "repeated-SIGINT":
					for range 3 {
						if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
							t.Fatal(err)
						}
						ack("signal") // Prevent OS coalescing from hiding repeated requests.
					}
				case "ctrl-c":
					for range 3 {
						send("\x03", "ctrl+c")
					}
				default:
					send("q", "q")
				}
				if quit != "later-q" {
					await("Exit requested; waiting for work")
				}
				for _, key := range []string{"x", "0", "a", "A", "1", "x", "r"} {
					send(key, key)
				}
				select {
				case <-wait:
					t.Fatal("process exited before release")
				default:
				}
				raw, err := term.GetState(slave.Fd())
				if err != nil || reflect.DeepEqual(before, raw) || strings.Contains(screen.String(), "\x1b[?1049l") {
					t.Fatalf("terminal restored before completion: %v", err)
				}
				if operation == "copy" {
					data, err := os.ReadFile(filepath.Join(target, "nested", "file"))
					if err != nil || string(data) != "or" {
						t.Fatalf("expected real partial copy: %q %v", data, err)
					}
				} else {
					if len(removeSnapshot(t, target)) != len(survivorBefore)-1 {
						t.Fatal("expected exactly one real removal before release")
					}
				}
				if quit == "later-q" {
					// Browse status is intentionally truncated. Inspect the full result in
					// help, opened while work is gated so no completion can race the toggle.
					from = screen.Len()
					send("?", "?")
					await(" | up/down scroll | ?/Esc close |")
					from = screen.Len()
				}
				if _, err := releaseW.Write([]byte("R")); err != nil {
					t.Fatal(err)
				}
				ack("completed")
				if quit == "later-q" {
					await(`injected copy failure\x1b[31m\nunsafe`)
					send("q", "q")
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
						t.Fatal("process timeout")
					}
				}
				after, err := term.GetState(slave.Fd())
				if err != nil || !reflect.DeepEqual(before, after) {
					t.Fatalf("terminal not restored: %v", err)
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
				wantCode := 0
				if fail && quit != "later-q" {
					wantCode = 1
					for _, text := range []string{"Add", displayText(fmt.Sprintf("%q", name)), "Agent / Project", displayText(filepath.Dir(target)), `injected copy failure\x1b[31m\nunsafe`} {
						if !strings.Contains(stderr.String(), text) {
							t.Errorf("missing persistent diagnostic %q: %q", text, stderr.String())
						}
					}
					if strings.ContainsAny(stderr.String(), "\x1b\r\t") || strings.Count(stderr.String(), "\n") != 1 || strings.Contains(stderr.String(), "survivor") {
						t.Errorf("unsafe or redirected diagnostic: %q", stderr.String())
					}
				}
				if cmd.ProcessState.ExitCode() != wantCode || (wantCode == 0 && stderr.Len() != 0) {
					t.Fatalf("exit: %v stderr=%q", waitErr, stderr.String())
				}
				for _, pair := range [][2]string{{"\x1b[?1049h", "\x1b[?1049l"}, {"\x1b[?25l", "\x1b[?25h"}} {
					if !strings.Contains(screen.String(), pair[0]) || strings.Index(screen.String(), pair[1]) <= strings.LastIndex(screen.String(), pair[0]) {
						t.Errorf("missing ordered restoration %q", pair)
					}
				}
				leave := strings.Index(screen.String(), "\x1b[?1049l")
				if leave >= 0 && strings.TrimSpace(ansi.Strip(screen.String()[leave+len("\x1b[?1049l"):])) != "" {
					t.Error("rendered after alternate-screen exit")
				}
				if fail {
					data, err := os.ReadFile(filepath.Join(target, "nested", "file"))
					if err != nil || string(data) != "or" {
						t.Fatalf("failure lost partial output: %q %v", data, err)
					}
				} else if operation == "copy" {
					source := filepath.Join(cfg.Library, name)
					want, got := removeSnapshot(t, source), removeSnapshot(t, target)
					if len(want) != len(got) {
						t.Fatal("incomplete copy")
					}
					for path, entry := range want {
						rel, err := filepath.Rel(source, path)
						if err != nil {
							t.Fatal(err)
						}
						copied, ok := got[filepath.Join(target, rel)]
						if !ok || copied.data != entry.data || copied.info.Mode().Type() != entry.info.Mode().Type() || os.SameFile(copied.info, entry.info) {
							t.Fatalf("missing, corrupt or non-independent copy: %s", rel)
						}
					}
				} else {
					setupAbsent(t, target)
				}
				assertRemoveSnapshot(t, cfg.Library, library)
				assertRemoveSnapshot(t, cfg.Agents[0].Global, global)
				assertRemoveSnapshot(t, survivor, survivorBefore)
				assertRemoveSnapshot(t, path, configBefore)
			})
		}
	}
}
