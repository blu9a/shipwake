package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/blu9a/shipwake/internal/build"
	"github.com/blu9a/shipwake/internal/judge"
	"github.com/blu9a/shipwake/internal/model"
)

// Check is one verdict about a commit.
type Check struct {
	Status string `json:"status"` // ok, warn or fail
	Kind   string `json:"kind"`   // tests, measurement or look
	Text   string `json:"text"`
	Run    string `json:"run"`                // the run's ID: runs/<id>.json on the results branch
	Metric string `json:"metric,omitempty"`   // the measurement's ID
	Base   string `json:"baseline,omitempty"` // the run compared with
	Look   string `json:"look,omitempty"`     // the screenshot's name, such as "Button [dark]"
}

func checkCmd() *cobra.Command {
	var commit, against, results string
	var asJSON bool
	c := &cobra.Command{
		Use:   "check [repo]",
		Short: "Judge a commit: failing tests, measurements over their thresholds, looks nobody approved (exit 1 when something blocks)",
		Long: "Check looks at the runs recorded for a commit. A test run with failures blocks; a measurement over\n" +
			"its blocking threshold blocks; over its warning threshold it warns; a screenshot that looks different\n" +
			"from the look approved for it, or is new, blocks until someone approves it (shipwake looks). Use it\n" +
			"as a CI gate, before merging or releasing.",
		Example: "  shipwake check                    # HEAD: against the last release, else the previous run\n" +
			"  shipwake check --against previous # what this change did\n" +
			"  shipwake check --json",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ps := projectsFrom(args, results)
			l, err := build.Load(ctx, ps[0], -1, logf(cmd))
			if err != nil {
				return err
			}
			defer l.Close()
			sha, err := l.Repo.Resolve(ctx, commit)
			if err != nil {
				return err
			}
			if against == "" {
				against = "auto"
			}
			if against != "auto" && against != "release" && against != "previous" {
				return fmt.Errorf("--against %q: want release or previous", against)
			}
			checks, err := judgeCommit(ctx, l, sha, against)
			if err != nil {
				return err
			}
			worst := "ok"
			for _, ch := range checks {
				if ch.Status == "fail" || (ch.Status == "warn" && worst == "ok") {
					worst = ch.Status
				}
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				_ = enc.Encode(map[string]any{"commit": sha, "against": against, "result": worst, "checks": checks})
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%.7s, compared with %s\n", sha, map[string]string{
					"auto": "the last release, else the previous run", "release": "the last release", "previous": "the previous run"}[against])
				if len(checks) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "  no runs recorded for this commit")
				}
				for _, ch := range checks {
					mark := map[string]string{"ok": "✓", "warn": "!", "fail": "✗"}[ch.Status]
					fmt.Fprintf(cmd.OutOrStdout(), "  %s %s\n", mark, ch.Text)
				}
			}
			if worst == "fail" {
				return exitError(1)
			}
			return nil
		},
	}
	c.Flags().StringVar(&commit, "commit", "HEAD", "the commit to judge")
	c.Flags().StringVar(&against, "against", "", "baseline: release or previous (default: the last release, else the previous run, else levels only)")
	c.Flags().StringVarP(&results, "results", "r", "", resultsHelp)
	c.Flags().BoolVar(&asJSON, "json", false, "print the verdict as JSON")
	return c
}

// judgeCommit returns the checks for one commit: the latest run of each test suite and each
// measurement series recorded on it (build.Decisive), and how their screenshots compare with the looks
// approved for them.
func judgeCommit(ctx context.Context, l *build.Loaded, sha, against string) ([]Check, error) {
	var checks []Check
	for _, r := range l.Decisive(sha) {
		if len(r.Tests) > 0 {
			var failed, retried int
			for _, t := range r.Tests {
				if t.Status == "failed" {
					failed++
				}
				if t.Retried {
					retried++
				}
			}
			ch := Check{Status: "ok", Kind: "tests", Run: r.ID, Text: fmt.Sprintf("%s: %s passed", r.Name, plural(len(r.Tests), "test"))}
			if failed > 0 {
				ch.Status, ch.Text = "fail", fmt.Sprintf("%s: %d of %d tests failed", r.Name, failed, len(r.Tests))
			} else if retried > 0 {
				ch.Status, ch.Text = "warn", fmt.Sprintf("%s: all passed, %d only on retry", r.Name, retried)
			}
			checks = append(checks, ch)
		}
		var ok int
		release := against == "release" || (against == "auto" && hasRelease(r))
		for _, m := range r.Metrics {
			c := m.Previous
			if release {
				c = m.Release
			}
			if c == nil || c.Status == "none" {
				continue
			}
			if c.Status == "ok" {
				ok++
				continue
			}
			text := fmt.Sprintf("%s · %s: %s", r.Name, metricName(m), judge.Format(judge.Mean(m.Values), m.Unit))
			if c.Delta != nil {
				text += fmt.Sprintf(" (%+.1f%% vs %s)", *c.Delta, c.Label)
			}
			if c.Partial {
				text += " (judged in part: some conditions need a baseline)"
			}
			st := "warn"
			if c.Status == "block" {
				st = "fail"
			}
			checks = append(checks, Check{Status: st, Kind: "measurement", Text: text, Run: r.ID, Metric: m.ID, Base: c.Run})
		}
		if ok > 0 {
			checks = append(checks, Check{Status: "ok", Kind: "measurement", Run: r.ID, Text: fmt.Sprintf("%s: %s within thresholds", r.Name, plural(ok, "measurement"))})
		}
	}
	ls, _, err := l.ReviewLooks(ctx, sha)
	if err != nil {
		return nil, err
	}
	same := 0
	for _, lk := range ls.Looks {
		switch lk.Status {
		case "same":
			same++
		case "changed":
			checks = append(checks, Check{Status: "fail", Kind: "look", Run: lk.Run, Look: lk.Name,
				Text: fmt.Sprintf("Looks: %q looks different from its approved look (%s), and nobody approved the new one", lk.Name, plural(lk.Pixels, "pixel"))})
		default:
			checks = append(checks, Check{Status: "fail", Kind: "look", Run: lk.Run, Look: lk.Name,
				Text: fmt.Sprintf("Looks: %q is new and waits for a first approval", lk.Name)})
		}
	}
	if same > 0 {
		checks = append(checks, Check{Status: "ok", Kind: "look", Text: fmt.Sprintf("Looks: %s as approved", plural(same, "screenshot"))})
	}
	return checks, nil
}

// hasRelease reports whether a run is compared with a run at the last release.
func hasRelease(r *model.Run) bool {
	for _, m := range r.Metrics {
		if m.Release != nil && m.Release.Run != "" {
			return true
		}
	}
	return false
}

func metricName(m model.Metric) string {
	if m.Suite != "" {
		return m.Name + " (" + m.Suite + ")"
	}
	return m.Name
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}
