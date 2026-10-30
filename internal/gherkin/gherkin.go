// Package gherkin reads the parts of .feature files Shipwake needs:
// the feature name and each scenario's steps.
package gherkin

import (
	"regexp"
	"strings"
)

var (
	featureRe  = regexp.MustCompile(`^\s*Feature:\s*(.+)$`)
	scenarioRe = regexp.MustCompile(`^\s*(?:Scenario|Scenario Outline|Scenario Template|Example):\s*(.+)$`)
	stepRe     = regexp.MustCompile(`^\s*(Given|When|Then|And|But|\*)\s+(.+)$`)
	blockRe    = regexp.MustCompile(`^\s*(?:Background|Rule):`)
	tagsRe     = regexp.MustCompile(`^\s*(@\S+(?:\s+@\S+)*)\s*$`)
)

// Feature is one parsed .feature file.
type Feature struct {
	Name      string
	Scenarios map[string][]string // scenario name → steps, keyword included
	Order     []string            // scenario names as written
	Tags      map[string][]string // scenario name → its tags, such as @proposed
}

// Parse reads a .feature file. It is lenient: lines it does not understand are skipped.
func Parse(text string) Feature {
	f := Feature{Scenarios: map[string][]string{}, Tags: map[string][]string{}}
	cur := ""
	var pending []string // tags waiting for the scenario under them
	for _, line := range strings.Split(text, "\n") {
		if m := tagsRe.FindStringSubmatch(line); m != nil {
			pending = append(pending, strings.Fields(m[1])...)
			continue
		}
		if m := featureRe.FindStringSubmatch(line); m != nil {
			f.Name = strings.TrimSpace(m[1])
			pending = nil // feature tags apply to all; not needed yet
			continue
		}
		if m := scenarioRe.FindStringSubmatch(line); m != nil {
			cur = strings.TrimSpace(m[1])
			if _, seen := f.Scenarios[cur]; !seen {
				f.Order = append(f.Order, cur)
			}
			f.Scenarios[cur] = []string{}
			f.Tags[cur], pending = pending, nil
			continue
		}
		if blockRe.MatchString(line) {
			cur = ""
			continue
		}
		if m := stepRe.FindStringSubmatch(line); m != nil && cur != "" {
			f.Scenarios[cur] = append(f.Scenarios[cur], m[1]+" "+strings.TrimSpace(m[2]))
		}
	}
	return f
}
