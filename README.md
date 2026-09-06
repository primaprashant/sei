# sei

A terminal skill-folder manager for copying skills from a personal library to
configured agent destinations and removing installed copies. Not a skill
marketplace, agent launcher, or agent skill. See [the PRD](prd.md) for the product
contract and [the implementation plan](implementation-plan.md) for progress.

## Status

The executable supports strict JSON configuration, `--config`, `--project`,
`--help`, `--version`, and an asynchronous configured folder browser
(quit with `q` or Ctrl+C). Missing configuration opens editable first-run setup;
`sei setup` reconfigures ordered agents, confirms config replacement, and saves/exits.
Lowercase `x` permanently removes the selected destination skill, without
confirmation, trash, backup, or undo. Library add keys delete an existing same-named
destination completely before copying: local edits and destination-only files are
lost, even if content seems identical. This is not merging or synchronization.
Later failure can leave missing/partial output; retry add or remove it, with no
rollback. Predictable preflight rejection leaves the existing tree intact. During work,
navigation remains available; extra mutations are ignored, refresh is coalesced,
and quit waits for completion. Failures remain visible and listings refresh.
Malformed or unreadable configuration fails without starting setup or writing files.
There is no published installer or usable release. Unknown flags, commands, and
unexpected positional arguments return status `2`. After configuration validation,
interactive startup requires terminal stdin and stdout and otherwise returns `1`.
Help/version use stdout and need neither configuration nor a terminal; errors use stderr.
SIGINT, SIGTERM, and SIGHUP request ordinary exit through the same model path as
`q`/Ctrl+C; repeated signals never force exit. Terminal restoration precedes final
runtime diagnostics. HUP restoration is best effort on a disconnected terminal;
SIGKILL cannot restore anything. See [lifecycle policy and evidence](docs/terminal-lifecycle.md).

Use global options **before** the optional `setup` subcommand:

```sh
sei --config /tmp/sei.json --project ./example
sei --config /tmp/sei.json setup
sei --help
```

Setup starts with Claude Code, Codex, and OpenCode. Tab/Up/Down selects a field;
typing appends, Backspace deletes, and Ctrl+U clears. Ctrl+A adds a preset (including
Pi/Cursor) or custom agent; Ctrl+D removes the selected agent; Ctrl+K/J reorders it.
Enter validates and previews resolved paths/shortcuts, then Enter saves (existing
config requires `y` confirmation). Esc/Ctrl+C cancels before saving; during a save,
quit waits for completion. Setup never creates library or destination folders.

