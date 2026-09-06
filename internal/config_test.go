package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

const validConfig = `{"library":"~/Library Skills","agents":[{"name":"One","global":"~/global","local":".local/skills"}]}`

func isolateConfigHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	return home
}

func writeTestFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestParseConfig(t *testing.T) {
	for _, tt := range []struct{ name, data, want string }{
		{"valid", validConfig, ""},
		{"whitespace", " \n" + validConfig + "\t\n", ""},
		{"escaped exact keys", strings.ReplaceAll(validConfig, `"library"`, `"libr\u0061ry"`), ""},
		{"empty input", "", "EOF"},
		{"invalid syntax", "{", "end"},
		{"truncated object", strings.TrimSuffix(validConfig, "}"), "end"},
		{"trailing comma", strings.TrimSuffix(validConfig, "}") + ",}", "invalid"},
		{"comment", validConfig + "// comment", "EOF"},
		{"second object", validConfig + "{}", "EOF"},
		{"second null", validConfig + "null", "EOF"},
		{"trailing junk", validConfig + "x", "EOF"},
		{"missing library", `{"agents":[{"name":"One","global":"/g","local":"l"}]}`, "library"},
		{"missing agents", `{"library":"/l"}`, "1-9"},
		{"empty agents", `{"library":"/l","agents":[]}`, "1-9"},
		{"empty library", strings.Replace(validConfig, "~/Library Skills", "", 1), "library"},
		{"duplicate root", strings.Replace(validConfig, `"library":`, `"library":"/other","library":`, 1), "duplicate key"},
		{"escaped duplicate root", strings.Replace(validConfig, `"library":`, `"libr\u0061ry":"/other","library":`, 1), "duplicate key"},
		{"duplicate agents", strings.Replace(validConfig, `"agents":`, `"agents":[],"agents":`, 1), "duplicate key"},
		{"unknown root", strings.Replace(validConfig, `"library":`, `"extra":{},"library":`, 1), "unknown"},
		{"wrong root case", strings.Replace(validConfig, `"library"`, `"Library"`, 1), "unknown"},
		{"wrong agents case", strings.Replace(validConfig, `"agents"`, `"AGENTS"`, 1), "unknown"},
		{"unknown agent", strings.Replace(validConfig, `"name":`, `"extra":[],"name":`, 1), "unknown"},
		{"empty agent", `{"library":"/l","agents":[{}]}`, "nonempty"},
		{"duplicate names", `{"library":"/l","agents":[{"name":"One","global":"/g","local":"l"},{"name":"\u004fne","global":"/h","local":"m"}]}`, "duplicate agent name"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseConfig([]byte(tt.data))
			if (tt.want == "" && err != nil) || (tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want))) {
				t.Fatalf("parseConfig(%s) error = %v, want %q", tt.data, err, tt.want)
			}
		})
	}
	for _, field := range []string{"name", "global", "local"} {
		for _, kind := range []string{"missing", "empty", "duplicate", "escaped duplicate", "case"} {
			t.Run(field+"/"+kind, func(t *testing.T) {
				fields := map[string]string{"name": `"One"`, "global": `"/g"`, "local": `"l"`}
				var entries []string
				for _, key := range []string{"name", "global", "local"} {
					if key == field {
						switch kind {
						case "missing":
							continue
						case "empty":
							fields[key] = `""`
						case "duplicate":
							entries = append(entries, fmt.Sprintf(`%q:%s`, key, fields[key]))
						case "escaped duplicate":
							entries = append(entries, fmt.Sprintf(`"\u%04x%s":%s`, key[0], key[1:], fields[key]))
						case "case":
							entries = append(entries, fmt.Sprintf(`%q:%s`, strings.ToUpper(key), fields[key]))
							continue
						}
					}
					entries = append(entries, fmt.Sprintf(`%q:%s`, key, fields[key]))
				}
				data := `{"library":"/l","agents":[{` + strings.Join(entries, ",") + `}]}`
				if _, err := parseConfig([]byte(data)); err == nil {
					t.Fatalf("accepted %s", data)
				}
			})
		}
	}
	for _, shape := range []string{"root", "library", "agents", "agent", "name", "global", "local"} {
		for _, value := range []string{"null", "true", "42", `"text"`, "[]", "{}"} {
			if (shape == "root" || shape == "agent") && value == "{}" || shape == "agents" && value == "[]" ||
				(shape == "library" || shape == "name" || shape == "global" || shape == "local") && value == `"text"` {
				continue
			}
			t.Run(shape+"/type/"+value, func(t *testing.T) {
				data := value
				switch shape {
				case "library":
					data = strings.Replace(validConfig, `"~/Library Skills"`, value, 1)
				case "agents":
					data = `{"library":"/l","agents":` + value + `}`
				case "agent":
					data = `{"library":"/l","agents":[` + value + `]}`
				case "name", "global", "local":
					old := map[string]string{"name": `"One"`, "global": `"~/global"`, "local": `".local/skills"`}[shape]
					data = strings.Replace(validConfig, old, value, 1)
				}
				if _, err := parseConfig([]byte(data)); err == nil {
					t.Fatalf("accepted %s", data)
				}
			})
		}
	}
	for _, count := range []int{0, 1, 3, 9, 10} {
		t.Run(fmt.Sprintf("count/%d", count), func(t *testing.T) {
			cfg := config{Library: "/l", Agents: []agentConfig{}}
			for i := range count {
				cfg.Agents = append(cfg.Agents, agentConfig{fmt.Sprintf("Agent %d", count-i), "/g", "l"})
			}
			data, err := json.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			got, err := parseConfig(data)
			if count == 0 || count == 10 {
				if err == nil {
					t.Fatal("accepted out-of-range count")
				}
			} else if err != nil || !reflect.DeepEqual(got, cfg) {
				t.Fatalf("got %+v, %v; want ordered %+v", got, err, cfg)
			}
		})
	}
}

