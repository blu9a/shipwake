package metricspec

import (
	"strings"
	"testing"
)

const exact = `@proposed
Metric: Exact totals
  The share of bills whose shares add up to the bill plus tip,
  rounded half up to the cent.

  Measured on evals/bills.jsonl
  Each case scores 1 when the shares add up exactly, else 0
  Higher is better
  Blocks a release below 100%
  Warns when it gets 1% worse than the baseline
`

func TestParse(t *testing.T) {
	s := Parse("metrics/totals.exact.metric", exact)
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	got := []string{s.ID, s.Name, s.Description, s.Dataset, s.Grader, s.Direction, s.Block, s.Warn}
	want := []string{"totals.exact", "Exact totals",
		"The share of bills whose shares add up to the bill plus tip, rounded half up to the cent.",
		"evals/bills.jsonl", "Each case scores 1 when the shares add up exactly, else 0", "higher", "< 100%", "-1%"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("field %d = %q, want %q", i, got[i], want[i])
		}
	}
	if !s.Proposed() {
		t.Error("not proposed")
	}
	if k := s.Lines[8].Kind; k != "block" || s.Lines[8].Value != "100%" {
		t.Errorf("line 9: %q %q", k, s.Lines[8].Value)
	}
}

func TestParseChinese(t *testing.T) {
	s := Parse("p95.metric", "指标：首屏时间\n  首屏渲染完成的时间。\n\n  越低越好\n  高于 200ms 时阻止发布\n  比基线差 10% 时警告\n")
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	if s.Name != "首屏时间" || s.Direction != "lower" || s.Block != "> 200ms" || s.Warn != "+10%" {
		t.Errorf("%+v", s)
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"no title":         "Exact totals\n",
		"unknown setting":  "Metric: X\n  Higher is better\n  Blocks a release eventually\n",
		"description late": "Metric: X\n  Higher is better\n  It counts bills.\n",
	}
	for name, text := range cases {
		if err := Parse("x.metric", text).Err(); err == nil {
			t.Errorf("%s: no error", name)
		} else if !strings.Contains(err.Error(), "line") && name != "no title" {
			t.Errorf("%s: %v has no line number", name, err)
		}
	}
}
