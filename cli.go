package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
)

const usage = `Usage: sei [options] [setup]

sei is a terminal skill-folder manager, currently in development.
Load strict JSON configuration and browse configured folders read-only.
Setup, navigation, refresh, and mutations are not implemented yet.
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

Terminal keys: q or Ctrl+C to quit.
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
			_, _ = fmt.Fprintf(stderr, "sei: write output: %v\n", err)
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
		_, _ = fmt.Fprintf(stderr, "sei: launch directory: %v\n", err)
		return 1
	}
	path, err := configLocation(launch, *configOverride)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sei: %v\n", err)
		return 1
	}
	cfg, missing, err := loadConfig(path)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sei: %v\n", err)
		return 1
	}
	project, err := resolveProject(launch, *projectOverride)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sei: %v\n", err)
		return 1
	}
	if missing {
		_, _ = fmt.Fprintf(stderr, "sei: configuration %q is missing; first-run setup is not implemented yet\n", path)
		return 1
	}
	resolved, err := resolveConfigPaths(cfg, project)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sei: config %q: %v\n", path, err)
		return 1
	}
	if flags.NArg() == 1 {
		_, _ = fmt.Fprintln(stderr, "sei: setup is not implemented yet")
		return 1
	}
	input, inputOK := stdin.(interface{ Fd() uintptr })
	output, outputOK := stdout.(interface{ Fd() uintptr })
	if !inputOK || !outputOK || !term.IsTerminal(input.Fd()) || !term.IsTerminal(output.Fd()) {
		_, _ = fmt.Fprintln(stderr, "sei: interactive mode requires terminal stdin and stdout; use --help or --version")
		return 1
	}
	if _, err := tea.NewProgram(newBrowseModel(resolved), tea.WithInput(stdin), tea.WithOutput(stdout)).Run(); err != nil {
		_, _ = fmt.Fprintf(stderr, "sei: terminal: %v\n", err)
		return 1
	}
	return 0
}
