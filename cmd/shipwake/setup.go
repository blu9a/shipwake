package main

import (
	"cmp"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/blu9a/shipwake/internal/build"
	"github.com/blu9a/shipwake/internal/gitx"
	"github.com/blu9a/shipwake/internal/judge"
	"github.com/blu9a/shipwake/internal/local"
	"github.com/blu9a/shipwake/internal/readers"
	"github.com/blu9a/shipwake/internal/store"
)

// stack is something init recognizes in a repository, and how to record its results.
type stack struct {
	Name    string
	Run     string // the test command, writing a file Shipwake reads
	Format  string
	File    string
	RunName string
}

func initCmd() *cobra.Command {
	var write, toLocal bool
	c := &cobra.Command{
		Use:   "init [repo]",
		Short: "Look at a repository and show how to record its results (nothing is changed unless --write)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			ctx := cmd.Context()
			repo := gitx.Repo{Dir: dir}
			top, err := repo.Git(ctx, "rev-parse", "--show-toplevel")
			if err != nil {
				return fmt.Errorf("not a Git repository: %w", err)
			}
			top = strings.TrimSpace(top)
			out := cmd.OutOrStdout()
			// In local mode Shipwake's files are in the project's workspace, never in the repository.
			if toLocal {
				if _, err := open(ctx, top); err != nil {
					return err
				}
			}
			proj := withLocal(build.Project{Dir: top})
			found := detect(top)
			features := count(top, ".feature")
			fmt.Fprintf(out, "%s\n\n", filepath.Base(top))
			if len(found) == 0 {
				fmt.Fprintln(out, "No test setup recognized. Any runner that writes JUnit XML works: pass --format junit.")
			} else {
				fmt.Fprintln(out, "Found:")
				for _, s := range found {
					fmt.Fprintf(out, "  • %s\n", s.Name)
				}
			}
			if features > 0 {
				fmt.Fprintf(out, "  • %d .feature files: scenario changes will show on each commit\n", features)
			}
			if proj.Spec != "" {
				fmt.Fprintf(out, "\nLocal mode: nothing is written into this repository. Settings, requirements and results are in\nthe workspace %s.\n", proj.Spec)
				fmt.Fprintln(out, "\nAfter running the tests here:")
			} else {
				fmt.Fprintln(out, "\nIn CI, after the tests:")
			}
			for _, s := range found {
				fmt.Fprintf(out, "  %s\n  shipwake add -f %s --file %s -n %q\n", s.Run, s.Format, s.File, s.RunName)
			}
			if len(found) == 0 {
				fmt.Fprintln(out, "  shipwake add -f junit --file report.xml -n \"Tests\"")
			}
			if proj.Spec != "" {
				fmt.Fprintln(out, "\nRuns go to the workspace's results branch.")
			} else {
				fmt.Fprintf(out, "\nResults go to the branch %q in this repository: no server or bucket needed.\n", store.Branch(store.DefaultBranch))
				fmt.Fprintln(out, "Shipwake never pushes on its own. To keep runs from CI, add --push: it sends that branch to origin.")
			}
			fmt.Fprintln(out, "Then: shipwake serve        (look at it locally)")
			fmt.Fprintln(out, "      shipwake build -o site  (publish the folder, e.g. on GitHub Pages)")
			fmt.Fprintln(out, "      shipwake check          (a CI gate: exit 1 when something blocks)")
			specDir := cmp.Or(proj.Spec, top)
			cfgPath := filepath.Join(specDir, ".shipwake.yaml")
			if write {
				if _, err := os.Stat(cfgPath); err == nil {
					fmt.Fprintf(out, "\n%s already exists; left as it is.\n", ".shipwake.yaml")
					return nil
				}
				trunk := "main"
				if out, err := repo.Git(ctx, "branch", "--show-current"); err == nil && strings.TrimSpace(out) == "master" {
					trunk = "master"
				}
				cfg := fmt.Sprintf(starterConfig, filepath.Base(top), trunk)
				ws := gitx.Repo{Dir: proj.Spec}
				if proj.Spec != "" {
					// on the repository's branch first: the settings belong to it
					if err := local.Follow(ctx, repo, ws, build.DefaultTrunk(ctx, repo)); err != nil {
						return err
					}
				}
				if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
					return err
				}
				if proj.Spec != "" {
					if _, err := ws.Git(ctx, "add", ".shipwake.yaml"); err != nil {
						return err
					}
					if _, err := local.Commit(ctx, top, ws, "chore: add Shipwake's settings"); err != nil {
						return err
					}
					fmt.Fprintf(out, "\nWrote and committed .shipwake.yaml in the workspace. Everything in it is optional.\n")
				} else {
					fmt.Fprintf(out, "\nWrote .shipwake.yaml. Everything in it is optional.\n")
				}
			} else {
				fmt.Fprintln(out, "\nOptional: shipwake init --write  adds a commented .shipwake.yaml (names, languages, thresholds).")
			}
			return nil
		},
	}
	c.Flags().BoolVar(&write, "write", false, "write a starter .shipwake.yaml (in local mode, into the workspace, committed)")
	c.Flags().BoolVar(&toLocal, "local", false, "put the repository in local mode: Shipwake's files go to a workspace of its own, never into the repository")
	return c
}

