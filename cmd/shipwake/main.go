// Command shipwake shows what every change did: tests, scenarios, evals and performance on one
// timeline, read from Git and the results your CI already produces.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/invopop/jsonschema"
	"github.com/spf13/cobra"

	"github.com/blu9a/shipwake/internal/build"
	"github.com/blu9a/shipwake/internal/gitx"
	"github.com/blu9a/shipwake/internal/judge"
	"github.com/blu9a/shipwake/internal/model"
	"github.com/blu9a/shipwake/internal/readers"
	"github.com/blu9a/shipwake/internal/site"
	"github.com/blu9a/shipwake/internal/store"
	"github.com/blu9a/shipwake/internal/story"
)

var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	root := &cobra.Command{
		Use:   "shipwake",
		Short: "See what every change did: tests, scenarios, evals and performance on one timeline",
		Long: "Shipwake reads Git and the results your CI already produces, and builds a static site:\n" +
			"the trunk as merges and commits, the runs on them, releases, and what each change did.\n\n" +
			"Start with: shipwake init",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(storiesCmd(), initCmd(), addCmd(), buildCmd(), serveCmd(), checkCmd(), doctorCmd(), formatsCmd(), schemaCmd(), versionCmd())
	if err := root.ExecuteContext(ctx); err != nil {
		var exit exitError
		if errors.As(err, &exit) {
			os.Exit(int(exit))
		}
		fmt.Fprintln(os.Stderr, "shipwake:", err)
		os.Exit(1)
	}
}

// exitError ends the program with a code and no message: the command already printed its report.
type exitError int

func (e exitError) Error() string { return fmt.Sprintf("exit %d", int(e)) }

// projectsFrom reads "path" or "path=results" arguments; no arguments means the current directory.
func projectsFrom(args []string, results string) []build.Project {
	if len(args) == 0 {
		args = []string{"."}
	}
	var ps []build.Project
	for _, a := range args {
		dir, res, _ := strings.Cut(a, "=")
		if res == "" && len(args) == 1 {
			res = results
		}
		ps = append(ps, build.Project{Dir: dir, Results: res})
	}
	return ps
}

func logf(cmd *cobra.Command) func(string, ...any) {
	return func(f string, a ...any) { fmt.Fprintf(cmd.ErrOrStderr(), f+"\n", a...) }
}

const resultsHelp = `where runs are stored: a directory, or git:<branch> (default: the config's "results", else git:shipwake-results)`

func buildCmd() *cobra.Command {
	var out, results string
	var rows int
	c := &cobra.Command{
		Use:   "build [repo[=results]]...",
		Short: "Write a static site for one or more repositories",
		Example: "  shipwake build --out site\n" +
			"  shipwake build ../app ../api=git:shipwake-results --out site",
		RunE: func(cmd *cobra.Command, args []string) error {
			return build.Run(cmd.Context(), build.Options{Projects: projectsFrom(args, results), Out: out, MaxRows: rows, Web: site.Web(), Log: logf(cmd)})
		},
	}
	c.Flags().StringVarP(&out, "out", "o", "site", "output directory")
	c.Flags().StringVarP(&results, "results", "r", "", resultsHelp)
	c.Flags().IntVar(&rows, "max-rows", 0, fmt.Sprintf("trunk history to include (default %d; -1 for all)", build.DefaultRows))
	return c
}

func serveCmd() *cobra.Command {
	var addr, results string
	var rows int
	c := &cobra.Command{
		Use:   "serve [repo[=results]]...",
		Short: "Build into a temporary directory and serve it locally",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.MkdirTemp("", "shipwake-*")
			if err != nil {
				return err
			}
			defer os.RemoveAll(dir)
			ws, err := newWorkspace(cmd.Context(), build.Options{Projects: projectsFrom(args, results), Out: dir, MaxRows: rows, Web: site.Web(), Log: logf(cmd)})
			if err != nil {
				return err
			}
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return err
			}
			srv := &http.Server{Handler: ws, ReadHeaderTimeout: 10 * time.Second}
			fmt.Fprintf(cmd.OutOrStdout(), "Serving on http://%s\n", ln.Addr())
			go func() { <-cmd.Context().Done(); _ = srv.Close() }()
			if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
	}
	c.Flags().StringVar(&addr, "addr", "127.0.0.1:7070", "address to listen on")
	c.Flags().StringVarP(&results, "results", "r", "", resultsHelp)
	c.Flags().IntVar(&rows, "max-rows", 0, "trunk history to include (-1 for all)")
	return c
}

