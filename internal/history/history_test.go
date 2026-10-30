package history

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/blu9a/shipwake/internal/gitx"
)

// newRepo builds a small repository: a feature branch merged into main, a release, an open branch.
func newRepo(t *testing.T) gitx.Repo {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
			"GIT_AUTHOR_DATE=2026-01-01T10:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T10:00:00Z")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(name, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q", "-b", "main")
	write("README.md", "hello\n")
	run("add", "-A")
	run("commit", "-q", "-m", "chore: start")
	run("checkout", "-q", "-b", "feat/login")
	write("specs/login.feature", "Feature: Signing in\n  Scenario: Signing in with a password\n    Given I have an account\n    When I sign in\n    Then I see my plans\n")
	run("add", "-A")
	run("commit", "-q", "-m", "feat(login): sign in with a password")
	run("checkout", "-q", "main")
	run("merge", "-q", "--no-ff", "feat/login", "-m", "Merge feat/login: signing in")
	run("tag", "-a", "v0.1.0", "-m", "First release\n\nSigning in.")
	write("specs/login.feature", "Feature: Signing in\n  Scenario: Signing in with a password\n    Given I have an account\n    When I sign in with my password\n    Then I see my plans\n  Scenario: Signing out\n    When I sign out\n    Then I see the sign in screen\n")
	run("commit", "-q", "-am", "feat(login): sign out")
	run("checkout", "-q", "-b", "fix/typo")
	write("README.md", "Hello\n")
	run("commit", "-q", "-am", "fix: capital letter")
	run("checkout", "-q", "main")
	return gitx.Repo{Dir: dir}
}

func TestLoad(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)
	h, err := Load(ctx, r, "main", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Rows) != 3 {
		t.Fatalf("rows = %d, want 3 (commit, merge, root)", len(h.Rows))
	}
	m := h.Rows[1]
	if !m.Merge || m.Branch != "feat/login" || m.Title != "signing in" || m.Kind != "feat" || len(m.Commits) != 1 {
		t.Errorf("merge row = %+v", m)
	}
	if h.Rows[0].Kind != "feat" || h.Rows[0].Merge {
		t.Errorf("direct commit row = %+v", h.Rows[0])
	}
	if len(h.Tags) != 1 || h.Tags[0].Name != "v0.1.0" || h.Tags[0].Subject != "First release" || h.Tags[0].SHA != m.SHA {
		t.Errorf("tags = %+v", h.Tags)
	}
	if len(h.Branches) != 1 || h.Branches[0].Name != "fix/typo" || len(h.Branches[0].Ahead) != 1 || h.Branches[0].Behind != 0 {
		t.Errorf("branches = %+v", h.Branches)
	}
}

func TestLoadMergeOrderMatchesGit(t *testing.T) {
	ctx := context.Background()
	r := bigRepo(t, 24)
	for _, maxRows := range []int{0, 7} {
		h, err := Load(ctx, r, "main", maxRows)
		if err != nil {
			t.Fatal(err)
		}
		if h.Truncated != (maxRows > 0) {
			t.Errorf("maxRows=%d: truncated=%v", maxRows, h.Truncated)
		}
		for _, row := range h.Rows {
			if !row.Merge {
				continue
			}
			c := h.Commits[row.SHA]
			want, err := r.Lines(ctx, "rev-list", c.Parents[0]+".."+c.Parents[1])
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(row.Commits, want) {
				t.Errorf("%s: commits=%v, Git says %v", row.SHA, row.Commits, want)
			}
			for _, sha := range want {
				if h.Commits[sha].SHA != sha {
					t.Errorf("missing commit header: %s", sha)
				}
			}
		}
	}
}

func TestDetailScenarios(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)
	h, err := Load(ctx, r, "main", 0)
	if err != nil {
		t.Fatal(err)
	}
	merge := h.Commits[h.Rows[1].SHA]
	d, diffs, err := Detail(ctx, r, merge)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Scenarios) != 1 || d.Scenarios[0].Change != "added" || d.Scenarios[0].Feature != "Signing in" || len(d.Scenarios[0].Steps) != 3 {
		t.Errorf("merge scenarios = %+v", d.Scenarios)
	}
	if len(d.Files) != 1 || d.Files[0].Status != "A" || *d.Files[0].Added != 5 || !slices.Contains(d.Tags, "v0.1.0") {
		t.Errorf("merge files = %+v tags = %v", d.Files, d.Tags)
	}
	if fd := diffs["specs/login.feature"]; len(fd.Lines) == 0 || fd.Lines[0][:2] != "@@" {
		t.Errorf("diff = %+v", fd)
	}

	d, _, err = Detail(ctx, r, h.Commits[h.Rows[0].SHA])
	if err != nil {
		t.Fatal(err)
	}
	var changes []string
	for _, s := range d.Scenarios {
		changes = append(changes, s.Change+": "+s.Title)
	}
	want := []string{"changed: Signing in with a password", "added: Signing out"}
	if !slices.Equal(changes, want) {
		t.Errorf("scenario changes = %v, want %v", changes, want)
	}
	if len(d.Tags) != 0 {
		t.Errorf("a commit after the release is in %v", d.Tags)
	}
}