const starterConfig = `# Shipwake. Every setting is optional.
name: %s
trunk: %s

# Languages for release notes and project text. The first is required for release notes.
# languages: [en, zh]
# translations:
#   zh: .shipwake/zh.yaml     # "original text": "translation"

# Display names for test targets (Playwright projects, devices…).
# targets:
#   desktop: { en: Web desktop, zh: 网页桌面版 }

# Measurements by ID or pattern: which way is better (overrides the tool), and thresholds.
# Numbers are absolute; "+15%%" and "-10%%" are changes from the baseline.
# metrics:
#   "api.*.p95": { direction: lower, warn: "+15%%", block: "+40%%" }
#   quality:     { direction: higher, warn: 0.88, block: 0.82 }

# Run kinds are your own words (--kind). Optionally an icon (test, gauge, bolt, timer, shield, chart)
# and names in other languages.
# kinds:
#   eval:      { icon: gauge, zh: 评测 }
#   load test: { icon: bolt, zh: 压测 }

# Where runs are stored: git:<branch> (default git:shipwake-results) or a directory.
# results: git:shipwake-results
`

func detect(top string) []stack {
	has := func(name string) bool { _, err := os.Stat(filepath.Join(top, name)); return err == nil }
	contains := func(name, s string) bool {
		data, err := os.ReadFile(filepath.Join(top, name))
		return err == nil && strings.Contains(string(data), s)
	}
	var res []stack
	if has("go.mod") {
		res = append(res, stack{"Go (go test)", "go test -json ./... > go-test.json", "go-test", "go-test.json", "Go tests"})
		if count(top, "_test.go") > 0 && grep(top, "_test.go", "func Benchmark") {
			res = append(res, stack{"Go benchmarks", "go test -run '^$' -bench . -benchmem -count 5 ./... > bench.txt", "go-bench", "bench.txt", "Benchmarks"})
		}
	}
	if contains("package.json", "@playwright/test") || has("playwright.config.ts") || has("playwright.config.js") {
		res = append(res, stack{"Playwright", "npx playwright test --reporter=json > playwright.json", "playwright", "playwright.json", "Web tests"})
	}
	if contains("package.json", "\"vitest\"") {
		res = append(res, stack{"Vitest", "npx vitest run --reporter=junit --outputFile=vitest.xml", "junit", "vitest.xml", "Unit tests"})
	} else if contains("package.json", "\"jest\"") {
		res = append(res, stack{"Jest (with jest-junit)", "npx jest --reporters=default --reporters=jest-junit", "junit", "junit.xml", "Unit tests"})
	}
	if has("pytest.ini") || contains("pyproject.toml", "pytest") || contains("requirements.txt", "pytest") {
		res = append(res, stack{"pytest", "pytest --junitxml=pytest.xml", "junit", "pytest.xml", "Python tests"})
	}
	if has("pom.xml") {
		res = append(res, stack{"Maven (Surefire)", "mvn test   # reports in target/surefire-reports/*.xml", "junit", "target/surefire-reports/TEST-*.xml", "Java tests"})
	}
	if has("build.gradle") || has("build.gradle.kts") {
		res = append(res, stack{"Gradle", "./gradlew test   # reports in build/test-results/test/*.xml", "junit", "build/test-results/test/TEST-*.xml", "JVM tests"})
	}
	if has("Cargo.toml") {
		res = append(res, stack{"Rust (cargo-nextest)", "cargo nextest run --profile ci   # with junit output enabled in .config/nextest.toml", "junit", "target/nextest/ci/junit.xml", "Rust tests"})
	}
	return res
}

