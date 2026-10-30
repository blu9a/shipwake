// Package history reads a repository's history into Shipwake's model:
// the trunk as merges and direct commits, releases, open branches, and
// for each commit its files, diffs and scenario changes.
package history

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/blu9a/shipwake/internal/gherkin"
	"github.com/blu9a/shipwake/internal/gitx"
	"github.com/blu9a/shipwake/internal/model"
)

const (
	us = "\x1f" // field separator in git log output
	rs = "\x1e" // record separator
)

var (
	convRe = regexp.MustCompile(`^(\w+)(?:\([^)]*\))?!?:\s*(.+)$`)
	// A release's tag names a version: v1.2.3 or 1.2.3, a SemVer pre-release and build allowed. Tags
	// with a package prefix (middleware/v1.2.0, @scope/pkg@1.2.3) release one package, not the project.
	versionTagRe = regexp.MustCompile(`^v?\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
	// The types Conventional Commits defines, and spec for requirements. Any other prefix ("middleware:",
	// "README:") names a component: the commit has no kind.
	commitTypes = map[string]bool{"feat": true, "fix": true, "perf": true, "refactor": true, "docs": true, "test": true,
		"build": true, "ci": true, "chore": true, "style": true, "revert": true, "spec": true}
	// Branch prefixes as Git Flow and most teams name branches, mapped to the commit types.
	branchKinds = map[string]string{"feat": "feat", "feature": "feat", "features": "feat", "fix": "fix", "bugfix": "fix",
		"hotfix": "fix", "perf": "perf", "docs": "docs", "doc": "docs", "refactor": "refactor", "test": "test", "tests": "test",
		"chore": "chore", "ci": "ci", "build": "build", "style": "style", "revert": "revert", "spec": "spec"}
	// Common merge message formats. Group 1 is the branch; group 2, when present, the title.
	mergeRes = []*regexp.Regexp{
		regexp.MustCompile(`^Merge pull request #\d+ from [^/\s]+/(\S+)`),
		regexp.MustCompile(`^Merge (?:remote-tracking )?branch '([^']+)'(?: of \S+)?(?: into '?[^']+'?)?$`),
		regexp.MustCompile(`^Merge ([\w./-]+):\s*(.+)$`),
	}
)

// History is a repository's history as the timeline shows it.
type History struct {
	Truncated bool // older trunk history was left out
	Trunk     string
	Rows      []model.Row
	Tags      []model.Tag
	Branches  []model.Branch
	Commits   map[string]model.Commit
}

// Load reads the trunk's first-parent history (the newest maxRows entries; 0 for all), releases
// (annotated tags, and lightweight ones named as versions) and unmerged branches, local or remote.
func Load(ctx context.Context, r gitx.Repo, trunk string, maxRows int) (*History, error) {
	h := &History{Trunk: trunk, Commits: map[string]model.Commit{}}
	if _, err := r.Resolve(ctx, trunk); err != nil {
		// A CI checkout often has the trunk only as a remote branch.
		if _, rerr := r.Resolve(ctx, "origin/"+trunk); rerr != nil {
			return nil, fmt.Errorf("trunk %q: %w", trunk, err)
		}
		h.Trunk = "origin/" + trunk
	}
	args := []string{"--first-parent", h.Trunk}
	if maxRows > 0 {
		args = append(args, "-n", strconv.Itoa(maxRows+1))
	}
	rows, err := commits(ctx, r, args...)
	if err != nil {
		return nil, err
	}
	if maxRows > 0 && len(rows) > maxRows {
		rows, h.Truncated = rows[:maxRows], true
	}
	merged, err := loadMerges(ctx, r, rows)
	if err != nil {
		return nil, err
	}
	for i, c := range rows {
		h.add(c)
		row := model.Row{SHA: c.SHA, Merge: len(c.Parents) > 1, Title: c.Subject, Kind: c.Type}
		if row.Merge {
			own := merged[i]
			for _, o := range own {
				h.add(o)
				row.Commits = append(row.Commits, o.SHA)
			}
			row.Branch, row.Title = mergeTitle(c, own)
			row.Kind = kindOf(row.Branch, row.Title)
		}
		h.Rows = append(h.Rows, row)
	}
	if h.Tags, err = tags(ctx, r); err != nil {
		return nil, err
	}
	if h.Branches, err = h.branches(ctx, r); err != nil {
		return nil, err
	}
	return h, nil
}

