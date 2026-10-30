package judge

import (
	"slices"
	"testing"
	"time"

	"github.com/blu9a/shipwake/internal/model"
)

func TestApplyContextTiesAndRunnerBaselines(t *testing.T) {
	at := func(min int) time.Time { return time.Date(2026, 1, 1, 0, min, 0, 0, time.UTC) }
	for _, reverseTie := range []bool{false, true} {
		p := &model.Project{Rows: []model.Row{{SHA: "c4"}, {SHA: "c3"}, {SHA: "c2"}, {SHA: "c1"}}}
		mk := func(id, sha, runner, version string, minute int) *model.Run {
			return &model.Run{ID: id, Name: "API", Commit: sha, Start: at(minute),
				Context: map[string]string{"runner": runner, "model": version},
				Metrics: []model.Metric{{ID: "latency", Values: []float64{100}}}}
		}
		r1 := mk("r1", "c1", "a", "one", 1)
		r2 := mk("r2", "c2", "b", "two", 2)
		r3 := mk("r3", "c3", "c", "three", 2)
		r4 := mk("r4", "c4", "a", "two", 3)
		runs := []*model.Run{r2, r4, r1, r3}
		want := []string{"runner"}
		if reverseTie {
			runs = []*model.Run{r3, r4, r2, r1}
			want = []string{"model", "runner"}
		}
		for _, r := range runs {
			p.Runs = append(p.Runs, model.RunSummary{ID: r.ID})
		}
		Apply(p, runs)
		for _, s := range p.Runs {
			if s.ID == r4.ID && !slices.Equal(s.Changed, want) {
				t.Errorf("reverseTie=%v: changes=%v, want %v", reverseTie, s.Changed, want)
			}
		}
		if c := r4.Metrics[0].Previous; c == nil || c.Run != "r1" {
			t.Errorf("runner a baseline = %+v, want r1", c)
		}
		// Runs at the same timestamp both compare their context with r1, never each other.
		for _, s := range p.Runs {
			if s.ID == r2.ID && (s.Was["runner"] == "" || len(s.Was) != len(s.Changed)) {
				t.Errorf("r2 was = %v, changed = %v", s.Was, s.Changed)
			}
			if (s.ID == r2.ID || s.ID == r3.ID) && !slices.Equal(s.Changed, []string{"model", "runner"}) {
				t.Errorf("simultaneous run changes = %+v", s)
			}
		}
	}
}

func TestApplyBranchAndMissingSummaryBaselines(t *testing.T) {
	p := &model.Project{
		Rows: []model.Row{{SHA: "c3"}, {SHA: "c2"}, {SHA: "c1"}},
		Branches: []model.Branch{
			{Name: "x", Base: "c1", Ahead: []string{"x2", "x1"}},
			{Name: "y", Base: "c2", Ahead: []string{"y1"}},
		},
		Tags: []model.Tag{{Name: "v1", SHA: "c2"}},
	}
	var runs []*model.Run
	for i, sha := range []string{"c1", "x1", "x2", "c2", "y1", "c3", "unknown"} {
		runs = append(runs, &model.Run{ID: sha, Name: "Bench", Commit: sha,
			Start:   time.Date(2026, 1, 1, 0, i, 0, 0, time.UTC),
			Metrics: []model.Metric{{ID: "latency", Values: []float64{100}}}})
		if sha != "c1" { // A missing summary must not remove a valid baseline.
			p.Runs = append(p.Runs, model.RunSummary{ID: sha})
		}
	}
	Apply(p, runs)
	for i, want := range []string{"", "c1", "x1", "c1", "c2", "c2", ""} {
		c := runs[i].Metrics[0].Previous
		if (want == "" && c != nil) || (want != "" && (c == nil || c.Run != want)) {
			t.Errorf("%s: previous=%+v, want %s", runs[i].ID, c, want)
		}
	}
	if runs[2].Metrics[0].Release != nil {
		t.Error("branch forked before the release compared with that later release")
	}
	if c := runs[5].Metrics[0].Release; c == nil || c.Run != "c2" {
		t.Errorf("trunk release baseline = %+v, want c2", c)
	}
}

