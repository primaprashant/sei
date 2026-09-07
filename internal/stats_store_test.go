package app

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func statsFixture(t *testing.T) config {
	t.Helper()
	cfg := rootFixture(t)
	cfg.ConfigPath = filepath.Join(cfg.Home, "config.json")
	writeTestFile(t, cfg.ConfigPath, "config stays unchanged")
	return cfg
}

func TestStatsPersistence(t *testing.T) {
	cfg := statsFixture(t)
	unchanged := configSaveSnapshot(t, cfg.ConfigPath, cfg.Library, cfg.Agents[0].Global, cfg.Project)
	before := removeSnapshot(t, cfg.Home)
	history, err := loadStats(statsPath(cfg.ConfigPath))
	if err != nil || history.Version != 0 {
		t.Fatalf("missing stats: %+v, %v", history, err)
	}
	assertRemoveSnapshot(t, cfg.Home, before)
	for _, copy := range []bool{true, true, false} {
		if err := recordStats(cfg, "example", copy, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	history, err = loadStats(statsPath(cfg.ConfigPath))
	if err != nil {
		t.Fatal(err)
	}
	summary := history.summarize(time.Now())
	if summary.copies != 2 || summary.removals != 1 || summary.activeDays != 1 || summary.allTime[0].name != "example" {
		t.Fatalf("history: %+v", summary)
	}
	for _, path := range []string{statsPath(cfg.ConfigPath), statsPath(cfg.ConfigPath) + ".lock"} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("file is not private: %v, %v", info, err)
		}
	}
	other := cfg
	other.ConfigPath = filepath.Join(cfg.Home, "other.json")
	writeTestFile(t, other.ConfigPath, "other config")
	if err := recordStats(other, "other-skill", true, time.Now()); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadStats(statsPath(cfg.ConfigPath))
	if err != nil || !reflect.DeepEqual(loaded, history) {
		t.Fatal("another configuration changed history")
	}
	unchanged()
}

