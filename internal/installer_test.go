package app

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
	"syscall"
	"testing"
	"time"
)

type installerFixture struct {
	root, dest, asset string
	env               []string
	binary, archive   []byte
	shell             string
	start             func(*exec.Cmd)
	pipes             []*os.File
}

func TestInstaller(t *testing.T) {
	for _, shell := range []string{"sh", "bash", "dash"} {
		t.Run(shell, func(t *testing.T) {
			if _, err := exec.LookPath(shell); err != nil {
				t.Skipf("shell unavailable: %v", err)
			}
			for name, test := range map[string]func(*testing.T, string){
				"Fresh": testInstallerFresh, "PATH": testInstallerPATH,
				"Invalid": testInstallerInvalid, "Safety": testInstallerSafety,
				"Upgrade": testInstallerUpgrade, "BrokenInputs": testInstallerBrokenInputs,
				"Utilities": testInstallerUtilities, "ArchiveSafety": testInstallerArchiveSafety,
				"Interruption": testInstallerInterruption, "Directory": testInstallerDirectory,
			} {
				t.Run(name, func(t *testing.T) { test(t, shell) })
			}
		})
	}
}

func newInstallerFixture(t *testing.T, shell, host, target string, headers ...*tar.Header) *installerFixture {
	t.Helper()
	f := &installerFixture{root: t.TempDir(), asset: "sei_0.1.0_" + target + ".tar.gz"}
	f.shell = shell
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
        [ "${MOCK_FAIL:-}" != latest-unavailable ] || exit 22
        if [ -e "$FIXTURE/resolved" ]; then printf '%s' https://github.com/primaprashant/sei/releases/tag/v9.9.9; exit 0; fi
        : > "$FIXTURE/resolved"
        printf '%s' "${MOCK_LATEST:-https://github.com/primaprashant/sei/releases/tag/v0.1.0}" ;;
    "https://github.com/primaprashant/sei/releases/download/v0.1.0/$MOCK_ASSET") cp "$FIXTURE/archive" "$out" ;;
    https://github.com/primaprashant/sei/releases/download/v0.1.0/sei_0.1.0_checksums.txt)
        [ "${MOCK_FAIL:-}" != manifest-download ] || exit 22
        if [ "${MOCK_FAIL:-}" = partial-manifest ]; then printf partial > "$out"; exit 22; fi
        cp "$FIXTURE/manifest" "$out" ;;
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
	script := repoPath(t, "scripts/install.sh")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	shell := f.shell
	argv := []string{script}
	if shell == "bash" {
		argv = append([]string{"--posix"}, argv...)
	}
	cmd := exec.CommandContext(ctx, shell, append(argv, args...)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = time.Second
	cmd.Dir, cmd.Env = f.root, f.env
	cmd.ExtraFiles = f.pipes
	beforeStages, err := filepath.Glob(filepath.Join(f.dest, ".sei-install.*"))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	err = cmd.Start()
	if err == nil {
		defer func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }()
		if f.start != nil {
			f.start(cmd)
		}
		err = cmd.Wait()
	}
	out := output.Bytes()
	data, readErr := os.ReadFile(filepath.Join(f.root, "archive"))
	if readErr != nil || !bytes.Equal(data, f.archive) {
		t.Fatal("source archive changed", readErr)
	}
	stages, globErr := filepath.Glob(filepath.Join(f.dest, ".sei-install.*"))
	if globErr != nil || strings.Join(stages, "\n") != strings.Join(beforeStages, "\n") {
		t.Fatalf("stage not cleaned: %v, %v", stages, globErr)
	}
	return string(out), err
}

