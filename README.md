# sei

A small terminal UI for copying skills from your personal library into coding
agents' project or global folders, and removing them when you're done.

## Purpose

Skills are useful, but you may not want every skill available for every task.
An agent reading an unwanted skill can introduce irrelevant instructions and
steer its work in an unexpected direction.

Keeping a separate collection lets you choose which skills to make available.
sei makes the repeated copying and removing easier, especially when you use
several coding agents.

### Is this for you?

sei is useful when all three sound familiar:

- You maintain your own collection of skills you've found useful.
- You prefer to make only task-relevant skills available to your agents.
- You find copying and removing skills across projects or agents tedious.

If you're happy leaving your skills installed, you may not need sei.
It manages configured folders; it cannot tell you everything an agent discovers
or control what it has already loaded.

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

1. Keep your skill folders in a separate library, such as `~/skill-library`.
2. Run `sei` from your project directory.
3. First-run setup asks for your library and agent destinations. Choose your paths
   and save to start browsing.

Setup offers Claude Code, Codex, OpenCode, Pi, and Cursor presets, plus custom
agents. All names and paths are editable.

```sh
sei setup                         # Change your configuration
sei --project ~/work/my-project    # Use a different project directory
```

The launch directory is the default project directory; sei does not search for
a Git root. Setup saves configuration but does not create your library or
agent destinations.

## Configuration

Setup writes the configuration for you. To edit it manually, use:

- **Linux:** `$XDG_CONFIG_HOME/sei/config.json`, or `~/.config/sei/config.json`.
- **macOS:** `~/Library/Application Support/sei/config.json`.

```json
{
  "library": "~/skill-library",
  "agents": [
    {"name": "Claude Code", "global": "~/.claude/skills", "local": ".claude/skills"},
    {"name": "Codex", "global": "~/.agents/skills", "local": ".agents/skills"}
  ]
}
```

`library` is your source collection. Each agent's `global` folder serves all
projects; its `local` folder is relative to the current project.
Library and global paths must be absolute or start with `~/`. Local paths must
stay inside the project, and all library and destination roots must be separate.

Configure 1–9 agents; their order determines the keyboard slots below.
Restart sei after editing the file. Use `sei --config /path/to/config.json`
to choose a different config, or `sei --help` for all options.

## Controls

The library is on the left, project folders at the top right, and global folders
below them. Use a terminal of at least **80×24**.

A **slot** is an agent's position in your configuration: the first agent is slot 1,
the second is slot 2, and so on.

| Key | Action |
| --- | --- |
| Up / Down | Move selection, or scroll help. |
| `0` | Focus the library. |
| `1`–`9` | Focus an agent's project folder. |
| `g`, then `1`–`9` | Focus an agent's global folder. |
| `x` | Permanently remove the selected skill from a destination. |
| `r` | Refresh folder listings. |
| `?` | Show help, full names, paths, and errors. |
| Esc | Close help or cancel a pending `g`. |
| `q` / Ctrl+C | Quit after any active copy or removal finishes. |

With a skill selected in the library, use these keys to copy it:

| Agent slot | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Project | `a` | `b` | `c` | `d` | `e` | `f` | `h` | `i` | `o` |
| Global | `A` | `B` | `C` | `D` | `E` | `F` | `H` | `I` | `O` |

For example, `a` copies to the first agent's project folder; `A` copies to its
global folder. Selection stays in the library so you can keep adding skills.

## How changes work

- Your source library stays unchanged.
- Adding replaces the entire same-named destination, including local edits.
- **Replacement and removal are immediate, with no confirmation or undo.**
- Changes persist after quitting. Failed operations can leave missing or partial
  output; inspect the reported destination before retrying.
