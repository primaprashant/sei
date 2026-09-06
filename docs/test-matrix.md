# Destructive And Failure Test Matrix

## Task 29 Trust Gate

2026-09-06 after `8ad23fe`: [trust runbook](release.md#task-29-trust-policy)
records approved intended attestations, future-only permission proposals and
integrity-verified local ShellCheck `0.11.0`. No credentials/write grants added.
Mac browser/extractor and eventual installer download paths, quarantine/first
launch on both native architectures, signing decision and any required signed
artifact retests remain **untested release blockers**. Checksums, CI extraction
and missing quarantine are not Gatekeeper passes. Task 30 local/mock Go installer
tests may proceed by owner approval; Task 28 floors/support approval remain open.

## Task 28 Archive Gate

`TestReleaseArchives` checks all four exact producer archives, hashes, embedded
commit/architecture/CPU baseline and dependencies; only the native target runs
exact help/version and `/PTY` against its extracted executable. The reused
`runSetupPTY` harness verifies raw entry, browser frame, q, zero status/empty
stderr, termios and ordered cursor/alternate-screen restoration. It does not
rebuild the application. Go/modules are harness-only; the application uses
isolated HOME/config/project and empty PATH. Linux offline/four-file no-runtime
proof remains separate from empty-PATH tests.

CI builds once on Linux and transfers the same four tarballs/checksum manifest
by producer artifact ID to the four existing native jobs. This workflow has not
run remotely for Task 28. Local Linux evidence, action/tool review, exact-host
capture commands and the floor-host runbook are in
[release evidence](release.md#task-28-exact-native-archives).

| Required Floor | amd64 | arm64 |
| --- | --- | --- |
| Ubuntu 22.04 with actual 5.15.x host kernel | Untested, release blocker | Untested, release blocker |
| macOS 13, native execution | Untested, release blocker | Untested, release blocker |

Newer CI hosts and local Debian are not floor evidence. Owner support-table
approval and macOS download/quarantine/Gatekeeper remain release blockers, with
no support claim or security bypass. Task 34-36 full final-artifact feature/PTY
verification is still required; this early test intentionally exercises no
copy/remove flow. No push, tag or remote run was initiated.

## Phase I Acceptance

2026-09-06: [CI 34014725152](https://github.com/primaprashant/sei/actions/runs/34014725152)
at `1566603` passes all four native and both fuzz jobs, including Linux race/coverage.
Owner reports native Mac tests, manual SSH first-run/primary-flow/restart/cancel,
and Step 4 quit checks worked as expected. Exact personal Mac/terminal versions,
dimensions and transcripts were not supplied; this is owner-reported acceptance.

Reported `TestAddSkill/raw_root1`, `raw_root2`, `raw_nested1`, `raw_nested2`
skips probe invalid UTF-8 filenames unsupported by the Mac filesystem.
`distinct_case_names` skips when case variants alias. These are expected
capability differences, covered on case-sensitive Linux, not disabled safety
assertions. Native Mac case-collision tests cover the opposite behavior.

Owner waived the separate macOS offline run because no isolated Mac environment
was available. Linux offline execution remains verified; macOS offline execution
is **not tested**, not a pass. Phase I is accepted with this scoped exception;
earlier historical open-gate notes below are superseded only to that extent.
Phase H performance/theme acceptance and later release/floor/trust gates remain
separate. No application-network or security policy was relaxed.

Checkpoint I at `049000d`: Linux standard/full PTY/race, both 30s fuzz targets
(332,274 config / 792,541 path executions), coverage (88.6%), and four rebuilt
snapshot archive checks pass. Native/SSH gates below remain open; no push.

Task 25 audit, 2026-09-06. References below are existing executable tests, not proposed names. Scope is the PRD's setup, keyboard/display, filesystem and testing/CI contracts; installer/release requirements remain Tasks 27-35. A mapped test is not evidence that every platform or branch passed.

## Isolation And Fuzzing

Filesystem tests use disposable trees and snapshot assertions; `isolateConfigHome` isolates HOME (including macOS's native config location) and XDG. PTY children receive explicit disposable environments. No test runs skill scripts or needs real agent installations. Umask changes occur only in subprocesses.

`fuzz_test.go` contains two non-destructive targets. `FuzzParseConfig` checks input immutability, determinism, zero config on error, required fields/count/unique names, no runtime-path population, value/order-preserving JSON and whitespace round trips, and rejection of trailing values, escaped duplicates and non-schema keys. Seeds include malformed/null JSON, root/nested duplicates, wrong case, 0/1/9/10 agents, duplicate names, traversal, controls, invalid UTF-8 and surrogate escapes. JSON decoding's replacement of malformed Unicode is not incorrectly treated as raw-byte preservation.

`FuzzPathName` checks exact single-component name acceptance (including legitimate raw bytes), direct-child containment, an independent local-path component walk, sibling-prefix exclusion, and raw traversal-preserving root expansion without shell evaluation. Seeds include absolute/relative/tilde paths, missing/link parent traversal, separators, NUL, controls, Unicode, invalid bytes, case variants and long names. Only pure lexical APIs receive generated paths: no load/save, stat/open, physical resolver or mutation calls. The base and HOME are disposable; arbitrary absolute/tilde paths are never accessed. This proves lexical properties, not physical symlink safety; the real-tree regressions below cover that separately.

Normal `go test` runs the seeds. CI has separate Linux amd64 matrix jobs, one anchored target in package `.` each, with `-fuzztime=30s -parallel=2 -timeout=2m` and a 10-minute job limit. Existing four native checks and Linux race remain. Coverage runs on Linux amd64, prints per-function results to logs and the job summary, and imposes no global percentage threshold. Actions/tool integrity and read-only permissions retain the existing policy.

Go writes minimized failures under `testdata/fuzz/<target>/<hash>` and prints a replay command; CI prints those inputs on failure. Preserve a discovered failure there (or as a named table regression), rerun its replay command, then rerun that target and the suite. Interesting cache entries are not failures. No failing input was found in this audit, so no fabricated failure corpus was added.

## Regression Mapping

| Requirement | Actual Tests And Owning Files | Assertions / Limits |
| --- | --- | --- |
| Strict config, duplicate/escaped keys, exact schema, types/null, EOF, 1-9 ordered unique agents | `TestParseConfig`, `TestConfigPaths`, `TestConfigPathsProject` in `config_test.go`; `FuzzParseConfig`, `FuzzPathName` | Rejection and ordered decoding; raw path semantics, home expansion, local containment. |
| Missing versus invalid/unreadable config; syntax/non-TTY/help/version behavior | `TestCLIConfig`, `TestCLIHelpWithoutConfig`, `TestCLIProjectAndOverrides`, `TestCLI`, `TestCLIOutputFailure` in `cli_test.go`; `TestPTYLifecycle` in `process_test.go` | Invalid existing config is not replaced or treated as setup; statuses/output streams and non-TTY rejection. |
| Setup cancel/replacement consent; no skill-directory creation | `TestSetupCancelNoWrites` in `setup_test.go`; `TestExplicitSetup` in `setup_agents_test.go`; `TestSaveConfig` in `config_save_test.go`; `TestPTYFirstRunSetup` in `setup_process_test.go` | Cancellation/refusal preserves bytes and absent destinations; built-binary explicit edit/preview Esc and confirmation Ctrl+C; confirmed persistence and restart. |
| Config pre-commit failure preserves old bytes and skills | `TestConfigSaveFailure` in `config_save_failure_test.go` | New/replace validation, invalid UTF-8, create, write, short write, sync, close, rename faults; snapshots, closed handles, temp cleanup. Typed schema cannot produce a marshal error. |
| Config placement, links, observed save changes, stderr/status | `TestConfigSaveSafety` in `config_save_failure_test.go`; `TestMutationConfigProtection` in `mutation_config_test.go`; `TestConfigSaveFailurePTY` in `setup_process_test.go` | Old/new roots and config protected from skill mutation; retargeted parent/root/temp/config rejected; failure restores terminal and exits 1. |
| Root identity/nesting/overlap, local physical escape, missing/dangling/non-directory/inaccessible ancestors | `TestResolveRoots`, `TestRootSafety`, `TestRootSafetyPermissions`, `TestRootSafetyCaseIdentity`, `TestRootSafetyAliasesAndScopes`, `TestRootSafetyRevalidation` in `roots_test.go` | Component and physical identities, configured-root links, raw `link/..`, absence distinct from unprovable safety; actual case capability logged. |
| Never delete home/project/library/destination roots or ancestors; invalid raw names | `TestSkillName`, `TestRootSafetyHomeAncestor` in `roots_test.go`; `TestRemoveSkill/protected_roots` and `/invalid_selection` in `mutation_fs_test.go` | Rejected operations preserve protected contents; no prefix-only authorization. |
| Ordinary remove, including folders absent from library; other rows/library unchanged | `TestRemoveSkill` in `mutation_fs_test.go`; `TestRemoveFlow` in `mutation_model_test.go`; `TestPTYRemoveRestart` in `mutation_process_test.go` | Nested/dot/raw names, global/local removal, following/preceding selection, restart persistence and snapshots. |
| Fresh add copies nested/hidden bytes independently; create destinations only on add | `TestAddSkill`, `TestAddSkillStreaming` in `mutation_copy_test.go`; `TestAddFlow` in `mutation_add_model_test.go`; `TestPTYAddRestart` in `mutation_add_process_test.go` | Independent files, raw names, missing roots/configured links, destination mappings, stable library focus/selection and restart. |
| Permissions and executable bits subject to umask | `TestCopyPermissions` in `mutation_copy_test.go`; `TestOperationFailurePermission`, `TestUnavailableRoots` in `operation_failure_test.go` | Masks 000/022/027/077 in child processes; real mode-bit failures and partial state. Root-capability skips do not establish permission coverage. |
| Links anywhere, dangling links, special files and same-name regular-file conflicts rejected before destruction | `TestRemoveSkill/preflight` in `mutation_fs_test.go`; `TestAddSkill/preflight_leaves_no_output` and `/existing_targets` in `mutation_copy_test.go`; `TestReplacePreflight` in `mutation_replace_test.go` | Full target/source snapshots, unchanged outside/library trees, no fresh output on preflight failure. Special-file fixture is FIFO, not every device/socket type. |
| Replacement removes edits and destination-only files; no merge/identical shortcut/backup | `TestReplaceSkill`, `TestReplaceSkillHardlinks` in `mutation_replace_test.go`; `TestPTYReplaceWorkflow` in `mutation_replace_process_test.go` | Delete-then-copy, independent new output even for identical/hardlinked files; real primary add-three/remove-two flow and library snapshot. |
| Failed preflight preserves old target and library | `TestReplacePreflight`, `TestReplacePreflightDestinationObservation`, `TestReplacePreflightDestinationAncestors` in `mutation_replace_test.go`; `TestOperationFailure/*/preflight_read` in `operation_failure_test.go` | Predictable failures precede mutation; destination identity retained across preparation, deletion and copy; no copies/removes on read-preflight failure. |
| Failed removal stops all copying into remainder | `TestReplaceFailure/remove_first`, `/remove_partial` in `mutation_replace_test.go`; `TestOperationFailure/replace/removal` in `operation_failure_test.go` | Removal/copy counters enforce ordering; first failure preserves old bytes; later failure retains partial removal with zero copies. |
| Mid-copy read/write/create/close/permission failure leaves truthful missing/partial state; retry/remove work | `TestReplaceFailure` in `mutation_replace_test.go`; `TestOperationFailure` in `operation_failure_test.go`; `TestAddSkillStreaming` in `mutation_copy_test.go` | Real output with injected faults, exact partial bytes, original error retained, closed handles, no rollback, successful retry/remove and unchanged library/other destination. No real disk exhaustion test. |
| Exact raw spelling and filesystem case aliases, including nested collisions | `TestRemoveSkill/exact_case`; `TestReplacePreflight/nested_case_alias`; `TestAddSkill/existing_targets/case_alias`; `TestCaseCollision` in `stale_state_test.go` | Case-capability probes; macOS HFS+/HFSX fixtures cover distinct source names colliding on target. Known aliases preserve target; previously unobservable nested aliases stop exclusive creation with partial output, not overwrite. |
| Stale selection never redirects to another row; root/entry swaps, links, new children abort when observed | `TestStaleSelection`, `TestStaleSelectionRawName`, `TestObservedChangeMissingAncestor` in `stale_state_test.go`; `TestRemoveSkillObservedChanges`, `TestRemoveSkillInventory` in `mutation_fs_test.go`; `TestAddSkillObservedChanges` in `mutation_copy_test.go` | Captured scan identities, missing ancestors appearing, retargeted aliases, inventory changes and partial deletion; deterministic hooks, not timing races. |
| Failure shows captured operation/name/scope/path; refresh reflects partial state and keeps error | `TestErrorPersistence`, `TestUnavailableRoots` in `operation_failure_test.go`; `TestMutationGuard` in `mutation_model_test.go`; `TestScanGeneration` in `stale_state_test.go` | Navigation/help preserve error; explicit action supersedes it; failed scan remains distinct from empty/missing; safe deletion with unavailable library, stale messages discarded. |
| All keys/uppercase/context/empty/blocked rows, pending g/Esc/paste/help, no mutation queuing/replay | `TestKeyContract`, `TestInputPrecedence`, `TestSelectionPrimaryWorkflow` in `keyboard_contract_test.go`; `TestNavigation`, `TestKeySequence`, `TestHelp`, `TestSelectionRefresh` in `navigation_test.go`; `TestAddGuards` in `mutation_add_model_test.go` | All 1-9 counts, captured mappings, cross-state precedence, deferred/coalesced refresh, raw selection memory and 1/3/9-agent real-tree model flows. Model messages are not terminal-byte evidence. |
| Busy quit waits, success/failure cleanup, persistent diagnostic/status after restoration | `TestPTYQuitDuringMutation`, `TestPTYQuitFailure` in `mutation_quit_process_test.go`; `TestPTYLifecycle`, `TestExitStatus` in `process_test.go`; `TestExitStatusCombinedFailure` in `cli_test.go` | q, Ctrl+C bytes, repeated SIGINT with gated production operations; terminal state/cursor/screen restored; pending failure exits 1 with sanitized stderr, later ordinary quit exits 0. Self-exec probes are test wrappers, not shipped flags. |
| Small-window mutation guard, full raw target inspection, hostile display text | `TestResize`, `TestLayoutBounds`, `TestViewSnapshots`, `TestViewProfiles`, `TestDisplaySafety`, `TestDisplaySafetyRawNames` in `view_test.go` | Snapshots and all color profiles; rendered raw-name add/replace/remove; no control injection or conversion of escaped display text into operation names. Human rendering is separate. |

## Coverage Review

Local `go test -count=1 -coverprofile=coverage.out ./...` and `go tool cover -func=coverage.out` passed: **88.6% statements**. Review used the profile's zero-count blocks and owning implementation/tests, not a percentage target. `coverage.out` is ignored. CI retains the function report in logs/job summary, not a published release asset.

| Area | Measured Functions | Reviewed Gap / Disposition |
| --- | --- | --- |
| Mutation | `addSkillWithOps` 91.2%, `prepareSkillSource` 86.3%, `copyPreparedSkill` 83.8%, `removeSkillInventory` 84.2% | Core destructive/failure scenarios are mapped above. Unhit paths include extra open/stat/readdir failures and identity changes in narrow intervals between checks, e.g. `mutation_copy.go:192-204,318-328` and final removal revalidation in `mutation_fs.go:164-184`. Do not add sleep-based races or a generic filesystem mock to inflate coverage; isolate deterministic regressions if a defect is found. |
| Physical validation | `resolveRoot` 91.8%, `rootWithin` 90.0%, `revalidateSkillRoot` 84.0%, `openSkillRoot` 78.6%, `replacementNames` 87.0% | Some ancestor-stat/open failures and alias branches are absent locally. Case-insensitive execution is a native gate, not replaced by lexical fuzzing or cross-builds. Mount/bind aliases and all filesystem implementations are not demonstrated. |
| Config validation/save | `parseConfig` 95.2%, `configTokens` 97.0%, `resolveConfigPaths`/`expandRoot`/`validateSkillName` 100%, `saveConfigWithIO` 77.5% | Token validation makes the typed-decode failure and non-string object-key fallback defensive paths. String-only schema marshal failures are unreachable. Save parent creation/open/stat/identity failure paths (`config_save.go:180-234`) remain incompletely injected; create/write/shortwrite/sync/close/rename preservation is directly asserted. |
| Input state | Browse/setup `Update`, `startMutation` 100% | Statement coverage is not branch/condition or all-interleaving coverage. Key/precedence/generation tables provide behavioral evidence; native terminal key decoding, resize timing and human usability remain separate Task 26 gates. |
| Process boundary | `run` 68.4%, `runLifecycle`/`main` 0% | Standard package coverage does not merge PTY child counters; separately built binaries are uninstrumented. Self-exec helpers receive disposable `GOCOVERDIR` to prevent runtime warnings from contaminating strict stderr checks; those counters are intentionally discarded. PTY assertions still run, not skipped or filtered. Task 26 may add merged process coverage if useful; 0% here is not proof that lifecycle is untested. |

## Evidence And Open Gates

Local host: Go 1.27.1, Linux amd64, non-root, case-sensitive temporary filesystem (capability-probed). On 2026-09-06:

| Check | Result |
| --- | --- |
| `go test . -run='^$' -fuzz='^FuzzParseConfig$' -fuzztime=30s` | PASS, 360,804 executions, 19 initial seeds, four workers. |
| `go test . -run='^$' -fuzz='^FuzzPathName$' -fuzztime=30s` | PASS, 197,183 executions, 25 initial seeds, four workers. |
| Full coverage + function analysis | PASS, 88.6%. Initial run failed on coverage-runtime stderr warnings; test-only environment fix above, clean rerun passed. |
| `CGO_ENABLED=1 go test -race -count=1 ./...` | PASS, 113.456s. First attempt exceeded the external 120s tool limit while checks ran concurrently; rerun allowed 300s. |
| Focused verbose config/root/mutation/failure/stale/key/selection regressions | PASS, including real permission cases and all four umasks; three case-dependent skips below. |
| Full uncached tests, module tidy diff, lint config/format/run, vet, build, diff whitespace | PASS; lint reports zero issues. |
| New CI jobs and current native Linux arm64/macOS changes | Not executed remotely; no push authorized. Earlier native evidence is recorded in the implementation plan, not a pass for this worktree. |
| Workflow validation | Diff/structure reviewed locally; `actionlint` unavailable. Hosted execution remains open. |

- Local skips: `TestAddSkill/existing_targets/case_alias`, `TestReplacePreflight/nested_case_alias`, and `TestCaseCollision` require case-insensitive targets. Existing macOS volume tests must run in the native matrix; do not convert their absence here into a pass. Invalid-UTF-8 names and permission cases ran locally; other hosts may reject those fixtures or bypass permissions and must record that capability.
- Special-file coverage uses FIFO; sockets/devices, ACLs, real ENOSPC/quota exhaustion and storage hardware failures are not individually reproduced. Injected short/read/write/permission/close failures prove partial-state handling, not every kernel error source. Add bounded real or deterministic regressions if those uncovered mechanisms expose different behavior.
- Ordinary coverage/race runs leave opt-in representative archive/release-binary performance tests unexercised. Prior budgets are in `prototype.md`; this audit does not revalidate them.
- Task 26 local process evidence follows; native results remain open. Manual GNOME Terminal/VTE, Terminal.app, light/dark/no-color, SSH, busy quit and owner workflow acceptance were unavailable, not passed. Platform floors and installer/archive/upgrade failure tests remain release tasks.
- No hostile-writer isolation, arbitrary race safety, crash durability, rollback/recovery, forced-kill completion or mount sandbox is claimed. Fuzzing is bounded, and race detection covers only exercised paths. These are explicit contract limits, not tests to mark green.

## Task 26 Process Gate

2026-09-06, Linux amd64, after Task 25 (`68f1c29`). `TestPTYFirstRunSetup/native-flow` reuses the setup harness and terminal emulator: no config override, first-run save into native HOME, same-process add-three/remove-two key flow, local/global focus and full help, bracketed-paste rejection, external destination change plus `r`, 200x40 -> 79x24 -> 200x40 resize, q, and restart. It asserts absent roots before add, persisted config, removed rows, independent surviving nested/hidden/empty copy, and unchanged library bytes/identities. Current cells, bounded deadlines and exit/stderr/raw-frame diagnostics replace historical-text waits; no sleeps. Existing gated `TestPTYQuitDuringMutation`/`TestPTYQuitFailure` retain real operations, repeated interrupt acknowledgments, success/failure cleanup and persistent stderr/status, not shipped-binary fault hooks.

All `TestPTY` child environments use disposable HOME/XDG and an empty disposable PATH (no agent/tool lookup); setup fixture preparation now isolates parent HOME too. The native-flow config expectation selects Linux `.config` or macOS `Library/Application Support`; macOS execution is still required.

| Local Check | Result |
| --- | --- |
| `unshare --user --map-current-user --net env GOTOOLCHAIN=go1.27.1 GOPROXY=off GOSUMDB=off go test -count=1 -run 'TestPTY' .` | PASS after tool/module preparation; isolated namespace has only down loopback. Proves offline execution without application network access, not a syscall-attempt audit or macOS network isolation. |
| Setup suite 10x; busy success/failure suite 10x; both suites with race 5x | PASS. Standalone `go build` children are uninstrumented; self-exec probes inherit race instrumentation. |
| Full uncached tests, full race, tidy diff, lint config/format/run, vet, build | PASS; zero lint issues. |
| macOS amd64/arm64 test cross-builds | PASS compilation only. Native Linux arm64/macOS matrix, manual terminal/theme/SSH and owner acceptance unavailable, not passed; no push authorized. |

### Native CI Follow-Up

2026-09-06: fetched failed logs with `gh run view 34014129461 --repo primaprashant/sei --log-failed`, head `acb04e4e5b129fab9a0e1d10e007ff3e3068002c`.

| Job | Failure Evidence |
| --- | --- |
| [macos-15 / darwin arm64](https://github.com/primaprashant/sei/actions/runs/34014129461/job/101434884219) | `TestPTYFirstRunSetup/native-flow`, `setup_process_test.go:423`: `timeout waiting for ["Enter save"]`; screen shows `Error: config must not overlap managed root` ending `/001/library`. |
| [macos-15-intel / darwin amd64](https://github.com/primaprashant/sei/actions/runs/34014129461/job/101434884248) | Same test, line, timeout and overlap diagnostic. Both jobs reached tests after lint reported `0 issues.` |

Both screens place config at `<HOME>/Library/Application Support/sei/config.json` while setup selects `~/library`. On these case-insensitive filesystems the precreated source aliases macOS's native config ancestor. `configSaveLocation` correctly rejects the overlap via `rootWithin` ancestor identity; preview never appears. The subsequent `setup_process_test.go:231: PTY reader: context deadline exceeded` is fallout from the same deadline, not a separate reader defect.

The test-only fix uses `~/skill-library` for native-flow input, fixture and exact persisted-config expectation on both OS families. Native config lookup (no override), safety guards, timeouts, terminal restoration, add/remove/restart and byte/identity assertions remain unchanged. No production or CI changes. The original run's Linux amd64/arm64 and both fuzz jobs passed; this does not verify the fixed worktree on those remote runners.

Local verification on Go 1.27.1, Linux amd64:

| Check | Result |
| --- | --- |
| `go test -count=10 -run '^TestPTYFirstRunSetup$/^native-flow$' .` | PASS, 16.812s. |
| `go test -count=1 ./...` | PASS, 21.941s. |
| `CGO_ENABLED=1 go test -race -count=1 ./...` | PASS, 112.816s; standalone built PTY children remain uninstrumented. |
| Lint config/format/run, `go vet ./...`, `go mod tidy -diff` | PASS, zero lint issues and no format/module diff. |
| Linux application build, `--help`, `--version` | PASS. |
| `CGO_ENABLED=0` test and application cross-builds for `darwin/amd64` and `darwin/arm64` | PASS compilation only, not native execution or case-insensitive filesystem evidence. |

Native macOS rerun remains pending; no push or rerun initiated. Manual terminal/SSH/owner gates remain open.
