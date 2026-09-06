package app

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseDraft(t *testing.T) {
	script := repoPath(t, "scripts/release.sh")
	const commit = "bd2f8c10000000000000000000000000000000000"
	for _, tc := range []struct {
		tag, fault string
		valid      bool
	}{
		{"v0.1.0", "", true}, {"v1.0.0", "", true}, {"v12.34.56", "", true},
		{"v0.1.0-rc.1", "", true}, {"v1.2.3-rc.1", "", true}, {"v1.2.3-0.a-b", "", true},
		{"v01.2.3", "", false}, {"v1.2", "", false}, {"v1.2.3+build", "", false},
		{"v1.2.3-01", "", false}, {"v1.2.3-rc..1", "", false}, {"v1.2.3\n", "", false},
		{"v1.2.3;touch bad", "", false}, {"1.2.3", "", false},
		{"v1.0.0", "branch", false}, {"v1.0.0", "tag-commit", false},
		{"v1.0.0", "event-commit", false}, {"v1.0.0", "producer-commit", false},
		{"v1.0.0", "snapshot", false}, {"v1.0.0", "archive", false},
		{"v1.0.0", "manifest", false}, {"v1.0.0", "installer", false},
		{"v1.0.0", "installer-sum", false}, {"v1.0.0", "source", false},
		{"v1.0.0", "existing", false}, {"v1.0.0", "api", false},
		{"v1.0.0", "remote", false}, {"v1.0.0", "upload", false},
	} {
		t.Run(tc.tag+"/"+tc.fault, func(t *testing.T) {
			root := t.TempDir()
			for _, dir := range []string{"tools", "scripts", "dist"} {
				if err := os.Mkdir(filepath.Join(root, dir), 0700); err != nil {
					t.Fatal(err)
				}
			}
			write := func(name, data string, mode os.FileMode) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(root, name), []byte(data), mode); err != nil {
					t.Fatal(err)
				}
			}
			write("tools/git", `#!/bin/sh
if [ "$FAULT" = tag-commit ] && [ "$2" = --verify ]; then printf 'wrong\n'; else printf '%s\n' "$COMMIT"; fi
`, 0755)
			write("tools/gh", `#!/bin/sh
case "$1" in
 --version) printf 'mock gh\n' ;;
 api)
   if [ "$2" = --paginate ]; then
     [ "$FAULT" != api ] || exit 1
     if [ "$FAULT" = existing ]; then printf '%s\n' "$GITHUB_REF_NAME"; fi
   elif [ "$FAULT" = remote ]; then printf 'wrong\n'
   else printf '%s\n' "$COMMIT"; fi ;;
 release)
   printf '%s\n' "$@" > "$LOG"
   [ "$FAULT" != upload ] ;;
 *) exit 99 ;;
esac
`, 0755)
			v := strings.TrimPrefix(tc.tag, "v")
			manifest := ""
			for _, target := range []string{"linux_amd64", "linux_arm64", "darwin_amd64", "darwin_arm64"} {
				name := "sei_" + v + "_" + target + ".tar.gz"
				write("dist/"+name, target, 0600)
				manifest += fmt.Sprintf("%x  %s\n", sha256.Sum256([]byte(target)), name)
			}
			write("dist/sei_"+v+"_checksums.txt", manifest, 0600)
			write("scripts/install.sh", "installer", 0600)
			write("dist/install.sh", "installer", 0600)
			write("dist/install.sh.sha256", fmt.Sprintf("%x  install.sh\n", sha256.Sum256([]byte("installer"))), 0600)
			for fault, name := range map[string]string{
				"archive": "dist/sei_" + v + "_linux_amd64.tar.gz", "manifest": "dist/sei_" + v + "_checksums.txt",
				"installer": "dist/install.sh", "installer-sum": "dist/install.sh.sha256", "source": "scripts/install.sh",
			} {
				if tc.fault == fault {
					write(name, "corrupt", 0600)
				}
			}
			ref, event, producer, version := "refs/tags/"+tc.tag, commit, commit, v
			switch tc.fault {
			case "branch":
				ref = "refs/heads/" + tc.tag
			case "event-commit":
				event = "wrong"
			case "producer-commit":
				producer = "wrong"
			case "snapshot":
				version += "-snapshot.bd2f8c1"
			}
			cmd := exec.Command("bash", script, "draft")
			cmd.Dir = root
			// Deliberately do not inherit tokens, gh configuration or repository identity.
			cmd.Env = []string{"PATH=" + filepath.Join(root, "tools") + ":" + os.Getenv("PATH"), "HOME=" + root,
				"COMMIT=" + commit, "FAULT=" + tc.fault, "LOG=" + filepath.Join(root, "upload"),
				"GITHUB_REF_NAME=" + tc.tag, "GITHUB_REF=" + ref, "GITHUB_REF_TYPE=tag", "GITHUB_SHA=" + event,
				"SEI_RELEASE_COMMIT=" + producer, "SEI_RELEASE_VERSION=" + version, "GITHUB_REPOSITORY=local/mock"}
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v: %v\n%s", tc.valid, err, out)
			}
			log, logErr := os.ReadFile(filepath.Join(root, "upload"))
			if !tc.valid && tc.fault != "upload" {
				if !os.IsNotExist(logErr) {
					t.Fatalf("invalid input reached upload: %s, %v", log, logErr)
				}
				return
			}
			pre := strings.Contains(tc.tag, "-")
			want := []string{"release", "create", tc.tag}
			for _, target := range []string{"linux_amd64", "linux_arm64", "darwin_amd64", "darwin_arm64"} {
				want = append(want, "sei_"+v+"_"+target+".tar.gz")
			}
			want = append(want, "sei_"+v+"_checksums.txt", "install.sh", "install.sh.sha256", "--repo", "local/mock",
				"--draft", "--verify-tag", fmt.Sprintf("--prerelease=%t", pre), "--latest=false", "--title", "sei "+tc.tag, "--notes")
			if logErr != nil || !strings.HasPrefix(string(log), strings.Join(want, "\n")+"\nPersonal-tool draft from "+commit) {
				t.Fatalf("unexpected upload arguments: %s, %v", log, logErr)
			}
		})
	}
}

