package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
)

type exitRequestMsg struct{}

// runLifecycle owns ordinary process interruption until terminal cleanup finishes.
// Quit decisions belong to Update, not the signal goroutine or command workers.
func runLifecycle(model tea.Model, stdin io.Reader, stdout io.Writer) (tea.Model, error) {
	input, ok := stdin.(interface{ Fd() uintptr })
	if !ok {
		return model, errors.New("terminal input has no file descriptor")
	}
	fd := input.Fd()
	before, err := term.GetState(fd)
	if err != nil {
		return model, fmt.Errorf("save terminal state: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	p := tea.NewProgram(model, tea.WithInput(stdin), tea.WithOutput(stdout), tea.WithoutSignalHandler(), tea.WithContext(ctx))
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	stop, done := make(chan struct{}), make(chan struct{})
	defer func() {
		cancel() // Also releases Send if Run rejected a nil model before its own cancel.
		signal.Stop(signals)
		close(stop)
		<-done
	}()
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			case <-signals:
				p.Send(exitRequestMsg{})
			}
		}
	}()
	final, err := p.Run()
	cancel()
	// Early initialization can fail after raw mode but before the renderer starts.
	// Restore only termios here; repeating renderer cleanup can hang or redraw.
	if restoreErr := term.Restore(fd, before); restoreErr != nil {
		err = errors.Join(err, fmt.Errorf("restore terminal state: %w", restoreErr))
	}
	return final, err
}