func addCmd() *cobra.Command {
	var format, file, name, kind, commit, branch, results, repoDir string
	var ctxPairs []string
	var push, dryRun bool
	c := &cobra.Command{
		Use:   "add --format F --file PATH --name NAME",
		Short: "Record a run: read a tool's output and save it with the commit it tested",
		Example: "  go test -json ./... > test.json\n" +
			"  shipwake add -f go-test --file test.json -n \"Go tests\"\n\n" +
			"  # in CI, keeping the run: --push sends the results branch to origin (never done on its own)\n" +
			"  npx playwright test --reporter=json > results.json\n" +
			"  shipwake add -f playwright --file results.json -n \"Web acceptance\" --context runner=ci-linux --push",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			repo := gitx.Repo{Dir: repoDir}
			top, err := repo.Git(ctx, "rev-parse", "--show-toplevel")
			if err != nil {
				return err
			}
			repo = gitx.Repo{Dir: strings.TrimSpace(top)}
			rd, err := readers.Find(format, repo.Dir) // the project's own converters first
			if err != nil {
				return err
			}
			f, err := os.Open(file)
			if err != nil {
				return err
			}
			defer f.Close()
			run, err := rd.Read(f)
			if err != nil {
				return fmt.Errorf("%s: %w", file, err)
			}
			// A file that names its commit and branch (Shipwake's own format) keeps them unless flags say otherwise.
			if run.Commit == "" || cmd.Flags().Changed("commit") {
				run.Commit = commit
			}
			if run.Commit, err = repo.Resolve(ctx, run.Commit); err != nil {
				return err
			}
			if branch == "" {
				branch = run.Branch
			}
			if branch == "" {
				branch = currentBranch(ctx, repo)
			}
			run.Branch, run.Format = branch, format
			if name != "" {
				run.Name = name
			}
			if run.Name == "" {
				return errors.New("--name is required: this format does not carry a run name")
			}
			if run.Kind == "" {
				run.Kind = rd.Kind
			}
			if kind != "" {
				run.Kind = kind
			}
			if run.Start.IsZero() {
				run.Start = time.Now().UTC()
			}
			for _, kv := range ctxPairs {
				k, v, ok := strings.Cut(kv, "=")
				if !ok {
					return fmt.Errorf("--context %q: want key=value", kv)
				}
				if run.Context == nil {
					run.Context = map[string]string{}
				}
				run.Context[k] = v
			}
			if dryRun {
				preview(cmd.OutOrStdout(), rd, run)
				return nil
			}
			run.ID = store.NewID(run.Start)

			spec := results
			if spec == "" {
				cfg, _ := build.LoadConfig(repo.Dir)
				spec = cfg.Results
			}
			if spec == "" {
				spec = store.DefaultBranch
			}
			src, _ := filepath.Abs(filepath.Dir(file))
			where := spec
			if b := store.Branch(spec); b != "" {
				dir, done, err := store.Checkout(ctx, repo, b)
				if err != nil {
					return err
				}
				if err := (store.Store{Dir: dir}).Save(&run, src); err != nil {
					_ = done("", false)
					return err
				}
				msg := fmt.Sprintf("%s %q on %.7s", run.Kind, run.Name, run.Commit)
				if err := done(msg, push); err != nil {
					return err
				}
				where = "branch " + b
				if push {
					where += " (pushed)"
				}
			} else if err := (store.Store{Dir: spec}).Save(&run, src); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Recorded %s %q on %.7s in %s: %s\n", run.Kind, run.Name, run.Commit, where, describe(run))
			return nil
		},
	}
	c.Flags().StringVarP(&format, "format", "f", "", "the tool's output format (see shipwake formats)")
	c.Flags().StringVar(&file, "file", "", "the output file")
	c.Flags().StringVarP(&name, "name", "n", "", `the run's name, such as "Web acceptance tests" (default: from the file, when the format has one)`)
	c.Flags().StringVar(&kind, "kind", "", "your word for this kind of run: test, eval, load test, benchmark… (default: from the format)")
	c.Flags().StringVar(&commit, "commit", "HEAD", "the commit that was tested")
	c.Flags().StringVar(&branch, "branch", "", "the branch (default: the current branch, or CI's)")
	c.Flags().StringVarP(&results, "results", "r", "", resultsHelp)
	c.Flags().StringVar(&repoDir, "repo", ".", "the repository")
	c.Flags().StringArrayVar(&ctxPairs, "context", nil, "what the run used, as key=value: runner, model, load… (repeatable)")
	c.Flags().BoolVar(&push, "push", false, "push the results branch to origin")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be recorded, and record nothing")
	for _, f := range []string{"format", "file"} {
		_ = c.MarkFlagRequired(f)
	}
	return c
}

