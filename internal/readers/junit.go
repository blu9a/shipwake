package readers

import (
	"encoding/xml"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/blu9a/shipwake/internal/model"
)

type junitSuites struct {
	Suites []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	Name      string       `xml:"name,attr"`
	Timestamp string       `xml:"timestamp,attr"`
	Time      string       `xml:"time,attr"`
	Cases     []junitCase  `xml:"testcase"`
	Suites    []junitSuite `xml:"testsuite"`
}

type junitCase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *junitProblem `xml:"failure"`
	Error     *junitProblem `xml:"error"`
	Skipped   *junitProblem `xml:"skipped"`
}

type junitProblem struct {
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}

// readJUnit reads JUnit XML: a <testsuites> root or a single <testsuite>.
func readJUnit(r io.Reader) (model.Run, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return model.Run{}, err
	}
	var root junitSuites
	if err := xml.Unmarshal(data, &root); err != nil || len(root.Suites) == 0 {
		var one junitSuite
		if err := xml.Unmarshal(data, &one); err != nil {
			return model.Run{}, err
		}
		root.Suites = []junitSuite{one}
	}
	run := model.Run{}
	var walk func(s junitSuite)
	walk = func(s junitSuite) {
		start, _ := time.Parse("2006-01-02T15:04:05", strings.TrimSuffix(s.Timestamp, "Z"))
		for _, c := range s.Cases {
			t := model.Test{Feature: firstNonEmpty(c.Classname, s.Name), Name: c.Name, Status: "passed", Duration: seconds(c.Time), Attempts: 1, Start: start}
			switch {
			case c.Failure != nil:
				t.Status, t.Error = "failed", problem(c.Failure)
			case c.Error != nil:
				t.Status, t.Error = "failed", problem(c.Error)
			case c.Skipped != nil:
				t.Status = "skipped"
			}
			run.Tests = append(run.Tests, t)
		}
		for _, sub := range s.Suites {
			walk(sub)
		}
	}
	for _, s := range root.Suites {
		walk(s)
	}
	finish(&run)
	return run, nil
}

func problem(p *junitProblem) string {
	text := strings.TrimSpace(p.Text)
	if p.Message != "" && !strings.Contains(text, p.Message) {
		return strings.TrimSpace(p.Message + "\n" + text)
	}
	return text
}

func seconds(s string) float64 {
	v, _ := strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64)
	return v * 1000
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if x != "" {
			return x
		}
	}
	return ""
}
