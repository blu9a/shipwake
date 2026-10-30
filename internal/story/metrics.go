package story

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/blu9a/shipwake/internal/gitx"
	"github.com/blu9a/shipwake/internal/metricspec"
	"github.com/blu9a/shipwake/internal/model"
)

// Metric is a measured requirement's story: a .metric file saying what it measures, on which
// dataset, how it is graded and when it blocks; then the runs that met it, and missed it.
//
//	proposed   the file first appears tagged @proposed
//	changed    its settings or description change in a later commit
//	approved   @proposed is removed; Approved-by names who approved (else the commit's author)
//	met        the first run within its thresholds, for each approved version
//	missed     a run outside them after that
type Metric struct {
	ID          string            `json:"id"`
	Path        string            `json:"path"`
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Dataset     string            `json:"dataset,omitempty"`
	Grader      string            `json:"grader,omitempty"`
	Definition  []string          `json:"definition"` // the file's sentences after its title, as written
	Lines       []metricspec.Line `json:"lines"`      // the latest file, each line with what it was read as
	State       string            `json:"state"`      // proposed, approved, met, missed, removed
	Events      []Event           `json:"events"`
}

// BuildMetrics reads every commit in rev's history that changed a .metric file, oldest first, then
// the runs. Runs must already be judged: a metric is met when its comparison's status is ok.
func BuildMetrics(ctx context.Context, src Source, runs []*model.Run) ([]*Metric, error) {
	r, rev := src.Repo, src.Rev
	const sep = "\x1f"
	out, err := r.Git(ctx, "log", "--reverse", "--format=%H"+sep+"%P"+sep+"%an"+sep+"%cI"+sep+"%s"+sep+
		"%(trailers:key=Approved-by,valueonly,separator=%x2C)"+"\x1e", rev, "--", "*.metric")
	if err != nil {
		return nil, err
	}
	metrics := map[string]*Metric{}
	var order []string
	for _, rec := range strings.Split(out, "\x1e") {
		p := strings.Split(strings.TrimSpace(rec), sep)
		if len(p) < 6 {
			continue
		}
		c := commit{sha: p[0], author: p[2], subject: p[4], approvedBy: strings.TrimSpace(p[5])}
		if parents := strings.Fields(p[1]); len(parents) > 0 {
			c.parent = parents[0]
		}
		c.at, _ = time.Parse(time.RFC3339, p[3])
		paths, err := r.Lines(ctx, "diff-tree", "--no-commit-id", "--name-only", "-r", "--root", c.sha, "--", "*.metric")
		if err != nil {
			return nil, err
		}
		for _, path := range paths {
			before, existed := specAt(ctx, r, c.parent, path)
			after, exists := specAt(ctx, r, c.sha, path)
			m, ok := metrics[path]
			if !ok {
				m = &Metric{Path: path}
				metrics[path] = m
				order = append(order, path)
			}
			base := Event{At: c.at, Commit: c.sha, Subject: c.subject, By: c.author}
			if !exists {
				if existed {
					ev := base
					ev.Kind, ev.Was = "removed", definition(before)
					m.Events = append(m.Events, ev)
					m.State = "removed"
				}
				continue
			}
			m.ID, m.Name, m.Description, m.Dataset, m.Grader = after.ID, after.Name, after.Description, after.Dataset, after.Grader
			lines := definition(after)
			ev := base
			ev.Steps = lines
			switch {
			case !existed && after.Proposed():
				ev.Kind, m.State = "proposed", "proposed"
			case !existed:
				ev.Kind, m.State, ev.Approved = "added", "approved", true
			case !slices.Equal(definition(before), lines):
				ev.Kind, ev.Was = "changed", definition(before)
			}
			if ev.Kind != "" {
				ev.Version = metricVersions(m) + 1
				m.Events = append(m.Events, ev)
			}
			switch {
			case existed && before.Proposed() && !after.Proposed():
				ap := base
				ap.Kind, ap.Steps, ap.Approved, ap.Version = "approved", lines, true, metricVersions(m)
				if c.approvedBy != "" {
					ap.By, _, _ = strings.Cut(c.approvedBy, " <")
				}
				m.Events = append(m.Events, ap)
				m.State = "approved"
			case existed && after.Proposed() && !before.Proposed():
				m.State = "proposed" // proposed again: a change waits for approval
			}
			m.Definition, m.Lines = lines, after.Lines
		}
	}
	res := make([]*Metric, 0, len(order))
	for _, path := range order {
		m := metrics[path]
		attachMetricRuns(m, runs)
		res = append(res, m)
	}
	return res, nil
}

func specAt(ctx context.Context, r gitx.Repo, sha, path string) (metricspec.Spec, bool) {
	if sha == "" {
		return metricspec.Spec{}, false
	}
	text, err := r.Git(ctx, "show", sha+":"+path)
	if err != nil {
		return metricspec.Spec{}, false
	}
	return metricspec.Parse(path, text), true
}

// definition is the file's sentences after its title: what people agreed on, as they wrote it.
func definition(s metricspec.Spec) []string {
	var out []string
	for _, l := range s.Lines {
		switch l.Kind {
		case "blank", "comment", "tag", "title":
		default:
			out = append(out, strings.TrimSpace(l.Text))
		}
	}
	return out
}

func metricVersions(m *Metric) int {
	n := 0
	for _, e := range m.Events {
		if e.Kind == "proposed" || e.Kind == "added" || e.Kind == "changed" {
			n++
		}
	}
	return n
}

// attachMetricRuns adds, after each approval, the first run within the thresholds and the runs
// outside them. Patterns ("api.*.p95") are thresholds for many metrics, not requirements, so only
// exact IDs count.
func attachMetricRuns(m *Metric, runs []*model.Run) {
	if strings.Contains(m.ID, "*") {
		return
	}
	sorted := slices.Clone(runs)
	slices.SortFunc(sorted, func(a, b *model.Run) int { return a.Start.Compare(b.Start) })
	met := map[int]bool{}
	for _, run := range sorted {
		for _, rm := range run.Metrics {
			if rm.ID != m.ID {
				continue
			}
			v := versionAt(m, run.Start)
			if v == 0 {
				continue // not approved yet when it ran
			}
			c := rm.Previous
			if c == nil || c.Status == "none" {
				c = rm.Release
			}
			if c == nil || c.Status == "none" {
				continue
			}
			ev := Event{At: run.Start, Commit: run.Commit, Run: run.ID, Version: v}
			switch {
			case c.Status == "ok" && !met[v]:
				met[v] = true
				ev.Kind = "met"
			case c.Status != "ok":
				ev.Kind = "missed"
			default:
				continue
			}
			m.Events = append(m.Events, ev)
		}
	}
	slices.SortStableFunc(m.Events, func(a, b Event) int { return a.At.Compare(b.At) })
	if m.State == "approved" {
		for i := len(m.Events) - 1; i >= 0; i-- {
			if k := m.Events[i].Kind; k == "met" || k == "missed" {
				m.State = k
				break
			}
		}
	}
}

// versionAt is the approved version in force at a time: the latest approval or plain addition before it.
func versionAt(m *Metric, at time.Time) int {
	v := 0
	for _, e := range m.Events {
		if e.At.After(at) {
			break
		}
		if e.Kind == "approved" || e.Kind == "added" {
			v = e.Version
		}
	}
	return v
}