func TestStatsConcurrentWriters(t *testing.T) {
	cfg := statsFixture(t)
	stop, readerDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(readerDone)
		var previous int64
		for {
			select {
			case <-stop:
				return
			default:
			}
			history, err := loadStats(statsPath(cfg.ConfigPath))
			if err != nil {
				t.Error(err)
				return
			}
			summary := history.summarize(time.Now())
			total := summary.copies + summary.removals
			if total < previous {
				t.Error("reader observed history going backwards")
				return
			}
			previous = total
			time.Sleep(time.Millisecond)
		}
	}()
	var wg sync.WaitGroup
	for i := range 6 {
		wg.Go(func() {
			for range 8 {
				if err := recordStats(cfg, "shared", i%2 == 0, time.Now()); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	wg.Wait()
	close(stop)
	<-readerDone
	history, err := loadStats(statsPath(cfg.ConfigPath))
	if err != nil {
		t.Fatal(err)
	}
	summary := history.summarize(time.Now())
	if summary.copies != 24 || summary.removals != 24 {
		t.Fatalf("lost concurrent updates: %+v", summary)
	}
}

func TestStatsWriteFailures(t *testing.T) {
	for _, failure := range []string{"create", "write", "short write", "sync", "close", "rename", "target drift", "lock drift", "parent drift"} {
		t.Run(failure, func(t *testing.T) {
			cfg := statsFixture(t)
			path := statsPath(cfg.ConfigPath)
			if err := recordStats(cfg, "original", true, time.Now()); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			output := configSaveTestIO()
			fault := errors.New("injected stats failure")
			switch failure {
			case "create":
				output.create = func(*os.Root, string) (*os.File, error) { return nil, fault }
			case "write":
				output.write = func(*os.File, []byte) (int, error) { return 0, fault }
			case "short write":
				output.write = func(f *os.File, b []byte) (int, error) { return f.Write(b[:1]) }
			case "sync":
				output.sync = func(*os.File) error { return fault }
			case "close":
				output.close = func(f *os.File) error { return errors.Join(f.Close(), fault) }
			case "rename":
				output.rename = func(*os.Root, string, string) error { return fault }
			case "target drift", "lock drift", "parent drift":
				output.sync = func(f *os.File) error {
					switch failure {
					case "target drift":
						if err := os.Rename(path, path+".old"); err != nil {
							return err
						}
						writeTestFile(t, path, "external edit")
					case "lock drift":
						if err := os.Rename(path+".lock", path+".old-lock"); err != nil {
							return err
						}
						writeTestFile(t, path+".lock", "new lock")
					case "parent drift":
						if err := os.Rename(cfg.Home, cfg.Home+"-moved"); err != nil {
							return err
						}
						browseMkdir(t, cfg.Home)
					}
					return f.Sync()
				}
			}
			if err := recordStatsWithIO(cfg, "new", true, time.Now(), output); err == nil {
				t.Fatal("stats save succeeded despite failure")
			} else if failure == "short write" && !errors.Is(err, io.ErrShortWrite) {
				t.Fatalf("wrong short-write error: %v", err)
			}
			switch failure {
			case "target drift":
				data, err := os.ReadFile(path)
				if err != nil || string(data) != "external edit" {
					t.Fatal("external edit overwritten")
				}
				path += ".old"
			case "parent drift":
				path = filepath.Join(cfg.Home+"-moved", filepath.Base(path))
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != string(before) {
				t.Fatalf("old history lost: %s, %v", after, err)
			}
			entries, err := os.ReadDir(filepath.Dir(path))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".sei-stats-") {
					t.Fatal("temporary stats file left behind")
				}
			}
		})
	}
}

func TestStatsUnsafeFiles(t *testing.T) {
	for _, kind := range []string{"corrupt", "version", "symlink", "dangling", "directory", "fifo", "lock symlink", "lock fifo", "in library", "in destination"} {
		t.Run(kind, func(t *testing.T) {
			cfg := statsFixture(t)
			path := statsPath(cfg.ConfigPath)
			switch kind {
			case "corrupt":
				writeTestFile(t, path, "{interrupted")
			case "version":
				writeTestFile(t, path, strings.Replace(sampleStats, `"version":1`, `"version":99`, 1))
			case "symlink", "dangling", "lock symlink":
				target := cfg.ConfigPath
				switch kind {
				case "dangling":
					target += ".absent"
				case "lock symlink":
					path += ".lock"
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			case "directory":
				browseMkdir(t, path)
			case "fifo", "lock fifo":
				if kind == "lock fifo" {
					path += ".lock"
				}
				if err := unix.Mkfifo(path, 0o600); err != nil {
					t.Fatal(err)
				}
			case "in library":
				cfg.ConfigPath = filepath.Join(cfg.Library, "config.json")
			case "in destination":
				cfg.ConfigPath = filepath.Join(cfg.Agents[0].Global, "config.json")
			}
			unchanged := configSaveSnapshot(t, cfg.Library, cfg.Agents[0].Global, cfg.Project)
			if !strings.HasPrefix(kind, "in ") {
				defer configSaveSnapshot(t, path)()
			}
			if err := recordStats(cfg, "example", true, time.Now()); err == nil {
				t.Fatal("unsafe stats write succeeded")
			}
			if !strings.HasPrefix(kind, "lock") && !strings.HasPrefix(kind, "in ") {
				if _, err := loadStats(path); err == nil {
					t.Fatal("invalid history read succeeded")
				}
			}
			unchanged()
		})
	}
}

func TestStatsLockTimeout(t *testing.T) {
	cfg := statsFixture(t)
	path := statsPath(cfg.ConfigPath)
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Close() }()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	err = recordStats(cfg, "example", true, start)
	if err == nil || !strings.Contains(err.Error(), "busy") || time.Since(start) > 3*time.Second {
		t.Fatalf("lock was not bounded: %v, %v", time.Since(start), err)
	}
	setupAbsent(t, path)
}
