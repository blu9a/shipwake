// Package story rebuilds each feature's requirements loop from Git and recorded runs: when its
// scenarios were proposed, how they changed, when they were approved, and when each version first
// passed, with its recording.
//
// The stages come from the scenarios' state, not from who typed them: the work belongs to the person
// responsible for it, whoever or whatever drafted it.
//
//	proposed   a scenario first appears tagged @proposed
//	changed    its steps change in a later commit
//	approved   the @proposed tag is removed; Approved-by names who approved (else the commit's author)
//	passed     the first run in which its test passed, for each version of its steps
package story

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/blu9a/shipwake/internal/gherkin"
	"github.com/blu9a/shipwake/internal/gitx"
	"github.com/blu9a/shipwake/internal/model"
)

// Event is one step of a scenario's story.
type Event struct {
	Kind     string    `json:"kind"`              // proposed, added, changed, approved, removed, passed, failed
	At       time.Time `json:"at"`                // the commit's or the run's time
	Commit   string    `json:"commit"`            // the commit, or the commit the run tested
	By       string    `json:"by,omitempty"`      // the commit's author, or the approver
	Steps    []string  `json:"steps,omitempty"`   // the scenario after this event
	Was      []string  `json:"was,omitempty"`     // before it, for a change
	Version  int       `json:"version,omitempty"` // the version of the steps this event is about
	Run      string    `json:"run,omitempty"`     // for passed and failed
	Video    string    `json:"video,omitempty"`   // the recording of that run's test
	Subject  string    `json:"subject,omitempty"` // the commit's subject
	Approved bool      `json:"approved,omitempty"`
}

// Scenario is one scenario's story.
type Scenario struct {
	Title  string   `json:"title"`
	State  string   `json:"state"` // proposed, approved, passing, failing, removed
	Steps  []string `json:"steps"`
	Events []Event  `json:"events"`
}

// Feature is one .feature file's story.
type Feature struct {
	Path      string      `json:"path"`
	Name      string      `json:"name"`
	Scenarios []*Scenario `json:"scenarios"`
}

type commit struct {
	sha, parent, author, subject, approvedBy string
	at                                       time.Time
}

// Build reads every commit in rev's history that changed a .feature file, oldest first, then the runs.
// Each branch has its own requirements: rev is the trunk, or a branch.
func Build(ctx context.Context, r gitx.Repo, rev string, runs []*model.Run) ([]*Feature, error) {
	const sep = "\x1f"
	out, err := r.Git(ctx, "log", "--reverse", "--format=%H"+sep+"%P"+sep+"%an"+sep+"%cI"+sep+"%s"+sep+
		"%(trailers:key=Approved-by,valueonly,separator=%x2C)"+"\x1e",
		rev, "--", "*.feature")
	if err != nil {
		return nil, err
	}
	features := map[string]*Feature{}
	var order []string
	find := func(path string) *Feature {
		if f, ok := features[path]; ok {
			return f
		}
		f := &Feature{Path: path}
		features[path] = f
		order = append(order, path)
		return f
	}
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
		paths, err := r.Lines(ctx, "diff-tree", "--no-commit-id", "--name-only", "-r", "--root", c.sha, "--", "*.feature")
		if err != nil {
			return nil, err
		}
		for _, path := range paths {
			var before, after gherkin.Feature
			if c.parent != "" {
				if t, err := r.Git(ctx, "show", c.parent+":"+path); err == nil {
					before = gherkin.Parse(t)
				}
			}
			if t, err := r.Git(ctx, "show", c.sha+":"+path); err == nil {
				after = gherkin.Parse(t)
			}
			f := find(path)
			if after.Name != "" {
				f.Name = after.Name
			}
			apply(f, c, before, after)
		}
	}
	res := make([]*Feature, 0, len(order))
	for _, path := range order {
		res = append(res, features[path])
	}
	attachRuns(ctx, r, res, runs)
	return res, nil
}

func (f *Feature) scenario(title string) *Scenario {
	for _, s := range f.Scenarios {
		if s.Title == title {
			return s
		}
	}
	s := &Scenario{Title: title}
	f.Scenarios = append(f.Scenarios, s)
	return s
}

func proposed(tags []string) bool { return slices.Contains(tags, "@proposed") }

