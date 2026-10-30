// Package mapping converts a tool's report into a Shipwake run from a declarative description: where
// the tests are and which fields hold their name, status and time. Nothing in a mapping executes, so
// a converter can be read and reviewed like any other configuration.
//
//	format: vitest-json
//	about: Vitest's JSON reporter (--reporter=json)
//	input: json                       # json, jsonl (one object per line) or xml
//	tests:
//	  each: testResults[].assertionResults[]
//	  feature: ^.name                 # ^ reaches the enclosing element
//	  name: fullName
//	  status: status                  # common words are understood; add status_map for others
//	  duration: duration              # with duration_unit: ms (default), s, us or ns
//	  error: failureMessages
//	totals:                           # the tool's own counts: the conversion must agree with them
//	  passed: numPassedTests
//	  failed: numFailedTests
//
// Paths are dot-separated keys. name[] visits every element of an array (in a field, it collects
// them), name[2] one element, and * every value of an object (its key is @key). A step that meets an
// array without [] takes the first element, which is how XML children are read. $ starts again at
// the root.
//
// A field can also be "=text" (that text), a template like "{keyword}{name}", or end in filters:
// "nodeid | after(::)". Filters: before(s), after(s) (after the first s), last(s) (after the last s),
// join(s), base (the last part of a path), trim. "a || b" takes b when a is empty. Totals can add
// fields: "numPendingTests + numTodoTests".
package mapping

import (
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/blu9a/shipwake/internal/model"
)

// Spec is one converter.
type Spec struct {
	Format string    `yaml:"format"`
	About  string    `yaml:"about"`
	Input  string    `yaml:"input"` // json (default), jsonl or xml
	Kind   string    `yaml:"kind"`
	Name   string    `yaml:"name"` // the run's name, a path or =text
	Start  string    `yaml:"start"`
	Tests  TestsList `yaml:"tests"` // one place in the report, or a list of places
	// Totals are the tool's own counts, read from the report. Converting checks it found the same.
	Totals  map[string]string `yaml:"totals"`
	Metrics *Metrics          `yaml:"metrics"`
	// Sample and Expected name a real report and the run it converts to, next to the spec.
	Sample   string `yaml:"sample"`
	Expected string `yaml:"expected"`
}

// TestsList is where tests are: one place, or several (a report that lists passed, failed and
// skipped tests separately has three, each with a fixed status such as "=passed").
type TestsList []*Tests

// UnmarshalYAML reads one mapping or a list of them.
func (l *TestsList) UnmarshalYAML(b []byte) error {
	if strings.HasPrefix(strings.TrimSpace(string(b)), "-") {
		var many []*Tests
		if err := yaml.UnmarshalWithOptions(b, &many, yaml.Strict()); err != nil {
			return err
		}
		*l = many
		return nil
	}
	var one Tests
	if err := yaml.UnmarshalWithOptions(b, &one, yaml.Strict()); err != nil {
		return err
	}
	*l = TestsList{&one}
	return nil
}

// Tests says where tests are and which fields describe them.
type Tests struct {
	Each         string            `yaml:"each"`
	Where        string            `yaml:"where"` // keep only these: "type = scenario", "type != background"
	Feature      string            `yaml:"feature"`
	Name         string            `yaml:"name"`
	Target       string            `yaml:"target"`
	Status       string            `yaml:"status"`
	StatusMap    map[string]string `yaml:"status_map"`
	Duration     string            `yaml:"duration"`
	DurationUnit string            `yaml:"duration_unit"`
	Start        string            `yaml:"start"`
	Error        string            `yaml:"error"`
	Steps        *Steps            `yaml:"steps"`
}

// Steps are a test's steps, such as Gherkin steps, relative to the test.
type Steps struct {
	Each         string            `yaml:"each"`
	Name         string            `yaml:"name"`
	Status       string            `yaml:"status"`
	StatusMap    map[string]string `yaml:"status_map"`
	Duration     string            `yaml:"duration"`
	DurationUnit string            `yaml:"duration_unit"`
}