func TestReleaseWorkflowContract(t *testing.T) {
	for file, fragments := range map[string][]string{
		".github/workflows/ci.yml": {
			"  push:\n  pull_request:\n  workflow_call:", "type: boolean\n        default: false",
			"fetch-depth: ${{ inputs.release && '0' || '1' }}", "run: bash scripts/release.sh tag",
			"GORELEASER_CURRENT_TAG=\"$GITHUB_REF_NAME\" GOPROXY=off goreleaser release --clean --skip=publish", "goreleaser release --snapshot --clean",
			"test \"$SEI_RELEASE_VERSION\" = \"${GITHUB_REF_NAME#v}\"", "cp scripts/install.sh dist/install.sh",
			"artifact-ids: ${{ needs.archives.outputs.artifact-id }}", "go test -race -count=1 ./...",
			"target: [FuzzParseConfig, FuzzPathName]", "go test -count=1 -run '^TestRelease' -v ./internal",
		},
		".github/workflows/release.yml": {
			"tags: ['v*']", "uses: ./.github/workflows/ci.yml\n    with:\n      release: true",
			"draft-upload:\n    needs: verify", "    permissions:\n      contents: write",
			"artifact-ids: ${{ needs.verify.outputs.artifact-id }}", "digest-mismatch: error",
			"GH_TOKEN: ${{ github.token }}", "SEI_RELEASE_VERSION: ${{ needs.verify.outputs.version }}",
			"SEI_RELEASE_COMMIT: ${{ needs.verify.outputs.commit }}", "run: bash scripts/release.sh draft",
		},
	} {
		data, err := os.ReadFile(repoPath(t, file))
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, fragment := range fragments {
			if !strings.Contains(text, fragment) {
				t.Errorf("%s missing contract: %s", file, fragment)
			}
		}
		wantWrites := 0
		if strings.HasSuffix(file, "/release.yml") {
			wantWrites = 1
		}
		if strings.Count(text, ": write") != wantWrites || strings.Contains(text, "secrets:") || strings.Contains(text, "persist-credentials: true") {
			t.Errorf("%s permission boundary changed", file)
		}
	}
}
