# UI polish plan

Give sei a quiet, framed interface with clear focus, a highlighted selection,
and concise contextual instructions. Keep its library/project/global layout
and edit → preview → save setup flow. Use the existing Bubble Tea and Lip Gloss
dependencies and small rendering helpers.

Visual references: [superfile](https://github.com/yorukot/superfile#readme)
for framed panels, [Crush](https://github.com/charmbracelet/crush#readme) for
restrained accents and spacing, [btop](https://github.com/aristocratos/btop#readme)
for integrated labels, and [witr](https://github.com/pranshuparmar/witr#readme)
for selection contrast.

## Phase 1: Shared styles and browser

- Small shared palette: violet focus, cyan section labels, readable muted text,
  green success, amber caution, red failure. Adapt to light/dark terminals and
  preserve hierarchy without color; require no special font.
- Rounded panels with one-cell horizontal padding. Strong selection only in
  the focused panel; keep other selections subdued.
- Library left, Project above Global on the right, one to three agent columns
  with a visible agent range. Account for borders at 80×24.
- Compact titles/counts, focused destination path, scroll position, stable
  operation feedback, and contextual controls. Full escaped targets stay in help.
- Distinguish empty, missing, loading, blocked and failed states. Preserve
  filesystem protections, panel IDs, copy/remove semantics and async lifecycle.

## Phase 2: Setup and help

- Bounded, width-aware setup form with grouped library/agent fields, active
  field emphasis, contextual guidance, and fixed title/footer around scrolling
  content. Preserve field editing and preset/custom/reorder controls.
- Style preset selection, review, config replacement confirmation, validation
  errors, saving and pending quit consistently.
- Review shows resolved targets and config location, plus replacement/removal
  consequences: lost destination edits, permanent removal, possible partial output.
- Help has grouped selection/paths, navigation, copy/remove and diagnostic
  information, with a fixed title and scroll controls.

## Phase 3: Navigation and final verification

- Tab/Shift+Tab cycle panels in visual order and expose hidden agents; preserve
  direct focus/copy keys and pending-key/help/busy/quit precedence.
- Update controls documentation and focused regression tests/snapshots.
- Verify 1, 3 and 9 agents, minimum/large terminals, light/dark/monochrome,
  Unicode and long paths, scrolling, resize, setup states and operation states.
- Review actual colored terminal rendering as well as text snapshots using
  disposable HOME/config/project/library/destination fixtures.

## Checks and commits

Commit each phase separately after relevant regression checks, `go test -count=1
./...`, `go vet ./...`, `go build -o bin/sei .`, and the CI-pinned golangci-lint
format/lint checks. Run race and PTY verification for interaction/lifecycle
changes. Review intentional snapshots and `git diff --check` before commits.
No release rehearsal or publication is needed for this UI change.

Done means that location, selection, available actions and the latest result are
recognizable on every screen, including at 80×24 and without color.
