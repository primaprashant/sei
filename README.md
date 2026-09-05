# sei

A terminal skill-folder manager for copying skills from a personal library to
configured agent destinations and removing installed copies. Not a skill
marketplace, agent launcher, or agent skill. See [the PRD](prd.md) for the product
contract and [the implementation plan](implementation-plan.md) for progress.

## Status

The bootstrap executable supports `--help`, `--version`, and a non-mutating
terminal shell (quit with `q` or Ctrl+C). Configuration, browsing, setup, and
skill mutations are not implemented. There is no published installer or usable
release. Unknown flags, commands, and positional arguments return status `2`;
interactive startup requires terminal stdin and stdout and otherwise returns `1`.

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
not a character-device heuristic. No extra PTY module is added in Phase A.
The shell uses Bubble Tea's default lifecycle only while no asynchronous work or
mutations exist; Task 9 must establish the lifecycle contract before mutations.

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
scripts. The current tests isolate HOME and do not access configuration or skill
folders. For an interactive shell smoke check with isolated data:

```sh
scratch=$(mktemp -d)
HOME="$scratch" XDG_CONFIG_HOME="$scratch" ./bin/sei
```

Generated tools, binaries, release output, coverage output, and `/.opencode/`
remain ignored. GoReleaser Community `v2.18.0` is reserved for the later release
tasks; Phase A does not install it or create a release configuration. Native
macOS/arm64 verification and CI are not established by local Linux checks.
