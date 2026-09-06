# Implementation Plan: sei

## Overview

Build and publish the `sei` terminal application described in [product-vision.md](product-vision.md) and [prd.md](prd.md). The deliverable is a self-contained skill-folder manager, not a new agent skill, skill marketplace, or agent launcher. This document contains implementation plan for the tasks: small vertical slices, explicit dependencies, tests inside each feature, and checkpoints every three tasks.

**Status:** Phase G implementation and automated Linux evidence are available in [the prototype report](docs/prototype.md); native Phase G and owner terminal/SSH review remain pending. No Phase H layout or minimum approved.

**Starting point:** Docs-only local repository; existing `origin` and `.gitignore` preserved.

## Decisions

### Confirmed With The Owner

| Item | Decision |
| --- | --- |
| Public repository | `https://github.com/primaprashant/sei` |
| Go module | `github.com/primaprashant/sei` |
| License | MIT; confirm the copyright attribution when creating `LICENSE`. |
| Release versions | Initial published release `v0.1.0`, followed by `v0.x` development releases; first usable release `v1.0.0`. |
| Product and engineering contracts | The PRD remains authoritative; this plan sequences its implementation. |

### Publication Identity

- Start published versions at `v0.1.0`, continuing with `v0.x` development releases as needed. Reserve `v1.0.0` for the first usable release meeting the PRD and release-readiness gates; subsequent releases continue from `v1.0.0` using semantic versioning.
- Use versioned assets under `https://github.com/primaprashant/sei/releases/download/<tag>/`.
- Publish `scripts/install.sh` as a release asset named `install.sh`. The planned stable entry point is `https://github.com/primaprashant/sei/releases/latest/download/install.sh`; reproducible instructions use `/releases/download/<tag>/install.sh` and an explicit version.
- These are intended URLs, not claims that the repository or assets already exist. Record the approved version policy in Task 1; individual tag/push/publication actions still require explicit authorization. Verify actual URLs before advertising installation in Task 36.

### Architecture

- Start with one root `package main`. Add responsibility-based files as needed, not empty package scaffolding. Keep `main` responsible for final exit after cleanup; use concrete functions and structs elsewhere.
- Use the PRD's Go 1.27.1, Bubble Tea v2.0.9, Lip Gloss v2.0.6, Bubbles v2.2.1, golangci-lint v2.13.2, and GoReleaser Community v2.18.0 baselines. Recheck availability, compatible patches, upstream APIs, and preset documentation at kickoff; record exact selected versions rather than silently substituting a different stack.
- Model state belongs to `Update`. Scans and a single sequential mutation run in `tea.Cmd`; messages carry operation identity and panel scan generation. Never batch deletion, copying, and refresh as independent commands.
- Use `encoding/json`, `flag`, `os`, `io/fs`, `path/filepath`, and `testing`. No generic filesystem framework, config framework, or speculative internal packages.
- A partial milestone may explicitly leave an unfinished action unavailable. Never wire an unsafe placeholder mutation. Tests and builds must stay green at every task; do not call an intermediate milestone a compliant public release.
- Setup/config saves, skill mutations, and installer upgrades have different replacement contracts. Config and installer replacement protect existing data until commit. Skill replacement deliberately deletes first and can leave partial work.

## Concrete Mechanisms

These are the proposed implementation mechanisms, not claims that they have already passed platform tests. Tasks 5, 8, 9, and 12 validate them before the destructive workflow is enabled.

### JSON And Configuration Saving

1. Read the configuration bytes once into a fresh parse operation. Walk `json.Decoder.Token` recursively, keeping a separate decoded-key set for each object. Reject repeated keys even when spelled using different JSON escapes. Validate exact schema key spelling and value shapes, including rejecting `null` in required objects, arrays, and strings.
2. Decode the same bytes into fresh typed structs with `DisallowUnknownFields`; require EOF after the one value. Do not decode over setup defaults. Validate required values, agent names/count/order, and path categories. The token pass avoids both duplicate-key collapse and Go's permissive case-insensitive struct-field matching.
3. Resolve configuration location using `os.UserConfigDir()` or `--config`. Resolve the project once from launch cwd or `--project`; do not change cwd or find a Git root. Expand only leading `~/` for library/global paths; no shell evaluation.
4. After preview, validation, and required configuration-replacement confirmation, marshal pretty JSON with a final newline. Safely create missing config-parent directories, subject to the approved write-placement policy; this must not create a skill destination or modify the library. Create a private temporary file in that parent directory, write, check `Sync` and `Close`, then rename to commit. Remove temporary output on pre-commit failure; never open the existing config with truncation. Cancellation before saving creates no directories; failed saving may leave an empty config-parent directory, not modified skill folders or truncated existing config.
5. Rename is the save commit point. Do not introduce a post-rename failure path that falsely promises the previous bytes remain unchanged. No portable crash-durability or concurrent-config-writer guarantee is being added.

### Rooted Filesystem Work

1. Validate raw selected names as one nonempty filename component, excluding `.`, `..`, separators, and NUL. Retain raw bytes for operations; display escaping must never become a filesystem name.
2. Resolve existing root components and symlink aliases using filesystem semantics. For absent suffixes, retain the deepest verifiable existing directory and missing components; distinguish absence from permission errors, dangling links, and non-directory ancestors. Do not let lexical cleaning hide `symlink/..` traversal. Enforce both lexical and resolved project-local containment.
3. Compare root relationships component-wise with `filepath.Rel`, supplemented by `os.SameFile` identity comparisons of existing roots/ancestors for aliases and case-insensitive filesystems. Reject library/destination identity or nesting and all destination overlaps. Never mutate a child that is a protected root or library ancestor. Revalidate these relationships immediately before mutations and after creating a missing destination.
4. Open validated roots using `os.OpenRoot`. Use `Root.Lstat`, `Root.Open` plus `File.ReadDir`, `Root.OpenRoot`, `Root.Mkdir`, `Root.OpenFile`, and `Root.Remove`. Do not substitute `os.CopyFS`, unrestricted recursive path operations, or a string-prefix safety check.
5. Preflight the complete source and existing target trees with `Lstat`, allowing only ordinary directories and regular files. Inspect readable source files before deletion where practical, compare opened-file identities, and retain a small per-operation entry inventory for observed-change checks. This is not a cross-operation cache or a consistent content snapshot.
6. Before replacing, require an exact raw-name match in the target directory. A lookup that resolves an existing entry without that exact spelling is an alias/case collision, not permission to overwrite. Detect actual filesystem behavior rather than using Unicode lowercasing as a proxy. Apply collision handling to nested copied names as well; reject predictable conflicts before deleting old content where they can be established without writing.
7. Delete preflighted target entries postorder with rooted `Remove`, rechecking observed types/identities. An unexpected new child should make removal fail, not be swept away by an unrestricted `RemoveAll`. Do not copy if deletion was incomplete.
8. On add only, create missing destination components beneath the validated ancestor, then reopen/revalidate the actual destination. Create independent output files with `O_CREATE|O_EXCL`; copy bytes and check read, write, and close errors. Use `0666 | (sourceMode & 0111)` for files and `0777` for directories before umask; do not restore special bits or circumvent umask with later chmod.
9. `os.Root` prevents escapes but permits some in-root links and crossing mount points. Explicit no-link preflight and observed-change checks remain necessary. No hostile-writer sandbox, multi-process lock, source snapshot, rollback, or recovery guarantee is claimed. If a safety relationship cannot be established, block the affected mutation with a reason rather than guess.

### Input And Lifecycle

- Use current `tea.KeyPressMsg` and `tea.View` APIs. Recognized paste messages never become action keys. Keep sequence handling, help, busy state, pending quit, and small-window mutation guards explicit in one state machine.
- A mutation request captures raw skill, destination, scope, and operation ID. While it runs, navigation stays responsive; extra mutations are rejected, refresh is deferred/coalesced, and pre-mutation scan results cannot overwrite current state.
- Check the pinned Bubble Tea signal behavior before relying on defaults: runtime interrupt/quit handling can bypass model updates, and quitting does not automatically join command work. The proposed approach is `WithoutSignalHandler()` with a lifecycle-owned `os/signal` loop forwarding an ordinary exit-request message; emit `tea.Quit` only when active mutation work has completed. Repeated SIGINT and raw Ctrl+C bytes must both obey wait-on-quit.
- Return final operation diagnostics out of the TUI, restore the terminal, then write sanitized stderr and choose status `1` for a failure after pending quit. A recoverable error followed by a later ordinary quit remains status `0`.

### Installer Ownership

- Establish ownership on first install rather than trying to identify arbitrary executables by running `--version`. Maintain a small installer-owned `.sei-install-receipt` beside the binary, containing validated version/digest records for verified installed or prepared binaries. Compare the existing executable's SHA-256 against those records before allowing an upgrade; refuse missing/mismatched receipts and symlink/non-regular receipt or executable paths.
- Prepare a replacement receipt retaining the old digest and adding the verified candidate digest, and commit that receipt before the final binary rename. Thus a receipt-write failure leaves the old binary untouched; a binary-rename failure leaves an old digest still recognized. Do not require a second fallible metadata commit after replacing the executable. This is local installer ownership evidence, not publisher authentication or protection against a hostile local user.
- Refuse automatic replacement of manually installed or locally modified binaries without a matching receipt; provide explicit manual replacement guidance. No headless application command, user runtime dependency, automatic privilege escalation, or broad overwrite flag is needed.

## Verification Baseline

