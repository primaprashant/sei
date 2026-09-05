package main

import (
	"flag"
	"fmt"
	"io"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"
)

const usage = `Usage: sei [--help | --version]

sei is a terminal skill-folder manager, currently in development.
Without options, open a non-mutating terminal shell.
Configuration, setup, browsing, and skill mutations are not available yet.

Options:
  --help       Show this help without configuration or a terminal
  --version    Show the build version

Terminal keys: q or Ctrl+C to quit.
`

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	// Diagnostics are best-effort: a broken stderr cannot report its own error.
	flags := flag.NewFlagSet("sei", flag.ContinueOnError)
	flags.SetOutput(stderr)
	showHelp := flags.Bool("help", false, "show help")
	showVersion := flags.Bool("version", false, "show version")
	flags.Usage = func() { _, _ = fmt.Fprint(stderr, usage) }
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
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
