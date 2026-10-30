package readers

import (
	"strings"
	"testing"
)

func TestJUnit(t *testing.T) {
	in := `<?xml version="1.0"?>
<testsuites>
  <testsuite name="api" timestamp="2026-01-01T10:00:00">
    <testcase classname="api.plans" name="lists plans" time="0.120"/>
    <testcase classname="api.plans" name="rejects bad ids" time="0.050"><failure message="expected 400">got 500</failure></testcase>
    <testcase classname="api.tasks" name="later" time="0"><skipped/></testcase>
  </testsuite>
</testsuites>`
	r, _ := Get("junit")
	run, err := r.Read(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Tests) != 3 {
		t.Fatalf("tests = %d", len(run.Tests))
	}
	f := run.Tests[1]
	if f.Status != "failed" || f.Feature != "api.plans" || f.Duration != 50 || !strings.Contains(f.Error, "expected 400") || !strings.Contains(f.Error, "got 500") {
		t.Errorf("failed test = %+v", f)
	}
	if run.Tests[2].Status != "skipped" {
		t.Errorf("skipped test = %+v", run.Tests[2])
	}
}

func TestGoTest(t *testing.T) {
	in := `{"Time":"2026-01-01T10:00:00Z","Action":"start","Package":"x/parse"}
{"Time":"2026-01-01T10:00:00.001Z","Action":"run","Package":"x/parse","Test":"TestLines"}
{"Time":"2026-01-01T10:00:00.002Z","Action":"pass","Package":"x/parse","Test":"TestLines","Elapsed":0.01}
{"Time":"2026-01-01T10:00:00.003Z","Action":"run","Package":"x/parse","Test":"TestQuote"}
{"Time":"2026-01-01T10:00:00.003Z","Action":"output","Package":"x/parse","Test":"TestQuote","Output":"=== RUN   TestQuote\n"}
{"Time":"2026-01-01T10:00:00.004Z","Action":"output","Package":"x/parse","Test":"TestQuote","Output":"    parse_test.go:9: want Ada\n"}
{"Time":"2026-01-01T10:00:00.005Z","Action":"fail","Package":"x/parse","Test":"TestQuote","Elapsed":0}
{"Time":"2026-01-01T10:00:00.006Z","Action":"fail","Package":"x/parse","Elapsed":0.006}`
	r, _ := Get("go-test")
	run, err := r.Read(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Tests) != 2 || run.Tests[1].Status != "failed" || run.Tests[1].Error != "parse_test.go:9: want Ada" || run.Tests[0].Duration != 10 {
		t.Errorf("tests = %+v", run.Tests)
	}
	if run.Duration != 6 {
		t.Errorf("duration = %v", run.Duration)
	}
}

func TestPlaywright(t *testing.T) {
	in := `{"stats":{"startTime":"2026-01-01T10:00:00Z","duration":9000},"suites":[{"title":"deciding.spec.ts","specs":[],"suites":[
	  {"title":"I decide on every suggestion","specs":[{"title":"Accepting applies it","tests":[
	    {"projectName":"desktop","results":[{"status":"failed","startTime":"2026-01-01T10:00:01Z","duration":900,"error":{"message":"boom"}},
	                                       {"status":"passed","startTime":"2026-01-01T10:00:02Z","duration":800,"steps":[{"title":"Given a goal","duration":10}]}]}]}]}]}]}`
	r, _ := Get("playwright")
	run, err := r.Read(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Tests) != 1 {
		t.Fatalf("tests = %d", len(run.Tests))
	}
	got := run.Tests[0]
	if got.Feature != "I decide on every suggestion" || got.Target != "desktop" || got.Status != "passed" || got.Attempts != 2 || len(got.Steps) != 1 {
		t.Errorf("test = %+v", got)
	}
}

func TestGoBench(t *testing.T) {
	in := `goos: linux
cpu: Some CPU
BenchmarkLines-96    	   23246	     65569 ns/op	    8064 B/op	     204 allocs/op
BenchmarkLines-96    	   19620	     61959 ns/op	    8064 B/op	     204 allocs/op
BenchmarkSet-96      	 8000000	       137 ns/op
PASS`
	r, _ := Get("go-bench")
	run, err := r.Read(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string][]float64{}
	for _, m := range run.Metrics {
		ids[m.ID] = m.Values
	}
	if len(ids["Lines.time"]) != 2 || ids["Lines.allocs"][0] != 204 || ids["Set.time"][0] != 137 || run.Context["cpu"] != "Some CPU" {
		t.Errorf("metrics = %v context = %v", ids, run.Context)
	}
}

func TestK6(t *testing.T) {
	in := `{"metrics":{"http_req_duration":{"avg":40,"med":35,"p(95)":90},"http_req_duration{name:GET /plans}":{"med":30,"p(95)":80},
	"http_req_failed":{"rate":0.004},"http_reqs":{"count":1000,"rate":120.5},"checks":{"value":0.99}}}`
	r, _ := Get("k6")
	run, err := r.Read(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, m := range run.Metrics {
		got[m.ID] = m.Suite + "|" + m.Direction
	}
	for id, want := range map[string]string{
		"http_req_duration.p95":                  "http_req_duration|lower",
		"http_req_duration{name:GET_/plans}.p95": "name:GET /plans|lower",
		"http_req_failed.rate":                   "http_req_failed|lower",
		"http_reqs.rate":                         "http_reqs|higher",
	} {
		if got[id] != want {
			t.Errorf("%s = %q, want %q (all: %v)", id, got[id], want, got)
		}
	}
}

func TestOwn(t *testing.T) {
	in := `{"name":"Agent evals","metrics":[{"id":"quality","name":"Suggestion quality","unit":"score","direction":"higher","values":[0.86,0.88]}],"context":{"model":"m-1"}}`
	r, _ := Get("json")
	run, err := r.Read(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if run.Kind != "measurement" || len(run.Metrics) != 1 || run.Context["model"] != "m-1" {
		t.Errorf("run = %+v", run)
	}
}
