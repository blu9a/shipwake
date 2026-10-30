// Package judge compares measurements with their baselines and applies thresholds.
//
// Runs form series: the same kind, name and runner. Within a series, runs are ordered by where their
// commit sits in history, not by when they ran, so re-running an old commit doesn't confuse the order.
// A run is compared with the previous run of its series that its commit descends from, and with the
// last run at or before the latest release.
package judge

import (
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/blu9a/shipwake/internal/model"
)

// History places commits on one line: older is smaller. Trunk commits and the commits merged into
// them are on the trunk; commits on an open branch sit just after the commit the branch left from.
type History struct {
	pos      map[string]float64
	trunk    map[string]bool
	branchOf map[string]string
	base     map[string]float64 // branch name → position of its base
}

// NewHistory builds the positions from a project's index.
func NewHistory(p *model.Project) *History {
	h := &History{pos: map[string]float64{}, trunk: map[string]bool{}, branchOf: map[string]string{}, base: map[string]float64{}}
	n := len(p.Rows)
	for i, r := range p.Rows {
		at := float64(n-1-i) * 1000
		h.pos[r.SHA], h.trunk[r.SHA] = at, true
		for j, c := range r.Commits {
			h.pos[c], h.trunk[c] = at-float64(j+1), true
		}
	}
	for _, b := range p.Branches {
		at, ok := h.pos[b.Base]
		if !ok {
			at = -1
		}
		h.base[b.Name] = at
		m := len(b.Ahead)
		for k, c := range b.Ahead {
			if _, onTrunk := h.trunk[c]; onTrunk {
				continue
			}
			h.pos[c], h.branchOf[c] = at+float64(m-k)*0.01, b.Name
		}
	}
	return h
}

// Pos is a commit's place in history; ok is false for commits the index doesn't know.
func (h *History) Pos(sha string) (float64, bool) { v, ok := h.pos[sha]; return v, ok }

// Reaches reports whether a run on commit a could be an ancestor of commit b in the sense that matters:
// a is on the trunk at or before b's trunk ancestry, or both are on the same open branch.
func (h *History) Reaches(a, b string) bool {
	pa, oka := h.pos[a]
	pb, okb := h.pos[b]
	if !oka || !okb || pa > pb {
		return false
	}
	if h.trunk[a] {
		if h.trunk[b] {
			return true
		}
		return pa <= h.base[h.branchOf[b]]
	}
	return !h.trunk[b] && h.branchOf[a] == h.branchOf[b]
}

// Series identifies runs that can be compared: the same name on the same runner. The kind is only a
// label, so renaming it doesn't start a new series.
func Series(r *model.Run) string { return r.Name + "\x00" + r.Context["runner"] }

