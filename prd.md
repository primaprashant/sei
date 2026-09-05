# Spec: sei

## Objective

Build a keyboard-first terminal tool that copies and removes skill folders between one personal library and explicitly configured agent destinations. The primary user works with multiple AI coding agents, prefers the terminal, and wants to make only selected skills available without repeatedly typing filesystem paths.

This PRD refines [product-vision.md](product-vision.md) and contains the product requirements, stack decision, and engineering baseline, not an implementation sequence. Product decisions were confirmed with the owner on September 5, 2026. No application code or tooling configuration is created as part of this PRD; configuration snippets below are implementation requirements.

### Primary Workflow

1. Run `sei` in a project; on first use, complete interactive setup.
2. See the library on the left, configured global destinations at the top right, and corresponding project-local destinations below.
3. Select a library folder and use a destination-specific letter to copy it. Keep focus and selection on that library row.
4. Add three skills to one local destination without repeatedly switching panels or entering paths.
5. Focus that destination, remove two skills, and quit. Filesystem changes persist.

### Scope

- One configurable, flat library, initially optimized around 20-30 skill folders.
- One to nine ordered agent configurations. Initially select Claude Code, Codex, and OpenCode in setup; offer Pi and Cursor as additional presets. Users can select fewer or more and configure custom agent names and destinations.
- Each configured agent has one user-global and one project-local destination. Configuration order determines panel numbers and add keys.
- Linux and macOS, amd64 and arm64, locally and over SSH. No agent executable, Go installation, special font, or graphical environment is required to run a released binary.
- Permanent single-folder copy/replacement and deletion, with no confirmation for either operation. Setup confirmation concerns configuration only, not skill mutations.
- No agent launching, session management, automatic cleanup, skill editing, synchronization, filtering, multi-select, tags, categories, content previews, remote downloads, or loaded-skill inspection.
- No network access during normal setup or TUI operation. Installation is a separate network-using operation. No telemetry or automatic update checks.

### Setup And Configuration

`sei` starts setup only when the chosen configuration file does not exist. An unreadable or malformed existing file is an error, not permission to overwrite it. `sei setup` explicitly opens setup, prepopulated from a valid existing configuration when present.

Setup collects the library path, selected agents in order, and their global/local destinations. Offer documented editable presets, show resolved paths and shortcuts before saving, and explain that replacement loses local edits and deletion is permanent. Configuration replacement requires confirmation. Cancelling leaves the existing configuration and skill folders unchanged; simply configuring or viewing a destination does not create it. After successful first-run setup, enter the TUI; explicit `sei setup` saves and exits.

Use pretty-printed JSON and `os.UserConfigDir()`:

| Platform | Default Configuration File |
| --- | --- |
| Linux | `$XDG_CONFIG_HOME/sei/config.json`, otherwise `~/.config/sei/config.json` |
| macOS | `~/Library/Application Support/sei/config.json` |

`--config` overrides the configuration file. Do not introduce project-local config discovery, multiple merged config layers, JSONC, or an environment-variable configuration system in v1. Reject a relative `XDG_CONFIG_HOME` on Linux instead of resolving it against the project. Go's native macOS config-directory behavior does not use that variable.

Initial configuration shape, with an illustrative library path that setup replaces with the user's choice:

```json
{
  "library": "~/Personal Agent Skills",
  "agents": [
    {
      "name": "Claude Code",
      "global": "~/.claude/skills",
      "local": ".claude/skills"
    },
    {
      "name": "Codex",
      "global": "~/.agents/skills",
      "local": ".agents/skills"
    },
    {
      "name": "OpenCode",
      "global": "~/.config/opencode/skills",
      "local": ".opencode/skills"
    }
  ]
}
```

- Use strict decoding: reject unknown fields, incorrect types, trailing JSON values, empty required fields, duplicate agent names, and agent counts outside 1-9. Do not silently accept duplicate JSON object keys; include that case in parser tests.
- Library and global paths must be absolute or start with `~/`. Expand only the current user's home shorthand, not environment variables, shell commands, globs, or `~otheruser`. Spaces and Unicode are valid path content.
- Local paths must be relative to the project base and must not escape it. The project base is the launch working directory, not an automatically discovered Git root. `--project` explicitly overrides it; relative overrides are resolved from the launch directory.
- Persist destinations and order, not the last project's absolute path, focused panel, selection, or session state. Editing JSON and restarting is supported; `r` does not reload configuration.
- Save configuration using a temporary file and replacement so a failed save does not truncate a valid existing file. This protection for configuration does not change the destructive skill-copy contract.

Documented preset paths, verified September 5, 2026:

