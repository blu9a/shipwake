package judge

import "testing"

func TestThreshold(t *testing.T) {
	f := func(x float64) *float64 { return &x }
	cases := []struct {
		th        string
		v         float64
		base      *float64
		unit, dir string
		hit, part bool
	}{
		// levels: bare numbers follow the direction; < and > are explicit
		{"0.88", 0.86, nil, "score", "higher", true, false},
		{"0.88", 0.90, nil, "score", "higher", false, false},
		{"4000", 4100, nil, "ms", "lower", true, false},
		{"> 4s", 4100, nil, "ms", "lower", true, false},
		{"< 0.5", 0.4, nil, "", "lower", true, false},
		{"< 99.5%", 99.1, nil, "%", "higher", true, false},
		// percentage changes
		{"+15%", 116, f(100), "ms", "lower", true, false},
		{"+15%", 114, f(100), "ms", "lower", false, false},
		{"-10%", 89, f(100), "req/s", "higher", true, false},
		{"±15%", 84, f(100), "B", "lower", true, false},
		// amounts, converted from their unit; none means the metric's own ("+0.5" points on a % metric)
		{"+20ms", 121, f(100), "ms", "lower", true, false},
		{"+20ms", 119, f(100), "ms", "lower", false, false},
		{"+20ms", 121_000_000, f(100_000_000), "ns", "lower", true, false},
		{"+1MiB", 3 << 20, f(1 << 20), "B", "lower", true, false},
		{"+0.5", 1.6, f(1.0), "%", "lower", true, false},
		{"-0.03", 0.86, f(0.90), "score", "higher", true, false},
		// and: both must hold; or: either
		{"+15% and +20ms", 3, f(2), "ms", "lower", false, false},         // +50%, but only 1 ms
		{"+15% and +20ms", 2020, f(2000), "ms", "lower", false, false},   // 20 ms, but only 1%
		{"+15% and +20ms", 140, f(100), "ms", "lower", true, false},      // both
		{"+40% or +200ms", 2300, f(2000), "ms", "lower", true, false},    // 300 ms
		{"+40% or +200ms", 1.5, f(1), "ms", "lower", true, false},        // +50%
		{"< 0.82 or -5%", 0.85, f(0.90), "score", "higher", true, false}, // fell 5.6%
		{"< 0.82 or -5%", 0.80, nil, "score", "higher", true, true},      // level holds; change unknown
		{"a or b and c", 0, nil, "", "", false, false},                   // invalid: no threshold (below)
		{"+15% and < 3", 2, f(1), "ms", "lower", true, false},
		{"+15% or 0.9 and +0.005", 0.85, f(0.84), "score", "higher", true, false}, // and binds first
		// changes need a baseline; units must convert
		{"+15%", 200, nil, "ms", "lower", false, true},
		{"+20ms", 200, f(100), "req/s", "higher", false, true},
	}
	for _, c := range cases {
		th, err := ParseThreshold(c.th)
		if c.th == "a or b and c" {
			if err == nil {
				t.Errorf("%q: want a parse error", c.th)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: %v", c.th, err)
			continue
		}
		hit, part := th.Hit(c.v, c.base, c.unit, c.dir)
		if hit != c.hit || part != c.part {
			t.Errorf("%q with %v (base %v, %s, %s): hit %v partial %v, want %v %v", c.th, c.v, deref(c.base), c.unit, c.dir, hit, part, c.hit, c.part)
		}
	}
}

func TestParseThresholdErrors(t *testing.T) {
	for _, s := range []string{"fast", "+", "< +5", "+5 or"} {
		if _, err := ParseThreshold(s); err == nil {
			t.Errorf("%q: want an error", s)
		}
	}
	if th, err := ParseThreshold("  "); th != nil || err != nil {
		t.Errorf("empty: %v %v", th, err)
	}
}

func deref(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}