// Apply fills each metric's comparisons and each run summary's tallies. runs and p.Runs share order.
func Apply(p *model.Project, runs []*model.Run) {
	h := NewHistory(p)
	// A metric ID has one project rule. Resolve it once per Apply, rather than sorting
	// patterns and compiling the same regex for every run and comparison.
	type match struct {
		rule        model.Rule
		ok          bool
		warn, block Threshold // parsed; a threshold that doesn't parse is none (doctor reports it)
	}
	rules := make(map[string]match)
	lookup := func(id string) match {
		if m, found := rules[id]; found {
			return m
		}
		rule, ok := Lookup(p.Rules, id)
		m := match{rule: rule, ok: ok}
		m.warn, _ = ParseThreshold(rule.Warn)
		m.block, _ = ParseThreshold(rule.Block)
		rules[id] = m
		return m
	}
	ruleFor := func(id string) (model.Rule, bool) { m := lookup(id); return m.rule, m.ok }
	for _, r := range runs { // the project's word on which way is better wins over the tool's
		for k := range r.Metrics {
			if rule, ok := ruleFor(r.Metrics[k].ID); ok && (rule.Direction == "higher" || rule.Direction == "lower") {
				r.Metrics[k].Direction = rule.Direction
			}
		}
	}
	ordered := slices.Clone(runs)
	slices.SortStableFunc(ordered, func(a, b *model.Run) int {
		pa, _ := h.Pos(a.Commit)
		pb, _ := h.Pos(b.Commit)
		if pa != pb {
			if pa < pb {
				return -1
			}
			return 1
		}
		return a.Start.Compare(b.Start)
	})
	var release string
	relPos := math.Inf(-1)
	if len(p.Tags) > 0 {
		if v, ok := h.Pos(p.Tags[0].SHA); ok {
			release, relPos = p.Tags[0].Name, v
		}
	}
	summary := map[string]*model.RunSummary{}
	for i := range p.Runs {
		summary[p.Runs[i].ID] = &p.Runs[i]
	}
	lastByTime := previousByTime(runs)
	seen := make(map[string][]*model.Run)
	for _, r := range ordered {
		key := Series(r)
		earlier := seen[key]
		seen[key] = append(earlier, r)
		s := summary[r.ID]
		if s == nil {
			continue
		}
		s.Metrics = len(r.Metrics)
		s.Order = -1
		if v, ok := h.Pos(r.Commit); ok {
			s.Order = v
		}
		// The previous run of the series by time, for "what changed" in the context.
		lastSame := lastByTime[r]
		if lastSame != nil {
			for _, k := range unionKeys(r.Context, lastSame.Context) {
				if r.Context[k] != lastSame.Context[k] {
					s.Changed = append(s.Changed, k)
					if s.Was == nil {
						s.Was = map[string]string{}
					}
					s.Was[k] = lastSame.Context[k]
				}
			}
		}
		if len(r.Metrics) == 0 {
			continue
		}
		var prev, rel *model.Run
		pos, _ := h.Pos(r.Commit)
		needRelease := release != "" && pos > relPos
		for j := len(earlier) - 1; j >= 0; j-- {
			o := earlier[j]
			if !h.Reaches(o.Commit, r.Commit) {
				continue
			}
			if prev == nil {
				prev = o
			}
			if p, ok := h.Pos(o.Commit); needRelease && ok && rel == nil && h.trunk[o.Commit] && p <= relPos {
				rel = o
			}
			if prev != nil && (!needRelease || rel != nil) {
				break
			}
		}
		s.Comparable = prev != nil
		s.HasRelease = rel != nil
		for k := range r.Metrics {
			m := &r.Metrics[k]
			th := lookup(m.ID)
			m.Previous = compare(m, prev, label(p, prev), th.warn, th.block)
			m.Release = compare(m, rel, release, th.warn, th.block)
			count(&s.Previous, m.Previous)
			count(&s.Release, m.Release)
		}
	}
}

// Context changes use wall-clock order across runners. Equal timestamps are not
// predecessors; if the preceding timestamp ties, the first input run wins as before.
func previousByTime(runs []*model.Run) map[*model.Run]*model.Run {
	byName := make(map[string][]*model.Run)
	for _, r := range runs {
		byName[r.Name] = append(byName[r.Name], r)
	}
	previous := make(map[*model.Run]*model.Run, len(runs))
	for _, group := range byName {
		slices.SortStableFunc(group, func(a, b *model.Run) int { return a.Start.Compare(b.Start) })
		var last *model.Run
		for i := 0; i < len(group); {
			j := i + 1
			for j < len(group) && group[j].Start.Equal(group[i].Start) {
				j++
			}
			for _, r := range group[i:j] {
				previous[r] = last
			}
			last, i = group[i], j
		}
	}
	return previous
}

func label(p *model.Project, r *model.Run) string {
	if r == nil {
		return ""
	}
	if c, ok := p.Commits[r.Commit]; ok {
		return c.Short
	}
	return r.Commit[:min(7, len(r.Commit))]
}