// Metrics says where measurements are.
type Metrics struct {
	Each      string `yaml:"each"`
	ID        string `yaml:"id"`
	Suite     string `yaml:"suite"`
	Name      string `yaml:"name"`
	Unit      string `yaml:"unit"`
	Direction string `yaml:"direction"`
	Value     string `yaml:"value"` // one value, or a path to a list of samples
}

// Load reads a spec file.
func Load(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// Parse reads a spec and checks that it can work.
func Parse(data []byte) (*Spec, error) {
	var s Spec
	if err := yaml.UnmarshalWithOptions(data, &s, yaml.Strict()); err != nil {
		return nil, err
	}
	if s.Format == "" {
		return nil, errors.New("format: a name is required, such as vitest-json")
	}
	if len(s.Tests) == 0 && s.Metrics == nil {
		return nil, errors.New("tests or metrics: say where the report keeps them")
	}
	for _, t := range s.Tests {
		if t.Each == "" || t.Name == "" {
			return nil, errors.New("tests: each and name are required")
		}
		if t.Status == "" && t.Steps == nil {
			return nil, errors.New("tests: status is required, unless steps give it")
		}
	}
	if s.Metrics != nil && (s.Metrics.Each == "" || s.Metrics.ID == "" || s.Metrics.Value == "") {
		return nil, errors.New("metrics: each, id and value are required")
	}
	switch s.Input {
	case "", "json", "jsonl", "xml":
	default:
		return nil, fmt.Errorf("input %q: want json, jsonl or xml", s.Input)
	}
	return &s, nil
}

// Convert reads a report. When the spec names the tool's own totals, it fails if they disagree with
// what was converted: a report the spec no longer understands is rejected rather than recorded wrong.
func (s *Spec) Convert(r io.Reader) (model.Run, error) {
	root, err := decode(r, s.Input)
	if err != nil {
		return model.Run{}, err
	}
	top := &scope{v: root}
	run := model.Run{Kind: s.Kind}
	run.Name = text(top.get(s.Name))
	if t, ok := parseTime(top.get(s.Start)); ok {
		run.Start = t
	}
	for _, ts := range s.Tests {
		for _, sc := range top.each(ts.Each) {
			if !sc.where(ts.Where) {
				continue
			}
			t, err := test(ts, sc)
			if err != nil {
				return run, err
			}
			run.Tests = append(run.Tests, t)
		}
	}
	if s.Metrics != nil {
		for _, sc := range top.each(s.Metrics.Each) {
			m := model.Metric{
				ID: text(sc.get(s.Metrics.ID)), Suite: text(sc.get(s.Metrics.Suite)), Name: text(sc.get(s.Metrics.Name)),
				Unit: text(sc.get(s.Metrics.Unit)), Direction: text(sc.get(s.Metrics.Direction)),
			}
			if m.Name == "" {
				m.Name = m.ID
			}
			if m.Direction == "" {
				m.Direction = "lower"
			}
			for _, v := range list(sc.get(s.Metrics.Value)) {
				if f, ok := number(v); ok {
					m.Values = append(m.Values, f)
				}
			}
			if m.ID != "" && len(m.Values) > 0 {
				run.Metrics = append(run.Metrics, m)
			}
		}
	}
	if run.Kind == "" {
		run.Kind = "test"
		if len(run.Tests) == 0 && len(run.Metrics) > 0 {
			run.Kind = "measurement"
		}
	}
	if err := s.checkTotals(top, run); err != nil {
		return run, err
	}
	if len(run.Tests) == 0 && len(run.Metrics) == 0 {
		return run, errors.New("found no tests or measurements: is this the right report, and does tests.each point at them?")
	}
	return run, nil
}

func test(ts *Tests, sc *scope) (model.Test, error) {
	t := model.Test{
		Feature: text(sc.get(ts.Feature)), Name: text(sc.get(ts.Name)), Target: text(sc.get(ts.Target)),
		Attempts: 1, Error: strings.Join(texts(sc.get(ts.Error)), "\n"),
	}
	var err error
	if ts.Status != "" {
		v := text(sc.get(ts.Status))
		if v == "" {
			return t, fmt.Errorf("test %q has no status at %q: is that the right field?", t.Name, ts.Status)
		}
		if t.Status, err = status(v, ts.StatusMap); err != nil {
			return t, fmt.Errorf("test %q: %w", t.Name, err)
		}
	}
	if d, ok := number(sc.get(ts.Duration)); ok {
		t.Duration = millis(d, ts.DurationUnit)
	}
	if v, ok := parseTime(sc.get(ts.Start)); ok {
		t.Start = v
	}
	if ts.Steps != nil {
		for _, ss := range sc.each(ts.Steps.Each) {
			step := model.Step{Name: text(ss.get(ts.Steps.Name))}
			if step.Status, err = status(text(ss.get(ts.Steps.Status)), ts.Steps.StatusMap); err != nil {
				return t, fmt.Errorf("test %q, step %q: %w", t.Name, step.Name, err)
			}
			if d, ok := number(ss.get(ts.Steps.Duration)); ok {
				step.Duration = millis(d, ts.Steps.DurationUnit)
			}
			t.Steps = append(t.Steps, step)
		}
		// A scenario's status follows its steps when the report has no status of its own.
		if ts.Status == "" {
			t.Status = "passed"
			for _, step := range t.Steps {
				if step.Status == "failed" {
					t.Status = "failed"
					break
				}
				if step.Status == "skipped" {
					t.Status = "skipped"
				}
			}
		}
	}
	if t.Status != "failed" {
		t.Error = "" // tools also put skip reasons and logs there; an error only explains a failure
	}
	if t.Duration == 0 {
		for _, step := range t.Steps {
			t.Duration += step.Duration
		}
	}
	return t, nil
}

func (s *Spec) checkTotals(top *scope, run model.Run) error {
	if len(s.Totals) == 0 {
		return nil
	}
	got := map[string]int{}
	for _, t := range run.Tests {
		got[t.Status]++
	}
	got["total"] = len(run.Tests)
	var diffs []string
	for _, k := range []string{"total", "passed", "failed", "skipped"} {
		path, ok := s.Totals[k]
		if !ok {
			continue
		}
		want := 0.0
		for _, term := range strings.Split(path, "+") {
			v := top.get(strings.TrimSpace(term))
			if v == nil {
				continue // reports often leave out counts that are zero
			}
			n, ok := number(v)
			if !ok {
				return fmt.Errorf("totals.%s: %q is not a number in this report", k, term)
			}
			want += n
		}
		if int(want) != got[k] {
			diffs = append(diffs, fmt.Sprintf("%s: the report says %d, converted %d", k, int(want), got[k]))
		}
	}
	if len(diffs) > 0 {
		return fmt.Errorf("the conversion disagrees with the tool's own counts (%s); the report's shape may have changed, so nothing was recorded", strings.Join(diffs, "; "))
	}
	return nil
}

// statuses are the words tools use, by meaning.
var statuses = map[string]string{
	"passed": "passed", "pass": "passed", "success": "passed", "succeeded": "passed", "ok": "passed", "green": "passed",
	"failed": "failed", "fail": "failed", "failure": "failed", "error": "failed", "errored": "failed", "broken": "failed", "timedout": "failed", "timeout": "failed",
	"skipped": "skipped", "skip": "skipped", "pending": "skipped", "todo": "skipped", "disabled": "skipped", "ignored": "skipped",
	"notexecuted": "skipped", "undefined": "skipped", "ambiguous": "failed", "xfail": "skipped", "xfailed": "skipped", "xpass": "passed", "xpassed": "passed",
}

func status(v string, custom map[string]string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(v))
	if s, ok := custom[v]; ok {
		return s, nil
	}
	if s, ok := custom[key]; ok {
		return s, nil
	}
	if s, ok := statuses[strings.NewReplacer(" ", "", "_", "", "-", "").Replace(key)]; ok {
		return s, nil
	}
	return "", fmt.Errorf("status %q is not one Shipwake knows: map it with status_map, e.g. %s: passed", v, v)
}

func millis(v float64, unit string) float64 {
	switch unit {
	case "s":
		return v * 1000
	case "us", "µs":
		return v / 1000
	case "ns":
		return v / 1e6
	}
	return v
}

// decode reads json, jsonl (into a list) or xml (into objects: attributes as @name, text as #text,
// children as lists by element name).
func decode(r io.Reader, input string) (any, error) {
	switch input {
	case "jsonl":
		var all []any
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 1<<20), 64<<20)
		for sc.Scan() {
			line := bytes.TrimSpace(sc.Bytes())
			if len(line) == 0 {
				continue
			}
			var v any
			if err := json.Unmarshal(line, &v); err != nil {
				return nil, fmt.Errorf("line %d: %w", len(all)+1, err)
			}
			all = append(all, v)
		}
		return all, sc.Err()
	case "xml":
		return decodeXML(r)
	default:
		var v any
		d := json.NewDecoder(r)
		d.UseNumber()
		if err := d.Decode(&v); err != nil {
			return nil, err
		}
		return v, nil
	}
}

func decodeXML(r io.Reader) (any, error) {
	d := xml.NewDecoder(r)
	root := map[string]any{}
	stack := []map[string]any{root}
	var texts []*strings.Builder
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			return root, nil
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			el := map[string]any{}
			for _, a := range t.Attr {
				el["@"+a.Name.Local] = a.Value
			}
			parent := stack[len(stack)-1]
			kids, _ := parent[t.Name.Local].([]any)
			parent[t.Name.Local] = append(kids, el)
			stack = append(stack, el)
			texts = append(texts, &strings.Builder{})
		case xml.CharData:
			if len(texts) > 0 {
				texts[len(texts)-1].Write(t)
			}
		case xml.EndElement:
			el := stack[len(stack)-1]
			if s := strings.TrimSpace(texts[len(texts)-1].String()); s != "" {
				el["#text"] = s
			}
			stack, texts = stack[:len(stack)-1], texts[:len(texts)-1]
		}
	}
}