func TestApply(t *testing.T) {
	// History, newest first: c3 (merge of b1), c2 (release v1), c1. Open branch x with x1 off c3.
	p := &model.Project{
		Rows:     []model.Row{{SHA: "c3", Merge: true, Commits: []string{"b1"}}, {SHA: "c2"}, {SHA: "c1"}},
		Branches: []model.Branch{{Name: "x", Ahead: []string{"x1"}, Base: "c3"}},
		Tags:     []model.Tag{{Name: "v1", SHA: "c2"}},
		Commits:  map[string]model.Commit{"c1": {Short: "c1"}, "c2": {Short: "c2"}, "c3": {Short: "c3"}, "b1": {Short: "b1"}, "x1": {Short: "x1"}},
		Rules:    map[string]model.Rule{"api.*.p95": {Warn: "+15%", Block: "+40%"}, "quality": {Warn: "0.88", Block: "0.82"}},
	}
	at := func(min int) time.Time { return time.Date(2026, 1, 1, 0, min, 0, 0, time.UTC) }
	mk := func(id, commit string, start int, runner string, p95, q float64) *model.Run {
		return &model.Run{ID: id, Kind: "perf", Name: "API", Commit: commit, Start: at(start), Context: map[string]string{"runner": runner},
			Metrics: []model.Metric{{ID: "api.plans.p95", Direction: "lower", Values: []float64{p95}}, {ID: "quality", Direction: "higher", Values: []float64{q}}}}
	}
	runs := []*model.Run{
		mk("r1", "c1", 1, "a", 100, 0.90),
		mk("r2", "c2", 2, "a", 105, 0.90), // the release
		mk("r0", "c1", 9, "a", 101, 0.90), // re-run of an old commit, later in time
		mk("r3", "b1", 3, "a", 130, 0.86), // inside the merge
		mk("r4", "c3", 4, "b", 150, 0.80), // another runner
		mk("r5", "x1", 5, "a", 160, 0.90), // open branch
	}
	for _, r := range runs {
		p.Runs = append(p.Runs, model.RunSummary{ID: r.ID})
	}
	Apply(p, runs)
	byID := map[string]*model.Run{}
	for _, r := range runs {
		byID[r.ID] = r
	}
	r3 := byID["r3"].Metrics[0]
	if r3.Previous == nil || r3.Previous.Run != "r2" || r3.Previous.Status != "warn" {
		t.Errorf("r3 p95 previous = %+v, want r2 warn (+23.8%%)", r3.Previous)
	}
	if r3.Release == nil || r3.Release.Label != "v1" || r3.Release.Run != "r2" {
		t.Errorf("r3 p95 release = %+v", r3.Release)
	}
	if q := byID["r3"].Metrics[1].Previous; q == nil || q.Status != "warn" {
		t.Errorf("r3 quality = %+v, want warn (absolute 0.88)", q)
	}
	if byID["r4"].Metrics[0].Previous != nil {
		t.Errorf("another runner was compared: %+v", byID["r4"].Metrics[0].Previous)
	}
	if q := byID["r4"].Metrics[1].Previous; q == nil || q.Status != "block" {
		t.Errorf("r4 quality = %+v, want block from the absolute threshold alone", q)
	}
	if prev := byID["r5"].Metrics[0].Previous; prev == nil || prev.Run != "r3" || prev.Status != "warn" {
		t.Errorf("branch run previous = %+v, want r3 (+23%%)", prev)
	}
	if prev := byID["r2"].Metrics[0].Previous; prev == nil || prev.Run != "r0" {
		t.Errorf("r2 previous = %+v, want r0: the latest run on c1, by history then time", prev)
	}
	s := p.Runs[3] // r3's summary
	if s.Previous.Warn != 2 || !s.Comparable {
		t.Errorf("r3 summary = %+v", s)
	}
}

func TestLookup(t *testing.T) {
	limits := map[string]model.Rule{"api.*.p95": {Warn: "+15%"}, "api.plans.p95": {Warn: "+5%"}}
	if l, _ := Lookup(limits, "api.plans.p95"); l.Warn != "+5%" {
		t.Errorf("exact match should win: %+v", l)
	}
	if l, ok := Lookup(limits, "api.tasks.p95"); !ok || l.Warn != "+15%" {
		t.Errorf("pattern: %+v %v", l, ok)
	}
	if _, ok := Lookup(limits, "api.tasks.sub.p95"); ok {
		t.Error("* must match one part only")
	}
}

func TestDirectionRule(t *testing.T) {
	// The tool recorded "lower" (the default for unknown tools); the project says higher is better.
	p := &model.Project{
		Rows:    []model.Row{{SHA: "c2"}, {SHA: "c1"}},
		Commits: map[string]model.Commit{"c1": {Short: "c1"}, "c2": {Short: "c2"}},
		Rules:   map[string]model.Rule{"hit_rate": {Direction: "higher", Warn: "-5%"}},
	}
	mk := func(id, commit string, min int, v float64) *model.Run {
		return &model.Run{ID: id, Name: "Cache", Commit: commit, Start: time.Date(2026, 1, 1, 0, min, 0, 0, time.UTC),
			Metrics: []model.Metric{{ID: "hit_rate", Direction: "lower", Values: []float64{v}}}}
	}
	runs := []*model.Run{mk("r1", "c1", 1, 0.90), mk("r2", "c2", 2, 0.80)}
	p.Runs = []model.RunSummary{{ID: "r1"}, {ID: "r2"}}
	Apply(p, runs)
	m := runs[1].Metrics[0]
	if m.Direction != "higher" || m.Previous == nil || m.Previous.Status != "warn" {
		t.Errorf("metric = %+v, previous = %+v", m, m.Previous)
	}
}
