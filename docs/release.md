# CI And Release Evidence

## Scope

Checkpoint J (2026-09-06): clean `f809d10` snapshots now occupy `dist/` as
`0.0.0-snapshot.f809d10`, replacing historical artifacts recorded below.
All four archive checks pass with the exact producer revision and clean-source
assertion; native Linux amd64 help/version/PTY and full standard/race/installer
checks pass. Subsequent [Phase J hosted evidence](#phase-j-hosted-evidence) closes
native CI execution at `54fbfb9`, not minimum-OS hosts, support approval or Mac
download trust. Tasks 31-32 are verified locally below; their new native execution
remains pending. Task 33 user documentation is implemented with
[local/mock verification](test-matrix.md#task-33-documentation); public URL and
fresh-user native/trust acceptance remain pending. No public release.

Task 4 establishes ordinary PR/push CI. Task 27 adds local snapshot packaging
with GoReleaser Community v2.18.0, not signing, tagging, uploading or publishing.
Commits, pushes, credentials, branch protection, tags, and publication require
explicit owner authorization.

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

- The checkout action is [actions/checkout v7.0.1](https://github.com/actions/checkout/releases/tag/v7.0.1),
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

## Task 27 Archive Contract

Implemented sequentially after `c09ad3ade1f7d709c934c73cfa5dfd292aa8c6f5`.
`.goreleaser.yaml` uses the JSON subset of YAML so ordinary Go tests can validate
the complete small contract without a YAML/tool dependency. GoReleaser's own
`check` remains the authoritative schema check. Use **Community v2.18.0 only**,
not Pro, a package-manager build, `go install`, or another version.

- Exactly `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`.
- Go `1.27.1`, `CGO_ENABLED=0`, `GOAMD64=v1`, `GOARM64=v8.0`, `-trimpath`,
  `-s -w`, and `-X main.version={{ .Version }}`. Existing CLI version handling
  needs no change. Module repair is disabled with `GOFLAGS=-mod=readonly`.
- `sei_<version-without-v>_<os>_<arch>.tar.gz` and
  `sei_<version-without-v>_checksums.txt` (SHA-256). An eventual authorized
  `v0.1.0` tag maps to `sei_0.1.0_linux_amd64.tar.gz`, for example.
- Each tar has exactly four regular root members: executable `sei` (0755),
  `README.md`, `LICENSE`, `THIRD_PARTY_NOTICES`. The explicit file allowlist
  replaces default globs; no config, fixtures, source, tools, or test binaries.
- Snapshot version is `<base-version>-snapshot.<short-commit>`. Without tags,
  GoReleaser reports its synthetic `v0.0.0` base. This is not an initial release
  tag and does not alter the owner's version policy. Dirty snapshots are allowed
  by GoReleaser; embedded Go build metadata truthfully records `vcs.modified=true`.
- `release.disable: true` is an additional publication guard; no publisher,
  signing, installer, or release workflow was added. `dist/` is ignored,
  disposable generated output, including GoReleaser's JSON/YAML metadata.

### Tool Provenance

Reviewed 2026-09-06 using HTTPS primary sources. `.bin` initially contained only
the pinned linter and its archive; `goreleaser` was absent from PATH. Downloaded
the approved upstream Community baseline, verified **before extraction/execution**:

| Upstream v2.18.0 asset | SHA-256 |
| --- | --- |
| `goreleaser_Linux_x86_64.tar.gz` | `41cdf49b653784b03a08013dd99e382cd5d463049e915c2d818eaed182ae6197` |
| `checksums.txt` | `5cde70ff710a88df1c6a21980400884712451b096b53815717ad8d535ca14888` |

Both hashes match [GitHub release asset digests](https://api.github.com/repos/goreleaser/goreleaser/releases/tags/v2.18.0);
the archive hash also matches the upstream [checksum manifest](https://github.com/goreleaser/goreleaser/releases/download/v2.18.0/checksums.txt).
The extracted executable reports `GitVersion: 2.18.0`, commit
`a38ac3174c591f95049234d25bb326104d3ca820`, build date
`2026-08-23T16:54:22Z`, Go `1.27.0`, linux/amd64. Reviewed the
[versioned config source](https://github.com/goreleaser/goreleaser/blob/v2.18.0/pkg/config/config.go)
and upstream [Go build options](https://goreleaser.com/customization/builds/go/).
No fallback or remote installer script was used. Sigstore verification was not
performed; same-publisher hashes are integrity evidence, not independent
publisher authentication or release attestations.

The existing Go installation reports `go1.27.1 linux/amd64`. Downloaded official
`go1.27.1.linux-amd64.tar.gz`, verified SHA-256
`63d339f0da5ab53635a56f2490a7984dfe12dfcff22ad749f63edaf590168445`
against [Go download metadata](https://go.dev/dl/?mode=json&include=all), and
`tar --compare --gzip --file .bin/go1.27.1.linux-amd64.tar.gz --directory /usr/local`
passed against the installed tree. No toolchain was replaced. The installed
linter archive again matched the README's `2277d43b...021d6` pin and its extracted
executable byte-compared equal to `.bin/golangci-lint`; it reports v2.13.2.

### Dependency And License Audit

`go.mod` and `go.sum` remain unchanged. `go mod verify` passed. For **each** of
the four targets, `CGO_ENABLED=0 go list -deps` produced the same 18 external
runtime modules, all listed with exact versions in `THIRD_PARTY_NOTICES`.
Their checked module-cache `LICENSE` files (`LICENSE.txt` for uniseg) were
reviewed: 16 MIT modules and two BSD-3-Clause modules (`x/sys`, `x/sync`).
Identical license bodies are consolidated while retaining every copyright.
The Go runtime/standard-library BSD license is included too. uniseg's generated
property-table headers explicitly reference the Unicode license; the notice
includes the current [Unicode License V3](https://www.unicode.org/license.txt)
for Unicode-derived tables, rather than assuming MIT alone covers the data.

`github.com/creack/pty v1.1.24`, `github.com/charmbracelet/x/vt
v0.0.0-20260906004030-3986e9119cf9`, and `github.com/charmbracelet/x/exp/ordered
v0.1.0` are test-only graph members and absent from all release executable build
metadata. Their code and development tools are not shipped. Package-level
`x/windows` is in the runtime graph even on these targets, so its notice is
retained. Archive tests reject replaced/unlisted dependencies and check embedded
module sums against `go.sum`. Dependency changes require repeating this audit;
the tests enforce inventory, not legal interpretation.

### Reproduction

From the repository root on Linux amd64, after preparing the pinned Go/linter
and modules as documented in README. Stop on any failed verification; do not
substitute another tool/version. Other hosts need their own reviewed upstream
v2.18.0 archive digest, not the Linux amd64 digest below.

```sh
set -eu
mkdir -p .bin
curl --fail --location --proto '=https' --tlsv1.2 \
  -o .bin/goreleaser_Linux_x86_64.tar.gz \
  https://github.com/goreleaser/goreleaser/releases/download/v2.18.0/goreleaser_Linux_x86_64.tar.gz
printf '%s\n' '41cdf49b653784b03a08013dd99e382cd5d463049e915c2d818eaed182ae6197  .bin/goreleaser_Linux_x86_64.tar.gz' | sha256sum --check -
tar -xzf .bin/goreleaser_Linux_x86_64.tar.gz -C .bin goreleaser
./.bin/goreleaser --version
# Assert the exact version even when reusing an existing verified .bin tool.
./.bin/goreleaser --version | grep -Eq '^GitVersion: +2\.18\.0$'
export GOTOOLCHAIN=go1.27.1 GOFLAGS=-mod=readonly
go mod download
go mod verify
./.bin/goreleaser check
GOPROXY=off ./.bin/goreleaser release --snapshot --clean
export SEI_RELEASE_DIST=dist
export SEI_RELEASE_VERSION=$(jq -er .version dist/metadata.json)
export SEI_RELEASE_COMMIT=$(git rev-parse HEAD)
test "$(jq -er .commit dist/metadata.json)" = "$SEI_RELEASE_COMMIT"
go test -count=1 -run TestRelease -v .
# Build the checker first, then run it with networking disabled. Its extracted
# native child gets only PATH=, disposable HOME and XDG_CONFIG_HOME.
go test -c -o bin/release.test .
unshare --user --map-root-user --net \
  ./bin/release.test -test.run '^TestRelease' -test.v
```

The opt-in test reads all four actual tarballs, rejects extra/duplicate/nonregular
members, compares documentation bytes, verifies SHA-256 and gzip trailers,
checks Go provenance/CPU baseline/CGO metadata and ELF/Mach-O headers, rejects
ELF dynamic linking/symbol/debug sections and Mach-O DWARF, then extracts and
executes only the native binary by absolute path. Exact help/version stdout,
empty stderr, zero status and a five-second deadline are required. It never
invokes Go, a shell, or agent tooling in the application child. Ordinary tests
only validate the configuration; they do not download tools or create releases.
Network namespace isolation is an explicit Linux verification command, not a
portable assertion made by the test. If unavailable, record that limitation
rather than claiming offline execution from `GOPROXY=off` or empty PATH alone.

### Local Evidence

2026-09-06, Linux amd64, kernel `6.12.90+deb13.1-cloud-amd64`. Snapshot source is
`c09ad3a` plus the uncommitted Task 27 files, not a clean published revision.
Tool integrity checks, module verify/tidy diff, linter config/fmt/lint (zero
issues), vet, uncached ordinary tests (including PTY), build and full CGO-enabled
race tests passed. Final snapshot/archive/offline evidence is recorded below.

`goreleaser check` and `goreleaser release --snapshot --clean` passed with the
verified Community binary. The only Git warning was the expected absence of
tags in snapshot mode. Four tar listings showed precisely the allowlisted
members; all four manifest checks passed. `TestReleaseConfig` and all four
`TestReleaseArchives` subtests passed inside an isolated network namespace,
including exact native help/version with empty PATH and isolated HOME/XDG.

| Archive in `dist/` | Verified SHA-256 |
| --- | --- |
| `sei_0.0.0-snapshot.c09ad3a_linux_amd64.tar.gz` | `0d9be44076202133007c590c1011b1b0a0ab29b26610e6e75fd3db412e144e58` |
| `sei_0.0.0-snapshot.c09ad3a_linux_arm64.tar.gz` | `67be95f731dc584398dfd6f6c5538b7965401ab00c626aedd7b9d3f2af18de66` |
| `sei_0.0.0-snapshot.c09ad3a_darwin_amd64.tar.gz` | `3645699913eb1a3ae454cf5e7a4bd31cc81dd0a17fa10b9a5e1706718f63059f` |
| `sei_0.0.0-snapshot.c09ad3a_darwin_arm64.tar.gz` | `6c743720f49b7e0537277a878eeebef7f0f9c6e331dbaf26720b3f4df28b35ce` |

Manifest: `dist/sei_0.0.0-snapshot.c09ad3a_checksums.txt`. These identify this
local run; archive documentation mtimes/ownership are not normalized, so this
task promises a stable asset contract, not byte-identical rebuilds elsewhere.

An additional Linux-only check extracted the native archive into a fresh chroot
containing **only those four files**, with no Go installation, dynamic loader,
shell, agent tools, or host configuration. Both commands below passed offline,
returning help and `sei 0.0.0-snapshot.c09ad3a` respectively (zero exit status).
`unshare`/`chroot` are host-side verification tools, not application dependencies:

```sh
# After release --snapshot --clean; native-runtime must not already exist.
mkdir dist/native-runtime
tar -xzf dist/sei_0.0.0-snapshot.c09ad3a_linux_amd64.tar.gz -C dist/native-runtime
env -i PATH= HOME=/ XDG_CONFIG_HOME=/ \
  /usr/bin/unshare --user --map-root-user --net \
  /usr/sbin/chroot dist/native-runtime /sei --help
env -i PATH= HOME=/ XDG_CONFIG_HOME=/ \
  /usr/bin/unshare --user --map-root-user --net \
  /usr/sbin/chroot dist/native-runtime /sei --version
```

Native Linux arm64 and both macOS executions, support-floor machines, extracted
artifact PTY flows, signing/notarization, independent attestations, and actual
publication remain external/later-task gates. Cross-compilation and header
inspection do not satisfy those gates. No tag, push, upload, or release
publication occurred.

Checkpoint I rebuilt all four archives from clean commit `049000d` as
`0.0.0-snapshot.049000d`; those now replace the earlier Task 27 artifacts in
`dist/`. All checksums, archive/metadata tests and native help/version passed
again. The earlier hashes above describe only the recorded Task 27 run.

## Task 28 Exact Native Archives

Phase J implementation and logical commits are authorized sequentially, without pushes or
tags. Support-floor access and macOS Gatekeeper are **release blockers**, not
waivers or support claims. Task 28 acceptance remains open until native evidence
and owner support-table approval exist.

CI now has one Linux amd64 producer using the verified Go 1.27.1 and Community
GoReleaser v2.18.0 archives above. It builds the four snapshots once, validates
them, and transfers only those four tarballs and their checksum manifest.
The existing native matrix depends on that producer and downloads its exact
artifact ID in the same workflow run. Ordinary source builds still test source;
they are never substituted for release executables. Producer outputs supply the
full checkout commit and metadata version, including PR merge commits. Each
consumer asserts its checkout matches, verifies all archive hashes and embedded
`vcs.revision`, rejects dirty CI binaries, and checks the snapshot commit suffix.
Logs identify archive hashes and `vcs.modified`; local dirty snapshots are honest
worktree evidence, not clean-commit release evidence.

`TestReleaseArchives/<native-os>/<native-arch>/PTY` passes the checksum-verified,
extracted executable directly to `runSetupPTY`. Disposable config/project/HOME,
empty tool directory PATH, xterm-256color/no-color and a 200x40 real PTY are used.
It waits for the browser, asserts raw mode, sends q, requires zero status and
empty stderr, compares restored termios and checks ordered alternate-screen and
cursor restoration with bounded deadlines. No application or wrapper is built
by this test. The Go toolchain/modules are harness prerequisites only. Help and
version still execute with literal `PATH=`. Existing ELF static/no-interpreter
checks and Linux offline/no-runtime commands above are retained; empty PATH by
itself is not network isolation. This is an early enter/quit gate, not the full
final-artifact feature/PTY rehearsal required in Tasks 34-36.

### Transfer Action Review

Read-only `gh api` review on 2026-09-06 resolved upstream release/tag refs directly
to commits and inspected each pinned `action.yml` plus upload/download source:

| Action | Full Commit | Reviewed Behavior |
| --- | --- | --- |
| `actions/upload-artifact` v7.0.1 | `043fb46d1a93c77aae656e7c1c64a875d1fc6a0a` | Node 24; explicit file allowlist, missing files fail, default no overwrite/hidden files, artifact ID output; seven-day retention. |
| `actions/download-artifact` v8.0.1 | `3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c` | Node 24; current-run artifact ID lookup without a supplied GitHub token; expected digest passed to client and mismatch fails. |

Sources: upstream `repos/actions/{upload-artifact,download-artifact}/git/ref/tags/`
and `contents/action.yml`, `src/upload/upload-artifact.ts`,
`src/download-artifact.ts` at the listed SHAs. This is targeted source review,
not a bundled-dependency audit. GoReleaser release API digests were rechecked;
local tool archive checksums, executable byte comparison, and installed Go tree
comparison passed. Existing tool provenance limitations still apply.
Artifact transfer uses GitHub's scoped Actions runtime service, not a new
publisher credential. `contents: read`, nonpersistent checkout credentials and
all other disabled permissions remain unchanged; no security/write permission,
release secret, signing, attestation or publication was added.

### Support Table

These are proposed floors, **not supported-platform claims**:

| Environment | amd64 | arm64 | Disposition |
| --- | --- | --- | --- |
| Ubuntu 22.04, actual 5.15-series host kernel | Untested | Untested | Release blocker; newer kernel/container userspace does not qualify. |
| macOS 13 | Untested | Untested | Release blocker; no Rosetta substitution. |
| Ubuntu 24.04 CI labels | Phase J exact archive run passed | Phase J exact archive run passed | Newer-host evidence only; see hosted evidence below. |
| macOS 15 CI labels | Phase J exact archive run passed | Phase J exact archive run passed | Not macOS 13 or Gatekeeper evidence. |
| Local Debian, kernel 6.12.90+deb13.1-cloud-amd64 | Local verification below | Unavailable | Not an Ubuntu floor. |

macOS quarantine/download/extraction/Gatekeeper validation remains Task 29's
release blocker. CI extraction cannot establish the end-user trust path; do not
remove quarantine or disable security checks to turn it green. The
[Phase J hosted run](#phase-j-hosted-evidence) establishes exact-archive execution
at `54fbfb9`, not new native verification of Tasks 31-33.

### Floor Host Runbook

1. Acquire native Ubuntu 22.04 machines/VMs actually booted into 5.15.x and macOS
   13 machines/VMs on both architectures. Record image/provider/build identity,
   CPU model/architecture, filesystem type/case behavior, terminal app/version,
   TERM, dimensions, local versus SSH and `ssh -V`. Save `uname -a`, Linux
   `/etc/os-release` and `findmnt -T .`, or `sw_vers`, `sysctl -n machdep.cpu.brand_string`,
   `diskutil info .` and `sysctl -in sysctl.proc_translated` on Macs. Reject
   translated execution, wrong architecture or newer floor kernels/OS versions.
2. Preserve producer run URL, full commit, version, artifact ID/digest and the
   four tarball hashes. Transfer that exact five-file bundle to each host. Check
   out the producer commit and use its harness/docs, not a newer source checkout.
   Do not run GoReleaser or rebuild sei on consumers. Prepare pinned Go/modules
   for the harness separately, recording tool versions; never install a runtime
   into the application's isolated environment.
3. From that checkout, set `SEI_RELEASE_DIST` to the bundle's absolute directory,
   `SEI_RELEASE_COMMIT` to the recorded full producer commit and
   `SEI_RELEASE_VERSION` to the recorded producer version (not values guessed
   from filenames). Run `go test -count=1 -run '^TestRelease' -v .` and retain
   full output/status, hashes and environment capture. Require the matching PTY
   subtest to execute, not only foreign metadata subtests. Missing host/tool/PTY
   access is a blocker, never a skip-to-pass. Fixtures are disposable.
4. On Linux, build only the checker with `go test -c -o bin/release.test .`, then
   use the isolated-network checker command above. Repeat the four-file chroot
   help/version proof with the recorded version/native archive filename and a
   fresh directory. Record namespace availability and status separately; no
   network isolation is inferred on macOS. Preserve actual downloaded macOS
   quarantine/trust-path evidence separately for Task 29.
5. Submit all four floor-host results and limitations for owner support-table
   approval. Keep the floors and Gatekeeper blocked until resolved; schedule
   full final-artifact feature/PTY tests again in Tasks 34-36.

### Task 28 Local Evidence

2026-09-06: Debian Linux x86_64, kernel `6.12.90+deb13.1-cloud-amd64`, ext4,
Go `1.27.1`, OpenSSH `10.0p2 Debian-7+deb13u4`. Automated local PTY is
200x40, `TERM=xterm-256color`, `NO_COLOR=1`; no human terminal or new SSH
acceptance is claimed. Producer base is
`afe864f4b1d4a699b03774abae82194de852f07e` plus uncommitted Task 28 changes;
all four binaries honestly record `vcs.modified=true`.

| Check | Result |
| --- | --- |
| Tool integrity/tree comparison, module verify/tidy diff, lint config/format/lint, vet, uncached full tests, application build | PASS; zero lint issues. |
| Full CGO-enabled race suite | PASS, 115.194s; extracted release children themselves are not race-instrumented. |
| Existing complete-restart PTY 5x; exact archive checks 5x; focused archive race | PASS, including native Linux amd64 PTY. Other three targets received metadata inspection only. |
| Verified GoReleaser schema check and four-target snapshot | PASS, version `0.0.0-snapshot.afe864f`; no tags warning expected. |
| Prebuilt archive checker inside isolated network namespace | PASS, including exact native help/version and PTY restoration. |
| Fresh four-file chroot with empty PATH and no network/runtime/loader/shell | PASS, native help/version; PTY was tested separately outside the chroot. |
| Both macOS test-harness cross-builds | PASS compilation only. |
| Ruby/Psych YAML parse, every run block `bash -n`, diff whitespace | PASS; actionlint and ShellCheck unavailable. Hosted workflow validation/execution still open. |

Current `dist/sei_0.0.0-snapshot.afe864f_checksums.txt` identifies these local
artifacts, replacing Checkpoint I output, not promising reproducible rebuilds:

| Target | SHA-256 |
| --- | --- |
| linux/amd64 | `6859a5eabd6fdeabe1de7b4df986f11c660bf52a953c092ce8b081136c1ee848` |
| linux/arm64 | `68586fa85c7222552d3ba58b98da221035b889d2a4a79847d1bcfff44c41f9de` |
| darwin/amd64 | `f37d34ed12a70e319be072b811f523b5de02aa84326c6ce339f76e0004b648c9` |
| darwin/arm64 | `97ab4a28a6ac789e8b3c0838df73d11f35c7f5d998c4a24833268a6f046310f4` |

## Task 29 Trust Policy

2026-09-06, sequentially after `8ad23fe`: owner approves local/CI slices while
floor/Mac gates remain blocked, verified ShellCheck v0.11.0, and GitHub artifact
attestations as the intended provenance policy. Apple signing/notarization is
deferred to actual download tests. **No credentials or write permissions are
authorized this phase.** No workflow, installer, signing or publication added.

### Future Protected Release Only

Reviewed primary [GitHub attestation guidance](https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations)
and [CLI verification policy](https://cli.github.com/manual/gh_attestation_verify).
Proposed permissions below require separate owner approval before implementation:

| Job / Capability | Exact Proposed Permission / Trust | Current State |
| --- | --- | --- |
| Ordinary PR/push checks | `contents: read`; all other token permissions disabled | Unchanged; no release credentials. |
| Protected release build/attest | `contents: read`, `id-token: write` (GitHub OIDC signing identity), `attestations: write` (persist attestations); all others disabled | Proposal only; no long-lived signing key needed. |
| Separate protected draft publisher | `contents: write`; all others disabled | Later approval only; no OIDC needed just to upload existing bytes. |
| Apple signing/notarization, if chosen | Developer ID Application certificate/private key and approved notary authentication in an isolated temporary keychain | Availability, team/account, authentication method and secret names **unknown**, not inspected or provisioned. |

Require protected release refs and a protected environment with owner review;
never grant these capabilities to PR code, `pull_request_target`, or arbitrary
workflow/ref inputs. Revalidate these protections before enabling the job.
No `packages: write`, `artifact-metadata: write`, PAT or broad workflow-level
write grant is proposed for tarballs. Existing selected action SHAs are in
[Pins And Review](#pins-and-review) and [Transfer Action Review](#transfer-action-review).
GitHub now documents `actions/attest@v4`; this is **not a selected immutable pin**.
Resolve/review its exact release commit and dependencies before the later job;
no attestation action SHA or Apple tool version is invented here.

Build/sign/package once, then checksum and attest each of the four **final**
tarballs and the checksum manifest. Test and publish those same bytes; signing
changes executable bytes, so never attest only a pre-signing build. SHA-256
detects corruption/substitution against a trusted expected digest. An attacker
who replaces both an archive and its same-release manifest can pass that check;
HTTPS/API hashes from the same publisher are not independent authentication.
Attestations bind bytes to a workflow identity, not proof of safe code, honest
workflow-controlled claims, or Apple trust; a compromised authorized builder
remains a risk.

Future consumer verification, before extraction: run `gh attestation verify
"$archive" --repo primaprashant/sei` for each selected artifact/manifest, adding
`--signer-workflow "$approved_signer_workflow" --source-ref "refs/tags/$tag"
--source-digest "$approved_commit" --deny-self-hosted-runners --format json`.
Values must come from the owner-approved release record, not untrusted predicate
fields; require the SLSA provenance v1 predicate and GitHub OIDC issuer defaults.
Record verified identity, subject digest, source/ref and run; fail closed on
missing/mismatched evidence. Reusable workflows require the actual signer
identity. Local `gh 2.92.0` help confirms these flags; no release attestation was
generated/verified, and a future CI CLI install still needs its own integrity pin.
This is release verification policy, not a new installed-user `gh` dependency.

### Mac Download Gate

Reviewed Apple's [current Gatekeeper guidance](https://support.apple.com/en-us/102445),
[TN2206 download/quarantine testing](https://developer.apple.com/library/archive/technotes/tn2206/_index.html),
and [custom notarization workflow](https://developer.apple.com/documentation/security/customizing-the-notarization-workflow)
(full text via its [DocC JSON](https://developer.apple.com/tutorials/data/documentation/security/customizing-the-notarization-workflow.json)).
TN2206's app-bundle examples are not proof of this standalone CLI's behavior.

On fresh native Macs on both architectures, including the proposed macOS 13
floor, use the exact checksummed candidate and record host details from Task 28,
source URL/tag/commit/hash, browser/version, extraction tool/version, destination,
default security settings, network availability and first-launch transcript.
Test browser download plus Archive Utility/command-line extraction separately
from the eventual curl/installer path. Inspect attributes read-only with
`xattr -l "$archive"` and `xattr -l "$binary"` before first execution; record
presence **or absence** of `com.apple.quarantine` and signature diagnostics via
`codesign -dv --verbose=4 "$binary"` and `codesign --verify --strict --verbose=2
"$binary"`. Preserve statuses even on failure; then test absolute-path help,
version and PTY startup/quit with disposable HOME/config. No quarantine on a
curl/CI copy is not proof of the quarantined browser path. Avoid cached trust
by repeating from a fresh machine/VM snapshot, not just a fresh user directory.

Do not remove/set quarantine to manufacture a result, disable Gatekeeper, add
trust exceptions, or use Open Anyway. Ordinary identified-developer consent is
distinct from a security override; record the actual prompt. No Mac evidence is
available here. If the promised flow fails, recommend Developer ID signing and
notarization and obtain the owner's decision; do not publish bypass guidance.

If signing is needed, these small follow-ups **block Task 34**:
1. Approve identity/authentication availability, scoped secret handling, runner
   access and cleanup; record actual macOS/Xcode/codesign/notarytool versions.
2. Implement and verify Developer ID signing for both binaries (including
   hardened-runtime/timestamp requirements), then notarization with `notarytool`,
   accepted status and reviewed logs. Apple accepts ZIP/UDIF/signed flat packages,
   not the current tar.gz as a submission container; verify a submission ZIP
   containing the exact signed payload and its final tar.gz distribution path.
3. Resolve ticket delivery and repeat fresh download tests before freezing final
   checksums/attestations. Apple cannot staple standalone binaries or ZIPs;
   online ticket lookup is not an offline-first-launch guarantee. Any packaging
   change needs owner approval and archive/installer contract tests. Phase I's
   macOS offline exception does not waive this separate trust decision.

### ShellCheck Provenance

Approved **v0.11.0** only. Reviewed [primary release](https://github.com/koalaman/shellcheck/releases/tag/v0.11.0)
and [asset API digests](https://api.github.com/repos/koalaman/shellcheck/releases/tags/v0.11.0)
on 2026-09-06. Selected gzip assets (no xz dependency):

| Asset `shellcheck-v0.11.0.<platform>.tar.gz` | API Asset ID | SHA-256 |
| --- | --- | --- |
| `linux.x86_64` | 336391469 | `b7af85e41cc99489dcc21d66c6d5f3685138f06d34651e6d34b42ec6d54fe6f6` |
| `linux.aarch64` | 336391401 | `68a8133197a50beb8803f8d42f9908d1af1c5540d4bb05fdfca8c1fa47decefc` |
| `darwin.x86_64` | 336391375 | `c2c15e08df0e8fbc374c335b230a7ee958c313fa5714817a59aa59f1aa594f51` |
| `darwin.aarch64` | 336391359 | `339b930feb1ea764467013cc1f72d09cd6b869ebf1013296ba9055ab2ffbd26f` |

Future local/CI installation from repository root on Linux/macOS with curl,
tar/gzip and `shasum` available; stop on failure, no remote installer or fallback:

```sh
set -eu
case "$(uname -s)/$(uname -m)" in
  Linux/x86_64) platform=linux.x86_64; digest=b7af85e41cc99489dcc21d66c6d5f3685138f06d34651e6d34b42ec6d54fe6f6 ;;
  Linux/aarch64) platform=linux.aarch64; digest=68a8133197a50beb8803f8d42f9908d1af1c5540d4bb05fdfca8c1fa47decefc ;;
  Darwin/x86_64) platform=darwin.x86_64; digest=c2c15e08df0e8fbc374c335b230a7ee958c313fa5714817a59aa59f1aa594f51 ;;
  Darwin/arm64) platform=darwin.aarch64; digest=339b930feb1ea764467013cc1f72d09cd6b869ebf1013296ba9055ab2ffbd26f ;;
  *) exit 1 ;;
esac
mkdir -p .bin
archive=".bin/shellcheck-v0.11.0.$platform.tar.gz"
curl --fail --location --proto '=https' --proto-redir '=https' --tlsv1.2 \
  -o "$archive" "https://github.com/koalaman/shellcheck/releases/download/v0.11.0/${archive##*/}"
printf '%s  %s\n' "$digest" "$archive" | shasum -a 256 --check -
tar -xzf "$archive" -C .bin --strip-components=1 shellcheck-v0.11.0/shellcheck
./.bin/shellcheck --version
test "$(./.bin/shellcheck --version | grep '^version:')" = 'version: 0.11.0'
```

Local Linux x86_64 archive matched the API digest **before listing/extraction
or execution**; ignored `.bin/shellcheck` reports `0.11.0`, executable SHA-256
`4da528ddb3a4d1b7b24a59d4e16eb2f5fd960f4bd9a3708a15baddbdf1d5a55b`.
Other platforms have reviewed API pins only, not downloaded/native execution
evidence. No independent signature/attestation verification is claimed.
Version checks also report Go `1.27.1`, golangci-lint `2.13.2`, Community
GoReleaser `2.18.0`; existing pins remain unchanged.

Task 29 added no installer. Task 30 implementation follows below.

## Task 30 Fresh Installer

Historical Task 30 baseline: the fresh-only replacement rules below are superseded
by [Task 31 verified upgrades](#task-31-verified-upgrades). Other input/download
and archive validation contracts still apply.

Implemented locally after `b822902`, with owner approval to continue local/CI
work despite the separate Task 28 floors/support approval and Task 29 Mac
Gatekeeper blockers. **No public release exists.** No installer asset is uploaded,
release workflow enabled, support claim made, or public install URL advertised.
This turn does not authorize commits or pushes.

`scripts/install.sh` is POSIX sh, invoked with `sh scripts/install.sh`. It accepts
`--version latest` (also the default), an explicit `vMAJOR.MINOR.PATCH` with
optional SemVer prerelease identifiers, and `--install-dir DIR` (default
`$HOME/.local/bin`). Numeric core components and numeric prerelease identifiers
cannot have leading zeros. Build metadata, unprefixed versions, whitespace,
URL separators/query/fragment characters, empty values and repeated options
are rejected. Control characters in install paths are rejected; ordinary spaces,
apostrophes and relative paths are supported. `--help` prints usage only.

Current uname mappings (Task 32 adds explicit aliases, not emulation):

| uname -s / uname -m | Asset Target |
| --- | --- |
| Linux / x86_64 or amd64 | linux_amd64 |
| Linux / aarch64 or arm64 | linux_arm64 |
| Darwin / x86_64 or amd64 | darwin_amd64 |
| Darwin / arm64 or aarch64 | darwin_arm64 |

Latest is resolved in one curl invocation against the fixed repository's
`/releases/latest`, requiring the effective URL to be its `/releases/tag/<tag>`
with a validated concrete tag. Both subsequent downloads use that same tag,
never a moving `/latest/download` URL. Explicit versions skip resolution.
curl disables curlrc loading with `-q`, fails HTTP errors, and permits only HTTPS
initial/redirect URLs with TLS 1.2 or later. No fake endpoint or application
network configuration override exists.

Prerequisites are POSIX shell/core utilities (`awk`, `chmod`, `cmp`, `mkdir`,
`mv`, `rm`, `sed`, `uname`, shell `printf`/`pwd`/`test`), plus `curl`, GNU/BSD
`tar`, `gzip`, `mktemp`, and either `sha256sum` or macOS's `shasum -a 256`.
No sudo, shell-profile edits, package manager, Go, agent, Node or Python runtime
is needed. `shasum` uses the host's existing utility implementation; the installer
does not download a Perl/runtime package. Success prints the absolute installed
path and an executable single-quoted absolute invocation, escaping apostrophes.
It also checks for the resolved absolute install directory as an exact PATH
component. If absent, it prints `export PATH='<dir>':"$PATH"`, with the directory
shell-quoted and `$PATH` left literal for the user's current shell. It prepends
the directory without adding a leading empty component and preserves the user's
existing PATH value. Relative or differently spelled aliases are not treated as
exact matches. Colon-containing directories remain valid install destinations,
but cannot be a single PATH component: the installer prints that limitation and
the absolute invocation, never an export. The installer never edits profiles or
claims PATH already changed.

### Historical Fresh-Only Ownership

1. Refuse **any** existing `sei` or `.sei-install-receipt` entry before download,
   including directories, symlinks and dangling links. Never run an existing
   executable to identify it. Recheck both paths before committing.
2. Create a private `mktemp -d` stage inside the resolved install directory with
   umask 077. All downloaded and prepared files stay on that filesystem. Normal
   exit and HUP/INT/TERM remove only this private stage. A failed install may leave
   newly created empty parent directories; it never removes user directories.
3. Require exactly one selected archive manifest entry with a lowercase 64-hex
   SHA-256 and no extra fields. Hash downloaded bytes before inspecting tar.
   Verify gzip integrity, then require exactly `sei`, `README.md`, `LICENSE`,
   `THIRD_PARTY_NOTICES`, each once and ordinary. Name listings are checked
   verbatim, including members after zero padding via `--ignore-zeros`; verbose
   listings use only GNU/BSD tar's common leading type byte, not differing owner/date columns.
   Links, specials, directories, duplicates,
   extra/missing members and non-root/traversal paths fail closed.
4. Only after both listings pass, run `tar -xOzf ... sei` to a private regular
   file selected by the installer, never general filesystem extraction. Make it
   0755, require successful `--version`, matching `sei <version>` output and empty
   stderr, then hash the candidate executable. Only a checksummed candidate runs;
   this is validation of trusted release content, not a sandbox for hostile code.
5. Write a private 0600 receipt, then move it beside the binary **before** moving
   the prepared executable to its final path. The receipt format is exactly:

```text
sei-install-receipt-v1
v0.1.0 <64-lowercase-hex SHA-256 of the executable, not the tarball>
```

The concrete tag and digest are validated before recording. A receipt commit
failure installs no binary; final binary-move failure retains the valid prepared
receipt. Retrying currently refuses that receipt too, with manual inspection and
relocation guidance. No cleanup trap deletes committed receipt/binary paths.
Task 31 now validates these records against existing executable bytes and retains
old plus candidate records before upgrades. There is still no broad overwrite switch.

Same-publisher checksums and local receipts are not independent publisher
authentication; the Task 29 attestation policy remains separate. No hostile
concurrent-writer isolation, crash durability, forced-kill cleanup, or automatic
rollback is promised. Both final moves are on the installation filesystem.

### Historical Tests And Evidence

`installer_test.go` uses Go `testing`/`os/exec`, disposable HOME/config/paths,
local Go-generated tar/gzip fixtures and PATH-local curl/uname/mv mocks. Shell
fixture executables allow every host mapping to be tested without executing
foreign architecture binaries. These tests are not native Mac or actual public
release evidence. They compare downloaded source bytes, receipt contents and
preserved existing entries; execute the printed apostrophe-containing command;
assert latest resolves once and both asset URLs share its tag; and exercise
malformed options/versions/checksums/archives, existing symlink/directory paths,
candidate version rejection and failures at both placement commits.

All four native CI jobs now install ShellCheck **v0.11.0**, verifying the four
approved [platform hashes](#shellcheck-provenance) before extraction/execution,
assert its exact version, and run `sh -n` plus `shellcheck -s sh`. Ordinary native
Go tests include the installer suite, using each host's own tar and shell. No
permissions, secrets or publication capabilities changed. Native hosted execution
at that checkpoint was pending; Phase J evidence below records the later run.
Task 32 coverage and its separate pending native gate are recorded below.

Local Linux amd64 verification: installer syntax/pinned ShellCheck and focused
offline fixtures pass; module tidy diff, linter config/format/lint (zero issues),
vet, full uncached tests and full CGO-enabled race suite pass. Cross-build and
final verification results are recorded in the test matrix. No live release
download or macOS Gatekeeper test was attempted.

### Task 30 Review Fix

Independent review found that BSD tar 3.7.4 rejects GNU's `-i` short option and
that absolute invocation alone omitted required PATH guidance. Both listings now
use the shared `--ignore-zeros` long option; the appended-archive regression
requires the member-validation diagnostic, not just any failure. `TestInstallerPATH`
covers missing/near-match and first/middle/last exact PATH components, suppresses
unnecessary guidance, and executes the printed export in separate shells with
different and empty PATH values. It verifies `command -v sei`, `sei --version`,
apostrophes/spaces, literal `$PATH` and the resulting PATH bytes. The absolute
Run command remains independently exercised by `TestInstallerFresh`.

The entire installer suite passes on Linux with GNU tar **1.35** and isolated
BSD tar/libarchive **3.7.4** from the review agent's `/tmp/opencode/sei-review-bsdtar`.
For the latter, the test parent's PATH selects an external `tar` wrapper at
`/tmp/opencode/sei-task30-bsd-tools`, which executes that bsdtar with its isolated
library directory. No installer flag or environment seam was added. This is
Linux BSD tar compatibility evidence, **not native macOS evidence**; native CI,
Mac Gatekeeper and support-floor gates remain pending. No push was made.

Review verification: syntax and pinned ShellCheck, module tidy diff, lint
config/format/run (zero issues), vet, full uncached tests (`22.578s`) and full
CGO-enabled race (`116.626s`) pass. The complete installer suite also passes
inside an isolated Linux network namespace with each tar implementation.

## Phase J Hosted Evidence

On 2026-09-06, `gh run view 34021805952 --json headSha,conclusion,jobs,url`
verified [run 34021805952](https://github.com/primaprashant/sei/actions/runs/34021805952)
at exact head `54fbfb95daf5e39b04e8b16354e8624db4b5289c`: all seven jobs passed.
These are the archive producer, both fuzz targets, and native checks on
`ubuntu-24.04`, `ubuntu-24.04-arm`, `macos-15-intel`, and `macos-15`.
Producer steps `Build once and verify archive contract` and
`Transfer only the four archives and checksum manifest` succeeded. On every
native runner, `Standard checks`, native ShellCheck installation/lint,
`Download identical producer archives`, and
`Execute exact native archive including PTY restoration` succeeded. The workflow
transfers the producer's artifact ID and verifies its version/commit, rather than
rebuilding native test archives. Linux amd64 race and coverage steps passed;
those two steps were intentionally skipped on the other three native runners.
This is Phase J evidence, not hosted verification of the uncommitted Task 31 work.
Minimum-OS/support approval and Mac quarantine/Gatekeeper/signing decisions remain
release blockers; successful CI extraction does not establish download trust.

## Task 31 Verified Upgrades

Implemented sequentially from clean `54fbfb9`. The same installer invocation now
replaces an existing executable only when its SHA-256 matches a strictly valid
Task 30 `.sei-install-receipt`; it never executes old bytes to identify them.
Manual installs, modified bytes, missing/mismatched/malformed receipts, and
symlink (including dangling), directory or special-file executable/receipt paths
are refused with manual inspection/relocation guidance. An existing binary must
be executable. There is no broad overwrite option or new user runtime dependency.

Receipt-v1 remains the persisted format: exact `sei-install-receipt-v1` header
followed by one or two newline-terminated `vVERSION SHA256` records, with exactly
one ASCII space, validated versions and lowercase 64-hex digests. Extra fields,
blank lines, duplicate versions or duplicate digests are rejected. Identical
old/candidate records are emitted only once; conflicting version/digest pairs
are refused. Older unused records are dropped when preparing the next receipt.

All downloads, archive checks, extraction, chmod 0755 and candidate `--version`
validation finish in a private same-filesystem stage. Before committing, the
installer rechecks the live executable type, executable permission and digest,
and receipt type and digest. It writes a private 0600 receipt retaining the
recognized old record plus verified candidate record, then renames that receipt
into place **before** renaming the candidate binary. It rechecks both live paths
again between these commits. No second metadata write is needed after the binary
rename. Predictable observed path changes abort; this is not hostile concurrent
writer isolation or a guarantee across the final check/rename window.

Every installer failure before binary commit leaves valid old bytes untouched
and recognized by either the original receipt or the retained two-record receipt.
A failed final rename can be retried normally without executing old bytes for
identification. A same-version/same-digest reinstall also succeeds. A failed
**fresh** install with only a prepared receipt and no binary still requires manual
inspection/relocation; it is not inferred to be an owned installation. Cleanup
only removes private staging. No rollback, crash durability, forced-kill cleanup,
publisher authentication or hostile-local-user protection is promised. Success
guidance is printed only after the binary commit, with no preservation claim.

`TestInstallerUpgrade` covers a working `0.0.9` marker executable upgraded to
`0.1.0`, exact bytes/permissions/receipts, same-version deduplication, strict
ownership refusal, observed path replacements, and injected download, partial
write, staging, chmod, candidate output/exit/empty, checksum, receipt preparation,
receipt rename and binary rename failures. Every failed upgrade in that fault
table executes the preserved old binary and then retries through its retained
receipt. Faults exist only in test PATH tools, not installer flags or endpoints.

Local Linux checks pass: POSIX syntax, pinned ShellCheck v0.11.0, module tidy diff,
lint config/format/run (zero issues), vet, full uncached tests (`23.541s`), full
CGO-enabled race (`119.692s`), and the complete installer suite with GNU tar and
isolated BSD tar/libarchive 3.7.4 using the Task 30 wrapper above. These fixtures
also pass with both tar implementations in isolated network namespaces. Module
verification and a local build/help/version smoke pass. These fixtures are not
native Mac Task 31 execution. Task 32 results follow; Task 33 documentation is
now recorded in the test matrix;
no push, tag, credentials or publication in this task.

## Task 32 Broken Inputs

Implemented sequentially after `abdb967`; [current test matrix](test-matrix.md#task-32-installer-matrix)
records the offline checks. The installer accepts explicit amd64/arm64 uname
aliases, preserves trailing newlines while validating the resolved latest URL,
and compares candidate stdout byte-for-byte to `sei <version>\n` with empty
stderr. Missing or extra trailing newlines are failures, not normalized output.
Invocation quoting is prepared before either commit. An observed receipt/path
change after receipt commit asks for inspection rather than falsely claiming the
prepared receipt remains intact. Post-binary-commit diagnostics never claim the
old binary was preserved. Colon-containing install paths retain a quoted absolute
invocation but get an explicit cannot-add-to-PATH notice and no export, even when
existing PATH fragments would otherwise falsely match the directory.

Receipt-v1 ownership and receipt-first ordering are unchanged. PAX/GNU extension
metadata is interpreted by the host tar: effective names and ordinary-file types
must still satisfy the exact four-member contract. Only executable member bytes
are streamed into installer-selected staging; archive paths, links, metadata and
documentation are never extracted onto live paths. Checksums do not sandbox
candidate execution or independently authenticate the publisher.

Run `go test -count=1 -run '^TestInstaller' -v .` for all available shell cases,
or `go test -count=1 -run '^TestInstaller/[^/]+/Upgrade' .` for upgrades.
Signal cases use explicit pipes at each commit boundary, not sleeps; timeouts
kill the fixture process group. All host/download/failure controls are test-only
PATH mocks with no live-curl fallback, public flags or production environment hooks.
Repeated GNU/BSD suites (also network-disabled), standard/full/race, syntax and
pinned lint pass locally. CI visibly schedules these tests on all four native
runners; no Task 32 hosted/native macOS result is claimed. Minimum-OS/support,
Mac quarantine/Gatekeeper/signing decisions and live release gates remain open.