// currentBranch finds the branch being tested, also on CI machines that check out a detached commit.
func currentBranch(ctx context.Context, repo gitx.Repo) string {
	for _, env := range []string{"GITHUB_HEAD_REF", "GITHUB_REF_NAME", "CI_COMMIT_REF_NAME", "BUILDKITE_BRANCH", "CIRCLE_BRANCH"} {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	if b, err := repo.Git(ctx, "branch", "--show-current"); err == nil {
		return strings.TrimSpace(b)
	}
	return ""
}

func describe(r model.Run) string {
	var parts []string
	if len(r.Tests) > 0 || len(r.Metrics) == 0 {
		var failed int
		for _, t := range r.Tests {
			if t.Status == "failed" {
				failed++
			}
		}
		parts = append(parts, fmt.Sprintf("%s, %d failed", plural(len(r.Tests), "test"), failed))
	}
	if len(r.Metrics) > 0 {
		parts = append(parts, plural(len(r.Metrics), "measurement"))
	}
	return strings.Join(parts, "; ")
}

func formatsCmd() *cobra.Command {
	var repoDir string
	c := &cobra.Command{
		Use:   "formats",
		Short: "List the output formats Shipwake reads: built in, and the project's own converters",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			for _, r := range readers.All() {
				fmt.Fprintf(out, "%-14s %-12s %s\n", r.Format, r.Kind, r.About)
			}
			own, err := readers.Project(repoDir)
			for _, r := range own {
				fmt.Fprintf(out, "%-14s %-12s %s (%s)\n", r.Format, r.Kind, r.About, r.Source)
			}
			return err
		},
	}
	c.Flags().StringVar(&repoDir, "repo", ".", "the repository whose own converters to list")
	c.AddCommand(formatsTestCmd())
	return c
}

func formatsTestCmd() *cobra.Command {
	var repoDir string
	var update, builtin bool
	c := &cobra.Command{
		Use:   "test [format]...",
		Short: "Check converters against their sample reports (the project's own; --builtin for Shipwake's)",
		Long: "Each converter names a real report from its tool (sample) and the run it converts to (expected).\n" +
			"test converts the sample again and compares. After changing a converter on purpose, run with\n" +
			"--update and review the new expected run like any other change.",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			list, err := readers.Project(repoDir)
			if err != nil {
				return err
			}
			if builtin {
				for _, r := range readers.All() {
					if r.Spec != nil {
						list = append(list, r)
					}
				}
			}
			failed, checked := 0, 0
			for _, r := range list {
				if len(args) > 0 && !slices.Contains(args, r.Format) {
					continue
				}
				checked++
				if err := readers.CheckSpec(r, repoDir, update); err != nil {
					failed++
					fmt.Fprintf(out, "✗ %s (%s): %v\n", r.Format, r.Source, err)
					continue
				}
				verb := "reads its sample as expected"
				if update {
					verb = "expected run written: review it"
				}
				fmt.Fprintf(out, "✓ %s (%s): %s\n", r.Format, r.Source, verb)
			}
			if checked == 0 {
				fmt.Fprintf(out, "No converters to check in %s.\n", readers.ProjectDir)
			}
			if failed > 0 {
				return exitError(1)
			}
			return nil
		},
	}
	c.Flags().StringVar(&repoDir, "repo", ".", "the repository")
	c.Flags().BoolVar(&update, "update", false, "write the expected runs from the samples instead of comparing")
	c.Flags().BoolVar(&builtin, "builtin", false, "also check Shipwake's built-in converters")
	return c
}

