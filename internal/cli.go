package app

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/x/term"
)

const usage = `Usage: sei [options] [setup]

Copy skills from a personal library into agents' project or global folders,
and remove them when done. Run sei setup to change configuration.
Options must precede setup.

Options:
  --help       Show this help without configuration or a terminal
  --version    Show the build version
  --config     Configuration file (relative to launch directory)
  --project    Project directory (default: launch directory, not Git root)

Examples:
  sei --config /tmp/sei.json --project ./example
  sei --config /tmp/sei.json setup

Browser controls:
  Up/Down          Move selection
  Tab/Shift+Tab    Focus next/previous panel
  0                Focus library
  1-9              Focus agent's project folder
  g, then 1-9      Focus agent's global folder
  x                Permanently remove selected destination skill
  r                Refresh folder listings
  ?                Show help, full paths, and errors
  Esc              Close help or cancel pending g
  q / Ctrl+C       Quit after active work finishes

Copy from the library by agent slot (configuration order):
  Project: a b c d e f h i o
  Global:  A B C D E F H I O

Adding replaces the entire same-named destination, including local edits.
Replacement and removal have no confirmation or undo. Failed operations can
leave missing or partial output. The source library stays unchanged.

sei shows configured folders, not everything an agent discovers or has loaded.
Minimum terminal size: 80x24.
`

// Run executes the CLI and returns its exit status.
func Run(version string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
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
	resolved.ConfigPath = path
	final, err := runLifecycle(newBrowseModel(resolved), stdin, stdout)
	result, _ := final.(browseModel)
	return browseExit(result, err, stderr)
}

// Called only after lifecycle cleanup, including terminal restoration.
func browseExit(result browseModel, err error, stderr io.Writer) int {
	if err != nil {
		err = fmt.Errorf("terminal: %w", err)
	}
	if err = errors.Join(err, result.exitError); err != nil {
		_, _ = fmt.Fprintf(stderr, "sei: %s\n", displayText(err.Error()))
		return 1
	}
	return 0
}
