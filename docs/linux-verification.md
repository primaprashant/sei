# Debian Verification

2026-09-06, actual Debian 13.6 (trixie), native amd64, kernel
`6.12.90+deb13.1-cloud-amd64`, UID/GID 1000. Repository filesystem: ext4;
disposable `/tmp` fixtures: tmpfs. Go: `go1.27.1 linux/amd64`.
OpenSSH: `OpenSSH_10.0p2 Debian-7+deb13u4` (version capture, not an SSH session test).
Source started clean at `11aa829251221cee049b12acbf6e784bfcdcd051`.

## Exact Artifact

[Successful CI run 34025005746](https://github.com/primaprashant/sei/actions/runs/34025005746),
artifact **9986772923**, named
`sei-archives-11aa829251221cee049b12acbf6e784bfcdcd051-1`.
GitHub API reports service digest
`sha256:e5db928c8ae00c78b7c94cdf3c4ccf5dc57794190da7b5eff37593e71b3bc29f`;
this identifies the service bundle, not an individual tarball checksum.
API metadata and all seven successful job conclusions were rechecked with
`gh api repos/primaprashant/sei/actions/artifacts/9986772923` and
`gh run view 34025005746 --json headSha,status,conclusion,jobs`.
Artifact expiry reported by GitHub: 2026-09-13T09:34:53Z.

The already-downloaded, unchanged bundle in
`/tmp/opencode/sei-debian-34025005746` contains exactly four archives and
`sei_0.0.0-snapshot.11aa829_checksums.txt`. No release rebuild or publication.

| Archive Suffix (Prefix `sei_0.0.0-snapshot.11aa829_`) | SHA-256 |
| --- | --- |
| `linux_amd64.tar.gz` | `f1de92dc846a255905a7ed67383549c23c3650418fd04895564545bafc2f6129` |
| `linux_arm64.tar.gz` | `eb036e9cf38ab4725182211f0c23d205757f4f568a6d1c618c180040b13be85d` |
| `darwin_amd64.tar.gz` | `24b11595cfedef68afcc8b35206b28514fe4660da1aaf0f6f637ba3f1a6bb865` |
| `darwin_arm64.tar.gz` | `e471c240772bf5bebdc10d47cbee2da4441e8758a76951b4124ce6f9f8e5b8b6` |

`sha256sum -c sei_0.0.0-snapshot.11aa829_checksums.txt` in that directory
printed `OK` for all four. `tar -tvzf` on the native archive listed only
LICENSE (1071 bytes), README.md (25274), THIRD_PARTY_NOTICES (6403), all mode
0644, and sei (5181600), mode 0755. `TestReleaseArchives` additionally validates
all four listings/types/content, gzip trailers, dependency provenance, embedded
commit, Go version, CPU baselines, stripped ELF/Mach-O architecture and static ELF
contract. All report `vcs.modified=false` with `CI=true`.

```sh
env CI=true \
  SEI_RELEASE_DIST=/tmp/opencode/sei-debian-34025005746 \
  SEI_RELEASE_VERSION=0.0.0-snapshot.11aa829 \
  SEI_RELEASE_COMMIT=11aa829251221cee049b12acbf6e784bfcdcd051 \
  GOTOOLCHAIN=go1.27.1 go test -count=1 -run '^TestRelease' -v .
```

Original result: `ok github.com/primaprashant/sei 0.422s`; native help/version
with empty PATH and early PTY restoration passed. Other targets were metadata-only
on this machine, explicitly logged as requiring matching native hosts.

## Native Workflow

A temporary `TestExactDebianTemporary` first reran the complete archive validator,
then extracted the native executable without changing bytes. Executable SHA-256:
`432e8c5796b85cff38ad7f9a59de3d9a12df52c4630e293224e1ef0a5874540a`.
The temporary test was removed after verification; no persistent test or
application changes were needed.

| Check | Observed Result |
| --- | --- |
| Existing `testPTYFreshUser` on exact executable | PASS: first-run native config save, add-three/remove-two, restart, help/focus, paste rejection, external change/refresh, resize 200x40 to 79x24 and back, quit and terminal restoration. Config/library identity and bytes, independent nested/hidden/empty copies, absent untouched global destinations asserted. |
| Empty chroot | PASS: exact executable was the only file; help/version matched with empty PATH, no loader/libraries/tools inside, and a separate network namespace. Root mapping was confined to an unprivileged user namespace. |
| Actual `scripts/install.sh` under sh | PASS: disposable quoted destination, local curl fixture served the unmodified CI tarball and original four-entry manifest at exact snapshot-tag URL patterns. Fresh install, receipt-owned same-version reinstall, injected download/final-binary-rename failures and retry all retained exact executable and receipt bytes and ran the expected version. No remote release endpoint was contacted. |

Command used while the temporary test existed:

```sh
unshare --user --map-current-user --net env PATH="$PATH:/usr/sbin" \
  CI=true SEI_RELEASE_DIST=/tmp/opencode/sei-debian-34025005746 \
  SEI_RELEASE_VERSION=0.0.0-snapshot.11aa829 \
  SEI_RELEASE_COMMIT=11aa829251221cee049b12acbf6e784bfcdcd051 \
  GOTOOLCHAIN=go1.27.1 GOPROXY=off GOSUMDB=off \
  go test -count=1 -run '^TestExactDebianTemporary$' -v .
```

Original final output: `--- PASS: TestExactDebianTemporary (2.70s)`,
`fresh-user (0.95s)`, `empty-chroot (0.08s)`, `sh (1.28s)`;
`ok github.com/primaprashant/sei 2.744s`.
First attempt failed solely because `chroot` was absent from the harness PATH;
adding `/usr/sbin` fixed tool discovery. Application PATH remained empty.

## Source Checks

Commands ran from the repository root with Go 1.27.1. Durations below are original
Go package output, not isolated performance benchmarks; some checks ran concurrently.

| Command | Result |
| --- | --- |
| `go test -count=1 ./...` | PASS, `49.458s` |
| `CGO_ENABLED=1 go test -race -count=1 ./...` | PASS, `146.686s` |
| `unshare --user --map-current-user --net env GOTOOLCHAIN=go1.27.1 GOPROXY=off GOSUMDB=off go test -count=1 -run '^(TestInstaller\|TestPTY)' .` | PASS, `39.734s`; sh/bash/dash installer matrix and PTY suite, network disabled |
| Same isolation with `PATH="/tmp/opencode/sei-task30-bsd-tools:$PATH"` and `-run '^TestInstaller'` | PASS, `26.581s`; existing isolated BSD tar fixture tools |
| `go mod tidy -diff`, `go mod verify`, `go vet ./...` | PASS; no module diff, `all modules verified` |
| `./.bin/golangci-lint config verify --config .golangci.yml`, `fmt --config .golangci.yml --diff`, `run --config .golangci.yml --timeout=5m ./...` | PASS, no formatting diff, `0 issues.` |
| `sh -n`, `dash -n`, `bash --posix -n scripts/install.sh`; `./.bin/shellcheck -s sh scripts/install.sh` | PASS (each syntax invocation targets the installer) |
| `go build -o /tmp/opencode/sei-debian-source-check .` | PASS; separate source compilation only, never substituted for CI archive execution |

Existing installer suites cover version-changing upgrade and broader failure,
ownership, unsafe-archive and interruption preservation with fixture executables.
The exact CI installer rehearsal above is specifically same-version preservation.
Source PTY helpers can build their own children; full race does not instrument the
shipped CI binary. All application/config/agent fixtures use disposable homes/trees.

## Limits

The owner now accepts this Debian amd64 testing as sufficient personal-tool Linux
acceptance under the [scope override](../prd.md#owner-scope-override), without an
all-distro/kernel-floor promise. The actual host is Debian 13.6 (trixie); it is not
relabeled Debian testing. Ubuntu 22.04/5.15 and macOS 13 remain untested, with those
extra floor gates WAIVED rather than passed.
Case-insensitive target cases remain capability-dependent; `TestCaseCollision`
explicitly skipped on this case-sensitive filesystem. Native arm64/Mac CI job success
is separate hosted evidence, not personal Mac acceptance. No manual terminal/theme,
real SSH workflow, Mac quarantine/Gatekeeper/signing, live release URL, or new
performance/fuzz-duration claim is made. Offline namespace/chroot tests are not a
network-syscall audit. No commit, push, tag, or release was made.
