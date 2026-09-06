# Task 21: Representative Prototype

## Status And Owner Input

Automated Linux evidence after `c94da92`, recorded 2026-09-06. No production or
Phase H layout changes, supported minimum, performance budget, or owner acceptance
is implied. Native Mac and human terminal/SSH review remain pending.

The owner initially approved a provisional synthetic fixture, then superseded
its source with [Addy Osmani's agent-skills](https://github.com/addyosmani/agent-skills).
The main fixture is **all 25 actual skills**, not synthetic replacements padded to
30. Owner geometries are **148x39 (M5 MacBook Air)** and **143x35 (M1 MacBook Air)**;
80x24 and 150x40 are additional exploratory sizes, not approved support boundaries.

## Provenance And Safety

| Item | Recorded Value |
| --- | --- |
| Upstream revision | `469d00f4e67ff4a21eb6e6e467a086c9a1f1deb8` |
| Archive | `https://codeload.github.com/addyosmani/agent-skills/tar.gz/469d00f4e67ff4a21eb6e6e467a086c9a1f1deb8` |
| Downloaded archive SHA-256 | `9f134b3c1c308b88a5f3acc37e0e33fcf25cf86a964644df1d2f24866ea0d192` |
| Managed data | 25 immediate ordinary skill directories, 30 regular files, 384,233 logical file bytes |
| License | MIT, Copyright (c) 2025 Addy Osmani; complete original `LICENSE` retained beside `skills/` |
| Executable data | `idea-refine/scripts/idea-refine.sh`, 342 bytes, executable bits preserved subject to umask; never executed |
| Nested reference | `constraint-driven-development/references/floor-guard.md` |
| Longest skill name | `observability-and-instrumentation`, 33 ASCII characters |

Only `skills/` and `LICENSE` are extracted into disposable paths. Archive links,
special entries under those paths, traversal, duplicates, and oversized files
are rejected. Repository configuration, hooks, and `.opencode` content are not
extracted, installed, loaded, or executed. The harness validates the archive digest
before using its bytes. Standard tests neither download nor require upstream data;
no upstream content or whole repository is vendored.

There are no Unicode names or dotfiles under upstream `skills/`. Separately,
`TestRepresentativeFixture` builds **supplemental test-only** archives containing
`.dot/nested/.data`, a CJK/combining-character skill name, and a 100-character
`long-` repeated name with inert script bytes. It verifies extraction, raw-name
scan, independent recursive copies, executable bits, source preservation, and
rejection of traversal/symlinks. These three supplemental skills are never mixed
into upstream totals or performance runs. A regular file substituted for an absent
global root is another explicitly test-only failure case, not upstream content.

## Reproduce

Run from the sei repository with Go 1.27.1. Download is an explicit opt-in action,
not part of `go test`. Verify the disposable parent before creating output:

```sh
export GOTOOLCHAIN=go1.27.1
ls /tmp/opencode
curl --fail --location --max-time 120 \
  -o /tmp/opencode/sei-task21-upstream.tar.gz \
  https://codeload.github.com/addyosmani/agent-skills/tar.gz/469d00f4e67ff4a21eb6e6e467a086c9a1f1deb8
go test -count=1 -run '^TestRepresentativeFixture$' .
ls bin docs/prototype-captures
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o ./bin/sei .
SEI_TEST_ARCHIVE=/tmp/opencode/sei-task21-upstream.tar.gz \
SEI_TEST_BINARY="$PWD/bin/sei" \
SEI_TEST_CAPTURES="$PWD/docs/prototype-captures" \
  go test -v -count=1 -run '^TestRepresentativePrototype$' .
```

`SEI_TEST_*` variables are understood only by `_test.go` code, never by the shipped
application. Omit `SEI_TEST_CAPTURES` to avoid rewriting generated snapshots.
Subtest filtering supports, for example,
`-run '^TestRepresentativePrototype/148x39/3/no-color$'`. Each case gets independent
temporary HOME, XDG config, project, library, and global/local targets. macOS's
native config directory is isolated by that HOME too.

## Observed Environment

| Item | Value |
| --- | --- |
| Machine | Hetzner KVM VM, reported AMD EPYC-Rome, 4 vCPUs, approximately 8 GB RAM |
| OS | Debian 13, Linux amd64 |
| Kernel | `6.12.90+deb13.1-cloud-amd64`, Debian `6.12.90-2 (2026-05-27)` |
| Fixture filesystem | `/tmp` tmpfs, approximately 3.8 GiB capacity; RAM-backed, **not SSD copy performance** |
| Binary filesystem | `/dev/sda1`, ext4; QEMU HARDDISK, 76.3 GiB virtual disk, nonrotational flag; physical backing unknown |
| Terminal | Local automated PTY, `TERM=xterm-256color`; no GUI emulator or SSH transport |
| Profiles | `COLORFGBG=15;0`, `COLORFGBG=0;15`, or `NO_COLOR=1`; profile inputs, not human contrast evidence |
| Toolchain | `go version go1.27.1 linux/amd64` |
| Build | `CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o ./bin/sei .`, GOAMD64=v1 |
| Artifact | 5,177,504 bytes; SHA-256 `ddd23e97e3b0df08e507935c7f680b1932f961701aa8c803e2d3c5bd9e6d7649` |
| Embedded VCS | `c94da92cbeb53ca488e59e593d36c5244361ccee`, modified=true (test/doc worktree); no production changes |

## Timing Protocol

The PTY child is the above stripped executable, **not `go run` or a test-process
wrapper**. Go testing drives the external process through the existing PTY reader.
No injected operation delays, arbitrary latency budgets, or sleep-based completion
checks are used. Measurements are monotonic wall latency to observed output,
**not photons**, CPU time, pure filesystem syscall time, or storage durability.

Startup begins immediately before PTY/process creation and ends after observing
the selected first upstream name, root-check text, and an absent-destination
marker. This is a first-populated-visible-frame proxy, not a barrier proving all
hidden panels have finished scanning. Extraction, snapshots, config writing, and
model capture preparation are excluded; they warm filesystem caches. All runs
are fresh processes with reset fixtures but **warm data**, with no cold-cache claim.

At 148x39, 143x35, and 150x40 with three agents, add the first three sorted actual
skills to local slot 1, remove the first two, then quit. The survivors and their
independent bytes are checked on disk after exit, and the entire upstream source
snapshot (identities, modes, bytes) must be unchanged. Restarting a second binary
is not timed here; existing restart regressions remain in the standard suite.

Input samples send Down followed immediately by `?`, ending when help emits the
new full selected name. Add/remove samples send `a?`/`X?`, ending on rendered
`complete`; closing help and root-ready output precede the next action. Thus these
samples include help-render overhead. This explicit fresh surface avoids treating
an unchanged differential-render suffix as missing output; hard-wrapped text is
joined for matching. It is not a terminal emulator. Early harness trials that
waited for repeated browse-status substrings timed out and are excluded as invalid
measurements, not application failures.

One recorded matrix, sequential with no concurrent standard/race check in that
measurement run, contains 36 launches (4 geometries x 1/3/9 agents x 3 profiles).
Only the nine three-agent, larger-geometry cases perform timed mutations: 27 adds,
18 removes, 18 selection/help inputs. Other cases check browse/help, last configured
local/global slot reachability, an actual unavailable-root error, and clean quit.
No replacement timing, large-file throughput, one/nine-agent mutation timing, or
80x24 mutation timing is claimed. Raw samples: [linux-timings.txt](prototype-captures/linux-timings.txt).

| Wall Latency (ms) | n | Min | Median* | p95* | Max |
| --- | ---: | ---: | ---: | ---: | ---: |
| Startup proxy | 36 | 51.908 | 56.671 | 63.767 | 67.966 |
| Selection + help render | 18 | 15.143 | 16.220 | 16.805 | 16.805 |
| Add + completion help render | 27 | 17.411 | 32.034 | 32.843 | 33.374 |
| Remove + completion help render | 18 | 14.553 | 16.466 | 17.541 | 17.541 |

*Harness reports upper-middle median and nearest-rank p95. Aggregates mix the
documented sizes/profiles; individual geometry samples are in the log. This small
warm-data sample is exploratory, not a release gate or a Mac performance prediction.

## Captures And Follow-Ups

Three-agent production `View` text: [148x39](prototype-captures/148x39.txt),
[143x35](prototype-captures/143x35.txt), [80x24](prototype-captures/80x24.txt),
[150x40](prototype-captures/150x40.txt). Paths are substituted **before** layout,
so these snapshots intentionally do not demonstrate real long-path truncation.
The separate actual-binary PTY runs use real temporary paths. No pixel screenshots,
theme contrast judgment, or human terminal acceptance is being substituted here.

Actual PTY startup output is also retained, with ANSI quoted and temporary paths
sanitized: [148x39](prototype-captures/148x39-pty.txt),
[143x35](prototype-captures/143x35-pty.txt), [80x24](prototype-captures/80x24-pty.txt),
[150x40](prototype-captures/150x40-pty.txt). These are separate capture reruns, not
the timing samples in `linux-timings.txt`.

- At 148x39 and 150x40 all 25 library names fit in the initial three-agent model view; at 143x35 it shows rows 1-23/25. These are observations, not approved minimums.
- At 80x24 only agents 1-2 initially appear, their header add hints are truncated, and the library shows 11/25 rows with several truncated names. Agent 3 is reachable by focus/help, but offscreen discoverability needs owner review in Task 22.
- Nine agents are addressable through last-slot local/global focus and full-target help, but not simultaneously visible. This is reachability evidence, not proof that the provisional grid is usable.
- Real temporary paths truncate in panel cells; long result lines truncate in browse view and wrap in help. Preserve full-target inspection and decide result visibility/overflow in Task 22 rather than fixing layout in this task.
- The shell-versus-sei workflow comparison, cold/warm native Mac measurements, replacement/larger-tree measurements, and approved budgets belong to the owner review and Task 23. RAM-backed results cannot justify storage budgets.

## Owner Review

Prepare a retained disposable fixture (the path must not already exist):

```sh
ls /tmp/opencode
SEI_TEST_ARCHIVE=/tmp/opencode/sei-task21-upstream.tar.gz \
SEI_TEST_PREPARE=/tmp/opencode/sei-owner-review \
  go test -v -count=1 -run '^TestRepresentativePrototype$' .
HOME=/tmp/opencode/sei-owner-review \
XDG_CONFIG_HOME=/tmp/opencode/sei-owner-review/.config \
  ./bin/sei --config /tmp/opencode/sei-owner-review/config.json \
  --project /tmp/opencode/sei-owner-review
```

Preparation was actually exercised at `/tmp/opencode/sei-task21-owner`; its config
uses Claude Code, Codex, and OpenCode labels with disposable global/local paths.
Automated matrix labels are deliberately short `Agent1` etc.; native review must
include the actual labels and longer paths. The prep command never runs skills or
starts agents. Keep `LICENSE` alongside the retained data when sharing it.

On each Mac, build natively and review at its stated geometry. Add three, remove
two, quit/relaunch, inspect the longest name and all help pages, then try a fresh
fixture with a root replaced by an ordinary file to inspect the failure. Repeat
in light/dark/no-color and over Mac-to-Linux SSH, recording terminal/app versions,
TERM, actual geometry, path lengths, readability, target certainty, and mistakes.
Use the harness's one/nine-agent cases for automated reachability, and disposable
`sei setup` for human review of those arrangements.

- [x] Pinned upstream acquired as data; fixture preparation and offline supplements verified.
- [x] Linux focused/full/race tests, formatter/lint/config verification, vet, module hygiene, stripped build, and opt-in PTY matrix pass on Go 1.27.1.
- [ ] Native M5/M1 execution, actual terminal theme/SSH inspection, and owner feedback recorded.
- [ ] Owner approves layout/minimum, workflow usability, and subsequent measurement protocol. No acceptance is inferred from automation.
