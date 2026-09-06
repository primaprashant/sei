# Task 21: Representative Prototype

## Status And Owner Input

Automated Linux evidence refreshed after `f0784fa`, 2026-09-06. Checkpoint review
fixed a P2 harness readiness gap; help now exposes listing state. Native CI passes at
`8eafad9`; owner feedback is recorded below. No final layout, minimum, budget, or
complete theme/SSH acceptance is implied by that historical evidence. Task 22
decisions and current verification are recorded below.

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
go test -count=1 -run '^TestRepresentative(Readiness|Fixture)$' .
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
| Artifact | 5,177,504 bytes; SHA-256 `7527850d53d23e13c8c41fae54f5c8d8bf94283cf292b0d1c26ab1d1c16ecdfe` |
| Embedded VCS | `f0784fa37a778e92cd331bbdfe56a89f5f62ff9c`, modified=true; includes help listing-state addition |

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

Before every mutation, fresh help output must identify the relevant panel, path,
expected selection, completed listing (including entry count), and checked roots.
Adds check both local destination and library; navigation waits for the library
scan too. Root safety alone can finish before scans, so it is not a readiness gate.
`TestRepresentativeReadiness` deterministically delivers safety before the scan and
rejects loading listings and wrong skill/operation/scope/path completion results.

Input timing starts at Down; mutation timing at `a`/`x`. The historical run below
used the previous `X` binding; its measurements are not a new-binding benchmark.
The harness opens help,
waits for its footer, and matches only that surface's output. If unmatched, it
renders browse then help again, without sleeps. Input ends on the exact new name
and ready listing; operations end on the full escaped `Result: <operation/skill/
scope/path>: complete` immediately before the help footer. Each operation in this
flow has a distinct target/result. Timing **includes help polling/render overhead**,
not prior readiness checks or subsequent help closure. ANSI is stripped and hard
wraps joined, not terminal-emulated; focus/error checks identify their specific
panel and listing state without requiring path bytes suppressed by differential
rendering. These samples supersede the old generic-`complete` protocol, whose
retained status could falsely accept an ignored action. Invalid harness trials
are excluded, not reported as application failures or performance regressions.

One recorded matrix, sequential with no concurrent standard/race check in that
measurement run, contains 36 launches (4 geometries x 1/3/9 agents x 3 profiles).
Only the nine three-agent, larger-geometry cases perform timed mutations: 27 adds,
18 removes, 18 selection/help inputs. Other cases check browse/help, last configured
local/global slot reachability, an actual unavailable-root error, and clean quit.
No replacement timing, large-file throughput, one/nine-agent mutation timing, or
80x24 mutation timing is claimed. Raw samples: [linux-timings.txt](prototype-captures/linux-timings.txt).

| Wall Latency (ms) | n | Min | Median* | p95* | Max |
| --- | ---: | ---: | ---: | ---: | ---: |
| Startup proxy | 36 | 51.837 | 57.140 | 68.274 | 69.001 |
| Selection + help render | 18 | 15.130 | 16.167 | 16.984 | 16.984 |
| Add + completion help polling | 27 | 15.323 | 49.227 | 49.967 | 50.009 |
| Remove + completion help polling | 18 | 15.141 | 15.657 | 16.949 | 16.949 |

*Harness reports upper-middle median and nearest-rank p95. Aggregates mix the
documented sizes/profiles; individual geometry samples are in the log. This small
warm-data sample is exploratory, not a release gate or a Mac performance prediction.

## Captures And Follow-Ups

These captures and timings predate the owner-approved removal-key change. Their
recorded `X` hints are historical: current builds remove with `x`; `X` is inactive.
Original output is preserved, not edited to imply a new capture run.

Three-agent production `View` text: [148x39](prototype-captures/148x39.txt),
[143x35](prototype-captures/143x35.txt), [80x24](prototype-captures/80x24.txt),
[150x40](prototype-captures/150x40.txt). Paths are substituted **before** layout,
so these snapshots intentionally do not demonstrate real long-path truncation.
The separate actual-binary PTY runs use real temporary paths. No pixel screenshots,
theme contrast judgment, or human terminal acceptance is being substituted here.

Actual PTY startup output is also retained, with ANSI quoted and temporary paths
sanitized: [148x39](prototype-captures/148x39-pty.txt),
[143x35](prototype-captures/143x35-pty.txt), [80x24](prototype-captures/80x24-pty.txt),
[150x40](prototype-captures/150x40-pty.txt). Captures and `linux-timings.txt` were
regenerated together in the isolated 36-case run; standard/race checks ran afterward.

- At 148x39 and 150x40 all 25 library names fit in the initial three-agent model view; at 143x35 it shows rows 1-23/25. These are observations, not approved minimums.
- At 80x24 only agents 1-2 initially appear, their header add hints are truncated, and the library shows 11/25 rows with several truncated names. Agent 3 is reachable by focus/help; offscreen discoverability is recorded for the deferred layout work.
- Nine agents are addressable through last-slot local/global focus and full-target help, but not simultaneously visible. This is reachability evidence, not proof that the provisional grid is usable.
- Real temporary paths truncate in panel cells; long result lines truncate in browse view and wrap in help. Preserve full-target inspection; result visibility/overflow improvements are deferred.
- The shell-versus-sei workflow comparison, cold/warm native Mac measurements, replacement/larger-tree measurements, and approved budgets belong to the owner review and Task 23. RAM-backed results cannot justify storage budgets.