func testInstallerFresh(t *testing.T, shell string) {
	for _, format := range []tar.Format{tar.FormatPAX, tar.FormatGNU} {
		t.Run("metadata/"+format.String(), func(t *testing.T) {
			var headers []*tar.Header
			for _, name := range []string{"sei", "README.md", "LICENSE", "THIRD_PARTY_NOTICES"} {
				h := &tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0755, Format: format}
				if format == tar.FormatPAX {
					h.PAXRecords = map[string]string{"comment": "metadata is not an extracted file", "SCHILY.xattr.user.fixture": "ignored"}
				}
				headers = append(headers, h)
			}
			f := newInstallerFixture(t, shell, "Linux/x86_64", "linux_amd64", headers...)
			f.env = append(f.env, "TAR_OPTIONS=--invalid-option", "GZIP=--invalid-option")
			// Only the exact selected asset matters; other release assets cannot
			// substitute for it, and documentation metadata is never extracted.
			sum := fmt.Sprintf("%x  %s\n", sha256.Sum256(f.archive), f.asset)
			writeTestFile(t, filepath.Join(f.root, "manifest"), sum+strings.Repeat("0", 64)+"  "+f.asset+".other\n")
			if out, err := f.run(t, "--install-dir", f.dest); err != nil {
				t.Fatalf("metadata: %v: %s", err, out)
			}
			if got, err := os.ReadFile(filepath.Join(f.dest, "sei")); err != nil || !bytes.Equal(got, f.binary) {
				t.Fatalf("metadata candidate: %q, %v", got, err)
			}
		})
	}
	for host, target := range map[string]string{"Linux/x86_64": "linux_amd64", "Linux/amd64": "linux_amd64", "Linux/aarch64": "linux_arm64", "Linux/arm64": "linux_arm64", "Darwin/x86_64": "darwin_amd64", "Darwin/amd64": "darwin_amd64", "Darwin/arm64": "darwin_arm64", "Darwin/aarch64": "darwin_arm64"} {
		for _, version := range []string{"latest", "v0.1.0"} {
			t.Run(host+"/"+version, func(t *testing.T) {
				f := newInstallerFixture(t, shell, host, target)
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
				if out, err := f.run(t, "--version", "v0.1.0", "--install-dir", f.dest); err != nil {
					t.Fatalf("reinstall failed: %v %s", err, out)
				}
			})
		}
	}
	t.Run("default", func(t *testing.T) {
		f := newInstallerFixture(t, shell, "Linux/x86_64", "linux_amd64")
		f.dest = filepath.Join(f.root, ".local", "bin")
		if out, err := f.run(t); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	})
	t.Run("relative", func(t *testing.T) {
		f := newInstallerFixture(t, shell, "Linux/x86_64", "linux_amd64")
		out, err := f.run(t, "--install-dir", "user's bin/nested")
		if err != nil || !strings.Contains(out, f.dest+"/sei") {
			t.Fatalf("relative directory: %v: %s", err, out)
		}
	})
}

