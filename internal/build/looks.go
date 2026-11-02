package build

import (
	"cmp"
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/blu9a/shipwake/internal/gitx"
	"github.com/blu9a/shipwake/internal/looks"
	"github.com/blu9a/shipwake/internal/model"
	"github.com/blu9a/shipwake/internal/store"
)

// Decisive returns the runs that judge a commit: of each series (runs by name) recorded on it, the
// newest, preferring one that can be compared (a run on the series' runner) over a newer one that can't,
// so a run on another machine never hides a regression. Sorted by name. `shipwake check` and the release
// page judge with these, so both give one verdict.
func (l *Loaded) Decisive(sha string) []*model.Run {
	comparable := map[string]bool{}
	for _, s := range l.Index.Runs {
		comparable[s.ID] = s.Comparable
	}
	latest := map[string]*model.Run{}
	for _, r := range l.Runs {
		if r.Commit != sha {
			continue
		}
		o, ok := latest[r.Name]
		if !ok || comparable[r.ID] && !comparable[o.ID] || comparable[r.ID] == comparable[o.ID] && r.Start.After(o.Start) {
			latest[r.Name] = r
		}
	}
	res := make([]*model.Run, 0, len(latest))
	for _, r := range latest {
		res = append(res, r)
	}
	slices.SortFunc(res, func(a, b *model.Run) int { return strings.Compare(a.Name, b.Name) })
	return res
}

// applies reports whether an approval made on commit applies to sha: the same commit or one before it.
func (l *Loaded) applies(ctx context.Context, commit, sha string) bool {
	if commit == sha {
		return true
	}
	key := [2]string{commit, sha}
	if v, ok := l.ancestors[key]; ok {
		return v
	}
	_, err := l.Repo.Git(ctx, "merge-base", "--is-ancestor", commit, sha)
	if l.ancestors == nil {
		l.ancestors = map[[2]string]bool{}
	}
	l.ancestors[key] = err == nil
	return err == nil
}

// lookCache keeps decoded screenshots and comparisons across a build's commits.
type lookCache struct {
	images map[string]looks.Image // by file
	diffs  map[[2]string]looks.Result
}

func (c *lookCache) load(path string) (looks.Image, error) {
	if img, ok := c.images[path]; ok {
		return img, nil
	}
	img, err := looks.Load(path)
	if err == nil {
		c.images[path] = img
	}
	return img, err
}

// ReviewLooks compares a commit's screenshots with the looks approved for them.
func (l *Loaded) ReviewLooks(ctx context.Context, sha string) (model.Looks, map[string]*image.NRGBA, error) {
	return l.reviewLooks(ctx, sha, &lookCache{images: map[string]looks.Image{}, diffs: map[[2]string]looks.Result{}})
}

// reviewLooks is ReviewLooks with a cache; it also gives the diff images, by file name.
func (l *Loaded) reviewLooks(ctx context.Context, sha string, c *lookCache) (model.Looks, map[string]*image.NRGBA, error) {
	res := model.Looks{Commit: sha, Looks: []model.Look{}}
	diffs := map[string]*image.NRGBA{}
	file := func(run, path string) string { return filepath.Join(l.Files[run], filepath.FromSlash(path)) }
	for _, s := range looks.Collect(l.Decisive(sha)) {
		now, err := c.load(file(s.Run, s.Path))
		if err != nil {
			return res, nil, fmt.Errorf("look %s: %w", looks.Name(s.Subject, s.Variant), err)
		}
		lk := model.Look{ID: looks.ID(s.Subject, s.Variant), Name: looks.Name(s.Subject, s.Variant), Subject: s.Subject, Variant: s.Variant,
			Status: "new", Run: s.Run, Path: s.Path, Hash: now.Hash, Width: now.Img.Rect.Dx(), Height: now.Img.Rect.Dy()}
		base := looks.Baseline(l.Approvals, s.Subject, s.Variant, func(a model.LookApproval) bool {
			if _, ok := l.Files[a.Run]; !ok {
				return false // its run isn't read (its commit is gone): nothing to compare with
			}
			return l.applies(ctx, a.Commit, sha)
		})
		if base != nil {
			lk.Approved, lk.Status = base, "same"
			if base.Hash != now.Hash {
				key := [2]string{now.Hash, base.Hash}
				r, ok := c.diffs[key]
				if !ok {
					was, err := c.load(file(base.Run, base.Path))
					if err != nil {
						return res, nil, fmt.Errorf("approved look %s: %w", lk.Name, err)
					}
					r = looks.Compare(now.Img, was.Img)
					c.diffs[key] = r
				}
				if r.Pixels > 0 {
					lk.Status, lk.Pixels = "changed", r.Pixels
					lk.Diff = now.Hash[:16] + "-" + base.Hash[:16] + ".png"
					diffs[lk.Diff] = r.Diff
				}
			}
		}
		res.Looks = append(res.Looks, lk)
	}
	return res, diffs, nil
}

// Tally sums up a commit's looks for the index.
func Tally(ls model.Looks) model.LookTally {
	var t model.LookTally
	for _, lk := range ls.Looks {
		switch lk.Status {
		case "same":
			t.Same++
		case "changed":
			t.Changed = append(t.Changed, lk.Name)
		default:
			t.New = append(t.New, lk.Name)
		}
	}
	return t
}

