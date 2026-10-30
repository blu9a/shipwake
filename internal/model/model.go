// Package model is Shipwake's data model: what the site, the CLI and agents read.
// It is the single source of truth; the JSON Schema in spec/ and the TypeScript
// types in web/ are generated from it.
package model

import (
	"fmt"
	"time"
)

// SchemaVersion changes when a file written by Shipwake changes incompatibly.
const SchemaVersion = 1

// Site lists the projects a built site contains.
type Site struct {
	Version     int          `json:"version"`
	GeneratedAt time.Time    `json:"generatedAt"`
	Projects    []ProjectRef `json:"projects"`
}

// ProjectRef points to one project's index.
type ProjectRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Project is a project's index: its trunk history, releases, open branches and runs.
// Details (files, diffs, test results) live in separate files and load on demand.
type Project struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Trunk     string            `json:"trunk"`
	Languages []string          `json:"languages"`
	Targets   map[string]Names  `json:"targets,omitempty"`
	Text      map[string]Names  `json:"text,omitempty"` // project text in other languages, keyed by the original
	Rows      []Row             `json:"rows"`
	Tags      []Tag             `json:"tags"`
	Branches  []Branch          `json:"branches"`
	Runs      []RunSummary      `json:"runs"`
	Commits   map[string]Commit `json:"commits"`             // every commit the index mentions, by full hash
	Rules     map[string]Rule   `json:"rules,omitempty"`     // direction and thresholds by metric ID or pattern ("api.*.p95")
	Kinds     map[string]Kind   `json:"kinds,omitempty"`     // how the project's own run kinds are shown
	Truncated bool              `json:"truncated,omitempty"` // older history was left out
}

// Rule is what a project says about a measurement: which way is better, overriding what the run
// recorded, and its warning and blocking thresholds. A threshold is either absolute ("0.88") or
// relative to the baseline ("+15%", "-10%"); the direction says which side is worse.
type Rule struct {
	Direction string `json:"direction,omitempty"` // higher or lower
	Warn      string `json:"warn,omitempty"`
	Block     string `json:"block,omitempty"`
}

// Kind is how runs of one kind are shown: an icon from the site's set and the kind's name in
// other languages.
type Kind struct {
	Icon  string `json:"icon,omitempty"`
	Names Names  `json:"names,omitempty"`
}

// Comparison is a measurement compared with a baseline run.
type Comparison struct {
	Run    string   `json:"run"`             // the baseline run's ID
	Label  string   `json:"label"`           // the baseline's commit or release name
	Value  *float64 `json:"value,omitempty"` // the baseline's mean; absent when only absolute thresholds apply
	Delta  *float64 `json:"delta,omitempty"` // percentage change from the baseline
	Status string   `json:"status"`          // ok, warn, block, or none without thresholds
	// Partial: some threshold conditions couldn't be judged, such as a change with no baseline.
	Partial bool `json:"partial,omitempty"`
}

// Names holds a text in several languages, keyed by language code.
type Names map[string]string

// Commit is one commit's header.
type Commit struct {
	SHA       string    `json:"sha"`
	Short     string    `json:"short"`
	Parents   []string  `json:"parents"`
	Author    string    `json:"author"`
	Date      time.Time `json:"date"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body,omitempty"`
	Type      string    `json:"type,omitempty"` // Conventional Commit type, such as feat or fix
	Scenarios int       `json:"scenarios,omitempty"`
}

// Row is one entry of the trunk's first-parent history: a merge or a direct commit.
type Row struct {
	SHA     string   `json:"sha"`
	Merge   bool     `json:"merge"`
	Branch  string   `json:"branch,omitempty"` // the merged branch, from the merge message
	Title   string   `json:"title"`
	Kind    string   `json:"kind,omitempty"`    // feat, fix, docs…: from the branch prefix or commit type
	Commits []string `json:"commits,omitempty"` // a merge's own commits, newest first
}

