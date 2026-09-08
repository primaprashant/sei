# sei - give your coding agents only the skills you deem relevant

sei is a CLI tool for people (like myself) who:
- have a personal collection of agent skills, and
- like to manually choose the skills available to their coding agents for each session.

If you're happy dumping 50 skills in your `.claude/skills/` and letting the agents figure out which ones to use and when, then this is **NOT for you**.

I built this TUI to replace the repeated `ls`, `cp`, and `rm` commands. I use it to inspect available skills in agents’ project and global skill folders, add skills from my collection, and remove them after I'm done.

## Install

Available for **Linux and macOS**, on **amd64 and arm64**.

### Install script (recommended)

```sh
curl -fsSL https://github.com/primaprashant/sei/releases/latest/download/install.sh | bash
```

The script pulls the pre-built binary and saves it to `~/.local/bin`. You don't need Go installed on your system.

### Release archive

Download a matching `.tar.gz` from the [releases](https://github.com/primaprashant/sei/releases) page, extract it, and put the `sei` binary in a directory on your PATH. In this case as well, you don't need Go installed on your system.

Choose `darwin` for macOS or `linux` for Linux, and `arm64` for Apple Silicon/ARM64
or `amd64` for Intel/AMD 64-bit processors.

### Go

With Go **1.27.1 or later**:

```sh
go install github.com/primaprashant/sei@latest
```

Make sure your Go binary directory (`GOBIN`, or `GOPATH/bin`, usually `~/go/bin`)
is on your PATH.

### Upgrade and uninstall

To upgrade, repeat your installation method: rerun the script or `go install`, or replace the binary with one from a newer release archive.

The install script only replaces installations it recognizes; keep its adjacent `.sei-install-receipt` file and use the same install directory. 

To uninstall, quit sei and remove its binary from your install directory, along
with `.sei-install-receipt` if you used the script.

Your configuration, stats history, library, and copied skills will remain on your system.

## Setup

1. Keep your skill folders in a separate library, such as `~/skill-library`. I like to keep all the skills I've written and adapted over time in a git repo named `personal-agent-skills`.
2. Run `sei` from your project directory.
3. First-run setup will ask for your skill library. Add/remove the coding agents you use. Choose your paths and save to start managing your skills.

Setup includes presets for Claude Code, Codex, OpenCode, Pi, Cursor, Antigravity CLI, Crush, GitHub Copilot CLI, and Cline CLI. You can also add any custom agents. All names and
paths are editable.

The order of the agents during the setup determines their order in the TUI and their keyboard shortcuts.

To change the order of the agents, run the setup again:

```sh
sei setup # Change your configuration
```

The launch directory is the default project directory; sei does not search for a Git root. Setup saves configuration but does not create your library or agent destinations.

## Configuration

Setup writes the configuration file to disk. To edit it manually, use:

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

Restart sei after editing the config file for the changes to take effect.

`library` is your source collection. Each agent's `global` folder serves all
projects; its `local` folder is relative to the current project. Library and global paths must be absolute or start with `~/`. Local paths must stay inside the project, and library and destination roots must be separate.

If an agent’s project and global paths resolve to the same folder (for example, when launching from home with the defaults), setup still saves. The browser dims that project panel and disables its copy/remove operations; use the global panel.

Launching from another project or using `--project` gives project paths their
usual meaning. Nested overlaps and overlaps between different agents are blocked.

## Stats

```sh
sei stats
```

You can see your total skill actions, copies, removals, active days, actions this calendar month, and your five most-copied skills both all time and over the last 30 days.

## Controls

The library is on the left, project folders at the top right, and global folders below them. Use a terminal of at least **80×24**.

The active panel has a violet border and highlighted selection. Project and global folders are grouped separately; the footer shows the focused path and current operation. Colors adapt to light/dark terminal backgrounds. Set `NO_COLOR=1` for monochrome output; focus and selection remain marked with `*` and `>`.

A **slot** is an agent's position in your configuration: the first agent is slot 1, the second is slot 2, and so on.

| Key | Action |
| --- | --- |
| Up / Down | Move selection, or scroll help. |
| Tab / Shift+Tab | Focus the next / previous panel (library, project, then global). |
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

For example, `a` copies to the first agent's project folder; `A` copies to its global folder. Selection stays in the library so you can keep adding skills.

## How changes work

- Your source library stays unchanged.
- Adding replaces the entire same-named destination, including local edits.
- **Replacement and removal are immediate, with no confirmation or undo.**
- Changes persist after quitting. Failed operations can leave missing or partial output; inspect the reported destination before retrying.
