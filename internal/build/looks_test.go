package build

import (
	"context"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blu9a/shipwake/internal/gitx"
	"github.com/blu9a/shipwake/internal/looks"
	"github.com/blu9a/shipwake/internal/model"
	"github.com/blu9a/shipwake/internal/store"
)

// lookRepo is a repository whose runs record screenshots, kept on its results branch.
type lookRepo struct {
	t    *testing.T
	ctx  context.Context
	repo gitx.Repo
	n    int
}

func newLookRepo(t *testing.T) *lookRepo {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	r := &lookRepo{t: t, ctx: context.Background(), repo: gitx.Repo{Dir: t.TempDir()}}
	r.git("init", "-q", "-b", "main")
	r.git("config", "user.name", "Ana")
	r.git("config", "user.email", "ana@example.com")
	return r
}

func (r *lookRepo) git(args ...string) string {
	r.t.Helper()
	out, err := r.repo.Git(r.ctx, args...)
	if err != nil {
		r.t.Fatal(err)
	}
	return strings.TrimSpace(out)
}

func (r *lookRepo) commit(msg string) string {
	r.git("commit", "-q", "--allow-empty", "-m", msg)
	return r.git("rev-parse", "HEAD")
}

// record saves a run on commit with one screenshot per name, each a flat image of its color.
func (r *lookRepo) record(commit string, shots map[string]color.NRGBA) {
	r.t.Helper()
	r.n++
	src := r.t.TempDir()
	test := model.Test{Name: "screens", Status: "passed"}
	for name, c := range shots {
		img := image.NewNRGBA(image.Rect(0, 0, 24, 12))
		for i := 0; i < len(img.Pix); i += 4 {
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
		}
		data, err := looks.EncodePNG(img)
		if err != nil {
			r.t.Fatal(err)
		}
		subject, variant, _ := looks.Parse(name)
		file := looks.ID(subject, variant) + ".png"
		if err := os.WriteFile(filepath.Join(src, file), data, 0o644); err != nil {
			r.t.Fatal(err)
		}
		test.Attachments = append(test.Attachments, model.Attachment{Name: name, Type: "image/png", Path: file})
	}
	start := time.Date(2026, 1, 1, 0, r.n, 0, 0, time.UTC)
	run := model.Run{ID: store.NewID(start), Kind: "test", Name: "Screens", Commit: commit, Start: start, Tests: []model.Test{test}}
	dir, done, err := store.Checkout(r.ctx, r.repo, "shipwake-results")
	if err != nil {
		r.t.Fatal(err)
	}
	if err := (store.Store{Dir: dir}).Save(&run, src); err != nil {
		r.t.Fatal(err)
	}
	if err := done("test", false); err != nil {
		r.t.Fatal(err)
	}
}

// review loads the project again and compares the commit's looks: status by name.
func (r *lookRepo) review(commit string) map[string]model.Look {
	r.t.Helper()
	l, err := Load(r.ctx, Project{Dir: r.repo.Dir}, -1, func(string, ...any) {})
	if err != nil {
		r.t.Fatal(err)
	}
	defer l.Close()
	ls, _, err := l.ReviewLooks(r.ctx, commit)
	if err != nil {
		r.t.Fatal(err)
	}
	res := map[string]model.Look{}
	for _, lk := range ls.Looks {
		res[lk.Name] = lk
	}
	return res
}

func (r *lookRepo) approve(commit string, names ...string) {
	r.t.Helper()
	if _, err := ApproveLooks(r.ctx, Project{Dir: r.repo.Dir}, commit, names, false, "Ana <ana@example.com>", false); err != nil {
		r.t.Fatal(err)
	}
}

var (
	blue  = color.NRGBA{R: 0x25, G: 0x63, B: 0xEB, A: 0xFF}
	green = color.NRGBA{R: 0x16, G: 0xA3, B: 0x4A, A: 0xFF}
	grey  = color.NRGBA{R: 0xF4, G: 0xF4, B: 0xF5, A: 0xFF}
)

