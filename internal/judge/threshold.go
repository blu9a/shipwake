package judge

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// A threshold is one or more conditions joined by "and" / "or" ("and" binds first):
//
//	0.88  or  < 0.88  > 4000     the value itself crosses a level; a bare number is read in the
//	                              metric's direction (below it for higher-is-better, above otherwise)
//	+20ms  -0.03  ±5             the value changed by this amount from the baseline; with no unit,
//	                              in the metric's own unit ("+0.5" on a % metric is 0.5 points)
//	+15%  -10%  ±15%             the value changed by this percentage of the baseline
//
// Change conditions need a baseline. On a run without one they can't hold, and the comparison
// says it was judged in part.
type Threshold [][]Cond // any of these groups, each needing all of its conditions

// Cond is one condition of a threshold.
type Cond struct {
	Kind  byte    // 'l' level, 'a' change by amount, 'p' change by percentage
	Op    byte    // level: '<', '>' or 0 (by direction); change: '+', '-' or '~' (either way)
	Value float64 // in the metric's unit after conversion, or a percentage
	Unit  string  // as written, converted when judged
}

var (
	splitOr  = regexp.MustCompile(`(?i)\s+or\s+`)
	splitAnd = regexp.MustCompile(`(?i)\s+and\s+`)
	term     = regexp.MustCompile(`^(<=?|>=?)?\s*([+\-±]?)\s*(\d+(?:\.\d*)?|\.\d+)\s*(%|[A-Za-zµ/]+)?$`)
)

// ParseThreshold reads a threshold; an empty string is no threshold.
func ParseThreshold(s string) (Threshold, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	var th Threshold
	for _, group := range splitOr.Split(s, -1) {
		var all []Cond
		for _, t := range splitAnd.Split(strings.TrimSpace(group), -1) {
			c, err := parseCond(strings.TrimSpace(t))
			if err != nil && strings.TrimSpace(t) != s {
				return nil, fmt.Errorf("threshold %q: %w", s, err)
			}
			if err != nil {
				return nil, err
			}
			all = append(all, c)
		}
		th = append(th, all)
	}
	return th, nil
}

func parseCond(t string) (Cond, error) {
	m := term.FindStringSubmatch(t)
	if m == nil {
		return Cond{}, fmt.Errorf("%q is not a level (0.88, < 0.88), an amount (+20ms) or a percentage (+15%%)", t)
	}
	cmp, sign, num, unit := m[1], m[2], m[3], m[4]
	if u := strings.ToLower(unit); u == "and" || u == "or" {
		return Cond{}, fmt.Errorf("%q: a condition is missing after %q", t, unit)
	}
	v, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return Cond{}, fmt.Errorf("%q: %w", t, err)
	}
	switch {
	case cmp != "" && sign != "":
		return Cond{}, fmt.Errorf("%q: a level (<, >) or a change (+, -), not both", t)
	case cmp != "":
		if unit == "%" {
			return Cond{Kind: 'l', Op: cmp[0], Value: v}, nil // a level on a % metric: "< 99.5%"
		}
		return Cond{Kind: 'l', Op: cmp[0], Value: v, Unit: unit}, nil
	case sign == "":
		return Cond{Kind: 'l', Value: v, Unit: strings.TrimSuffix(unit, "%")}, nil
	}
	op := map[string]byte{"+": '+', "-": '-', "±": '~'}[sign]
	if unit == "%" {
		return Cond{Kind: 'p', Op: op, Value: v}, nil
	}
	return Cond{Kind: 'a', Op: op, Value: v, Unit: unit}, nil
}

// NeedsBaseline reports whether every condition compares with a baseline.
func (th Threshold) NeedsBaseline() bool {
	for _, g := range th {
		for _, c := range g {
			if c.Kind == 'l' {
				return false
			}
		}
	}
	return len(th) > 0
}

// Hit reports whether the threshold is crossed. partial is true when some conditions couldn't be
// judged for lack of a baseline or a convertible unit; those count as not holding.
func (th Threshold) Hit(v float64, base *float64, unit, direction string) (hit, partial bool) {
	for _, g := range th {
		all := len(g) > 0
		for _, c := range g {
			ok, known := c.holds(v, base, unit, direction)
			if !known {
				partial = true
			}
			all = all && ok && known
		}
		if all {
			hit = true
		}
	}
	return hit, partial
}

func (c Cond) holds(v float64, base *float64, unit, direction string) (ok, known bool) {
	x, convertible := convert(c.Value, c.Unit, unit)
	if c.Kind != 'p' && !convertible {
		return false, false
	}
	switch c.Kind {
	case 'l':
		switch c.Op {
		case '<':
			return v < x, true
		case '>':
			return v > x, true
		}
		if direction == "higher" {
			return v < x, true
		}
		return v > x, true
	case 'a', 'p':
		if base == nil {
			return false, false
		}
		d := v - *base
		if c.Kind == 'p' {
			if *base == 0 {
				return false, false
			}
			d = d / math.Abs(*base) * 100
		}
		switch c.Op {
		case '+':
			return d >= x, true
		case '-':
			return d <= -x, true
		default:
			return math.Abs(d) >= x, true
		}
	}
	return false, false
}

// units by family, each in the family's smallest unit.
var units = []map[string]float64{
	{"ns": 1, "µs": 1e3, "us": 1e3, "ms": 1e6, "s": 1e9, "min": 60e9, "h": 3600e9},
	{"B": 1, "KiB": 1 << 10, "MiB": 1 << 20, "GiB": 1 << 30, "TiB": 1 << 40, "KB": 1e3, "MB": 1e6, "GB": 1e9, "TB": 1e12},
}

// convert expresses x, written in unit from, in the metric's unit to. No unit means the metric's.
func convert(x float64, from, to string) (float64, bool) {
	if from == "" || from == to {
		return x, true
	}
	for _, fam := range units {
		f, ok1 := fam[from]
		t, ok2 := fam[to]
		if ok1 && ok2 {
			return x * f / t, true
		}
	}
	return 0, false
}
