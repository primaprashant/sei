# Task 23: Performance Evidence

Optional historical measurements/runbook. The [owner scope override](../prd.md#owner-scope-override)
WAIVES mandatory budgets, benchmarking and timed human comparison; results below
retain their original measured scope, not current release-gate status.

## Protocol

Owner approved this protocol on 2026-09-06; shell baseline is **typed `cp`/`rm`
with ordinary tab completion**, not a script or pretyped command benchmark.

- Actual stripped binary, three real agent labels (Claude Code, Codex, OpenCode), 143x35 and 148x39, local PTY, `TERM=xterm-256color`, `NO_COLOR=1`.
- Per size: one first-observed trial, then 20 warm trials. Each uses a fresh process and reset disposable HOME/config/project/library/destinations.
- Pinned upstream fixture: 25 skills, 30 regular files, 384,233 bytes; [provenance and license](prototype.md#provenance-and-safety). Timed adds use the first three sorted skills; replacement uses the first, with a destination-only file whose deletion is verified; removals use the first two. Source snapshots and the final survivor's independent bytes are checked.
- Startup: immediately before PTY/process creation through all scans/root checks ready and the first selection visible. Fixture extraction, config creation, snapshots, and emulator allocation are excluded.
- Navigation: Down input through the exact updated browse selection in reconstructed terminal cells. No help polling or filesystem completion is included.
- Add/replacement/removal: action key through the exact captured operation/skill/scope/path success in help. Opening help and terminal decoding are included, so this is **completion-confirmation wall latency**, not pure worker/syscall time. Readiness is checked before every action; replacement differs from the previous result to prevent stale-success acceptance.
- Report min, upper-middle median, nearest-rank p95, and max by size and first/warm group. Never combine native Mac/SSH samples with local Linux. No GUI/photons, cold-cache, SSD throughput, crash-durability, or human workflow claim follows from PTY timing.
- Pilot runs exposed retained renderer characters and hard-wrap trailing-space loss in the harness. Both have regressions; pilot/invalid runs are excluded, and the complete final sample set was rerun without concurrent benchmarks/race/tests. A failed run produces no aggregate summary.

All data is warm: preparation reads/writes it before timing. "First-observed"
means trial 00 of this recorded series, **not a cold boot/cache**. Cold-cache and
native Mac/SSH measurements remain separate pending evidence.

## Test Tool Review

Owner approved test-only `github.com/charmbracelet/x/vt`
`v0.0.0-20260906004030-3986e9119cf9` (MIT, Copyright 2023 Charmbracelet).
Go requirement 1.24.2 is compatible with 1.27.1; existing pinned Charm modules
remain unchanged. Added transitive `x/exp/ordered v0.1.0`; module checksums are
pinned in `go.sum`. The emulator parses retained cells, cursor edits, line
insertion, alternate screens, and split CSI/UTF-8 chunks. Query responses are
drained, not sent to the application, so measurements do not enable new keyboard
protocols. Its input reader is joined before closing mutable emulator state.
`TestPerformanceScreen` covers retained characters, stale-screen rejection, and
spaces at hard-wrap boundaries. A measured trial also passed under the race
detector (excluded from timing evidence). `go version -m bin/sei` confirms the
emulator is **not linked into the application**. No runtime dependency or public
measurement flag was added.

## Recorded Linux Run

2026-09-06, Debian 13.6, Linux amd64 `6.12.90+deb13.1-cloud-amd64`, AMD EPYC-Rome
KVM VM, 4 vCPUs, approximately 8 GB RAM. Fixture/benchmark data: `/tmp` tmpfs;
binary: `/dev/sda1` ext4. Local automated PTY only, no GUI terminal or SSH.

Build: Go 1.27.1, `CGO_ENABLED=0`, `GOAMD64=v1`, `-trimpath -ldflags='-s -w'`.
VCS `cc63e302ce0dbfdfe63e71f4c4455ba368e30fad`, modified=true (Task 23 test/module
changes, no production change). Binary **5,181,600 bytes**; SHA-256
`412169acca56d500b93da8f831db72036222c05aba00e87d28e0c7b5cd9a280d`.

All 42 trials passed. [Raw trials and distributions](performance-timings.txt).
Warm results, milliseconds:

| Size | Metric | n | Min | Median | p95 | Max |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| 143x35 | Startup | 20 | 53.873 | 56.993 | 62.622 | 62.791 |
| 143x35 | Navigation/render | 40 | 14.699 | 16.124 | 17.032 | 17.109 |
| 143x35 | Add confirmation | 60 | 16.386 | 32.680 | 33.447 | 34.436 |
| 143x35 | Replacement confirmation | 20 | 31.485 | 32.401 | 33.074 | 49.458 |
| 143x35 | Removal confirmation | 40 | 15.129 | 16.086 | 17.358 | 17.699 |
| 148x39 | Startup | 20 | 55.788 | 59.588 | 64.172 | 65.479 |
| 148x39 | Navigation/render | 40 | 12.285 | 15.899 | 16.682 | 16.828 |
| 148x39 | Add confirmation | 60 | 18.767 | 32.286 | 33.020 | 33.830 |
| 148x39 | Replacement confirmation | 20 | 30.671 | 32.433 | 33.034 | 34.358 |
| 148x39 | Removal confirmation | 40 | 15.084 | 15.817 | 16.715 | 16.931 |

First-observed startups: 62.516 ms (143x35), 54.457 ms (148x39); their action
samples/distributions are retained separately in the raw log. These small samples
do not predict rare tail latency or other filesystems/skill sizes.

Five benchmark repetitions also passed: [raw results](performance-benchmarks.txt).
Synthetic render fixture: 25 long-named empty folders per panel, 143x35, 1/3/9
agents, pre-scanned outside timing. Synthetic operation fixture: one skill,
three non-executable files (including a dotfile/nested file), 6,272 bytes; one
agent. Setup/reset is untimed; production root checks/delete/copy are timed.
Median run averages: View 0.312/0.556/0.543 ms for 1/3/9 agents; fresh add
13.177 ms, replacement 18.632 ms, remove 3.143 ms. These are microbenchmarks,
not startup or workflow proof. No optimization or safety-check reduction is
justified by these results.

## Budget Decision

Owner-approved **local Linux amd64 warm-fixture** regression gates (2026-09-06):
startup p95 <=100 ms, navigation/render p95 <=25 ms, add confirmation p95 <=50 ms,
replacement confirmation p95 <=60 ms, removal confirmation p95 <=25 ms;
stripped Linux amd64 binary <=6 MiB. These leave explicit headroom over the
measured distributions; they are not universal throughput/support promises.
Native Mac/SSH/cold and larger-tree limits require their own evidence before
broader release claims. The owner-run workflow comparison remains a release gate
even if these quantitative checks pass.

The opt-in test enforces these limits on Linux amd64; other platforms record
evidence without inheriting unapproved limits. Budget-boundary regressions are
in `TestPerformanceBudget`. Owner authorized proceeding to Task 24 with the human
comparison explicitly open; this is not Task 23 or Checkpoint H acceptance.
After enforcement was added, a fresh 42-trial run passed all budgets. The existing
36-case prototype matrix, full Linux tests, lint/format/module checks also pass;
no new production behavior was needed for Task 23.

## Reproduce

Use the [verified upstream download](prototype.md#reproduce), then run sequentially
with no concurrent load. Standard tests are offline; process measurements require
explicit local archive/binary paths. `SEI_TEST_*` variables are test-only.

```sh
export GOTOOLCHAIN=go1.27.1
ls bin docs
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o ./bin/sei .
go test -count=1 -run '^Test(PerformanceScreen|RepresentativeReadiness)$' .
SEI_TEST_ARCHIVE=/tmp/opencode/sei-task21-upstream.tar.gz \
SEI_TEST_BINARY="$PWD/bin/sei" \
  go test -v -failfast -count=1 -run '^TestPerformanceRelease$' .
go test -run='^$' -bench=. -benchmem -count=5 ./...
```

## Owner Workflow Comparison

Pending, not replaced by automated `cp`/`rm` timing. Prepare separate fresh
fixtures for sei and shell using `SEI_TEST_PREPARE` and the existing
`TestRepresentativePrototype` preparation flow. Use a new absolute disposable
path for each repetition; never reset a real agent installation or reuse mutated
fixtures. Both copies must have the same verified upstream bytes and missing
destinations before starting. Keep native HOME/config isolated when launching sei.

Owner-approved human protocol: one untimed practice per method, then five paired trials,
alternating which method goes first. Start timing at the ready sei screen or shell
prompt in the disposable project; stop at sei quit/shell completion, including
thinking/typing/repair time. Startup is measured separately above. Add
`api-and-interface-design`, `browser-testing-with-devtools`, and
`ci-cd-and-automation` to local1, remove the first two, and finish. Verify the last
skill survives and the library is unchanged after every trial. In sei this is
`a Down a Down a 1 x x q`: **9 logical keys, one panel switch** after readiness,
excluding inspection or mistakes. This is a planned minimum, not an observed score.

From the shell fixture's directory, use the owner's normal typing/completion for
`mkdir local1`, `cp -R skills/<name> local1/` for each of the three names, then
`rm -r local1/<name>` for the first two. Do not pretype/paste scripts to replace
the human baseline; only these disposable paths are in scope.

Record each trial: machine/OS/filesystem, terminal/version/TERM/size/theme,
local versus SSH, method/order, elapsed seconds, physical/logical keys (including
Tab/Enter/arrows), panel or directory switches, mistakes/repairs, readability,
and target certainty. Report all trials and distributions, not only the fastest.
Owner must approve the workflow improvement and any resulting follow-up fixes.
