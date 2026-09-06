# sei

A small terminal tool for copying skill folders from a personal library to
configured agent destinations and removing installed copies. Not a marketplace,
agent launcher, or skill runner. Normal use has no network access, telemetry,
or automatic updates.

**Replacement and removal are permanent, with no confirmation or undo.** Try
disposable folders first.

## Install

Download from [GitHub Releases](https://github.com/primaprashant/sei/releases).
Binaries are available for Linux and macOS, on amd64 and arm64; Go is not required.

### One Command

This downloads the complete installer before running it. It still executes
downloaded code; review the [installer source](https://github.com/primaprashant/sei/blob/main/scripts/install.sh)
first if preferred.

```sh
sh -c 'set -eu; d=$(mktemp -d); trap '\''rm -rf "$d"'\'' 0; trap '\''exit 1'\'' HUP INT TERM; curl -q --fail --silent --show-error --location --proto "=https" --proto-redir "=https" --tlsv1.2 --output "$d/install.sh" "https://github.com/primaprashant/sei/releases/latest/download/install.sh"; sh "$d/install.sh"'
```

The installer requires curl, tar, gzip, mktemp, and `sha256sum` or `shasum`.
It verifies the archive and installs to `~/.local/bin`, without sudo or shell-profile
changes. Follow its printed PATH instructions, or run `~/.local/bin/sei` directly.
For a custom install, download the release's `install.sh`, review it, then run:

```sh
sh install.sh --version v0.1.0 --install-dir "$HOME/.local/bin"
```

Use the script and version from the same release. Checksums detect corruption,
not a compromised publisher. Releases are not signed or notarized; do not bypass
Gatekeeper or remove quarantine if macOS refuses to launch them.

### Upgrade And Uninstall

Repeat installation into the same directory to upgrade. The installer only
replaces a binary matching its adjacent `.sei-install-receipt`; it refuses manual,
modified, symlinked, or mismatched installations. Do not edit receipts to force an
upgrade. If refused, inspect and move the old binary/receipt aside before a fresh
install, keeping the old copy until the new one works.

To uninstall, quit sei, inspect your actual install directory, and remove only
`sei` and `.sei-install-receipt`. Your config, library, and copied skills remain.
Remove any PATH entry you added only if no longer needed.

## Setup

1. Prepare a library of skill folders, separate from destination roots.
2. Run `sei` from the intended project directory. The launch directory is the project; sei does not search for a Git root.
3. First-run setup asks for the library and agent destinations. Review paths and order before saving.

```sh
sei setup
sei --project "$HOME/work/example"
sei --config "$HOME/sei-test.json" --project "$HOME/work/example"
sei --help
sei --version
```

Global options go before `setup`. Explicit setup edits valid existing config and
exits after saving; malformed config fails instead of being silently replaced.
Setup creates config parents/files, never the library or destinations.

In setup, Tab/Up/Down selects fields; Ctrl+U clears. Ctrl+A adds a preset/custom
agent, Ctrl+D removes it, and Ctrl+K/J reorders it. Enter previews and saves;
`e` returns to editing and `y` confirms replacing config. Esc/Ctrl+C cancels.
Recognized paste is ignored.

### Configuration

Linux uses `$XDG_CONFIG_HOME/sei/config.json` or `~/.config/sei/config.json`.
macOS uses `~/Library/Application Support/sei/config.json`, ignoring XDG.

```json
{
  "library": "~/skill-library",
  "agents": [
    {"name": "Claude Code", "global": "~/.claude/skills", "local": ".claude/skills"},
    {"name": "Codex", "global": "~/.agents/skills", "local": ".agents/skills"},
    {"name": "OpenCode", "global": "~/.config/opencode/skills", "local": ".opencode/skills"}
  ]
}
```

Configure 1-9 ordered agents with unique, nonempty names and nonempty paths.
Setup also offers Pi and Cursor presets; all names and paths are editable.
JSON is strict: unknown/duplicate keys, wrong types, nulls, and trailing values
are rejected. There is no config merging.

Library/global paths must be absolute or begin with `~/`; no other shell expansion
is performed. Local paths must stay inside the project, including through aliases.
Roots cannot overlap each other or the library; the config is also protected.
Relative `--config`/`--project` paths resolve from launch cwd; relative Linux
`XDG_CONFIG_HOME` is rejected. Restart after editing JSON; refresh does not reload it.

## Controls

The library is on the left, locals above globals on the right. Config order sets
agent slots. Minimum terminal size is **80x24**; smaller windows disable new
mutations but allow quitting and completion of active work.

| Input | Action |
| --- | --- |
| Up / Down | Move selection; scroll expanded help. |
| `0` | Focus library. |
| `1` ... `9` | Focus local agent slot. |
| `g`, then `1` ... `9` | Focus global agent slot. |
| `x` | Permanently remove selected destination folder. Uppercase `X` does nothing. |
| `r` | Refresh listings, not config. |
| `?` | Toggle help, including full escaped names, paths, and errors. |
| Esc | Close help or cancel pending `g`. |
| `q` / Ctrl+C | Quit after any active mutation finishes. |

Add keys work only from library focus:

| Slot | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Local | `a` | `b` | `c` | `d` | `e` | `f` | `h` | `i` | `o` |
| Global | `A` | `B` | `C` | `D` | `E` | `F` | `H` | `I` | `O` |

Unconfigured slots do nothing. Add retains library focus. Help and recognized
paste cannot trigger mutations. Navigation stays responsive during work, but
additional mutations are rejected, not queued. Focus and selections are not saved.

## Filesystem Behavior

- **Add deletes the entire same-named destination before copying.** No merge, backup, Git check, trash, or rollback. Local edits and destination-only files are lost.
- Preflight failures preserve the old tree; later failures can leave missing or partial output. Inspect the reported target before retrying.
- The library is read-only. Immediate ordinary directories, including dotfolders such as `.git`, are listed without parsing `SKILL.md`; loose files are ignored.
- Skill symlinks and internal links/special files block mutation. Configured-root symlinks are allowed only when boundary checks pass.
- Copies include nested/hidden files and executable bits, subject to umask, but not ownership, timestamps, ACLs, or xattrs.
- Only add creates missing destinations. Missing and inaccessible roots are shown differently.
- Quit and SIGINT/SIGTERM/SIGHUP wait for active work. Forced termination can leave partial changes. There is no multi-process lock, source snapshot, crash recovery, or hostile-writer guarantee.

sei shows configured folder contents, not everything an agent discovers or loads.
Other agents may load skills from the same folders.

Setup and browsing require terminal stdin/stdout; help/version do not. Exit status
is `0` for ordinary quit/cancel, `1` for startup/save/runtime failure (including
active work failing after quit was requested), and `2` for invalid CLI syntax.
A recoverable in-TUI error followed by a later ordinary quit returns `0`.

## Development

This is a personal tool, not an enterprise platform. Keep changes and process small.

- [Product vision](https://github.com/primaprashant/sei/blob/main/docs/product-vision.md): original motivation and wording, preserved as written; its PRD reference is historical.
- [Development guide](https://github.com/primaprashant/sei/blob/main/docs/development.md): build/tests, source map, and UI changes.
- [Filesystem safety](https://github.com/primaprashant/sei/blob/main/docs/filesystem-safety.md): constraints to preserve when changing destructive operations.
- [Release guide](https://github.com/primaprashant/sei/blob/main/docs/release.md): existing automation and installer behavior.

CI exercises Linux/macOS on both architectures; this is not an all-distro or
minimum-OS guarantee. Windows, Homebrew, and self-update are not provided.
For bugs, include version, OS/CPU, terminal size, reproduction steps, and the full
error. Use disposable data and redact private paths/content.

## License

MIT. See [LICENSE](LICENSE) and [THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES).