// writeLooks writes the visual review of every commit whose runs recorded screenshots: looks/<sha>.json,
// the diff images in looks/diffs/, and the tally in the index.
func writeLooks(ctx context.Context, l *Loaded, dir string) error {
	commits := map[string]bool{}
	for _, r := range l.Runs {
		if len(looks.Collect([]*model.Run{r})) > 0 {
			commits[r.Commit] = true
		}
	}
	shas := make([]string, 0, len(commits))
	for sha := range commits {
		shas = append(shas, sha)
	}
	slices.Sort(shas)
	c := &lookCache{images: map[string]looks.Image{}, diffs: map[[2]string]looks.Result{}}
	written := map[string]bool{}
	for _, sha := range shas {
		ls, diffs, err := l.reviewLooks(ctx, sha, c)
		if err != nil {
			return fmt.Errorf("commit %.7s: %w", sha, err)
		}
		if len(ls.Looks) == 0 {
			continue // its screenshots are in a run that no longer decides
		}
		for name, img := range diffs {
			if written[name] {
				continue
			}
			data, err := looks.EncodePNG(img)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Join(dir, "looks", "diffs"), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(dir, "looks", "diffs", name), data, 0o644); err != nil {
				return err
			}
			written[name] = true
		}
		if err := writeJSON(filepath.Join(dir, "looks", sha+".json"), ls); err != nil {
			return err
		}
		if l.Index.Looks == nil {
			l.Index.Looks = map[string]model.LookTally{}
		}
		l.Index.Looks[sha] = Tally(ls)
	}
	return nil
}

// ResultsOf is where a project keeps its runs: a store location ("git:<branch>" or a directory) and the
// repository whose branch it is.
func ResultsOf(p Project) (spec string, repo gitx.Repo, err error) {
	top, err := gitx.Repo{Dir: p.Dir}.Git(context.Background(), "rev-parse", "--show-toplevel")
	if err != nil {
		return "", gitx.Repo{}, err
	}
	specDir := cmp.Or(p.Spec, strings.TrimSpace(top))
	spec = p.Results
	if spec == "" {
		cfg, err := LoadConfig(specDir)
		if err != nil {
			return "", gitx.Repo{}, err
		}
		spec = cfg.Results
	}
	return cmp.Or(spec, store.DefaultBranch), gitx.Repo{Dir: cmp.Or(p.ResultsRepo, specDir)}, nil
}

// ApproveLooks approves how a commit's screenshots look: those that changed or are new, of the ones
// named (by name, such as "Button [dark]", by subject for all its variants, or by ID), or all of them.
// by is the approver ("Name <email>"). The approvals are kept with the runs; on a results branch they
// are committed with Approved-by, and push sends the branch to origin (never done otherwise).
func ApproveLooks(ctx context.Context, p Project, commit string, names []string, all bool, by string, push bool) ([]model.LookApproval, error) {
	if strings.TrimSpace(by) == "" || strings.HasPrefix(strings.TrimSpace(by), "<") {
		return nil, fmt.Errorf("set your name in Git first: git config user.name")
	}
	l, err := Load(ctx, p, -1, func(string, ...any) {})
	if err != nil {
		return nil, err
	}
	defer l.Close()
	sha, err := l.Repo.Resolve(ctx, commit)
	if err != nil {
		return nil, err
	}
	ls, _, err := l.ReviewLooks(ctx, sha)
	if err != nil {
		return nil, err
	}
	if len(ls.Looks) == 0 {
		return nil, fmt.Errorf("no screenshots are recorded for %.7s", sha)
	}
	var picked []model.Look
	for _, n := range names {
		found := false
		for _, lk := range ls.Looks {
			if n == lk.Name || n == lk.Subject || n == lk.ID {
				found = true
				if lk.Status != "same" && !slices.ContainsFunc(picked, func(x model.Look) bool { return x.ID == lk.ID }) {
					picked = append(picked, lk)
				}
			}
		}
		if !found {
			return nil, fmt.Errorf("no screenshot %q on %.7s", n, sha)
		}
	}
	if all {
		for _, lk := range ls.Looks {
			if lk.Status != "same" && !slices.ContainsFunc(picked, func(x model.Look) bool { return x.ID == lk.ID }) {
				picked = append(picked, lk)
			}
		}
	}
	if len(picked) == 0 {
		return nil, fmt.Errorf("nothing to approve on %.7s: its screenshots look as approved", sha)
	}
	now := time.Now().UTC()
	approvals := make([]model.LookApproval, 0, len(picked))
	for i, lk := range picked {
		approvals = append(approvals, model.LookApproval{ID: store.NewID(now.Add(time.Duration(i))), Subject: lk.Subject, Variant: lk.Variant,
			Commit: sha, Run: lk.Run, Path: lk.Path, Hash: lk.Hash, Width: lk.Width, Height: lk.Height, By: strings.TrimSpace(by), At: now})
	}
	spec, repo, err := ResultsOf(p)
	if err != nil {
		return nil, err
	}
	save := func(dir string) error {
		for _, a := range approvals {
			if err := (store.Store{Dir: dir}).SaveApproval(a); err != nil {
				return err
			}
		}
		return nil
	}
	branch := store.Branch(spec)
	if branch == "" {
		return approvals, save(spec)
	}
	dir, done, err := store.Checkout(ctx, repo, branch)
	if err != nil {
		return nil, err
	}
	if err := save(dir); err != nil {
		_ = done("", false)
		return nil, err
	}
	subject := fmt.Sprintf("look: approve %q on %.7s", picked[0].Name, sha)
	body := ""
	if len(picked) > 1 {
		subject = fmt.Sprintf("look: approve %d looks on %.7s", len(picked), sha)
		for _, lk := range picked {
			body += "- " + lk.Name + "\n"
		}
		body += "\n"
	}
	return approvals, done(subject+"\n\n"+body+"Approved-by: "+strings.TrimSpace(by), push)
}
