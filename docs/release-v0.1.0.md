# v0.1.0 Public Verification

Verified 2026-09-06 on Debian Linux amd64, starting 11:24:13Z, after owner-authorized
publication. Verification used disposable installations and did not change tagged
or published bytes, real user configuration or agent paths.

## Publication Identity

- Release: https://github.com/primaprashant/sei/releases/tag/v0.1.0
- Published: `2026-09-06T11:22:35Z`; `isDraft=false`, `isPrerelease=false`.
- Tag/producer commit: `d8c5ca50af2141dc25869f0d88d56007f3baa002`.
- https://github.com/primaprashant/sei/releases/latest resolved to the release above.
- Workflow: https://github.com/primaprashant/sei/actions/runs/34029551890
- All eight jobs succeeded: producer, four native checks (Ubuntu amd64/arm64,
  macOS Intel/arm64), two fuzz jobs and draft upload. Linux amd64 race and native
  installer/archive/PTY steps passed. Inspected read-only with `gh run view` and
  `gh release view`; local tag commit and embedded four-archive provenance agree.

## Anonymous Downloads

Used `curl -q --fail --silent --show-error --location --proto '=https'
--proto-redir '=https' --tlsv1.2`, with GitHub token variables unset, no auth
headers, cookies or netrc options. All seven assets returned HTTP 200 through
**both** bases (14 complete downloads):

- `https://github.com/primaprashant/sei/releases/download/v0.1.0/`
- `https://github.com/primaprashant/sei/releases/latest/download/`

Every downloaded file passed `cmp` against the unchanged prepublication bundle
at `/tmp/opencode/sei-v0.1.0-draft`. Both routes passed `sha256sum -c` for
`install.sh.sha256` and the four-entry archive manifest. Digests also agree with
GitHub asset metadata:

| Asset | Bytes | SHA-256 |
| --- | ---: | --- |
| `install.sh` | 10146 | `33415a1d90cea7918cd68634d6bf5b27aee819d159979ae69d68293e39725b02` |
| `install.sh.sha256` | 77 | `82478c5fdb367d0caa3959757430a936451f0f453a7e22785a5804ca6f33607c` |
| `sei_0.1.0_checksums.txt` | 382 | `00a3bc6ad0d73a441a7c8e10d2015eafce9ac0f39b45339ecd35f0fa07edeec3` |
| `sei_0.1.0_darwin_amd64.tar.gz` | 2113373 | `9a07fae0788bbef05b44985c0b3ef509dea29bffed031aa375c4708071389cf8` |
| `sei_0.1.0_darwin_arm64.tar.gz` | 1957282 | `ee06e17fd0922e31f18bd660616bd622bc88bc572b4a270ffc3de284b076735a` |
| `sei_0.1.0_linux_amd64.tar.gz` | 2112804 | `809794025ed0e40438c52b39e9bb89da95a1f73240ab038bef4a84fcbd433b8e` |
| `sei_0.1.0_linux_arm64.tar.gz` | 1909787 | `c1296351ee7ad3b86bfc421b57885359b0ec2bf85dea42fb3baedcd6a512efe0` |

Anonymous GETs also returned HTTP 200 for the release notes page, latest redirect,
repository, `blob/main/README.md`, `blob/main/docs/release.md` and workflow URL.
The installer URLs in the release notes are the exact verified URLs above.
The repository README and this report document post-publication evidence;
the tagged source and bundled README remain immutable.

## Installation And Runtime

| Check | Result |
| --- | --- |
| Downloaded latest script, default version and `$HOME/.local/bin` | PASS in disposable `default-home` |
| Downloaded versioned script, `--version v0.1.0 --install-dir "$HOME/Tools/sei's bin"` | PASS in disposable `custom-home`; printed shell quoting correct |
| README latest one-command, complete download before execution | PASS in separate disposable `one-command-home` |
| Installed bytes versus public archive executable | PASS; SHA-256 `489a81a62ff8c3c0147a5655d2ead0044e2fd7c337272299a1c2b0db35c362a6` |
| `--version` and `--help` | PASS, exit 0, stdout only, empty stderr; version `sei 0.1.0`, expected usage/options/keys/safety text |
| Ownership receipt and same-version rerun, default and custom | PASS; canonical v1 receipt with `v0.1.0` and executable digest, bytes unchanged after rerun |
| Unavailable `v999999.0.0`, default and custom | Expected HTTP 404/nonzero failure; existing executable and receipt remain byte-identical |
| `TestInstaller`, uncached | PASS on sh/bash/dash; existing mock upgrade, bad candidate version, truncated download, checksum, ownership and interruption fixtures |
| `TestReleaseArchives`, public versioned bundle | PASS all four hashes, members and build metadata; native Linux amd64 help/version/PTY; `vcs.modified=false` |
| Existing `testPTYFreshUser` on exact publicly installed executable | PASS, normal and network-disabled runs |

The temporary `TestPublicInstalledFreshUser` wrapper called only
`testPTYFreshUser(t, "/tmp/opencode/sei-v0.1.0-public/default-home/.local/bin/sei")`.
It was added and removed with `apply_patch`; no permanent application/test edits.
The harness compiled as a test runner, but the application under test was the
installed public executable, not a `go build` substitute. The full flow covered
first-run native config save, three-agent add/two-agent remove, restart, focus/help,
paste rejection, external change/refresh, resize below minimum and recovery, quit
and terminal restoration. It asserted config/library preservation, independent
nested/hidden/empty copies and untouched absent global destinations in temporary data.

Commands used before documentation edits (archive tests compare bundled docs to source):

```sh
SEI_RELEASE_DIST=/tmp/opencode/sei-v0.1.0-public/versioned \
SEI_RELEASE_VERSION=0.1.0 \
SEI_RELEASE_COMMIT=d8c5ca50af2141dc25869f0d88d56007f3baa002 \
go test -count=1 -v -run '^(TestReleaseArchives|TestPublicInstalledFreshUser|TestInstaller)$' .
unshare -Urn env GOPROXY=off GOSUMDB=off \
  go test -count=1 -v -run '^TestPublicInstalledFreshUser$' .
```

Live install commands, after downloading the complete matching public script:

```sh
sh install.sh                         # latest, default directory
sh install.sh --version v0.1.0 --install-dir "$HOME/Tools/sei's bin"
```

The [README one-command](../README.md#one-command) is the safe complete-download
entry point. Local raw evidence is retained at `/tmp/opencode/sei-v0.1.0-public/`:
`verification.log`, `tests.log`, `offline-pty.log`, captured help/version streams,
both downloaded bundles and disposable installs. The download/install driver is
`/tmp/opencode/verify-sei-public.sh`.

## Limitations

Local native execution is Linux amd64 only; the other targets retain their passing
native release CI evidence, not new personal Mac or Linux arm64 download execution.
Additional personal Mac/trust, external minimum-OS/floor, human and benchmark gates
are waived, not passed. No signing, notarization, attestations, Gatekeeper bypass,
Windows, Homebrew or self-update. Same-release checksums detect corruption, not a
compromised publisher. Offline PTY confirms this workflow without network access,
not exhaustive runtime auditing. Live upgrade smoke is same-version plus unavailable
release preservation; cross-version success/failures use existing offline fixtures.
Bundled README and CLI help retain their original development wording; published
bytes were deliberately not replaced. Any future artifact defect requires a new
version through the existing checks, not silent asset/tag replacement.
