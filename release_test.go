package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"debug/elf"
	"debug/macho"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestReleaseConfig(t *testing.T) {
	// JSON is a YAML subset: check the entire small contract without adding a
	// YAML dependency or requiring release tooling in ordinary tests.
	want := `{
		"version":2,"project_name":"sei",
		"env":["GOTOOLCHAIN=go1.27.1","GOFLAGS=-mod=readonly"],
		"builds":[{"id":"sei","main":".","binary":"sei","env":["CGO_ENABLED=0"],
			"goos":["linux","darwin"],"goarch":["amd64","arm64"],
			"goamd64":["v1"],"goarm64":["v8.0"],"flags":["-trimpath"],
			"ldflags":["-s -w -X main.version={{ .Version }}"],"mod_timestamp":"{{ .CommitTimestamp }}"}],
		"archives":[{"formats":["tar.gz"],"name_template":"sei_{{ .Version }}_{{ .Os }}_{{ .Arch }}",
			"files":["README.md","LICENSE","THIRD_PARTY_NOTICES"]}],
		"checksum":{"name_template":"sei_{{ .Version }}_checksums.txt","algorithm":"sha256"},
		"snapshot":{"version_template":"{{ .Version }}-snapshot.{{ .ShortCommit }}"},
		"changelog":{"disable":true},"release":{"disable":true}
	}`
	data, err := os.ReadFile(".goreleaser.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var gotConfig, wantConfig any
	if err := json.Unmarshal(data, &gotConfig); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &wantConfig); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotConfig, wantConfig) {
		t.Fatal("release contract changed; review targets, archive allowlist, version and publication policy")
	}
}

