package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/sys/unix"
)

func statsBrowseFixture(t *testing.T) browseModel {
	t.Helper()
	cfg := statsFixture(t)
	browseMkdir(t, filepath.Join(cfg.Library, "example"))
	writeTestFile(t, filepath.Join(cfg.Library, "example", "SKILL.md"), "inert fixture")
	m := newBrowseModel(cfg)
	next, refresh := m.Update(startBrowseMsg{})
	return finishMutationRefresh(t, next.(browseModel), refresh)
}

func TestStatsMutationRecording(t *testing.T) {
	m := statsBrowseFixture(t)
	for _, key := range []rune{'a', 'a', '1', 'x'} {
		var worker tea.Cmd
		m, worker = press(m, key)
		if key == '1' {
			continue
		}
		if worker == nil {
			t.Fatal("missing operation")
		}
		result := worker().(mutationResult)
		if result.err != nil || result.statsErr != nil {
			t.Fatalf("operation failed: %+v", result)
		}
		next, refresh := m.Update(result)
		m = finishMutationRefresh(t, next.(browseModel), refresh)
		// Duplicate completion delivery must not count the action again.
		next, cmd := m.Update(result)
		if cmd != nil {
			t.Fatal("duplicate completion scheduled work")
		}
		m = next.(browseModel)
	}
	history, err := loadStats(statsPath(m.config.ConfigPath))
	if err != nil {
		t.Fatal(err)
	}
	summary := history.summarize(time.Now())
	if summary.copies != 2 || summary.removals != 1 {
		t.Fatalf("replacement or removal miscounted: %+v", summary)
	}
	before := removeSnapshot(t, m.config.Home)
	// Empty destination is blocked; stale source fails inside the command.
	if _, cmd := press(m, 'x'); cmd != nil {
		t.Fatal("empty removal allowed")
	}
	m, _ = press(m, '0')
	_, worker := press(m, 'a')
	if err := os.Rename(filepath.Join(m.config.Library, "example"), filepath.Join(m.config.Library, "moved")); err != nil {
		t.Fatal(err)
	}
	result := worker().(mutationResult)
	if result.err == nil || result.statsErr != nil {
		t.Fatalf("stale action recorded: %+v", result)
	}
	assertRemoveSnapshot(t, m.config.Home, before)
}

func TestStatsFailurePreservesSuccess(t *testing.T) {
	for _, quitting := range []bool{false, true} {
		m := statsBrowseFixture(t)
		writeTestFile(t, statsPath(m.config.ConfigPath), "broken history")
		m, worker := press(m, 'a')
		if quitting {
			m, _ = press(m, 'q')
		}
		result := worker().(mutationResult)
		if result.err != nil || result.statsErr == nil {
			t.Fatalf("wrong result: %+v", result)
		}
		next, cmd := m.Update(result)
		m = next.(browseModel)
		if m.statusFailed || m.statsWarning == nil || m.exitError != nil {
			t.Fatalf("stats error became mutation failure: %+v", m)
		}
		if quitting {
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatal("pending quit did not finish")
			}
		} else {
			m = finishMutationRefresh(t, m, cmd)
			if !strings.Contains(m.View().Content, "Copied example · stats not saved") || !strings.Contains(strings.Join(m.helpLines(), "\n"), "Stats warning:") {
				t.Fatal("missing nonfatal warning")
			}
		}
		var stderr bytes.Buffer
		if code := browseExit(m, nil, &stderr); code != 0 || !strings.Contains(stderr.String(), "stats not saved") {
			t.Fatalf("wrong exit: %d, %s", code, &stderr)
		}
		data, err := os.ReadFile(filepath.Join(m.config.Agents[0].Local, "example", "SKILL.md"))
		if err != nil || string(data) != "inert fixture" {
			t.Fatal("copy did not complete")
		}
		data, err = os.ReadFile(statsPath(m.config.ConfigPath))
		if err != nil || string(data) != "broken history" {
			t.Fatal("invalid history overwritten")
		}
	}
}

func TestStatsQuitWaitsForRecording(t *testing.T) {
	m := statsBrowseFixture(t)
	path := statsPath(m.config.ConfigPath)
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Close() }()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	m, worker := press(m, 'a')
	done := make(chan mutationResult, 1)
	go func() { done <- worker().(mutationResult) }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(m.config.Agents[0].Local, "example", "SKILL.md")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("copy did not finish")
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case <-done:
		t.Fatal("operation returned before stats lock released")
	default:
	}
	next, cmd := m.Update(exitRequestMsg{})
	m = next.(browseModel)
	if cmd != nil || !m.pendingQuit || m.active == nil {
		t.Fatal("quit did not wait for stats")
	}
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-done:
		if result.err != nil || result.statsErr != nil {
			t.Fatalf("operation: %+v", result)
		}
		_, cmd = m.Update(result)
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatal("quit did not complete")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stats save did not finish")
	}
	history, err := loadStats(path)
	if err != nil || history.summarize(time.Now()).copies != 1 {
		t.Fatalf("copy lost on quit: %+v, %v", history, err)
	}
}
