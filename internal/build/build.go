// Package build writes a static site: the web app plus each project's data, split into
// small files the page loads on demand.
//
//	data/site.json
//	data/<project>/index.json            trunk rows, releases, branches, run summaries, commit headers
//	data/<project>/commits/<sha>.json    files and scenario changes
//	data/<project>/diffs/<sha>.json      patches, per file
//	data/<project>/runs/<id>.json        test results and measurements
//	data/<project>/files/<id>/…          attachments
package build

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/blu9a/shipwake/internal/changelog"
	"github.com/blu9a/shipwake/internal/gitx"
	"github.com/blu9a/shipwake/internal/history"
	"github.com/blu9a/shipwake/internal/judge"
	"github.com/blu9a/shipwake/internal/metricspec"
	"github.com/blu9a/shipwake/internal/model"
	"github.com/blu9a/shipwake/internal/store"
	"github.com/blu9a/shipwake/internal/story"
)

// Project is one repository to include.
type Project struct {
	Dir     string // the repository
	Results string // a results store; empty for none
}

// Options control a build.
type Options struct {
	Projects []Project
	Out      string
	MaxRows  int   // trunk history to include; 0 means DefaultRows, negative means all
	Web      fs.FS // the web app's files
	Log      func(format string, args ...any)
}

// Run builds the site.
func Run(ctx context.Context, o Options) error {
	if o.Log == nil {
		o.Log = func(string, ...any) {}
	}
	switch {
	case o.MaxRows == 0:
		o.MaxRows = DefaultRows
	case o.MaxRows < 0:
		o.MaxRows = 0
	}
	if err := os.MkdirAll(o.Out, 0o755); err != nil {
		return err
	}
	if o.Web != nil {
		if err := copyFS(o.Web, o.Out); err != nil {
			return fmt.Errorf("web app: %w", err)
		}
	}
	site := model.Site{Version: model.SchemaVersion, GeneratedAt: time.Now().UTC()}
	used := map[string]bool{}
	for _, p := range o.Projects {
		ref, err := project(ctx, p, o, used)
		if err != nil {
			return fmt.Errorf("%s: %w", p.Dir, err)
		}
		site.Projects = append(site.Projects, ref)
	}
	return writeJSON(filepath.Join(o.Out, "data", "site.json"), site)
}

var idRe = regexp.MustCompile(`[^a-z0-9-]+`)

// Loaded is a project read into memory: its index, its runs (judged) and where their files are.
type Loaded struct {
	Index *model.Project
	Runs  []*model.Run
	Files map[string]string // run ID → directory with its attachments
	Repo  gitx.Repo
	clean []func()
}

// Close removes temporary copies of the results.
func (l *Loaded) Close() {
	for _, f := range l.clean {
		f()
	}
}

// DefaultRows is how much trunk history a build includes unless told otherwise.
const DefaultRows = 300

