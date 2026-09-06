# Terminal Lifecycle

## Policy

Owner-approved September 5, 2026: `q`, raw Ctrl+C, SIGINT, SIGTERM, and SIGHUP
are ordinary exit requests. `runLifecycle` owns one `signal.Notify` loop until
cleanup finishes, disables Tea's signal handler, and sends `exitRequestMsg`.
Only model `Update` decides to quit. Repeated signals have no forced-exit shortcut;
OS signal coalescing is harmless because requests are idempotent.

Idle quit does not join read-only scans. Task 13 defers ordinary quit until its
real sequential mutation completes. Navigation remains responsive before quit;
additional mutations are rejected and refresh coalesces. Pending-quit failure
returns sanitized stderr/status `1` after restoration. Task 18 busy-quit PTY tests
pass on Linux; native macOS/manual checks and Task 19 diagnostic coverage remain pending.
HUP follows the same wait-and-restore policy, but a disconnected terminal may no
longer accept restoration or diagnostics. SIGKILL/forced termination cannot run
cleanup and mutations may leave partial work.

## Reviewed APIs

- [Bubble Tea v2.0.9 `tea.go`](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tea.go):
  `handleSignals` sends SIGINT as `InterruptMsg`, TERM as `QuitMsg`, then exits;
  `eventLoop` handles both before `Update`. No default HUP handler exists.
  `WithoutSignalHandler`, `Program.Send`, and ordinary model messages avoid this
  bypass. `handleCommands` explicitly does not join command goroutines; `Run`
  returning is not evidence of mutation completion.
- `Run` normally calls `shutdown`, but some initialization errors return after
  raw mode without shutdown. `shutdown` discards terminal restoration errors.
  `runLifecycle` saves the input fd and `term.State` before `Run`, then explicitly
  calls `term.Restore` afterward, joining any error with the runtime error.
  It never repeats renderer cleanup: `ReleaseTerminal` can block sending to an
  unstarted renderer after cancelreader initialization fails; after normal shutdown
  it can force-flush a retained application frame onto the main screen.
  An owned `WithContext` is canceled after `Run`, before joining the signal loop,
  including when a nil model makes `Run` return before its internal cancellation.
  Final errors return to `run`, which escapes diagnostic text before stderr;
  only `main` chooses process exit after cleanup.
- Pinned Ultraviolet `terminal_reader.go:231-235` sends `eventc <- event` without
  selecting on cancellation. An input burst during exit can leave an input
  goroutine blocked until process termination; Tea's bounded read-loop wait does
  not prove that it joined. We join our signal loop and test PTY reader, not all
  upstream workers. No dependency patch or safe in-process reuse claim is made.
- Upstream `recoverFromPanic` restores then prints raw panic/stack text directly
  to `os.Stderr`; `recoverFromGoPanic` can print before cleanup. Those upstream
  panic diagnostics are not a sanitized application-error API. The targeted
  runtime test instead fails the real terminal reader after actual input, testing
  the returned-error path without adding a production panic wrapper/framework.
  Upstream also ignores some output/flush errors; a broken terminal cannot be
  promised visible restoration. We verify restoration sequences, not emulator
  internals or restoration of arbitrary preexisting cursor modes.
- Owner-approved test-only [creack/pty v1.1.24](https://github.com/creack/pty/tree/v1.1.24):
  reviewed Linux/macOS `Open` implementations and `Setsize` ioctl. No module
  dependencies of its own. Existing `x/term v0.2.2` uses native termios ioctls for
  `IsTerminal`, `GetState`, and `Restore`; tests compare complete saved states.
  Existing `x/sys v0.47.0` is promoted to direct test use for nonblocking
  `unix.Read`/`Poll`: retry EAGAIN/EINTR and check cancellation between bounded
  poll waits. This avoids relying on Darwin's PTY fd being in Go's runtime poller.

## Evidence

Linux amd64, kernel `6.12.90+deb13.1-cloud-amd64`, Go 1.27.1, disposable PTY,
`TERM=xterm-256color`, 100x30, no-color: focused PTY, full tests, focused/full race,
lint/format/config verification, vet, module tidy, and build pass.

`TestPTYLifecycle` builds the actual CLI and checks `q`, byte `0x03`, INT/TERM/HUP,
pre-lifecycle config failure, post-raw cancelreader initialization failure,
runtime reader failure, exact mode restoration, ordered cursor
show/alternate-screen leave, stdin-only and stdout-only non-TTY rejection, and
non-TTY help/version despite invalid native config. Runtime diagnostics are
captured separately and accepted only after modes/cursor/screen are restored.
No printable application output or alternate-screen reentry may follow the first
alternate-screen leave. The initialization fault is test-only and triggers after
observing raw mode: Linux gets an invalid fd at epoll registration; Darwin gets
a zero soft descriptor limit immediately before kqueue allocation in `Name()`.
The hard limit and existing descriptors remain intact, and the original soft
limit is restored before assertions or process exit.
Test-only subprocess wrappers acknowledge three successive signals through fd 3
to prove every delivered request reaches `Update`, without inventing mutation
work. Render readiness and fd acknowledgments coordinate tests; build/process/
drain deadlines fail, never skip. Closed channels are disabled; unexpected reader
errors are reported, and reader cancellation/join precedes fd closure. No sleeps
or shipped test flags/environment hooks exist.

Every child has disposable HOME, Linux XDG and macOS native config, cwd, and
configured paths. Existing full-test CI picks these tests up automatically;
no workflow changes. Tasks 8-12 passed all four native jobs in run `33994932478`.
Manual Terminal.app/emulator/SSH checks remain pending; PTYs are not that evidence.

## macOS CI Regression

[Run 33970232319](https://github.com/primaprashant/sei/actions/runs/33970232319)
passed both Linux jobs but failed both macOS jobs in the PTY harness. The parent
queried the slave after controlling-session-leader exit, when Darwin revokes that
terminal. The harness now creates a session without acquiring a controlling
terminal; explicit stdin/stdout PTY fds still exercise raw mode and full state
restoration. It does not test foreground job control or kernel-generated hangup;
SIGINT/TERM/HUP are delivered explicitly. No ENOTTY suppression or macOS skips.

The invalid-fd initialization fault also assumed Linux epoll behavior. Darwin's
cancelreader constructor only prepares kevents; it submits them later during
reading, so an invalid fd did not force constructor failure. The allocation fault
above tests the intended post-raw initialization error on Darwin instead.
Linux standard checks/build, five PTY repetitions, and three PTY race repetitions
pass after the fix; both Darwin architectures cross-compile. Native verification
of these fixes subsequently passed in runs `33970786802` and `33994932478`.

Phase E Linux model/race and real add/remove/replace/restart PTY checks pass.
Help always exposes busy/pending-quit status in its footer. Native execution passed
all four jobs in CI `33999522026`; ordinary active-operation signal proof is Task 18.
