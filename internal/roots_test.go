package app

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func rootFixture(t *testing.T) config {
	t.Helper()
	base := t.TempDir()
	cfg := config{Home: base + "/home", Project: base + "/project", Library: base + "/library",
		Agents: []agentConfig{{Name: "Example", Global: base + "/global", Local: base + "/project/local"}}}
	for _, p := range []string{cfg.Home, cfg.Project, cfg.Library, cfg.Agents[0].Global, cfg.Agents[0].Local} {
		browseMkdir(t, p)
	}
	return cfg
}

func TestResolveRoots(t *testing.T) {
	base := t.TempDir()
	browseMkdir(t, base+"/real/deep")
	if err := os.Symlink("real/deep", base+"/link"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("absent", base+"/dangling"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("loop", base+"/loop"); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, base+"/file", "x")
	physical := resolveRoot(base)
	if physical.err != nil {
		t.Fatal(physical.err)
	}
	for _, tc := range []struct {
		path, want string
		missing    []string
		bad        bool
	}{
		{"/link/..", "/real", nil, false},
		{"/link/../new/child", "/real/new/child", []string{"new", "child"}, false},
		{"/real/./deep/..", "/real", nil, false},
		{"/missing/./child/", "/missing/child", []string{"missing", "child"}, false},
		{"/missing/../real", "", nil, true},
		{"/dangling", "", nil, true}, {"/dangling/..", "", nil, true},
		{"/file", "", nil, true}, {"/file/..", "", nil, true},
		{"/loop", "", nil, true}, {"/loop/..", "", nil, true},
	} {
		t.Run(tc.path, func(t *testing.T) {
			r := resolveRoot(base + tc.path)
			if (r.err != nil) != tc.bad {
				t.Fatalf("%+v", r)
			}
			if tc.bad {
				return
			}
			if r.path != physical.path+tc.want || !reflect.DeepEqual(r.missing, tc.missing) {
				t.Fatalf("got %+v", r)
			}
			if len(r.ancestors) == 0 || r.ancestor != r.ancestors[0].path {
				t.Fatal("missing verified ancestor")
			}
		})
	}
}

func TestRootSafety(t *testing.T) {
	for _, kind := range []string{"safe", "library equal", "library above", "library below", "dest equal", "dest above", "dest below", "alias", "local escape", "lexical escape", "missing library", "unknown library", "unknown destination"} {
		t.Run(kind, func(t *testing.T) {
			cfg := rootFixture(t)
			g := cfg.Agents[0].Global
			switch kind {
			case "library equal":
				cfg.Library = g
			case "library above":
				cfg.Library = filepath.Dir(g)
			case "library below":
				cfg.Library = g + "/nested"
			case "dest equal":
				cfg.Agents[0].Local = g
			case "dest above":
				cfg.Agents[0].Local = filepath.Dir(g)
			case "dest below":
				cfg.Agents[0].Local = g + "/nested"
			case "alias":
				cfg.Library = g + "-alias"
				if err := os.Symlink(g, cfg.Library); err != nil {
					t.Fatal(err)
				}
			case "local escape":
				if err := os.Symlink(cfg.Home, cfg.Project+"/link"); err != nil {
					t.Fatal(err)
				}
				cfg.Agents[0].Local = cfg.Project + "/link/../outside"
			case "lexical escape":
				cfg.Agents[0].Local = cfg.Project + "/../elsewhere"
			case "missing library":
				cfg.Library += "/absent/child"
			case "unknown library":
				cfg.Library += "/dangling"
				if err := os.Symlink("absent", cfg.Library); err != nil {
					t.Fatal(err)
				}
			case "unknown destination":
				cfg.Agents[0].Local += "/file"
				writeTestFile(t, cfg.Agents[0].Local, "x")
			}
			s := resolveRoots(cfg)
			wantGlobal := kind != "safe" && kind != "missing library" && kind != "local escape" && kind != "lexical escape"
			if (s.blocked[1] != nil) != wantGlobal {
				t.Fatalf("global: %v", s.blocked[1])
			}
			if (kind == "local escape" || kind == "lexical escape") && s.blocked[2] == nil {
				t.Fatal("local escape accepted")
			}
			if kind == "missing library" {
				if _, err := revalidateSkillRoot(cfg, 1, "unlisted"); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestRootSafetyPermissions(t *testing.T) {
	cfg := rootFixture(t)
	library := cfg.Library
	if err := os.Chmod(cfg.Library, 0o111); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(library, 0o700); err != nil {
			t.Error(err)
		}
	})
	_, _, listingErr := scanFolder(cfg.Library)
	if os.Geteuid() != 0 && !errors.Is(listingErr, os.ErrPermission) {
		t.Fatalf("expected unreadable listing: %v", listingErr)
	}
	if _, err := revalidateSkillRoot(cfg, 1, "skill"); err != nil {
		t.Fatalf("unreadable contents are not unknown boundary: %v", err)
	}
	if err := os.Chmod(cfg.Library, 0); err != nil {
		t.Fatal(err)
	}
	cfg.Library += "/hidden"
	r := resolveRoot(cfg.Library)
	if os.Geteuid() == 0 {
		if r.err != nil {
			t.Fatal(r.err)
		}
		t.Log("root bypasses directory permissions; non-root denial assertions require unprivileged execution")
	} else {
		if !errors.Is(r.err, os.ErrPermission) {
			t.Fatalf("expected permission error: %v", r.err)
		}
		if resolveRoots(cfg).blocked[1] == nil {
			t.Fatal("unknown library allowed")
		}
	}
}

func TestRootSafetyCaseIdentity(t *testing.T) {
	cfg := rootFixture(t)
	upper := cfg.Agents[0].Global + "/CaseRoot"
	browseMkdir(t, upper)
	a, err := os.Stat(upper)
	if err != nil {
		t.Fatal(err)
	}
	lower := cfg.Agents[0].Global + "/caseroot"
	b, err := os.Stat(lower)
	if errors.Is(err, os.ErrNotExist) {
		browseMkdir(t, lower)
		b, err = os.Stat(lower)
	}
	if err != nil {
		t.Fatal(err)
	}
	same := os.SameFile(a, b)
	t.Logf("actual filesystem case alias: %v", same)
	cfg.Library = upper
	cfg.Agents[0].Global = lower
	if got := resolveRoots(cfg).blocked[1] != nil; got != same {
		t.Fatalf("case identity blocked=%v, same=%v", got, same)
	}
	if same {
		cfg.Agents[0].Global = lower + "/missing"
		if resolveRoots(cfg).blocked[1] == nil {
			t.Fatal("aliased ancestor nesting accepted")
		}
	}
}

func TestSkillName(t *testing.T) {
	for _, name := range []string{"", ".", "..", "a/b", "/a", "a\x00b"} {
		if validateSkillName(name) == nil {
			t.Errorf("accepted %q", name)
		}
	}
	for _, name := range []string{".hidden", "..valid", "with spaces", "a\\b", "bad\xff", "\x1bname"} {
		if err := validateSkillName(name); err != nil {
			t.Error(err)
		}
	}
	for _, protected := range []string{"home", "project", "library", "library ancestor", "destination"} {
		t.Run(protected, func(t *testing.T) {
			cfg := rootFixture(t)
			child := cfg.Agents[0].Global + "/skill"
			browseMkdir(t, child)
			switch protected {
			case "home":
				cfg.Home = child
			case "project":
				cfg.Project = child
				cfg.Agents[0].Local = child + "/local"
			case "library":
				cfg.Library = child
			case "library ancestor":
				cfg.Library = child + "/library"
			case "destination":
				cfg.Agents[0].Local = child
			}
			if _, err := revalidateSkillRoot(cfg, 1, "skill"); err == nil {
				t.Fatal("protected child accepted")
			}
		})
	}
}

func TestRootSafetyHomeAncestor(t *testing.T) {
	for _, kind := range []string{"existing", "alias", "missing"} {
		t.Run(kind, func(t *testing.T) {
			cfg := rootFixture(t)
			child := cfg.Agents[0].Global + "/skill"
			browseMkdir(t, child)
			home := child + "/home"
			if kind != "missing" {
				browseMkdir(t, home)
			}
			if kind == "alias" {
				alias := cfg.Home + "/alias"
				if err := os.Symlink(home, alias); err != nil {
					t.Fatal(err)
				}
				home = alias
			}
			cfg.Home = home
			if err := resolveRoots(cfg).blocked[1]; err != nil {
				t.Fatalf("fixture blocked before child protection: %v", err)
			}
			if _, err := revalidateSkillRoot(cfg, 1, "skill"); err == nil {
				t.Fatal("home ancestor accepted")
			}
			if _, err := revalidateSkillRoot(cfg, 1, "other"); err != nil {
				t.Fatalf("unaffected sibling blocked: %v", err)
			}
		})
	}
}

func TestRootSafetyRevalidation(t *testing.T) {
	cfg := rootFixture(t)
	real := cfg.Agents[0].Global
	cfg.Agents[0].Global += "-alias"
	if err := os.Symlink(real, cfg.Agents[0].Global); err != nil {
		t.Fatal(err)
	}
	root, err := openSkillRoot(cfg, 1, "skill")
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(cfg.Agents[0].Global); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(cfg.Library, cfg.Agents[0].Global); err != nil {
		t.Fatal(err)
	}
	if _, err := openSkillRoot(cfg, 1, "skill"); err == nil {
		t.Fatal("retarget accepted")
	}
	cfg.Agents[0].Global = real + "/new/deep"
	if _, err := revalidateSkillRoot(cfg, 1, "skill"); err != nil {
		t.Fatal(err)
	}
	if _, err := openSkillRoot(cfg, 1, "skill"); err == nil {
		t.Fatal("opened missing root")
	}
	browseMkdir(t, cfg.Agents[0].Global)
	root, err = openSkillRoot(cfg, 1, "skill")
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	cfg.Agents[0].Global = real + "/other/deep"
	if err := os.Symlink(cfg.Library, real+"/other"); err != nil {
		t.Fatal(err)
	}
	if _, err := revalidateSkillRoot(cfg, 1, "skill"); err == nil {
		t.Fatal("new ancestor alias accepted")
	}
}

func TestRootSafetyAliasesAndScopes(t *testing.T) {
	cfg := rootFixture(t)
	local := cfg.Agents[0].Local
	cfg.Agents[0].Local = cfg.Project + "/alias"
	if err := os.Symlink(local, cfg.Agents[0].Local); err != nil {
		t.Fatal(err)
	}
	if err := resolveRoots(cfg).blocked[2]; err != nil {
		t.Fatalf("contained configured alias: %v", err)
	}
	browseMkdir(t, cfg.Agents[0].Global+"/skill")
	if err := os.Symlink(cfg.Agents[0].Global+"/skill", cfg.Home+"/alias"); err != nil {
		t.Fatal(err)
	}
	cfg.Home += "/alias"
	if _, err := revalidateSkillRoot(cfg, 1, "skill"); err == nil {
		t.Fatal("aliased home accepted as child")
	}
	if _, err := revalidateSkillRoot(cfg, 1, "other"); err != nil {
		t.Fatalf("unaffected child blocked: %v", err)
	}
	if err := os.Symlink(local, cfg.Agents[0].Global+"/link"); err != nil {
		t.Fatal(err)
	}
	if _, err := revalidateSkillRoot(cfg, 1, "link"); err == nil {
		t.Fatal("skill link accepted")
	}
	// Cross-agent global overlaps must be rejected too, not only paired panels.
	cfg.Agents = append(cfg.Agents, agentConfig{"Second", cfg.Agents[0].Global + "/nested", cfg.Project + "/second"})
	s := resolveRoots(cfg)
	if s.blocked[1] == nil || s.blocked[2] == nil || s.blocked[3] != nil || s.blocked[4] != nil {
		t.Fatalf("incorrect affected scope: %v", s.blocked)
	}
}

func TestRootSafetyBrowser(t *testing.T) {
	cfg := rootFixture(t)
	cfg.Library = cfg.Agents[0].Global
	browseMkdir(t, cfg.Library+"/visible")
	m := newBrowseModel(cfg)
	u, cmd := m.Update(startBrowseMsg{})
	m = u.(browseModel)
	commands := cmd().(tea.BatchMsg)
	safety := commands[len(commands)-1]().(rootSafetyMsg)
	u, _ = m.Update(safety)
	m = u.(browseModel)
	if !m.panels[1].loading || m.panels[1].safetyErr == nil {
		t.Fatal("safety depended on listing")
	}
	u, _ = m.Update(commands[1]())
	m = u.(browseModel)
	if m.panels[1].selectedName != "visible" || m.panels[1].safetyErr == nil {
		t.Fatal("listing erased safety or was blocked")
	}
	m.focused, m.showHelp = 1, true
	if !strings.Contains(m.View().Content, "Root safety blocked") {
		t.Fatal("missing safety reason")
	}
	u, _ = m.Update(startBrowseMsg{})
	m = u.(browseModel)
	u, _ = m.Update(safety)
	if u.(browseModel).panels[1].safetyChecked {
		t.Fatal("stale safety accepted")
	}
}
