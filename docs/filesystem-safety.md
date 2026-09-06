# Filesystem Safety

## Paths And Boundaries

- `config.go` preserves raw absolute paths: cleaning `link/..` before traversal
  changes its meaning. `roots.go` walks components before normalization; configured
  directory symlinks are allowed, but dangling links, loops, non-directories and
  permission failures are not treated as ordinary absence. A missing suffix with
  `..` is unprovable until it exists.
- Local roots must stay inside the project both lexically and physically.
  Component-aware containment and `os.SameFile` ancestor identities detect aliases,
  nesting and actual filesystem case identity without lowercasing paths. Library
  and destinations must be disjoint, as must destinations across agents/scopes.
- Unknown boundaries block affected mutations, not browsing. A missing/unreadable
  library listing alone need not block safe removal. Scan/safety results are display
  observations, never cached authorization: every mutation revalidates.
- Operational names remain raw single Unix filename components, excluding empty,
  `.`, `..`, slash and NUL. Escaped display strings are never filesystem targets.
  A selected child must not equal or contain home, project, the active config and
  its aliases, library, or any destination. Observed entry/root drift is rejected.
- `os.OpenRoot` handles are identity-checked against the observation and closed by
  their owner. Rooted operations constrain traversal, but do not themselves reject
  in-root links, special files or mount crossings; sei supplies the ordinary-file
  and no-skill-symlink checks. Only explicit adds create missing destination parents.

## Two Replacement Behaviors

**Skills (`mutation_fs.go`, `mutation_copy.go`):** inventory source and destination
before replacement deletion, including reading/closing source files in preflight.
Copying reopens verified files and streams bytes; it is not a source snapshot.
Only ordinary directories and regular files are accepted. Removal consumes the
original postorder inventory, rechecking identities and unexpected children; it
uses rooted `Remove`, never `RemoveAll`. Incomplete deletion prevents copying.
Exact spelling/lookup checks reject known aliases before deletion; nested collisions
under absent directories may only fail at exclusive creation. Files use
`0666 | source execute bits`, directories `0777`, subject to umask; hard links become
independent copies. There is no merge, staging, backup or rollback: failures can
leave missing or partial destinations. Skill scripts are data and never executed.

**Config (`config_save.go`):** validation writes nothing. Saving creates parents through rooted
handles, writes a private exclusive temporary file, checks write/sync/close, then renames to commit.
Pre-commit failure preserves config bytes; cleanup is best effort and empty new parents may remain.
Config symlinks/nonregular files cannot be replaced; a safe config symlink can be loaded for browsing.
Placement must not overlap current or previous managed roots during reconfiguration. Parent, target,
temporary-file identities and root relationships are rechecked before commit; no fallible metadata step follows.

## Limits

Not a hostile-concurrent-writer guarantee: observation, opening and mutation are not atomic.
There is no lock, sandbox, crash recovery/durability promise, or protection against adversarial
mount/namespace manipulation. Keep external writers out of trees being changed.
Terminal/quit behavior is described in [development](development.md#async-and-lifecycle).
