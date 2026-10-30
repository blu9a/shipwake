package readers

import (
	"encoding/json"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/blu9a/shipwake/internal/model"
)

// k6Summary is k6's --summary-export file: metrics by name, each with its statistics.
type k6Summary struct {
	Metrics map[string]map[string]any `json:"metrics"`
}

var k6Tag = regexp.MustCompile(`^([^{]+)(?:\{(.*)\})?$`)

// readK6 reads `k6 run --summary-export=summary.json`. Durations give p50, p95 and the average,
// lower is better; rates give requests per second (higher is better) or failure rates (lower).
// Sub-metrics with tags, like http_req_duration{name:GET /plans}, become their own suite.
func readK6(r io.Reader) (model.Run, error) {
	var s k6Summary
	if err := json.NewDecoder(r).Decode(&s); err != nil {
		return model.Run{}, err
	}
	run := model.Run{}
	names := make([]string, 0, len(s.Metrics))
	for n := range s.Metrics {
		names = append(names, n)
	}
	slices.Sort(names)
	num := func(m map[string]any, k string) (float64, bool) {
		v, ok := m[k].(float64)
		return v, ok
	}
	for _, full := range names {
		stats := s.Metrics[full]
		m := k6Tag.FindStringSubmatch(full)
		base, tags := strings.TrimSpace(m[1]), m[2]
		suite := base
		if tags != "" {
			suite = tags
		}
		id := func(stat string) string { return strings.ReplaceAll(full, " ", "_") + "." + stat }
		add := func(stat, name, unit, dir string, v float64) {
			run.Metrics = append(run.Metrics, model.Metric{ID: id(stat), Suite: suite, Name: name, Unit: unit, Direction: dir, Values: []float64{v}})
		}
		if p95, ok := num(stats, "p(95)"); ok && strings.Contains(base, "duration") {
			if med, ok := num(stats, "med"); ok {
				add("p50", "p50 "+label(base), "ms", "lower", med)
			}
			add("p95", "p95 "+label(base), "ms", "lower", p95)
			continue
		}
		if rate, ok := num(stats, "rate"); ok {
			switch {
			case strings.Contains(base, "failed") || strings.Contains(base, "error"):
				add("rate", label(base), "%", "lower", rate*100)
			case base == "http_reqs" || base == "iterations":
				add("rate", label(base), "req/s", "higher", rate)
			}
			continue
		}
		if v, ok := num(stats, "value"); ok && base == "checks" {
			add("value", "checks passed", "%", "higher", v*100)
		}
	}
	return run, nil
}

func label(base string) string {
	switch base {
	case "http_req_duration":
		return "request time"
	case "http_req_failed":
		return "failed requests"
	case "http_reqs":
		return "requests per second"
	case "iteration_duration":
		return "iteration time"
	case "iterations":
		return "iterations per second"
	}
	return strings.ReplaceAll(base, "_", " ")
}
