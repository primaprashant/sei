# sei: Product Vision

**sei** takes its name from the on'yomi (Sino-Japanese reading) of **整**, a kanji meaning "arrange" or "put in order." It captures the idea: put agent skills where I want them, and keep the rest out of the way.

## Why This Exists

I do a lot of work with AI CLI coding agents, and I work a lot in the terminal. Over time, I have found different sets of agent skills useful: folders of instructions, sometimes with supporting scripts, examples, and other files.

But I don't want every skill available all the time. Sometimes the agent reads a skill I don't want to apply. It pollutes the context and makes the results go in a direction I did not want.

So I keep all of my skills in a separate repository, "Personal Agent Skills," rather than leaving them in agents' global or project-local skill folders. When I need a skill, I copy it into the right place. When I'm done, I remove it. That works well, but sometimes I use Codex, sometimes Claude, OpenCode, Pi, or Cursor. Repeatedly copying and removing skills across different paths feels tiresome.

I want a simple terminal UI to see those folders together and quickly add or remove skills, so only the skills I want are available where I want them.

## The Product

A keyboard-first replacement for listing, copying, and removing skill folders, not an agent or session manager. Open it in a project, inspect the folders, add or remove skills, and close it. Remember destinations, not paths or commands.

Use one configurable, flat personal library of roughly 20-30 skills. Choose which agents to display and configure their global and project-local destinations. Global folders serve work across projects; local folders belong to the current project. Don't clutter the UI with agents I don't use.

Keep it in the terminal so it works where development happens, locally or on remote machines over SSH, without a separate graphical app. Desktop terminals come first; narrow or mobile layouts are optional.

## The Experience

- Show library folder names on the left, agents' global folders at the top right, and their project-local folders below.
- Navigate lists with the keyboard. Keep the selected skill, focused panel, agent, global/local scope, destination path, and action result obvious. Show key hints so shortcuts don't need memorizing.
- From the library, press a destination-specific letter key to add the selected skill. Stay on that library row so adding several skills to one destination takes minimal typing and panel switching.
- Press `0` to focus the library, `1-9` for agents' local panels, and `g` followed by `1-9` for their global panels. These are ordinary key sequences, not modifier combinations. The convention allows up to nine agents; fitting nine agents comfortably on screen is not a first-version requirement.
- In a destination panel, press `X` to remove the selected skill, even if it isn't in the library.

## Clear Rules

- Adding copies the entire skill folder. It completely replaces any same-named destination, including local edits. No merging or confirmation.
- Removing deletes the selected destination skill immediately. No confirmation or Git checks. Never change the source library.
- Changes happen immediately and persist until explicitly changed. There is no apply/save step. Closing the tool does not undo changes or clean up skills.
- Panels show the contents of configured folders, not every skill an agent might discover or has already loaded. This tool controls folders, not agent behavior or session isolation.

## Keep It Small

No agent launching or lifecycle management, automatic cleanup, broader skill discovery, loaded-skill inspection, skill editing, synchronization, tags, or categories. No multi-select or filtering in the first version: use short, repeated single-skill actions and a small, flat list. Revisit filtering only if real use shows a need.

Make startup fast, navigation immediate, and repeated actions responsive, without sacrificing correctness. The interface should be polished and clear, with no distracting animation or required fonts. Installation should take one command, without a separately installed runtime or agent tooling. Keep development, maintenance, and cross-platform releases simple.

## What Success Looks Like

In a real project, add three skills to one local destination, remove two, and quit without typing paths or repeatedly switching panels. This should beat the current shell workflow. Check that real folder names are easy to navigate without filtering, panels and hints fit the usual terminal size, and destructive actions have unmistakable targets.

The product scope and interaction direction are settled. Exact add keys, agent ordering, setup, and smaller interaction details such as incomplete key sequences, cancellation, selection after deletion, and failure feedback are defined in [prd.md](prd.md). The PRD also contains the technical decisions and remaining implementation and distribution questions. These are refinements, not reasons to expand the product.