// Tag is a release: an annotated tag, its notes and the commit it marks.
type Tag struct {
	Name    string    `json:"name"`
	SHA     string    `json:"sha"`
	Date    time.Time `json:"date"`
	Subject string    `json:"subject"`
	Body    string    `json:"body,omitempty"`
	// The release's section in the changelog file, per language, when the file has one: the record,
	// shown instead of the tag's message.
	Notes map[string]string `json:"notes,omitempty"`
}

// Branch is a branch not yet merged into the trunk.
type Branch struct {
	Name    string    `json:"name"`
	Updated time.Time `json:"updated"`
	Ahead   []string  `json:"ahead"` // commits not on the trunk, newest first
	Behind  int       `json:"behind"`
	Base    string    `json:"base"` // where it left the trunk
}

// CommitDetail is everything a commit page shows beyond the header.
type CommitDetail struct {
	SHA       string           `json:"sha"`
	Files     []FileChange     `json:"files"`
	Scenarios []ScenarioChange `json:"scenarios"`
	Tags      []string         `json:"tags"` // releases that contain the commit
}

// FileChange is one changed file, compared with the first parent.
type FileChange struct {
	Path    string `json:"path"`
	Old     string `json:"old,omitempty"` // previous path when renamed
	Status  string `json:"status"`        // A, M, D or R
	Added   *int   `json:"added"`         // nil for binary files
	Deleted *int   `json:"deleted"`
}

// FileDiff is one file's patch, possibly shortened.
type FileDiff struct {
	Lines []string `json:"lines,omitempty"`
	// HL colors each line, when the language is known: token classes and lengths ("k3_1f4": three
	// characters of keyword, one plain, four of function name), lengths in UTF-16 code units.
	HL      []string `json:"hl,omitempty"`
	More    int      `json:"more,omitempty"`    // lines left out
	Binary  bool     `json:"binary,omitempty"`  // no text diff
	Skipped int      `json:"skipped,omitempty"` // too large to include: its length
}

// ScenarioChange is a scenario added, changed or removed in a commit's .feature files.
type ScenarioChange struct {
	Feature string   `json:"feature"`
	Title   string   `json:"title"`
	Change  string   `json:"change"` // added, changed or removed
	Steps   []string `json:"steps"`
	Was     []string `json:"was,omitempty"` // steps before, for changed scenarios
}

// RunSchema is the version of the run format this Shipwake writes. It changes only when a change
// isn't additive; new optional fields keep it. Every Shipwake reads runs of its own version and all
// earlier ones, so results recorded years ago keep working (docs/compatibility.md).
const RunSchema = 1

// Run is one execution of a test suite or a measurement suite against one commit.
type Run struct {
	Schema   int               `json:"schemaVersion,omitempty"` // RunSchema when written; absent means 1
	ID       string            `json:"id"`
	Kind     string            `json:"kind"` // the project's own word: test, eval, load test, benchmark…
	Name     string            `json:"name"`
	Commit   string            `json:"commit"`
	Branch   string            `json:"branch,omitempty"`
	Start    time.Time         `json:"start"`
	Duration float64           `json:"duration"` // milliseconds
	Format   string            `json:"format"`   // the reader that produced it
	Context  map[string]string `json:"context,omitempty"`
	Tests    []Test            `json:"tests,omitempty"`
	Metrics  []Metric          `json:"metrics,omitempty"`
	Cases    []Case            `json:"cases,omitempty"` // an evaluation's dataset items, with what each scored
}

// CheckSchema says whether this Shipwake can read the run: one written by a newer Shipwake, in a format
// it doesn't know, would be misread.
func (r Run) CheckSchema() error {
	if r.Schema > RunSchema {
		return fmt.Errorf("written in run format %d by a newer Shipwake; this one reads up to format %d: upgrade shipwake", r.Schema, RunSchema)
	}
	return nil
}

