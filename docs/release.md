# Releases

See [README](../README.md) for installation/support. Tool versions/hashes are in
`.github/workflows/ci.yml`, packaging in `.goreleaser.yaml`; use those pinned upstream tools.

## Local Snapshot

From the repository root with pinned Go/Community GoReleaser on `PATH`, `jq` and `shasum`.
This replaces disposable `dist/` output, not release assets:

```sh
set -eu
export GOTOOLCHAIN=go1.27.1 GOENV=off GOWORK=off
export GOFLAGS=-mod=readonly GOSUMDB=sum.golang.org
go mod download
go mod verify
goreleaser check
GOPROXY=off goreleaser release --snapshot --clean
cp scripts/install.sh dist/install.sh
(cd dist && shasum -a 256 install.sh > install.sh.sha256)
export SEI_RELEASE_DIST=dist
export SEI_RELEASE_VERSION=$(jq -er .version dist/metadata.json)
export SEI_RELEASE_COMMIT=$(git rev-parse HEAD)
test "$(jq -er .commit dist/metadata.json)" = "$SEI_RELEASE_COMMIT"
go test -count=1 -run '^TestRelease' -v ./internal
```

Relative `SEI_RELEASE_DIST` paths resolve from the repository root.
Archive tests need all three `SEI_RELEASE_*` inputs; downloaded bundles need their producer
version/full commit and matching checkout. All archives are inspected; only the native target runs help/version/PTY.
Dirty local snapshots are identified; CI rejects dirty binaries. `GOPROXY=off` is not network isolation.
Snapshots neither test tag publishing nor promise byte-identical rebuilds. No rehearsal tag is needed.

## Tag, Then Publish

1. Run the [development checks](development.md#checks), commit the release changes,
   and choose a new tag: `vMAJOR.MINOR.PATCH`, optionally `-rc.1` or another SemVer
   prerelease suffix. No leading numeric zeros or build metadata are accepted.
2. Create and push that tag, for example `git tag v0.2.0 <commit>` then
   `git push origin refs/tags/v0.2.0`. Do not move released tags.
3. Inspect the Draft Release workflow and resulting draft in GitHub. Tag pushes
   call reusable CI with `release: true`; ordinary push/PR CI builds snapshots.
   The producer checks tag/event/checkout identity and builds once using
   `GORELEASER_CURRENT_TAG`, `--clean --skip=publish`; GoReleaser publishing is disabled.
   Four native Linux/macOS amd64/arm64 jobs test the same artifact ID, alongside
   source, installer, PTY, Linux race and two fuzz jobs.
4. Only the final upload job has `contents: write`. `scripts/release.sh draft`
   rechecks tag/commit/version, checksums and installer source bytes, then creates
   a draft with exactly four tarballs, their manifest, `install.sh` and its checksum.
   Each tarball contains only `sei`, `README.md`, `LICENSE`, `THIRD_PARTY_NOTICES`.
5. Publish manually after checking the draft and notes. For a stable release:
   `gh release edit v0.2.0 --draft=false --prerelease=false --latest`.
   Prerelease tags produce prerelease drafts; keep them off stable/latest when
   publishing. The workflow always uses `--latest=false` and never publishes.
6. Check anonymous tag-specific downloads and, for stable releases, latest downloads
   and installation in a disposable directory. Preserve the tested asset bytes.

Existing releases/API failures stop upload; failed uploads may leave partial drafts requiring manual inspection.
Reruns do not repair drafts. Never force tags, clobber assets or silently rebuild a published version.

## Installer And Support

`scripts/install.sh` resolves latest once to a concrete tag, verifies archive SHA-256
before inspection, accepts exactly four regular members, and checks the staged
binary's exact version. It never runs an existing binary to establish ownership.
Replacement requires an executable regular `sei` matching a valid adjacent
`.sei-install-receipt` digest. The receipt is local ownership bookkeeping, not a
signature: it holds the matched old and candidate version/digest records and is
renamed before the binary, so a failed final rename can leave a prepared receipt.
Unknown binaries, missing/malformed receipts and symlinks are refused. There is no
two-file atomic transaction, rollback or hostile-writer/crash-durability guarantee.
The installer uses a private same-filesystem stage, no sudo or profile edits.

Linux/macOS amd64/arm64 are tested on current native CI runners, not every distro/minimum OS.
Releases are unsigned, unnotarized and unattested; checksums do not authenticate a compromised publisher.
CI does not establish browser-download/Gatekeeper behavior; do not bypass OS protections.
