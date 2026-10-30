// Package metricspec reads .metric files: a measured requirement written in sentences, the way
// Gherkin writes behavior.
//
//	@proposed
//	Metric: Exact totals
//	  The share of bills whose shares add up to the bill plus tip, rounded half up to the cent.
//
//	  Measured on evals/bills.jsonl
//	  Each case scores 1 when the shares add up exactly, else 0
//	  Higher is better
//	  Blocks a release below 100%
//	  Warns when it gets 1% worse than the baseline
//
// Free sentences after the Metric: line describe it. Then each line is one setting, in a fixed
// pattern; a line that fits none is an error, so what Shipwake understood is never a guess. The
// metric's ID, as runs report it, is the file name without .metric. Chinese patterns work too.
package metricspec

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

// Spec is one .metric file.
type Spec struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Tags        []string `json:"tags,omitempty"`
	Description string   `json:"description,omitempty"`
	Dataset     string   `json:"dataset,omitempty"`
	Grader      string   `json:"grader,omitempty"`    // the sentence, for people
	Direction   string   `json:"direction,omitempty"` // higher or lower
	Warn        string   `json:"warn,omitempty"`      // in the judge's threshold syntax
	Block       string   `json:"block,omitempty"`
	Lines       []Line   `json:"lines"`
}

// Line is one line of the file and what it was read as.
type Line struct {
	N     int    `json:"n"`
	Text  string `json:"text"`
	Kind  string `json:"kind"`            // tag, title, description, dataset, grader, direction, block, warn, blank, comment, error
	Value string `json:"value,omitempty"` // the part that varies: a name, path, number…
	Error string `json:"error,omitempty"`
}

// Proposed reports whether the metric waits for approval.
func (s Spec) Proposed() bool {
	for _, t := range s.Tags {
		if t == "@proposed" {
			return true
		}
	}
	return false
}

// Err is the first line that wasn't understood, if any.
func (s Spec) Err() error {
	for _, l := range s.Lines {
		if l.Kind == "error" {
			return fmt.Errorf("line %d: %s", l.N, l.Error)
		}
	}
	if s.Name == "" {
		return fmt.Errorf("no Metric: line")
	}
	return nil
}

const num = `(\d+(?:\.\d+)?)`

// A level: a number, a percentage of a % metric, or a number with a unit ("200ms").
const level = `(\d+(?:\.\d+)?\s*(?:%|[A-Za-zµ/]+)?)`

// threshold is a Blocks or Warns line. A change against the baseline gets its sign from the
// direction, which may be stated later in the file.
type threshold struct {
	warn  bool
	level string // "< 100%"
	worse string // "10%", "0.5": worse by this much
}

type rule struct {
	re   *regexp.Regexp
	kind string
	// read fills the spec from the match; it returns the varying part to show, and a threshold for
	// Blocks and Warns lines.
	read func(s *Spec, m []string) (string, *threshold)
}

func atLevel(warn, below bool, v string) *threshold {
	op := "> "
	if below {
		op = "< "
	}
	return &threshold{warn: warn, level: op + strings.ReplaceAll(v, " ", "")}
}

func worseBy(warn bool, n, unit string) *threshold {
	switch unit {
	case "point", "points", "个点":
		unit = ""
	}
	return &threshold{warn: warn, worse: n + unit}
}