Every feature task includes its tests and focused verification; Tasks 25-26 audit and broaden coverage rather than introducing testing for the first time. Proposed test names below are contracts to create with those tasks, not commands claimed to work in the current docs-only repository.

**Standard checks**, available after Task 4, run from the repository root with `GOTOOLCHAIN=go1.27.1`:

```sh
go version
go mod tidy -diff
./.bin/golangci-lint config verify --config .golangci.yml
./.bin/golangci-lint fmt --config .golangci.yml --diff
./.bin/golangci-lint run --config .golangci.yml --timeout=5m ./...
go vet ./...
go test -count=1 ./...
go build -o ./bin/sei .
```

Create `bin/` before build commands. CI must fail on formatting differences without rewriting tracked files. Run the relevant tests and build in each task and the standard checks at each checkpoint. Run `CGO_ENABLED=1 go test -race -count=1 ./...` on Linux amd64 with a C compiler; this is separate from CGO-disabled release builds.

All manual experiments and tests use disposable HOME/config/project/library/destination paths. Test helpers must explicitly isolate macOS's native config location as well as Linux XDG paths. No tests execute skill scripts, inspect real agent installations, or read/mutate the maintainer's actual skill library. Permission tests account for root; umask tests run in isolated subprocesses; case tests detect the actual filesystem.

### CI And Release Matrix

| Purpose | Selected Runner Or Environment | Evidence Required |
| --- | --- | --- |
| Linux amd64 PR tests, lint, race, fuzz | `ubuntu-24.04` | Native tests; C compiler for race; captured runner image/tool versions. |
| Linux arm64 PR/release execution | `ubuntu-24.04-arm` | Native tests and execution of the actual arm64 release archive. |
| macOS amd64 PR/release execution | `macos-15-intel` | Native tests and actual amd64 release binary; not Rosetta-only evidence. |
| macOS arm64 PR/release execution | `macos-15` | Native arm64 tests and actual release binary. |
| Proposed Linux release floor | Ubuntu 22.04 with a 5.15-series kernel, amd64 and arm64 VMs/machines | Record exact image and kernel. A container on a newer host does not validate the kernel floor. Owner approves the measured floor in Task 28. |
| PRD macOS minimum | macOS 13 machines/VMs on both architectures | Acquire manual/release-test access; newer CI images alone cannot validate this floor. Block the support claim if evidence is unavailable. |
| Terminal usability | Linux GNOME Terminal/VTE and macOS Terminal.app; OpenSSH from macOS to Linux | Record exact versions, `$TERM`, dimensions, light/dark/no-color behavior, and local versus SSH results. Owner's usual terminal is included in the prototype review. |

GitHub's [runner reference](https://docs.github.com/en/actions/reference/runners/github-hosted-runners) lists the selected labels at planning time. They pin OS families, not immutable images; recheck availability and actual architecture during Task 4. Never silently drop an architecture if a runner is unavailable.

**Test tooling proposal:** ShellCheck `v0.11.0`, with release-integrity verification, for POSIX installer lint. Use Go's existing `testing`, `os/exec`, and local/mocked downloads for shell integration tests; no Bats or extra shell test framework. For real terminal automation, propose test-only `github.com/creack/pty v1.1.24`, subject to owner approval and compatibility verification in Task 9; use the resolved terminal-mode helper from the Charm dependency graph only after explicitly reviewing the direct dependency. Pin any approved test dependency in `go.mod`/`go.sum`; no additional runtime is required by installed users.

## Dependency Graph

```text
1 -> 2 -> 3 -> 4                 Repository, executable, repeatable checks
          2 -> 5 -> 6 -> 7       Launch from config, inspect real folders, navigate
                    5 -> 8      Physical root safety before mutation
               4 + 7 -> 9       Real terminal/lifecycle proof
           7 + 8 + 9 -> 10 -> 11 -> 12    First-run and explicit setup
               8 + 9 + 12 -> 13 -> 14 -> 15    Remove, fresh add, replacement
                             15 -> 16 -> 17 -> 18 -> 19 -> 20
                                  Recoverable failures and complete input lifecycle
                             20 -> 21 -> 22 -> 23 -> 24
                                  Representative prototype, owner gates, polished layout
                        17 + 20 -> 25
                        19 + 24 + 25 -> 26    Coverage and real process audit
              3 + 4 + 9 -> 27 -> 28 -> 29    Archives, support evidence, trust policy
                             27 + 29 -> 30 -> 31 -> 32    Installer slices
                         23 + 24 + 28 + 29 + 32 -> 33    User documentation
                         26 + 28 + 29 + 32 -> 34         Draft-release pipeline
                         23 + 33 + 34 -> 35 -> 36        RC rehearsal, publication
```

Task numbers below are the default execution order; Tasks 27-32 can move earlier along their dependency branch. Checkpoints still occur after each two or three completed tasks in a parallel branch. Do not interpret the diagram as permission to edit shared files concurrently without coordination.

## Task List

### Phase A: Repository Bootstrap

### Task 1: Establish The Public Project

**Description:** Make the existing local repository ready to become the MIT-licensed `primaprashant/sei` project without disturbing its history or local agent setup.

**Acceptance criteria:**
- [x] Confirm GitHub repository existence/ownership with `gh`; record `github.com/primaprashant/sei`, copyright attribution, and owner-approved initial tags. Create a remote repository or configure `origin` only during authorized implementation, without overwriting an existing remote.
- [x] Add MIT `LICENSE`, a short README with scope and implementation status, and ignores for `.bin/`, `bin/`, `dist/`, coverage output, retaining `/.opencode/`.
- [x] Record that public pushing, credentials, branch protection, and releasing require explicit authorization; do not invent already-working install instructions.

**Verification:** Inspect `git status --short`, `git diff --check`, `git remote -v`, and `gh repo view primaprashant/sei`; verify the license and ignore behavior. No application tests exist yet.

**Dependencies:** None; owner review of this plan.

**Files likely touched:** `LICENSE`, `README.md`, `.gitignore`.

**Estimated scope:** Small, 3 short metadata files; no application code.

### Task 2: Run The First Executable

**Description:** Initialize the real Go module and produce the smallest working CLI and disposable terminal shell, proving the pinned Charm v2 stack builds before feature work.

**Acceptance criteria:**
- [x] Initialize with `go mod init github.com/primaprashant/sei`; verify/pin the PRD toolchain and Charm versions, commit module sums when committing is authorized, and document any approved patch adjustment.
- [x] `--help` and `--version` work without a terminal/config; unsupported arguments return syntax status `2`. A minimal interactive screen can render and quit without filesystem mutations; non-TTY startup fails clearly.
- [x] Use current Charm imports, `tea.KeyPressMsg`, and `tea.View`; introduce the selected dependencies through actual screen/help use rather than unused imports.

**Verification:** `go test -count=1 ./...`, `go build -o ./bin/sei .`, `./bin/sei --help`, `./bin/sei --version`; manual terminal enter/quit and piped-start rejection. Use a temporary version such as `dev`, not a fake release.

**Dependencies:** Task 1.

**Files likely touched:** `go.mod`, `go.sum`, `main.go`, `cli.go`, `cli_test.go`.

**Estimated scope:** Medium, 5 files; keep the initial screen minimal.

### Task 3: Make Local Checks Repeatable

**Description:** Establish editor-independent formatting and the small PRD lint ruleset before implementation grows.

**Acceptance criteria:**
- [x] Create the PRD's v2 `.golangci.yml` with only the five selected linters and `goimports`; include tests and avoid broad suppressions.
- [x] Add EditorConfig for LF, final newlines, and Go tabs, with no arbitrary line-length limit; document optional editor format/import organization on save.
- [x] Document pinned tool installation, integrity/review expectations, exact `GOTOOLCHAIN`, root-level developer commands, and disposable-data precautions.

**Verification:** Run formatter/config verification, lint, `go vet ./...`, tests, and build. Confirm generated tooling and binaries remain ignored.

**Dependencies:** Task 2.

**Files likely touched:** `.golangci.yml`, `.editorconfig`, `README.md`.

**Estimated scope:** Small, 3 configuration/documentation files.

### Checkpoint A: Tasks 1-3

- [x] A fresh local checkout builds and displays help/version using the documented commands.
- [x] Formatting, lint, tests, and build pass; no generated binary or local agent config is tracked.
- [x] Owner approves the repository/license/tag decisions before public repository changes proceed.

**Verified (2026-09-05):** Fresh-checkout build/help/version, local checks, Linux race and PTY smoke tests. Tooling details are in README; cross-platform lifecycle verification remains Task 9.

### Phase B: Inspect A Real Library

### Task 4: Verify Every Change In CI

**Description:** Put the working executable behind repeatable, secret-free native CI rather than deferring automation to release week.

**Acceptance criteria:**
- [x] Add PR/push checks with reviewed full-SHA action references and exact Go/linter versions; verify downloaded tool integrity and module hygiene.
- [x] Run native Linux/macOS tests on the selected four architecture labels, Linux amd64 race checks, and non-rewriting formatting/lint/vet checks; record actual runner OS/architecture.
- [x] Use read-only permissions for PR jobs, no release secrets or live agent installations, and no generated-file rewriting to make CI pass.

**Verification:** Run standard checks locally; exercise the workflow on an authorized branch/PR and verify all four jobs actually ran. Record unavailable runner access as a blocker, not a pass.

**Verified (2026-09-05):** Local checks and all four native CI jobs pass; see `docs/release.md`.

**Dependencies:** Task 3.

**Files likely touched:** `.github/workflows/ci.yml`, `docs/release.md`.

