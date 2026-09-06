package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func FuzzParseConfig(f *testing.F) {
	for _, seed := range []string{
		validConfig, "", "null", "{}", "{", validConfig + "{}",
		`{"library":null,"agents":[]}`,
		strings.Replace(validConfig, `"library":`, `"libr\u0061ry":"/other","library":`, 1),
		strings.Replace(validConfig, `"name":`, `"n\u0061me":"Other","name":`, 1),
		strings.Replace(validConfig, `"library"`, `"Library"`, 1),
		strings.Replace(validConfig, "~/Library Skills", "../link/../escape", 1),
		strings.Replace(validConfig, "One", `\u0000\u001b\ufffd`, 1),
		strings.Replace(validConfig, "One", "raw\xff", 1),
		strings.Replace(validConfig, "One", `\ud800`, 1),
	} {
		f.Add([]byte(seed))
	}
	for _, count := range []int{0, 1, 9, 10} {
		cfg := config{Library: "/library", Agents: []agentConfig{}}
		for i := range count {
			cfg.Agents = append(cfg.Agents, agentConfig{fmt.Sprint(i), "~/global", ".local/skills"})
		}
		data, err := json.Marshal(cfg)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
		if count == 9 {
			cfg.Agents[8].Name = cfg.Agents[0].Name
			data, err = json.Marshal(cfg)
			if err != nil {
				f.Fatal(err)
			}
			f.Add(data)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		before := bytes.Clone(data)
		cfg, err := parseConfig(data)
		again, againErr := parseConfig(data)
		if !bytes.Equal(data, before) || !reflect.DeepEqual(cfg, again) || fmt.Sprint(err) != fmt.Sprint(againErr) {
			t.Fatal("parser mutated input or was nondeterministic")
		}
		if err != nil {
			if !reflect.DeepEqual(cfg, config{}) {
				t.Fatal("failed parse leaked partial configuration")
			}
			return
		}
		if !json.Valid(data) || cfg.Library == "" || len(cfg.Agents) < 1 || len(cfg.Agents) > 9 || cfg.Home != "" || cfg.Project != "" || cfg.ConfigPath != "" {
			t.Fatal("accepted invalid schema or populated runtime paths")
		}
		names := map[string]bool{}
		for _, agent := range cfg.Agents {
			if agent.Name == "" || agent.Global == "" || agent.Local == "" || names[agent.Name] {
				t.Fatal("accepted empty field or duplicate agent name")
			}
			names[agent.Name] = true
		}
		canonical, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		for _, encoded := range [][]byte{canonical, append(append([]byte(" \n"), data...), '\t', '\n')} {
			roundtrip, err := parseConfig(encoded)
			if err != nil || !reflect.DeepEqual(cfg, roundtrip) {
				t.Fatalf("round trip changed values/order: %v", err)
			}
		}
		for _, invalid := range []string{
			string(data) + " null",
			`{"libr\u0061ry":"duplicate",` + string(canonical[1:]),
			`{"agents":[],` + string(canonical[1:]),
			strings.Replace(string(canonical), `"name":`, `"n\u0061me":"duplicate","name":`, 1),
			strings.Replace(string(canonical), `"library":`, `"Library":`, 1),
			`{"unknown":null,` + string(canonical[1:]),
		} {
			if _, err := parseConfig([]byte(invalid)); err == nil {
				t.Fatal("accepted trailing value, duplicate, or non-schema key")
			}
		}
	})
}

func FuzzPathName(f *testing.F) {
	// Only lexical APIs receive fuzz input. Never stat, open, resolve symlinks,
	// or mutate a generated path, even when configuration parsing accepts it.
	base := f.TempDir()
	f.Setenv("HOME", base)
	f.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "xdg"))
	for _, seed := range []string{
		"", ".", "..", "../escape", "a/../../escape", "/absolute", "//absolute",
		"a/..", "link/../skills", "missing/../skills", "..hidden", ".hidden",
		"~/skills", "~//skills", "~other/skills", "$HOME/$(cmd)/*", "back\\slash",
		"with spaces", "\u6280/e\u0301", "raw\xff", "bad\x00name", "\x1b[31m\n\r\t",
		"Skill", "skill", strings.Repeat("a", 256),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		validName := raw != "" && raw != "." && raw != ".."
		for _, b := range []byte(raw) {
			if b == 0 || b == '/' || b == byte(filepath.Separator) {
				validName = false
			}
		}
		if err := validateSkillName(raw); (err == nil) != validName {
			t.Fatalf("name acceptance changed for %q: %v", raw, err)
		}
		if validName {
			child := filepath.Join(base, raw)
			if filepath.Dir(child) != base || filepath.Base(child) != raw || !pathWithin(base, child) || pathWithin(child, base) {
				t.Fatal("accepted name escaped its parent or changed raw bytes")
			}
		}
		cfg := config{Library: base + "/library", Agents: []agentConfig{{"One", base + "/global", raw}}}
		resolved, err := resolveConfigPaths(cfg, base)
		// Independent component walk: leading parent traversal cannot be canceled
		// by later ordinary components. Dot components need no filesystem lookup.
		local := raw != "" && !strings.HasPrefix(raw, "/") && !strings.ContainsRune(raw, 0)
		depth := 0
		for _, part := range strings.Split(raw, "/") {
			switch part {
			case "", ".":
			case "..":
				depth--
				if depth < 0 {
					local = false
				}
			default:
				depth++
			}
		}
		if (err == nil) != local || cfg.Agents[0].Local != raw {
			t.Fatalf("local acceptance or input changed for %q: %v", raw, err)
		}
		if err == nil {
			want := base + string(filepath.Separator) + raw
			if resolved.Agents[0].Local != want || !pathWithin(base, want) || pathWithin(base, base+"-sibling/"+raw) {
				t.Fatal("local path escaped or raw traversal was cleaned")
			}
		}
		expanded, err := expandRoot(raw)
		rootOK := !strings.ContainsRune(raw, 0) && (filepath.IsAbs(raw) || strings.HasPrefix(raw, "~/"))
		if (err == nil) != rootOK {
			t.Fatalf("root acceptance changed for %q: %v", raw, err)
		}
		if err == nil {
			want := raw
			if strings.HasPrefix(raw, "~/") {
				want = base + string(filepath.Separator) + raw[2:]
			}
			if expanded != want || !filepath.IsAbs(expanded) {
				t.Fatal("root expansion cleaned traversal or evaluated shell text")
			}
		}
	})
}