func TestConfigPaths(t *testing.T) {
	home := isolateConfigHome(t)
	launch := t.TempDir()
	for _, tt := range []struct {
		name, xdg, override, want string
		fail                      bool
	}{
		{"native fallback", "", "", filepath.Join(home, ".config", "sei", "config.json"), false},
		{"native xdg", filepath.Join(home, "xdg"), "", filepath.Join(home, "xdg", "sei", "config.json"), false},
		{"relative xdg", "relative", "", "", runtime.GOOS == "linux"},
		{"override bypasses xdg", "relative", "custom.json", launch + "/custom.json", false},
		{"absolute override", "", home + "/custom.json", home + "/custom.json", false},
		{"raw override", "", "link/../config.json", launch + "/link/../config.json", false},
		{"spaces and Unicode override", "", "Config \u6280.json", launch + "/Config \u6280.json", false},
		{"no config tilde expansion", "", "~/config.json", launch + "/~/config.json", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", tt.xdg)
			want := tt.want
			if runtime.GOOS == "darwin" && tt.override == "" {
				want = filepath.Join(home, "Library", "Application Support", "sei", "config.json")
			}
			got, err := configLocation(launch, tt.override)
			if (err != nil) != tt.fail || !tt.fail && got != want {
				t.Fatalf("got %q, %v; want %q, failure %v", got, err, want, tt.fail)
			}
		})
	}
	for _, tt := range []struct {
		path, want string
		fail       bool
	}{
		{"~/Skills \u6280", home + "/Skills \u6280", false},
		{"~/link/../skills", home + "/link/../skills", false},
		{"~//skills", home + "//skills", false},
		{"/raw/link/../skills", "/raw/link/../skills", false},
		{"/literal/$HOME/$(cmd)/*/~other", "/literal/$HOME/$(cmd)/*/~other", false},
		{"", "", true}, {"relative", "", true}, {"~", "", true}, {"~other/skills", "", true},
		{"$HOME/skills", "", true}, {"$(pwd)/skills", "", true}, {"/bad\x00path", "", true},
	} {
		t.Run("root/"+tt.path, func(t *testing.T) {
			for _, field := range []string{"library", "global"} {
				cfg := config{Library: "/library", Agents: []agentConfig{{"One", "/global", "local"}}}
				if field == "library" {
					cfg.Library = tt.path
				} else {
					cfg.Agents[0].Global = tt.path
				}
				got, err := resolveConfigPaths(cfg, launch)
				if (err != nil) != tt.fail {
					t.Fatalf("%s %q: %v", field, tt.path, err)
				}
				if !tt.fail {
					path := got.Library
					if field == "global" {
						path = got.Agents[0].Global
					}
					if path != tt.want {
						t.Fatalf("got %q, want %q", path, tt.want)
					}
				}
			}
		})
	}
	for _, tt := range []struct {
		path string
		fail bool
	}{
		{".local/skills", false}, {"folder with spaces/\u6280", false}, {"link/../skills", false},
		{".", false}, {"a/..", false}, {"..hidden/skills", false}, {"$HOME/*/$(cmd)", false},
		{"~/skills", false}, {"", true}, {"/absolute", true}, {"..", true}, {"../sibling", true},
		{"a/../../escape", true}, {"../project-sibling", true}, {"bad\x00path", true},
	} {
		t.Run("local/"+tt.path, func(t *testing.T) {
			cfg := config{Library: "/library", Agents: []agentConfig{{"One", "/global", tt.path}}}
			got, err := resolveConfigPaths(cfg, launch)
			if (err != nil) != tt.fail {
				t.Fatalf("%q: %v", tt.path, err)
			}
			if !tt.fail && (got.Agents[0].Local != launch+"/"+tt.path || cfg.Agents[0].Local != tt.path) {
				t.Fatalf("raw local or original config changed: %+v, %+v", got, cfg)
			}
		})
	}
	t.Run("missing home", func(t *testing.T) {
		t.Setenv("HOME", "")
		if _, err := expandRoot("~/skills"); err == nil {
			t.Fatal("expanded absent home")
		}
		if got, err := expandRoot("/skills"); err != nil || got != "/skills" {
			t.Fatalf("absolute root needs no home: %q %v", got, err)
		}
	})
}

