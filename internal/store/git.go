package store

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blu9a/shipwake/internal/gitx"
)

// DefaultBranch is where runs are kept when a project says nothing else: an orphan branch in the
// project's own repository, so results need no server, bucket or artifact storage.
const DefaultBranch = "git:shipwake-results"

// Branch returns the branch of a "git:<branch>" location, or "" for a directory.
func Branch(spec string) string {
	b, ok := strings.CutPrefix(spec, "git:")
	if !ok {
		return ""
	}
	return b
}

// Checkout gives a writable work tree of the results branch, creating the branch (orphan) if needed.
// Call done with a message to commit the changes; push also sends them to origin, retrying on races.
func Checkout(ctx context.Context, repo gitx.Repo, branch string) (dir string, done func(msg string, push bool) error, err error) {
	dir, err = os.MkdirTemp("", "shipwake-results-*")
	if err != nil {
		return "", nil, err
	}
	_ = os.Remove(dir) // git worktree wants to create it
	cleanup := func() {
		_, _ = repo.Git(context.WithoutCancel(ctx), "worktree", "remove", "--force", dir)
		_ = os.RemoveAll(dir)
	}
	local := exists(ctx, repo, "refs/heads/"+branch)
	if !local {
		fetch(ctx, repo, branch)
	}
	switch {
	case local:
		_, err = repo.Git(ctx, "worktree", "add", "--force", dir, branch)
	case exists(ctx, repo, "refs/remotes/origin/"+branch):
		_, err = repo.Git(ctx, "worktree", "add", "--force", "-b", branch, dir, "origin/"+branch)
	default: // a new branch with no history of its own, made with commands every Git version has
		var tree, root string
		if tree, err = repo.Git(ctx, "mktree"); err == nil {
			root, err = repo.Git(ctx, identity(ctx, repo, "commit-tree", strings.TrimSpace(tree), "-m", "Shipwake results")...)
		}
		if err == nil {
			_, err = repo.Git(ctx, "update-ref", "refs/heads/"+branch, strings.TrimSpace(root))
		}
		if err == nil {
			_, err = repo.Git(ctx, "worktree", "add", "--force", dir, branch)
		}
	}
	if err != nil {
		cleanup()
		return "", nil, err
	}
	wt := gitx.Repo{Dir: dir}
	done = func(msg string, push bool) error {
		defer cleanup()
		if _, err := wt.Git(ctx, "add", "-A"); err != nil {
			return err
		}
		if out, _ := wt.Git(ctx, "status", "--porcelain"); strings.TrimSpace(out) == "" {
			return nil
		}
		if _, err := wt.Git(ctx, identity(ctx, wt, "commit", "-q", "-m", msg)...); err != nil {
			return err
		}
		if !push {
			return nil
		}
		var last error
		for range 4 { // another job may have pushed in between: rebase onto it and try again
			if _, last = wt.Git(ctx, "push", "-q", "origin", branch); last == nil {
				return nil
			}
			if _, err := wt.Git(ctx, "fetch", "-q", "origin", branch); err != nil {
				return err
			}
			if _, err := wt.Git(ctx, identity(ctx, wt, "rebase", "-q", "origin/"+branch)...); err != nil {
				return err
			}
		}
		return fmt.Errorf("push %s: %w", branch, last)
	}
	return dir, done, nil
}

// identity adds a committer when git has none configured, as on fresh CI machines.
func identity(ctx context.Context, r gitx.Repo, args ...string) []string {
	if out, err := r.Git(ctx, "config", "user.email"); err == nil && strings.TrimSpace(out) != "" {
		return args
	}
	return append([]string{"-c", "user.name=Shipwake", "-c", "user.email=shipwake@users.noreply.github.com"}, args...)
}

// Export copies the results branch (local, else origin's) into dir, read-only. found is false when
// the branch doesn't exist yet.
func Export(ctx context.Context, repo gitx.Repo, branch, dir string) (found bool, err error) {
	if !exists(ctx, repo, "refs/heads/"+branch) {
		fetch(ctx, repo, branch)
	}
	rev := ""
	for _, r := range []string{"refs/heads/" + branch, "refs/remotes/origin/" + branch} {
		if exists(ctx, repo, r) {
			rev = r
			break
		}
	}
	if rev == "" {
		return false, nil
	}
	cmd := exec.CommandContext(ctx, "git", "-C", repo.Dir, "archive", "--format=tar", rev)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return false, err
	}
	if err := cmd.Start(); err != nil {
		return false, err
	}
	if err := untar(out, dir); err != nil {
		_ = cmd.Wait()
		return false, err
	}
	return true, cmd.Wait()
}

// fetch updates origin's copy of the branch, as CI checkouts usually have only the commit under test.
// Without an origin or the branch there, nothing happens.
func fetch(ctx context.Context, repo gitx.Repo, branch string) {
	if _, err := repo.Git(ctx, "remote", "get-url", "origin"); err != nil {
		return
	}
	_, _ = repo.Git(ctx, "fetch", "-q", "origin", "+refs/heads/"+branch+":refs/remotes/origin/"+branch)
}

func exists(ctx context.Context, repo gitx.Repo, ref string) bool {
	_, err := repo.Git(ctx, "rev-parse", "--verify", "--quiet", ref)
	return err == nil
}

func untar(r io.Reader, dir string) error {
	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dir, filepath.FromSlash(h.Name))
		if !strings.HasPrefix(target, filepath.Clean(dir)+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe path in results: %s", h.Name)
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
	}
}
