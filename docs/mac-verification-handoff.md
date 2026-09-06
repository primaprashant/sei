# Mac Verification Handoff

**SUPERSEDED:** The [owner scope override](../prd.md#owner-scope-override) waives
the additional personal Mac artifact/trust/floor and human gates below. This is
an optional historical runbook, not a current assignment or prerequisite for
Task 34. Do not request credentials, Apple signup or signing follow-ups from it.
Actual limited source-suite results are preserved in [Mac verification](mac-verification.md).

---

Verify sei on this actual Mac before Phase L release work. This is verification,
not permission to publish, change the support promise, disable security checks,
or access signing credentials. Carry out safe automated checks, request human
browser/terminal interaction where necessary, and report blockers honestly.

## Context And Boundaries

- Repository: `https://github.com/primaprashant/sei`.
- Phase K source: `11aa829251221cee049b12acbf6e784bfcdcd051`.
- Passing CI run: `34025005746`; all seven jobs passed, including native Intel
  and Apple Silicon jobs, installer tests, and exact archive execution.
- Artifact ID: `9986772923`; name:
  `sei-archives-11aa829251221cee049b12acbf6e784bfcdcd051-1`.
- Version: `0.0.0-snapshot.11aa829`. No public sei release exists. Never present
  snapshot checks as an actual published-release or unauthenticated URL test.
- The exact Linux archive passed on Debian 13.6 amd64, kernel
  `6.12.90+deb13.1-cloud-amd64`, including offline workflow and installer checks.
  This did not establish Ubuntu 22.04/5.15 or Linux arm64 support.
- The separate Phase I macOS offline run was waived; do not disconnect this Mac
  or require a VM just to repeat it. That waiver does NOT cover Gatekeeper,
  support floors, or signing/notarization decisions.
- Preserve all existing worktree changes. No commits, pushes, tags, publication,
  credentials, sudo, profile edits, installs into real agent roots, or filesystem
  cleanup outside your own disposable fixtures without separate authorization.
- Read `implementation-plan.md`, `docs/release.md`, `docs/test-matrix.md`,
  `release_test.go`, `setup_process_test.go`, `installer_test.go` and
  `installer_signal_test.go` before improvising a harness.

## 1. Record Host And Prepare

Locate the user's existing repository and record `git status --short` and HEAD.
Record these commands' output, including failures, in a verification report:

```sh
uname -a
sw_vers
sysctl -n machdep.cpu.brand_string
sysctl -in sysctl.proc_translated 2>/dev/null || true
diskutil info .
ssh -V
printf 'TERM=%s\n' "${TERM:-unset}"
test ! -t 0 || stty size
tar --version
command -v hdiutil
```

Only the current native architecture is personal-host evidence. If translated
execution reports `1`, use a native terminal/toolchain before proceeding. Never
claim Intel execution from Rosetta on Apple Silicon, or macOS 13 from a newer OS.
Record exact terminal application/version, dimensions, and theme when known.

Prepare the pinned Go 1.27.1, golangci-lint 2.13.2 and ShellCheck 0.11.0 using
the repository's integrity-pinned instructions. Verify downloads before running
them; use private tooling paths, not system modifications. Prepare modules while
online. If a required tool is unavailable, ask or record the blocker; no silent
version substitutions. Do not run GoReleaser to replace the artifacts below.

The checker compares archive documentation against source files. Use the exact
producer checkout. If HEAD differs or has edits, create a disposable detached
worktree at the producer commit, after verifying its parent directory; never
reset/stash/overwrite the user's checkout. Keep final notes in the main checkout.

## 2. Download The Exact Bundle

Use an already-authenticated `gh`, or ask the owner to authenticate without
exposing tokens. From the matching checkout, create a disposable artifact dir:

```sh
artifact_dir=$(mktemp -d)
gh api repos/primaprashant/sei/actions/artifacts/9986772923
gh run view 34025005746 --repo primaprashant/sei \
  --json headSha,status,conclusion,jobs
gh run download 34025005746 --repo primaprashant/sei \
  --name sei-archives-11aa829251221cee049b12acbf6e784bfcdcd051-1 \
  --dir "$artifact_dir"
```

Check every command succeeds. Expected service-bundle digest:
`sha256:e5db928c8ae00c78b7c94cdf3c4ccf5dc57794190da7b5eff37593e71b3bc29f`.
This is not an individual tarball hash. GitHub reported expiry on
2026-09-13; if expired, stop and request preserved original artifacts or a new
explicitly identified CI run. Do not rebuild under the old identity.

The bundle must have four tarballs and one checksum manifest. Check all manifest
entries with `shasum -a 256 --check` from its directory, and compare the native
archive against this separately recorded digest:

| Archive | SHA-256 |
| --- | --- |
| `sei_0.0.0-snapshot.11aa829_darwin_amd64.tar.gz` | `24b11595cfedef68afcc8b35206b28514fe4660da1aaf0f6f637ba3f1a6bb865` |
| `sei_0.0.0-snapshot.11aa829_darwin_arm64.tar.gz` | `e471c240772bf5bebdc10d47cbee2da4441e8758a76951b4124ce6f9f8e5b8b6` |

## 3. Automated Native Checks

```sh
export GOTOOLCHAIN=go1.27.1 GOFLAGS=-mod=readonly
go version
go env GOHOSTOS GOHOSTARCH GOOS GOARCH
go mod download
go mod verify
go mod tidy -diff
go vet ./...
sh -n scripts/install.sh
./.bin/shellcheck -s sh scripts/install.sh
go test -count=1 -v -run '^TestInstaller' .
go test -count=1 -v -run 'TestPTY|TestConfigSaveFailurePTY' .
go test -count=1 -v -run '^TestCaseCollision$' .
go test -count=1 ./...

env CI=true SEI_RELEASE_DIST="$artifact_dir" \
  SEI_RELEASE_VERSION=0.0.0-snapshot.11aa829 \
  SEI_RELEASE_COMMIT=11aa829251221cee049b12acbf6e784bfcdcd051 \
  GOTOOLCHAIN=go1.27.1 \
  go test -count=1 -run '^TestRelease' -v .
```

Run pinned lint config/format-diff/lint checks too. If tooling is in another
private path, adjust the command explicitly. Retain output/status and skip
reasons. Do not label these runs offline: module preparation and general host
networking remain available; installer download mocks are local-only.

The matching native archive PTY subtest must RUN, not only foreign metadata
checks. `TestCaseCollision` uses `hdiutil` to provision disposable HFSX/HFS+
volumes; attachment failure is not permission to weaken assertions. Invalid
UTF-8 name tests may skip on Mac filesystems; distinct-case-name fixtures may
skip on case-insensitive storage. Those are complementary to Linux coverage.
Unavailable `dash` may skip; native `/bin/sh` and Bash POSIX tests must run.

## 4. Complete Workflow On The Exact Binary

`TestRelease` currently tests only early native help/version and enter/quit.
Do not confuse source PTY tests, which may build children, with final artifact
workflow evidence. Reuse `testPTYFreshUser(t, binary)` through a narrowly scoped
temporary Go test, passing the checksum-verified extracted native executable.
Run it from the producer worktree, then remove only your temporary test.

Also rehearse the actual `scripts/install.sh` with test-only PATH download mocks
serving the UNMODIFIED native archive and manifest. Use the correct
`v0.0.0-snapshot.11aa829` URL/version shape; no public endpoint exists.
Verify fresh install into a private quoted path, receipt-owned reinstall,
injected download/final-rename failures, unchanged usable binary and receipt,
and successful retry. Existing version-changing/interrupt fixtures remain
separate evidence; do not invent a second released version or modify the
verified executable. No production flags or network overrides should be added.

All fixture HOME/config/project/library/destination paths must be disposable.
Use `~/skill-library`, NOT `~/library`, which aliases macOS's native `~/Library`
config ancestor on case-insensitive storage. Run children with no agent tooling
on PATH. Capture bytes/digests, target contents, library invariance and terminal
restoration; do not execute skill scripts.

## 5. Real Mac Download Trust

This requires human browser interaction if the coding agent lacks browser tools.
Ask the owner to use Safari (record version) to download the ORIGINAL artifact
from the Actions run page:
`https://github.com/primaprashant/sei/actions/runs/34025005746`.
GitHub serves an outer ZIP; record that wrapping and extraction chain. Do not
pretend it is the future direct release tarball URL. The inner native tarball
must still match the digest above.

Use new disposable extraction locations. Inspect attributes READ-ONLY before
any first execution, on the browser-downloaded ZIP, inner tarball and extracted
binary. Test Archive Utility and native command-line tar extraction separately
where possible, keeping file provenance clear:

```sh
xattr -l "/exact/downloaded/file"
xattr -l "/exact/extracted/sei"
codesign -dv --verbose=4 "/exact/extracted/sei"
codesign --verify --strict --verbose=2 "/exact/extracted/sei"
spctl --assess --type execute --verbose=4 "/exact/extracted/sei"
```

Capture each status; do not chain expected diagnostic failures so subsequent
observations are lost. An ad-hoc signature is not Developer ID/notarization.
`spctl` results alone do not establish actual CLI launch behavior. After recording
attributes/signature evidence, test absolute-path help/version and terminal
startup/quit with disposable HOME, preserving error output and prompts.

NEVER remove/set quarantine to manufacture evidence, run `xattr -d`/`-c`, disable
Gatekeeper, use Open Anyway, ad-hoc re-sign, or access Apple keys/accounts. If
macOS blocks execution, record the exact error and recommend the appropriate
signing/notarization follow-up; do not bypass it. If quarantine is absent or
lost through the Actions ZIP wrapping, record the trust test as incomplete.

A normal Mac with cached trust is not a clean-machine/VM snapshot. Record this
limitation honestly; lack of a VM does not prevent useful personal-host evidence.
Direct published browser/curl/installer paths and clean-machine trust remain
release gates unless actually tested. Do not publish just to obtain a URL.

Ask the owner to inspect the exact binary's UI in Terminal.app light/dark/no-color
profiles, at >=80x24 and the normal larger size. Record target readability,
help/long names, resize and quit behavior separately from automated PTY output.
Only mark scenarios the owner actually observed. Do not claim the timed shell
comparison or SSH acceptance from this local check.

## 6. Report And Handoff

Write `docs/mac-verification.md` with host/architecture/filesystem, producer
commit/run/artifact/hash, tool versions, commands/statuses, skips, exact-binary
workflow and installer evidence, download chain, quarantine/signature/Gatekeeper
observations, human feedback, and unresolved blockers. Update
`implementation-plan.md` and `docs/test-matrix.md` VERY concisely. Do not commit
or push unless separately asked. No secrets or private skill content in logs.

Only mark the tested Mac OS and architecture verified. Leave macOS 13, the
other personal architecture, clean-machine trust, direct release URLs, Apple
signing decisions and release permission grants open wherever untested. Do not
change the PRD/support table or waive a gate without explicit owner approval.
For a concrete code defect, explain it, add a minimal regression/fix, and rerun
relevant checks; a source fix does not retroactively repair the old artifact.

Return a concise PASS / FAIL / BLOCKED table, files changed, exact artifact
identity, any needed owner decisions, and whether Task 34 can proceed. Preserve
original artifact bytes until evidence is reviewed; clean only your own temporary
fixtures and detach only disk images the tests created.