**Estimated scope:** Small, 2 files plus CI verification.

### Task 5: Launch From Strict Configuration

**Description:** Let a user point the executable at a real JSON configuration and project, with actionable errors before any terminal or filesystem mutation.

**Acceptance criteria:**
- [x] Implement the JSON mechanism above with table tests for duplicate/escaped keys, unknown/exactly spelled keys, incorrect types/null, trailing values, required fields, duplicate names, and 1-9 ordered agents.
- [x] Resolve native config locations, relative-XDG rejection on Linux, `--config`, launch-directory project semantics, relative `--project`, `~/`, spaces/Unicode, and local lexical containment without config merging or shell expansion.
- [x] Wire command dispatch and stdout/stderr/status behavior: malformed/unreadable existing config never starts setup or gets overwritten. Missing configuration produces an explicit not-yet-implemented setup diagnostic until Task 10.

**Verification:** `go test -count=1 -run 'Test(ParseConfig|ConfigPaths|CLI)' .`, standard build, and disposable manual launches with config/project overrides. Include non-TTY help/version with invalid config present.

**Verified (2026-09-05):** Focused/standard/race checks and disposable CLI launches pass, including permissions, dangling config, and raw path traversal. Physical root safety remains Task 8.

**Dependencies:** Task 2.

**Files likely touched:** `config.go`, `config_test.go`, `cli.go`, `cli_test.go`.

**Estimated scope:** Medium, 4 files.

### Task 6: Browse Actual Configured Folders

**Description:** Replace the terminal shell with a read-only vertical slice from configured paths through asynchronous directory scans to library and destination panels.

**Acceptance criteria:**
- [x] Scan immediate ordinary directories, including dot-directories, without parsing `SKILL.md`; ignore loose files and show symlinks as blocked. Use deterministic raw Go string ordering.
- [x] Render the library left and configured global/local destinations right, with raw selected name stored separately from sanitized/cell-width-aware display text. Missing destinations stay absent and show not-created; inaccessible paths show errors.
- [x] Scans run outside `Update`/`View`, with typed results and generation checks. An unavailable library or panel does not prevent reading other panels; no add/remove inputs are enabled yet.

**Verification:** `go test -count=1 -run 'TestBrowse' .` with real temporary trees and injected scan completion order; build and inspect an isolated three-agent configuration.

**Verified (2026-09-05):** Focused/standard/race checks and disposable three-agent PTY smoke pass (Linux, xterm-256color, 150x36): populated panels, absent destinations unchanged, actions disabled, clean quit. Layout and lifecycle proof remain provisional.

**Dependencies:** Task 5.

**Files likely touched:** `cli.go`, `model.go`, `view.go`, `skills.go`, `model_test.go`.

**Estimated scope:** Medium, 5 files; initial browse tests cover the integrated scan path.

### Checkpoint B: Tasks 4-6

- [x] Standard checks pass locally and native CI passes on all four selected architecture jobs.
- [x] A manually written config opens populated real folders without creating any destination.
- [x] Invalid configuration, unavailable folders, and blocked links are distinguishable; mutations remain disabled.

