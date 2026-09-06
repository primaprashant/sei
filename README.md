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

Available for **Linux and macOS**, on **amd64 and arm64**.

### Install script

```sh
curl -fsSL https://github.com/primaprashant/sei/releases/latest/download/install.sh | bash
```

The script verifies the archive checksum and installs to `~/.local/bin`, without
sudo or Go. Follow its printed PATH instructions if `sei` is not found.

### Go

With Go **1.27.1 or later**:

```sh
go install github.com/primaprashant/sei@latest
```

Make sure your Go binary directory (`GOBIN`, or `GOPATH/bin`, usually `~/go/bin`)
is on your PATH.

### Release archive

Download a matching `.tar.gz` from [GitHub Releases](https://github.com/primaprashant/sei/releases),
extract it, and put the `sei` binary in a directory on your PATH.
Choose `darwin` for macOS or `linux` for Linux, and `arm64` for Apple Silicon/ARM64
or `amd64` for Intel/AMD 64-bit processors. No Go installation is needed.

### Upgrade and uninstall

To upgrade, repeat your installation method: rerun the script or `go install`,
or replace the binary with one from a newer release archive.
The script only replaces installations it recognizes; keep its adjacent
`.sei-install-receipt` file and use the same install directory.

To uninstall, quit sei and remove its binary from your install directory, along
with `.sei-install-receipt` if you used the script. Your configuration, library,
and copied skills remain.

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