Without `--config`, configuration uses `os.UserConfigDir()`: Linux uses
`$XDG_CONFIG_HOME/sei/config.json` or `~/.config/sei/config.json`; macOS uses
`~/Library/Application Support/sei/config.json` and ignores XDG. Relative Linux
`XDG_CONFIG_HOME` is rejected. Relative config/project overrides use the launch
directory, never a Git root or the config file's parent. There is no config merging.
The [PRD configuration example](prd.md#setup-and-configuration) documents the exact
schema: a library and 1-9 ordered agents, each with a name and global/local paths.
Library/global paths must be absolute or begin with `~/`; only that home shorthand
is expanded. Local paths are project-relative and lexically contained. Raw path
components are retained; Task 8 checks physical containment, root overlap, and
existing filesystem identity aliases asynchronously. This milestone is still not
mutation-ready; see [the safety algorithm and limits](docs/filesystem-safety.md).

The browser lists immediate ordinary directories, including dot-directories,
without parsing `SKILL.md`. Loose files are ignored; symlink entries are shown as
blocked and are not followed. Listings use raw, case-sensitive Go string ordering;
display labels are escaped and truncated by terminal cell width without changing
raw selection names. The library appears left, with configured locals above
globals on the right. Missing destinations show `Not created` and remain absent;
inaccessible or invalid roots show errors independently of other panels.
Configured root aliases are permitted when root checks pass. Safety warnings are
independent of listings, with full reasons in help; unsafe roots remain inspectable.
Every mutation revalidates, including after missing-root creation. The active
config location and its aliases are protected even when the project changes.
The grid follows the focused agent, shows the visible slot range, and scrolls lists
to the selection. Shortcut rows stay separate from titles; `*` marks panel focus
and `>` marks selection. Paths are home/project-relative where possible, with full
paths in help. Below the approved **80x24** minimum, new mutations are disabled;
quit still works and active work finishes normally.

Keys: Up/Down clamp; `0` library; `1-9` local; `g` then `1-9` global.
Each panel remembers its raw-name selection. `r` refreshes listings, not config,
preserving the raw name if present, otherwise clamping the old index.
`g` stays visibly pending without a timeout; invalid continuations are consumed.
`Esc` cancels/closes help; `q`/Ctrl+C retain quit priority. Paste is ignored.
`?` shows full sanitized selected name/root path and mappings; Up/Down scroll
wrapped help, including arbitrarily long targets. Navigation retains errors.
Add slots are exactly `a b c d e f h i o` local and `A B C D E F H I O` global,
from library only. Lowercase `x` removes from destination panels only;
uppercase `X` and unconfigured slots do nothing.

Other agents may also load skills from these folders. sei shows configured folder
contents, not everything an agent discovers or has loaded.

## Project Decisions

- Repository and Go module: `github.com/primaprashant/sei`.
- MIT license, copyright 2026 Prashant Anand, confirmed by the owner.
- Approved version policy: initial published tag `v0.1.0`, then `v0.x`
  development releases; reserve `v1.0.0` for the first usable release.
- Commits, public pushes, credential changes, branch protection, tags, and
  releases require explicit authorization. Version policy is not permission to
  publish. Do not replace an existing Git remote or local agent configuration.

The existing `origin` points to the intended repository. At kickoff,
`gh repo view primaprashant/sei` confirmed that it is public, owned by
`primaprashant`, and accessible with `ADMIN` permission. No remote changes were
made during bootstrap.

## Development

Run commands from the repository root. Install Go **1.27.1** using the official
[Go downloads](https://go.dev/dl/) and verify its platform archive's published
SHA-256 before installing. Do not silently upgrade the toolchain or dependencies.

```sh
export GOTOOLCHAIN=go1.27.1
go version
go mod download
go mod verify
mkdir -p bin
go test -count=1 ./...
go build -o ./bin/sei .
./bin/sei --help
./bin/sei --version
```

The exact selected runtime versions are Bubble Tea `v2.0.9`, Lip Gloss `v2.0.6`,
and Bubbles `v2.2.1`, using `charm.land/.../v2` imports. All prescribed versions
were available at kickoff; no patch adjustment was needed. Module checksums are
in `go.sum`; review both module files when changing dependencies and keep Go's
checksum verification enabled.

The existing Charm dependency `github.com/charmbracelet/x/term v0.2.2` is also a
direct dependency for stdin/stdout TTY detection. Its `IsTerminal` implementation
was reviewed: on Linux/macOS it uses a read-only termios ioctl via `x/sys/unix`,
not a character-device heuristic. The existing `github.com/charmbracelet/x/ansi`
`v0.11.8` is now direct: its reviewed `StringWidth` and `Truncate` APIs measure
grapheme clusters in terminal cells; untrusted text is escaped before these calls.
Task 9 adds only owner-approved, test-only `github.com/creack/pty v1.1.24`.
The lifecycle disables Bubble Tea's default signal handlers; read-only scans are
not joined on exit. Ordinary quit waits for the active sequential mutation. If it
then fails, terminal cleanup precedes sanitized stderr and status `1`.

### Pinned Linter

Use the official **golangci-lint v2.13.2** release binary, not `go install`, a
floating package-manager version, or a tool dependency in this module. Review
[upstream installation guidance](https://golangci-lint.run/docs/welcome/install/local/)
and the [versioned release](https://github.com/golangci/golangci-lint/releases/tag/v2.13.2)
before executing downloaded tooling. Download and verify before extraction; do
not pipe a remote script directly into a shell.

This Linux amd64 example pins the archive digest checked against the release's
checksum manifest and GitHub asset metadata:

```sh
mkdir -p .bin
curl --fail --location --proto '=https' --tlsv1.2 \
  -o .bin/golangci-lint-2.13.2-linux-amd64.tar.gz \
  https://github.com/golangci/golangci-lint/releases/download/v2.13.2/golangci-lint-2.13.2-linux-amd64.tar.gz
printf '%s\n' '2277d43b98ec0054280f2ac26b53268bae97682444678a59a657dd565da021d6  .bin/golangci-lint-2.13.2-linux-amd64.tar.gz' | sha256sum --check -
# Continue only if the checksum check succeeds.
tar -xzf .bin/golangci-lint-2.13.2-linux-amd64.tar.gz -C .bin \
  --strip-components=1 golangci-lint-2.13.2-linux-amd64/golangci-lint
./.bin/golangci-lint version
```

For Linux arm64 or macOS, select the matching `linux-arm64`, `darwin-amd64`, or
`darwin-arm64` archive from that same release, verify its entry in
`golangci-lint-2.13.2-checksums.txt` before extraction (`shasum -a 256` on macOS),
and place the executable at `.bin/golangci-lint`. Never reuse the amd64 digest
for another archive. Checksums from the same publisher detect corruption, not a
compromised publisher. Pin/review any future installer script before executing it.

### Checks

```sh
export GOTOOLCHAIN=go1.27.1
go version
go mod tidy -diff
./.bin/golangci-lint config verify --config .golangci.yml
./.bin/golangci-lint fmt --config .golangci.yml --diff
./.bin/golangci-lint run --config .golangci.yml --timeout=5m ./...
go vet ./...
go test -count=1 ./...
mkdir -p bin
go build -o ./bin/sei .
```

Formatting checks must fail on differences without rewriting source. To apply
formatting intentionally, run `./.bin/golangci-lint fmt --config .golangci.yml`.
The ruleset includes tests and only `errcheck`, `govet`, `ineffassign`,
`staticcheck`, `unused`, and the `goimports` formatter. Optional editor
format/import organization on save should follow these rules and EditorConfig;
no particular editor or line-length limit is required.

On Linux amd64 with a C compiler, also run:

```sh
GOTOOLCHAIN=go1.27.1 CGO_ENABLED=1 go test -race -count=1 ./...
```

Tests and manual experiments must use disposable HOME/config/project/library/
destination directories. Set both `HOME` (including macOS's native
`$HOME/Library/Application Support` location) and Linux `XDG_CONFIG_HOME` to
disposable paths; never inspect real agent installations or execute skill
scripts. Tests isolate native HOME/XDG configuration paths and use disposable
config files. For an interactive browser smoke check with isolated data:

```sh
scratch=$(mktemp -d)
mkdir -p "$scratch/library/.hidden" "$scratch/library/example" "$scratch/global/installed"
printf '%s\n' '{"library":"~/library","agents":[{"name":"Example","global":"~/global","local":".local/skills"}]}' > "$scratch/config.json"
HOME="$scratch" XDG_CONFIG_HOME="$scratch" ./bin/sei --config "$scratch/config.json" --project "$scratch"
```

Browsing alone creates no destinations or skills. The local destination in this
example stays absent until an add key is used.
Focused browser checks: `go test -count=1 -run 'TestBrowse' .`.
Task 7 checks: `go test -count=1 -run 'Test(Navigation|KeySequence|Help|Refresh)' .`.
Standard/build/race and disposable 1/3/9-agent Linux PTY smoke passed (100x30,
xterm-256color, no-color); cross-terminal proof remains pending.

Task 8 checks: `go test -count=1 -run 'Test(RootSafety|ResolveRoots|SkillName)' .`.
Focused/standard/build/race checks pass on Linux amd64 with Go 1.27.1, including
unprivileged permission tests; the disposable filesystem detected case-sensitive names.
Tasks 8-12 passed all four native jobs in [CI 33994932478](https://github.com/primaprashant/sei/actions/runs/33994932478).

Task 9 checks: `go test -count=1 -run 'TestPTYLifecycle' .`. Linux PTY and full
race checks pass, including post-raw initialization failure, saved-termios
restoration without repeated renderer cleanup, and sanitized runtime diagnostics;
manual Terminal.app evidence remains pending. Existing CI
full-test jobs automatically include these tests; no workflow was added or changed.
PTY reads use existing `x/sys` Poll/Read with cancellation. Upstream input bursts
can leave a reader goroutine until process exit; no in-process reuse is promised.

Phase E adds rooted copy/removal/replacement, preflight and injected-failure tests,
umask subprocesses, and PTY add-three/remove-two/restart workflows. Linux standard
and race checks pass; all four native jobs pass in [CI 33999522026](https://github.com/primaprashant/sei/actions/runs/33999522026). The
scan-to-operation identity audit is Task 17; this is a prototype, not a release.

Generated tools, binaries, release output, coverage output, and `/.opencode/`
remain ignored. GoReleaser Community `v2.18.0` is reserved for the later release
tasks; Phase A does not install it or create a release configuration. Native
macOS/arm64 verification and CI are not established by local Linux checks.
