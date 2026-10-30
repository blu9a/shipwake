package main

// Local mode: a repository Shipwake only reads, its requirements, settings and runs kept in a
// workspace beside it (internal/local). Here: finding a repository's workspace, serving every project
// opened before, and adopting a workspace into its repository.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/blu9a/shipwake/internal/build"
	"github.com/blu9a/shipwake/internal/gitx"
	"github.com/blu9a/shipwake/internal/local"
	"github.com/blu9a/shipwake/skill"
)

// withLocal fills in where a project's files and runs are when it was opened in local mode.
func withLocal(p build.Project) build.Project {
	ctx := context.Background()
	top, err := local.Top(ctx, p.Dir)
	if err != nil {
		return p // not a repository: loading it says so
	}
	l, err := local.Load()
	if err != nil {
		return p
	}
	if e, ok := l.Find(top); ok {
		return fromEntry(*e, p.Results)
	}
	return p
}

func fromEntry(e local.Entry, results string) build.Project {
	p := build.Project{Dir: e.Repo, Results: results, ResultsRepo: e.Workspace}
	if e.Local() {
		p.Spec = e.Workspace
	}
	return p
}

// serveProjects are the repositories named, or else every project opened before, or else this one.
func serveProjects(ctx context.Context, args []string, results string) ([]build.Project, error) {
	if len(args) > 0 {
		return projectsFrom(args, results), nil
	}
	l, err := local.Load()
	if err != nil {
		return nil, err
	}
	var ps []build.Project
	for _, e := range l.Projects {
		if _, err := os.Stat(e.Repo); err != nil {
			continue // moved or deleted since
		}
		ps = append(ps, fromEntry(e, ""))
	}
	if len(ps) == 0 {
		if _, err := local.Top(ctx, "."); err == nil {
			return projectsFrom([]string{"."}, results), nil
		}
	}
	return ps, nil
}

// open lists a repository: in local mode with a workspace of its own, unless it is already set up the
// usual way, with a committed .shipwake.yaml.
func open(ctx context.Context, dir string) (build.Project, error) {
	top, err := local.Top(ctx, dir)
	if err != nil {
		return build.Project{}, err
	}
	repo := gitx.Repo{Dir: top}
	l, err := local.Load()
	if err != nil {
		return build.Project{}, err
	}
	if e, ok := l.Find(top); ok {
		return fromEntry(*e, ""), nil
	}
	for _, name := range []string{".shipwake.yaml", ".shipwake.yml"} {
		if _, err := repo.Git(ctx, "cat-file", "-e", "HEAD:"+name); err == nil {
			l.Projects = append(l.Projects, local.Entry{Repo: top})
			return build.Project{Dir: top}, l.Save()
		}
	}
	e, err := local.Open(ctx, top, build.DefaultTrunk(ctx, repo))
	if err != nil {
		return build.Project{}, err
	}
	return fromEntry(e, ""), nil
}

// skillFile writes the agent skill where agents can read it, and returns its path.
func skillFile() (string, error) {
	home, err := local.Home()
	if err != nil {
		return "", err
	}
	path := filepath.Join(home, "skill", "SKILL.md")
	if old, err := os.ReadFile(path); err == nil && string(old) == skill.Text {
		return path, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, []byte(skill.Text), 0o644)
}

func adoptCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "adopt [repo]",
		Short: "Move a project's requirements and settings from local mode into its repository, in one commit",
		Long: "Copies the workspace's files (.shipwake.yaml, features, metrics, converters) into the repository and\n" +
			"commits them as you, listing what was approved in local mode and by whom. The workspace is kept: it\n" +
			"holds the requirements' history and the runs recorded so far, which Shipwake keeps showing.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			msg, err := adopt(cmd.Context(), dir)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), msg)
			return nil
		},
	}
}

func adopt(ctx context.Context, dir string) (string, error) {
	top, err := local.Top(ctx, dir)
	if err != nil {
		return "", err
	}
	l, err := local.Load()
	if err != nil {
		return "", err
	}
	e, ok := l.Find(top)
	if !ok || !e.Local() {
		return "", fmt.Errorf("%s is not in local mode", top)
	}
	repo, ws := gitx.Repo{Dir: top}, gitx.Repo{Dir: e.Workspace}
	if err := local.Follow(ctx, repo, ws, build.DefaultTrunk(ctx, repo)); err != nil {
		return "", err
	}
	files, err := ws.Lines(ctx, "ls-files")
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("the workspace %s has nothing to adopt yet", e.Workspace)
	}
	var taken []string
	for _, f := range files {
		if _, err := os.Lstat(filepath.Join(top, f)); err == nil {
			taken = append(taken, f)
		}
	}
	if len(taken) > 0 {
		return "", fmt.Errorf("the repository already has %s; move them aside first", strings.Join(taken, ", "))
	}
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(e.Workspace, f))
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(top, f)), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(top, f), data, 0o644); err != nil {
			return "", err
		}
	}
	if _, err := repo.Git(ctx, append([]string{"add", "--"}, files...)...); err != nil {
		return "", err
	}
	// What was approved in local mode, oldest first, and by whom: the record the repository keeps.
	log, err := ws.Lines(ctx, "log", "--reverse", "--format=%s\x1f%(trailers:key=Approved-by,valueonly,separator=%x2C)\x1f%as")
	if err != nil {
		return "", err
	}
	msg := "spec: adopt the requirements kept in local mode\n\nApproved in local mode:\n"
	approved := 0
	for _, line := range log {
		f := strings.Split(line, "\x1f")
		if len(f) < 3 || strings.TrimSpace(f[1]) == "" {
			continue
		}
		msg += fmt.Sprintf("- %s, by %s on %s\n", strings.TrimPrefix(f[0], "spec: "), strings.TrimSpace(f[1]), f[2])
		approved++
	}
	if approved == 0 {
		msg += "- nothing yet\n"
	}
	msg += fmt.Sprintf("\nTheir history and the runs recorded so far stay in the workspace %s.\n", e.Workspace)
	if _, err := repo.Git(ctx, append([]string{"commit", "-q", "-m", msg, "--"}, files...)...); err != nil {
		return "", err
	}
	e.Adopted = true
	if err := l.Save(); err != nil {
		return "", err
	}
	sha, _ := repo.Git(ctx, "rev-parse", "--short", "HEAD")
	return fmt.Sprintf("Adopted %d files into %s in %s. The workspace %s is kept.", len(files), top, strings.TrimSpace(sha), e.Workspace), nil
}
