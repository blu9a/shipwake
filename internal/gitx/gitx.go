// Package gitx runs the git command. Shipwake calls git rather than a Go
// implementation so it behaves correctly on every real repository: worktrees,
// partial clones, large histories.
package gitx

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Repo is a Git working tree or bare repository.
type Repo struct{ Dir string }

// Git runs git with args in the repository and returns its standard output.
func (r Repo) Git(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", r.Dir, "-c", "core.quotepath=off"}, args...)...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if i := strings.Index(msg, "\n"); i > 0 { // the first line says what went wrong; usage text follows
			msg = msg[:i]
		}
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
	}
	return out.String(), nil
}

// Lines runs git and splits its output into lines.
func (r Repo) Lines(ctx context.Context, args ...string) ([]string, error) {
	out, err := r.Git(ctx, args...)
	if err != nil {
		return nil, err
	}
	out = strings.TrimRight(out, "\n")
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// Resolve returns the full hash of a revision.
func (r Repo) Resolve(ctx context.Context, rev string) (string, error) {
	out, err := r.Git(ctx, "rev-parse", "--verify", rev+"^{commit}")
	return strings.TrimSpace(out), err
}

// EmptyTree is the hash of the empty tree, the parent of a root commit in diffs.
const EmptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
