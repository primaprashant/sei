# sei

A terminal skill-folder manager for copying skills from a personal library to
configured agent destinations and removing installed copies. Not a skill
marketplace, agent launcher, or agent skill. See [the PRD](prd.md) for the product
contract and [the implementation plan](implementation-plan.md) for progress.

**Development status:** setup, browsing, copying/replacement, removal, and the
receipt-owned installer are implemented. **No public release or installer asset
exists yet. All sei download URLs below are pending examples, not working install
claims.** For use now, [build from source](#development) and try disposable data.
The owner has authorized `v0.1.0` as a normal release, to become stable/latest
after publication. Only tags with a SemVer prerelease suffix such as `-rc.1`
are GitHub prereleases; there is no mandatory `v1.0.0` release gate.

## Install

The future released binary needs no Go, agent tooling, or language runtime.
The installer needs POSIX shell/core utilities, curl, GNU/BSD tar, gzip, mktemp,
and either `sha256sum` or `shasum -a 256` (available on macOS). It detects the
host, verifies the selected archive, and defaults to `$HOME/.local/bin`. It never
uses sudo, edits shell profiles, or disables macOS security checks.

### One Command (Pending Publication)

This explicit-version example downloads the **complete** script before executing
it, and cleans its private temporary directory on exit. Do not use `curl | sh`:
a failed/truncated transfer can otherwise execute a dangerous partial script.

```sh
sh -c 'set -eu; d=$(mktemp -d); trap '\''rm -rf "$d"'\'' 0; trap '\''exit 1'\'' HUP INT TERM; curl -q --fail --silent --show-error --location --proto "=https" --proto-redir "=https" --tlsv1.2 --output "$d/install.sh" "https://github.com/primaprashant/sei/releases/download/v0.1.0/install.sh"; sh "$d/install.sh" --version v0.1.0'
```

This still trusts downloaded shell code. Prefer reviewing it first:

### Inspect Then Run (Pending Publication)

```sh
installer_dir=$(mktemp -d)
curl -q --fail --silent --show-error --location --proto '=https' \
  --proto-redir '=https' --tlsv1.2 --output "$installer_dir/install.sh" \
  https://github.com/primaprashant/sei/releases/download/v0.1.0/install.sh &&
# Stop if curl fails. Review the complete script before running the next command.
less "$installer_dir/install.sh" &&
sh "$installer_dir/install.sh" --version v0.1.0 --install-dir "$HOME/Tools/sei bin"
rm -r "$installer_dir"
```

Use the printed absolute `Run` command immediately, or execute the printed
`export PATH=...` in your current shell. For the custom directory above:

```sh
"$HOME/Tools/sei bin/sei" --version
export PATH="$HOME/Tools/sei bin":$PATH
sei --help
```

PATH is not changed by the installer; persist it yourself only if desired.
Directories containing `:` cannot be a single PATH component: use the absolute
invocation instead. Quote custom directories, including spaces or apostrophes.

`--version latest` (the default) resolves GitHub's latest **stable** release once,
then pins both downloads to that tag. It does not select GitHub prereleases.
The future stable script URL is
`https://github.com/primaprashant/sei/releases/latest/download/install.sh`;
it is intended to serve `v0.1.0` after normal publication and latest promotion.
Publication and live URL verification remain pending. For any
explicit target, use the script asset and `--version` from the **same tag**.

### Manual Archive (Pending Publication)

Choose your native platform; these are the four planned `v0.1.0` artifacts:

| OS / CPU | Archive |
| --- | --- |
| Linux x86_64 (amd64) | `sei_0.1.0_linux_amd64.tar.gz` |
| Linux aarch64 (arm64) | `sei_0.1.0_linux_arm64.tar.gz` |
| macOS Intel (amd64) | `sei_0.1.0_darwin_amd64.tar.gz` |
| macOS Apple Silicon (arm64) | `sei_0.1.0_darwin_arm64.tar.gz` |

The manifest is `sei_0.1.0_checksums.txt`. The following Linux amd64 example
uses a fresh directory, strictly selects exactly one valid checksum entry,
verifies bytes **before extraction**, and allows only the four ordinary root
members `sei`, `README.md`, `LICENSE`, `THIRD_PARTY_NOTICES`. Change `target`
to `linux_arm64`, `darwin_amd64`, or `darwin_arm64` as appropriate. Stop on any
failure; do not substitute another archive's checksum.

```sh
(
set -eu
export LC_ALL=C
unset TAR_OPTIONS GZIP
tag=v0.1.0
target=linux_amd64
asset="sei_${tag#v}_${target}.tar.gz"
base="https://github.com/primaprashant/sei/releases/download/$tag"
work=$(mktemp -d)
trap 'rm -rf "$work"' 0
trap 'exit 1' HUP INT TERM
curl -q --fail --silent --show-error --location --proto '=https' \
  --proto-redir '=https' --tlsv1.2 --output "$work/archive.tar.gz" "$base/$asset"
curl -q --fail --silent --show-error --location --proto '=https' \
  --proto-redir '=https' --tlsv1.2 --output "$work/checksums" "$base/sei_${tag#v}_checksums.txt"
expected=$(awk -v name="$asset" '
  $2 == name { n++; if (NF != 2 || length($1) != 64 || $1 ~ /[^0-9a-f]/) bad=1; sum=$1 }
  END { if (n != 1 || bad) exit 1; print sum }' "$work/checksums")
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum < "$work/archive.tar.gz")
else
  actual=$(shasum -a 256 < "$work/archive.tar.gz")
fi
test "${actual%% *}" = "$expected"
gzip -t "$work/archive.tar.gz"
tar --ignore-zeros -tzf "$work/archive.tar.gz" > "$work/names"
awk '
  $0 != "sei" && $0 != "README.md" && $0 != "LICENSE" && $0 != "THIRD_PARTY_NOTICES" { bad=1 }
  seen[$0]++ { bad=1 }
  END { exit bad || NR != 4 }' "$work/names"
tar --ignore-zeros -tvzf "$work/archive.tar.gz" > "$work/types"
awk 'substr($0,1,1) != "-" { bad=1 } END { exit bad || NR != 4 }' "$work/types"
tar -xOzf "$work/archive.tar.gz" sei > "$work/sei"
chmod 755 "$work/sei"
"$work/sei" --version
install_dir="$HOME/.local/bin"
mkdir -p "$install_dir"
# Fresh manual install only: do not overwrite any binary or receipt entry.
for path in "$install_dir/sei" "$install_dir/.sei-install-receipt"; do
  test ! -e "$path" && test ! -L "$path"
done
cp "$work/sei" "$install_dir/sei"
)
```

Run `"$HOME/.local/bin/sei" --version`; for current-shell PATH use
`export PATH="$HOME/.local/bin":$PATH`. Manual installation creates **no receipt**.
Checksums from the same release detect corruption, not a compromised publisher;
neither checksum checks nor executing `--version` sandbox downloaded code.
Signing, notarization and attestations are out of scope for this personal tool;
Mac download trust is not verified. Do not remove quarantine or bypass Gatekeeper
if macOS refuses launch.

### Upgrade And Uninstall

For an installer-owned binary, repeat the installer flow with the desired tag's
script, the same `--version` target, and the original `--install-dir`. Ownership
requires the existing executable's digest to match `.sei-install-receipt` beside
it. Candidate verification and receipt preparation precede binary replacement;
pre-commit failures preserve the old executable. A failed final binary rename
can be retried using the retained receipt. This is not crash recovery or hostile
concurrent-writer protection.

**Manual or modified installs are refused**, as are missing/mismatched receipts,
symlinks, and nonregular paths. Never forge/edit a receipt to bypass refusal.
For explicit manual replacement, quit sei, verify the new archive as above,
inspect the exact existing binary and receipt paths, then move those entries to
a separate backup directory you control. Rerun the fresh manual procedure (no
receipt), or rerun the installer to establish real ownership. Do not delete the
old copy until the new one works. A receipt without a binary after a failed fresh
install also requires this inspection/relocation, not an automatic upgrade.

To uninstall, quit sei and inspect the chosen install directory first. Remove
only these two entries, not the directory or anything in your skill folders:

```sh
install_dir="$HOME/.local/bin" # Use your actual custom directory if different.
ls -ld "$install_dir/sei" "$install_dir/.sei-install-receipt"
# A manual install has no receipt. After confirming these are your install files:
rm -f "$install_dir/sei" "$install_dir/.sei-install-receipt"
```

`rm -f` does not recursively delete directories. **Copied skills, the source
library, and configuration remain unchanged.** Remove any PATH profile entry
you added yourself only if you no longer need it.

## First Run

1. Prepare a library of skill folders, separate from all destination roots. sei does not download skills or create the library.
2. Run `sei` in the intended project directory, or use `sei --project "$HOME/work/example"` with an existing directory. The launch directory is the default project, never an inferred Git root.
3. With no config, setup asks for the library and starts with Claude Code, Codex, and OpenCode. Review paths and order, preview, and save to enter the browser.
4. From the library, select a folder with Up/Down and press `a` to copy to the first agent's local destination. Repeat for three folders without switching panels. Press `1`, select a copied folder and press `x` to remove it; repeat for a second folder, then `q` to quit. **These are real permanent filesystem changes.** Try the disposable example under [Checks](#checks) first.

### Setup And Config

Use global options **before** `setup`:

```sh
sei setup
sei --config "$HOME/sei-test.json" setup
sei --config "$HOME/sei-test.json" --project "$HOME/work/example"
sei --help
sei --version
```

Explicit setup prepopulates valid config, saves and exits. A malformed/unreadable
existing config fails rather than being overwritten or silently repaired.
Tab/Up/Down selects a field; type to append, Backspace deletes, Ctrl+U clears.
Ctrl+A opens the preset/custom agent menu; Ctrl+D removes the selected agent;
Ctrl+K/J moves it up/down. Enter validates/previews resolved paths and shortcuts;
`e` edits again, Enter saves, and existing config requires `y` confirmation
(`n` goes back). Esc/Ctrl+C cancels before saving; during a save, quit waits.
Recognized paste is ignored, including in setup. Setup creates only config
parents/files, never library or destination folders.

| Editable Preset | Global | Project-Local |
| --- | --- | --- |
| Claude Code | `~/.claude/skills` | `.claude/skills` |
| Codex | `~/.agents/skills` | `.agents/skills` |
| OpenCode | `~/.config/opencode/skills` | `.opencode/skills` |
| Pi | `~/.pi/agent/skills` | `.pi/skills` |
| Cursor | `~/.cursor/skills` | `.cursor/skills` |

Select 1-9 ordered agents, including custom names/paths. Presets are not discovery;
see [upstream references and strict JSON schema](prd.md#setup-and-configuration).

| Platform | Default Config (`os.UserConfigDir()`) |
| --- | --- |
| Linux | `$XDG_CONFIG_HOME/sei/config.json`, otherwise `~/.config/sei/config.json` |
| macOS | `~/Library/Application Support/sei/config.json`; ignores XDG |

Relative Linux `XDG_CONFIG_HOME` is rejected. `--config` and `--project` overrides
resolve relative to launch cwd, not the config's parent. There is no config merging.
Library/global paths must be absolute or start with `~/`; no other shell expansion
is performed. Local paths must stay inside the project, both lexically and after
filesystem resolution. Roots cannot overlap the library or each other; config
and its aliases are protected. Configured root symlinks are allowed only when
safety checks pass. Restart after editing JSON; `r` does not reload it.

### Keys

| Input | Action |
| --- | --- |
| Up / Down | Move in the focused list, stopping at either end; scroll expanded help. |
| `0` | Focus library. |
| `1` ... `9` | Focus configured local slot 1 ... 9. |
| `g`, then `1` ... `9` | Focus configured global slot 1 ... 9 (two ordinary key presses). |
| `x` | Permanently remove selected destination folder, never from the library. `X` does nothing. |
| `r` | Refresh listings, not config; preserve raw-name selection if present, otherwise clamp. |
| `?` | Toggle help with full sanitized selected name, root path, errors, and mappings. |
| Esc | Close help or cancel pending `g`; otherwise no action. |
| `q` / Ctrl+C | Quit; wait for active mutation to finish first. |

All **18 add mappings**, from library focus only; config order determines slots:

| Slot | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Local Add | `a` | `b` | `c` | `d` | `e` | `f` | `h` | `i` | `o` |
| Global Add | `A` | `B` | `C` | `D` | `E` | `F` | `H` | `I` | `O` |

Unconfigured slots do nothing. Pending `g` is visible, has no timeout, and consumes
invalid continuations instead of replaying them as actions; Esc cancels and quit
keeps priority. No enhanced keyboard protocol, special font, or reliable repeat
detection is required. Paste is not action input. Each panel remembers selection
only for this run; add retains library focus, delete selects the following row
(or preceding last row). Navigation does not clear errors.

The library is left, locals above globals on the right. The agent window follows
focus and labels its visible slot range; not all nine agents need fit at once.
`*` marks focus and `>` selection. Compact/escaped labels do not alter raw names;
use help for full resolved targets. At less than **80x24**, new mutations are
disabled; quit still works and active work finishes normally.

## Safety And Scope

**Add is delete-then-copy, not merging or synchronization.** It removes an existing
same-named destination completely before copying, losing local edits and
destination-only files even if content looks identical. Lowercase `x` deletes
immediately and permanently, including folders absent from the library. Neither
action asks for confirmation, checks Git, uses trash, backs up, or offers undo.
Predictable preflight rejection preserves the old tree; later read/write/removal
failure can leave missing or partial output. There is **no rollback**. Inspect the
reported target and refreshed listing, then retry add or remove the remainder.

The library stays read-only. Immediate ordinary directories (including dotfolders)
are listed without parsing `SKILL.md`; loose files are ignored, symlink entries
are blocked. Links anywhere inside a skill and special files block mutation.
Missing destinations show `Not created` until first add; inaccessible roots show
errors, not empty success. Copies include nested/hidden files and preserve bytes
and executable bits subject to umask, not ownership/timestamps/ACLs/xattrs.
See [filesystem safety and limits](docs/filesystem-safety.md).

Only one mutation runs at a time. Navigation stays responsive; extra mutations
are rejected (not queued), refresh is coalesced, and help blocks mutations.
Normal quit and SIGINT/SIGTERM/SIGHUP wait for active work; repeated signals do
not force exit. Forced termination can leave partial work; no multi-process lock,
consistent source snapshot, crash recovery, or hostile-writer sandbox is promised.
Terminal restoration precedes final errors; HUP restoration is best effort and
SIGKILL cannot restore anything. [Lifecycle details](docs/terminal-lifecycle.md).

Setup/TUI require terminal stdin and stdout. Help/version need neither config nor
a terminal and write stdout; errors use stderr. Exit codes: `0` normal quit/cancel,
`1` startup/config/save/runtime failure or active work failing after pending quit,
`2` invalid CLI syntax. A recoverable in-TUI error then ordinary later quit is `0`.

Normal setup/TUI operation has **no network access, telemetry, or automatic update
checks**. The installer separately downloads releases. sei never executes skill
scripts or launches/manages agent sessions, and does not inspect loaded skills.
No focus/selection/project/session state is persisted; config and filesystem
changes persist. Folder labels do not provide agent isolation:

> Other agents may also load skills from these folders. sei shows configured folder contents, not everything an agent discovers or has loaded.

## Support And Measurements

This is a small personal tool. [Debian 13.6 amd64 verification](docs/linux-verification.md)
is sufficient owner-approved Linux acceptance, not an all-distro or kernel-floor
promise. Existing native CI covers Linux/macOS amd64/arm64, including archive
help/version/PTY and installer tests at the recorded revisions.

The [owner-reported Mac source suite](docs/mac-verification.md) passed on macOS
15.7.2/M1; its source SHA is unknown and the personal exact-archive test did not
run. Go requires macOS 13+, but that minimum and Mac download trust are not tested
claims. Extra floor hosts, personal Mac artifact/trust checks and human acceptance
are [waived, not passed](prd.md#owner-scope-override). Windows, Homebrew and
self-update are not provided. Public installation remains pending publication.

The approved minimum is 80x24; 143x35 and 148x39 are the owner-selected larger
geometries. Recorded **Linux amd64 warm-data local PTY** measurements used 25
skills, 30 files, 384,233 bytes on tmpfs, 20 warm trials per size. At 143x35 / 148x39,
startup p95 was **62.622 / 64.172 ms**, navigation/render **17.032 / 16.682 ms**,
add confirmation **33.447 / 33.020 ms**, replacement **33.074 / 33.034 ms**, and
removal **17.358 / 16.715 ms**. The measured stripped binary was **5,181,600 bytes**.
These are historical measurements, not current artifact sizes or Mac/SSH/cold-cache
promises; operation timings include help/render confirmation, not just filesystem
work. Former p95/binary budgets and timed human comparison are no longer release
gates. [Protocol and raw data](docs/performance.md) are optional historical references.

## Reporting Issues

Include `sei --version`, install method/tag and archive name; OS/version/kernel
and native CPU (mention Rosetta); filesystem/case behavior; terminal/version,
`TERM`, dimensions, theme/no-color, and local versus SSH. Provide exact command
and key sequence, expected/actual result, full error and exit status, and whether
an operation was active at quit. For install failures include the command,
download stage and receipt presence, but do not edit the receipt to force a retry.
Prefer a minimal disposable fixture and redacted config/paths; do not post secrets
or private skill content. Distinguish a missing destination from an inaccessible
one, and report any partial output rather than cleaning away evidence first.

## License

MIT, copyright 2026 Prashant Anand. See [LICENSE](LICENSE) and
[THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES) for bundled dependency/runtime notices.
Publication, tags, pushes, credentials, and branch-protection changes require
explicit owner authorization; the version policy is not permission to publish.

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
mkdir -p "$scratch/skill-library/.hidden" "$scratch/skill-library/example" "$scratch/global/installed"
printf '%s\n' '{"library":"~/skill-library","agents":[{"name":"Example","global":"~/global","local":".local/skills"}]}' > "$scratch/config.json"
HOME="$scratch" XDG_CONFIG_HOME="$scratch" ./bin/sei --config "$scratch/config.json" --project "$scratch"
```

Browsing alone creates no destinations or skills. The local destination in this
example stays absent until an add key is used.
Focused checks:

```sh
go test -count=1 -run 'TestBrowse' .
go test -count=1 -run 'Test(Navigation|KeySequence|Help|Refresh)' .
go test -count=1 -run 'Test(RootSafety|ResolveRoots|SkillName)' .
go test -count=1 -run 'TestPTYLifecycle' .
```

Generated tools, binaries, release output, coverage output, and `/.opencode/`
remain ignored. Use `~/skill-library` in Mac fixtures, not `~/library`, which can
alias native `~/Library` on case-insensitive filesystems. Remove only the recorded
disposable scratch directory when finished.

Installer checks: `sh -n scripts/install.sh`, `./.bin/shellcheck -s sh scripts/install.sh`,
and `go test -count=1 -run '^TestInstaller' -v .`. Use verified ShellCheck **v0.11.0**
and GoReleaser Community **v2.18.0** binaries with the exact installation/provenance
commands in [release reproduction](docs/release.md#reproduction) and
[ShellCheck provenance](docs/release.md#shellcheck-provenance), not floating tools.
See the [current test matrix](docs/test-matrix.md), [performance reproduction](docs/performance.md#reproduce),
and optional historical floor/trust/performance runbooks. Local Linux checks do not prove Mac/arm64
native behavior, OS floors, or end-user download trust.