// Load reads a project's history, config and runs, and judges the measurements.
func Load(ctx context.Context, p Project, maxRows int, log func(string, ...any)) (*Loaded, error) {
	repo := gitx.Repo{Dir: p.Dir}
	top, err := repo.Git(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	top = strings.TrimSpace(top)
	repo = gitx.Repo{Dir: top}
	cfg, err := LoadConfig(top)
	if err != nil {
		return nil, err
	}
	if cfg.Name == "" {
		cfg.Name = filepath.Base(top)
	}
	if cfg.Trunk == "" {
		cfg.Trunk = defaultTrunk(ctx, repo)
	}
	if len(cfg.Languages) == 0 {
		cfg.Languages = []string{"en"}
	}
	h, err := history.Load(ctx, repo, cfg.Trunk, maxRows)
	if err != nil {
		return nil, err
	}
	idx := &model.Project{Name: cfg.Name, Trunk: cfg.Trunk, Languages: cfg.Languages, Targets: cfg.Targets,
		Rows: h.Rows, Tags: nonNil(h.Tags), Branches: nonNil(h.Branches), Commits: h.Commits, Runs: []model.RunSummary{},
		Rules: rules(cfg), Kinds: kinds(cfg), Truncated: h.Truncated}
	// The changelog file is the releases' record; the trunk's newest version of it holds them all.
	for lang, file := range ChangelogFiles(cfg) {
		text, err := repo.Git(ctx, "show", h.Trunk+":"+file) // the trunk as resolved: local, or its remote
		if err != nil {
			continue // not written yet: the tags' messages stand
		}
		for i := range idx.Tags {
			if body, ok := changelog.Find(text, idx.Tags[i].Name); ok {
				if idx.Tags[i].Notes == nil {
					idx.Tags[i].Notes = map[string]string{}
				}
				idx.Tags[i].Notes[lang] = body
			}
		}
	}
	// Approved .metric files are thresholds too, and win over the config's for their ID.
	for id, r := range agreedMetrics(ctx, repo) {
		if idx.Rules == nil {
			idx.Rules = map[string]model.Rule{}
		}
		idx.Rules[id] = r
	}
	if idx.Text, err = loadText(top, cfg.Text); err != nil {
		return nil, err
	}
	l := &Loaded{Index: idx, Files: map[string]string{}, Repo: repo}

	// Where the runs are: an explicit location, the config's, or the default results branch.
	spec := p.Results
	if spec == "" {
		spec = cfg.Results
	}
	if spec == "" {
		spec = store.DefaultBranch
	}
	dir := spec
	if branch := store.Branch(spec); branch != "" {
		tmp, err := os.MkdirTemp("", "shipwake-read-*")
		if err != nil {
			return nil, err
		}
		l.clean = append(l.clean, func() { _ = os.RemoveAll(tmp) })
		found, err := store.Export(ctx, repo, branch, tmp)
		if err != nil {
			l.Close()
			return nil, fmt.Errorf("results branch %s: %w", branch, err)
		}
		dir = tmp
		if !found {
			dir = ""
			if p.Results != "" {
				log("%s: no results branch %q yet", cfg.Name, branch)
			}
		}
	}
	if dir != "" {
		st := store.Store{Dir: dir}
		runs, err := st.List()
		if err != nil {
			l.Close()
			return nil, err
		}
		for i := range runs {
			r := &runs[i]
			full, err := repo.Resolve(ctx, r.Commit)
			if err != nil {
				log("%s: skipping run %s: commit %.12s is not in this repository", cfg.Name, r.ID, r.Commit)
				continue
			}
			r.Commit = full
			if _, ok := idx.Commits[full]; !ok {
				if c, err := history.Commit(ctx, repo, full); err == nil {
					idx.Commits[full] = c
				}
			}
			idx.Runs = append(idx.Runs, summarize(*r))
			l.Runs = append(l.Runs, r)
			l.Files[r.ID] = st.FilesDir(r.ID)
		}
	}
	judge.Apply(idx, l.Runs)
	return l, nil
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// rules normalizes the config's metrics: numbers become strings, "+15%" stays as written.
func rules(cfg model.Config) map[string]model.Rule {
	if len(cfg.Metrics) == 0 {
		return nil
	}
	str := func(v any) string {
		switch x := v.(type) {
		case nil:
			return ""
		case string:
			return x
		case float64:
			return strconv.FormatFloat(x, 'f', -1, 64)
		case uint64:
			return strconv.FormatUint(x, 10)
		case int64:
			return strconv.FormatInt(x, 10)
		default:
			return fmt.Sprint(x)
		}
	}
	res := map[string]model.Rule{}
	for k, v := range cfg.Metrics {
		res[k] = model.Rule{Direction: strings.ToLower(strings.TrimSpace(v.Direction)), Warn: str(v.Warn), Block: str(v.Block)}
	}
	return res
}

// ChangelogFiles are the changelog's files by language: CHANGELOG.md for the first language and
// CHANGELOG.<lang>.md for the others unless the config says otherwise; none when it says "none".
func ChangelogFiles(cfg model.Config) map[string]string {
	res := map[string]string{}
	switch v := cfg.Changelog.(type) {
	case string:
		if strings.EqualFold(strings.TrimSpace(v), "none") {
			return nil
		}
		if len(cfg.Languages) > 0 {
			res[cfg.Languages[0]] = v
		}
		return res
	case map[string]any:
		for lang, file := range v {
			if f, ok := file.(string); ok {
				res[lang] = f
			}
		}
		return res
	}
	for i, lang := range cfg.Languages {
		if i == 0 {
			res[lang] = "CHANGELOG.md"
		} else {
			res[lang] = "CHANGELOG." + lang + ".md"
		}
	}
	return res
}

// agreedMetrics reads the committed, approved .metric files: a proposal decides nothing yet, and a
// file with an error is left out rather than guessed at.
func agreedMetrics(ctx context.Context, repo gitx.Repo) map[string]model.Rule {
	paths, err := repo.Lines(ctx, "ls-tree", "-r", "--name-only", "HEAD")
	if err != nil {
		return nil
	}
	res := map[string]model.Rule{}
	for _, p := range paths {
		if !strings.HasSuffix(p, ".metric") {
			continue
		}
		text, err := repo.Git(ctx, "show", "HEAD:"+p)
		if err != nil {
			continue
		}
		s := metricspec.Parse(p, text)
		if s.Err() != nil || s.Proposed() {
			continue
		}
		res[s.ID] = model.Rule{Direction: s.Direction, Warn: s.Warn, Block: s.Block}
	}
	return res
}

// kinds splits each configured kind into its icon and its names in other languages.
func kinds(cfg model.Config) map[string]model.Kind {
	if len(cfg.Kinds) == 0 {
		return nil
	}
	res := map[string]model.Kind{}
	for k, v := range cfg.Kinds {
		kind := model.Kind{Icon: v["icon"], Names: model.Names{}}
		for lang, name := range v {
			if lang != "icon" {
				kind.Names[lang] = name
			}
		}
		res[k] = kind
	}
	return res
}

func project(ctx context.Context, p Project, o Options, used map[string]bool) (model.ProjectRef, error) {
	l, err := Load(ctx, p, o.MaxRows, o.Log)
	if err != nil {
		return model.ProjectRef{}, err
	}
	defer l.Close()
	idx := l.Index
	id := strings.Trim(idRe.ReplaceAllString(strings.ToLower(idx.Name), "-"), "-")
	if id == "" {
		id = "project"
	}
	for base, n := id, 2; used[id]; n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	used[id] = true
	idx.ID = id
	dir := filepath.Join(o.Out, "data", id)
	// Each feature's requirements loop, for the feature pages.
	stories, err := story.Build(ctx, l.Repo, l.Runs)
	if err != nil {
		return model.ProjectRef{}, fmt.Errorf("stories: %w", err)
	}
	if err := writeJSON(filepath.Join(dir, "stories.json"), stories); err != nil {
		return model.ProjectRef{}, err
	}
	// Measured requirements: metrics proposed, approved, then met or missed by runs (judged above).
	metrics, err := story.BuildMetrics(ctx, l.Repo, l.Runs)
	if err != nil {
		return model.ProjectRef{}, fmt.Errorf("metric stories: %w", err)
	}
	if err := writeJSON(filepath.Join(dir, "metric-stories.json"), metrics); err != nil {
		return model.ProjectRef{}, err
	}
	for _, r := range l.Runs {
		if err := writeJSON(filepath.Join(dir, "runs", r.ID+".json"), r); err != nil {
			return model.ProjectRef{}, err
		}
		if src := l.Files[r.ID]; exists(src) {
			if err := copyFS(os.DirFS(src), filepath.Join(dir, "files", r.ID)); err != nil {
				return model.ProjectRef{}, err
			}
		}
	}

	// Commit pages: details and diffs for every commit the index mentions.
	shas := make([]string, 0, len(idx.Commits))
	for sha := range idx.Commits {
		shas = append(shas, sha)
	}
	slices.Sort(shas)
	o.Log("%s: %d commits, %d releases, %d branches, %d runs", id, len(shas), len(idx.Tags), len(idx.Branches), len(idx.Runs))
	// Each commit is independent: read and highlight up to eight at once.
	scenarios := make([]int, len(shas))
	errs := make([]error, len(shas))
	slots := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i, sha := range shas {
		slots <- struct{}{}
		wg.Go(func() {
			defer func() { <-slots }()
			c := idx.Commits[sha]
			detail, diffs, err := history.Detail(ctx, l.Repo, c)
			if err == nil {
				scenarios[i] = len(detail.Scenarios)
				err = writeJSON(filepath.Join(dir, "commits", sha+".json"), detail)
			}
			if err == nil {
				err = writeJSON(filepath.Join(dir, "diffs", sha+".json"), diffs)
			}
			if err != nil {
				errs[i] = fmt.Errorf("commit %s: %w", c.Short, err)
			}
		})
	}
	wg.Wait()
	for i, sha := range shas {
		if errs[i] != nil {
			return model.ProjectRef{}, errs[i]
		}
		c := idx.Commits[sha]
		c.Scenarios = scenarios[i]
		idx.Commits[sha] = c
	}
	if err := writeJSON(filepath.Join(dir, "index.json"), idx); err != nil {
		return model.ProjectRef{}, err
	}
	return model.ProjectRef{ID: id, Name: idx.Name}, nil
}

func summarize(r model.Run) model.RunSummary {
	s := model.RunSummary{ID: r.ID, Kind: r.Kind, Name: r.Name, Commit: r.Commit, Branch: r.Branch, Start: r.Start, Duration: r.Duration, Context: r.Context}
	targets := map[string]bool{}
	for _, t := range r.Tests {
		switch t.Status {
		case "passed":
			s.Passed++
		case "failed":
			s.Failed++
		default:
			s.Skipped++
		}
		if t.Target != "" && !targets[t.Target] {
			targets[t.Target] = true
			s.Targets = append(s.Targets, t.Target)
		}
	}
	slices.Sort(s.Targets)
	return s
}

func defaultTrunk(ctx context.Context, r gitx.Repo) string {
	if out, err := r.Git(ctx, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		if b := strings.TrimPrefix(strings.TrimSpace(out), "origin/"); b != "" {
			return b // history reads it locally, or from origin when only the remote has it
		}
	}
	for _, b := range []string{"main", "master", "trunk"} {
		for _, ref := range []string{b, "origin/" + b} {
			if _, err := r.Resolve(ctx, ref); err == nil {
				return b
			}
		}
	}
	return "HEAD"
}

// LoadConfig reads a repository's optional .shipwake.yaml.
func LoadConfig(top string) (model.Config, error) {
	var cfg model.Config
	for _, name := range []string{".shipwake.yaml", ".shipwake.yml"} {
		data, err := os.ReadFile(filepath.Join(top, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return cfg, err
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("%s: %w", name, err)
		}
		return cfg, nil
	}
	return cfg, nil
}

// loadText reads translation files: flat maps from original text to translated text.
func loadText(top string, files map[string]string) (map[string]model.Names, error) {
	res := map[string]model.Names{}
	for lang, file := range files {
		data, err := os.ReadFile(filepath.Join(top, file))
		if err != nil {
			return nil, fmt.Errorf("translations %s: %w", lang, err)
		}
		var m map[string]string
		if err := yaml.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		for k, v := range m {
			if res[k] == nil {
				res[k] = model.Names{}
			}
			res[k][lang] = v
		}
	}
	if len(res) == 0 {
		return nil, nil
	}
	return res, nil
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func copyFS(src fs.FS, dst string) error {
	return fs.WalkDir(src, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dst, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		data, err := fs.ReadFile(src, path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }
