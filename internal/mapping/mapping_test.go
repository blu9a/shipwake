package mapping

import (
	"strings"
	"testing"
)

func convert(t *testing.T, spec, report string) (string, error) {
	t.Helper()
	s, err := Parse([]byte(spec))
	if err != nil {
		t.Fatalf("spec: %v", err)
	}
	run, err := s.Convert(strings.NewReader(report))
	var b strings.Builder
	for _, x := range run.Tests {
		b.WriteString(x.Status + " " + x.Feature + "/" + x.Name)
		if x.Error != "" {
			b.WriteString(" err=" + x.Error)
		}
		for _, st := range x.Steps {
			b.WriteString(" [" + st.Name + ":" + st.Status + "]")
		}
		b.WriteString("\n")
	}
	for _, m := range run.Metrics {
		b.WriteString(m.ID + "=" + strings.TrimSpace(strings.Repeat("x", len(m.Values))) + " " + m.Unit + "\n")
	}
	return b.String(), err
}

func TestXML(t *testing.T) {
	// The shape of .NET's TRX: results with attributes, error text in nested elements.
	spec := `
format: trx
input: xml
tests:
  each: TestRun.Results.UnitTestResult[]
  name: "@testName"
  status: "@outcome"
  duration: "@duration"
  error: Output.ErrorInfo.Message
totals:
  total: TestRun.ResultSummary.Counters.@total
  passed: TestRun.ResultSummary.Counters.@passed
  failed: TestRun.ResultSummary.Counters.@failed
`
	report := `<?xml version="1.0"?>
<TestRun><Results>
  <UnitTestResult testName="Adds" outcome="Passed" duration="00:00:00.0123"/>
  <UnitTestResult testName="Divides" outcome="Failed" duration="00:00:01.5"><Output><ErrorInfo><Message>Expected 2</Message></ErrorInfo></Output></UnitTestResult>
  <UnitTestResult testName="Later" outcome="NotExecuted"/>
</Results><ResultSummary><Counters total="3" passed="1" failed="1"/></ResultSummary></TestRun>`
	got, err := convert(t, spec, report)
	want := "passed /Adds\nfailed /Divides err=Expected 2\nskipped /Later\n"
	if err != nil || got != want {
		t.Errorf("got %q, %v; want %q", got, err, want)
	}
}

func TestTotalsMustAgree(t *testing.T) {
	spec := `
format: x
tests: { each: "tests[]", name: name, status: ok }
totals: { total: count }
`
	_, err := convert(t, spec, `{"count": 3, "tests": [{"name": "a", "ok": "pass"}, {"name": "b", "ok": "fail"}]}`)
	if err == nil || !strings.Contains(err.Error(), "report says 3, converted 2") {
		t.Errorf("want a totals mismatch, got %v", err)
	}
}

func TestUnknownStatus(t *testing.T) {
	spec := `
format: x
tests: { each: "tests[]", name: name, status: s }
`
	_, err := convert(t, spec, `{"tests": [{"name": "a", "s": "flaky-ish"}]}`)
	if err == nil || !strings.Contains(err.Error(), "status_map") {
		t.Errorf("want advice to use status_map, got %v", err)
	}
	spec2 := `
format: x
tests: { each: "tests[]", name: name, status: s, status_map: { flaky-ish: failed } }
`
	got, err := convert(t, spec2, `{"tests": [{"name": "a", "s": "flaky-ish"}]}`)
	if err != nil || got != "failed /a\n" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestPathsTemplatesFilters(t *testing.T) {
	spec := `
format: x
input: jsonl
tests:
  each: "[]"
  where: type != note
  feature: id | before(::) | base
  name: id | after(::)
  status: outcome
  error: problems[].text | join(; )
  steps:
    each: steps[]
    name: "{kw}{text}"
    status: "=passed"
`
	report := `{"type":"test","id":"src/a/cart.py::adds","outcome":"passed","steps":[{"kw":"Given ","text":"a cart"}]}
{"type":"note","id":"ignored"}
{"type":"test","id":"src/a/cart.py::Tax::rounds","outcome":"failed","problems":[{"text":"off by 1"},{"text":"twice"}]}
`
	got, err := convert(t, spec, report)
	want := "passed cart.py/adds [Given a cart:passed]\nfailed cart.py/Tax::rounds err=off by 1; twice\n"
	if err != nil || got != want {
		t.Errorf("got %q, %v; want %q", got, err, want)
	}
}

func TestMetricsWithKeys(t *testing.T) {
	spec := `
format: x
metrics:
  each: results.*
  id: "@key"
  value: samples
  unit: "=ms"
`
	got, err := convert(t, spec, `{"results": {"parse": {"samples": [1, 2, 3]}, "load": {"samples": 4}}}`)
	want := "load=x ms\nparse=xxx ms\n"
	if err != nil || got != want {
		t.Errorf("got %q, %v; want %q", got, err, want)
	}
}

func TestStepsGiveStatus(t *testing.T) {
	spec := `
format: x
tests:
  each: "scenarios[]"
  name: name
  steps: { each: "steps[]", name: name, status: result }
`
	got, err := convert(t, spec, `{"scenarios": [{"name": "a", "steps": [{"name": "s1", "result": "passed"}, {"name": "s2", "result": "failed"}]}]}`)
	if err != nil || !strings.HasPrefix(got, "failed /a") {
		t.Errorf("a failing step should fail its scenario: %q, %v", got, err)
	}
}

func TestParseErrors(t *testing.T) {
	for _, spec := range []string{
		"tests: {each: x, name: y, status: z}", // no format
		"format: x",                            // nothing to read
		"format: x\ntests: {each: x, name: y}", // no status and no steps
		"format: x\ninput: csv\ntests: {each: x, name: y, status: z}",
		"format: x\ntests: {each: x, name: y, status: z, colour: red}", // unknown key: strict
	} {
		if _, err := Parse([]byte(spec)); err == nil {
			t.Errorf("%q: want an error", spec)
		}
	}
}
