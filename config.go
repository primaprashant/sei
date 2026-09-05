package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type config struct {
	Project string        `json:"-"`
	Home    string        `json:"-"`
	Library string        `json:"library"`
	Agents  []agentConfig `json:"agents"`
}

type agentConfig struct {
	Name   string `json:"name"`
	Global string `json:"global"`
	Local  string `json:"local"`
}

func parseConfig(data []byte) (config, error) {
	var cfg config
	if err := configTokens(json.NewDecoder(bytes.NewReader(data)), "config"); err != nil {
		return cfg, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return config{}, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return config{}, fmt.Errorf("expected EOF after configuration, got %v", err)
	}
	if cfg.Library == "" {
		return config{}, fmt.Errorf("library is required and must not be empty")
	}
	if len(cfg.Agents) < 1 || len(cfg.Agents) > 9 {
		return config{}, fmt.Errorf("agents must contain 1-9 entries")
	}
	names := make(map[string]bool)
	for i, agent := range cfg.Agents {
		if agent.Name == "" || agent.Global == "" || agent.Local == "" {
			return config{}, fmt.Errorf("agent %d requires nonempty name, global, and local", i+1)
		}
		if names[agent.Name] {
			return config{}, fmt.Errorf("duplicate agent name %q", agent.Name)
		}
		names[agent.Name] = true
	}
	return cfg, nil
}

// configTokens checks decoded keys and exact shapes before struct decoding can
// collapse duplicates or accept case-insensitive field names and null values.
func configTokens(decoder *json.Decoder, shape string) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	switch shape {
	case "config", "agent":
		if token != json.Delim('{') {
			return fmt.Errorf("%s must be an object", shape)
		}
		seen := make(map[string]bool)
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := token.(string)
			if !ok {
				return fmt.Errorf("expected object key")
			}
			if seen[key] {
				return fmt.Errorf("duplicate key %q", key)
			}
			seen[key] = true
			child := "string"
			switch {
			case shape == "config" && key == "agents":
				child = "agents"
			case shape == "config" && key == "library":
			case shape == "agent" && (key == "name" || key == "global" || key == "local"):
			default:
				return fmt.Errorf("unknown %s key %q", shape, key)
			}
			if err := configTokens(decoder, child); err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
		}
	case "agents":
		if token != json.Delim('[') {
			return fmt.Errorf("agents must be an array")
		}
		for decoder.More() {
			if err := configTokens(decoder, "agent"); err != nil {
				return err
			}
		}
	default:
		if _, ok := token.(string); !ok {
			return fmt.Errorf("expected string")
		}
		return nil
	}
	_, err = decoder.Token() // The decoder verifies the matching closing delimiter.
	return err
}

// absolutePath deliberately avoids Join/Abs: cleaning link/.. changes filesystem
// traversal. Keep these paths intact for physical root validation in Task 8.
func absolutePath(base, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return base + string(filepath.Separator) + path
}

func configLocation(launch, override string) (string, error) {
	if override != "" {
		return absolutePath(launch, override), nil
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); runtime.GOOS == "linux" && xdg != "" && !filepath.IsAbs(xdg) {
		return "", fmt.Errorf("XDG_CONFIG_HOME must be absolute")
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate configuration: %w", err)
	}
	return absolutePath(dir, "sei/config.json"), nil
}

func resolveProject(launch, override string) (string, error) {
	project := launch
	if override != "" {
		project = absolutePath(launch, override)
	}
	info, err := os.Stat(project)
	if err != nil {
		return "", fmt.Errorf("inspect project %q: %w", project, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project %q is not a directory", project)
	}
	return project, nil
}

func expandRoot(path string) (string, error) {
	if strings.ContainsRune(path, 0) {
		return "", fmt.Errorf("path contains NUL")
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = home + string(filepath.Separator) + path[2:]
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("path %q must be absolute or start with ~/", path)
	}
	return path, nil
}

func resolveConfigPaths(cfg config, project string) (config, error) {
	resolved := config{Agents: make([]agentConfig, len(cfg.Agents)), Project: project}
	resolved.Home, _ = os.UserHomeDir()
	var err error
	resolved.Library, err = expandRoot(cfg.Library)
	if err != nil {
		return config{}, fmt.Errorf("library: %w", err)
	}
	for i, agent := range cfg.Agents {
		global, err := expandRoot(agent.Global)
		if err != nil {
			return config{}, fmt.Errorf("agent %q global: %w", agent.Name, err)
		}
		if !filepath.IsLocal(agent.Local) || strings.ContainsRune(agent.Local, 0) {
			return config{}, fmt.Errorf("agent %q local path %q must be relative and remain within the project", agent.Name, agent.Local)
		}
		resolved.Agents[i] = agentConfig{agent.Name, global, absolutePath(project, agent.Local)}
	}
	return resolved, nil
}

func loadConfig(path string) (cfg config, missing bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// ENOENT also describes dangling links. Inspect raw prefixes so neither
			// a dangling config nor a dangling ancestor is mistaken for first use.
			prefix := ""
			for _, component := range strings.Split(path, string(filepath.Separator)) {
				prefix += component
				if prefix == "" {
					prefix = string(filepath.Separator)
					continue
				}
				info, inspectErr := os.Lstat(prefix)
				if errors.Is(inspectErr, os.ErrNotExist) {
					return config{}, true, nil
				}
				if inspectErr != nil {
					return config{}, false, fmt.Errorf("inspect config path %q: %w", prefix, inspectErr)
				}
				if info.Mode()&os.ModeSymlink != 0 {
					if _, linkErr := os.Stat(prefix); linkErr != nil {
						return config{}, false, fmt.Errorf("resolve config link %q: %w", prefix, linkErr)
					}
				}
				prefix += string(filepath.Separator)
			}
		}
		return config{}, false, fmt.Errorf("read config %q: %w", path, err)
	}
	cfg, err = parseConfig(data)
	if err != nil {
		return config{}, false, fmt.Errorf("parse config %q: %w", path, err)
	}
	return cfg, false, nil
}
