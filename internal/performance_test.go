package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

func representativeHelpMatches(output string, wants ...string) bool {
	text := strings.NewReplacer("\r", "", "\n", "").Replace(ansi.Strip(output))
	for _, want := range wants {
		if !strings.Contains(text, want) {
			return false
		}
	}
	return true
}

func TestRepresentativeReadiness(t *testing.T) {
	cfg := rootFixture(t)
	browseMkdir(t, filepath.Join(cfg.Library, "first"))
	m := newBrowseModel(cfg)
	next, refresh := m.Update(m.Init()())
	m = finishMutationRefresh(t, next.(browseModel), refresh)
	m.showHelp = true
	ready := []string{"Focused: Library", "Selected name: firstListing: ready (1 entries)", "Root relations checked; every mutation revalidates."}
	for _, width := range []int{80, 143, 148, 150} {
		m.width, m.height = width, 100
		next, _ = m.Update(startBrowseMsg{})
		m = next.(browseModel)
		next, _ = m.Update(rootSafetyMsg{m.safetyGeneration, resolveRoots(cfg)})
		m = next.(browseModel)
		m.status = `Add "previous" to Agent1 / Project (/target): complete`
		if representativeHelpMatches(m.View().Content, ready...) {
			t.Fatal("safety completion accepted a still-loading listing")
		}
		next, _ = m.Update(scanPanel(0, m.panels[0].generation, cfg.Library)())
		m = next.(browseModel)
		if !representativeHelpMatches(m.View().Content, ready...) {
			t.Fatal("completed listing not recognized")
		}
		want := "Result: " + displayText(`Add "first" to Agent1 / Project (/target): complete`) + "Help 1/"
		for _, status := range []string{m.status, `Add "first" to Agent1 / Project (/other): complete`, `Remove "first" from Agent1 / Project (/target): complete`, `Add "first" to Agent1 / Global (/target): complete`, `Add "first" to Agent1 / Project (/target): working`, `Add "first" to Agent1 / Project (/target): complete`} {
			m.status = status
			got := representativeHelpMatches(m.View().Content, want)
			if got != (status == `Add "first" to Agent1 / Project (/target): complete`) {
				t.Fatalf("wrong completion match: %q", status)
			}
		}
	}
}

// Opt-in local DATA and already-built release binary; ordinary tests are offline.
func TestRepresentativeUI(t *testing.T) {
	runRepresentativeUI(t, false)
}

func TestPerformanceRelease(t *testing.T) {
	runRepresentativeUI(t, true)
}

func checkPerformanceBudget(metric string, p95 float64) error {
	limit, ok := map[string]float64{"startup": 100, "input": 25, "add": 50, "replace": 60, "remove": 25}[metric]
	if !ok || p95 > limit {
		return fmt.Errorf("%s p95 %.3f ms exceeds approved %.3f ms budget (known metric: %v)", metric, p95, limit, ok)
	}
	return nil
}

func TestPerformanceBudget(t *testing.T) {
	for metric, limit := range map[string]float64{"startup": 100, "input": 25, "add": 50, "replace": 60, "remove": 25} {
		if checkPerformanceBudget(metric, limit) != nil || checkPerformanceBudget(metric, limit+0.001) == nil {
			t.Fatalf("budget boundary not enforced: %s", metric)
		}
	}
	if checkPerformanceBudget("unknown", 0) == nil {
		t.Fatal("unknown metric silently accepted")
	}
}