// Each range still uses Git's own history walk, including its ordering. Read independent
// merges concurrently, capped at eight Git processes; assemble rows on the caller in order.
func loadMerges(ctx context.Context, r gitx.Repo, rows []model.Commit) ([][]model.Commit, error) {
	merged := make([][]model.Commit, len(rows))
	errs := make([]error, len(rows))
	slots := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i, c := range rows {
		if len(c.Parents) < 2 {
			continue
		}
		slots <- struct{}{}
		wg.Go(func() {
			defer func() { <-slots }()
			merged[i], errs[i] = commits(ctx, r, c.Parents[0]+".."+c.Parents[1])
		})
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return merged, nil
}

func (h *History) add(c model.Commit) { h.Commits[c.SHA] = c }

// commits runs git log with a parseable format.
func commits(ctx context.Context, r gitx.Repo, args ...string) ([]model.Commit, error) {
	out, err := r.Git(ctx, append([]string{"log", "--format=%H" + us + "%h" + us + "%P" + us + "%cI" + us + "%an" + us + "%s" + us + "%b" + rs}, args...)...)
	if err != nil {
		return nil, err
	}
	var res []model.Commit
	for _, rec := range strings.Split(out, rs) {
		rec = strings.Trim(rec, "\n")
		if rec == "" {
			continue
		}
		f := strings.SplitN(rec, us, 7)
		if len(f) < 7 {
			continue
		}
		date, _ := time.Parse(time.RFC3339, f[3])
		c := model.Commit{SHA: f[0], Short: f[1], Parents: strings.Fields(f[2]), Date: date, Author: f[4], Subject: f[5], Body: strings.TrimSpace(f[6])}
		c.Type = typeOf(c.Subject)
		res = append(res, c)
	}
	return res, nil
}

// mergeTitle finds the merged branch and a readable title for a merge.
func mergeTitle(c model.Commit, own []model.Commit) (branch, title string) {
	for _, re := range mergeRes {
		if m := re.FindStringSubmatch(c.Subject); m != nil {
			branch = m[1]
			if len(m) > 2 && m[2] != "" {
				return branch, m[2]
			}
			if c.Body != "" {
				return branch, strings.SplitN(c.Body, "\n", 2)[0]
			}
			break
		}
	}
	for i := len(own) - 1; i >= 0; i-- { // the branch's first own commit
		if len(own[i].Parents) < 2 {
			return branch, own[i].Subject
		}
	}
	return branch, c.Subject
}

// typeOf is a commit subject's Conventional Commits type, if it has one of the defined ones.
func typeOf(subject string) string {
	m := convRe.FindStringSubmatch(subject)
	if m == nil {
		return ""
	}
	if t := strings.ToLower(m[1]); commitTypes[t] {
		return t
	}
	return ""
}

// kindOf is what a merge changed: from its branch's prefix (feature/…, hotfix/…), else from its title
// when that follows Conventional Commits (squash and pull request titles often do).
func kindOf(branch, title string) string {
	if i := strings.Index(branch, "/"); i > 0 {
		if k := branchKinds[strings.ToLower(branch[:i])]; k != "" {
			return k
		}
	}
	return typeOf(title)
}

func tags(ctx context.Context, r gitx.Repo) ([]model.Tag, error) {
	out, err := r.Git(ctx, "for-each-ref", "refs/tags", "--sort=-creatordate",
		"--format=%(refname:short)"+us+"%(objecttype)"+us+"%(creatordate:iso-strict)"+us+"%(contents:subject)"+us+"%(contents:body)"+rs)
	if err != nil {
		return nil, err
	}
	var res []model.Tag
	for _, rec := range strings.Split(out, rs) {
		rec = strings.Trim(rec, "\n")
		if rec == "" {
			continue
		}
		f := strings.SplitN(rec, us, 5)
		if len(f) < 5 {
			continue
		}
		// A release is an annotated tag, or a lightweight one named as a version, as GitHub's release
		// button and many projects make them. A lightweight tag has no message of its own.
		annotated := f[1] == "tag"
		if !annotated && !versionTagRe.MatchString(f[0]) {
			continue
		}
		sha, err := r.Resolve(ctx, f[0])
		if err != nil {
			continue
		}
		date, _ := time.Parse(time.RFC3339, f[2]) // a lightweight tag's: its commit's
		t := model.Tag{Name: f[0], SHA: sha, Date: date}
		if annotated {
			t.Subject, t.Body = f[3], strings.TrimSpace(f[4])
		}
		res = append(res, t)
	}
	return res, nil
}

// branchRefs are the branches to show, by name: local ones, and those only on a remote, as a fresh
// clone or a CI checkout has them (origin first). The remote's HEAD and the trunk are left out.
func branchRefs(ctx context.Context, r gitx.Repo, trunk string) ([][2]string, error) {
	refs, err := r.Lines(ctx, "for-each-ref", "refs/heads", "refs/remotes", "--format=%(refname)")
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(refs, func(a, b string) int { // local, then origin, then other remotes
		rank := func(s string) int {
			switch {
			case strings.HasPrefix(s, "refs/heads/"):
				return 0
			case strings.HasPrefix(s, "refs/remotes/origin/"):
				return 1
			}
			return 2
		}
		return rank(a) - rank(b)
	})
	seen := map[string]bool{trunk: true, strings.TrimPrefix(trunk, "origin/"): true}
	var res [][2]string // name as people know it, ref for Git
	for _, ref := range refs {
		var name string
		switch {
		case strings.HasPrefix(ref, "refs/heads/"):
			name = strings.TrimPrefix(ref, "refs/heads/")
		case strings.HasPrefix(ref, "refs/remotes/"):
			_, rest, ok := strings.Cut(strings.TrimPrefix(ref, "refs/remotes/"), "/")
			if !ok || rest == "HEAD" {
				continue
			}
			name = rest
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		res = append(res, [2]string{name, ref})
	}
	return res, nil
}

func (h *History) branches(ctx context.Context, r gitx.Repo) ([]model.Branch, error) {
	named, err := branchRefs(ctx, r, h.Trunk)
	if err != nil {
		return nil, err
	}
	var res []model.Branch
	for _, nr := range named {
		name, ref := nr[0], nr[1]
		ahead, err := commits(ctx, r, h.Trunk+".."+ref)
		if err != nil || len(ahead) == 0 {
			continue // merged, or unreadable
		}
		b := model.Branch{Name: name, Updated: ahead[0].Date}
		for _, c := range ahead {
			b.Ahead = append(b.Ahead, c.SHA)
		}
		if n, err := r.Git(ctx, "rev-list", "--count", ref+".."+h.Trunk); err == nil {
			b.Behind, _ = strconv.Atoi(strings.TrimSpace(n))
		}
		base, err := r.Git(ctx, "merge-base", h.Trunk, ref)
		if err != nil {
			continue // unrelated history, such as the results branch
		}
		b.Base = strings.TrimSpace(base)
		if bc, err := commits(ctx, r, "-1", b.Base); err == nil && len(bc) == 1 {
			h.add(bc[0])
		}
		for _, c := range ahead {
			h.add(c)
		}
		res = append(res, b)
	}
	slices.SortFunc(res, func(a, b model.Branch) int { return b.Updated.Compare(a.Updated) })
	return res, nil
}

// MaxDiffLines is how many lines of one file's patch a commit page includes.
const MaxDiffLines = 400

// Detail reads a commit's files, patches, scenario changes and the releases that contain it.
func Detail(ctx context.Context, r gitx.Repo, c model.Commit) (model.CommitDetail, map[string]model.FileDiff, error) {
	d := model.CommitDetail{SHA: c.SHA, Files: []model.FileChange{}, Scenarios: []model.ScenarioChange{}, Tags: []string{}}
	base := gitx.EmptyTree
	if len(c.Parents) > 0 {
		base = c.Parents[0]
	}
	status := map[string]model.FileChange{}
	names, err := r.Lines(ctx, "diff", "-M", "--name-status", base, c.SHA)
	if err != nil {
		return d, nil, err
	}
	for _, line := range names {
		p := strings.Split(line, "\t")
		fc := model.FileChange{Status: p[0][:1], Path: p[len(p)-1]}
		if fc.Status == "R" || fc.Status == "C" {
			fc.Old = p[1]
		}
		status[fc.Path] = fc
	}
	nums, err := r.Git(ctx, "diff", "-M", "--numstat", "-z", base, c.SHA)
	if err != nil {
		return d, nil, err
	}
	d.Files = append(d.Files, numstat(nums, status)...)
	patch, err := r.Git(ctx, "diff", "-M", base, c.SHA)
	if err != nil {
		return d, nil, err
	}
	diffs := splitPatch(patch)
	highlightDiffs(ctx, r, base, c.SHA, d.Files, diffs)
	if t, err := r.Lines(ctx, "tag", "--contains", c.SHA, "--sort=creatordate"); err == nil {
		d.Tags = append(d.Tags, t...)
	}
	d.Scenarios = scenarioChanges(ctx, r, base, c.SHA, d.Files)
	return d, diffs, nil
}

// numstat pairs --numstat -z output with the statuses, keeping git's file order.
func numstat(out string, status map[string]model.FileChange) []model.FileChange {
	var res []model.FileChange
	// With -z, records are "added\tdeleted\tpath\0" or, for renames, "added\tdeleted\t\0old\0new\0".
	fields := strings.Split(out, "\x00")
	for i := 0; i < len(fields); i++ {
		rec := strings.TrimPrefix(fields[i], "\n")
		if rec == "" {
			continue
		}
		p := strings.SplitN(rec, "\t", 3)
		if len(p) < 3 {
			continue
		}
		path := p[2]
		if path == "" && i+2 < len(fields) { // rename: old and new follow
			path = fields[i+2]
			i += 2
		}
		fc, ok := status[path]
		if !ok {
			fc = model.FileChange{Path: path, Status: "M"}
		}
		if p[0] != "-" {
			a, _ := strconv.Atoi(p[0])
			del, _ := strconv.Atoi(p[1])
			fc.Added, fc.Deleted = &a, &del
		}
		res = append(res, fc)
	}
	return res
}

// splitPatch splits a unified diff into per-file hunks, shortening long ones.
func splitPatch(patch string) map[string]model.FileDiff {
	res := map[string]model.FileDiff{}
	var cur string
	var lines []string
	flush := func() {
		if cur == "" {
			return
		}
		start := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, "@@") })
		switch {
		case slices.ContainsFunc(lines, func(l string) bool { return strings.HasPrefix(l, "Binary files ") }):
			res[cur] = model.FileDiff{Binary: true}
		case start < 0:
			res[cur] = model.FileDiff{}
		case len(lines)-start > 1500:
			res[cur] = model.FileDiff{Skipped: len(lines) - start}
		default:
			body := lines[start:]
			if len(body) > 0 && body[len(body)-1] == "" {
				body = body[:len(body)-1]
			}
			fd := model.FileDiff{Lines: body}
			if len(body) > MaxDiffLines {
				fd = model.FileDiff{Lines: body[:MaxDiffLines], More: len(body) - MaxDiffLines}
			}
			res[cur] = fd
		}
	}
	for _, l := range strings.Split(patch, "\n") {
		if strings.HasPrefix(l, "diff --git ") {
			flush()
			if i := strings.LastIndex(l, " b/"); i >= 0 {
				cur = l[i+3:]
			}
			lines = nil
			continue
		}
		lines = append(lines, l)
	}
	flush()
	return res
}

