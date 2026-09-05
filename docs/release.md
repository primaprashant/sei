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

## Verification Evidence

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

Remote verification on 2026-09-05 used read-only `gh run view 33966759655
--repo primaprashant/sei --json url,headSha,status,conclusion,jobs` and the same
command with `--log`. [Run 33966759655](https://github.com/primaprashant/sei/actions/runs/33966759655)
completed with **success**, testing commit
`0c6f150aca18388e3ea30adad37d5191c8d7b113`. All four jobs actually executed and
completed successfully; none was skipped, canceled, or merely queued.

| Job / Source Log | Actual OS / Native Architecture | Set Up Job Image / Version | Go Version / Platform | golangci-lint | Result |
| --- | --- | --- | --- | --- | --- |
| [ubuntu-24.04](https://github.com/primaprashant/sei/actions/runs/33966759655/job/101308141064) | Ubuntu 24.04.4 LTS; Linux `6.17.0-1022-azure`; X64 / `x86_64` | `ubuntu-24.04` / `20260831.293.1` | `1.27.1` / `linux/amd64` | `2.13.2` | Success; race passed |
| [ubuntu-24.04-arm](https://github.com/primaprashant/sei/actions/runs/33966759655/job/101308141087) | Ubuntu 24.04.4 LTS; Linux `6.17.0-1022-azure`; ARM64 / `aarch64` | `ubuntu-24.04-arm` / `20260831.111.1` | `1.27.1` / `linux/arm64` | `2.13.2` | Success; race skipped by design |
| [macos-15-intel](https://github.com/primaprashant/sei/actions/runs/33966759655/job/101308141005) | macOS 15.7.9 (`24G830`); Darwin `24.6.0`; X64 / `x86_64` | `macos-15` / `20260824.0482.1` | `1.27.1` / `darwin/amd64` | `2.13.2` | Success; race skipped by design |
| [macos-15](https://github.com/primaprashant/sei/actions/runs/33966759655/job/101308140968) | macOS 15.7.9 (`24G830`); Darwin `24.6.0`; ARM64 / `arm64` | `macos-15-arm64` / `20260829.0321.1` | `1.27.1` / `darwin/arm64` | `2.13.2` | Success; race skipped by design |

The logged `ImageOS` values were `ubuntu24`, `ubuntu24-arm64`, and `macos15`
(both Macs); image versions matched **Set up job**. Every linter reported
build Go `1.27.0`, revision `27774aaf`. Linux used GCC `13.3.0`
(`13.3.0-6ubuntu2~24.04.1`); both Macs used Apple clang `17.0.0`
(`clang-1700.0.13.5`). Native host/Go target assertions and macOS's no-Rosetta
assertion passed. All jobs passed archive verification, standard checks (zero
lint issues, uncached tests, build and help/version), and tracked-file cleanliness.
Linux amd64 passed `CGO_ENABLED=1 go test -race -count=1 ./...` (`ok`, `1.301s`);
the other three race steps were intentionally skipped, not claimed as passes.
This evidence inspection did not dispatch/rerun workflows, commit, or push.

These newer CI images do not validate the proposed Ubuntu 22.04/5.15 kernel
floor or macOS 13. Native machines/VMs on both architectures still need to be
arranged for Task 28; access and support-floor claims are unverified. Actual
release archive execution, terminal lifecycle, signing/notarization, provenance,
installer verification, and publishing remain their later tasks.
