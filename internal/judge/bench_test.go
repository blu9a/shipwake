package judge

import (
	"fmt"
	"testing"
	"time"

	"github.com/blu9a/shipwake/internal/model"
)

// BenchmarkJudgeApply judges 600 runs of 12 measurements on a 300-commit trunk, two runners.
func BenchmarkJudgeApply(b *testing.B) {
	const commits = 300
	base := &model.Project{
		Commits: map[string]model.Commit{},
		Rules:   map[string]model.Rule{"api.*.p95": {Warn: "+15%", Block: "+40%"}, "quality": {Direction: "higher", Warn: "0.88"}},
		Tags:    []model.Tag{{Name: "v1", SHA: "c100"}},
	}
	for i := commits - 1; i >= 0; i-- {
		sha := fmt.Sprintf("c%d", i)
		base.Rows = append(base.Rows, model.Row{SHA: sha})
		base.Commits[sha] = model.Commit{Short: sha}
	}
	fresh := func() []*model.Run {
		var runs []*model.Run
		for i := range commits {
			for _, runner := range []string{"a", "b"} {
				r := &model.Run{ID: fmt.Sprintf("r%d%s", i, runner), Name: "API", Commit: fmt.Sprintf("c%d", i),
					Start: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Minute), Context: map[string]string{"runner": runner}}
				for k := range 11 {
					r.Metrics = append(r.Metrics, model.Metric{ID: fmt.Sprintf("api.e%d.p95", k), Direction: "lower", Values: []float64{100 + float64(i%7), 101, 99}})
				}
				r.Metrics = append(r.Metrics, model.Metric{ID: "quality", Direction: "higher", Values: []float64{0.9}})
				runs = append(runs, r)
			}
		}
		return runs
	}
	runs := fresh()
	for b.Loop() {
		p := *base
		p.Runs = make([]model.RunSummary, len(runs))
		for i, r := range runs {
			p.Runs[i] = model.RunSummary{ID: r.ID}
		}
		Apply(&p, runs)
	}
}