// preview prints what add would record.
func preview(w io.Writer, rd readers.Reader, run model.Run) {
	source := rd.Source
	if source == "" {
		source = "built-in reader"
	}
	fmt.Fprintf(w, "Would record %s %q on %.7s (branch %s), read as %s (%s): %s\n", run.Kind, run.Name, run.Commit, run.Branch, rd.Format, source, describe(run))
	byStatus := map[string]int{}
	for _, t := range run.Tests {
		byStatus[t.Status]++
	}
	if len(run.Tests) > 0 {
		fmt.Fprintf(w, "  %d passed, %d failed, %d skipped\n", byStatus["passed"], byStatus["failed"], byStatus["skipped"])
	}
	for i, t := range run.Tests {
		if i == 8 {
			fmt.Fprintf(w, "  … %d more\n", len(run.Tests)-i)
			break
		}
		name := t.Name
		if t.Feature != "" {
			name = t.Feature + " / " + name
		}
		steps := ""
		if len(t.Steps) > 0 {
			steps = fmt.Sprintf(" (%s)", plural(len(t.Steps), "step"))
		}
		fmt.Fprintf(w, "  %-7s %s%s\n", t.Status, name, steps)
	}
	for i, m := range run.Metrics {
		if i == 8 {
			fmt.Fprintf(w, "  … %d more\n", len(run.Metrics)-i)
			break
		}
		fmt.Fprintf(w, "  %s = %s\n", m.ID, judge.Format(judge.Mean(m.Values), m.Unit))
	}
	fmt.Fprintln(w, "Nothing was recorded (--dry-run).")
}

func schemaCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "schema",
		Short: "Print the JSON Schema of the files Shipwake reads and writes",
		RunE: func(cmd *cobra.Command, args []string) error {
			r := &jsonschema.Reflector{}
			s := r.Reflect(&struct {
				Site   model.Site                `json:"site"`
				Index  model.Project             `json:"index"`
				Commit model.CommitDetail        `json:"commit"`
				Diffs  map[string]model.FileDiff `json:"diffs"`
				Run    model.Run                 `json:"run"`
				Config model.Config              `json:"config"`
			}{})
			s.Title = "Shipwake"
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(s)
		},
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Run:   func(cmd *cobra.Command, args []string) { fmt.Fprintln(cmd.OutOrStdout(), "shipwake", version) },
	}
}

// storiesCmd prints each feature's requirements loop.
func storiesCmd() *cobra.Command {
	var results string
	var asJSON bool
	c := &cobra.Command{
		Use:    "stories [repo]",
		Short:  "Print each feature's requirements loop: proposed, changed, approved, passing (spike)",
		Hidden: true,
		Args:   cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := build.Load(cmd.Context(), projectsFrom(args, results)[0], -1, logf(cmd))
			if err != nil {
				return err
			}
			defer l.Close()
			features, err := story.Build(cmd.Context(), l.Repo, l.Runs)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if asJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(features)
			}
			for _, f := range features {
				fmt.Fprintf(out, "%s (%s)\n", f.Name, f.Path)
				for _, s := range f.Scenarios {
					fmt.Fprintf(out, "  %s — %s\n", s.Title, s.State)
					for _, e := range s.Events {
						who := e.By
						line := fmt.Sprintf("    %s  %-8s v%d", e.At.Local().Format("Jan 2 15:04"), e.Kind, e.Version)
						if who != "" {
							line += "  by " + who
						}
						if e.Commit != "" {
							line += fmt.Sprintf("  %.7s", e.Commit)
						}
						if e.Video != "" {
							line += "  ▶ " + e.Video
						}
						fmt.Fprintln(out, line)
						if e.Kind == "changed" {
							for _, st := range e.Was {
								if !slices.Contains(e.Steps, st) {
									fmt.Fprintf(out, "                 - %s\n", st)
								}
							}
							for _, st := range e.Steps {
								if !slices.Contains(e.Was, st) {
									fmt.Fprintf(out, "                 + %s\n", st)
								}
							}
						}
					}
				}
			}
			return nil
		},
	}
	c.Flags().StringVarP(&results, "results", "r", "", resultsHelp)
	c.Flags().BoolVar(&asJSON, "json", false, "print as JSON")
	return c
}
