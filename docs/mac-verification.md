# Mac Verification

Owner-reported results from the Mac coding agent; the full transcript and source
commit SHA were not supplied. Do not assign this run an inferred producer SHA.

## Host And Command

- macOS 15.7.2, native Apple M1/arm64, Go 1.27.1.
- Disposable HOME/config/test paths; worktree reported clean after testing.
- Command: `go test -count=1 -v ./...`.
- Result: `PASS`, exit 0, `ok github.com/primaprashant/sei 260.798s`.

## Results

| Check | Reported Result |
| --- | --- |
| Full uncached source suite | PASS, approximately 4m 21s |
| Installer matrix | PASS under native sh, Bash POSIX and dash; 223.07s |
| PTY setup/add/remove/restart/restoration | PASS |
| Busy quit and failure diagnostics | PASS |
| Case-collision tests | PASS; all five subtests executed |

Six invalid-UTF-8 filename cases and one distinct-case-name fixture skipped
because the filesystem does not support those fixtures. These complement Linux
coverage; the opposite case-collision behavior executed on this Mac.
Two opt-in prototype/performance tests and `TestReleaseArchives` skipped because
their artifact inputs were not supplied. Those skips are unperformed checks,
not capability-based passes.

## Limits And Scope

The source SHA remains unknown. This is source-suite evidence, not personal
exact-artifact, Mac race, macOS 13, Intel or download/Gatekeeper evidence.
Existing native CI and the Linux race gate remain separate recorded checks.

The [owner scope override](../prd.md#owner-scope-override) WAIVES extra personal
Mac artifact/trust/floor checks, signing and attestations; none is marked passed.
The [old handoff](mac-verification-handoff.md) is superseded and optional only.
No additional Mac work is required before Task 34. Public URLs and publication
remain future, separately authorized work.
