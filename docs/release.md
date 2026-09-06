# CI And Release Evidence

## Scope

Task 4 establishes ordinary PR/push CI. Task 27 adds local snapshot packaging
with GoReleaser Community v2.18.0, not signing, tagging, uploading or publishing.
Commits, pushes, credentials, branch protection, tags, and publication require
explicit owner authorization. Task commits are authorized; pushing and publication
are not.

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
| Ubuntu 24.04 CI labels | Pending remote archive run | Pending remote archive run | Newer-host evidence only, even when successful. |
| macOS 15 CI labels | Pending remote archive run | Pending remote archive run | Not macOS 13 or Gatekeeper evidence. |
| Local Debian, kernel 6.12.90+deb13.1-cloud-amd64 | Local verification below | Unavailable | Not an Ubuntu floor. |

macOS quarantine/download/extraction/Gatekeeper validation remains Task 29's
release blocker. CI extraction cannot establish the end-user trust path; do not
remove quarantine or disable security checks to turn it green. No remote run of
this change was available or initiated. Earlier native source CI success does
not establish exact-archive execution for this worktree.

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
