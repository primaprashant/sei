package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

const representativeSHA = "469d00f4e67ff4a21eb6e6e467a086c9a1f1deb8"

// Extract data only, never repository hooks, links, or executable tooling.
// LICENSE is a sibling of skills, not an apparent managed skill.
func unpackRepresentative(r io.Reader, root string) error {
	z, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer func() { _ = z.Close() }()
	tr := tar.NewReader(z)
	prefix := "agent-skills-" + representativeSHA + "/"
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if h.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		if !strings.HasPrefix(h.Name, prefix) {
			return fmt.Errorf("unexpected archive prefix: %q", h.Name)
		}
		name := strings.TrimPrefix(h.Name, prefix)
		if name != "LICENSE" && name != "skills/" && !strings.HasPrefix(name, "skills/") {
			continue
		}
		if !filepath.IsLocal(name) || strings.Contains(name, "\\") {
			return fmt.Errorf("unsafe archive path: %q", name)
		}
		for _, component := range strings.Split(name, "/") {
			if component == ".." {
				return fmt.Errorf("archive traversal: %q", name)
			}
		}
		path := filepath.Join(root, name)
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0o700); err != nil {
				return err
			}
		case tar.TypeReg:
			if h.Size > 2<<20 {
				return fmt.Errorf("oversized fixture file: %q", name)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return err
			}
			f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600|os.FileMode(h.Mode)&0o111)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(f, tr)
			closeErr := f.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return fmt.Errorf("unsupported fixture entry: %q", name)
		}
	}
}

func TestRepresentativeFixture(t *testing.T) {
	// Supplemental test-only data. None of these names or bytes claim upstream provenance.
	for _, bad := range []string{"", "skills/../escape", "skills/link"} {
		t.Run(fmt.Sprintf("supplement-%q", bad), func(t *testing.T) {
			archive := filepath.Join(t.TempDir(), "fixture.gz")
			f, err := os.Create(archive)
			if err != nil {
				t.Fatal(err)
			}
			z := gzip.NewWriter(f)
			tw := tar.NewWriter(z)
			files := map[string]string{"LICENSE": "Supplemental test-only license marker", "skills/.dot/nested/.data": "hidden", "skills/\u754c-e\u0301/SKILL.md": "unicode", "skills/" + strings.Repeat("long-", 20) + "/script.sh": "not executed"}
			if bad != "" {
				files[bad] = "bad"
			}
			for name, data := range files {
				h := &tar.Header{Name: "agent-skills-" + representativeSHA + "/" + name, Mode: 0o755, Size: int64(len(data)), Typeflag: tar.TypeReg}
				if name == "skills/link" {
					h.Typeflag, h.Linkname, h.Size = tar.TypeSymlink, "outside", 0
					data = ""
				}
				if err := tw.WriteHeader(h); err != nil {
					t.Fatal(err)
				}
				if _, err := io.WriteString(tw, data); err != nil {
					t.Fatal(err)
				}
			}
			if err := tw.Close(); err != nil {
				t.Fatal(err)
			}
			if err := z.Close(); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			f, err = os.Open(archive)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := f.Close(); err != nil {
					t.Error(err)
				}
			}()
			root := t.TempDir()
			err = unpackRepresentative(f, root)
			if (err != nil) != (bad != "") {
				t.Fatalf("unpack: %v", err)
			}
			if bad != "" {
				return
			}
			entries, missing, err := scanFolder(filepath.Join(root, "skills"))
			if err != nil || missing || len(entries) != 3 {
				t.Fatalf("supplement scan: %v %v %v", entries, missing, err)
			}
			for name, data := range files {
				b, err := os.ReadFile(filepath.Join(root, name))
				if err != nil || string(b) != data {
					t.Fatalf("bytes %q: %v", name, err)
				}
			}
			cfg, err := resolveConfigPaths(config{Library: filepath.Join(root, "skills"), Agents: []agentConfig{{Name: "Supplement", Global: filepath.Join(root, "global"), Local: "local"}}}, root)
			if err != nil {
				t.Fatal(err)
			}
			m := newBrowseModel(cfg)
			next, refresh := m.Update(m.Init()())
			m = finishMutationRefresh(t, next.(browseModel), refresh)
			before := removeSnapshot(t, cfg.Library)
			for range entries {
				var worker tea.Cmd
				m, worker = press(m, 'a')
				if worker == nil {
					t.Fatal("supplement add not scheduled")
				}
				result := worker().(mutationResult)
				if result.err != nil {
					t.Fatal(result.err)
				}
				next, refresh = m.Update(result)
				m = finishMutationRefresh(t, next.(browseModel), refresh)
				m, _ = press(m, tea.KeyDown)
			}
			for source, entry := range before {
				if !entry.info.Mode().IsRegular() {
					continue
				}
				rel, err := filepath.Rel(cfg.Library, source)
				if err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(root, "local", rel)
				b, err := os.ReadFile(target)
				info, statErr := os.Stat(target)
				if err != nil || statErr != nil || string(b) != entry.data || os.SameFile(entry.info, info) || info.Mode()&0o111 != entry.info.Mode()&0o111 {
					t.Fatalf("supplement copy %q: %v %v", rel, err, statErr)
				}
			}
			assertRemoveSnapshot(t, cfg.Library, before)
		})
	}
}