var (
	titleRe = regexp.MustCompile(`^(?:Metric|指标)\s*[:：]\s*(.+)$`)
	rules   = []rule{
		{regexp.MustCompile(`^Measured on\s+(\S+)$`), "dataset", func(s *Spec, m []string) (string, *threshold) { s.Dataset = m[1]; return m[1], nil }},
		{regexp.MustCompile(`^在\s*(\S+?)\s*上测量$`), "dataset", func(s *Spec, m []string) (string, *threshold) { s.Dataset = m[1]; return m[1], nil }},
		{regexp.MustCompile(`^(Each case scores .+)$`), "grader", func(s *Spec, m []string) (string, *threshold) { s.Grader = m[1]; return "", nil }},
		{regexp.MustCompile(`^(每个用例.+)$`), "grader", func(s *Spec, m []string) (string, *threshold) { s.Grader = m[1]; return "", nil }},
		{regexp.MustCompile(`^(Higher|Lower) is better$`), "direction", func(s *Spec, m []string) (string, *threshold) {
			s.Direction = strings.ToLower(m[1])
			return m[1], nil
		}},
		{regexp.MustCompile(`^越(高|低)越好$`), "direction", func(s *Spec, m []string) (string, *threshold) {
			s.Direction = map[string]string{"高": "higher", "低": "lower"}[m[1]]
			return m[1], nil
		}},
		{regexp.MustCompile(`^(Blocks a release|Warns) (below|above) ` + level + `$`), "", func(_ *Spec, m []string) (string, *threshold) {
			return m[3], atLevel(m[1] == "Warns", m[2] == "below", m[3])
		}},
		{regexp.MustCompile(`^(低于|高于)\s*` + level + `\s*时(阻止发布|警告)$`), "", func(_ *Spec, m []string) (string, *threshold) {
			return m[2], atLevel(m[3] == "警告", m[1] == "低于", m[2])
		}},
		{regexp.MustCompile(`^(Blocks a release|Warns) when it gets ` + num + `\s*(%|points?|[A-Za-zµ/]+)? worse than the baseline$`), "", func(_ *Spec, m []string) (string, *threshold) {
			return m[2] + m[3], worseBy(m[1] == "Warns", m[2], m[3])
		}},
		{regexp.MustCompile(`^比基线差\s*` + num + `\s*(%|个点|[A-Za-zµ/]+)?\s*时(阻止发布|警告)$`), "", func(_ *Spec, m []string) (string, *threshold) {
			return m[1] + m[2], worseBy(m[3] == "警告", m[1], m[2])
		}},
	}
)

// Parse reads a .metric file; file is its path in the repository, for the ID.
func Parse(file, text string) Spec {
	s := Spec{ID: strings.TrimSuffix(path.Base(file), ".metric")}
	var ths []*threshold
	settings := false
	var desc []string
	for i, raw := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		t := strings.TrimSpace(raw)
		l := Line{N: i + 1, Text: raw}
		switch {
		case t == "":
			l.Kind = "blank"
		case strings.HasPrefix(t, "#"):
			l.Kind = "comment"
		case strings.HasPrefix(t, "@") && s.Name == "":
			l.Kind = "tag"
			s.Tags = append(s.Tags, strings.Fields(t)...)
		case s.Name == "":
			if m := titleRe.FindStringSubmatch(t); m != nil {
				l.Kind, l.Value, s.Name = "title", strings.TrimSpace(m[1]), strings.TrimSpace(m[1])
			} else {
				l.Kind, l.Error = "error", "a .metric file starts with Metric: and its name"
			}
		default:
			matched := false
			for _, r := range rules {
				m := r.re.FindStringSubmatch(t)
				if m == nil {
					continue
				}
				matched, settings = true, true
				var th *threshold
				l.Value, th = r.read(&s, m)
				l.Kind = r.kind
				if th != nil {
					ths = append(ths, th)
					l.Kind = map[bool]string{true: "warn", false: "block"}[th.warn]
				}
				break
			}
			switch {
			case matched:
			case !settings:
				l.Kind = "description"
				desc = append(desc, t)
			default:
				l.Kind, l.Error = "error", "not a setting Shipwake knows; settings come after the description, one per line"
			}
		}
		s.Lines = append(s.Lines, l)
	}
	s.Description = strings.Join(desc, " ")
	for _, th := range ths {
		v := th.level
		if th.worse != "" {
			v = "-" + th.worse
			if s.Direction == "lower" {
				v = "+" + th.worse
			}
		}
		if th.warn {
			s.Warn = joinOr(s.Warn, v)
		} else {
			s.Block = joinOr(s.Block, v)
		}
	}
	return s
}

func joinOr(a, b string) string {
	if a == "" {
		return b
	}
	return a + " or " + b
}