// scope is a place in the report: a value, the key it was found under, and the element around it.
type scope struct {
	v      any
	key    string
	parent *scope
}

func (s *scope) root() *scope {
	for s.parent != nil {
		s = s.parent
	}
	return s
}

// get reads a field: alternatives (||), each a source and filters (|). See the package comment.
func (s *scope) get(expr string) any {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil
	}
	if strings.HasPrefix(expr, "=") {
		return expr[1:]
	}
	var v any
	for _, alt := range strings.Split(expr, "||") {
		parts := strings.Split(alt, " | ")
		v = s.source(strings.TrimSpace(parts[0]))
		for _, f := range parts[1:] {
			v = filter(v, strings.TrimSpace(f))
		}
		if strings.TrimSpace(text(v)) != "" {
			return v
		}
	}
	return v
}

// source is a template ("{keyword}{name}") or a path.
func (s *scope) source(src string) any {
	if strings.Contains(src, "{") {
		var b strings.Builder
		for src != "" {
			i := strings.IndexByte(src, '{')
			j := strings.IndexByte(src, '}')
			if i < 0 || j < i {
				b.WriteString(src)
				break
			}
			b.WriteString(src[:i])
			var parts []string
			for _, x := range list(s.path(src[i+1 : j])) {
				if t := text(x); t != "" {
					parts = append(parts, t)
				}
			}
			b.WriteString(strings.Join(parts, " › "))
			src = src[j+1:]
		}
		return b.String()
	}
	return s.path(src)
}

