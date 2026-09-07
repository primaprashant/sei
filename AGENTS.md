# Working On sei

sei is a small personal Go terminal tool for copying and removing skill folders,
not an agent/session manager. Keep code, dependencies, documentation, and release
process proportional to that purpose. Prefer the smallest correct change; do not
add speculative abstractions, enterprise process, or implementation journals.

## Context On Demand

- [docs/product-vision.md](docs/product-vision.md): the owner's original motivation.
- [README.md](README.md): current user-facing behavior and configuration.
- [docs/development.md](docs/development.md): source map, tooling, disposable UI
  setup, snapshot updates, and async/terminal constraints. Read relevant sections
  when changing UI, keys, or lifecycle behavior.
- [docs/filesystem-safety.md](docs/filesystem-safety.md): read before changing path
  resolution, config saves, copying, or deletion. Simpler process must not weaken
  filesystem protections.
- [docs/release.md](docs/release.md): read for installer/packaging/release work,
  not routine application edits. Publishing or pushing tags requires a request.

## Verification

Run from the repository root using Go from `go.mod`. For code changes:

```sh
go test -count=1 ./...
go vet ./...
go build -o bin/sei .
```

Use the CI-pinned golangci-lint binary, not a floating tool version;
`.golangci.yml` owns formatting/lint rules. Commands and additional race, PTY,
snapshot, installer, and optional artifact checks are in the development guide.
Add or adjust focused regression tests for behavior changes. For documentation-only
edits, check links and `git diff --check`; no full release rehearsal is needed.
Report checks actually run and any unavailable or skipped verification.

## Safe Experiments

Use disposable HOME, config, project, library, and destination paths. Never test
mutations against real agent installations or execute skill scripts. On macOS,
avoid `~/library` fixtures, which can alias the native `~/Library` config location.
The source library stays read-only; replacement is delete-then-copy with no undo.
