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
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

// This wrapper observes repeated requests, not simulated mutation work. Each
// acknowledgment proves Update received a signal before the next is delivered.
type lifecycleProbe struct {
	browseModel
	requests int
	ack      *os.File
}

func (m lifecycleProbe) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(exitRequestMsg); ok {
		m.requests++
		if _, err := m.ack.Write([]byte{byte(m.requests)}); err != nil {
			panic(err)
		}
		if m.requests < 3 {
			return m, nil
		}
	}
	next, cmd := m.browseModel.Update(msg)
	m.browseModel = next.(browseModel)
	return m, cmd
}

type lifecycleReadFailure struct{ *os.File }

// Tea's raw-mode setup gets the real fd; cancelreader initialization gets an
// invalid one. No descriptor is closed and restoration uses the saved real fd.
type lifecycleInitFailure struct {
	*os.File
	before  *term.State
	tripped bool
}

func (r *lifecycleInitFailure) Fd() uintptr {
	state, err := term.GetState(r.File.Fd())
	if err == nil && !reflect.DeepEqual(r.before, state) {
		r.tripped = true
		return ^uintptr(0)
	}
	return r.File.Fd()
}

func (r lifecycleReadFailure) Read(b []byte) (int, error) {
	if _, err := r.File.Read(b); err != nil {
		return 0, err
	}
	return 0, errors.New("injected read failure\x1b[31m\nunsafe")
}

type lifecycleDiagnostics struct {
	*os.File
	before *term.State
	output *lifecycleOutput
}

type lifecycleOutput struct {
	*os.File
	bytes bytes.Buffer
}

func (w *lifecycleOutput) Write(b []byte) (int, error) {
	_, _ = w.bytes.Write(b)
	return w.File.Write(b)
}

func (w lifecycleDiagnostics) Write(b []byte) (int, error) {
	after, err := term.GetState(os.Stdin.Fd())
	if err != nil || !reflect.DeepEqual(w.before, after) {
		return 0, errors.New("diagnostic attempted before terminal restoration")
	}
	text := w.output.bytes.String()
	for _, pair := range [][2]string{{"\x1b[?1049h", "\x1b[?1049l"}, {"\x1b[?25l", "\x1b[?25h"}} {
		if strings.LastIndex(text, pair[1]) <= strings.LastIndex(text, pair[0]) {
			return 0, errors.New("diagnostic attempted before screen/cursor restoration")
		}
	}
	return w.File.Write(b)
}

// Only the test executable understands these arguments. The shipped CLI has no
// coordination flags, environment variables, or failure/delay hooks.
func TestPTYLifecycleProcess(t *testing.T) {
	args := os.Args
	for i, arg := range args {
		if arg != "--" || i+2 >= len(args) {
			continue
		}
		mode, path := args[i+1], args[i+2]
		if mode == "init-failure" {
			before, err := term.GetState(os.Stdin.Fd())
			if err != nil {
				t.Fatal(err)
			}
			input := &lifecycleInitFailure{File: os.Stdin, before: before}
			code := run([]string{"--config", path}, input, os.Stdout, os.Stderr)
			after, err := term.GetState(os.Stdin.Fd())
			if code != 1 || !input.tripped || err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("post-raw failure: code=%d tripped=%v restored=%v err=%v", code, input.tripped, reflect.DeepEqual(before, after), err)
			}
			_, _ = fmt.Fprintln(os.Stderr, "post-raw fault verified")
			os.Exit(code)
		}
		if mode == "read-failure" {
			before, err := term.GetState(os.Stdin.Fd())
			if err != nil {
				t.Fatal(err)
			}
			output := &lifecycleOutput{File: os.Stdout}
			os.Exit(run([]string{"--config", path}, lifecycleReadFailure{os.Stdin}, output, lifecycleDiagnostics{os.Stderr, before, output}))
		}
		cfg, _, err := loadConfig(path)
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
		ack := os.NewFile(3, "ack")
		_, err = runLifecycle(lifecycleProbe{browseModel: newBrowseModel(cfg), ack: ack}, os.Stdin, os.Stdout)
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, displayText(err.Error()))
			os.Exit(1)
		}
		os.Exit(0)
	}
}

