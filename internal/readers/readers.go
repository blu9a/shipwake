// Package readers turns test and measurement output from common tools into Shipwake runs.
// Each reader only fills tests or metrics; the caller sets the run's name, commit and branch.
package readers

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/blu9a/shipwake/internal/mapping"
	"github.com/blu9a/shipwake/internal/model"
)

// Reader parses one tool's output.
type Reader struct {
	Format string
	Kind   string // the run kind it suggests; projects name their own with --kind
	About  string
	Read   func(io.Reader) (model.Run, error)
	// A converter described in a mapping (built in, or the project's own in .shipwake/formats).
	Spec   *mapping.Spec
	Source string // where the mapping lives: "built-in" or its path in the project
}

// Built-in converters are mappings too: the same format projects use for their own.
//
//go:embed formats
var builtin embed.FS

func init() {
	files, _ := fs.Glob(builtin, "formats/*.yaml")
	for _, f := range files {
		data, _ := builtin.ReadFile(f)
		spec, err := mapping.Parse(data)
		if err != nil {
			panic(fmt.Sprintf("built-in format %s: %v", f, err))
		}
		all = append(all, fromSpec(spec, "built-in"))
	}
}

func fromSpec(spec *mapping.Spec, source string) Reader {
	kind := spec.Kind
	if kind == "" {
		kind = "test"
	}
	return Reader{Format: spec.Format, Kind: kind, About: spec.About, Spec: spec, Source: source, Read: func(r io.Reader) (model.Run, error) {
		run, err := spec.Convert(r)
		finish(&run)
		return run, err
	}}
}

// ProjectDir is where a project keeps its own converters.
const ProjectDir = ".shipwake/formats"

// Project reads a repository's own converters.
func Project(repo string) ([]Reader, error) {
	files, _ := filepath.Glob(filepath.Join(repo, ProjectDir, "*.yaml"))
	var res []Reader
	for _, f := range files {
		spec, err := mapping.Load(f)
		if err != nil {
			return res, fmt.Errorf("%s: %w", f, err)
		}
		rel, _ := filepath.Rel(repo, f)
		res = append(res, fromSpec(spec, rel))
	}
	return res, nil
}

// Find looks for a format among the project's converters first, then the built-in ones.
func Find(format, repo string) (Reader, error) {
	own, err := Project(repo)
	if err != nil {
		return Reader{}, err
	}
	for _, r := range own {
		if r.Format == format {
			return r, nil
		}
	}
	return Get(format)
}

// CheckSpec checks a converter against its sample: built-in ones from the binary, a project's from
// its directory. With update, it writes the expected run (only for a project's converters).
func CheckSpec(r Reader, repo string, update bool) error {
	if r.Source == "built-in" {
		read := func(name string) ([]byte, error) { return builtin.ReadFile(path.Join("formats", name)) }
		return r.Spec.Check(read, nil, false)
	}
	dir := filepath.Join(repo, filepath.Dir(r.Source))
	read := func(name string) ([]byte, error) { return os.ReadFile(filepath.Join(dir, name)) }
	write := func(name string, data []byte) error { return os.WriteFile(filepath.Join(dir, name), data, 0o644) }
	return r.Spec.Check(read, write, update)
}

var all = []Reader{
	{Format: "junit", Kind: "test", About: "JUnit XML, written by most test runners in every language", Read: readJUnit},
	{Format: "go-test", Kind: "test", About: "go test -json", Read: readGoTest},
	{Format: "playwright", Kind: "test", About: "Playwright's JSON reporter", Read: readPlaywright},
	{Format: "go-bench", Kind: "benchmark", About: "go test -bench output (benchmark format)", Read: readGoBench},
	{Format: "k6", Kind: "load test", About: "k6 run --summary-export=summary.json", Read: readK6},
	{Format: "json", Kind: "", About: "Shipwake's own run format (shipwake schema): evals, scripts, anything else", Read: readOwn},
}

// All lists the readers.
func All() []Reader { return slices.Clone(all) }

// Get finds a reader by format name.
func Get(format string) (Reader, error) {
	for _, r := range all {
		if r.Format == format {
			return r, nil
		}
	}
	var names []string
	for _, r := range all {
		names = append(names, r.Format)
	}
	return Reader{}, fmt.Errorf("unknown format %q (known: %s)", format, strings.Join(names, ", "))
}

// finish fills a run's start and duration from its tests when the tool did not record them.
func finish(run *model.Run) {
	if run.Start.IsZero() {
		for _, t := range run.Tests {
			if !t.Start.IsZero() && (run.Start.IsZero() || t.Start.Before(run.Start)) {
				run.Start = t.Start
			}
		}
	}
	if run.Duration == 0 {
		var end time.Time
		for _, t := range run.Tests {
			if e := t.Start.Add(time.Duration(t.Duration * float64(time.Millisecond))); e.After(end) {
				end = e
			}
		}
		if !run.Start.IsZero() && end.After(run.Start) {
			run.Duration = float64(end.Sub(run.Start).Milliseconds())
		}
	}
}