// count counts files with a suffix, skipping dependency and build folders.
func count(top, suffix string) int {
	n := 0
	_ = walk(top, func(path string) {
		if strings.HasSuffix(path, suffix) {
			n++
		}
	})
	return n
}

func grep(top, suffix, text string) bool {
	found := false
	_ = walk(top, func(path string) {
		if found || !strings.HasSuffix(path, suffix) {
			return
		}
		if data, err := os.ReadFile(path); err == nil && strings.Contains(string(data), text) {
			found = true
		}
	})
	return found
}

func walk(top string, fn func(string)) error {
	skip := []string{".git", "node_modules", "vendor", "target", "dist", "build", ".venv", ".cache", ".tools"}
	return filepath.WalkDir(top, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && slices.Contains(skip, d.Name()) {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			fn(path)
		}
		return nil
	})
}

func doctorCmd() *cobra.Command {
	var results string
	c := &cobra.Command{
		Use:   "doctor [repo]",
		Short: "Check that a repository's setup works: config, history, results",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			problems := 0
			say := func(ok bool, f string, a ...any) {
				mark := "✓"
				if !ok {
					mark, problems = "✗", problems+1
				}
				fmt.Fprintf(out, "  %s %s\n", mark, fmt.Sprintf(f, a...))
			}
			ps := projectsFrom(args, results)
			var warnings []string
			l, err := build.Load(cmd.Context(), ps[0], build.DefaultRows, func(f string, a ...any) { warnings = append(warnings, fmt.Sprintf(f, a...)) })
			if err != nil {
				say(false, "%v", err)
				return exitError(1)
			}
			defer l.Close()
			p := l.Index
			fmt.Fprintf(out, "%s\n", p.Name)
			say(true, "trunk %q: %d entries%s, %d releases, %d open branches", p.Trunk, len(p.Rows), map[bool]string{true: " (truncated)", false: ""}[p.Truncated], len(p.Tags), len(p.Branches))
			say(len(l.Runs) > 0, "%d runs recorded", len(l.Runs))
			for _, w := range warnings {
				say(false, "%s", w)
			}
			for id, rule := range p.Rules {
				for _, v := range []string{rule.Warn, rule.Block} {
					if _, err := judge.ParseThreshold(v); err != nil {
						say(false, "metric %q: %v", id, err)
					}
				}
				if d := rule.Direction; d != "" && d != "higher" && d != "lower" {
					say(false, "metric %q: direction %q should be higher or lower", id, d)
				}
			}
			// The project's own converters must still read their sample reports as reviewed.
			own, err := readers.Project(l.Repo.Dir)
			if err != nil {
				say(false, "%v", err)
			}
			for _, r := range own {
				if err := readers.CheckSpec(r, l.Repo.Dir, false); err != nil {
					say(false, "converter %s (%s): %v", r.Format, r.Source, err)
				} else {
					say(true, "converter %s reads its sample as expected", r.Format)
				}
			}
			if len(p.Languages) > 1 && len(p.Text) == 0 {
				// A note, not a problem: release notes are still written in every language.
				fmt.Fprintf(out, "  • languages %v, no translations file: feature and run names stay in %s\n", p.Languages, p.Languages[0])
			}
			if problems == 0 {
				fmt.Fprintln(out, "Everything looks fine.")
				return nil
			}
			return exitError(1)
		},
	}
	c.Flags().StringVarP(&results, "results", "r", "", resultsHelp)
	return c
}
