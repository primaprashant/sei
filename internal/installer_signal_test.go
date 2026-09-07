package app

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Pipes acknowledge the exact commit boundary; no timing assumption or polling.
func testInstallerInterruption(t *testing.T, shell string) {
	for _, upgrade := range []bool{false, true} {
		for _, point := range []string{"before-receipt", "after-receipt", "after-binary"} {
			for _, signal := range []syscall.Signal{syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM} {
				t.Run(fmt.Sprintf("upgrade=%t/%s/%s", upgrade, point, signal), func(t *testing.T) {
					f := newInstallerFixture(t, shell, "Linux/x86_64", "linux_amd64")
					if err := os.MkdirAll(f.dest, 0700); err != nil {
						t.Fatal(err)
					}
					old := []byte("#!/bin/sh\nprintf 'sei 0.0.9\\n'\n")
					oldReceipt := ""
					if upgrade {
						if err := os.WriteFile(filepath.Join(f.dest, "sei"), old, 0755); err != nil {
							t.Fatal(err)
						}
						oldReceipt = fmt.Sprintf("sei-install-receipt-v1\nv0.0.9 %x\n", sha256.Sum256(old))
						writeTestFile(t, filepath.Join(f.dest, ".sei-install-receipt"), oldReceipt)
					}
					writeTestFile(t, filepath.Join(f.dest, "unrelated"), "untouched")
					if err := os.Mkdir(filepath.Join(f.dest, ".sei-install.keep"), 0700); err != nil {
						t.Fatal(err)
					}
					writeTestFile(t, filepath.Join(f.dest, ".sei-install.keep", "sentinel"), "untouched")
					writeTestFile(t, filepath.Join(f.root, "outside"), "untouched")
					if err := os.Symlink(filepath.Join(f.root, "outside"), filepath.Join(f.dest, "outside-link")); err != nil {
						t.Fatal(err)
					}
					readyR, readyW, err := os.Pipe()
					if err != nil {
						t.Fatal(err)
					}
					defer func() { _ = readyR.Close(); _ = readyW.Close() }()
					releaseR, releaseW, err := os.Pipe()
					if err != nil {
						t.Fatal(err)
					}
					defer func() { _ = releaseR.Close(); _ = releaseW.Close() }()
					f.pipes = []*os.File{readyW, releaseR}
					body := `#!/bin/sh
set -eu
pause() { printf x >&3; IFS= read -r release <&4; exit 0; }
case "$POINT/$1" in before-receipt/*/receipt) pause ;; esac
"$REAL_MV" "$@"
case "$POINT/$1" in after-receipt/*/receipt|after-binary/*/sei) pause ;; esac
`
					if err := os.WriteFile(filepath.Join(f.root, "tools", "mv"), []byte(body), 0755); err != nil {
						t.Fatal(err)
					}
					f.env = append(f.env, "POINT="+point)
					f.start = func(cmd *exec.Cmd) {
						defer func() { _, _ = releaseW.Write([]byte("continue\n")) }()
						ready := make(chan error, 1)
						go func() { var b [1]byte; _, err := io.ReadFull(readyR, b[:]); ready <- err }()
						select {
						case err := <-ready:
							if err != nil {
								t.Error(err)
								return
							}
						case <-time.After(5 * time.Second):
							t.Error("commit handshake timed out")
							_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
							_ = readyR.Close()
							<-ready
							return
						}
						if err := cmd.Process.Signal(signal); err != nil {
							t.Error(err)
						}
					}
					out, err := f.run(t, "--version", "v0.1.0", "--install-dir", f.dest)
					if err == nil || strings.Contains(out, "Installed sei") || strings.Contains(out, "preserved") {
						t.Fatalf("interruption diagnostic: %v: %s", err, out)
					}
					wantReceipt := oldReceipt
					if point != "before-receipt" {
						if wantReceipt == "" {
							wantReceipt = "sei-install-receipt-v1\n"
						}
						wantReceipt += fmt.Sprintf("v0.1.0 %x\n", sha256.Sum256(f.binary))
					}
					wantBinary := old
					if !upgrade {
						wantBinary = nil
					}
					if point == "after-binary" {
						wantBinary = f.binary
					}
					for name, want := range map[string][]byte{"sei": wantBinary, ".sei-install-receipt": []byte(wantReceipt), "unrelated": []byte("untouched"), ".sei-install.keep/sentinel": []byte("untouched"), "outside-link": []byte("untouched")} {
						got, err := os.ReadFile(filepath.Join(f.dest, name))
						if len(want) == 0 {
							if !os.IsNotExist(err) {
								t.Fatalf("unexpected %s: %v", name, err)
							}
						} else if err != nil || !bytes.Equal(got, want) {
							t.Fatalf("%s: %q, %v; want %q", name, got, err, want)
						}
					}
					if upgrade {
						want := "sei 0.0.9\n"
						if point == "after-binary" {
							want = "sei 0.1.0\n"
						}
						if got, err := exec.Command(filepath.Join(f.dest, "sei"), "--version").CombinedOutput(); err != nil || string(got) != want {
							t.Fatalf("live binary: %q, %v", got, err)
						}
					}
					f.start, f.pipes = nil, nil
					if err := os.Remove(filepath.Join(f.root, "tools", "mv")); err != nil {
						t.Fatal(err)
					}
					out, err = f.run(t, "--version", "v0.1.0", "--install-dir", f.dest)
					if !upgrade && point == "after-receipt" {
						if err == nil || !strings.Contains(out, "unrecognized sei or receipt") {
							t.Fatalf("orphan receipt accepted: %v: %s", err, out)
						}
					} else if err != nil {
						t.Fatalf("interrupted install retry: %v: %s", err, out)
					}
				})
			}
		}
	}
}
