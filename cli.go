package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/x/term"
)

const usage = `Usage: sei [options] [setup]

sei is a terminal skill-folder manager, currently in development.
Load strict JSON configuration and manage configured folders.
Setup, fresh skill copies, and permanent removal are available; replacement is not enabled yet.
Global options must precede the optional setup subcommand.

Options:
  --help       Show this help without configuration or a terminal
  --version    Show the build version
  --config     Configuration file (relative to launch directory)
  --project    Project directory (default: launch directory, not Git root)

Examples:
  sei --config /tmp/sei.json --project ./example
  sei --config /tmp/sei.json setup

Other agents may also load skills from these folders. sei shows configured folder contents, not everything an agent discovers or has loaded.

Terminal keys: Up/Down (clamped), 0 library, 1-9 local, g then 1-9 global.
r refreshes listings (not config); ? full sanitized targets/help, Up/Down scroll.
g stays pending until a key: invalid continuations are consumed; Esc cancels/closes.
q or Ctrl+C always quit; recognized paste is ignored. Selections are per panel;
refresh preserves raw names, otherwise clamps the old index.
Add mappings by slot: a b c d e f h i o local, A B C D E F H I O global
(library only). X permanently removes (destination only); x does nothing.
Removal has no confirmation, trash, backup, or undo. Quit waits for active work.
Layout is provisional; no minimum terminal size has been approved.
`

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	// Diagnostics are best-effort: a broken stderr cannot report its own error.
	flags := flag.NewFlagSet("sei", flag.ContinueOnError)
	flags.SetOutput(stderr)
	showHelp := flags.Bool("help", false, "show help")
	flags.BoolVar(showHelp, "h", false, "show help")
	showVersion := flags.Bool("version", false, "show version")
	configOverride := flags.String("config", "", "configuration file")
	projectOverride := flags.String("project", "", "project directory")
	flags.Usage = func() { _, _ = fmt.Fprint(stderr, usage) }
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() > 1 || (flags.NArg() == 1 && flags.Arg(0) != "setup") {
		_, _ = fmt.Fprintf(stderr, "sei: unexpected argument %q; use --help\n", flags.Arg(0))
		return 2
	}
	if *showHelp || *showVersion {
		text := usage
		if !*showHelp {
			text = "sei " + version + "\n"
		}
		if _, err := io.WriteString(stdout, text); err != nil {
			_, _ = fmt.Fprintf(stderr, "sei: write output: %s\n", displayText(err.Error()))
			return 1
		}
		return 0
	}
	invalidPathFlag := ""
	flags.Visit(func(f *flag.Flag) {
		if (f.Name == "config" || f.Name == "project") && f.Value.String() == "" {
			invalidPathFlag = f.Name
		}
	})
	if invalidPathFlag != "" {
		_, _ = fmt.Fprintf(stderr, "sei: --%s must not be empty\n", invalidPathFlag)
		return 2
	}
	launch, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sei: launch directory: %s\n", displayText(err.Error()))
		return 1
	}
	path, err := configLocation(launch, *configOverride)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sei: %s\n", displayText(err.Error()))
		return 1
	}
	cfg, missing, err := loadConfig(path)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sei: %s\n", displayText(err.Error()))
		return 1
	}
	project, err := resolveProject(launch, *projectOverride)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sei: %s\n", displayText(err.Error()))
		return 1
	}
	var resolved config
	if !missing {
		resolved, err = resolveConfigPaths(cfg, project)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sei: config %q: %s\n", path, displayText(err.Error()))
		return 1
	}
	explicitSetup := flags.NArg() == 1
	input, inputOK := stdin.(interface{ Fd() uintptr })
	output, outputOK := stdout.(interface{ Fd() uintptr })
	if !inputOK || !outputOK || !term.IsTerminal(input.Fd()) || !term.IsTerminal(output.Fd()) {
		_, _ = fmt.Fprintln(stderr, "sei: interactive mode requires terminal stdin and stdout; use --help or --version")
		return 1
	}
	if missing || explicitSetup {
		setup := newSetupModel(project, path)
		if !missing {
			setup.cfg = cfg
			setup.cfg.Agents = append([]agentConfig(nil), cfg.Agents...)
			setup.replace = true
		}
		final, setupErr := runLifecycle(setup, stdin, stdout)
		if setupErr == nil {
			result := final.(setupModel)
			setupErr = result.fatal
			if setupErr == nil && (!result.saved || result.pendingQuit || explicitSetup) {
				return 0
			}
			resolved = result.resolved
		}
		if setupErr != nil {
			_, _ = fmt.Fprintf(stderr, "sei: setup: %s\n", displayText(setupErr.Error()))
			return 1
		}
	}
	final, err := runLifecycle(newBrowseModel(resolved), stdin, stdout)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sei: terminal: %s\n", displayText(err.Error()))
		return 1
	}
	if result, ok := final.(browseModel); ok && result.exitError != nil {
		_, _ = fmt.Fprintf(stderr, "sei: %s\n", displayText(result.exitError.Error()))
		return 1
	}
	return 0
}
