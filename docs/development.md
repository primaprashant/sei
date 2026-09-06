# Development

See [README](../README.md) for usage/configuration. A root entry point calls package `app` in `internal/`; tests live beside the application code.

## File Map

| Files | Responsibility |
| --- | --- |
| `main.go` | Entry point and build version |
| `internal/cli.go`, `internal/lifecycle.go` | Arguments/errors, terminal and signals |
| `internal/config.go`, `internal/config_save.go`, `internal/setup.go` | Config parsing/paths, safe saving, setup UI |
| `internal/skills.go`, `internal/roots.go` | Listings and filesystem boundary validation |
| `internal/mutation_fs.go`, `internal/mutation_copy.go` | Rooted removal, add and delete-then-copy replacement |
| `internal/model.go`, `internal/view.go` | Browser state/commands, keyboard handling, layout/display |
| `scripts/`, `.github/workflows/`, `.goreleaser.yaml` | Installer and release automation |
| `internal/testdata/views.golden` | Escaped model-view snapshots |

## Checks

Use Go from `go.mod` and the linter pinned in `.github/workflows/ci.yml` (here in `.bin`). From the repository root:

```sh
export GOTOOLCHAIN=go1.27.1 GOFLAGS=-mod=readonly
go mod verify
go mod tidy -diff
./.bin/golangci-lint config verify
./.bin/golangci-lint fmt --diff
./.bin/golangci-lint run --timeout=5m ./...
go vet ./...
go test -count=1 ./...
go build -o bin/sei .
CGO_ENABLED=1 go test -race -count=1 ./...
```

Race tests need a C compiler. Ordinary tests use disposable directories/PTYs, without fixture downloads or live agent installs.

## Disposable UI

After building, run in a terminal with disposable targets, HOME and native config locations:

```sh
demo=$(mktemp -d /tmp/sei-demo.XXXXXXXX)
mkdir -p "$demo/home" "$demo/project" "$demo/library/example"
printf '# Inert demo skill\n' > "$demo/library/example/SKILL.md"
printf '{"library":"%s/library","agents":[{"name":"Demo","global":"%s/global","local":".demo/skills"}]}\n' \
  "$demo" "$demo" > "$demo/config.json"
HOME="$demo/home" XDG_CONFIG_HOME="$demo/home/.config" \
  ./bin/sei --config "$demo/config.json" --project "$demo/project"
```

Try add/replace/remove, help, resize and quit; rerun to inspect persistence. Never use real agent paths for mutation tests.

## UI Invariants

Panel IDs are **library, configured globals, corresponding locals**; rendering puts library left,
**locals above globals**, with up to three agent columns. Never reorder IDs to match rendering:
slot/key mappings depend on config order. Focus windows retain access to all nine agents.
Below 80x24, new mutations are blocked; active work continues and quit stays available.
Preserve raw names through escaping/truncation; full escaped targets remain in help.

Intentional snapshot changes: `SEI_TEST_UPDATE_VIEWS=1 go test -run '^TestViewSnapshots$' ./internal`,
then review `git diff -- internal/testdata/views.golden`. Normal tests only compare snapshots.

## Async And Lifecycle

`Update`/`View` do no filesystem I/O. Commands capture immutable config, raw names
and targets; scan/safety generations and operation IDs reject stale results.
Safety and listing readiness are independent. Only one mutation runs at a time;
refresh during work coalesces into completion refresh. Navigation stays responsive.
`q`, Ctrl+C, INT/TERM/HUP request model-owned quit; active writes/saves finish first,
while idle quit need not join read-only scans. Repeated signals do not force exit.
`lifecycle.go` owns signal handling and explicitly restores termios after Tea exits,
without repeating renderer cleanup. Errors reach escaped stderr after restoration;
pending-quit operation failure exits 1, later idle quit after recoverable failure 0.
SIGKILL cannot clean up; disconnected terminals may reject restoration. See
[filesystem safety](filesystem-safety.md) for mutation guarantees and limits.

## Tests

All test files below live in `internal/`.

- Config/setup: `config*_test.go`, `setup*_test.go`; paths/files: `roots_test.go`, `mutation*_test.go`.
- UI: `model_test.go`, `navigation_test.go`, `keyboard_contract_test.go`, `view_test.go`, `stale_state_test.go`.
- Real terminals/errors: `*process_test.go`, `operation_failure_test.go`, `terminal_screen_test.go`.
- Input robustness: `fuzz_test.go`; packaging/install: `release*_test.go`, `installer*_test.go`.
- Optional archive tests need `SEI_RELEASE_DIST`, `SEI_RELEASE_VERSION`, `SEI_RELEASE_COMMIT`; see [release reproduction](release.md#local-snapshot).

## Optional Performance Checks

Microbenchmarks: `go test -run '^$' -bench . -benchmem ./internal`. Optional PTY tests require this exact digest-pinned archive:

- URL: https://codeload.github.com/addyosmani/agent-skills/tar.gz/469d00f4e67ff4a21eb6e6e467a086c9a1f1deb8
- SHA-256: `9f134b3c1c308b88a5f3acc37e0e33fcf25cf86a964644df1d2f24866ea0d192`

Download explicitly to a disposable path. The harness checks the digest and extracts only
`skills/` and MIT `LICENSE`, rejecting unsafe entries. No scripts/configuration execute; keep the license.
Build with `CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/sei .`, then run:

```sh
SEI_TEST_ARCHIVE=/absolute/path/to/archive.tar.gz SEI_TEST_BINARY="$PWD/bin/sei" \
  go test -count=1 -v -run '^(TestRepresentativePrototype|TestPerformanceRelease)$' ./internal
```

Run without competing builds/race tests. These are harness latencies, not disk-throughput claims;
warm-fixture budgets apply on Linux amd64. `SEI_TEST_PREPARE` creates a new absolute disposable
fixture instead of running the prototype. Omit `SEI_TEST_CAPTURES` unless writing captures intentionally.
