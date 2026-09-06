package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func saveConfigFixture(t *testing.T) (config, string, string) {
	t.Helper()
	base := t.TempDir()
	home, project := base+"/home", base+"/project"
	for _, dir := range []string{home, project} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	return config{Library: "~/library", Agents: []agentConfig{
		{Name: "Example", Global: "~/global", Local: ".example/skills"},
	}}, project, home + "/config/sei/config.json"
}

func TestSaveConfig(t *testing.T) {
	t.Run("validate is read only and returns resolved copy", func(t *testing.T) {
		cfg, project, path := saveConfigFixture(t)
		resolved, err := validateSetupConfig(cfg, project, path)
		if err != nil {
			t.Fatal(err)
		}
		if resolved.Project != project || resolved.Home != os.Getenv("HOME") || resolved.Library != os.Getenv("HOME")+"/library" || resolved.Agents[0].Local != project+"/.example/skills" {
			t.Fatalf("unexpected resolved config: %+v", resolved)
		}
		resolved.Agents[0].Name = "changed"
		if cfg.Agents[0].Name != "Example" {
			t.Fatal("validation mutated input")
		}
		for _, absent := range []string{filepath.Dir(filepath.Dir(path)), resolved.Library, resolved.Agents[0].Global, project + "/.example"} {
			if _, err := os.Lstat(absent); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("validation created %q: %v", absent, err)
			}
		}
	})

	t.Run("first save and replacement", func(t *testing.T) {
		cfg, project, path := saveConfigFixture(t)
		cfg.Project, cfg.Home = "not persisted", "not persisted"
		if err := saveConfig(cfg, project, path, false); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		want, _ := json.MarshalIndent(cfg, "", "  ")
		want = append(want, '\n')
		if !bytes.Equal(data, want) {
			t.Fatalf("saved %s; want %s", data, want)
		}
		loaded, missing, err := loadConfig(path)
		cfg.Project, cfg.Home = "", ""
		if err != nil || missing || !reflect.DeepEqual(loaded, cfg) {
			t.Fatalf("round trip: %+v, %v, %v", loaded, missing, err)
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("private config: %v, %v", info, err)
		}
		cfg.Agents[0].Name = "Replacement"
		if err := saveConfig(cfg, project, path, false); !errors.Is(err, os.ErrExist) {
			t.Fatalf("first-run overwrite: %v", err)
		}
		unchanged, _ := os.ReadFile(path)
		if !bytes.Equal(data, unchanged) {
			t.Fatal("refused overwrite changed bytes")
		}
		if err := saveConfig(cfg, project, path, true); err != nil {
			t.Fatal(err)
		}
		loaded, _, err = loadConfig(path)
		if err != nil || !reflect.DeepEqual(loaded, cfg) {
			t.Fatalf("replacement: %+v, %v", loaded, err)
		}
		entries, err := os.ReadDir(filepath.Dir(path))
		if err != nil || len(entries) != 1 || entries[0].Name() != "config.json" {
			t.Fatalf("temporary output remains: %v, %v", entries, err)
		}
		for _, absent := range []string{os.Getenv("HOME") + "/library", os.Getenv("HOME") + "/global", project + "/.example"} {
			if _, err := os.Lstat(absent); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("save created managed directories %q: %v", absent, err)
			}
		}
	})

	for _, kind := range []string{"empty library", "no agents", "duplicate names", "too many agents", "relative global", "absolute local", "escaping local", "invalid utf8", "overlap", "local alias escape", "dangling root", "missing project", "relative config", "trailing slash", "missing dotdot", "library placement", "global placement", "local placement", "root equality", "parent alias placement", "root alias placement", "config symlink", "dangling config", "config directory", "parent file", "dangling parent", "malformed existing"} {
		t.Run(kind, func(t *testing.T) {
			cfg, project, path := saveConfigFixture(t)
			home := os.Getenv("HOME")
			link := func(target, name string) {
				t.Helper()
				if err := os.Symlink(target, name); err != nil {
					t.Fatal(err)
				}
			}
			switch kind {
			case "empty library":
				cfg.Library = ""
			case "no agents":
				cfg.Agents = nil
			case "duplicate names":
				cfg.Agents = append(cfg.Agents, cfg.Agents[0])
			case "too many agents":
				cfg.Agents = make([]agentConfig, 10)
			case "relative global":
				cfg.Agents[0].Global = "relative"
			case "absolute local":
				cfg.Agents[0].Local = home + "/local"
			case "escaping local":
				cfg.Agents[0].Local = "../escape"
			case "invalid utf8":
				cfg.Library += "\xff"
			case "overlap":
				cfg.Agents[0].Global = "~/library/nested"
			case "local alias escape":
				link(home, project+"/.example")
			case "dangling root":
				link("absent", home+"/library")
			case "missing project":
				project += "/absent"
			case "relative config":
				path = "config.json"
			case "trailing slash":
				path += "/"
			case "missing dotdot":
				path = home + "/absent/../config.json"
			case "library placement":
				path = home + "/library/new/config.json"
			case "global placement":
				path = home + "/global/new/config.json"
			case "local placement":
				path = project + "/.example/skills/new/config.json"
			case "root equality":
				path = home + "/global"
			case "parent alias placement":
				if err := os.Mkdir(home+"/library", 0o700); err != nil {
					t.Fatal(err)
				}
				link(home+"/library", home+"/alias")
				path = home + "/alias/new/config.json"
			case "root alias placement":
				if err := os.Mkdir(home+"/real", 0o700); err != nil {
					t.Fatal(err)
				}
				link(home+"/real", home+"/library")
				path = home + "/real/new/config.json"
			case "config symlink", "dangling config":
				path = home + "/config.json"
				if kind == "config symlink" {
					writeTestFile(t, home+"/target", "untouched")
				}
				link("target", path)
			case "config directory":
				path = home
			case "parent file":
				writeTestFile(t, home+"/config", "untouched")
			case "dangling parent":
				link("absent", home+"/config")
			case "malformed existing":
				path = home + "/config.json"
				writeTestFile(t, path, `{"library":"x","library":"y"}`)
			}
			before, _ := os.ReadFile(path)
			for _, replace := range []bool{false, true} {
				if err := saveConfig(cfg, project, path, replace); err == nil {
					t.Fatalf("unsafe save accepted, replace=%v", replace)
				}
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(before, after) {
				t.Fatal("failed save changed existing bytes")
			}
			if err := filepath.WalkDir(home, func(path string, entry os.DirEntry, err error) error {
				if err == nil && (strings.HasPrefix(entry.Name(), ".sei-config-") || entry.Name() == "new" || entry.Name() == "sei") {
					t.Errorf("failed save left output %q", path)
				}
				return err
			}); err != nil {
				t.Fatal(err)
			}
		})
	}

	t.Run("safe parent alias and raw dotdot", func(t *testing.T) {
		cfg, project, _ := saveConfigFixture(t)
		home := os.Getenv("HOME")
		if err := os.MkdirAll(home+"/real/deep", 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(home+"/real/deep", home+"/alias"); err != nil {
			t.Fatal(err)
		}
		if err := saveConfig(cfg, project, home+"/alias/../new/config.json", false); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(home + "/real/new/config.json"); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(home + "/new"); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("raw path was cleaned: %v", err)
		}
	})

	t.Run("observed parent replacement", func(t *testing.T) {
		_, _, path := saveConfigFixture(t)
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		before := resolveRoot(dir)
		if err := os.Rename(dir, dir+"-old"); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := checkConfigParent(before, resolveRoot(dir)); err == nil {
			t.Fatal("replaced parent accepted")
		}
	})

	t.Run("nonregular config", func(t *testing.T) {
		cfg, project, _ := saveConfigFixture(t)
		path := os.Getenv("HOME") + "/pipe"
		if err := unix.Mkfifo(path, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := saveConfig(cfg, project, path, true); err == nil {
			t.Fatal("FIFO config accepted")
		}
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeNamedPipe == 0 {
			t.Fatalf("FIFO changed: %v, %v", info, err)
		}
	})

	t.Run("unwritable parent preserves existing config", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root bypasses directory write permissions")
		}
		cfg, project, path := saveConfigFixture(t)
		if err := saveConfig(cfg, project, path, false); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		dir := filepath.Dir(path)
		if err := os.Chmod(dir, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.Chmod(dir, 0o700); err != nil {
				t.Error(err)
			}
		})
		cfg.Agents[0].Name = "Changed"
		if err := saveConfig(cfg, project, path, true); err == nil {
			t.Fatal("save into unwritable parent succeeded")
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("failed save changed bytes: %v", err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 1 {
			t.Fatalf("failed save left temporary output: %v, %v", entries, err)
		}
	})
}