// apply turns one commit's change to a feature file into events.
func apply(f *Feature, c commit, before, after gherkin.Feature) {
	base := Event{At: c.at, Commit: c.sha, Subject: c.subject, By: c.author}
	for _, title := range after.Order {
		s := f.scenario(title)
		steps := after.Scenarios[title]
		was, existed := before.Scenarios[title]
		ev := base
		ev.Steps = steps
		switch {
		case !existed && proposed(after.Tags[title]):
			ev.Kind, s.State = "proposed", "proposed"
		case !existed:
			ev.Kind, s.State = "added", "approved"
			ev.Approved = true
		case !slices.Equal(was, steps):
			ev.Kind, ev.Was = "changed", was
		}
		if ev.Kind != "" {
			ev.Version = versions(s) + 1
			s.Events = append(s.Events, ev)
		}
		if existed && proposed(before.Tags[title]) && !proposed(after.Tags[title]) {
			ap := base
			ap.Kind, ap.Steps, ap.Approved = "approved", steps, true
			ap.By = c.author
			if c.approvedBy != "" {
				ap.By, _, _ = strings.Cut(c.approvedBy, " <") // the name, as for authors
			}
			ap.Version = versions(s)
			s.Events = append(s.Events, ap)
			s.State = "approved"
		}
		s.Steps = steps
	}
	for _, title := range before.Order {
		if _, still := after.Scenarios[title]; !still {
			s := f.scenario(title)
			ev := base
			ev.Kind, ev.Was = "removed", before.Scenarios[title]
			s.Events = append(s.Events, ev)
			s.State = "removed"
		}
	}
}

// versions counts the distinct versions of a scenario's steps so far.
func versions(s *Scenario) int {
	n := 0
	for _, e := range s.Events {
		if e.Kind == "proposed" || e.Kind == "added" || e.Kind == "changed" {
			n++
		}
	}
	return n
}

// attachRuns adds, for each version of each scenario, the first run where its test passed, and
// failures after that. A test belongs to a scenario by feature and scenario name, as Gherkin runners
// name them; the version is the scenario's steps at the commit the run tested.
func attachRuns(ctx context.Context, r gitx.Repo, features []*Feature, runs []*model.Run) {
	sorted := slices.Clone(runs)
	slices.SortFunc(sorted, func(a, b *model.Run) int { return a.Start.Compare(b.Start) })
	at := map[string]map[string]gherkin.Feature{} // commit → path → parsed
	parsed := func(sha, path string) gherkin.Feature {
		if at[sha] == nil {
			at[sha] = map[string]gherkin.Feature{}
		}
		if f, ok := at[sha][path]; ok {
			return f
		}
		var f gherkin.Feature
		if t, err := r.Git(ctx, "show", sha+":"+path); err == nil {
			f = gherkin.Parse(t)
		}
		at[sha][path] = f
		return f
	}
	for _, f := range features {
		for _, s := range f.Scenarios {
			passedVersions := map[int]bool{}
			for _, run := range sorted {
				for _, t := range run.Tests {
					if t.Name != s.Title || (t.Feature != "" && t.Feature != f.Name) {
						continue
					}
					steps := parsed(run.Commit, f.Path).Scenarios[s.Title]
					v := versionOf(s, steps)
					if v == 0 {
						continue
					}
					switch t.Status {
					case "passed":
						if passedVersions[v] {
							continue
						}
						passedVersions[v] = true
						s.Events = append(s.Events, Event{Kind: "passed", At: run.Start, Commit: run.Commit, Run: run.ID, Version: v, Video: video(t), Steps: steps})
					case "failed":
						s.Events = append(s.Events, Event{Kind: "failed", At: run.Start, Commit: run.Commit, Run: run.ID, Version: v, Steps: steps})
					}
				}
			}
			slices.SortStableFunc(s.Events, func(a, b Event) int { return a.At.Compare(b.At) })
			if s.State != "removed" && s.State != "proposed" {
				for i := len(s.Events) - 1; i >= 0; i-- {
					e := s.Events[i]
					if e.Kind == "passed" && e.Version == versions(s) {
						s.State = "passing"
						break
					}
					if e.Kind == "failed" && e.Version == versions(s) {
						s.State = "failing"
						break
					}
				}
			}
		}
	}
}

// versionOf finds which version of the scenario these steps are.
func versionOf(s *Scenario, steps []string) int {
	if steps == nil {
		return 0
	}
	for i := len(s.Events) - 1; i >= 0; i-- {
		e := s.Events[i]
		if (e.Kind == "proposed" || e.Kind == "added" || e.Kind == "changed") && slices.Equal(e.Steps, steps) {
			return e.Version
		}
	}
	return 0
}

func video(t model.Test) string {
	for _, a := range t.Attachments {
		if strings.HasPrefix(a.Type, "video/") {
			return a.Path
		}
	}
	return ""
}