func TestLooksApproval(t *testing.T) {
	r := newLookRepo(t)
	first := r.commit("feat: a button")
	r.record(first, map[string]color.NRGBA{"look: Button [light]": blue, "look: Button [dark]": blue})

	got := r.review(first)
	if got["Button [light]"].Status != "new" || got["Button [dark]"].Status != "new" {
		t.Fatalf("screenshots nobody approved are new: %+v", got)
	}
	r.approve(first, "Button") // every variant of the subject
	got = r.review(first)
	if lk := got["Button [light]"]; lk.Status != "same" || lk.Approved == nil || lk.Approved.By != "Ana <ana@example.com>" {
		t.Fatalf("an approved look is the baseline, with its approver: %+v", lk)
	}
	if msg := r.git("log", "-1", "--format=%B", "shipwake-results"); !strings.Contains(msg, "Approved-by: Ana <ana@example.com>") ||
		!strings.HasPrefix(msg, "look: approve 2 looks on ") {
		t.Errorf("the approval is a commit with Approved-by on the results branch:\n%s", msg)
	}

	// A later change that recolors the dark button: only that one differs.
	second := r.commit("feat: a green dark button")
	r.record(second, map[string]color.NRGBA{"look: Button [light]": blue, "look: Button [dark]": green, "look: Card": grey})
	got = r.review(second)
	if got["Button [light]"].Status != "same" || got["Button [dark]"].Status != "changed" || got["Card"].Status != "new" {
		t.Fatalf("statuses: %+v", got)
	}
	if lk := got["Button [dark]"]; lk.Pixels != 24*12 || lk.Diff == "" || lk.Approved.Commit != first {
		t.Errorf("a changed look counts its pixels, has a diff and names its approval: %+v", lk)
	}

	// Approving it makes it what later commits are compared with.
	r.approve(second, "Button [dark]", "Card")
	third := r.commit("docs: no visible change")
	r.record(third, map[string]color.NRGBA{"look: Button [light]": blue, "look: Button [dark]": green, "look: Card": grey})
	for name, lk := range r.review(third) {
		if lk.Status != "same" {
			t.Errorf("%s: %s, want same as approved on the commit before", name, lk.Status)
		}
	}

	// A branch that left before the approval still compares with the look approved before it.
	r.git("checkout", "-q", "-b", "old", first)
	side := r.commit("feat: elsewhere")
	r.record(side, map[string]color.NRGBA{"look: Button [dark]": green})
	if lk := r.review(side)["Button [dark]"]; lk.Status != "changed" || lk.Approved.Commit != first {
		t.Errorf("an approval applies to its commit and the ones after it, not to other branches: %+v", lk)
	}
}

func TestApproveLooksRefuses(t *testing.T) {
	r := newLookRepo(t)
	c := r.commit("feat: a button")
	r.record(c, map[string]color.NRGBA{"look: Button": blue})
	for _, names := range [][]string{{"Card"}, nil} {
		if _, err := ApproveLooks(r.ctx, Project{Dir: r.repo.Dir}, c, names, false, "Ana <ana@example.com>", false); err == nil {
			t.Errorf("approving %v should fail", names)
		}
	}
	if _, err := ApproveLooks(r.ctx, Project{Dir: r.repo.Dir}, c, nil, true, " <>", false); err == nil {
		t.Error("an approval needs the approver's name")
	}
	r.approve(c, "Button")
	if _, err := ApproveLooks(r.ctx, Project{Dir: r.repo.Dir}, c, nil, true, "Ana <ana@example.com>", false); err == nil {
		t.Error("nothing left to approve should say so")
	}
}

func TestWriteLooks(t *testing.T) {
	r := newLookRepo(t)
	c := r.commit("feat: a button")
	r.record(c, map[string]color.NRGBA{"look: Button": blue})
	r.approve(c, "Button")
	d := r.commit("feat: a green button")
	r.record(d, map[string]color.NRGBA{"look: Button": green})
	out := t.TempDir()
	if err := Run(r.ctx, Options{Projects: []Project{{Dir: r.repo.Dir}}, Out: out}); err != nil {
		t.Fatal(err)
	}
	id := filepath.Base(r.repo.Dir)
	id = strings.Trim(idRe.ReplaceAllString(strings.ToLower(id), "-"), "-")
	data, err := os.ReadFile(filepath.Join(out, "data", id, "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"looks":{"`) || !strings.Contains(string(data), `"changed":["Button"]`) {
		t.Errorf("the index tallies each commit's looks: %s", data)
	}
	diffs, _ := filepath.Glob(filepath.Join(out, "data", id, "looks", "diffs", "*.png"))
	if len(diffs) != 1 {
		t.Errorf("one diff image, got %v", diffs)
	}
	if _, err := os.Stat(filepath.Join(out, "data", id, "looks", d+".json")); err != nil {
		t.Error(err)
	}
}
