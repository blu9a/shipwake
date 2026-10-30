package readers

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/blu9a/shipwake/internal/model"
)

type goEvent struct {
	Time    time.Time
	Action  string
	Package string
	Test    string
	Elapsed float64
	Output  string
}

// readGoTest reads `go test -json`. Each package is a group; subtests are folded into their parent.
func readGoTest(r io.Reader) (model.Run, error) {
	type key struct{ pkg, test string }
	started, out := map[key]time.Time{}, map[key][]string{}
	run := model.Run{}
	var first, last time.Time
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		var e goEvent
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		if first.IsZero() {
			first = e.Time
		}
		last = e.Time
		if e.Test == "" {
			continue
		}
		k := key{e.Package, e.Test}
		switch e.Action {
		case "run":
			started[k] = e.Time
		case "output":
			if !strings.HasPrefix(e.Output, "=== ") && !strings.HasPrefix(e.Output, "--- ") {
				out[k] = append(out[k], e.Output)
			}
		case "pass", "fail", "skip":
			if strings.Contains(e.Test, "/") {
				continue
			}
			t := model.Test{Feature: e.Package, Name: e.Test, Start: started[k], Duration: e.Elapsed * 1000, Attempts: 1,
				Status: map[string]string{"pass": "passed", "fail": "failed", "skip": "skipped"}[e.Action]}
			if e.Action == "fail" {
				t.Error = strings.TrimSpace(strings.Join(out[k], ""))
			}
			run.Tests = append(run.Tests, t)
		}
	}
	if err := sc.Err(); err != nil {
		return run, err
	}
	run.Start = first
	if !first.IsZero() {
		run.Duration = float64(last.Sub(first).Microseconds()) / 1000
	}
	return run, nil
}