**Verified (2026-09-05):** Local checks/terminal smoke and all four jobs in [CI run 33966759655](https://github.com/primaprashant/sei/actions/runs/33966759655) pass; platform evidence is in `docs/release.md`.

### Phase C: Targets And Terminal Safety

### Task 7: Navigate Unambiguous Targets

**Description:** Make the read-only UI genuinely usable with all positional focus keys, selection memory, visible pending sequences, and inspectable full targets.

**Acceptance criteria:**
- [x] Implement Up/Down clamping, `0`, local `1-9`, global `g` then `1-9`, per-panel selection memory, and `r` selection preservation by raw name; unconfigured slots do nothing.
- [x] Show headers and exact `a b c d e f h i o` / uppercase destination mappings, persistent pending `g` without timeout, consumed invalid continuations, and effective Esc/quit behavior. Ignore recognized paste input.
- [x] `?` exposes full sanitized name/path and the shared-discovery disclaimer; focus/scope/agent are readable without color, and navigation does not erase a displayed error. All 1-9 slots are addressable even while final layout dimensions remain provisional.

**Verification:** `go test -count=1 -run 'Test(Navigation|KeySequence|Help|SelectionRefresh)' .`; manual one/three/nine-agent navigation and long-name inspection, including `g` then a would-be action key.

**Verified (2026-09-05):** Focused/standard/build/race checks and disposable 1/3/9-agent Linux PTY smoke pass (100x30, xterm-256color, no-color): navigation, long-name help, consumed `ga`, paste, pending quit, restored termios, destinations absent. Layout provisional; mutations disabled; native macOS/SSH and lifecycle proof deferred.

**Dependencies:** Task 6.

**Files likely touched:** `keys.go`, `model.go`, `view.go`, `model_test.go`.

**Estimated scope:** Medium, 4 files.

### Task 8: Reject Unsafe Resolved Targets

**Description:** Connect physical root validation to launch so the UI cannot present an unsafe configuration as mutation-ready later.

**Acceptance criteria:**
- [x] Implement the rooted-resolution mechanism above, including missing ancestors, permitted configured-root symlinks, local resolved containment, component-aware overlap checks, and filesystem identity/case aliases.
- [x] Reject source/destination nesting and destination overlap in both directions; protect library ancestors, project/home/destination roots, and invalid child names. Keep root revalidation callable by each mutation, not a one-time setup check.
- [x] Test absent, dangling, non-directory, inaccessible, and alias cases on native Linux/macOS. Distinguish an unavailable listing from an unverifiable safety boundary; preserve usable panels when their mutation safety can still be established.

**Verification:** `go test -count=1 -run 'Test(RootSafety|ResolveRoots|SkillName)' .`; review rooted API usage against pinned Go docs and record the selected algorithm/limitations. No skill mutation is enabled in this task.

**Verified (2026-09-05):** Linux focused/standard/build/race checks pass, including protected-root ancestry. Native macOS tests pending; mutations disabled. Algorithm/API review: [filesystem safety](docs/filesystem-safety.md).

**Dependencies:** Task 5.

**Files likely touched:** `config.go`, `config_test.go`, `skills.go`, `skills_test.go`, `docs/filesystem-safety.md`.

**Estimated scope:** Medium, 5 files; stop and split alias-resolution cases into a follow-up if the proof exceeds one focused session.

### Task 9: Prove Real Terminal Lifecycle

**Description:** Test the real process boundary early, before async mutations make incorrect interrupt behavior destructive.

**Acceptance criteria:**
- [x] Approve and pin the minimal PTY test dependency proposal, verify both stdin/stdout TTY checks, and test actual key bytes under a PTY rather than model messages only.
- [x] Establish one lifecycle owner and ordinary exit-request message path for raw Ctrl+C and repeated SIGINT; verify the pinned Bubble Tea default handlers cannot bypass it. Approve/document SIGTERM and SIGHUP policy without changing the existing Ctrl+C contract.
- [x] Demonstrate idle quit, startup/runtime failure cleanup, no alternate-screen output on non-TTY failure, and restored terminal modes/cursor/screen state. Keep test coordination out of shipped flags and environment variables.

**Verification:** `go test -count=1 -run 'TestPTYLifecycle' .`, Linux race check, and a manual terminal round trip on both OS families. PTY timeout is a test failure, not a skipped result.

**Evidence (2026-09-05):** Linux checks pass. Both Macs failed PTY probes in run `33970232319`; fixed session-revocation and deferred-kevent fault assumptions. Native rerun/manual checks pending; assertions retained. [Details](docs/terminal-lifecycle.md).

**Dependencies:** Tasks 4 and 7; owner approval of test tooling and interruption clarification.

**Files likely touched:** `main.go`, `model.go`, `process_test.go`, `go.mod`, `go.sum`.

**Estimated scope:** Medium, 5 files; test-only helpers stay in `process_test.go` initially.

### Checkpoint C: Tasks 7-9

- [x] Standard checks and PTY lifecycle checks pass; all configured targets are inspectable without mutation.
- [x] Review filesystem containment/overlap evidence before enabling either destructive action.
- [x] Resolve ambiguity gates for unverifiable roots, configuration placement, and signals before the tasks that depend on them; do not silently broaden supported behavior.

**Verified (2026-09-05):** Local standard/build/race/PTY checks pass; root/lifecycle review fixes tested. Native macOS safety/PTY execution and manual terminal round trips remain pending; mutations disabled, no push performed.

### Phase D: First-Run Setup

### Task 10: Complete First-Run Setup

**Description:** A fresh user can select a library, review the default three agents, save configuration, and enter the real browser without hand-editing JSON.

**Acceptance criteria:**
- [x] Start setup only when config does not exist; initially select Claude Code, Codex, and OpenCode with the documented editable paths. Collect a library, validate paths/roots, and preview resolved destinations and shortcuts.
- [x] Show permanent deletion/replacement warnings and the exact shared-discovery disclaimer. Create missing config parents only when saving, then save pretty JSON with the safe temp-file/rename mechanism; configuring/viewing destinations never creates them.
- [x] Cancellation leaves config bytes and skill folders unchanged with exit `0`; successful first-run save opens the TUI. All I/O during interactive setup follows the same responsive model/command ownership rule.

**Verification:** `go test -count=1 -run 'Test(FirstRunSetup|SaveConfig)' .`; manually complete and cancel setup with isolated native-location config paths, then restart to verify persistence.

**Verified (2026-09-06):** Linux tests/lint/build and isolated PTY cancel/save/restart pass; native macOS/manual review pending.

**Dependencies:** Tasks 7, 8, and 9; configuration-write policy gate below.

**Files likely touched:** `setup.go`, `setup_test.go`, `config.go`, `config_test.go`, `cli.go`.

**Estimated scope:** Medium, 5 files; default-agent flow only in this slice.

### Task 11: Reconfigure Ordered Agents

**Description:** Extend the working setup flow to explicit reconfiguration, all presets, and custom one-to-nine-agent arrangements.

**Acceptance criteria:**
- [x] `sei setup` is prepopulated from a valid existing file, requires confirmation before configuration replacement, saves and exits, and never repairs/overwrites malformed existing JSON implicitly.
- [x] Users can select fewer agents, add Pi/Cursor or a custom name/path pair, edit destinations, and choose order; bounds and duplicate names are enforced. Preview key mappings reflect that exact order.
- [x] Cancellation at any step leaves existing config and skills unchanged; no last project, selection, or focus is persisted and no unconfigured agent is discovered.

**Verification:** `go test -count=1 -run 'Test(SetupAgents|ExplicitSetup)' .`; manual one-agent and nine-agent setup, reorder, replacement refusal, and confirmed save.

**Verified (2026-09-06):** Linux tests/race/lint/build and PTY one/nine-agent save, refusal, and exit pass; manual/native review pending.

**Dependencies:** Task 10.

**Files likely touched:** `setup.go`, `setup_test.go`, `config.go`, `cli.go`.

**Estimated scope:** Medium, 4 files.

### Task 12: Survive Configuration Save Failures

**Description:** Complete the setup slice's persistence failure coverage before real skill mutations depend on saved targets.

**Acceptance criteria:**
- [x] Exercise marshal/validation, temporary create/write/sync/close, and rename failures using real filesystem conditions or narrowly scoped deterministic seams; pre-commit failures preserve valid existing bytes.
- [x] Apply the owner-approved config symlink/placement policy and reject observed unsafe changes before committing; no config save can mutate the source library or become part of a deletable managed skill inadvertently.
- [x] Report setup-save failures on stderr with status `1`, clean temporary config output where possible, and distinguish committed saves from pre-commit failures without claiming crash durability.

**Verification:** `go test -count=1 -run 'Test(ConfigSaveFailure|ConfigSaveSafety)' .`; compare byte snapshots of config, library, and destinations after every failure case.

**Verified (2026-09-06):** Injected I/O failures, old/new-root placement, observed-change snapshots, and PTY stderr/status/cleanup pass. Typed schema has no reachable marshal-error case; invalid values/UTF-8 tested.

**Dependencies:** Task 11.

**Files likely touched:** `config.go`, `config_test.go`, `setup_test.go`.

**Estimated scope:** Medium, 3 files.

### Checkpoint D: Tasks 10-12

- [x] Standard checks pass; first-run setup opens the browser and explicit setup saves/exits.
- [x] Cancel and failed-save tests prove previous config and all skill folders remain unchanged.
- [ ] Owner can configure their actual intended paths in a disposable equivalent without hand-editing JSON.

**Verified (2026-09-06):** Linux standard/build/race/PTY checks and both macOS test cross-builds pass. Owner disposable-path review/native execution pending; no push, mutations disabled.

### Phase E: First Destructive Workflow

**Dependency evidence (2026-09-06):** Reviewed rooted safety code and [CI 33994932478](https://github.com/primaprashant/sei/actions/runs/33994932478): all four native jobs pass at `2dc65db`, including safety/setup/PTY tests and Linux race. Manual terminal/owner reviews remain pending.

### Task 13: Remove A Destination Skill

**Description:** Deliver the first complete mutation path: focus a destination, press uppercase `X`, safely preflight/delete one ordinary directory, and see the resulting listing.

**Acceptance criteria:**
- [x] Connect captured selection through root/name/type validation, full target-tree no-link/special-file preflight, postorder rooted removal, and destination refresh. Removing a skill absent from the library works; lowercase `x`, library focus, empty lists, and blocked entries never delete.
- [x] No confirmation, trash, Git check, root deletion, or library write occurs. Preserve existing content on predictable preflight rejection; report stale/disappeared selections without substituting another row.
- [x] Run one mutation asynchronously, keep navigation responsive, reject busy/help mutation input, and wait on quit from the first enabled mutation. Success selects the following row or preceding last row; failure remains visible and refreshes actual partial state.

**Verification:** `go test -count=1 -run 'Test(RemoveSkill|RemoveFlow|MutationGuard)' .`, race check, and a disposable real remove/restart flow. Test target-internal symlink/special-file rejection and byte-for-byte unchanged library.

**Verified (2026-09-06):** Linux full tests/race/lint/build and disposable PTY remove/restart pass; add disabled. New native/manual checks pending.

**Dependencies:** Tasks 8, 9, and 12.

**Files likely touched:** `skills.go`, `skills_test.go`, `model.go`, `model_test.go`, `view.go`.

**Estimated scope:** Medium, 5 files; reuse established lifecycle and root validation.

### Task 14: Add A Fresh Skill

**Description:** A library row can now be copied into a missing skill target by its destination-specific letter without switching panels.

**Acceptance criteria:**
- [x] Wire all configured lowercase local and uppercase global slot keys from library focus only to captured requests, source preflight, destination revalidation/creation, independent recursive copy, and affected-panel refresh.
- [x] Copy nested directories, dotfiles, bytes, and executable bits subject to umask; reject links/special files, raw-name conflicts/case aliases, and invalid roots. Missing destinations are created only by an actual add.
- [x] Preserve library selection/focus after success, show the exact action target/result, and keep failure/busy/quit behavior from Task 13. Existing same-named targets remain explicitly unavailable until Task 15, never accidentally merged.

**Verification:** `go test -count=1 -run 'Test(AddSkill|AddFlow|CopyPermissions)' .`; compare independent file contents and source tree snapshots; run umask checks in subprocesses and test every slot's captured destination.

**Verified (2026-09-06):** Linux full tests/race/lint/build, 18 mappings, umask subprocesses, and PTY add/restart pass. Existing targets preserved; native case-alias execution pending.

**Dependencies:** Task 13.

**Files likely touched:** `skills.go`, `skills_test.go`, `model.go`, `model_test.go`, `view.go`.

**Estimated scope:** Medium, 5 files.

### Task 15: Replace An Existing Skill

**Description:** Complete the product's add contract by replacing an existing same-named skill using validated delete-then-copy, not synchronization.

**Acceptance criteria:**
- [x] Preflight the entire source and existing target before removal; reject links, special files, file conflicts, case collisions, and unsafe relationships without deleting old content.
- [x] Remove the old directory completely before copying; destination-only files and local edits disappear, even if content seems identical. A deletion failure prevents all subsequent copying into the remainder.
- [x] A later copy failure leaves truthful missing/partial output with no rollback, staging, backup, hidden success, or library changes; the user can retry or remove it through the existing UI.

**Verification:** `go test -count=1 -run 'Test(ReplaceSkill|ReplacePreflight|ReplaceFailure)' .`; manually replace a locally edited disposable skill and inspect destination-only file removal.

**Verified (2026-09-06):** Preflight/failure/retry tests and PTY replacement pass. Review fixes protect active config, retain destination identity across phases, allow distinct hardlinks, and expose pending quit in help.

**Dependencies:** Task 14.

**Files likely touched:** `skills.go`, `skills_test.go`, `model_test.go`.

**Estimated scope:** Medium, 3 files; reuse remove/copy paths without generalizing them into a framework.

### Checkpoint E: Tasks 13-15

- [x] Standard checks and race checks pass, including destructive preflight and partial-failure regression tests.
- [x] Complete add-three/remove-two/quit in a disposable project; restart confirms persistence and the library is unchanged.
- [x] Review captured targets, no-confirmation warnings, and delete-then-copy behavior before wider testing; this is a functional prototype, not a release-ready claim.

**Verified (2026-09-06):** Linux standard/build/race, ten PTY workflow repetitions, and macOS cross-builds pass; code review complete. Native Phase E/case-volume and human terminal checks pending; no push. Scan-to-operation identity audit remains Task 17.

**CI follow-up (2026-09-06):** Both Macs failed [33999023333](https://github.com/primaprashant/sei/actions/runs/33999023333): unsupported raw-name fixtures and `/var` aliases in hook/PTY expectations. Tests fixed with capability probes and Linux alias regressions; full/race/lint/vet, ten repetitions, and macOS cross-builds pass. Native rerun pending; production unchanged.

**Native verified (2026-09-06):** [CI 33999522026](https://github.com/primaprashant/sei/actions/runs/33999522026) passes all four Linux/macOS architecture jobs at `664a898`, including Linux race. Human terminal review and Task 17 cross-filesystem collision audit remain pending.

### Phase F: Failure And Concurrency Behavior

### Task 16: Recover From Operation Failures

**Description:** Make mid-operation errors usable and truthful across filesystem results, status display, and refreshed listings.

**Acceptance criteria:**
- [x] Inject/readily reproduce read, write, close, permission, and removal failures at deterministic points; assert missing/partial state is retained and a retry/removal works without touching the library.
- [x] Errors identify operation, captured skill, scope, and actual destination; navigation leaves them visible, while explicit actions can supersede them. Failed refresh does not hide the mutation error or present stale contents as current.
- [x] Unavailable library disables adds but permits otherwise-safe destination deletion; inaccessible destinations remain visibly distinct from empty/missing ones and unaffected panels remain usable.

**Verification:** `go test -count=1 -run 'Test(OperationFailure|ErrorPersistence|UnavailableRoots)' .`; use channels/completion signals rather than sleep-based fault timing.

**Evidence (2026-09-06):** Linux failure/recovery tests and standard/race checks pass on Go 1.27.1. Native macOS/manual acceptance pending.

**Dependencies:** Task 15.

**Files likely touched:** `skills.go`, `skills_test.go`, `model.go`, `model_test.go`, `view.go`.

**Estimated scope:** Medium, 5 files.

### Task 17: Reject Stale Filesystem State

**Description:** Exercise observed external changes between selection, scan, preflight, and mutation without pretending to provide concurrent-writer isolation.

**Acceptance criteria:**
- [x] Revalidate captured names/types/identities and root relationships at mutation time, including a newly created missing ancestor or retargeted configured-root alias; disappearance never redirects an action to a different row.
- [x] Detect observed entry replacement, new target children, links, and actual filesystem case aliases; abort safely where observed. Test case-insensitive target behavior, including collisions between nested source names, on an appropriate volume.
- [x] Coalesce/defer refresh while mutating and discard stale scan/operation results; a scan started before a mutation cannot resurrect deleted rows or erase the newest outcome.

**Verification:** `go test -count=1 -run 'Test(StaleSelection|ObservedChange|CaseCollision|ScanGeneration)' .`, native Linux/macOS tests, and race check. Verify skip reasons only for genuinely unsupported test filesystem features, with required cases covered elsewhere in the matrix.

**Evidence (2026-09-06):** Linux stale-state/generation tests pass; scans retain identities. Native macOS case-volume acceptance pending; case-sensitive targets now provision an isolated HFS+ volume.

**Dependencies:** Task 16.

**Files likely touched:** `skills.go`, `skills_test.go`, `model.go`, `model_test.go`.

**Estimated scope:** Medium, 4 files.

### Task 18: Wait For Work On Real Quit

**Description:** Prove the already-wired wait-on-quit behavior with a real process and a deterministic active operation, not just model-state tests.

**Acceptance criteria:**
- [x] Test `q`, Ctrl+C bytes, and repeated SIGINT during an active operation: navigation remains responsive before quit, finishing-before-exit is visible afterward, and no new mutation can start.
- [x] Do not cancel copy/delete or terminate the command worker on ordinary quit; release resources and restore the terminal only after the operation completes.
- [x] Use a test-only subprocess wrapper around production operation code with explicit start/release signals; no public delay/failure flags, fixed sleeps, or emergency second-Ctrl+C shortcut are added.

**Verification:** `go test -count=1 -run 'TestPTYQuitDuringMutation' .` on both OS families, `CGO_ENABLED=1 go test -race -count=1 ./...`, and manual busy-quit confirmation.

**Evidence (2026-09-06):** Six Linux PTY cases verify busy-quit completion and terminal restoration; the copy gate targets a specific file. Native macOS/manual acceptance pending.

**Dependencies:** Task 17.

**Files likely touched:** `main.go`, `model.go`, `view.go`, `model_test.go`, `process_test.go`.

**Estimated scope:** Medium, 5 files.

### Checkpoint F: Tasks 16-18

- [x] Standard, failure-injection, native case/permission, race, and active-operation PTY tests pass.
- [x] No destructive action is queued, replayed against a new selection, or canceled by normal quit.
- [x] Review residual external-writer/forced-termination limits and ensure help/docs do not promise a sandbox or recovery.

**Evidence:** Go 1.27.1 Linux standard/full/race checks, focused tests (10x; race 5x), build, macOS cross-builds, and limits review pass. Combined native macOS and manual acceptance pending.

**Native follow-up (2026-09-06):** [CI 34002263817](https://github.com/primaprashant/sei/actions/runs/34002263817) at `89ed161` passes all four native standard jobs and Linux race, completing Task 17/Checkpoint F native criteria. Human acceptance remains pending.

### Phase G: Complete Interaction Prototype

### Task 19: Preserve Failure Diagnostics On Exit

**Description:** Complete the subtle failure-after-quit path so the alternate screen cannot swallow the user's only diagnostic.

**Acceptance criteria:**
- [x] If active work fails after quit was requested, restore the terminal first, emit the sanitized captured target/failure to stderr, and return status `1`.
- [x] An in-TUI recoverable failure followed by a later ordinary quit returns `0`; fatal startup/runtime errors return `1`, and syntax errors remain `2`.
- [x] PTY/subprocess tests assert persistent stderr separately from rendered status and verify terminal restoration on both success and failure.

**Verification:** `go test -count=1 -run 'Test(PTYQuitFailure|ExitStatus)' .`; capture stdout/stderr/status independently and inspect a failed pending-quit run manually.

**Evidence (2026-09-06):** Linux standard/full/race/build, focused 10x/race 5x, and both macOS test cross-builds pass. Gated real partial-copy failures verify captured escaped stderr after restoration and later-quit `0`; cleanup errors retain operation diagnostics. Native Task 19 and human terminal review pending; Tasks 20-21 untouched.

**Dependencies:** Task 18.

**Files likely touched:** `main.go`, `cli.go`, `model.go`, `process_test.go`.

**Estimated scope:** Medium, 4 files.

### Task 20: Verify The Full Keyboard Contract

**Description:** Audit the integrated state machine now that every action exists, filling interaction regressions in the owning code rather than adding a separate input layer.

**Acceptance criteria:**
- [x] Cover every configured/unconfigured focus/add key, uppercase distinction, removal context, empty/blocked row, pending `g` with consumed continuation, Esc, help, and paste using table-driven model tests.
- [x] Test cross-state precedence: help/busy/pending-quit reject mutations, quit remains effective during a pending sequence, refresh is deferred, and no input captured while busy is later replayed.
- [x] Confirm initial first-row focus, independent panel selections, post-add stability, post-delete following/preceding selection, raw-name refresh preservation, and persistence only in the filesystem/config.

**Verification:** `go test -count=1 -run 'Test(KeyContract|InputPrecedence|Selection)' .`; complete the primary flow with one, three, and nine configured agents in isolated projects.

**Evidence (2026-09-06):** Audit required no production fix. Added all-count (1-9) key/context and cross-state tables; reused existing removal/guard tests and renamed raw-refresh coverage to `TestSelectionRefresh`. Real-filesystem 1/3/9-agent model flows add three to one local destination, remove middle/last, quit/reload config, and snapshot all disposable paths to exclude session persistence. Go 1.27.1 standard/full/race/build and both macOS test cross-builds pass; focused 10x/race 5x and existing add/restart/replacement PTY 10x pass. Exact 1/3/9 primary flows are model integration, not PTY; native Task 20, human terminal/SSH acceptance remain pending. No Task 21 work.

**Dependencies:** Task 19.

**Files likely touched:** `keys.go`, `model.go`, `model_test.go`, `view.go`.

**Estimated scope:** Medium, 4 files.

### Task 21: Review A Representative Prototype

**Description:** Put the working application in front of the owner with realistic data before fixing dimensions or announcing performance targets.

**Acceptance criteria:**
- [x] Prepare the owner-selected 25 actual upstream skills with pinned provenance/counts/bytes; keep supplemental long/Unicode/dot/nested test cases separate and commit no private skill content.
- [ ] Capture the three-agent screen at the owner's normal dimensions and candidate smaller/larger sizes; inspect full targets, help, failures, one-agent and nine-agent reachability in light/dark/no-color terminals and SSH.
- [ ] Measure an initial stripped CGO-disabled release-style build, not `go run`, for startup-to-populated-panels, input/render latency, and filesystem operations. Record observations and owner feedback without inventing pass/fail budgets.

**Verification:** `go test -count=1 -run 'TestRepresentativeFixture' .`; `CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o ./bin/sei .`; record machine/OS/filesystem/storage/terminal/build flags and artifact bytes in the prototype report.

**Evidence:** [Report, captures, measurements, and reproduction](docs/prototype.md): Linux checks and opt-in PTY matrix pass with the owner-selected source and geometries. Human/native Mac/SSH review and acceptance remain pending.

**Dependencies:** Task 20.

**Files likely touched:** `fixtures_test.go`, `performance_test.go`, `docs/prototype.md`.

**Estimated scope:** Medium, 3 files plus owner review.

### Checkpoint G: Tasks 19-21

- [x] Standard Linux checks and the automated full primary workflow pass with representative folders; human usability remains owner-reviewed.
- [ ] Owner reviews the actual three-agent prototype before final dimensions/overflow tests are fixed.
- [x] Record problems as concrete follow-up work; screenshots alone do not establish startup or copy performance.

**Evidence:** [Refreshed report](docs/prototype.md): P2 readiness/completion waits fixed, isolated 36-case measurements/captures regenerated, then focused/full/race/lint/build checks passed. Owner/native/SSH acceptance remains pending.

### Phase H: Evidence-Based Polish

### Task 22: Implement The Approved Layout

**Description:** Turn the reviewed prototype into a reliable desktop layout without silently choosing a minimum terminal size on the owner's behalf.

**Acceptance criteria:**
- [ ] Owner approves dimensions, panel overflow/focus visibility for 1-9 agents, full-target inspection, and a minimum terminal size based on Task 21 evidence; record exact decisions.
- [ ] Implement those choices while retaining library-left/global-above-local organization, exact key hints, visible scope/path/result, and text focus markers without a required font.
- [ ] Below the approved minimum, block new mutations and show a resize message while retaining quit support; resizing during work does not cancel it or re-enable input prematurely.

**Verification:** `go test -count=1 -run 'Test(Layout|Resize)' .`; manual resize before/during/after a mutation, help inspection, and focus to all nine agent slots.

**Dependencies:** Task 21 and explicit layout approval.

**Files likely touched:** `view.go`, `model.go`, `view_test.go`, `model_test.go`, `docs/prototype.md`.

**Estimated scope:** Medium, 5 files.

### Task 23: Set Measured Release Budgets

**Description:** Establish quantitative gates and demonstrate that the actual task beats the owner's existing shell workflow, without treating microbenchmarks as usability proof.

**Acceptance criteria:**
- [ ] Agree on a repeatable protocol and sample counts; separate first/cold runs from warm runs, input-to-render from I/O completion, and local from SSH. Report distributions, fixture characteristics, environment, flags, and artifact size.
- [ ] Time add-three/remove-two/quit against the current shell method on reset copies of the same fixture; record keystrokes, panel switches, mistakes, readability, and target certainty.
- [ ] Owner approves explicit startup/navigation/copy/replacement/removal/artifact budgets based on results. If workflow or budgets fail, identify and implement narrowly scoped follow-up fixes with regression tests before release; no arbitrary budgets or automatic scope expansion.

**Verification:** `go test -run='^$' -bench=. -benchmem -count=5 ./...` plus the recorded release-build process/PTY timing protocol and owner-run shell comparison. Benchmarks alone do not close this task.

**Dependencies:** Task 22 and owner availability.

**Files likely touched:** `performance_test.go`, `docs/performance.md`, `docs/prototype.md`.

**Estimated scope:** Medium, 3 files plus measurement/review; discovered optimization work becomes separate small tasks.

### Task 24: Harden Display Edge Cases

**Description:** Lock in a small set of representative view regressions and ensure unusual names cannot spoof the destructive action target.

**Acceptance criteria:**
- [ ] Escape terminal control characters in names, paths, agent labels, and error text; measure/truncate by terminal cells, not bytes or rune counts, preserving raw names for operations and inspectable full targets.
- [ ] Cover long names/paths, combining/wide Unicode, control-sequence filenames, empty/error/busy/help/no-color states, and the approved small-window fallback with representative deterministic snapshots.
- [ ] Confirm light/dark/no-color readability and no dependency on enhanced keyboard protocols, reliable repeat detection, special glyphs, or animation. Do not reject legitimate raw filenames merely because they need display escaping.

**Verification:** `go test -count=1 -run 'Test(View|DisplaySafety)' .`; inspect snapshot diffs and manual VTE/Terminal.app/SSH rendering. Re-run raw-name add/remove tests for hostile display text.

**Dependencies:** Task 23.

**Files likely touched:** `view.go`, `view_test.go`, `testdata/views.golden`, `model_test.go`.

**Estimated scope:** Medium, 4 files; use one small snapshot corpus, not a snapshot per styling detail.

### Checkpoint H: Tasks 22-24

- [ ] Standard checks and approved-size view tests pass; undersized windows cannot start mutations.
- [ ] Owner approves the layout and quantitative gates, with measured evidence of workflow improvement.
- [ ] Any observed usability/performance failure has a resolved follow-up before this checkpoint is marked complete.

### Phase I: Release Verification Coverage

### Task 25: Audit Destructive Coverage

**Description:** Turn the existing feature tests into a traceable safety gate with bounded fuzzing and reviewed coverage gaps.

**Acceptance criteria:**
- [ ] Add `FuzzParseConfig` and `FuzzPathName` with non-destructive invariants, seeded duplicate/path/name edge cases, no panics, and no escape from disposable roots; preserve discovered failures as regression inputs.
- [ ] CI runs one fuzz target/package per bounded job, existing race checks, and coverage reporting. Review mutation/validation/input-state gaps without imposing a global percentage threshold.
- [ ] Map every specified destructive and failure scenario to a regression test, including failed preflight preservation, failed removal stopping copy, partial copy, unchanged library, permissions/umask, links/special files, aliases, and stale selections.

**Verification:** `go test . -run='^$' -fuzz='^FuzzParseConfig$' -fuzztime=30s`; separately `go test . -run='^$' -fuzz='^FuzzPathName$' -fuzztime=30s`; `go test -coverprofile=coverage.out ./...`; `go tool cover -func=coverage.out`; full race check.

**Dependencies:** Tasks 17 and 20.

**Files likely touched:** `config_test.go`, `skills_test.go`, `.github/workflows/ci.yml`, `docs/test-matrix.md`.

**Estimated scope:** Medium, 4 files; missing behavior fixes must be split into their own bounded regression tasks if necessary.

### Task 26: Smoke-Test The Complete Process

**Description:** Extend the early PTY tests into a fresh-user end-to-end release gate against a built executable.

**Acceptance criteria:**
- [ ] Automate first-run setup, explicit setup/cancellation, actual focus/add/remove key bytes, refresh, help, paste rejection, resize, normal quit, restart persistence, and unchanged library under a PTY.
- [ ] Include deterministic busy-success and busy-failure quit tests with repeated interrupts, persistent stderr/status, and restored terminal state; keep model-only and process-level coverage distinct.
- [ ] Exercise with no live agent tooling and no application network access after dependencies/tools are prepared; test configuration isolation on both platforms and capture actionable CI diagnostics without sleep synchronization.

**Verification:** `go test -count=1 -run 'TestPTY' .` on the native matrix, standard checks, and manual SSH first-run/primary-flow/quit smoke tests recorded in the test matrix.

**Dependencies:** Tasks 19, 24, and 25.

**Files likely touched:** `process_test.go`, `.github/workflows/ci.yml`, `docs/test-matrix.md`.

**Estimated scope:** Medium, 3 files; reuse the Task 9 harness.

### Task 27: Build Downloadable Archives

**Description:** Package the working binary into the four actual release artifacts, without granting publication permission yet.

**Acceptance criteria:**
- [ ] Configure pinned GoReleaser Community for exactly Linux/macOS amd64/arm64, `CGO_ENABLED=0`, baseline CPU targets, stripped binaries, embedded version, `.tar.gz` archives, and SHA-256 checksums.
- [ ] Use a stable asset contract: `sei_<version-without-v>_<os>_<arch>.tar.gz` and `sei_<version-without-v>_checksums.txt`; archives contain `sei`, README, MIT license, and any required third-party notices, not user config or fixtures.
- [ ] Validate snapshot archives/checksums and run the extracted native binary's help/version; record exact tool/dependency provenance and verify runtime independence from Go/agent tools.

**Verification:** `goreleaser check`; `goreleaser release --snapshot --clean`; inspect archive contents/checksums and execute the matching extracted binary. `dist/` is disposable generated output only.

**Dependencies:** Tasks 3, 4, and 9; may proceed on a packaging branch before UI polish completes.

**Files likely touched:** `.goreleaser.yaml`, `cli.go`, `release_test.go`, `docs/release.md`, `THIRD_PARTY_NOTICES` if required by the dependency audit.

**Estimated scope:** Medium, at most 5 files.

### Checkpoint I: Tasks 25-27

- [ ] Standard, fuzz, race, coverage-audit, and full PTY checks pass at their documented scope.
- [ ] Four snapshot artifacts exist with verified checksums and no unexpected archive content.
- [ ] Test matrix links every destructive/failure requirement to evidence; unresolved mandatory coverage is a blocker.

### Phase J: Install A Verified Release

### Task 28: Validate The Support Floor

**Description:** Test actual release-style artifacts on promised architectures and proposed minimum systems instead of equating cross-compilation with support.

**Acceptance criteria:**
- [ ] Execute each exact archive's extracted binary natively on the four selected CI runners, asserting architecture/version, help, terminal enter/quit, and no separately installed application runtime. These early checks must not depend on copy/remove features that may not exist on the packaging branch yet.
- [ ] Run those early checks on the proposed Linux distro/kernel floor and macOS 13 on both architectures; capture exact machines/VM images, kernel/OS, filesystem, terminal, and SSH versions. Full feature/PTY verification on final artifacts remains mandatory in Tasks 34-36; early checks alone are not release-readiness evidence.
- [ ] Owner approves a truthful support table. Missing floor access blocks that claim and must be resolved by acquiring test access or explicit PRD/support adjustment, never silently skipped or inferred from newer hosts.

**Verification:** Run the early artifact checks above on the listed environments; keep evidence in `docs/release.md` and `docs/test-matrix.md`. Verify Linux kernel versions on the actual hosts, not merely container userspace. Schedule the Task 26 workflow against complete release artifacts in Tasks 34-36.

**Dependencies:** Task 27; complete workflow verification converges with Task 26 in Task 34 rather than blocking this early environment check.

**Files likely touched:** `release_test.go`, `.github/workflows/ci.yml`, `docs/release.md`, `docs/test-matrix.md`.

**Estimated scope:** Medium, 4 files plus environment access; scheduling risk is not hidden in a code estimate.

### Task 29: Approve Release Trust Policy

**Description:** Resolve macOS installation trust and release provenance before final installer/publication instructions are written.

**Acceptance criteria:**
- [ ] Test actual macOS download/extraction/execution paths, including quarantine/Gatekeeper behavior. Recommend Developer ID signing/notarization if required for the promised installation flow; record owner decision, credentials availability, and verified archive/notarization procedure. Never prescribe disabling security checks.
- [ ] Decide provenance explicitly: recommend GitHub artifact attestations for the final checksummed artifacts, explaining verification and same-release checksum limitations. Approve any `id-token: write`, `attestations: write`, or Apple credential usage before adding it.
- [ ] Pin ShellCheck `v0.11.0`, approve its verified installation, and document Go-based local installer tests. Record selected action SHAs and trust-tool versions in the release runbook. If signing is chosen, enumerate its implementation/test follow-ups as small tasks blocking Task 34.

**Verification:** Owner reviews the trust/permissions table; verify proposed tools against primary docs and test fresh-machine macOS execution without `xattr`/Gatekeeper bypasses. A checksum is not represented as publisher authentication.

**Dependencies:** Task 28 and owner security/credential decision.

**Files likely touched:** `docs/release.md`, `docs/test-matrix.md`.

**Estimated scope:** Small documentation/decision task, 2 files; any credential-backed implementation is separately scoped after the decision.

### Task 30: Install Into An Empty Directory

**Description:** Deliver a complete first-install path from a selected release artifact to a runnable user-owned binary.

**Acceptance criteria:**
- [ ] Implement POSIX `sh` installer host detection for the four supported targets and explicit `--version` / `--install-dir` options. Resolve latest once to a concrete release tag; download archive and matching checksums from that same tag.
- [ ] Verify exactly the selected archive's SHA-256 before extraction/installation, validate the expected archive layout, and install executable `sei` to a user-writable directory such as `~/.local/bin` without sudo or agent/Go prerequisites. Establish the version/digest receipt described above before the final binary commit; unrelated existing receipt files are conflicts, not overwrite permission.
- [ ] Report the installed version and absolute invocation; if not on PATH, print an exact correctly quoted follow-up command without editing shell profiles or claiming PATH is already set. Initially refuse an existing binary until Task 31 adds verified upgrades.

**Verification:** `sh -n scripts/install.sh`; `shellcheck -s sh scripts/install.sh` using the pinned binary; `go test -count=1 -run 'TestInstallerFresh' .` with local release fixtures/mock download tools on Linux and macOS.

**Dependencies:** Tasks 27 and 29.

**Files likely touched:** `scripts/install.sh`, `installer_test.go`, `docs/release.md`.

**Estimated scope:** Medium, 3 files.

### Checkpoint J: Tasks 28-30

- [ ] Native artifact/support-floor evidence is recorded; untested support claims remain blocked.
- [ ] Owner approves trust/provenance/credential decisions and any implementation follow-up tasks.
- [ ] First install works against isolated release fixtures with checksum verification and honest PATH guidance.

### Phase K: Upgrade And Document

### Task 31: Upgrade Without Losing A Working Binary

**Description:** Add upgrade support while deliberately using a safer replacement contract than skill-folder copying.

**Acceptance criteria:**
- [ ] Refuse unrelated existing files and symlink/directory targets. Match the existing executable digest to the Task 30 installer-owned receipt without executing that file; missing/mismatched receipts, manual installs, and ambiguous ownership are refused with manual guidance.
- [ ] Finish download, checksum/archive checks, extraction, permission checks, and new-binary validation in a temporary location on the installation filesystem before same-directory replacement of a recognized sei binary.
- [ ] Commit a receipt retaining both old and candidate digest records before replacing the binary; test failure between those commits. Any pre-binary-commit failure leaves the old binary byte-for-byte usable and recognizable; cleanup traps do not remove it, and post-commit messaging cannot falsely claim it was preserved.

**Verification:** `go test -count=1 -run 'TestInstallerUpgrade' .`; run lint/syntax checks and injected failures for write, permissions, validation, and final rename. Execute the old binary after each failed upgrade.

**Dependencies:** Task 30.

**Files likely touched:** `scripts/install.sh`, `installer_test.go`, `docs/release.md`.

**Estimated scope:** Medium, 3 files; extend the first-install receipt mechanism, not executable introspection.

### Task 32: Reject Broken Install Inputs

**Description:** Close the installer failure cases with local reproducible tests before exposing it to public download conditions.

**Acceptance criteria:**
- [ ] Test supported architecture aliases, unsupported OS/CPU, invalid options/version, missing utilities, failed/partial download, checksum mismatch/missing/duplicate entries, unavailable release, and changing latest pointers.
- [ ] Reject unsafe archive paths, links, special files, unexpected/duplicate executable entries, and malformed archives before installing; extraction never writes over live data or outside disposable staging.
- [ ] Cover custom paths with spaces/quotes, PATH present/absent, unwritable directories, unrelated existing files, cleanup/interruption, and failed upgrades on Linux/macOS POSIX shells. Do not add network overrides to the application config for test convenience.

**Verification:** `go test -count=1 -run 'TestInstaller' .`, `sh -n scripts/install.sh`, pinned ShellCheck; run tests against local/mocked downloads with no live GitHub dependency.

**Dependencies:** Task 31.

**Files likely touched:** `scripts/install.sh`, `installer_test.go`, `.github/workflows/ci.yml`.

**Estimated scope:** Medium, 3 files.

### Task 33: Document The Fresh-User Workflow

**Description:** Write accurate public instructions against the now-tested application and installer contracts.

**Acceptance criteria:**
- [ ] README covers the one-command and inspect-then-run installer paths, explicit version/custom directory, PATH follow-up, manual archive/checksum verification, receipt-based upgrades versus manual installs, and uninstalling the binary/installer receipt without deleting copied skills/config automatically.
- [ ] Explain native config locations, first-run/explicit setup, overrides/global-option ordering, presets and shared-discovery warning, exact shortcut table, resolved targets, refresh semantics, no network/telemetry, and no agent-session isolation.
- [ ] State delete-then-copy/local-edit loss, immediate permanent deletion, partial failures/forced termination, no undo, supported systems and measured limits, plus developer/test commands and a useful issue-report checklist. Include notices and no untested compatibility promises.

**Verification:** Follow every example in disposable fresh environments, compare keyboard/config docs against tests, run `git diff --check`, and verify release-relative URL construction. Mark public URLs pending until Task 36 verifies them live.

**Dependencies:** Tasks 23, 24, 28, 29, and 32.

**Files likely touched:** `README.md`, `docs/release.md`, `docs/test-matrix.md`.

**Estimated scope:** Medium, 3 documentation files.

### Checkpoint K: Tasks 31-33

- [ ] Installer lint/local integration tests pass, including preservation of an existing binary on failed upgrade.
- [ ] A new user can follow documentation without Go, agent tooling, sudo, profile edits, or security bypasses.
- [ ] Instructions distinguish persistent copied skills from removable installation files and explain all destructive risks.

### Phase L: Publish And Verify

### Task 34: Automate A Gated Draft Release

**Description:** Connect trusted tags to exact artifacts and required tests, keeping publication separate from ordinary CI and avoiding a build-after-test artifact mismatch.

**Acceptance criteria:**
- [ ] Add a separate full-history, SHA-pinned, exact-tool-version tag workflow. Validate full approved tag syntax rather than relying only on a loose glob; rerun required checks and complete all approved signing/notarization follow-ups.
- [ ] Build/sign/package once, checksum the final bytes, and transfer those exact artifacts to native smoke/installer tests. Only after all gates pass may a publishing job upload those same bytes to a draft release, with approved attestations tied to them.
- [ ] Limit `contents: write` and any approved trust credentials to protected release jobs; PR tests never receive them. Drafts include installer, four archives, checksums, notices, trust metadata, and release notes; failed verification never promotes a partial release to latest.

**Verification:** Validate release config and rehearse on an explicitly authorized candidate tag; inspect job permissions, full history/version metadata, artifact digests before/after upload, and all four native execution results. Do not tag/push merely because the workflow exists.

**Dependencies:** Tasks 26, 28, 29, and 32, plus completed conditional trust implementation tasks.

**Files likely touched:** `.github/workflows/release.yml`, `.goreleaser.yaml`, `docs/release.md`.

**Estimated scope:** Medium, 3 files; reuse existing installer and artifact tests.

### Task 35: Publish Pre-1.0 Development Releases

**Description:** Start public development releases at `v0.1.0` and iterate through subsequent `v0.x` versions as needed, exercising the complete distribution path before declaring `v1.0.0` the first usable release.

**Acceptance criteria:**
- [ ] With explicit owner authorization, publish `v0.1.0` as the initial development release and subsequent `v0.x` versions as needed. Mark these GitHub releases as prereleases, keeping them out of stable latest; test the tag-specific public installer URL and manual download instructions without GitHub authentication.
- [ ] On each supported architecture, install into a fresh user environment with no Go/agent runtime, complete setup/add-three/remove-two/quit, verify persistence/library invariance, and test successful upgrade plus a failed-upgrade fixture. Validate approved macOS trust behavior and support floors.
- [ ] Run the complete checks and measured budgets on the candidate build; obtain owner sign-off on usability, layout, support evidence, docs, and test matrix. Record any defect as a regression task and create a new candidate rather than replacing an already-published artifact invisibly.

**Verification:** Standard checks, race/fuzz/PTY/installer suites, `goreleaser check`, artifact checksum/provenance verification, all matrix smoke tests, and the Task 23 measurement protocol. Keep a release checklist with links to logs and owner approval.

**Dependencies:** Tasks 23, 33, and 34; explicit prerelease publication authorization.

**Files likely touched:** `docs/release.md`, `docs/test-matrix.md`.

**Estimated scope:** Medium verification task, 2 evidence files; waiting for native machines/owner review is an external dependency.

### Task 36: Publish The First Usable Release

**Description:** Publish `v1.0.0` as the first usable release and verify the public installation experience, rather than ending the project at a successful upload.

**Acceptance criteria:**
- [ ] Obtain explicit `v1.0.0` tag/push/publication approval. Build and verify the final-tag artifacts through the gated workflow; a final version rebuild must be retested even when its source matches the candidate. Publish all required assets and honest release notes, then promote `v1.0.0` to stable latest.
- [ ] Test the public stable one-command installer URL and tag-specific URL, version/custom directory, checksums/trust metadata, README links, fresh-user primary workflow, and upgrade path on the four architecture targets. Verify the final version reported by the installed binary.
- [ ] Record tag, commit, artifact digests, URLs, checks, support/budget results, and approval. If publication/install fails, stop advertising it and publish a corrected version through the same gates; do not disable checks, rewrite published tags, or promise automatic rollback.

**Verification:** Use `gh` to inspect release assets/workflow results and verify unauthenticated public download URLs; repeat final artifact smoke/installer checks and a fresh first-run workflow. Close the release checklist only after post-publication verification passes.

**Dependencies:** Task 35 and explicit final publication authorization.

**Files likely touched:** `README.md`, `docs/release.md`; GitHub release metadata/assets through the reviewed workflow.

**Estimated scope:** Medium release/verification task, 2 files plus authorized remote operations.

### Checkpoint L: Tasks 34-36

- [ ] Public release contains the tested final artifacts and the one-command installer actually works.
- [ ] All PRD success criteria are evidenced, including owner-reviewed workflow improvement and post-publication native checks.
- [ ] Return the release URL, installation entry point, supported systems, and any explicitly documented limitations to users.

## Requirement Traceability

| PRD Requirement Group | Primary Tasks |
| --- | --- |
| Repository, stack, style, basic configuration | 1-4 |
| Strict config, native paths, project overrides, setup | 5, 8, 10-12 |
| Real folder listings, missing/error/blocked states | 6-8, 16 |
| Exact shortcuts, selection, help, input safety | 7, 13-14, 20, 22, 24 |
| Never mutate library; containment, aliases, no links/special files | 8, 12-17, 25 |
| Destructive full-copy/replacement and independent deletion | 13-17 |
| Async ownership, busy rejection, stale scans, refresh | 6, 13-20 |
| Quit waiting, restoration, stderr/status, real PTY | 9, 18-19, 26 |
| Representative layout, real usability, measured budgets | 21-24, 35 |
| Unit/integration/failure tests, fuzz/race/coverage/native CI | Every feature task; 4, 25-26 |
| Four runtime-free artifacts and truthful support floor | 27-29, 34-36 |
| Safe one-command installation/upgrade and supply-chain disclosures | 29-36 |
| No network during normal operation, no scope expansion | 6-26, 33, 35 |
| PRD success criteria 1-3 | 5, 10-12, 28, 30-36 |
| PRD success criteria 4-7 | 7-9, 13-20, 22, 24, 26 |
| PRD success criterion 8 | 3-4, all feature tests, 25-26, 34-36 |
| PRD success criteria 9-10 | 21-24, 35-36 |

## Parallel Work

- Tasks 3-4 can progress independently of Task 5 after Task 2, with one owner for shared README changes.
- Task 8's root-safety implementation can progress alongside Tasks 6-7 only after agreeing on resolved-root/error types. Both touch `skills.go` indirectly; coordinate file ownership rather than letting agents overwrite each other.
- After Task 9, the packaging/support branch (27-29) can run alongside application slices. Discover missing macOS floor machines and signing access early, not at the final tag.
- Tasks 30-32 installer work can run alongside Tasks 21-26 once asset/trust contracts are settled; coordinate `.github/workflows/ci.yml` and release documentation edits.
- Keep Tasks 13-20 sequential: they share `skills.go`, `model.go`, lifecycle invariants, and mutation behavior. Do not parallelize copy/remove implementations against changing safety contracts.
- Documentation, independent test-matrix review, and primary-source verification are safe parallel work when their implementation contracts are stable. Release promotion remains sequential after every required branch converges.

## Decision Gates

Repository identity, MIT licensing, and the release-version policy are confirmed, and the owner has approved the plan. The following task-specific decisions and execution authorizations remain explicit gates; plan approval does not claim that future prototype reviews or publication approvals have already occurred.

**Approved (2026-09-05):** PTY v1.1.24/existing terminal helper; inspection with affected unsafe mutations blocked; SIGINT/SIGTERM/SIGHUP wait-and-restore (HUP best-effort); reject config symlink writes/managed-root placement without creating destinations. Config-write enforcement remains Phase D; ShellCheck approval remains later.

| Gate | Recommendation / Question | Resolve Before |
| --- | --- | --- |
| Copyright and tag execution | Confirm copyright attribution and record the approved `v0.1.0` onward / first-usable `v1.0.0` policy. | Task 1 metadata; individual tags still need explicit execution authorization. |
| New test tooling | Approve test-only PTY dependency and resolved terminal helper; approve pinned ShellCheck. No new application framework or installed-user runtime. | Task 9 / Task 29 respectively. |
| Unverifiable root safety | Keep inspection usable; allow removal with unavailable library contents if root relationships remain provable. Block only affected mutations when aliases/permissions make safety unknowable. | Task 8 safety review. |
| Config write placement | Recommend rejecting config-file symlinks for writes and config saves inside any managed library/destination root; preserve read-only loading where safe. Config-parent creation must not create skill destinations. Clarify before implementing because PRD does not specify config symlink/placement semantics. | Task 10 saving, verified in Task 12. |
| Non-key interruptions | Recommend wait-and-restore for catchable SIGINT/SIGTERM, with documented best-effort SIGHUP/disconnected-terminal behavior; SIGKILL remains forced termination. Ctrl+C wait behavior is already fixed. | Task 9 lifecycle implementation. |
| Layout and minimum | Approve exact three-agent dimensions/overflow and behavior for 1-9 agents using actual prototype evidence. No number is assumed here. | Task 22. |
| Quantitative budgets | Approve measurement protocol, sample counts, latency/artifact limits, and observed improvement over shell workflow. | Task 23. |
| Native support floor access | Approve/test proposed Ubuntu 22.04/5.15 floor; arrange macOS 13 amd64/arm64 access or explicitly revise an unsupported claim. | Task 28, begin arranging during Task 4. |
| Signing/notarization/provenance | Decide using clean-machine evidence; approve credentials/permissions and enumerate small implementation follow-ups if needed. | Task 29; blocks Task 34. |
| Public remote actions | Confirm repository creation/push, candidate publication, and final publication separately. This plan itself grants none of those actions. | Tasks 1, 35, 36 as applicable. |

## Risks And Mitigations

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Destructive target aliases or stale paths | High, unintended data loss | Rooted operations plus physical overlap/name/type preflight, repeat validation, native case/alias regressions before enabling mutations. |
| Assuming rooted operations reject all symlinks | High | Explicit tree policy checks and observed identity checks; document hostile-writer and mount limits. |
| Bubble Tea exits while a command still mutates | High | Early source/API review and real PTY signal tests; one lifecycle owner and explicit wait completion. |
| Installer failure destroys an old binary | High | Same-filesystem staging, conservative ownership identification, commit only after verification, fault tests asserting old bytes/execution. |
| Config writing violates read-only library safety | High | Resolve placement/symlink policy before setup saving; validate save target independently of skill targets. |
| Config strictness relies on permissive Go defaults | Medium | Token-level duplicate/schema validation before typed decoding, null/type tests, fuzzing. |
| UI grows into nine unreadable columns | Medium | Early real-data prototype, owner-approved overflow, guaranteed focus reachability, resize mutation guard. |
| Missing floor hardware or Apple credentials | High for publishing | Start access checks in early CI work; explicit release gate rather than unsupported compatibility/trust claims. |
| Tools/runner labels evolve | Medium | Recheck primary sources at kickoff, pin reviewed versions/SHAs, capture actual image versions, never hide missing jobs. |
| Too many concerns in one task | Medium | Default S/M tasks with at most about five files; split if work exceeds one focused session, retaining integrated feature verification. |

## Planning Sources

- [Product vision](product-vision.md) and [PRD](prd.md), read in full for this plan.
- Local Planning and Task Breakdown skill, used for task structure, vertical slices, dependencies, sizing, and checkpoint cadence.
- [Go traversal-resistant APIs](https://go.dev/blog/osroot) and [Go 1.27.1 os documentation](https://pkg.go.dev/os@go1.27.1), for rooted access choices and limitations.
- [Bubble Tea v2.0.9 lifecycle source](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tea.go), for interrupt and command lifecycle verification tasks.
- [GitHub-hosted runner reference](https://docs.github.com/en/actions/reference/runners/github-hosted-runners), for explicit native runner selection.
- [ShellCheck v0.11.0 release](https://github.com/koalaman/shellcheck/releases/tag/v0.11.0), for the pinned installer-lint proposal.
- Recheck the PRD's primary dependency, agent-preset, GoReleaser, and macOS platform sources at implementation kickoff; research observations are not build or release evidence.

## Plan Approval

- [x] Owner has reviewed and approved this plan, including the corrected release-version policy. Implementation has not started.
- [ ] Every task has explicit acceptance criteria, verification, dependencies, and bounded file scope.
- [ ] Checkpoints occur after each three tasks; safety and terminal risks are addressed before destructive workflows.
- [ ] Tests ship with vertical feature slices; packaging and support-access work can proceed early in parallel.
- [ ] Open owner decisions are gated at the point they matter; no deferred prototype measurement is represented as an already-set release budget.
- [ ] Publication ends with verified public installation, not merely a tag or uploaded archive.
