package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"
)

const usage = `Usage: sei [options] [setup]

sei is a terminal skill-folder manager, currently in development.
Load strict JSON configuration and open a non-mutating terminal shell.
Setup is recognized but unavailable; browsing and mutations are not implemented.
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
	if _, err := resolveConfigPaths(cfg, project); err != nil {
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
	if _, err := tea.NewProgram(shellModel{}, tea.WithInput(stdin), tea.WithOutput(stdout)).Run(); err != nil {
		_, _ = fmt.Fprintf(stderr, "sei: terminal: %v\n", err)
		return 1
	}
	return 0
}

type shellModel struct{}

func (shellModel) Init() tea.Cmd { return nil }

func (m shellModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (shellModel) View() tea.View {
	title := lipgloss.NewStyle().Bold(true).Render("sei")
	h := help.New()
	keys := h.ShortHelpView([]key.Binding{
		key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q / ctrl+c", "quit")),
	})
	v := tea.NewView(title + "\n\nDevelopment shell. No folders are read or changed.\n" +
		"Setup, browsing, and mutations are not implemented.\n\n" + keys)
	v.AltScreen = true
	return v
}
