package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type installerFixture struct {
	root, dest, asset string
	env               []string
	binary, archive   []byte
}

func newInstallerFixture(t *testing.T, host, target string, headers ...*tar.Header) *installerFixture {
	t.Helper()
	f := &installerFixture{root: t.TempDir(), asset: "sei_0.1.0_" + target + ".tar.gz"}
	root, err := filepath.EvalSymlinks(f.root)
	if err != nil {
		t.Fatal(err)
	}
	f.root = root
	f.dest = filepath.Join(f.root, "user's bin", "nested")
	f.binary = []byte("#!/bin/sh\nprintf 'sei 0.1.0\\n'\n")
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if headers == nil {
		for _, name := range []string{"sei", "README.md", "LICENSE", "THIRD_PARTY_NOTICES"} {
			headers = append(headers, &tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0755})
		}
	}
	for _, h := range headers {
		var body []byte
		if h.Typeflag == tar.TypeReg {
			body = []byte("fixture documentation\n")
			if h.Name == "sei" {
				body = f.binary
			}
		}
		h.Size = int64(len(body))
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	f.archive = buf.Bytes()
	writeTestFile(t, filepath.Join(f.root, "archive"), string(f.archive))
	writeTestFile(t, filepath.Join(f.root, "manifest"), fmt.Sprintf("%x  %s\n", sha256.Sum256(f.archive), f.asset))
	tools := filepath.Join(f.root, "tools")
	if err := os.Mkdir(tools, 0700); err != nil {
		t.Fatal(err)
	}
	// Only harness-owned PATH mocks know these variables. The installer has no
	// fake endpoint, host, delay or failure switches and cannot reach live curl.
	mocks := map[string]string{
		"uname": `case "$1" in -s) printf '%s\n' "${MOCK_HOST%/*}" ;; -m) printf '%s\n' "${MOCK_HOST#*/}" ;; *) exit 2 ;; esac`,
		"curl": `
printf '%s\n' "$*" >> "$FIXTURE/requests"
out=
while [ "$#" -gt 0 ]; do
    case "$1" in --output) out=$2; shift 2 ;; *) url=$1; shift ;; esac
done
[ "${MOCK_FAIL:-}" != download ] || exit 22
if [ "${MOCK_FAIL:-}" = partial ]; then printf partial > "$out"; exit 22; fi
case "$url" in
    https://github.com/primaprashant/sei/releases/latest)
        [ ! -e "$FIXTURE/resolved" ] || exit 23
        : > "$FIXTURE/resolved"
        printf '%s' "${MOCK_LATEST:-https://github.com/primaprashant/sei/releases/tag/v0.1.0}" ;;
    "https://github.com/primaprashant/sei/releases/download/v0.1.0/$MOCK_ASSET") cp "$FIXTURE/archive" "$out" ;;
    https://github.com/primaprashant/sei/releases/download/v0.1.0/sei_0.1.0_checksums.txt) cp "$FIXTURE/manifest" "$out" ;;
    *) exit 24 ;;
esac`,
		"mv": `
case "$1" in
    */receipt) [ "${MOCK_FAIL:-}" != receipt ] || exit 1 ;;
    */sei)
        [ -f "${2%/*}/.sei-install-receipt" ] || exit 2
        [ "${MOCK_FAIL:-}" != binary ] || exit 1 ;;
esac
exec "$REAL_MV" "$@"`,
	}
	for name, body := range mocks {
		if err := os.WriteFile(filepath.Join(tools, name), []byte("#!/bin/sh\nset -eu\n"+body+"\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	mv, err := exec.LookPath("mv")
	if err != nil {
		t.Fatal(err)
	}
	f.env = []string{"PATH=" + tools + ":" + os.Getenv("PATH"), "HOME=" + f.root, "XDG_CONFIG_HOME=" + f.root,
		"FIXTURE=" + f.root, "MOCK_HOST=" + host, "MOCK_ASSET=" + f.asset, "REAL_MV=" + mv}
	return f
}

func (f *installerFixture) run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	script, err := filepath.Abs("scripts/install.sh")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", append([]string{script}, args...)...)
	cmd.Dir, cmd.Env = f.root, f.env
	out, err := cmd.CombinedOutput()
	data, readErr := os.ReadFile(filepath.Join(f.root, "archive"))
	if readErr != nil || !bytes.Equal(data, f.archive) {
		t.Fatal("source archive changed", readErr)
	}
	stages, globErr := filepath.Glob(filepath.Join(f.dest, ".sei-install.*"))
	if globErr != nil || len(stages) != 0 {
		t.Fatalf("stage not cleaned: %v, %v", stages, globErr)
	}
	return string(out), err
}

func TestInstallerFresh(t *testing.T) {
	for host, target := range map[string]string{"Linux/x86_64": "linux_amd64", "Linux/aarch64": "linux_arm64", "Darwin/x86_64": "darwin_amd64", "Darwin/arm64": "darwin_arm64"} {
		for _, version := range []string{"latest", "v0.1.0"} {
			t.Run(host+"/"+version, func(t *testing.T) {
				f := newInstallerFixture(t, host, target)
				out, err := f.run(t, "--version", version, "--install-dir", f.dest)
				if err != nil {
					t.Fatalf("%v: %s", err, out)
				}
				data, err := os.ReadFile(filepath.Join(f.dest, "sei"))
				if err != nil || !bytes.Equal(data, f.binary) {
					t.Fatalf("installed bytes: %q, %v", data, err)
				}
				receipt, err := os.ReadFile(filepath.Join(f.dest, ".sei-install-receipt"))
				want := fmt.Sprintf("sei-install-receipt-v1\nv0.1.0 %x\n", sha256.Sum256(f.binary))
				if err != nil || string(receipt) != want {
					t.Fatalf("receipt: %q, %v", receipt, err)
				}
				requests, err := os.ReadFile(filepath.Join(f.root, "requests"))
				if err != nil {
					t.Fatal(err)
				}
				count := 2
				if version == "latest" {
					count++
				}
				if strings.Count(string(requests), "\n") != count || !strings.Contains(string(requests), "/download/v0.1.0/"+f.asset) || !strings.Contains(string(requests), "/download/v0.1.0/sei_0.1.0_checksums.txt") {
					t.Fatalf("wrong requests: %s", requests)
				}
				// Execute the actual quoted follow-up, including the apostrophe.
				_, follow, ok := strings.Cut(out, "\nRun:\n")
				if !ok || !strings.Contains(out, f.dest+"/sei") {
					t.Fatalf("missing absolute guidance: %s", out)
				}
				cmd := exec.Command("sh", "-c", follow)
				cmd.Env = f.env
				if got, err := cmd.CombinedOutput(); err != nil || string(got) != "sei 0.1.0\n" {
					t.Fatalf("follow-up: %s, %v", got, err)
				}
				if out, err := f.run(t, "--install-dir", f.dest); err == nil || !strings.Contains(out, "existing sei or receipt") {
					t.Fatalf("reinstall not refused: %v %s", err, out)
				}
			})
		}
	}
	t.Run("default", func(t *testing.T) {
		f := newInstallerFixture(t, "Linux/x86_64", "linux_amd64")
		f.dest = filepath.Join(f.root, ".local", "bin")
		if out, err := f.run(t); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	})
	t.Run("relative", func(t *testing.T) {
		f := newInstallerFixture(t, "Linux/x86_64", "linux_amd64")
		out, err := f.run(t, "--install-dir", "user's bin/nested")
		if err != nil || !strings.Contains(out, f.dest+"/sei") {
			t.Fatalf("relative directory: %v: %s", err, out)
		}
	})
}

func TestInstallerPATH(t *testing.T) {
	for _, position := range []string{"absent", "near-match", "first", "middle", "last"} {
		t.Run(position, func(t *testing.T) {
			f := newInstallerFixture(t, "Linux/x86_64", "linux_amd64")
			path := strings.TrimPrefix(f.env[0], "PATH=")
			present := true
			switch position {
			case "absent":
				present = false
			case "near-match":
				path = f.dest + "-other:" + path + ":" + f.dest + "/child"
				present = false
			case "first":
				path = f.dest + ":" + path
			case "middle":
				path = path + ":" + f.dest + ":/unused"
			case "last":
				path += ":" + f.dest
			}
			f.env[0] = "PATH=" + path
			out, err := f.run(t, "--install-dir", f.dest)
			if err != nil {
				t.Fatalf("%v: %s", err, out)
			}
			var export string
			for _, line := range strings.Split(out, "\n") {
				if strings.HasPrefix(line, "export PATH=") {
					export += line
				}
			}
			if present {
				if export != "" || strings.Contains(out, "To add sei to PATH") {
					t.Fatalf("unnecessary PATH guidance: %s", out)
				}
				return
			}
			want := "export PATH='" + strings.ReplaceAll(f.dest, "'", "'\\''") + "':\"$PATH\""
			if export != want {
				t.Fatalf("export = %q, want %q", export, want)
			}
			// A different current-shell PATH proves the printed $PATH is literal,
			// not the installer's expanded value. Empty PATH must not become a
			// leading empty component ahead of the installed directory.
			for _, shellPath := range []string{"/usr/bin:/bin", ""} {
				cmd := exec.Command("sh", "-c", export+"\ncommand -v sei\nsei --version\nprintf '%s\\n' \"$PATH\"")
				cmd.Env = append([]string(nil), f.env...)
				cmd.Env[0] = "PATH=" + shellPath
				got, err := cmd.CombinedOutput()
				want := f.dest + "/sei\nsei 0.1.0\n" + f.dest + ":" + shellPath + "\n"
				if err != nil || string(got) != want {
					t.Fatalf("PATH follow-up: %q, %v; want %q", got, err, want)
				}
			}
		})
	}
}

func TestInstallerInvalid(t *testing.T) {
	for _, args := range [][]string{{"--bad"}, {"--version"}, {"--install-dir"}, {"--install-dir", ""}, {"--install-dir", "trailing\n"}, {"--version", "v0.1.0", "--version", "v0.1.0"}, {"--version", ""}, {"--version", "0.1.0"}, {"--version", "v01.1.0"}, {"--version", "v0.1.0/../../evil"}, {"--version", "v0.1.0?x=y"}, {"--version", "v0.1.0\nv1.0.0"}, {"--version", "v0.1.0-01"}, {"--version", "v0.1.0+metadata"}} {
		t.Run(fmt.Sprint(args), func(t *testing.T) {
			f := newInstallerFixture(t, "Linux/x86_64", "linux_amd64")
			if out, err := f.run(t, args...); err == nil {
				t.Fatalf("accepted invalid input: %s", out)
			}
			if _, err := os.Lstat(filepath.Join(f.root, "requests")); !os.IsNotExist(err) {
				t.Fatal("invalid arguments reached download")
			}
		})
	}
	for _, host := range []string{"FreeBSD/x86_64", "Linux/arm64", "Linux/amd64", "Darwin/aarch64", "Linux/i686"} {
		t.Run(host, func(t *testing.T) {
			f := newInstallerFixture(t, host, "linux_amd64")
			if out, err := f.run(t, "--install-dir", f.dest); err == nil || !strings.Contains(out, "unsupported host") {
				t.Fatalf("%v: %s", err, out)
			}
		})
	}
}

func TestInstallerSafety(t *testing.T) {
	for _, name := range []string{"sei", ".sei-install-receipt"} {
		for _, kind := range []string{"file", "directory", "symlink", "dangling"} {
			t.Run(name+"/"+kind, func(t *testing.T) {
				f := newInstallerFixture(t, "Linux/x86_64", "linux_amd64")
				if err := os.MkdirAll(f.dest, 0700); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(f.dest, name)
				sentinel := "#!/bin/sh\n: > '" + filepath.Join(f.root, "executed") + "'\n"
				switch kind {
				case "file":
					if err := os.WriteFile(path, []byte(sentinel), 0755); err != nil {
						t.Fatal(err)
					}
				case "directory":
					if err := os.Mkdir(path, 0700); err != nil {
						t.Fatal(err)
					}
				default:
					target := filepath.Join(f.root, "outside")
					if kind == "symlink" {
						writeTestFile(t, target, sentinel)
					}
					if err := os.Symlink(target, path); err != nil {
						t.Fatal(err)
					}
				}
				before, err := os.Lstat(path)
				if err != nil {
					t.Fatal(err)
				}
				if out, err := f.run(t, "--install-dir", f.dest); err == nil || !strings.Contains(out, "existing sei or receipt") {
					t.Fatalf("%v: %s", err, out)
				}
				after, err := os.Lstat(path)
				if err != nil || !os.SameFile(before, after) {
					t.Fatal("existing path replaced", err)
				}
				if kind == "file" || kind == "symlink" {
					got, err := os.ReadFile(path)
					if err != nil || string(got) != sentinel {
						t.Fatal("existing bytes changed", err)
					}
				}
				for _, absent := range []string{"executed", "requests"} {
					if _, err := os.Lstat(filepath.Join(f.root, absent)); !os.IsNotExist(err) {
						t.Fatalf("unexpected %s", absent)
					}
				}
			})
		}
	}
}

func TestInstallerBrokenInputs(t *testing.T) {
	for _, problem := range []string{"missing-sum", "duplicate-sum", "bad-sum", "mismatch", "download", "partial", "receipt", "binary", "latest-url", "latest-tag", "candidate-version", "corrupt-gzip", "appended-archive"} {
		t.Run(problem, func(t *testing.T) {
			f := newInstallerFixture(t, "Linux/x86_64", "linux_amd64")
			sum := fmt.Sprintf("%x  %s\n", sha256.Sum256(f.archive), f.asset)
			switch problem {
			case "missing-sum":
				sum = strings.ReplaceAll(sum, f.asset, "other.tar.gz")
			case "duplicate-sum":
				sum += sum
			case "bad-sum":
				sum = "z" + sum[1:]
			case "mismatch":
				sum = strings.Repeat("0", 64) + "  " + f.asset + "\n"
			case "latest-url":
				f.env = append(f.env, "MOCK_LATEST=https://evil.invalid/tag/v0.1.0")
			case "latest-tag":
				f.env = append(f.env, "MOCK_LATEST=https://github.com/primaprashant/sei/releases/tag/v0.1.0/evil")
			case "candidate-version":
				if err := os.WriteFile(filepath.Join(f.root, "tools", "tar"), []byte("#!/bin/sh\ncase $1 in -xOzf) printf '#!/bin/sh\\nprintf wrong\\\\n\\n' ;; *) exec \"$REAL_TAR\" \"$@\" ;; esac\n"), 0755); err != nil {
					t.Fatal(err)
				}
				realTar, err := exec.LookPath("tar")
				if err != nil {
					t.Fatal(err)
				}
				f.env = append(f.env, "REAL_TAR="+realTar)
			case "corrupt-gzip":
				f.archive = []byte("not gzip")
				writeTestFile(t, filepath.Join(f.root, "archive"), string(f.archive))
				sum = fmt.Sprintf("%x  %s\n", sha256.Sum256(f.archive), f.asset)
			case "appended-archive":
				f.archive = append(bytes.Clone(f.archive), f.archive...)
				writeTestFile(t, filepath.Join(f.root, "archive"), string(f.archive))
				sum = fmt.Sprintf("%x  %s\n", sha256.Sum256(f.archive), f.asset)
			default:
				f.env = append(f.env, "MOCK_FAIL="+problem)
			}
			writeTestFile(t, filepath.Join(f.root, "manifest"), sum)
			out, err := f.run(t, "--install-dir", f.dest)
			if err == nil {
				t.Fatalf("accepted %s: %s", problem, out)
			}
			if problem == "appended-archive" && !strings.Contains(out, "unexpected, missing or duplicate archive member") {
				t.Fatalf("appended members were not inspected: %s", out)
			}
			if _, err := os.Lstat(filepath.Join(f.dest, "sei")); !os.IsNotExist(err) {
				t.Fatal("binary placed on failure", err)
			}
			receipt, err := os.ReadFile(filepath.Join(f.dest, ".sei-install-receipt"))
			if problem == "binary" {
				if err != nil || string(receipt) != fmt.Sprintf("sei-install-receipt-v1\nv0.1.0 %x\n", sha256.Sum256(f.binary)) {
					t.Fatalf("prepared receipt not retained: %q, %v", receipt, err)
				}
			} else if !os.IsNotExist(err) {
				t.Fatal("receipt placed before validation", err)
			}
		})
	}
}

func TestInstallerUtilities(t *testing.T) {
	for _, missingCurl := range []bool{false, true} {
		t.Run(fmt.Sprintf("missing-curl=%t", missingCurl), func(t *testing.T) {
			f := newInstallerFixture(t, "Linux/x86_64", "linux_amd64")
			tools := filepath.Join(f.root, "tools")
			// Deliberately omit sha256sum to exercise the native macOS fallback
			// even on Linux. Keep only explicit installer/mock utility dependencies.
			for _, name := range []string{"awk", "cat", "chmod", "cp", "gzip", "mkdir", "mktemp", "rm", "sed", "shasum", "tar"} {
				path, err := exec.LookPath(name)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path, filepath.Join(tools, name)); err != nil {
					t.Fatal(err)
				}
			}
			f.env[0] = "PATH=" + tools
			if missingCurl {
				if err := os.Remove(filepath.Join(tools, "curl")); err != nil {
					t.Fatal(err)
				}
			}
			out, err := f.run(t, "--install-dir", f.dest)
			if missingCurl {
				if err == nil || !strings.Contains(out, "required utility missing: curl") {
					t.Fatalf("%v: %s", err, out)
				}
			} else if err != nil {
				t.Fatalf("shasum fallback: %v: %s", err, out)
			}
		})
	}
}

func TestInstallerArchiveSafety(t *testing.T) {
	for _, problem := range []string{"extra", "missing", "duplicate", "traversal", "absolute", "dot", "newline", "directory", "symlink", "hardlink", "fifo", "device"} {
		t.Run(problem, func(t *testing.T) {
			headers := []*tar.Header{}
			for _, name := range []string{"sei", "README.md", "LICENSE", "THIRD_PARTY_NOTICES"} {
				headers = append(headers, &tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0755})
			}
			switch problem {
			case "extra":
				headers = append(headers, &tar.Header{Name: "extra", Typeflag: tar.TypeReg})
			case "missing":
				headers = headers[:3]
			case "duplicate":
				headers[3].Name = "sei"
			case "traversal":
				headers[0].Name = "../sei"
			case "absolute":
				headers[0].Name = "/sei"
			case "dot":
				headers[0].Name = "./sei"
			case "newline":
				headers[0].Name = "sei\nLICENSE"
			default:
				headers[0].Typeflag = map[string]byte{"directory": tar.TypeDir, "symlink": tar.TypeSymlink, "hardlink": tar.TypeLink, "fifo": tar.TypeFifo, "device": tar.TypeChar}[problem]
				headers[0].Linkname = "README.md"
			}
			f := newInstallerFixture(t, "Linux/x86_64", "linux_amd64", headers...)
			if out, err := f.run(t, "--install-dir", f.dest); err == nil {
				t.Fatalf("accepted unsafe archive: %s", out)
			}
			entries, err := os.ReadDir(f.dest)
			if err != nil || len(entries) != 0 {
				t.Fatalf("archive left output: %v, %v", entries, err)
			}
		})
	}
}
