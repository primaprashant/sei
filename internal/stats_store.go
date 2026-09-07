package app

import (
	"crypto/rand"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

func statsPath(configPath string) string { return configPath + ".stats.json" }

// Reading history never creates parents, a lock, or an empty history file.
func loadStats(path string) (statsHistory, error) {
	dir, name := filepath.Split(path)
	parent := resolveRoot(dir)
	if parent.err != nil {
		return statsHistory{}, parent.err
	}
	if len(parent.missing) != 0 {
		return statsHistory{}, nil
	}
	root, err := openStatsParent(parent)
	if err != nil {
		return statsHistory{}, err
	}
	defer func() { _ = root.Close() }()
	history, _, err := readStatsAt(root, name)
	return history, err
}

func openStatsParent(parent resolvedRoot) (*os.Root, error) {
	root, err := os.OpenRoot(parent.path)
	if err != nil {
		return nil, err
	}
	held, err := root.Stat(".")
	if err == nil && !os.SameFile(held, parent.ancestors[0].info) {
		err = fmt.Errorf("stats parent changed while opening")
	}
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	return root, nil
}

func readStatsAt(root *os.Root, name string) (statsHistory, os.FileInfo, error) {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return statsHistory{}, nil, nil
	}
	if err != nil {
		return statsHistory{}, nil, err
	}
	if !info.Mode().IsRegular() {
		return statsHistory{}, nil, fmt.Errorf("stats must be a regular file, not a symlink or special file")
	}
	file, err := root.OpenFile(name, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return statsHistory{}, nil, err
	}
	defer func() { _ = file.Close() }()
	held, err := file.Stat()
	if err != nil {
		return statsHistory{}, nil, err
	}
	// A cooperating writer may have renamed a new history between Lstat and
	// OpenFile. Either regular-file snapshot is valid; the opened identity is
	// what a subsequent save must preserve until commit.
	if !held.Mode().IsRegular() {
		return statsHistory{}, nil, fmt.Errorf("stats must be a regular file")
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return statsHistory{}, nil, err
	}
	history, err := parseStats(data)
	return history, held, err
}

func recordStats(cfg config, name string, copy bool, now time.Time) error {
	return recordStatsWithIO(cfg, name, copy, now, configSaveIO{
		create: func(root *os.Root, name string) (*os.File, error) {
			return root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		},
		write: (*os.File).Write, sync: (*os.File).Sync, close: (*os.File).Close, rename: (*os.Root).Rename,
	})
}

// Reuse the config save's per-call file operations so failure tests exercise
// real writes without global hooks. A stats error never requests a skill retry.
func recordStatsWithIO(cfg config, skill string, copy bool, now time.Time, output configSaveIO) error {
	if cfg.ConfigPath == "" {
		return nil // Models without a persisted configuration do not record history.
	}
	path := statsPath(cfg.ConfigPath)
	parent, name, _, err := configSaveLocation(cfg, path)
	if err != nil {
		return err
	}
	if len(parent.missing) != 0 {
		return fmt.Errorf("stats parent no longer exists")
	}
	root, err := openStatsParent(parent)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	lockPath, lockName := path+".lock", name+".lock"
	lockParent, _, _, err := configSaveLocation(cfg, lockPath)
	if err != nil {
		return err
	}
	if err := checkConfigParent(parent, lockParent); err != nil {
		return err
	}
	lock, err := root.OpenFile(lockName, os.O_RDWR|os.O_CREATE|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0o600)
	if err != nil {
		return err
	}
	// The lock file stays in place: locking the renamed JSON inode (or unlinking
	// the lock on release) would allow concurrent writers to hold different locks.
	defer func() { _ = lock.Close() }()
	lockInfo, err := lock.Stat()
	if err != nil {
		return err
	}
	if !lockInfo.Mode().IsRegular() {
		return fmt.Errorf("stats lock must be a regular file")
	}
	if err := lockStats(lock); err != nil {
		return err
	}
	defer func() { _ = unix.Flock(int(lock.Fd()), unix.LOCK_UN) }()
	history, original, err := readStatsAt(root, name)
	if err != nil {
		return err
	}
	if err := history.record(skill, copy, now); err != nil {
		return err
	}
	data, err := json.Marshal(history, json.Deterministic(true), jsontext.WithIndent("  "))
	if err != nil {
		return err
	}
	data = append(data, '\n')
	check := func() error {
		fresh, _, current, err := configSaveLocation(cfg, path)
		if err != nil {
			return err
		}
		if err := checkConfigParent(parent, fresh); err != nil {
			return err
		}
		if (original == nil) != (current == nil) || (original != nil && !sameCopyEntry(original, current)) {
			return fmt.Errorf("stats changed before save")
		}
		if _, _, currentLock, err := configSaveLocation(cfg, lockPath); err != nil {
			return err
		} else if currentLock == nil || !os.SameFile(lockInfo, currentLock) {
			return fmt.Errorf("stats lock changed")
		}
		return nil
	}
	if err := check(); err != nil {
		return err
	}
	temp := ".sei-stats-" + rand.Text()
	dir, _ := filepath.Split(path)
	tempParent, _, _, err := configSaveLocation(cfg, dir+temp)
	if err != nil {
		return err
	}
	if err := checkConfigParent(parent, tempParent); err != nil {
		return err
	}
	file, err := output.create(root, temp)
	if err != nil {
		return err
	}
	defer func() { _ = root.Remove(temp) }()
	n, writeErr := output.write(file, data)
	if writeErr == nil && n != len(data) {
		writeErr = io.ErrShortWrite
	}
	var syncErr error
	if writeErr == nil {
		syncErr = output.sync(file)
	}
	info, statErr := file.Stat()
	if err := errors.Join(writeErr, syncErr, statErr, output.close(file)); err != nil {
		return err
	}
	currentTemp, err := root.Lstat(temp)
	if err != nil {
		return err
	}
	if !sameCopyEntry(info, currentTemp) {
		return fmt.Errorf("temporary stats changed")
	}
	if err := check(); err != nil {
		return err
	}
	return output.rename(root, temp, name)
}

func lockStats(file *os.File) error {
	deadline := time.Now().Add(time.Second)
	for {
		err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EINTR) {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("stats are busy; this action was not recorded")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
