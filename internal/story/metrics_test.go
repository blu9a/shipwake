package story

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blu9a/shipwake/internal/gitx"
	"github.com/blu9a/shipwake/internal/model"
)

func TestMetricStory(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo := gitx.Repo{Dir: dir}
	git := func(args ...string) string {
		t.Helper()
		out, err := repo.Git(ctx, args...)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	commit := func(text, msg string) string {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(dir, "metrics"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "metrics", "totals.exact.metric"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		git("add", "metrics")
		git("commit", "-q", "-m", msg)
		return git("rev-parse", "HEAD")
	}
	git("init", "-q")
	git("config", "user.name", "Ana")
	git("config", "user.email", "ana@example.com")
	spec := "Metric: Exact totals\n  Higher is better\n  Blocks a release below 100%\n"
	commit("@proposed\n"+spec, "propose")
	approved := commit(spec, "approve\n\nApproved-by: Bo <bo@example.com>")
	run := func(id, status string, at time.Duration) *model.Run {
		return &model.Run{ID: id, Commit: approved, Start: time.Now().Add(at), Metrics: []model.Metric{{ID: "totals.exact", Values: []float64{0.9},
			Previous: &model.Comparison{Status: status}}}}
	}
	ms, err := BuildMetrics(ctx, repo, []*model.Run{run("r2", "ok", 2*time.Hour), run("r1", "block", time.Hour), run("r3", "ok", 3*time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 {
		t.Fatalf("got %d metrics, want 1", len(ms))
	}
	m := ms[0]
	var kinds []string
	for _, e := range m.Events {
		kinds = append(kinds, e.Kind)
	}
	want := []string{"proposed", "approved", "missed", "met"} // r3 meets it again: not news
	if len(kinds) != len(want) {
		t.Fatalf("events %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("events %v, want %v", kinds, want)
		}
	}
	if m.Events[1].By != "Bo" || m.State != "met" || m.Name != "Exact totals" || m.ID != "totals.exact" {
		t.Errorf("approved by %q, state %q, name %q, id %q", m.Events[1].By, m.State, m.Name, m.ID)
	}
	if len(m.Definition) != 2 || m.Definition[1] != "Blocks a release below 100%" {
		t.Errorf("definition %q", m.Definition)
	}
}
