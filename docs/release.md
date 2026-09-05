# CI And Release Evidence

## Scope

Task 4 establishes ordinary PR/push CI only. It does not package, sign, tag,
upload, or publish releases. GoReleaser Community v2.18.0 remains reserved for
Task 27. Commits, pushes, credentials, branch protection, tags, and publication
still require explicit owner authorization. Task commits are authorized for this
implementation; pushing and publication are not.

## Native CI

`.github/workflows/ci.yml` expands into four independent native jobs, with
`fail-fast: false` so one failure does not cancel the other architectures:

| Runner | Required Native Platform | Checks |
| --- | --- | --- |
| `ubuntu-24.04` | Linux x86_64 / Go linux/amd64 | Standard checks and CGO-enabled race tests |
| `ubuntu-24.04-arm` | Linux aarch64 / Go linux/arm64 | Standard checks |
| `macos-15-intel` | Darwin x86_64 / Go darwin/amd64 | Standard checks |
| `macos-15` | Darwin arm64 / Go darwin/arm64 | Standard checks |

Every job logs runner OS/architecture, image identity/version, `uname -a`, OS
release, C compiler version, Go version/environment, and linter version. Host
and Go target assertions reject architecture mismatches; macOS also rejects
Rosetta translation. Runner labels select OS families, not immutable images.
Preserve the job's **Set up job** image details along with these logs.

Standard checks match the [README](../README.md#checks): module tidy diff,
linter config verification, formatting diff, lint, vet, uncached tests, and
build. CI additionally downloads/verifies modules, checks the built binary's
help/version, and rejects tracked-file changes even after a failed step.
`GOFLAGS=-mod=readonly` prevents build-time module repair; neither formatter
fixes nor `go mod tidy` without `-diff` are used. No cache action is used.

Both events have only `contents: read`; all unspecified token permissions are
disabled. Checkout does not persist credentials. There is no
`pull_request_target`, release secret, OIDC permission, privileged publishing
job, or live agent installation. Checks use disposable `HOME` and
`XDG_CONFIG_HOME`, isolating macOS's `$HOME/Library/Application Support` too.
Future filesystem tests must continue using disposable project/library/target
fixtures and must never execute skill scripts.

## Pins And Review

Reviewed on 2026-09-05 using read-only `gh api` queries and primary upstream
sources:

- The only action is [actions/checkout v7.0.1](https://github.com/actions/checkout/releases/tag/v7.0.1),
  pinned to `3d3c42e5aac5ba805825da76410c181273ba90b1`. The upstream
  [tag ref](https://api.github.com/repos/actions/checkout/git/ref/tags/v7.0.1)
  resolves directly to that commit, not an unresolved annotated-tag object.
  Reviewed release notes, [action metadata](https://github.com/actions/checkout/blob/3d3c42e5aac5ba805825da76410c181273ba90b1/action.yml),
  and [checkout/auth cleanup flow](https://github.com/actions/checkout/blob/3d3c42e5aac5ba805825da76410c181273ba90b1/src/git-source-provider.ts).
  It uses Node 24 main/post entry points; `persist-credentials: false` removes
  checkout authentication before project commands. This is a targeted source
  review, not an audit of every bundled dependency.
- Go is exactly **1.27.1**, including `GOTOOLCHAIN=go1.27.1` without `+auto`.
  All four archive hashes were checked against the official
  [Go download metadata](https://go.dev/dl/?mode=json&include=all).
  CI installs the verified archive directly and asserts `GOVERSION`, rather
  than trusting a preinstalled toolchain or a floating setup action.
- golangci-lint is the official **v2.13.2** binary. All four archive hashes
  agree between the [release asset metadata](https://api.github.com/repos/golangci/golangci-lint/releases/tags/v2.13.2)
  and [checksum manifest](https://github.com/golangci/golangci-lint/releases/download/v2.13.2/golangci-lint-2.13.2-checksums.txt).
  Reviewed [installation guidance](https://golangci-lint.run/docs/welcome/install/local/):
  use release binaries, not `go install` or module tool dependencies. No remote
  installer script is executed.
- The [GitHub runner reference](https://docs.github.com/en/actions/reference/runners/github-hosted-runners)
  lists all four selected labels and their native architectures for public
  repositories. Documentation availability is not proof that jobs ran.

The workflow matrix is the authoritative list of eight per-archive SHA-256
pins. HTTPS downloads are checked against these reviewed values with
`shasum -a 256 --check` before either archive is extracted. Updating a tool
requires reviewing its release, all four archive hashes, and the matching
version declarations; updating an action requires resolving and reviewing its
new full commit SHA. Checksums detect corruption/substitution relative to the
reviewed bytes, not a compromised publisher. Module checksum verification stays
enabled with `sum.golang.org`; these controls are not release attestations.

## Verification And Blockers

Local verification on 2026-09-05 passed on Linux amd64 (Debian kernel
`6.12.90+deb13.1-cloud-amd64`, GCC `14.2.0`):

- Go `1.27.1`; golangci-lint `2.13.2` (upstream binary built with Go `1.27.0`).
  The local linter archive matched its pinned SHA-256 and its executable was
  byte-compared with `.bin/golangci-lint` before execution.
- Module download/verify, tidy diff, linter config verification, formatting
  diff, lint (zero issues), vet, `go test -count=1 ./...`, and build passed.
- `CGO_ENABLED=1 go test -race -count=1 ./...` passed; built help/version passed
  with disposable HOME/config paths (`sei dev`, not a release version).
- Ruby/Psych parsed the workflow YAML; four matrix entries and eight digest
  shapes were checked; every `run` block passed `bash -n`. `git diff --check`
  passed and application/module/README/linter files remained unchanged.
  `actionlint` and ShellCheck were not installed; YAML/Bash syntax checks are
  not a substitute for GitHub workflow validation or native remote execution.

Remote runner execution is **blocked: no push authorization**. No branch/PR was
pushed or workflow dispatched for Task 4. There are no remote
run URLs or actual runner image results to report; all four native execution
results remain pending, not passed.

After owner review, a sequential Task 4 commit, and explicit push authorization:

1. Exercise PR/push CI on the authorized branch/PR.
2. Inspect the run with `gh run view <run-id> --repo primaprashant/sei` and
   `gh run view <run-id> --repo primaprashant/sei --log`.
3. Record the run URL, tested commit, all four job conclusions, actual image/OS/
   architecture/tool versions, and Linux amd64 race result here. Confirm jobs
   actually executed, rather than being skipped, canceled, or only queued.
4. Investigate unavailable runner access as a blocker; never silently remove a
   matrix entry. Close Task 4's native-execution criterion only with this evidence.

These newer CI images do not validate the proposed Ubuntu 22.04/5.15 kernel
floor or macOS 13. Native machines/VMs on both architectures still need to be
arranged for Task 28; access and support-floor claims are unverified. Actual
release archive execution, terminal lifecycle, signing/notarization, provenance,
installer verification, and publishing remain their later tasks.