func testInstallerPATH(t *testing.T, shell string) {
	for _, fragments := range []bool{false, true} {
		t.Run(fmt.Sprintf("colon/fragments=%t", fragments), func(t *testing.T) {
			f := newInstallerFixture(t, shell, "Linux/x86_64", "linux_amd64")
			f.dest = filepath.Join(f.root, "user's bin:quoted \"directory\"")
			if fragments {
				// These are two PATH components, never the colon-containing directory.
				f.env[0] += ":" + f.dest
			}
			out, err := f.run(t, "--install-dir", f.dest)
			if err != nil {
				t.Fatalf("%v: %s", err, out)
			}
			if strings.Contains(out, "export PATH=") || strings.Contains(out, "To add sei to PATH") ||
				!strings.Contains(out, "Cannot add a directory containing : to PATH; use the absolute invocation below.") {
				t.Fatalf("misleading colon PATH guidance: %s", out)
			}
			_, follow, ok := strings.Cut(out, "\nRun:\n")
			want := "'" + strings.ReplaceAll(f.dest, "'", "'\\''") + "/sei'\n"
			if !ok || follow != want {
				t.Fatalf("absolute invocation = %q, want %q", follow, want)
			}
			args := []string{"-c", follow}
			if f.shell == "bash" {
				args = append([]string{"--posix"}, args...)
			}
			cmd := exec.Command(f.shell, args...)
			cmd.Env = f.env
			if got, err := cmd.CombinedOutput(); err != nil || string(got) != "sei 0.1.0\n" {
				t.Fatalf("colon absolute invocation: %q, %v", got, err)
			}
		})
	}
	for _, position := range []string{"absent", "near-match", "first", "middle", "last"} {
		t.Run(position, func(t *testing.T) {
			f := newInstallerFixture(t, shell, "Linux/x86_64", "linux_amd64")
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

func testInstallerInvalid(t *testing.T, shell string) {
	for _, args := range [][]string{{"--bad"}, {"--force"}, {"--no-verify"}, {"--url", "https://evil.invalid"}, {"--version"}, {"--install-dir"}, {"--install-dir", ""}, {"--install-dir", "trailing\n"}, {"--install-dir", "one", "--install-dir", "two"}, {"--version", "v0.1.0", "--version", "v0.1.0"}, {"--version", ""}, {"--version", "0.1.0"}, {"--version", "v01.1.0"}, {"--version", "v0.1.0/../../evil"}, {"--version", "v0.1.0?x=y"}, {"--version", "v0.1.0\nv1.0.0"}, {"--version", "v0.1.0\n"}, {"--version", "v0.1.0\r"}, {"--version", "v0.1.0-01"}, {"--version", "v0.1.0+metadata"}} {
		t.Run(fmt.Sprint(args), func(t *testing.T) {
			f := newInstallerFixture(t, shell, "Linux/x86_64", "linux_amd64")
			if out, err := f.run(t, args...); err == nil {
				t.Fatalf("accepted invalid input: %s", out)
			}
			if _, err := os.Lstat(filepath.Join(f.root, "requests")); !os.IsNotExist(err) {
				t.Fatal("invalid arguments reached download")
			}
		})
	}
	for _, host := range []string{"FreeBSD/x86_64", "Linux/armv7l", "Windows/amd64", "Darwin/i386", "Linux/i686"} {
		t.Run(host, func(t *testing.T) {
			f := newInstallerFixture(t, shell, host, "linux_amd64")
			if out, err := f.run(t, "--install-dir", f.dest); err == nil || !strings.Contains(out, "unsupported host") {
				t.Fatalf("%v: %s", err, out)
			}
		})
	}
}

func testInstallerSafety(t *testing.T, shell string) {
	for _, name := range []string{"sei", ".sei-install-receipt"} {
		for _, kind := range []string{"file", "directory", "symlink", "dangling"} {
			t.Run(name+"/"+kind, func(t *testing.T) {
				f := newInstallerFixture(t, shell, "Linux/x86_64", "linux_amd64")
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
				if out, err := f.run(t, "--install-dir", f.dest); err == nil || !strings.Contains(out, "unrecognized sei or receipt") {
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

func testInstallerUpgrade(t *testing.T, shell string) {
	for _, point := range []string{"before-receipt", "before-binary"} {
		for _, changed := range []string{"sei", ".sei-install-receipt"} {
			for _, mutationKind := range []string{"symlink", "bytes"} {
				t.Run("recheck/"+point+"/"+changed+"/"+mutationKind, func(t *testing.T) {
					f := newInstallerFixture(t, shell, "Linux/x86_64", "linux_amd64")
					if out, err := f.run(t, "--version", "v0.1.0", "--install-dir", f.dest); err != nil {
						t.Fatalf("setup: %v: %s", err, out)
					}
					tool := "chmod"
					body := "exec \"$REAL_TOOL\" \"$@\""
					if point == "before-binary" {
						tool = "mv"
						body = "\"$REAL_TOOL\" \"$@\"\n"
					}
					realTool, err := exec.LookPath(tool)
					if err != nil {
						t.Fatal(err)
					}
					// Simulate an observable external path replacement, not a hostile
					// writer inside the final check/rename window (which is not promised).
					mutation := "\"$REAL_MV\" \"$DEST/" + changed + "\" \"$FIXTURE/old\"\nln -s \"$FIXTURE/old\" \"$DEST/" + changed + "\"\n"
					if mutationKind == "bytes" {
						mutation = "printf changed >> \"$DEST/" + changed + "\"\n"
					}
					if point == "before-binary" {
						body += mutation
					} else {
						body = mutation + body
					}
					f.env = append(f.env, "REAL_TOOL="+realTool, "DEST="+f.dest)
					if err := os.WriteFile(filepath.Join(f.root, "tools", tool), []byte("#!/bin/sh\nset -eu\n"+body), 0755); err != nil {
						t.Fatal(err)
					}
					out, err := f.run(t, "--version", "v0.1.0", "--install-dir", f.dest)
					if err == nil || !strings.Contains(out, "install paths changed") {
						t.Fatalf("missed path change: %v: %s", err, out)
					}
					if strings.Contains(out, "prepared receipt retained") || (point == "before-binary" && !strings.Contains(out, "inspect paths")) {
						t.Fatalf("untruthful path-change diagnostic: %s", out)
					}
					if target, err := os.Readlink(filepath.Join(f.dest, changed)); mutationKind == "symlink" && (err != nil || target != filepath.Join(f.root, "old")) {
						t.Fatalf("observable symlink clobbered: %q, %v", target, err)
					}
					want := f.binary
					if changed == ".sei-install-receipt" {
						want = []byte(fmt.Sprintf("sei-install-receipt-v1\nv0.1.0 %x\n", sha256.Sum256(f.binary)))
					}
					path := filepath.Join(f.root, "old")
					if mutationKind == "bytes" {
						path = filepath.Join(f.dest, changed)
						want = append(bytes.Clone(want), []byte("changed")...)
					}
					if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, want) {
						t.Fatalf("old bytes changed: %q, %v", got, err)
					}
					other, otherWant := "sei", f.binary
					if changed == "sei" {
						other = ".sei-install-receipt"
						otherWant = []byte(fmt.Sprintf("sei-install-receipt-v1\nv0.1.0 %x\n", sha256.Sum256(f.binary)))
					}
					if got, err := os.ReadFile(filepath.Join(f.dest, other)); err != nil || !bytes.Equal(got, otherWant) {
						t.Fatalf("unchanged path clobbered: %q, %v", got, err)
					}
				})
			}
		}
	}
	for _, problem := range []string{"success", "download", "partial", "stage-write", "permissions", "candidate-write", "candidate-version", "candidate-exit", "candidate-empty", "candidate-newlines", "candidate-no-newline", "candidate-stderr", "checksum", "receipt-write", "receipt", "binary", "guidance", "postcommit"} {
		t.Run(problem, func(t *testing.T) {
			f := newInstallerFixture(t, shell, "Linux/x86_64", "linux_amd64")
			if err := os.MkdirAll(f.dest, 0700); err != nil {
				t.Fatal(err)
			}
			binaryPath := filepath.Join(f.dest, "sei")
			receiptPath := filepath.Join(f.dest, ".sei-install-receipt")
			marker := filepath.Join(f.root, "executed")
			old := []byte("#!/bin/sh\n: > '" + marker + "'\nprintf 'sei 0.0.9\\n'\n")
			if err := os.WriteFile(binaryPath, old, 0755); err != nil {
				t.Fatal(err)
			}
			oldReceipt := fmt.Sprintf("sei-install-receipt-v1\nv0.0.9 %x\n", sha256.Sum256(old))
			writeTestFile(t, receiptPath, oldReceipt)
			prepared := oldReceipt + fmt.Sprintf("v0.1.0 %x\n", sha256.Sum256(f.binary))
			cleanEnv := append([]string(nil), f.env...)
			tool, body := "", ""
			switch problem {
			case "postcommit":
				tool, body = "rm", "\"$REAL_TOOL\" \"$@\"\nexit 1"
			case "guidance":
				tool, body = "sed", "exit 1"
			case "stage-write":
				tool, body = "mktemp", "exit 1"
			case "permissions":
				tool, body = "chmod", "exit 1"
			case "receipt-write":
				tool, body = "chmod", "mkdir \"${2%/*}/receipt\"\nexec \"$REAL_TOOL\" \"$@\""
			case "candidate-write":
				tool, body = "tar", "case $1 in -xOzf) printf partial; exit 1 ;; *) exec \"$REAL_TOOL\" \"$@\" ;; esac"
			case "candidate-version", "candidate-exit", "candidate-empty", "candidate-newlines", "candidate-no-newline", "candidate-stderr":
				candidate := "#!/bin/sh\nprintf wrong"
				switch problem {
				case "candidate-exit":
					candidate = "#!/bin/sh\nexit 1"
				case "candidate-empty":
					candidate = ""
				case "candidate-newlines":
					candidate = "#!/bin/sh\nprintf \"sei 0.1.0\\n\\n\""
				case "candidate-no-newline":
					candidate = "#!/bin/sh\nprintf \"sei 0.1.0\""
				case "candidate-stderr":
					candidate = "#!/bin/sh\nprintf \"sei 0.1.0\\n\"\nprintf warning >&2"
				}
				tool, body = "tar", "case $1 in -xOzf) printf '%s' '"+candidate+"' ;; *) exec \"$REAL_TOOL\" \"$@\" ;; esac"
			case "checksum":
				writeTestFile(t, filepath.Join(f.root, "manifest"), strings.Repeat("0", 64)+"  "+f.asset+"\n")
			default:
				f.env = append(f.env, "MOCK_FAIL="+problem)
			}
			if tool != "" {
				realTool, err := exec.LookPath(tool)
				if err != nil {
					t.Fatal(err)
				}
				f.env = append(f.env, "REAL_TOOL="+realTool)
				if err := os.WriteFile(filepath.Join(f.root, "tools", tool), []byte("#!/bin/sh\nset -eu\n"+body+"\n"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			out, err := f.run(t, "--version", "v0.1.0", "--install-dir", f.dest)
			if (err == nil) != (problem == "success") {
				t.Fatalf("unexpected result: %v: %s", err, out)
			}
			if _, err := os.Lstat(marker); !os.IsNotExist(err) {
				t.Fatal("old executable used for identification", err)
			}
			wantBinary, wantReceipt := old, oldReceipt
			if problem == "success" || problem == "postcommit" {
				wantBinary = f.binary
			}
			if problem == "success" || problem == "binary" || problem == "postcommit" {
				wantReceipt = prepared
			}
			got, err := os.ReadFile(binaryPath)
			if err != nil || !bytes.Equal(got, wantBinary) {
				t.Fatalf("binary changed: %q, %v", got, err)
			}
			got, err = os.ReadFile(receiptPath)
			if err != nil || string(got) != wantReceipt {
				t.Fatalf("receipt: %q, %v; want %q", got, err, wantReceipt)
			}
			if problem == "success" || problem == "postcommit" {
				if !strings.Contains(out, "Installed sei 0.1.0") || strings.Contains(out, "preserved") {
					t.Fatalf("untruthful committed diagnostic: %s", out)
				}
				if got, err := exec.Command(binaryPath, "--version").CombinedOutput(); err != nil || string(got) != "sei 0.1.0\n" {
					t.Fatalf("new binary unusable: %q, %v", got, err)
				}
				for path, mode := range map[string]os.FileMode{binaryPath: 0755, receiptPath: 0600} {
					info, err := os.Stat(path)
					if err != nil || info.Mode().Perm() != mode {
						t.Fatalf("incorrect installed permissions: %s, %v", path, err)
					}
				}
				if problem == "postcommit" {
					return
				}
				if out, err := f.run(t, "--version", "v0.1.0", "--install-dir", f.dest); err != nil {
					t.Fatalf("two-record receipt not recognized: %v: %s", err, out)
				}
				got, err := os.ReadFile(receiptPath)
				if err != nil || string(got) != fmt.Sprintf("sei-install-receipt-v1\nv0.1.0 %x\n", sha256.Sum256(f.binary)) {
					t.Fatalf("same-version receipt not deduplicated: %q, %v", got, err)
				}
				return
			}
			if got, err := exec.Command(binaryPath, "--version").CombinedOutput(); err != nil || string(got) != "sei 0.0.9\n" {
				t.Fatalf("old binary unusable: %q, %v", got, err)
			}
			if err := os.Remove(marker); err != nil {
				t.Fatal(err)
			}
			if tool != "" {
				if err := os.Remove(filepath.Join(f.root, "tools", tool)); err != nil {
					t.Fatal(err)
				}
			}
			f.env = cleanEnv
			writeTestFile(t, filepath.Join(f.root, "manifest"), fmt.Sprintf("%x  %s\n", sha256.Sum256(f.archive), f.asset))
			if out, err := f.run(t, "--version", "v0.1.0", "--install-dir", f.dest); err != nil {
				t.Fatalf("retry through retained receipt: %v: %s", err, out)
			}
			got, err = os.ReadFile(binaryPath)
			if err != nil || !bytes.Equal(got, f.binary) {
				t.Fatal("retry did not install candidate", err)
			}
			if _, err := os.Lstat(marker); !os.IsNotExist(err) {
				t.Fatal("retry executed old binary", err)
			}
		})
	}
	for _, problem := range []string{"missing", "mismatch", "header", "version", "prerelease", "nul", "digest", "extra-field", "tab", "blank", "duplicate", "ambiguous", "duplicate-version", "no-newline", "receipt-symlink", "binary-symlink", "receipt-directory", "binary-directory", "receipt-fifo", "binary-fifo"} {
		t.Run("refuse/"+problem, func(t *testing.T) {
			f := newInstallerFixture(t, shell, "Linux/x86_64", "linux_amd64")
			if err := os.MkdirAll(f.dest, 0700); err != nil {
				t.Fatal(err)
			}
			binaryPath, receiptPath := filepath.Join(f.dest, "sei"), filepath.Join(f.dest, ".sei-install-receipt")
			old := "#!/bin/sh\n: > '" + filepath.Join(f.root, "executed") + "'\n"
			if err := os.WriteFile(binaryPath, []byte(old), 0755); err != nil {
				t.Fatal(err)
			}
			record := fmt.Sprintf("v0.0.9 %x\n", sha256.Sum256([]byte(old)))
			receipt := "sei-install-receipt-v1\n" + record
			switch problem {
			case "mismatch":
				receipt = "sei-install-receipt-v1\nv0.0.9 " + strings.Repeat("0", 64) + "\n"
			case "header":
				receipt = strings.Replace(receipt, "v1", "v2", 1)
			case "version":
				receipt = strings.Replace(receipt, "v0.0.9", "v00.0.9", 1)
			case "prerelease":
				receipt = strings.Replace(receipt, "v0.0.9", "v0.0.9-01", 1)
			case "nul":
				receipt = strings.Replace(receipt, "v0.0.9", "v0.0.9\x00", 1)
			case "digest":
				receipt = strings.Replace(receipt, " ", " z", 1)
			case "extra-field":
				receipt = strings.TrimSuffix(receipt, "\n") + " extra\n"
			case "tab":
				receipt = strings.Replace(receipt, " ", "\t", 1)
			case "blank":
				receipt += "\n"
			case "duplicate":
				receipt += record
			case "ambiguous":
				receipt += strings.Replace(record, "v0.0.9", "v0.0.8", 1)
			case "duplicate-version":
				receipt += "v0.0.9 " + strings.Repeat("0", 64) + "\n"
			case "no-newline":
				receipt = strings.TrimSuffix(receipt, "\n")
			}
			if problem != "missing" {
				writeTestFile(t, receiptPath, receipt)
			}
			if prefix, kind, ok := strings.Cut(problem, "-"); ok && (prefix == "binary" || prefix == "receipt") {
				path := receiptPath
				if prefix == "binary" {
					path = binaryPath
				}
				outside := filepath.Join(f.root, "outside")
				if err := os.Rename(path, outside); err != nil {
					t.Fatal(err)
				}
				var err error
				switch kind {
				case "symlink":
					err = os.Symlink(outside, path)
				case "directory":
					err = os.Mkdir(path, 0700)
				case "fifo":
					err = exec.Command("mkfifo", path).Run()
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			beforeBinary, _ := os.Lstat(binaryPath)
			beforeReceipt, _ := os.Lstat(receiptPath)
			if out, err := f.run(t, "--version", "v0.1.0", "--install-dir", f.dest); err == nil || !strings.Contains(out, "inspect and relocate") {
				t.Fatalf("unsafe ownership accepted: %v: %s", err, out)
			}
			for path, before := range map[string]os.FileInfo{binaryPath: beforeBinary, receiptPath: beforeReceipt} {
				after, err := os.Lstat(path)
				if before == nil {
					if !os.IsNotExist(err) {
						t.Fatal("absent path created", err)
					}
				} else if err != nil || !os.SameFile(before, after) {
					t.Fatal("refused path changed", err)
				}
			}
			for _, name := range []string{"requests", "executed"} {
				if _, err := os.Lstat(filepath.Join(f.root, name)); !os.IsNotExist(err) {
					t.Fatalf("unexpected %s", name)
				}
			}
		})
	}
}

func testInstallerBrokenInputs(t *testing.T, shell string) {
	for _, problem := range []string{"missing-sum", "duplicate-sum", "bad-sum", "extra-field", "short-sum", "uppercase-sum", "prefix-sum", "mismatch", "download", "partial", "receipt", "binary", "latest-url", "latest-tag", "latest-newline", "latest-unavailable", "unavailable", "manifest-download", "partial-manifest", "candidate-version", "corrupt-gzip", "corrupt-tar", "truncated-gzip", "appended-archive"} {
		t.Run(problem, func(t *testing.T) {
			f := newInstallerFixture(t, shell, "Linux/x86_64", "linux_amd64")
			sum := fmt.Sprintf("%x  %s\n", sha256.Sum256(f.archive), f.asset)
			switch problem {
			case "extra-field":
				sum = strings.TrimSuffix(sum, "\n") + " extra\n"
			case "short-sum":
				sum = sum[1:]
			case "uppercase-sum":
				sum = strings.ToUpper(sum[:64]) + sum[64:]
			case "prefix-sum":
				sum = strings.ReplaceAll(sum, f.asset, "./"+f.asset)
			case "latest-newline":
				f.env = append(f.env, "MOCK_LATEST=https://github.com/primaprashant/sei/releases/tag/v0.1.0\n")
			case "unavailable":
				f.env = append(f.env, "MOCK_LATEST=https://github.com/primaprashant/sei/releases/tag/v9.9.9")
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
			case "corrupt-tar", "truncated-gzip":
				if problem == "corrupt-tar" {
					var buf bytes.Buffer
					gz := gzip.NewWriter(&buf)
					if _, err := gz.Write(bytes.Repeat([]byte("bad tar"), 200)); err != nil {
						t.Fatal(err)
					}
					if err := gz.Close(); err != nil {
						t.Fatal(err)
					}
					f.archive = buf.Bytes()
				} else {
					f.archive = f.archive[:len(f.archive)-8]
				}
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
			args := []string{"--install-dir", f.dest}
			if problem == "download" || problem == "partial" {
				args = append(args, "--version", "v0.1.0")
			}
			out, err := f.run(t, args...)
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

func testInstallerUtilities(t *testing.T, shell string) {
	for _, missing := range []string{"", "awk", "chmod", "cmp", "curl", "gzip", "mkdir", "mktemp", "mv", "rm", "sed", "tar", "uname", "shasum"} {
		t.Run("missing-"+missing, func(t *testing.T) {
			f := newInstallerFixture(t, shell, "Linux/x86_64", "linux_amd64")
			tools := filepath.Join(f.root, "tools")
			// Deliberately omit sha256sum to exercise the native macOS fallback
			// even on Linux. Keep only explicit installer/mock utility dependencies.
			for _, name := range []string{"awk", "chmod", "cmp", "cp", "gzip", "mkdir", "mktemp", "rm", "sed", "shasum", "tar"} {
				path, err := exec.LookPath(name)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path, filepath.Join(tools, name)); err != nil {
					t.Fatal(err)
				}
			}
			f.env[0] = "PATH=" + tools
			if missing != "" {
				if err := os.Remove(filepath.Join(tools, missing)); err != nil {
					t.Fatal(err)
				}
			}
			out, err := f.run(t, "--install-dir", f.dest)
			if missing != "" {
				if missing == "shasum" {
					missing = "sha256sum or shasum"
				}
				if err == nil || !strings.Contains(out, "required utility missing: "+missing) {
					t.Fatalf("%v: %s", err, out)
				}
			} else if err != nil {
				t.Fatalf("shasum fallback: %v: %s", err, out)
			}
		})
	}
}

func testInstallerArchiveSafety(t *testing.T, shell string) {
	for _, problem := range []string{"extra", "missing", "duplicate", "duplicate-doc", "traversal", "absolute", "dot", "newline", "tab", "backslash", "nested", "directory", "symlink", "hardlink", "fifo", "device", "block", "pax-path", "gnu-long-path", "pax-link", "gnu-long-link"} {
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
			case "duplicate-doc":
				headers[3].Name = "README.md"
			case "tab":
				headers[0].Name = "sei\t"
			case "backslash":
				headers[0].Name = "dir\\sei"
			case "nested":
				headers[0].Name = "dir/../sei"
			case "pax-path", "gnu-long-path":
				headers[0].Name = strings.Repeat("nested/", 40) + "../sei"
				headers[0].Format = tar.FormatPAX
				if problem == "gnu-long-path" {
					headers[0].Format = tar.FormatGNU
				}
			case "pax-link", "gnu-long-link":
				headers[0].Typeflag = tar.TypeSymlink
				headers[0].Format = tar.FormatPAX
				if problem == "gnu-long-link" {
					headers[0].Format = tar.FormatGNU
					headers[0].Typeflag = tar.TypeLink
				}
				headers[0].Linkname = "/" + strings.Repeat("outside/", 40)
			case "traversal":
				headers[0].Name = "../sei"
			case "absolute":
				headers[0].Name = "/sei"
			case "dot":
				headers[0].Name = "./sei"
			case "newline":
				headers[0].Name = "sei\nLICENSE"
			default:
				headers[0].Typeflag = map[string]byte{"directory": tar.TypeDir, "symlink": tar.TypeSymlink, "hardlink": tar.TypeLink, "fifo": tar.TypeFifo, "device": tar.TypeChar, "block": tar.TypeBlock}[problem]
				headers[0].Linkname = "README.md"
			}
			f := newInstallerFixture(t, shell, "Linux/x86_64", "linux_amd64", headers...)
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

func testInstallerDirectory(t *testing.T, shell string) {
	for _, problem := range []string{"unwritable", "file-parent", "resolved-control", "tab", "escape", "carriage-return"} {
		t.Run(problem, func(t *testing.T) {
			f := newInstallerFixture(t, shell, "Linux/x86_64", "linux_amd64")
			switch problem {
			case "unwritable":
				if os.Geteuid() == 0 {
					t.Skip("permission denial requires non-root")
				}
				if err := os.MkdirAll(f.dest, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(f.dest, 0500); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(f.dest, 0700) })
			case "file-parent":
				writeTestFile(t, filepath.Join(f.root, "user's bin"), "unrelated")
			case "resolved-control":
				physical := filepath.Join(f.root, "trailing\n")
				if err := os.Mkdir(physical, 0700); err != nil {
					t.Fatal(err)
				}
				f.dest = filepath.Join(f.root, "alias")
				if err := os.Symlink(physical, f.dest); err != nil {
					t.Fatal(err)
				}
			default:
				f.dest += map[string]string{"tab": "\t", "escape": "\x1b", "carriage-return": "\r"}[problem]
			}
			out, err := f.run(t, "--install-dir", f.dest)
			if err == nil {
				t.Fatalf("accepted directory: %s", out)
			}
			if _, err := os.Lstat(filepath.Join(f.root, "requests")); !os.IsNotExist(err) {
				t.Fatal("directory failure reached download", err)
			}
			if problem == "file-parent" {
				if got, err := os.ReadFile(filepath.Join(f.root, "user's bin")); err != nil || string(got) != "unrelated" {
					t.Fatal("parent clobbered", err)
				}
			}
		})
	}
}