func TestConfigPathsProject(t *testing.T) {
	isolateConfigHome(t)
	root := t.TempDir()
	launch := filepath.Join(root, "repo", "subdir")
	if err := os.MkdirAll(launch, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "repo", ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(launch, "Project \u6280"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(launch, "file"), "file")
	for _, tt := range []struct {
		path, want string
		fail       bool
	}{
		{"", launch, false}, {".", launch + "/.", false}, {"..", launch + "/..", false},
		{"Project \u6280", launch + "/Project \u6280", false},
		{root, root, false}, {"missing", "", true}, {"file", "", true}, {"~/project", "", true},
	} {
		t.Run(tt.path, func(t *testing.T) {
			got, err := resolveProject(launch, tt.path)
			if (err != nil) != tt.fail || !tt.fail && got != tt.want {
				t.Fatalf("got %q, %v; want %q", got, err, tt.want)
			}
		})
	}
	// A cleaned path would refer to launch, but the kernel traverses the link
	// before '..' and reaches the outside directory instead.
	outside := t.TempDir()
	if err := os.Mkdir(filepath.Join(outside, "child"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "child"), filepath.Join(launch, "link")); err != nil {
		t.Fatal(err)
	}
	project, err := resolveProject(launch, "link/..")
	if err != nil || project != launch+"/link/.." {
		t.Fatalf("raw project = %q, %v", project, err)
	}
	actual, err := os.Stat(project)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.Stat(outside)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(actual, want) {
		t.Fatal("project traversal changed")
	}
	writeTestFile(t, filepath.Join(outside, "config.json"), validConfig)
	writeTestFile(t, filepath.Join(launch, "config.json"), "malformed")
	path, err := configLocation(launch, "link/../config.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, missing, err := loadConfig(path); missing || err != nil {
		t.Fatalf("raw config traversal: missing %v, %v", missing, err)
	}
}