// path reads a path from here: ^ steps out, $ starts at the root, @key is the key of a * visit.
func (s *scope) path(path string) any {
	path = strings.TrimSpace(path)
	switch {
	case path == "":
		return nil
	case path == "@key":
		return s.key
	}
	at := s
	if rest, ok := strings.CutPrefix(path, "$"); ok {
		at, path = s.root(), strings.TrimPrefix(rest, ".")
	}
	for strings.HasPrefix(path, "^") {
		if at.parent != nil {
			at = at.parent
		}
		path = strings.TrimPrefix(strings.TrimPrefix(path, "^"), ".")
	}
	if path == "@key" {
		return at.key
	}
	if path == "" {
		return at.v
	}
	vals := []any{at.v}
	collect := false
	for _, step := range strings.Split(path, ".") {
		var next []any
		for _, v := range vals {
			if name, ok := strings.CutSuffix(step, "[]"); ok {
				collect = true
				if name != "" {
					v = walk(v, name)
				}
				next = append(next, list(v)...)
				continue
			}
			if w := walk(v, step); w != nil {
				next = append(next, w)
			}
		}
		vals = next
	}
	if collect {
		return vals
	}
	if len(vals) == 0 {
		return nil
	}
	return vals[0]
}

// filter applies one text filter; a list is filtered element by element, except by join.
func filter(v any, f string) any {
	name, arg, _ := strings.Cut(f, "(")
	arg = strings.TrimSuffix(arg, ")")
	if l, ok := v.([]any); ok && name != "join" {
		out := make([]any, 0, len(l))
		for _, x := range l {
			out = append(out, filter(x, f))
		}
		return out
	}
	t := text(v)
	switch name {
	case "join":
		return strings.Join(texts(v), arg)
	case "before":
		before, _, _ := strings.Cut(t, arg)
		return before
	case "after":
		if _, after, ok := strings.Cut(t, arg); ok {
			return after
		}
		return t
	case "last":
		if i := strings.LastIndex(t, arg); i >= 0 {
			return t[i+len(arg):]
		}
		return t
	case "base":
		return t[strings.LastIndexAny(t, `/\`)+1:]
	case "trim":
		return strings.TrimSpace(t)
	}
	return v
}

// where tests a condition "field = value" or "field != value"; empty is always true.
func (s *scope) where(cond string) bool {
	if strings.TrimSpace(cond) == "" {
		return true
	}
	if l, r, ok := strings.Cut(cond, "!="); ok {
		return text(s.get(strings.TrimSpace(l))) != strings.TrimSpace(r)
	}
	if l, r, ok := strings.Cut(cond, "="); ok {
		return text(s.get(strings.TrimSpace(l))) == strings.TrimSpace(r)
	}
	return true
}

// each visits every element a path with [] or * reaches; each visit's parent is the element around it.
func (s *scope) each(path string) []*scope {
	at := s
	if rest, ok := strings.CutPrefix(path, "$"); ok {
		at, path = s.root(), strings.TrimPrefix(rest, ".")
	}
	cur := []*scope{at}
	for _, step := range strings.Split(path, ".") {
		var next []*scope
		for _, sc := range cur {
			name, iter := step, ""
			if strings.HasSuffix(step, "[]") {
				name, iter = strings.TrimSuffix(step, "[]"), "[]"
			}
			if step == "*" {
				name, iter = "", "*"
			}
			v := sc.v
			if name != "" {
				v = walk(v, name)
			}
			switch iter {
			case "[]":
				for _, x := range list(v) {
					next = append(next, &scope{v: x, parent: sc})
				}
			case "*":
				if m, ok := v.(map[string]any); ok {
					for _, k := range sortedKeys(m) {
						next = append(next, &scope{v: m[k], key: k, parent: sc})
					}
				}
			default:
				if v != nil {
					next = append(next, &scope{v: v, key: sc.key, parent: sc.parent})
				}
			}
		}
		cur = next
	}
	return cur
}

// walk takes one step: a key, or key[n]. Arrays met along the way give their first element.
func walk(v any, step string) any {
	name, index := step, -1
	if i := strings.IndexByte(step, '['); i >= 0 && strings.HasSuffix(step, "]") {
		n, err := strconv.Atoi(step[i+1 : len(step)-1])
		if err == nil {
			name, index = step[:i], n
		}
	}
	if a, ok := v.([]any); ok && len(a) > 0 && name != "" {
		v = a[0]
	}
	if name != "" {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[name]
	}
	if index >= 0 {
		a := list(v)
		if index >= len(a) {
			return nil
		}
		return a[index]
	}
	return v
}

func list(v any) []any {
	switch x := v.(type) {
	case nil:
		return nil
	case []any:
		return x
	default:
		return []any{x}
	}
}

func text(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case json.Number:
		return x.String()
	case []any:
		if len(x) == 0 {
			return ""
		}
		return text(x[0])
	case map[string]any:
		return text(x["#text"])
	case bool:
		return strconv.FormatBool(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}

func texts(v any) []string {
	var out []string
	for _, x := range list(v) {
		if s := strings.TrimSpace(text(x)); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func number(v any) (float64, bool) {
	s := strings.TrimSpace(text(v))
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err == nil && !math.IsNaN(f) {
		return f, true
	}
	// .NET-style durations: 00:00:01.2340000
	if d, ok := clock(s); ok {
		return d, true
	}
	return 0, false
}

// clock reads hh:mm:ss(.fraction) as milliseconds.
func clock(s string) (float64, bool) {
	p := strings.Split(s, ":")
	if len(p) != 3 {
		return 0, false
	}
	h, e1 := strconv.Atoi(p[0])
	m, e2 := strconv.Atoi(p[1])
	sec, e3 := strconv.ParseFloat(p[2], 64)
	if e1 != nil || e2 != nil || e3 != nil {
		return 0, false
	}
	return (float64(h)*3600 + float64(m)*60 + sec) * 1000, true
}

func parseTime(v any) (time.Time, bool) {
	s := strings.TrimSpace(text(v))
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.9999999", "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && f > 1e9 {
		if f > 1e12 { // milliseconds since 1970
			return time.UnixMilli(int64(f)).UTC(), true
		}
		return time.Unix(int64(f), 0).UTC(), true
	}
	return time.Time{}, false
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys) // a stable order: the same report converts to the same run
	return keys
}

// Check converts the spec's sample report and compares the run with the expected one, both named in
// the spec and read with read. With update, it writes the expected run instead. It is how a converter
// shows it still reads its tool's output the way a person reviewed.
func (s *Spec) Check(read func(string) ([]byte, error), write func(string, []byte) error, update bool) error {
	if s.Sample == "" || s.Expected == "" {
		return errors.New("no sample: add a real report as sample and its converted run as expected")
	}
	data, err := read(s.Sample)
	if err != nil {
		return fmt.Errorf("sample: %w", err)
	}
	run, err := s.Convert(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("sample %s: %w", s.Sample, err)
	}
	got, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	got = append(got, '\n')
	if update {
		return write(s.Expected, got)
	}
	want, err := read(s.Expected)
	if err != nil {
		return fmt.Errorf("expected: %w (run with --update to write it, then review it)", err)
	}
	if !bytes.Equal(bytes.TrimSpace(got), bytes.TrimSpace(want)) {
		return fmt.Errorf("%s no longer converts to %s: %s", s.Sample, s.Expected, firstDiff(want, got))
	}
	return nil
}

// firstDiff names the first line that differs, enough to start looking.
func firstDiff(want, got []byte) string {
	w, g := strings.Split(string(want), "\n"), strings.Split(string(got), "\n")
	for i := 0; i < len(w) || i < len(g); i++ {
		var a, b string
		if i < len(w) {
			a = strings.TrimSpace(w[i])
		}
		if i < len(g) {
			b = strings.TrimSpace(g[i])
		}
		if a != b {
			return fmt.Sprintf("line %d: expected %q, got %q", i+1, a, b)
		}
	}
	return "only whitespace differs"
}