func runRepresentativeUI(t *testing.T, measured bool) {
	t.Helper()
	archive, binary := os.Getenv("SEI_TEST_ARCHIVE"), os.Getenv("SEI_TEST_BINARY")
	prepare := os.Getenv("SEI_TEST_PREPARE")
	if measured && prepare != "" {
		t.Fatal("measurement requires a built binary, not fixture preparation")
	}
	if archive == "" || (binary == "" && prepare == "") {
		t.Skip("set SEI_TEST_ARCHIVE and SEI_TEST_BINARY; see docs/development.md#optional-performance-checks")
	}
	if prepare == "" && !filepath.IsAbs(binary) {
		t.Fatal("SEI_TEST_BINARY must be absolute")
	}
	scopedBudgets := measured && runtime.GOOS == "linux" && runtime.GOARCH == "amd64"
	if measured {
		info, err := os.Stat(binary)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("ARTIFACT bytes=%d; approved Linux amd64 warm-fixture budgets enforced=%v", info.Size(), scopedBudgets)
		if scopedBudgets && info.Size() > 6*1024*1024 {
			t.Fatal("binary exceeds approved 6 MiB budget")
		}
	}
	data, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != "9f134b3c1c308b88a5f3acc37e0e33fcf25cf86a964644df1d2f24866ea0d192" {
		t.Fatal("archive digest mismatch")
	}
	if prepare != "" {
		if !filepath.IsAbs(prepare) {
			t.Fatal("SEI_TEST_PREPARE must be a new absolute disposable directory")
		}
		if err := os.Mkdir(prepare, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := unpackRepresentative(bytes.NewReader(data), prepare); err != nil {
			t.Fatal(err)
		}
		cfg := config{Library: filepath.Join(prepare, "skills")}
		for i, name := range []string{"Claude Code", "Codex", "OpenCode"} {
			cfg.Agents = append(cfg.Agents, agentConfig{Name: name, Global: filepath.Join(prepare, fmt.Sprintf("global%d", i+1)), Local: fmt.Sprintf("local%d", i+1)})
		}
		if err := saveConfig(cfg, prepare, filepath.Join(prepare, "config.json"), false); err != nil {
			t.Fatal(err)
		}
		t.Logf("Prepared DATA only at %s; LICENSE outside skills; no scripts executed", prepare)
		return
	}
	timings := map[string][]float64{}
	defer func() {
		if t.Failed() {
			t.Log("INVALID RUN: no timing summary; fix harness/application failure and rerun the complete sample set")
			return
		}
		kinds := make([]string, 0, len(timings))
		for kind := range timings {
			kinds = append(kinds, kind)
		}
		slices.Sort(kinds)
		for _, kind := range kinds {
			values := timings[kind]
			if len(values) == 0 {
				continue
			}
			slices.Sort(values)
			p95 := values[(95*len(values)+99)/100-1]
			t.Logf("SUMMARY %s n=%d min=%.3f median=%.3f p95=%.3f max=%.3f ms", kind, len(values), values[0], values[len(values)/2], p95, values[len(values)-1])
			if _, metric, warm := strings.Cut(kind, "/warm/"); scopedBudgets && warm {
				if err := checkPerformanceBudget(metric, p95); err != nil {
					t.Error(err)
				}
			}
		}
	}()
	sizes, counts, profiles, trials := [][2]int{{148, 39}, {143, 35}, {80, 24}, {150, 40}}, []int{1, 3, 9}, []string{"dark", "light", "no-color"}, 1
	if measured {
		sizes, counts, profiles, trials = [][2]int{{143, 35}, {148, 39}}, []int{3}, []string{"no-color"}, 21
	}
	for _, size := range sizes {
		for _, count := range counts {
			for _, profile := range profiles {
				for trial := range trials {
					name := fmt.Sprintf("%dx%d/%d/%s", size[0], size[1], count, profile)
					if measured {
						name += fmt.Sprintf("/trial-%02d", trial)
					}
					t.Run(name, func(t *testing.T) {
						root := t.TempDir()
						if err := unpackRepresentative(bytes.NewReader(data), root); err != nil {
							t.Fatal(err)
						}
						library := filepath.Join(root, "skills")
						entries, missing, err := scanFolder(library)
						if err != nil || missing || len(entries) != 25 {
							t.Fatalf("upstream scan: %d %v", len(entries), err)
						}
						before := removeSnapshot(t, library)
						license, err := os.ReadFile(filepath.Join(root, "LICENSE"))
						if err != nil || !strings.Contains(string(license), "Copyright (c) 2025 Addy Osmani") {
							t.Fatalf("upstream license: %v", err)
						}
						script := before[filepath.Join(library, "idea-refine", "scripts", "idea-refine.sh")]
						if len(script.data) != 342 || script.info.Mode()&0o111 == 0 {
							t.Fatal("upstream executable fixture changed")
						}
						files, bytes := 0, 0
						for _, e := range before {
							if e.info.Mode().IsRegular() {
								files++
								bytes += len(e.data)
							}
						}
						if files != 30 || bytes != 384233 {
							t.Fatalf("upstream totals: %d %d", files, bytes)
						}
						cfg := config{Library: library}
						for i := range count {
							label := fmt.Sprintf("Agent%d", i+1)
							if measured {
								label = []string{"Claude Code", "Codex", "OpenCode"}[i]
							}
							cfg.Agents = append(cfg.Agents, agentConfig{Name: label, Global: filepath.Join(root, fmt.Sprintf("global%d", i+1)), Local: fmt.Sprintf("local%d", i+1)})
						}
						path := filepath.Join(root, "config.json")
						if err := saveConfig(cfg, root, path, false); err != nil {
							t.Fatal(err)
						}
						resolved, err := resolveConfigPaths(cfg, root)
						if err != nil {
							t.Fatal(err)
						}
						m := newBrowseModel(resolved)
						next, refresh := m.Update(m.Init()())
						m = finishMutationRefresh(t, next.(browseModel), refresh)
						m.width, m.height = size[0], size[1]
						for i := range m.panels {
							m.panels[i].path = strings.ReplaceAll(m.panels[i].path, root, "<FIXTURE>")
						}
						if out := os.Getenv("SEI_TEST_CAPTURES"); out != "" && count == 3 && profile == "no-color" {
							text := "Production View snapshot; paths substituted before layout; not a terminal screenshot.\n\n" + ansi.Strip(m.View().Content) + "\n"
							lines := strings.Split(text, "\n")
							for i := range lines {
								lines[i] = strings.TrimRight(lines[i], " ")
							}
							text = strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
							if err := os.WriteFile(filepath.Join(out, fmt.Sprintf("%dx%d.txt", size[0], size[1])), []byte(text), 0o600); err != nil {
								t.Fatal(err)
							}
						}
						ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
						defer cancel()
						cmd := exec.CommandContext(ctx, binary, "--config", path)
						cmd.Dir = root
						cmd.Env = []string{"HOME=" + root, "XDG_CONFIG_HOME=" + filepath.Join(root, ".config"), "TERM=xterm-256color", "PATH=" + os.Getenv("PATH")}
						switch profile {
						case "dark":
							cmd.Env = append(cmd.Env, "COLORFGBG=15;0")
						case "light":
							cmd.Env = append(cmd.Env, "COLORFGBG=0;15")
						case "no-color":
							cmd.Env = append(cmd.Env, "NO_COLOR=1")
						}
						terminal := newPTYScreen(t, size[0], size[1])
						start := time.Now()
						master, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(size[0]), Rows: uint16(size[1])})
						if err != nil {
							t.Fatal(err)
						}
						defer func() {
							if err := master.Close(); err != nil {
								t.Error(err)
							}
						}()
						wait := make(chan error, 1)
						go func() { wait <- cmd.Wait() }()
						waited := false
						defer func() {
							if !waited {
								cancel()
								<-wait
							}
						}()
						fd := int(master.Fd())
						if err := unix.SetNonblock(fd, true); err != nil {
							t.Fatal(err)
						}
						chunks := make(chan string)
						readerCtx, stop := context.WithCancel(ctx)
						done := make(chan struct{})
						go func() { defer close(done); defer close(chunks); _ = readLifecyclePTY(readerCtx, fd, chunks) }()
						defer func() { stop(); <-done }()
						var screen strings.Builder
						await := func(wants ...string) {
							t.Helper()
							for !ptyScreenMatches(terminal, wants...) {
								select {
								case chunk, ok := <-chunks:
									if !ok {
										t.Fatalf("closed awaiting %q: %s", wants, terminal.String())
									}
									screen.WriteString(chunk)
									if _, err := terminal.WriteString(chunk); err != nil {
										t.Fatal(err)
									}
								case <-ctx.Done():
									t.Fatalf("timeout awaiting %q: %s", wants, terminal.String())
								}
							}
						}
						send := func(keys string) {
							t.Helper()
							start = time.Now()
							if _, err := io.WriteString(master, keys); err != nil {
								t.Fatal(err)
							}
						}
						closeHelp := func() { send("?"); await("PROJECT") }
						// Match current cells, including retained text and scroll operations,
						// never concatenated differential output or a prior help surface.
						helpUntil := func(wants ...string) {
							t.Helper()
							send("?")
							await(append([]string{"sei | Help", " | up/down scroll | ?/Esc close |"}, wants...)...)
						}
						await("> " + entries[0].name)
						await("Ready")
						await("Folder not created")
						measure := func(kind string) {
							ms := float64(time.Since(start).Microseconds()) / 1000
							if measured {
								group := "warm"
								if trial == 0 {
									group = "first-observed"
								}
								kind = fmt.Sprintf("%dx%d/%s/%s", size[0], size[1], group, kind)
							}
							timings[kind] = append(timings[kind], ms)
							t.Logf("TIMING %s %.3f ms", kind, ms)
						}
						measure("startup")
						if out := os.Getenv("SEI_TEST_CAPTURES"); out != "" && count == 3 && profile == "no-color" {
							// Preserve actual emitted control sequences as quoted text, not
							// executable terminal controls or an invented reconstructed screen.
							raw := regexp.MustCompile(`/[^\s\x1b]*TestRep[^\s\x1b]*`).ReplaceAllString(screen.String(), "<FIXTURE-PATH>")
							text := fmt.Sprintf("Actual Linux PTY startup output, %dx%d, 3 agents, NO_COLOR=1. Temporary paths (including truncated paths) sanitized; ANSI quoted, not a screenshot.\n%q\n", size[0], size[1], raw)
							if err := os.WriteFile(filepath.Join(out, fmt.Sprintf("%dx%d-pty.txt", size[0], size[1])), []byte(text), 0o600); err != nil {
								t.Fatal(err)
							}
						}
						if count == 3 && size[0] >= 143 {
							local := filepath.Join(root, "local1")
							label := cfg.Agents[0].Name
							ready := func(focus, path, name, listing string) {
								helpUntil("Focused: "+focus, "Root path: "+path, "Selected name: "+name+"\nListing: "+listing, "Root relations checked; every mutation revalidates.")
								closeHelp()
							}
							for i := range 3 {
								send("1")
								name, listing := "(none)", "not created"
								if i > 0 {
									name, listing = entries[0].name, fmt.Sprintf("ready (%d entries)", i)
								}
								ready(label+" / Project", local, name, listing)
								send("0")
								ready("Library", library, entries[i].name, "ready (25 entries)")
								send("a")
								actionStart := start
								helpUntil("Result: " + displayText(fmt.Sprintf("Add %q to %s / Project (%s): complete", entries[i].name, label, local)) + "\nHelp 1/")
								start = actionStart
								measure("add")
								closeHelp()
								if i < 2 {
									ready("Library", library, entries[i].name, "ready (25 entries)")
									send("\x1b[B")
									await("> " + entries[i+1].name)
									measure("input")
								}
							}
							if measured {
								// A different skill from the preceding add prevents a retained
								// success from falsely satisfying the replacement wait.
								send("\x1b[A\x1b[A")
								ready("Library", library, entries[0].name, "ready (25 entries)")
								writeTestFile(t, filepath.Join(local, entries[0].name, "destination-only"), "must be deleted")
								send("a")
								actionStart := start
								helpUntil("Result: " + displayText(fmt.Sprintf("Add %q to %s / Project (%s): complete", entries[0].name, label, local)) + "\nHelp 1/")
								start = actionStart
								measure("replace")
								setupAbsent(t, filepath.Join(local, entries[0].name, "destination-only"))
								closeHelp()
							}
							send("1")
							for i := range 2 {
								ready(label+" / Project", local, entries[i].name, fmt.Sprintf("ready (%d entries)", 3-i))
								send("x")
								actionStart := start
								helpUntil("Result: " + displayText(fmt.Sprintf("Remove %q from %s / Project (%s): complete", entries[i].name, label, local)) + "\nHelp 1/")
								start = actionStart
								measure("remove")
								closeHelp()
							}
						}
						for i, keys := range []string{fmt.Sprint(count), "g" + fmt.Sprint(count)} {
							send(keys)
							helpUntil(fmt.Sprintf("Focused: %s / %s", cfg.Agents[count-1].Name, []string{"Project", "Global"}[i]), "Selected name: (none)\nListing: not created", "Root relations checked; every mutation revalidates.")
							closeHelp()
						}
						writeTestFile(t, filepath.Join(root, "global1"), "Supplemental test-only failure: root is a file")
						send("g1r")
						helpUntil("Focused: "+cfg.Agents[0].Name+" / Global", "Selected name: (none)\nListing: error\nError:")
						send("q")
						for chunk := range chunks {
							screen.WriteString(chunk)
						}
						if ctx.Err() != nil {
							t.Fatal(ctx.Err())
						}
						if err := <-wait; err != nil {
							waited = true
							t.Fatal(err)
						}
						waited = true
						assertRemoveSnapshot(t, library, before)
						if count == 3 && size[0] >= 143 {
							remaining, _, err := scanFolder(filepath.Join(root, "local1"))
							if err != nil || len(remaining) != 1 || remaining[0].name != entries[2].name {
								t.Fatalf("primary flow persistence: %v %v", remaining, err)
							}
							for source, entry := range before {
								base := filepath.Join(library, entries[2].name)
								rel, err := filepath.Rel(base, source)
								if err != nil || !filepath.IsLocal(rel) || !entry.info.Mode().IsRegular() {
									continue
								}
								target := filepath.Join(root, "local1", entries[2].name, rel)
								data, err := os.ReadFile(target)
								info, statErr := os.Stat(target)
								if err != nil || statErr != nil || string(data) != entry.data || os.SameFile(entry.info, info) {
									t.Fatalf("independent copied bytes %s: %v %v", rel, err, statErr)
								}
							}
						}
					})
				}
			}
		}
	}
}