// scenarioChanges compares scenarios in .feature files before and after a commit.
func scenarioChanges(ctx context.Context, r gitx.Repo, base, sha string, files []model.FileChange) []model.ScenarioChange {
	res := []model.ScenarioChange{}
	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".feature") {
			continue
		}
		var before, after gherkin.Feature
		if f.Status != "A" && base != gitx.EmptyTree {
			old := f.Path
			if f.Old != "" {
				old = f.Old
			}
			if text, err := r.Git(ctx, "show", base+":"+old); err == nil {
				before = gherkin.Parse(text)
			}
		}
		if f.Status != "D" {
			if text, err := r.Git(ctx, "show", sha+":"+f.Path); err == nil {
				after = gherkin.Parse(text)
			}
		}
		feature := after.Name
		if feature == "" {
			feature = before.Name
		}
		for _, name := range after.Order {
			steps := after.Scenarios[name]
			was, existed := before.Scenarios[name]
			switch {
			case !existed:
				res = append(res, model.ScenarioChange{Feature: feature, Title: name, Change: "added", Steps: steps})
			case !slices.Equal(was, steps):
				res = append(res, model.ScenarioChange{Feature: feature, Title: name, Change: "changed", Steps: steps, Was: was})
			}
		}
		for _, name := range before.Order {
			if _, still := after.Scenarios[name]; !still {
				res = append(res, model.ScenarioChange{Feature: feature, Title: name, Change: "removed", Steps: before.Scenarios[name]})
			}
		}
	}
	return res
}

// Commit reads one commit's header.
func Commit(ctx context.Context, r gitx.Repo, sha string) (model.Commit, error) {
	cs, err := commits(ctx, r, "-1", sha)
	if err != nil {
		return model.Commit{}, err
	}
	if len(cs) == 0 {
		return model.Commit{}, fmt.Errorf("commit %s not found", sha)
	}
	return cs[0], nil
}
