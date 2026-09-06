package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func benchmarkConfig(b *testing.B, count int) config {
	b.Helper()
	base := b.TempDir()
	cfg := config{Home: filepath.Join(base, "home"), Project: filepath.Join(base, "project")}
	cfg.Library = filepath.Join(cfg.Home, "skill-library")
	agents := []struct{ name, folder string }{
		{"Claude Code", ".claude"}, {"Codex", ".codex"}, {"OpenCode", ".opencode"},
		{"Gemini CLI", ".gemini"}, {"GitHub Copilot", ".copilot"}, {"Cursor", ".cursor"},
		{"Windsurf", ".windsurf"}, {"Cline", ".cline"}, {"Roo Code", ".roo"},
	}
	paths := []string{cfg.Home, cfg.Project, cfg.Library}
	for _, agent := range agents[:count] {
		global := filepath.Join(cfg.Home, agent.folder, "skills")
		local := filepath.Join(cfg.Project, agent.folder, "skills")
		cfg.Agents = append(cfg.Agents, agentConfig{Name: agent.name, Global: global, Local: local})
		paths = append(paths, global, local)
	}
	for _, path := range paths {
		if err := os.MkdirAll(path, 0o700); err != nil {
			b.Fatal(err)
		}
	}
	for id, err := range resolveRoots(cfg).blocked {
		if err != nil {
			b.Fatalf("fixture root %d: %v", id, err)
		}
	}
	return cfg
}

// Render-only microbenchmark, not startup or usability evidence. Each panel has
// 25 synthetic empty folders (0 payload bytes), scanned before timing.
func BenchmarkView(b *testing.B) {
	for _, count := range []int{1, 3, 9} {
		b.Run(fmt.Sprintf("agents=%d/143x35/folders=25", count), func(b *testing.B) {
			b.StopTimer()
			m := newBrowseModel(benchmarkConfig(b, count))
			m.width, m.height = 143, 35
			for i := range m.panels {
				p := &m.panels[i]
				for n := range 25 {
					name := fmt.Sprintf("%02d-synthetic-skill-with-a-long-descriptive-folder-name", n+1)
					if err := os.Mkdir(filepath.Join(p.path, name), 0o700); err != nil {
						b.Fatal(err)
					}
				}
				var err error
				p.entries, p.missing, err = scanFolder(p.path)
				if err != nil || p.missing || len(p.entries) != 25 {
					b.Fatalf("scan %q: entries=%d missing=%v err=%v", p.path, len(p.entries), p.missing, err)
				}
				p.loading, p.safetyChecked = false, true
				p.selectedName = p.entries[0].name
			}
			b.ReportAllocs()
			var content string
			b.StartTimer()
			for i := 0; i < b.N; i++ {
				content = m.View().Content
			}
			b.StopTimer()
			if content == "" {
				b.Fatal("empty view")
			}
		})
	}
}

// One synthetic skill: SKILL.md (4096 bytes), references/notes.md (2048), and
// .metadata (128), totaling 6272 bytes in three non-executable regular files.
// File-operation microbenchmarks include production safety checks, not UI/startup.
func BenchmarkSkill(b *testing.B) {
	for _, operation := range []string{"fresh-add", "replacement", "remove"} {
		b.Run(operation+"/files=3/bytes=6272", func(b *testing.B) {
			b.StopTimer()
			cfg := benchmarkConfig(b, 1)
			const name = "synthetic-skill"
			const destination panelID = 2 // Claude Code / Local.
			for _, file := range []struct {
				path string
				size int
			}{
				{"SKILL.md", 4096}, {"references/notes.md", 2048}, {".metadata", 128},
			} {
				path := filepath.Join(cfg.Library, name, file.path)
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					b.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(strings.Repeat("x", file.size)), 0o600); err != nil {
					b.Fatal(err)
				}
			}
			if operation == "replacement" {
				if err := addSkill(cfg, destination, name); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			if operation != "remove" {
				// Logical payload copied, not total preflight/read/write traffic.
				// Removal only changes metadata, so it has no byte-throughput metric.
				b.SetBytes(6272)
			}
			for i := 0; i < b.N; i++ {
				if operation == "remove" {
					if err := addSkill(cfg, destination, name); err != nil {
						b.Fatal(err)
					}
				}
				b.StartTimer()
				var err error
				if operation == "remove" {
					err = removeSkill(cfg, destination, name)
				} else {
					err = addSkill(cfg, destination, name)
				}
				b.StopTimer()
				if err != nil {
					b.Fatal(err)
				}
				if operation == "fresh-add" {
					if err := os.RemoveAll(filepath.Join(cfg.Agents[0].Local, name)); err != nil {
						b.Fatal(err)
					}
				}
				// Replacement reuses the previous copy: identical content still
				// exercises production deletion and copying, never a no-op.
			}
		})
	}
}
