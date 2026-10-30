// Package local keeps projects in local mode: Shipwake reads a repository and writes nothing into it.
// Each such project has a workspace of its own, a Git repository beside it, with what a repository
// set up the usual way would hold: .shipwake.yaml, features, metrics, converters, and its runs and
// recordings on a results branch. Deleting the workspace removes every trace.
//
//	~/.shipwake/projects.yaml     every project opened, in local mode or not
//	~/.shipwake/<name>/           a project's workspace
//	~/.shipwake/skill/SKILL.md    the agent skill, for agents to read
//
// SHIPWAKE_HOME moves the folder elsewhere.
package local

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/blu9a/shipwake/internal/gitx"
)

// Home is the folder Shipwake keeps its own files in.
func Home() (string, error) {
	if h := os.Getenv("SHIPWAKE_HOME"); h != "" {
		return filepath.Abs(h)
	}
	u, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(u, ".shipwake"), nil
}

// Entry is one project. With a workspace it is in local mode; once adopted, the repository holds its
// requirements and the workspace only its earlier runs.
type Entry struct {
	Repo      string `yaml:"repo"`
	Workspace string `yaml:"workspace,omitempty"`
	Adopted   bool   `yaml:"adopted,omitempty"`
}

// Local says whether Shipwake's files are in the workspace rather than the repository.
func (e Entry) Local() bool { return e.Workspace != "" && !e.Adopted }

// List is projects.yaml.
type List struct {
	Projects []Entry `yaml:"projects"`
	path     string
}

// Load reads projects.yaml; none yet is an empty list.
func Load() (*List, error) {
	home, err := Home()
	if err != nil {
		return nil, err
	}
	l := &List{path: filepath.Join(home, "projects.yaml")}
	data, err := os.ReadFile(l.path)
	if errors.Is(err, os.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, l); err != nil {
		return nil, fmt.Errorf("%s: %w", l.path, err)
	}
	return l, nil
}

// Save writes projects.yaml.
func (l *List) Save() error {
	data, err := yaml.Marshal(l)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(l.path, data, 0o644)
}

// Find is the entry of a repository, by its top folder.
func (l *List) Find(repo string) (*Entry, bool) {
	for i := range l.Projects {
		if l.Projects[i].Repo == repo {
			return &l.Projects[i], true
		}
	}
	return nil, false
}

// Top is a repository's top folder, absolute, as entries name it.
func Top(ctx context.Context, dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	out, err := (gitx.Repo{Dir: abs}).Git(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("%s is not a Git repository", dir)
	}
	return strings.TrimSpace(out), nil
}

// Open puts a repository in local mode: it makes its workspace, an empty Git repository whose first
// branch is named as the repository's trunk, and lists it. The repository is only read. Opening one
// already listed returns its entry.
func Open(ctx context.Context, dir, trunk string) (Entry, error) {
	repo, err := Top(ctx, dir)
	if err != nil {
		return Entry{}, err
	}
	l, err := Load()
	if err != nil {
		return Entry{}, err
	}
	if e, ok := l.Find(repo); ok {
		return *e, nil
	}
	home, err := Home()
	if err != nil {
		return Entry{}, err
	}
	ws := filepath.Join(home, filepath.Base(repo))
	for n := 2; exists(ws); n++ {
		ws = filepath.Join(home, fmt.Sprintf("%s-%d", filepath.Base(repo), n))
	}
	w := gitx.Repo{Dir: ws}
	if err := os.MkdirAll(ws, 0o755); err != nil {
		return Entry{}, err
	}
	if _, err := w.Git(ctx, "init", "-q", "-b", trunk); err != nil {
		return Entry{}, err
	}
	// A first commit, so the trunk exists and branches can start from it.
	if _, err := Commit(ctx, repo, w, "chore: start the workspace of "+filepath.Base(repo), "--allow-empty"); err != nil {
		return Entry{}, err
	}
	e := Entry{Repo: repo, Workspace: ws}
	l.Projects = append(l.Projects, e)
	return e, l.Save()
}

// Commit commits in the workspace as the person: the identity the repository's Git gives them, since
// that is who they are in this project.
func Commit(ctx context.Context, repo string, ws gitx.Repo, msg string, args ...string) (string, error) {
	r := gitx.Repo{Dir: repo}
	name, _ := r.Git(ctx, "config", "user.name")
	email, _ := r.Git(ctx, "config", "user.email")
	git := []string{"-c", "user.name=" + strings.TrimSpace(name), "-c", "user.email=" + strings.TrimSpace(email), "commit", "-q", "-m", msg}
	if _, err := ws.Git(ctx, append(git, args...)...); err != nil {
		return "", err
	}
	sha, err := ws.Git(ctx, "rev-parse", "HEAD")
	return strings.TrimSpace(sha), err
}

// Follow checks out, in the workspace, the branch the repository has checked out: requirements belong
// to a branch. A branch new to the workspace starts from its trunk. A detached repository leaves the
// workspace as it is.
func Follow(ctx context.Context, repo, ws gitx.Repo, trunk string) error {
	out, err := repo.Git(ctx, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil {
		return nil
	}
	branch := strings.TrimSpace(out)
	if here, err := ws.Git(ctx, "symbolic-ref", "--short", "-q", "HEAD"); err == nil && strings.TrimSpace(here) == branch {
		return nil
	}
	if dirty, err := ws.Git(ctx, "status", "--porcelain"); err != nil {
		return err
	} else if strings.TrimSpace(dirty) != "" {
		return fmt.Errorf("the workspace %s has changes that aren't committed; commit or discard them first", ws.Dir)
	}
	if _, err := ws.Git(ctx, "rev-parse", "-q", "--verify", "refs/heads/"+branch); err == nil {
		_, err = ws.Git(ctx, "switch", "-q", branch)
		return err
	}
	_, err = ws.Git(ctx, "switch", "-q", "-c", branch, trunk)
	return err
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
