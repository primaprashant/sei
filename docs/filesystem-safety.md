# Filesystem Safety

Task 8 provides root observation; Tasks 13-15 add rooted removal, copying, and
delete-then-copy replacement. Inspection remains usable while affected mutations
are blocked when safety is unprovable. An unavailable library may still permit
removal when its boundaries are provable.

## Algorithm

1. Preserve absolute raw configuration paths, including `symlink/..`. Configuration
   expansion never cleans them before traversal. Local configuration must pass
   lexical containment, and root validation checks both lexical and physical
   project containment again.
2. `resolveRoot` traverses one raw component at a time with `Lstat`. Each existing
   component must resolve to a directory. For links, `Stat` rejects dangling links,
   loops, permission failures, and non-directory targets before `EvalSymlinks`
   resolves that component. Only then is the verified component normalized. Thus
   `link/..` means the parent of the link target, not the parent of the link name.
   Configured-root links are permitted; skill links are not.
3. Ordinary absence retains the deepest verified directory and missing suffix.
   Permission errors, dangling links, loops, and non-directory ancestors are not
   absence. A `..` after a missing component is conservatively unprovable, since
   actual traversal currently fails; sei does not invent a cleaned target for it.
4. Record identities of the existing directory and its ancestors. Compare paths
   using component-aware `filepath.Rel`, never string prefixes. Supplement this
   with `os.SameFile` on existing ancestors and compare relative tails from shared
   identities. This catches aliases and actual filesystem case identity without
   case folding. Reject library/destination identity or nesting in either direction
   and every pair of overlapping destinations, across agents and scopes.
5. `resolveRoots` returns per-panel boundary errors independently of listings.
   A known local containment violation blocks that local destination, not unrelated
   globals. An unknown library or destination boundary blocks all destinations
   whose required disjointness cannot be proved. Unknown home/project protection
   also blocks dependent mutations. A missing or unreadable library listing alone
   is not an unknown boundary and does not prohibit otherwise-safe removal.
6. `revalidateSkillRoot` accepts the captured configuration, destination panel ID,
   and raw name. It resolves everything afresh, validates the name as one nonempty
   Unix filename component (not `.`, `..`, slash, or NUL), rejects an existing
   non-directory/link child, and checks protected roots. The child cannot equal or
    contain home, project, the active config location/aliases, any destination, or library, including
   identity aliases. Backslashes, control bytes, and non-UTF-8 names remain raw Unix
   names; display escaping is never an operational name.
7. `openSkillRoot` revalidates and opens only an existing destination with
   `os.OpenRoot`, comparing the handle's `Stat(".")` identity with the observation.
   The caller owns and must close the handle. Missing destinations require explicit
   future creation work; this API does not create anything.

The browser starts an independent safety command alongside scans on launch and
refresh. Generation-tagged safety messages cannot overwrite a newer refresh.
Safety warnings do not hide listings or listing errors; full reasons are in help.
`Update` and `View` perform no filesystem I/O. These observations are display state,
not cached authorization for later mutations.

## Configuration Saves

Setup validates without writing. Saving creates only config parents through rooted
handles, writes private exclusive temporary output, checks write/sync/close, and
renames to commit. Pre-commit failures preserve existing config bytes; cleanup is
best effort and newly created empty config parents may remain. There is no fallible
post-commit metadata step or crash-durability guarantee.

Config-file symlinks and nonregular files cannot be replaced. Placement must not
overlap any managed root, including physical aliases and roots from the existing
config during reconfiguration. Parent, target, temporary-file identities and root
relationships are checked again before commit. Observed unsafe changes abort;
these checks do not lock out concurrent writers. Loading a safe config symlink for
read-only browsing remains supported.

Linux tests cover each fallible output operation, short writes, validation failures,
retargeted aliases, substituted config/temp files, unchanged populated skill trees,
and terminal-restored save-failure diagnostics. Tasks 8-12 passed native CI on all
four runners in run `33994932478`.

## Skill Mutations

Each command captures raw config paths, name, destination, scope, and operation ID.
Adds retain destination resolution and ancestor identities across preparation,
deletion, and copying. Only explicit adds create missing components, through rooted
handles followed by revalidation. Missing suffixes have no identity until created.

Both trees are fully inventoried before replacement deletion; source files are
opened, read to discard, and closed during preflight. Copying reopens verified files
and streams bytes, without buffering a source snapshot. Only regular files and
ordinary directories are accepted. Removal consumes the original postorder inventory,
rechecking identities, ancestry, protected roots, and unexpected children. It never
uses `RemoveAll`; incomplete deletion prevents copying. Files use exclusive creation
with `0666 | source execute bits`, directories `0777`, subject to umask without chmod.

Exact directory spellings and actual lookups reject known aliases before deletion.
Distinct hard-linked files remain regular files and become independent copies.
Nested collisions beneath absent target directories may only be discovered during
exclusive creation; later failures leave truthful missing/partial output. No merge,
staging, backup, or rollback exists. Conservative repeated-tree checks are not yet
performance-tuned. Scan-to-operation identity capture remains the Task 17 audit.

## API Review And Limits

Reviewed the installed Go **1.27.1** documentation using `go doc` for `os.Root`,
`os.OpenRoot`, `Root.OpenRoot`, `Root.Lstat`, `Root.Stat`, `Root.Open`,
`File.ReadDir`, `Root.Mkdir`, `Root.OpenFile`, and `Root.Remove`.
See the [versioned standard library docs](https://pkg.go.dev/os@go1.27.1#Root).

- `OpenRoot` follows configured-root symlinks. On Linux/macOS the handle follows
  the opened directory across renames; it is not just a path string.
- Root methods constrain traversal but allow some in-root links, mount crossings,
  bind mounts, special files, and devices. They do not enforce the product's
  no-link or ordinary-file rules. `Remove` removes only a file or empty directory.
- This is **not a hostile-writer guarantee**. Resolution, identity observation,
  opening, preflight, and mutation are not one atomic filesystem transaction.
  Concurrent changes can invalidate observations; identity comparisons detect
  some changes, not all. There is no lock, source snapshot, sandbox, rollback,
  crash recovery, or guarantee against adversarial mount/namespace manipulation.
- An unverifiable relationship fails closed for affected mutations, not for
  inspection. Permission tests run unprivileged where possible; root explicitly
  exercises its permission-bypass behavior rather than falsely claiming denial.

## Evidence

Task 8 tests use real disposable directories for raw link/dotdot traversal,
missing suffixes, dangling/loop/file/permission failures, nested roots in both
directions, configured aliases, protected children, scope isolation, actual
filesystem case identity, repeated revalidation, post-creation validation, rooted
opening, and independent/stale browser safety messages.

Phase E Linux tests/race, injected failures, active-config protection, umask tests,
and PTY add/remove/replace/restart pass. Both macOS test binaries cross-build;
native Phase E/case-volume execution is pending. Earlier native CI validates only
Tasks 8-12. No push was performed during Phase E.

CI run `33999023333` passed both Linux jobs but failed both Macs in test fixtures:
APFS rejected invalid-UTF-8 names (`EILSEQ`), and `/var` versus `/private/var` aliases
prevented a change hook and PTY expectation from matching. Raw-name subcases now
probe filesystem support; ordinary copy/removal and binary-content tests never skip
for this capability. Hook expectations use physical paths, while PTY expectations
distinguish the child's physical cwd from unchanged configured global spellings.
Explicit symlink fixtures reproduce alias coverage on Linux. Production safety
checks are unchanged; local full/race/repeated tests pass, native rerun pending.