func TestMapLines(t *testing.T) {
	lines := []string{"@@ -2,2 +2,3 @@", " same", "-old", "+new", "+added", "\\ No newline at end of file"}
	older := []string{"o1", "o2", "o3"}
	newer := []string{"n1", "n2", "n3", "n4"}
	got := mapLines(lines, older, newer)
	want := []string{"", "n2", "o3", "n3", "n4", ""}
	if !slices.Equal(got, want) {
		t.Errorf("mapLines = %q, want %q", got, want)
	}
}

func TestDetailHighlights(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)
	head, err := r.Resolve(ctx, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	c, err := Commit(ctx, r, head)
	if err != nil {
		t.Fatal(err)
	}
	_, diffs, err := Detail(ctx, r, c)
	if err != nil {
		t.Fatal(err)
	}
	d := diffs["specs/login.feature"]
	if len(d.HL) != len(d.Lines) || !slices.ContainsFunc(d.HL, func(s string) bool { return strings.Contains(s, "k") }) {
		t.Errorf("feature diff not highlighted: lines %q hl %q", d.Lines, d.HL)
	}
}

// TestRealWorldConventions loads a clone of a repository made the way many projects are: lightweight
// version tags (GitHub's release button), other tags that aren't releases, component prefixes and
// capitalized types in commit subjects, and branches that exist only on the remote.
func TestRealWorldConventions(t *testing.T) {
	ctx := context.Background()
	origin := t.TempDir()
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git(origin, "init", "-q", "-b", "master")
	for _, s := range []string{"feat: routing", "middleware: preserve status (#1213)", "Fix: a typo", "README: links"} {
		git(origin, "commit", "-q", "--allow-empty", "-m", s)
	}
	git(origin, "tag", "v1.0.0", "HEAD~2")                     // lightweight, as GitHub's release button makes
	git(origin, "tag", "1.1.0", "HEAD~1")                      // without the v
	git(origin, "tag", "latest")                               // not a version
	git(origin, "tag", "middleware/v1.0.0")                    // one package's release
	git(origin, "tag", "-a", "stable", "-m", "Stable", "HEAD") // annotated: a release, as before
	git(origin, "tag", "-a", "v1", "-m", "v1.1.0", "HEAD~1")   // moves with the newest 1.x release
	git(origin, "tag", "-a", "v1.1", "-m", "v1.1.0", "HEAD~1") // and with the newest 1.1.x
	git(origin, "checkout", "-q", "-b", "feature/xff")
	git(origin, "commit", "-q", "--allow-empty", "-m", "middleware: faster XFF")
	git(origin, "checkout", "-q", "master")

	clone := t.TempDir()
	git(clone, "clone", "-q", origin, ".")
	git(clone, "checkout", "-q", "--detach") // as CI checks out: no local trunk branch
	git(clone, "branch", "-q", "-D", "master")

	h, err := Load(ctx, gitx.Repo{Dir: clone}, "master", 0)
	if err != nil {
		t.Fatal(err)
	}
	if h.Trunk != "origin/master" {
		t.Errorf("trunk read from %q, want origin/master", h.Trunk)
	}
	var names []string
	for _, tg := range h.Tags {
		names = append(names, tg.Name)
		if tg.Name == "v1.0.0" && (tg.Subject != "" || tg.Body != "") {
			t.Errorf("a lightweight tag took its commit's message: %q", tg.Subject)
		}
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"1.1.0", "stable", "v1.0.0"}) {
		t.Errorf("releases %v, want 1.1.0, stable, v1.0.0", names)
	}
	kinds := map[string]string{}
	for _, r := range h.Rows {
		kinds[r.Title] = r.Kind
	}
	for subject, want := range map[string]string{"feat: routing": "feat", "middleware: preserve status (#1213)": "", "Fix: a typo": "fix", "README: links": ""} {
		if kinds[subject] != want {
			t.Errorf("%q has kind %q, want %q", subject, kinds[subject], want)
		}
	}
	if len(h.Branches) != 1 || h.Branches[0].Name != "feature/xff" || len(h.Branches[0].Ahead) != 1 {
		t.Errorf("branches %+v, want feature/xff with 1 commit ahead", h.Branches)
	}
}