## Owner Review

### Feedback Received (2026-09-06)

The owner reports that the workflow is mostly functional, but the layout needs work:

- Around 140 columns, three agents feel crowded; panel titles and trailing add shortcuts clip, including Claude Code Global.
- Repeated `Root relations checked` text is not useful in the main view; retain actionable problems, not routine internal status.
- Using `>` for both panel focus and row selection is confusing, especially with one skill; distinguish the two visually.
- Show home-relative global paths such as `~/.claude/skills` and project-relative local paths such as `.claude/skills`; preserve full paths in help and actual filesystem targets.
- The UI feels plain and colorless; improve visual hierarchy and color later without losing no-color readability.
- Exclude `.git` from skill listings was requested as a narrow exception, not a blanket exclusion of dot-directories or a change to nested copying.
- The owner approved lowercase `x` replacing uppercase `X` for removal; uppercase `X` is inactive. Permanent deletion and all mutation guards are unchanged.

The owner subsequently deferred all other feedback above until after the initial
release, prioritizing the remaining implementation work. No next task is started
by this decision. Final panel arrangement/minimum, terminal versions, per-Mac
results, and theme/SSH coverage remain open; safety and publication gates are not
waived. Reconcile the plan's existing polish dependencies before proceeding.

### Approved Layout (Task 22)

Owner reopened Phase H on 2026-09-06 and approved **80x24** minimum, with the
explicit revision **local above global**. This supersedes the UI deferral above;
`.git` listing changes remain deferred. PRD/vision updated; bindings unchanged.

- Library left; up to three destination columns, locals above corresponding globals.
- Agent windows follow focus, show the visible slot range, and retain all 1-9 mappings.
- Separate title/shortcut rows; `*` panel focus versus `>` selected row.
- Home-relative library/global paths and project-relative local paths; full raw-target display in scrollable help.
- Routine per-panel root-check success removed; one readiness status, actionable errors retained.
- Below 80x24: resize warning, no new mutations, quit available; resizing never cancels work.

Linux `Test(Layout|Resize)` covers 80x24, 143x35, 148x39, all agent counts/focus
targets, consecutive heights 24-65, full-target help and before/during/after
mutation resizing. Real gated copy/delete PTYs resize to 79x24 and back, reject
extra work, finish on quit, and restore terminal state. Full/race/lint/vet/build
and ten repeated affected PTY runs pass. PTY assertions account for retained
renderer text; model tests separately check complete frames. No new dependency.
Native Mac, VTE/Terminal.app/theme/SSH inspection and final human acceptance remain open.

### Measurements (Task 23)

[Approved protocol, current 42-trial Linux evidence, benchmarks, and owner shell
comparison](performance.md). Current timing uses reconstructed terminal cells,
all-panel readiness, and direct browse navigation timing; historical P2 timing and
captures above are not silently relabeled. Scoped Linux budgets approved; human
comparison remains open, with owner authorization to proceed to Task 24.

### Display Hardening (Task 24)

Six [reviewed golden frames](../testdata/views.golden) cover desktop/minimum,
Unicode/combining/control names, truncation, blocked/empty/absent/error states,
busy help/full results, pending quit, undersized fallback, and ninth-agent focus.
Golden text encodes Unicode/backslashes as Go escapes; terminal-cell bounds are
asserted before encoding, so these are regression fixtures, not screenshots.
Normal tests only compare; intentional regeneration uses
`SEI_TEST_UPDATE_VIEWS=1 go test -run '^TestViewSnapshots$' .` followed by diff review.

`TestDisplaySafety` checks C0/C1, OSC clipboard/hyperlink, bidi/format characters,
invalid UTF-8, long paths, and full escaped help fields. Raw-name workflows render
browse/help before real add/replace/remove, preserving selected/entry raw bytes,
the source snapshot, and an escaped-spelling decoy. Unsupported invalid-UTF-8
filesystems are capability-probed; pure display coverage is unconditional.

The quit hint no longer forces fixed gray colors: text inherits terminal colors.
Light/dark/no-color model profiles retain text focus/selection/scope/key cues, with
no font, animation, enhanced-keyboard or repeat-detection requirement. Linux
standard/full/race checks, Mac test cross-builds, the 36-case real-binary PTY
matrix, and a fresh 42-trial budget run pass after this change. VTE/Terminal.app
contrast, native execution, and SSH/human inspection are still open acceptance
gates, not inferred from profiles, snapshots, or cross-compilation.

### Reproduce The Review

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
two with lowercase `x`, quit/relaunch, inspect the longest name and all help pages, then try a fresh
fixture with a root replaced by an ordinary file to inspect the failure. Repeat
in light/dark/no-color and over Mac-to-Linux SSH, recording terminal/app versions,
TERM, actual geometry, path lengths, readability, target certainty, and mistakes.
Use the harness's one/nine-agent cases for automated reachability, and disposable
`sei setup` for human review of those arrangements.

- [x] Pinned upstream acquired as data; fixture preparation and offline supplements verified.
- [x] Linux focused/full/race tests, formatter/lint/config verification, vet, module hygiene, stripped build, and opt-in PTY matrix pass on Go 1.27.1.
- [ ] Native M5/M1 execution, actual terminal theme/SSH inspection, and owner feedback recorded.
- [ ] Owner approves layout/minimum, workflow usability, and subsequent measurement protocol. No acceptance is inferred from automation.