| Agent | Global | Project-Local | Primary Documentation |
| --- | --- | --- | --- |
| Claude Code | `~/.claude/skills` | `.claude/skills` | [Claude Code skills](https://code.claude.com/docs/en/skills#where-skills-live) |
| Codex | `~/.agents/skills` | `.agents/skills` | [Codex skills](https://developers.openai.com/codex/build-skills#where-codex-loads-local-skills) |
| OpenCode | `~/.config/opencode/skills` | `.opencode/skills` | [OpenCode skills](https://opencode.ai/docs/skills/#place-files) |
| Pi | `~/.pi/agent/skills` | `.pi/skills` | [Pi skills](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/skills.md#locations) |
| Cursor | `~/.cursor/skills` | `.cursor/skills` | [Cursor skills](https://cursor.com/docs/context/skills#skill-directories) |

These are presets, not exhaustive discovery implementations. Customized agent environments may require different paths; sei does not inspect agent installations or settings. In particular, do not substitute macOS's sei configuration convention for OpenCode's documented skill path.

**Folder labels do not imply isolation.** Codex's `.agents/skills` destinations can also be read by OpenCode, Pi, and Cursor. Other agents also read some `.claude/skills` locations. Setup and help must state: "Other agents may also load skills from these folders. sei shows configured folder contents, not everything an agent discovers or has loaded."

### Keyboard And Display Contract

| Input | Behavior |
| --- | --- |
| Up / Down | Move within the focused list; stop at the ends rather than wrap. |
| `0` | Focus the library. |
| `1` through `9` | Focus the corresponding configured agent's local panel. |
| `g`, then `1` through `9` | Focus the corresponding global panel; these are two ordinary key presses. |
| `a b c d e f h i o` | From the library only, add to local destination slots 1-9 respectively. |
| `A B C D E F H I O` | From the library only, add to global destination slots 1-9 respectively. |
| `X` | From a destination only, permanently remove the selected ordinary skill directory. Lowercase `x` does nothing. |
| `r` | Refresh configured folder listings; no watcher or config reload. |
| `?` | Toggle expanded help, including the full focused destination path. |
| `Esc` | Close help or cancel a pending key sequence; otherwise do nothing. |
| `q` / `Ctrl+C` | Quit immediately when idle; when mutating, request exit after the active operation completes. |

The initial three agents therefore use `a/A`, `b/B`, and `c/C`. Positional keys deliberately skip `g` and other reserved controls. Key customization and Vim-style movement bindings are not v1 requirements.

- Start focused on the first library row. Remember each panel's selection during the current run. Use deterministic, case-sensitive Go string ordering of raw folder names, not locale-dependent sorting or case folding.
- Show the selected folder, focused panel, agent name, global/local scope, destination path, and last action result. A pending `g` must be visible and has no timeout. `Esc` cancels; an invalid continuation cancels and is consumed, not reinterpreted as an add/delete. Quit keys remain effective. Unconfigured numeric/letter slots do not mutate anything.
- Each panel header shows its focus shortcut. Library add hints show exact agent/scope mappings for the configured slots; do not require shortcut memorization.
- A successful add leaves library selection and focus unchanged. After deletion, select the following row, or the preceding row if the last row was removed. Empty panels have no actionable selection. Refresh preserves selection by raw name when it still exists, otherwise clamps to a valid row.
- A busy operation is bound to the captured skill and destination, not whatever is selected when it completes. Keep navigation responsive. Reject additional mutation input with a visible busy state; do not queue destructive actions. Defer refresh requests until the mutation finishes.
- While help is open, do not accept skill mutations. Display a finishing-before-exit state after a quit request and reject further mutations. Restore terminal state after success, error, or normal interruption. If the in-flight operation fails after quit was requested, print its target and failure summary to stderr after restoration and exit with status `1`; do not lose the diagnostic with the alternate screen.
- Show errors with operation, skill, scope, and destination. Refresh the affected destination even on failure so partial state is not mistaken for success. Keep errors visible until dismissed or superseded by an explicit action; navigation alone must not erase them.
- List every immediate ordinary subdirectory, including dot-directories, without requiring `SKILL.md` or parsing frontmatter. Ignore loose regular files in listings. Show symlink entries as blocked, with a reason; do not follow or delete those entries. No recursive skill discovery.
- Names and paths are untrusted display text: escape terminal control characters, keep raw names for filesystem operations, and truncate using terminal cell width. A truncated label must not be the only way to inspect the full action target.
- Use text labels and focus markers as well as color; support light/dark terminals and a readable no-color presentation. No required patched fonts or distracting animation. Ignore paste events as action input, and do not depend on enhanced keyboard protocols or reliable key-repeat detection.
- Prototype the default three-agent layout before choosing dimensions, overflow behavior, or a minimum supported terminal size. One to nine agents must remain addressable, but nine simultaneously comfortable columns are not required. Below the eventually agreed minimum, disable new mutations and show a clear resize message while retaining quit support.

### Filesystem Contract

**Add is destructive replacement, not synchronization.** Validate the selected name, source tree, configured roots, and target type; remove an existing same-named destination directory; then copy the whole source folder. Include nested directories and hidden files. Remove destination-only files and local edits. Do not merge, skip an apparently identical copy, ask for confirmation, check Git, stage a skill copy, back up, or roll back.

**Failure may leave no destination or a partial destination.** The owner explicitly chose delete-then-copy simplicity over staged replacement. Predictable validation failures must occur before deleting existing content, but later read, write, disk-space, permission, or removal failures may still be destructive. Report the failure accurately, retain any partial state, and let the user retry or remove it. Do not claim transactions, crash recovery, or portable atomic replacement.

- Never mutate the source library. Confine mutation to a validated immediate skill child of a configured destination; never delete the destination root, project root, home directory, or a library ancestor.
- Reject identical or nested source/destination roots and overlapping destination roots, including aliases through existing symlinks. Validate the resolved project-local paths each launch. For absent destinations, validate existing ancestors and revalidate at mutation time, not just by comparing textual prefixes.
- Configured root paths may themselves resolve through symlinks, provided containment and overlap checks pass. Symlinked skill entries, links anywhere inside a skill, and special files are unsupported. Preflight the source and any existing target tree before destructive copy or deletion; block unsupported entries rather than following them. A regular file occupying the same-named target is a conflict, not permission to overwrite it.
- Preserve file contents and executable permission bits subject to the user's umask. Do not promise preservation of ownership, timestamps, ACLs, extended attributes, hard-link identity, or special permission bits. Create independent files, not links back to the library.
- Missing destinations display as empty with a not-created indicator and are created on first add. An inaccessible destination displays an error, not an empty listing; unaffected panels remain usable. An unavailable library disables adding but does not prevent inspecting/removing supported destination folders.
- Removal deletes the selected destination directory even if it does not exist in the library. There is no trash integration or undo. Revalidate stale selections and types immediately before mutation; if the selected item disappeared, refresh and report it rather than operating on a different row.
- On case-insensitive filesystems, do not silently replace a differently cased name. Detect name collisions and report them. Preserve raw names and use platform filesystem behavior rather than pretending every supported filesystem is case-sensitive.
- Run one mutation at a time within sei. Normal quit waits for it, including repeated `Ctrl+C`; forced process termination can leave partial work. No multi-process locking, consistent snapshot of a concurrently edited source, or isolation from simultaneous external writers is promised in v1. Detect observed changes and fail safely where possible; do not market this as a sandbox against hostile filesystem actors.

## Tech Stack

### Decision And Rationale

Use **Go + Bubble Tea v2 + Lip Gloss v2 + Bubbles v2**. Prioritize responsive terminal interaction, correct filesystem operations, a self-contained executable, and simple development, maintenance, and cross-platform releases. The maintainer is very comfortable with Python and comfortable with Go.

- Bubble Tea's centralized update model fits the multi-panel UI and explicit keyboard/focus state; Lip Gloss and Bubbles support a polished interface without required fonts.
- Go offers a favorable startup/footprint baseline and straightforward per-platform executables. Users need neither a language runtime nor agent tooling installed, and binary installers can provide the primary installation flow.
- Python/Textual offers greater language familiarity but adds interpreter/import overhead and less direct standalone packaging. Fast `uv` installation does not eliminate application startup costs.
- Rust/Ratatui adds learning and build complexity without a demonstrated need. TypeScript/OpenTUI/Ink adds runtime and packaging overhead. Go/tview remains viable, but Bubble Tea's update model better suits these custom interactions.
- Performance advantages are expectations, not measured guarantees. Verify startup, navigation, filesystem responsiveness, and artifact size with representative release builds before setting budgets; never trade correctness for speed.

### Versions

Versions below were observed from primary upstream sources on September 5, 2026. Pin the selected versions in module/CI/release configuration during implementation; recheck patch releases and compatibility then. These are researched selections, not a claim that this docs-only repository has built or tested them.

| Component | Baseline | Purpose |
| --- | --- | --- |
| Go | `1.27.1` | Language, standard library, compiler, testing tools. |
| Bubble Tea | `charm.land/bubbletea/v2 v2.0.9` | Terminal runtime, explicit model/update/view loop. |
| Lip Gloss | `charm.land/lipgloss/v2 v2.0.6` | Terminal styling and layout. |
| Bubbles | `charm.land/bubbles/v2 v2.2.1` | Use applicable components such as key/help; do not force the UI into a feature-heavy list component. |
| golangci-lint | `v2.13.2` | Small static-analysis ruleset and bundled `goimports` formatter. |
| GoReleaser Community | `v2.18.0` | Versioned release archives and checksums. |
| GitHub Actions | SHA-pinned actions; exact revisions selected in implementation | Tests and tag-triggered releases. |

Use standard-library `flag`, `encoding/json`, `os`, `io/fs`, `path/filepath`, and `testing` before adding alternatives. This small command surface does not require Cobra, Viper, a dependency-injection framework, a database, or a generic filesystem abstraction. JSON duplicate-key validation must be deliberate because `encoding/json` does not reject duplicates by default.

### Current API Guidance

- Charm v2 uses `charm.land/.../v2` imports. Do not copy v1 examples unchanged: key presses use `tea.KeyPressMsg`, and the model's `View` returns `tea.View`.
- Perform scans/copies/deletes in `tea.Cmd` work and send typed result messages back. Only `Update` owns model mutation; keep blocking I/O out of `Update` and `View`. A command closure captures immutable request values, not a model pointer it later mutates.
- `tea.Batch` is concurrent and unordered. Do not batch delete/copy/refresh as independent commands. A destructive operation's sequential filesystem work belongs to one command, with refresh driven by completion and stale results ignored.
- `os.CopyFS` is not an overwrite operation, can leave partial output, and can preserve source symlinks on current Go. Existing destination symlinks can be followed. Do not assume it enforces this product's no-links or containment policy.
- Evaluate current `os.Root` APIs for confined filesystem operations. They help constrain traversal but do not themselves reject all symlinks, special files, or overlapping configured roots. Select and test the exact filesystem mechanism during implementation planning.
- No speculative concurrency optimization. Async UI work is required for responsiveness; parallel mutations, worker pools, and precomputed caches are not.

### Distribution Baseline

Build explicit `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64` artifacts with `CGO_ENABLED=0`, provided the resolved dependencies permit it. Use baseline CPU targets, stripped release binaries, `.tar.gz` archives, and SHA-256 checksums. Go 1.27 sets a macOS 13 minimum; document it. Determine the tested Linux distribution/kernel floor in the release plan rather than claiming every Linux version works.

Provide a one-command Unix installer that detects supported OS/architecture, downloads the matching versioned artifact, verifies its checksum before installation, and installs to a user-writable location such as `~/.local/bin`. Support explicit version and installation-directory selection. Do not automatically invoke `sudo`, overwrite unrelated files, edit shell startup files, or disable macOS security checks. If the directory is not on `PATH`, give an exact follow-up instruction and an immediately usable absolute invocation; do not claim PATH is already configured. Installation and upgrade failures must leave an existing binary usable.

The installer URL depends on the repository identity and is intentionally not invented here. A checksum fetched from the same release protects against corruption, not a compromised release publisher. Decide signing/notarization and provenance details during release planning. Homebrew, Windows, self-update, and package-manager submissions are deferred. Current GoReleaser guidance favors Homebrew casks over its deprecated formula integration, so research that choice again if Homebrew becomes a requirement.

### Research Sources

- [Go release feed](https://go.dev/dl/?mode=json), [release history](https://go.dev/doc/devel/release), and [Go 1.27 release notes](https://go.dev/doc/go1.27): verified Go version and platform floor.
- [Official module layout](https://go.dev/doc/modules/layout): root commands and optional `internal`/`cmd` organization; no universal multi-layer directory template.
- [Google Go Style Guide](https://google.github.io/styleguide/go/guide), [style decisions](https://google.github.io/styleguide/go/decisions), and [best practices](https://google.github.io/styleguide/go/best-practices): readability and least-mechanism principles.
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments), [Go doc comments](https://go.dev/doc/comment), and [Effective Go](https://go.dev/doc/effective_go): idioms and documentation. Effective Go explicitly is not a complete guide to modern Go.
- [Tool dependencies](https://go.dev/doc/modules/managing-dependencies#tool-dependencies) and [toolchain selection](https://go.dev/doc/toolchain): modern tool pinning and the distinction between minimum and exact Go versions.
- [golangci-lint changelog](https://golangci-lint.run/docs/product/changelog/), [v2 configuration](https://golangci-lint.run/docs/configuration/file/), [CLI](https://golangci-lint.run/docs/configuration/cli/), and [installation guidance](https://golangci-lint.run/docs/welcome/install/local/): selected version, formatter placement, and binary installation.
- [Bubble Tea v2.0.9](https://github.com/charmbracelet/bubbletea/releases/tag/v2.0.9), [v2 upgrade guide](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/UPGRADE_GUIDE_V2.md), [Lip Gloss v2.0.6](https://github.com/charmbracelet/lipgloss/releases/tag/v2.0.6), and [Bubbles v2.2.1](https://github.com/charmbracelet/bubbles/releases/tag/v2.2.1): released v2 stack, not prerelease or v1 examples.
- [Bubble Tea commands](https://charm.land/blog/commands-in-bubbletea/) and [v2 command implementation](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/commands.go): asynchronous work and ordering; the older article is conceptual, not a source of current signatures.
- [Go os documentation](https://pkg.go.dev/os@go1.27.1), [Go 1.25 os changes](https://go.dev/doc/go1.25#os), and [traversal-resistant file access](https://go.dev/blog/osroot): current copy/link semantics and containment caveats.
- [Go test guidance](https://go.dev/wiki/TestComments), [race detector](https://go.dev/doc/articles/race_detector), and [fuzzing](https://go.dev/doc/security/fuzz/): standard testing approach and limitations.
- [GoReleaser v2.18.0](https://github.com/goreleaser/goreleaser/releases/tag/v2.18.0), [Go builds](https://goreleaser.com/customization/builds/builders/go/), [GitHub Actions](https://goreleaser.com/customization/ci/actions/), and [Homebrew casks](https://goreleaser.com/customization/publish/homebrew_casks/): release tooling and distribution tradeoffs.

## Commands

### User Interface

These are the required command contracts once implemented. `--help` and `--version` work without configuration or a terminal. Reject unknown commands, flags, and unexpected positional arguments. Use global options before the optional subcommand; document that ordering rather than implementing multiple parsing conventions.

```sh
sei
sei --project "$HOME/work/example"
sei setup
sei --config "$HOME/sei-test.json" setup
sei --config "$HOME/sei-test.json" --project "$HOME/work/example"
sei --help
sei --version
```

Use stdout for requested help/version output and stderr for startup/usage errors. TUI and setup require interactive stdin/stdout; non-TTY invocation fails with a clear message rather than hanging or emitting screen-control sequences. Exit `0` on normal quit or setup cancellation, `1` on startup/configuration/setup-save failures, fatal runtime errors, or an operation failing after a pending quit request, and `2` on invalid CLI syntax. Recoverable in-TUI operation errors remain visible without terminating the program; normal later quit still exits `0`. Do not add headless add/remove commands in v1.

### Developer Workflow

Commands run from the repository root after the initial module and source files exist. The actual module path depends on the publication repository and must be settled before `go mod init`; no fictional import path is prescribed. Contributors need Go 1.27.1; the race job additionally needs a supported host and C compiler. Release commands require GoReleaser 2.18.0 on `PATH`.

Initial dependency selection, after module initialization:

```sh
go get go@1.27.1
go get charm.land/bubbletea/v2@v2.0.9 charm.land/lipgloss/v2@v2.0.6 charm.land/bubbles/v2@v2.2.1
go mod tidy
```

Commit `go.mod` and `go.sum`. The `go` directive sets a minimum, not an exact toolchain lock. Pin CI's Go version and use this environment setting for matching local verification:

```sh
export GOTOOLCHAIN=go1.27.1
go version
```

Install the pinned linter binary using its upstream installer, not `go install` into the application's dependency graph:

```sh
mkdir -p .bin
curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b ./.bin v2.13.2
./.bin/golangci-lint version
./.bin/golangci-lint config verify --config .golangci.yml
```

The implementation bootstrap should document reviewing the installer or using upstream release binaries directly; CI must pin the binary and verify release integrity. `.bin/`, `bin/`, `dist/`, and coverage artifacts are ignored, not committed.

| Task | Full Command |
| --- | --- |
| Develop | `go run . --project .` |
| Exercise setup | `go run . setup` |
| Build locally | `go build -o ./bin/sei .` |
| Format in place | `./.bin/golangci-lint fmt --config .golangci.yml` |
| Inspect formatting diff | `./.bin/golangci-lint fmt --config .golangci.yml --diff` |
| Check lint and formatting | `./.bin/golangci-lint run --config .golangci.yml --timeout=5m ./...` |
| Check module hygiene without edits | `go mod tidy -diff` |
| Test | `go test -count=1 ./...` |
| Vet explicitly | `go vet ./...` |
| Detect exercised data races | `CGO_ENABLED=1 go test -race -count=1 ./...` |
| Coverage | `go test -coverprofile=coverage.out ./...` |
| Read coverage | `go tool cover -func=coverage.out` |
| Benchmarks once present | `go test -run='^$' -bench=. -benchmem -count=5 ./...` |
| Bounded config fuzzing once present | `go test . -run='^$' -fuzz='^FuzzParseConfig$' -fuzztime=30s` |
| Validate release config | `goreleaser check` |
| Build release artifacts locally without publishing | `goreleaser release --snapshot --clean` |

Build commands writing `bin/` assume `mkdir -p bin` has run. Development against real configuration performs real destructive operations: use a disposable project and library for experiments. Fuzz commands name a planned root-package target; update the path if that code is later extracted. `goreleaser release --snapshot --clean` replaces the disposable `dist/` output and must never target user data.

## Project Structure

Start with a single module and a root command package, as supported by Go's official layout guide. Separate concerns through names and files before adding packages. This is a guide to responsibilities, not a requirement to create every empty file up front.

```text
.
|-- main.go              # Small entry point; delegates work and chooses exit status.
|-- cli.go               # Flag parsing and setup/TUI command dispatch.
|-- config.go            # Config decoding, validation, presets, and path resolution.
|-- setup.go             # Interactive configuration flow.
|-- model.go             # TUI state, messages, focus, and Update transitions.
|-- view.go              # Layout and presentation, with no filesystem I/O.
|-- keys.go              # Slot bindings, help text, and key-sequence rules.
|-- skills.go            # Directory listing, validation, copy, and removal.
|-- *_test.go            # Tests adjacent to the corresponding implementation.
|-- testdata/            # Small config fixtures and deterministic view snapshots.
|-- scripts/             # Installer and only scripts with a concrete release/test use.
|-- .github/workflows/   # CI and a separate tag-triggered release workflow.
|-- .golangci.yml        # Version-2 lint/format configuration.
|-- .editorconfig       # LF, final newlines, Go tabs; no imposed Go line-length limit.
|-- .goreleaser.yaml     # Explicit supported builds, archives, and checksums.
|-- go.mod              # Module identity, minimum Go version, pinned dependencies.
|-- go.sum              # Dependency integrity checksums.
|-- README.md           # Installation, setup, shortcuts, risks, and developer commands.
|-- product-vision.md   # Product intent and scope.
`-- prd.md              # This specification.
```

Use `internal/config`, `internal/skills`, or `internal/tui` only if a cohesive boundary makes code materially easier to understand and test. Do not introduce them all speculatively. `cmd/sei` becomes useful if the repository gains multiple executables or a public library; it is not mandatory for one CLI. Avoid `src/`, generic `utils`/`common` packages, a public `pkg/` tree without external consumers, interface-per-struct design, and controller/service/repository layers for local folder operations.

## Code Style

### Readable Go

Use official Go documentation and Code Review Comments as the baseline, with Google's Go Style Guide as the readability reference. Adopt clarity, simplicity, useful names, and least mechanism, not every Google-internal convention. Current language/library behavior takes precedence over dated examples.

- Formatting is mechanical: canonical `gofmt` layout plus `goimports` import organization, run through the pinned golangci-lint binary. No separate formatter dependency or `gofumpt` ruleset is necessary initially.
- Use `MixedCaps`/`mixedCaps`, conventional initialisms such as `ID` and `URL`, short lowercase package names, and descriptive names whose length fits their scope. Do not use Python-style `snake_case` identifiers or `ALL_CAPS` constants.
- Go uses `//` doc comments, not triple-quoted strings or mandatory Python `Args`/`Returns` sections. Document exported declarations and non-obvious internal contracts. Start declaration comments with the identifier; explain behavior, ownership, failure, and rationale where useful, not each statement.
- No fixed line-length, function-length, or file-length lint limits. Refactor when it improves comprehension, not to satisfy an arbitrary count. Keep straightforward operations together unless a helper is reusable or establishes a meaningful boundary.
- Check errors promptly and keep the normal path minimally indented. Add actionable context; use `%w` when retaining the underlying error is intentional, and `errors.Is`/`errors.As` instead of matching messages. Error text is normally lowercase without trailing punctuation; proper names retain their spelling.
- Return expected failures as errors. Do not panic, call `log.Fatal`, or call `os.Exit` inside filesystem, config, or TUI code. The outer entry point handles process exit after terminal/resource cleanup.
- Use concrete types first. Define a small interface at its consumer only when a real substitution boundary needs one; do not mock the whole filesystem by default. Prefer ordinary functions and structs over framework-like wrappers or unnecessary generics.
- Pass `context.Context` first for genuinely cancellable work, propagate it, and release cancellation resources. Do not add context to pure helpers or use it as a bag of configuration. Do not introduce copy cancellation contrary to the wait-on-quit product contract.
- Every goroutine must have a clear owner and completion path. Do not mutate TUI model state from workers. Handle relevant write/close errors and do not discard cleanup failures that affect correctness.
- Keep dependencies deliberate. Use modern Go facilities when they simplify the actual problem, not merely because they are new. No legacy `tools.go` pattern: future suitable Go tools can use Go's `tool` directives and `go tool`; golangci-lint remains a separately pinned release binary per upstream guidance.

Representative style example, not the complete project-path validator:

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// resolveProject returns an absolute path to an existing project directory.
// It does not search parent directories for a Git repository.
func resolveProject(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve project %q: %w", path, err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("inspect project %q: %w", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project %q is not a directory", abs)
	}
	return abs, nil
}
```

### Lint And Format Configuration

Create this `.golangci.yml` during implementation:

```yaml
version: "2"

linters:
  default: none
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused

formatters:
  enable:
    - goimports
```

This modest ruleset checks unhandled errors, suspicious constructs, ineffective assignments, static-analysis findings, unused code, and formatting without imposing a large house-style framework. Test files are included. In v2, formatters belong under `formatters`, not `linters`. `run` checks without rewriting; `fmt` rewrites. CI uses checking commands only. Any suppression must be narrow and explain the reason; do not enable every linter or exempt all tests.

Configure the editor to format/organize imports on save using Go tooling. Commit an EditorConfig with LF endings, final newlines, and tabs for Go files; let the formatter determine alignment. Do not require contributors to use a particular editor. Additional lint rules or a stricter formatter require an observed maintenance benefit, not a desire to maximize checks.

## Testing Strategy

Use Go's standard `testing` package with adjacent `_test.go` files, descriptive subtests, and table-driven cases where the same assertion logic repeats. Failure messages identify input, actual, and expected values. Prefer direct assertions over introducing a test framework. Use `t.TempDir`, explicit dependency inputs, and `t.Cleanup`; tests never read or mutate real user skill folders.

| Level | Coverage |
| --- | --- |
| Config/CLI unit tests | Strict JSON, duplicate/unknown keys, paths with spaces/Unicode, home expansion, relative local containment, overlapping roots, ordering, 1-9 bounds, missing vs invalid config, help/version, flag errors. |
| Model tests | Every focus/add/remove binding, uppercase distinction, pending `g`, cancellation, invalid continuations, paste rejection, help, empty lists, selection after delete, stale results, busy-input rejection, quit waiting, and resize state. |
| Filesystem integration tests | Real disposable nested trees, hidden entries, byte-for-byte copying, destination-only file removal, executable bits/umask, removal of non-library folders, blocked symlinks/special files, root aliases/overlap, same-path prevention, missing/unreadable paths, case collisions, and stale selections. |
| Failure regression tests | Failed preflight preserves existing target; mid-copy failure may leave partial output; failed deletion prevents copying into the remainder; errors include the actual target; affected panels refresh and the library stays unchanged. |
| View tests | Fixed-size representative snapshots for default three agents, empty/error/busy/help states, long names/paths, no-color rendering, and the later-approved small-window fallback. Avoid snapshots for every incidental styling change. |
| Process/terminal smoke tests | A built binary under a PTY: first-run setup, key decoding, one add/remove flow, normal quit, Ctrl+C during work, terminal restoration, and persistent stderr diagnostics/status `1` when work fails after a quit request. Plain message injection alone does not cover terminal behavior. |
| Installer/release tests | Architecture selection, unsupported hosts, checksum mismatch, failed download, custom directory, PATH guidance, upgrade failure, archive contents, and execution on the supported platform matrix. |

Use small targeted failure seams only where real filesystem tests cannot reliably induce the required error. Permission tests must account for elevated users; case-sensitivity tests must detect the actual filesystem. Do not encode Linux-only assumptions into macOS tests. Do not use sleep-based synchronization when an operation-completion signal can express the condition.

Test public behavior where practical, but same-package tests are appropriate for unexported root-command logic. Optional experimental Charm `teatest/v2` tooling must be compatibility-checked and pinned before adoption; it is not required by this PRD.

### CI And Coverage

- Pull requests run module-hygiene checks, formatting/lint, `go vet`, and tests with Go 1.27.1. Test on native Linux and macOS; release verification must exercise each promised OS/architecture, not merely cross-compile it. Select currently available runner labels in the implementation plan.
- Run `go test -race` on at least a supported Linux amd64 runner with a C compiler, even though release binaries disable CGO. The race detector checks exercised paths, not all possible interleavings.
- Add bounded fuzzing for config parsing and path/name validation with non-destructive invariants. Normal tests run fuzz seeds; a timed fuzz job selects one target in one package. Preserve discovered failures as regressions.
- Generate coverage reports and review gaps in mutation, validation, and input-state logic. No arbitrary global percentage gate in v1; every specified destructive/failure scenario requires a regression test, regardless of aggregate coverage.
- Keep release publishing separate from ordinary CI. Use reviewed SHA-pinned actions and pinned Go/linter/GoReleaser versions. Tag releases with full Git history available; grant write permissions only to publishing jobs. Never make PR tests require release secrets or live agent installations.
- Installer shell code needs linting and mocked/local download tests; choose the exact pinned shell tooling in release planning. Application tests run offline once dependencies and tools are available.

### Performance And Usability Baseline

Do not claim a startup, operation-latency, terminal-size, or artifact-size budget before measurement. The owner explicitly deferred these decisions to a prototype.

Record an agreed representative library of approximately 30 real-looking folders, including file counts, total bytes, long names, nesting, and executable scripts. Record machine, OS, filesystem, storage, terminal, dimensions, build flags, and artifact size. Use release builds, distinguish first/cold runs from repeated warm runs, and distinguish input-to-render latency from I/O completion. Report distributions and sample counts for startup-to-populated-panels, navigation, fresh copy, replacement, and removal. Assess SSH usability separately from local timings so network latency is not misattributed to application work.

Time the add-three/remove-two/quit task against the user's current shell workflow on the same fixture, resetting data between attempts. Record keystrokes, panel switches, mistakes, readability, and whether targets were unambiguous. Review a three-agent layout with the owner, then set explicit release budgets and layout minimums before declaring the release ready. A rendered screen with placeholder data is not proof of fast folder scanning or copying.

## Boundaries

- **Always:** Preserve the read-only library invariant; validate filesystem targets before destructive work; make copy/delete risks and targets explicit; keep UI state changes in `Update`; use source-backed current APIs; format and run relevant tests before submitting changes, and the full checks before a release; add regression tests for bug fixes; keep module sums and tooling versions reproducible.
- **Ask first:** Changes to destructive-copy semantics, shortcut rules, defaults, scope, or configuration compatibility; new dependencies beyond the agreed stack; new abstraction layers or commands; Windows/Homebrew support; performance/layout budgets after prototype evidence; CI permission or release-credential changes beyond the reviewed baseline; migrations once a configuration format has actually shipped.
- **Never:** Modify the source library; follow skill-internal symlinks; mutate outside validated destination children; delete roots; hide partial-copy failures; add confirmation or rollback to skill actions without changing this spec; execute skill scripts; inspect Git to permit or prevent deletion; silently claim agent isolation; add telemetry, automatic cleanup, or agent lifecycle management; commit credentials/generated binaries; disable failing tests or blanket-suppress lint to pass CI.

## Success Criteria

1. On a fresh installation without Go or agent tooling, one documented installer command installs a working binary on every supported OS/architecture. Any necessary PATH follow-up is explicit; no automatic privilege escalation or shell-profile modification occurs.
2. First-run setup creates valid native-location JSON and opens the TUI with Claude Code, Codex, and OpenCode initially selected. Cancelling or failed saving leaves prior configuration and skill folders unchanged. Pi, Cursor, custom destinations, and one-to-nine-agent configurations work without discovering unconfigured folders.
3. Launching in a subdirectory uses that exact directory for local paths; `--project` overrides it. The UI displays resolved targets and explains shared discovery paths without claiming control of agent sessions.
4. The user completes add-three/remove-two/quit without entering paths after setup or switching panels for every add. All specified keys, selection behavior, sequence cancellation, and help hints match the documented contract.
5. Successful add produces an independent full copy and removes destination-only content. Remove works for a supported destination folder absent from the library. Neither action asks for confirmation or checks Git, and neither changes the source library.
6. Unsafe paths, overlaps, links, special files, and type conflicts are rejected before destructive work. Ordinary mid-operation failures are reported with truthful missing/partial state; no success indication, hidden backup, or rollback is invented.
7. Filesystem work does not block navigation. Busy mutation inputs are rejected, not queued or replayed against a later selection. Normal quit waits for active work and restores the terminal; partial work after forced termination is a documented limitation.
8. Unit, integration, terminal smoke, lint/format, module-hygiene, and race checks pass at their documented scope. All destructive/error scenarios have regression coverage, and no test touches the developer's real library or destination folders.
9. A three-agent prototype and reproducible performance/usability report are reviewed by the owner. The workflow must improve on the existing shell process in real use; if it does not, revise the interaction before release rather than claim success from benchmark numbers alone.
10. The prototype review resolves terminal minimum/overflow behavior and quantitative performance/artifact budgets, followed by measurement against those agreed targets. These are release-readiness decisions still open below, not already demonstrated results.

## Open Questions

These are intentional implementation/prototype follow-ups, not invitations to reopen the agreed product scope.

| Question | When To Resolve |
| --- | --- |
| What repository/module path, license, initial release tag, and public installer URL will be used? | Before module initialization and distribution setup. |
| What three-agent layout looks good with the user's actual folder names, and what terminal size/overflow behavior supports one through nine agents safely? | Visual prototype review before finalizing layout tests. |
| What measured startup, navigation, filesystem latency, artifact-size budgets, and repeatable measurement protocol become release gates? | After the first representative release-build baseline. |
| What exact rooted filesystem operations and preflight algorithm enforce no-links/overlap rules, including missing ancestors and observed concurrent changes? | Filesystem implementation plan and platform-specific tests. |
| How will strict JSON duplicate-key detection and configuration-save failure handling be implemented with minimal standard-library code? | Configuration implementation plan; parser acceptance rules are already fixed. |
| What Linux floor, native architecture runners, terminal emulators, and SSH environments form the release support matrix? | CI/release plan; Linux/macOS amd64/arm64 scope is fixed. |
| What macOS signing/notarization and release provenance are needed for a friction-free installer, and what pinned installer-lint/test tools will be used? | Release plan before publishing installation instructions. |

All other requirements above are the agreed v1 baseline. Revisit current dependency patches and external agent-path documentation at implementation kickoff; do not silently change product semantics in the name of updating best practices.