func TestReleaseArchives(t *testing.T) {
	dist := os.Getenv("SEI_RELEASE_DIST")
	if dist == "" {
		t.Skip("opt in with SEI_RELEASE_DIST and SEI_RELEASE_VERSION after a verified GoReleaser snapshot")
	}
	v := os.Getenv("SEI_RELEASE_VERSION")
	if v == "" || strings.ContainsAny(v, "/\\ \n\t") || strings.HasPrefix(v, "v") {
		t.Fatal("SEI_RELEASE_VERSION must be the expected version without v")
	}
	manifest, err := os.ReadFile(filepath.Join(dist, "sei_"+v+"_checksums.txt"))
	if err != nil {
		t.Fatal(err)
	}
	sums := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(manifest)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || len(fields[0]) != 64 || sums[fields[1]] != "" {
			t.Fatalf("invalid/duplicate checksum entry: %q", line)
		}
		sums[fields[1]] = fields[0]
	}
	archives, err := filepath.Glob(filepath.Join(dist, "*.tar.gz"))
	if err != nil || len(archives) != 4 || len(sums) != 4 {
		t.Fatalf("want exactly four archives/checksums: %v, %v, %v", archives, sums, err)
	}
	for _, goos := range []string{"linux", "darwin"} {
		for _, arch := range []string{"amd64", "arm64"} {
			t.Run(goos+"/"+arch, func(t *testing.T) {
				name := "sei_" + v + "_" + goos + "_" + arch + ".tar.gz"
				data, err := os.ReadFile(filepath.Join(dist, name))
				if err != nil {
					t.Fatal(err)
				}
				if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != sums[name] {
					t.Fatalf("checksum mismatch: %s: %s != %s", name, got, sums[name])
				}
				gz, err := gzip.NewReader(bytes.NewReader(data))
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = gz.Close() }()
				tr := tar.NewReader(gz)
				seen := map[string]bool{}
				var binary []byte
				for {
					h, err := tr.Next()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
					if seen[h.Name] || h.Typeflag != tar.TypeReg || h.Size > 32<<20 {
						t.Fatalf("invalid archive member: %+v", h)
					}
					seen[h.Name] = true
					content, err := io.ReadAll(tr)
					if err != nil {
						t.Fatal(err)
					}
					switch h.Name {
					case "sei":
						if h.Mode != 0755 {
							t.Fatalf("binary mode: %o", h.Mode)
						}
						binary = content
					case "README.md", "LICENSE", "THIRD_PARTY_NOTICES":
						want, err := os.ReadFile(h.Name)
						if err != nil || !bytes.Equal(content, want) {
							t.Fatalf("archive documentation differs: %s: %v", h.Name, err)
						}
					default:
						t.Fatalf("unexpected member: %q", h.Name)
					}
				}
				if len(seen) != 4 || len(binary) == 0 {
					t.Fatalf("incomplete archive: %v", seen)
				}
				// Drain the gzip stream to verify its trailer as well as the tar.
				if _, err := io.Copy(io.Discard, gz); err != nil {
					t.Fatal(err)
				}
				info, err := buildinfo.Read(bytes.NewReader(binary))
				if err != nil {
					t.Fatal(err)
				}
				if info.GoVersion != "go1.27.1" || info.Path != "github.com/primaprashant/sei" {
					t.Fatalf("unexpected build provenance: %v", info)
				}
				settings := map[string]string{}
				for _, s := range info.Settings {
					settings[s.Key] = s.Value
				}
				baselineKey, baseline := "GOAMD64", "v1"
				if arch == "arm64" {
					baselineKey, baseline = "GOARM64", "v8.0"
				}
				for key, want := range map[string]string{"GOOS": goos, "GOARCH": arch, "CGO_ENABLED": "0", "-trimpath": "true", baselineKey: baseline} {
					if settings[key] != want {
						t.Errorf("%s = %q, want %q", key, settings[key], want)
					}
				}
				notices, err := os.ReadFile("THIRD_PARTY_NOTICES")
				if err != nil {
					t.Fatal(err)
				}
				moduleSums, err := os.ReadFile("go.sum")
				if err != nil {
					t.Fatal(err)
				}
				for _, dep := range info.Deps {
					if dep.Replace != nil || !bytes.Contains(notices, []byte(dep.Path+" "+dep.Version+"\n")) ||
						!bytes.Contains(moduleSums, []byte(dep.Path+" "+dep.Version+" "+dep.Sum+"\n")) {
						t.Errorf("unaudited dependency: %+v", dep)
					}
				}
				if goos == "linux" {
					f, err := elf.NewFile(bytes.NewReader(binary))
					if err != nil {
						t.Fatal(err)
					}
					wantMachine := elf.EM_X86_64
					if arch == "arm64" {
						wantMachine = elf.EM_AARCH64
					}
					if f.Machine != wantMachine || f.Type != elf.ET_EXEC {
						t.Fatalf("unexpected ELF header: %+v", f.FileHeader)
					}
					if f.Section(".symtab") != nil || f.Section(".debug_info") != nil || f.Section(".dynamic") != nil {
						t.Fatal("ELF must be stripped and statically linked")
					}
					for _, p := range f.Progs {
						if p.Type == elf.PT_INTERP {
							t.Fatal("ELF has a runtime interpreter")
						}
					}
				} else {
					f, err := macho.NewFile(bytes.NewReader(binary))
					if err != nil {
						t.Fatal(err)
					}
					wantCPU := macho.CpuAmd64
					if arch == "arm64" {
						wantCPU = macho.CpuArm64
					}
					if f.Cpu != wantCPU || f.Type != macho.TypeExec {
						t.Fatalf("unexpected Mach-O header: %+v", f.FileHeader)
					}
					if f.Section("__debug_info") != nil {
						t.Fatal("Mach-O contains DWARF debug information")
					}
				}
				if goos != runtime.GOOS || arch != runtime.GOARCH {
					t.Log("metadata checked; execution requires a matching native host")
					return
				}
				home := t.TempDir()
				exe := filepath.Join(home, "sei")
				if err := os.WriteFile(exe, binary, 0755); err != nil {
					t.Fatal(err)
				}
				for flag, want := range map[string]string{"--help": usage, "--version": "sei " + v + "\n"} {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					cmd := exec.CommandContext(ctx, exe, flag)
					cmd.Dir = home
					cmd.Env = []string{"PATH=", "HOME=" + home, "XDG_CONFIG_HOME=" + home}
					var stderr bytes.Buffer
					cmd.Stderr = &stderr
					out, err := cmd.Output()
					if err != nil || string(out) != want || stderr.Len() != 0 {
						t.Fatalf("%s: %v, stdout=%q, stderr=%q", flag, err, out, stderr.String())
					}
				}
			})
		}
	}
}
