package store

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/blu9a/shipwake/internal/gitx"
	"github.com/blu9a/shipwake/internal/model"
)

// TestBranchInCI follows CI: shallow clones that know nothing of the results branch record runs
// and push, one of them after the other already did, and a fresh clone reads both.
func TestBranchInCI(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@x", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@x")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	origin := filepath.Join(root, "origin.git")
	git(root, "init", "-q", "--bare", origin)
	seed := filepath.Join(root, "seed")
	git(root, "clone", "-q", origin, seed)
	git(seed, "commit", "-q", "--allow-empty", "-m", "first")
	git(seed, "push", "-q", "origin", "HEAD:main")

	record := func(name string) {
		t.Helper()
		dir := filepath.Join(root, name)
		git(root, "clone", "-q", "--depth", "1", "--branch", "main", "file://"+origin, dir)
		repo := gitx.Repo{Dir: dir}
		wt, done, err := Checkout(ctx, repo, "shipwake-results")
		if err != nil {
			t.Fatal(err)
		}
		run := model.Run{Kind: "test", Name: name, Start: time.Now(), Tests: []model.Test{{Name: "a", Status: "passed"}}}
		run.ID = NewID(run.Start)
		if err := (Store{Dir: wt}).Save(&run, dir); err != nil {
			t.Fatal(err)
		}
		if err := done("run "+name, true); err != nil {
			t.Fatal(err)
		}
	}
	record("one")
	record("two")

	reader := filepath.Join(root, "reader")
	git(root, "clone", "-q", "--depth", "1", "--branch", "main", "file://"+origin, reader)
	out := t.TempDir()
	found, err := Export(ctx, gitx.Repo{Dir: reader}, "shipwake-results", out)
	if err != nil || !found {
		t.Fatalf("export: found=%v err=%v", found, err)
	}
	runs, err := (Store{Dir: out}).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("runs = %d, want 2", len(runs))
	}
}