// Case is one item of an evaluation's dataset: what went in, what came out, its score and why. The
// same ID in another run is the same item, so the page can show it before and now.
type Case struct {
	ID     string  `json:"id"`
	Metric string  `json:"metric,omitempty"` // the metric it counts toward
	Title  string  `json:"title,omitempty"`  // a short name; the input when absent
	Input  string  `json:"input"`
	Output string  `json:"output,omitempty"`
	Score  float64 `json:"score"`            // from 0 to 1; 1 is fully right
	Reason string  `json:"reason,omitempty"` // the grader's explanation
}

// RunSummary is what the timeline needs from a run.
type RunSummary struct {
	ID       string            `json:"id"`
	Kind     string            `json:"kind"`
	Name     string            `json:"name"`
	Commit   string            `json:"commit"`
	Branch   string            `json:"branch,omitempty"`
	Start    time.Time         `json:"start"`
	Duration float64           `json:"duration"`
	Passed   int               `json:"passed"`
	Failed   int               `json:"failed"`
	Skipped  int               `json:"skipped"`
	Targets  []string          `json:"targets,omitempty"`
	Context  map[string]string `json:"context,omitempty"`
	Metrics  int               `json:"metrics,omitempty"`
	// Measurement statuses against the previous comparable run and against the last release.
	Previous   Tally             `json:"previous,omitzero"`
	Release    Tally             `json:"release,omitzero"`
	Changed    []string          `json:"changed,omitempty"`    // context keys that differ from the previous run of the series
	Was        map[string]string `json:"was,omitempty"`        // their values in that run ("" when it had none)
	Comparable bool              `json:"comparable,omitempty"` // a previous run on the same runner exists
	HasRelease bool              `json:"hasRelease,omitempty"` // a run at the last release on the same runner exists
	Order      float64           `json:"order"`                // place in history: runs sort by order, then start
}

// Tally counts measurement statuses.
type Tally struct {
	OK    int `json:"ok,omitempty"`
	Warn  int `json:"warn,omitempty"`
	Block int `json:"block,omitempty"`
}

// Test is one test result.
type Test struct {
	Feature     string       `json:"feature,omitempty"` // group: Gherkin feature, suite, package…
	Name        string       `json:"name"`
	Target      string       `json:"target,omitempty"` // where it ran: browser project, device…
	Status      string       `json:"status"`           // passed, failed or skipped
	Start       time.Time    `json:"start,omitzero"`
	Duration    float64      `json:"duration"`
	Attempts    int          `json:"attempts"`
	Error       string       `json:"error,omitempty"`
	Retried     bool         `json:"retried,omitempty"` // passed only after a failed attempt
	Steps       []Step       `json:"steps,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
	Evidence    []Evidence   `json:"evidence,omitempty"`
}

// Step is a test step, such as a Gherkin step.
type Step struct {
	Name        string       `json:"name"`
	Status      string       `json:"status"`
	Start       time.Time    `json:"start,omitzero"` // when it began; lets subtitles follow a recording
	Duration    float64      `json:"duration"`
	Attachments []Attachment `json:"attachments,omitempty"`
	Steps       []Step       `json:"steps,omitempty"`
}

// Attachment is a file kept with a result: a screenshot, a video, a log.
type Attachment struct {
	Name  string    `json:"name"`
	Type  string    `json:"type"` // media type
	Path  string    `json:"path"` // relative to the run's files
	Size  int64     `json:"size"`
	Start time.Time `json:"start,omitzero"` // for a recording: when it began
}

// Evidence is what a result step checked: the expected text and what was actually seen.
type Evidence struct {
	Step     string   `json:"step"`
	Expected []string `json:"expected"`
	Observed string   `json:"observed"`
}

// Metric is one measurement with its repeated values.
type Metric struct {
	ID        string    `json:"id"`
	Suite     string    `json:"suite,omitempty"`
	Name      string    `json:"name"`
	Unit      string    `json:"unit"`
	Direction string    `json:"direction"` // higher or lower is better
	Values    []float64 `json:"values"`
	// Filled in by the build: comparisons with the previous comparable run and the last release.
	Previous *Comparison `json:"previous,omitempty"`
	Release  *Comparison `json:"release,omitempty"`
}