func TestPTYLifecycle(t *testing.T) {
	buildDir := t.TempDir()
	binary := filepath.Join(buildDir, "sei")
	buildCtx, buildCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer buildCancel()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", binary, ".")
	build.WaitDelay = 5 * time.Second
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"q", "ctrl-c", "SIGINT", "SIGTERM", "SIGHUP", "repeated-SIGINT", "repeated-SIGTERM", "repeated-SIGHUP", "read-failure", "init-failure", "startup-failure", "stdin-pipe", "stdout-pipe", "help", "version"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "config.json")
			writeTestFile(t, path, validConfig)
			// Both native locations are isolated, including macOS's non-XDG path.
			for _, native := range []string{".config/sei", "Library/Application Support/sei"} {
				dir := filepath.Join(root, native)
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, filepath.Join(dir, "config.json"), "invalid native config")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			args := []string{"--config", path}
			program := binary
			repeated := strings.HasPrefix(scenario, "repeated-")
			if repeated || scenario == "read-failure" || scenario == "init-failure" {
				program, args = self, []string{"-test.run=^TestPTYLifecycleProcess$", "--", scenario, path}
			}
			if scenario == "startup-failure" {
				writeTestFile(t, path, "invalid\x1b[31m")
			}
			if scenario == "help" || scenario == "version" {
				args = []string{"--" + scenario} // Invalid native config must not be read.
			}
			cmd := exec.CommandContext(ctx, program, args...)
			cmd.WaitDelay = 5 * time.Second
			cmd.Dir = root
			cmd.Env = []string{"HOME=" + root, "XDG_CONFIG_HOME=" + filepath.Join(root, ".config"), "TERM=xterm-256color", "NO_COLOR=1", "PATH=" + os.Getenv("PATH")}
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
			if err := pty.Setsize(master, &pty.Winsize{Rows: 30, Cols: 100}); err != nil {
				t.Fatal(err)
			}
			masterFD := int(master.Fd())
			if err := unix.SetNonblock(masterFD, true); err != nil {
				t.Fatal(err)
			}
			before, err := term.GetState(slave.Fd())
			if err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, &stderr
			cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
			interactive := scenario != "stdin-pipe" && scenario != "stdout-pipe" && scenario != "help" && scenario != "version" && scenario != "startup-failure" && scenario != "init-failure"
			if scenario == "stdin-pipe" || scenario == "help" || scenario == "version" {
				cmd.Stdin = strings.NewReader("")
				cmd.SysProcAttr = nil
			}
			if scenario == "stdout-pipe" || scenario == "help" || scenario == "version" {
				cmd.Stdout = &stdout
			}
			ackR, ackW, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				for _, f := range []*os.File{ackR, ackW} {
					if err := f.Close(); err != nil {
						t.Error(err)
					}
				}
			}()
			if repeated {
				cmd.ExtraFiles = []*os.File{ackW}
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			wait := make(chan struct{})
			var waitErr error
			go func() {
				waitErr = cmd.Wait()
				close(wait)
			}()
			defer func() {
				cancel()
				<-wait
			}()
			chunks := make(chan string)
			readerCtx, stopReader := context.WithCancel(ctx)
			readerDone := make(chan struct{})
			var readerErr error
			go func() {
				defer close(readerDone)
				defer close(chunks)
				readerErr = readLifecyclePTY(readerCtx, masterFD, chunks)
			}()
			defer func() {
				stopReader()
				<-readerDone
				if readerErr != nil && !errors.Is(readerErr, context.Canceled) {
					t.Errorf("PTY reader: %v", readerErr)
				}
			}()
			var screen strings.Builder
			if interactive {
				for !strings.Contains(screen.String(), "Read-only configured folders") {
					select {
					case chunk, ok := <-chunks:
						if !ok {
							t.Fatalf("PTY closed before first frame: %v", readerErr)
						}
						screen.WriteString(chunk)
					case <-ctx.Done():
						t.Fatal("timeout waiting for first frame")
					}
				}
				raw, err := term.GetState(slave.Fd())
				if err != nil || reflect.DeepEqual(before, raw) {
					t.Fatalf("terminal did not enter raw mode: %v", err)
				}
				if sig := map[string]syscall.Signal{"SIGINT": syscall.SIGINT, "SIGTERM": syscall.SIGTERM, "SIGHUP": syscall.SIGHUP}[strings.TrimPrefix(scenario, "repeated-")]; sig != 0 {
					count := 1
					if repeated {
						count = 3
					}
					for i := 1; i <= count; i++ {
						if err := cmd.Process.Signal(sig); err != nil {
							t.Fatal(err)
						}
						if repeated {
							if err := ackR.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
								t.Fatal(err)
							}
							var ack [1]byte
							if _, err := io.ReadFull(ackR, ack[:]); err != nil || ack[0] != byte(i) {
								t.Fatalf("request %d did not reach Update: %v, %v", i, ack, err)
							}
						}
					}
				} else {
					key := "q"
					switch scenario {
					case "ctrl-c":
						key = "\x03"
					case "read-failure":
						key = "!"
					}
					if _, err := io.WriteString(master, key); err != nil {
						t.Fatal(err)
					}
				}
			}
		waiting:
			for {
				select {
				case chunk, ok := <-chunks:
					if !ok {
						chunks = nil
						if readerErr != nil {
							t.Fatalf("PTY reader: %v", readerErr)
						}
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
			wantCode := 0
			if scenario == "read-failure" || scenario == "init-failure" || scenario == "startup-failure" || strings.HasSuffix(scenario, "-pipe") {
				wantCode = 1
			}
			if ctx.Err() != nil || cmd.ProcessState.ExitCode() != wantCode {
				t.Fatalf("exit=%d want=%d: %v; stderr=%q", cmd.ProcessState.ExitCode(), wantCode, waitErr, stderr.String())
			}
			if interactive {
				if leave := strings.Index(screen.String(), "\x1b[?1049l"); leave >= 0 {
					afterLeave := screen.String()[leave+len("\x1b[?1049l"):]
					if strings.TrimSpace(ansi.Strip(afterLeave)) != "" || strings.Contains(afterLeave, "\x1b[?1049h") {
						t.Errorf("application rendered after alternate-screen leave: %q", afterLeave)
					}
				}
				for _, pair := range [][2]string{{"\x1b[?1049h", "\x1b[?1049l"}, {"\x1b[?25l", "\x1b[?25h"}} {
					if strings.LastIndex(screen.String(), pair[0]) < 0 || strings.LastIndex(screen.String(), pair[1]) <= strings.LastIndex(screen.String(), pair[0]) {
						t.Errorf("missing ordered terminal restoration %q: %q", pair, screen.String())
					}
				}
			} else if strings.ContainsAny(screen.String()+stdout.String(), "\x1b") {
				t.Errorf("noninteractive/startup failure emitted terminal controls: %q %q", screen.String(), stdout.String())
			}
			if strings.ContainsAny(stderr.String(), "\x1b\r") || (wantCode == 0 && stderr.Len() != 0) {
				t.Errorf("unexpected/unsafe stderr: %q", stderr.String())
			}
			if scenario == "read-failure" && !strings.Contains(stderr.String(), `injected read failure\x1b[31m\nunsafe`) {
				t.Errorf("missing sanitized persistent failure: %q", stderr.String())
			}
			if scenario == "init-failure" && (!strings.Contains(stderr.String(), "could not create cancelable reader") || !strings.Contains(stderr.String(), "post-raw fault verified")) {
				t.Errorf("missing post-raw initialization failure evidence: %q", stderr.String())
			}
			if strings.HasSuffix(scenario, "-pipe") && !strings.Contains(stderr.String(), "requires terminal stdin and stdout") {
				t.Errorf("missing TTY diagnostic: %q", stderr.String())
			}
			if (scenario == "help" || scenario == "version") && !strings.Contains(stdout.String(), "sei") {
				t.Errorf("missing requested output: %q", stdout.String())
			}
		})
	}
}

// Darwin's pty.Open returns a descriptor not registered with Go's poller.
// Poll and Read explicitly so EAGAIN is retryable, and cancellation is bounded
// even while the parent retains the slave for the final termios comparison.
func readLifecyclePTY(ctx context.Context, fd int, chunks chan<- string) error {
	buf := make([]byte, 8192)
	poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := unix.Poll(poll, 100)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return err
		}
		if n == 0 {
			continue
		}
		n, err = unix.Read(fd, buf)
		if errors.Is(err, unix.EINTR) || errors.Is(err, unix.EAGAIN) {
			continue
		}
		if runtime.GOOS == "linux" && errors.Is(err, unix.EIO) {
			return nil // Linux reports slave closure as EIO, Darwin as EOF.
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return nil
		}
		select {
		case chunks <- string(buf[:n]):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