func count(t *model.Tally, c *model.Comparison) {
	if c == nil {
		return
	}
	switch c.Status {
	case "ok":
		t.OK++
	case "warn":
		t.Warn++
	case "block":
		t.Block++
	}
}

func compare(m *model.Metric, base *model.Run, lbl string, warn, block Threshold) *model.Comparison {
	v := Mean(m.Values)
	c := &model.Comparison{Status: "none"}
	if base != nil {
		for _, bm := range base.Metrics {
			if bm.ID != m.ID || len(bm.Values) == 0 {
				continue
			}
			b := Mean(bm.Values)
			c.Run, c.Label, c.Value = base.ID, lbl, &b
			if b != 0 {
				d := (v - b) / math.Abs(b) * 100
				c.Delta = &d
			}
		}
	}
	needsBase := func(th Threshold) bool { return th == nil || th.NeedsBaseline() }
	if (warn == nil && block == nil) || (c.Value == nil && needsBase(warn) && needsBase(block)) {
		if c.Run == "" {
			return nil // nothing to compare with and nothing absolute to check
		}
		return c
	}
	hitBlock, partBlock := block.Hit(v, c.Value, m.Unit, m.Direction)
	hitWarn, partWarn := warn.Hit(v, c.Value, m.Unit, m.Direction)
	switch {
	case hitBlock:
		c.Status = "block"
	case hitWarn:
		c.Status = "warn"
	default:
		c.Status = "ok"
	}
	c.Partial = partBlock || partWarn
	return c
}

// Lookup finds a metric's thresholds: an exact ID first, then the first matching pattern, where *
// stands for one dot-separated part ("api.*.p95").
func Lookup(limits map[string]model.Rule, id string) (model.Rule, bool) {
	if l, ok := limits[id]; ok {
		return l, true
	}
	keys := make([]string, 0, len(limits))
	for k := range limits {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if !strings.Contains(k, "*") {
			continue
		}
		parts := strings.Split(k, "*")
		for i := range parts {
			parts[i] = regexp.QuoteMeta(parts[i])
		}
		if regexp.MustCompile("^" + strings.Join(parts, `[^.]+`) + "$").MatchString(id) {
			return limits[k], true
		}
	}
	return model.Rule{}, false
}

// Mean is the average of values.
func Mean(v []float64) float64 {
	if len(v) == 0 {
		return math.NaN()
	}
	var s float64
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func unionKeys(a, b map[string]string) []string {
	seen := map[string]bool{}
	var keys []string
	for _, m := range []map[string]string{a, b} {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	slices.Sort(keys)
	return keys
}

var binaryUnits = []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}

// Format shows a value in its unit the way people read it: 62.7 µs, 61.5 MiB, 0.88, 3.2%.
func Format(v float64, unit string) string {
	if math.IsNaN(v) {
		return "—"
	}
	trim := func(x float64, prec int) string { return strconv.FormatFloat(x, 'f', prec, 64) }
	if i := slices.Index(binaryUnits, unit); i >= 0 {
		for math.Abs(v) >= 1024 && i < len(binaryUnits)-1 {
			v /= 1024
			i++
		}
		prec := 1
		if i == 0 {
			prec = 0
		}
		return trim(v, prec) + " " + binaryUnits[i]
	}
	switch unit {
	case "ns":
		switch {
		case v >= 1e6:
			return trim(v/1e6, 2) + " ms"
		case v >= 1000:
			return trim(v/1000, 1) + " µs"
		}
		return trim(v, 0) + " ns"
	case "ms":
		if v >= 100 {
			return trim(v, 0) + " ms"
		}
		return trim(v, 1) + " ms"
	case "allocs":
		return trim(v, 0)
	case "score":
		return trim(v, 2)
	case "%":
		return strconv.FormatFloat(v, 'g', 3, 64) + "%"
	}
	return strconv.FormatFloat(v, 'g', 4, 64) + " " + unit
}
