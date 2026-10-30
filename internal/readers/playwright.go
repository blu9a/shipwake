package readers

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"strings"
	"time"

	"github.com/blu9a/shipwake/internal/model"
)

type pwReport struct {
	Stats struct {
		StartTime time.Time `json:"startTime"`
		Duration  float64   `json:"duration"`
	} `json:"stats"`
	Suites []pwSuite `json:"suites"`
}

type pwSuite struct {
	Title  string    `json:"title"`
	Specs  []pwSpec  `json:"specs"`
	Suites []pwSuite `json:"suites"`
}

type pwSpec struct {
	Title string   `json:"title"`
	Tests []pwTest `json:"tests"`
}

type pwTest struct {
	ProjectName string     `json:"projectName"`
	Results     []pwResult `json:"results"`
}

type pwResult struct {
	Status      string         `json:"status"`
	StartTime   time.Time      `json:"startTime"`
	Duration    float64        `json:"duration"`
	Error       *pwError       `json:"error"`
	Steps       []pwStep       `json:"steps"`
	Attachments []pwAttachment `json:"attachments"`
}

type pwError struct {
	Message string `json:"message"`
}

type pwStep struct {
	Title    string   `json:"title"`
	Duration float64  `json:"duration"`
	Error    *pwError `json:"error"`
	Steps    []pwStep `json:"steps"`
}

type pwAttachment struct {
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	Path        string `json:"path"`
	Body        string `json:"body"` // base64, for attachments kept in the report
}

// readPlaywright reads Playwright's JSON reporter. The top-level describe block (or the file) is the group,
// the Playwright project is the target, and retries count as attempts.
func readPlaywright(r io.Reader) (model.Run, error) {
	var rep pwReport
	if err := json.NewDecoder(r).Decode(&rep); err != nil {
		return model.Run{}, err
	}
	run := model.Run{Start: rep.Stats.StartTime, Duration: rep.Stats.Duration}
	var walk func(s pwSuite, group string)
	walk = func(s pwSuite, group string) {
		for _, sp := range s.Specs {
			for _, t := range sp.Tests {
				if len(t.Results) == 0 {
					continue
				}
				last := t.Results[len(t.Results)-1]
				test := model.Test{Feature: group, Name: sp.Title, Target: t.ProjectName, Start: last.StartTime, Duration: last.Duration,
					Attempts: len(t.Results), Status: pwStatus(last.Status), Steps: pwSteps(last.Steps)}
				if last.Error != nil {
					test.Error = last.Error.Message
				}
				test.Retried = test.Status == "passed" && len(t.Results) > 1
				for _, a := range last.Attachments {
					if ev, ok := evidence(a); ok { // what a result step checked, recorded by the decisive helper
						test.Evidence = append(test.Evidence, ev)
						continue
					}
					if a.Path != "" {
						test.Attachments = append(test.Attachments, model.Attachment{Name: a.Name, Type: a.ContentType, Path: a.Path})
					}
				}
				run.Tests = append(run.Tests, test)
			}
		}
		for _, sub := range s.Suites {
			g := group
			if g == "" || len(sub.Specs) > 0 {
				g = sub.Title
			}
			walk(sub, g)
		}
	}
	for _, s := range rep.Suites {
		walk(s, "")
	}
	finish(&run)
	return run, nil
}

// evidence reads an "observed: <step>" JSON attachment: {step, expected, observed}.
func evidence(a pwAttachment) (model.Evidence, bool) {
	var ev model.Evidence
	if !strings.HasPrefix(a.Name, "observed: ") || a.ContentType != "application/json" {
		return ev, false
	}
	var data []byte
	if a.Body != "" {
		data, _ = base64.StdEncoding.DecodeString(a.Body)
	} else if a.Path != "" {
		data, _ = os.ReadFile(a.Path)
	}
	if json.Unmarshal(data, &ev) != nil || ev.Observed == "" {
		return ev, false
	}
	return ev, true
}

func pwStatus(s string) string {
	switch s {
	case "passed":
		return "passed"
	case "skipped":
		return "skipped"
	default: // failed, timedOut, interrupted
		return "failed"
	}
}

func pwSteps(in []pwStep) []model.Step {
	var out []model.Step
	for _, s := range in {
		st := model.Step{Name: s.Title, Status: "passed", Duration: s.Duration, Steps: pwSteps(s.Steps)}
		if s.Error != nil {
			st.Status = "failed"
		}
		out = append(out, st)
	}
	return out
}
