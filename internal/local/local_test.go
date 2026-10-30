package local

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blu9a/shipwake/internal/gitx"
)

func repo(t *testing.T, dir string) gitx.Repo {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	r := gitx.Repo{Dir: dir}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"}, {"config", "user.name", "Ana"}, {"config", "user.email", "ana@example.com"},
		{"commit", "-q", "--allow-empty", "-m", "start"},
	} {
		if _, err := r.Git(context.Background(), args...); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

func TestOpenAndFollow(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	t.Setenv("SHIPWAKE_HOME", filepath.Join(root, "home"))
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	code := repo(t, filepath.Join(root, "bills"))
	before, _ := code.Git(ctx, "for-each-ref")

	e, err := Open(ctx, code.Dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	if !e.Local() || e.Workspace != filepath.Join(root, "home", "bills") {
		t.Fatalf("entry %+v", e)
	}
	ws := gitx.Repo{Dir: e.Workspace}
	if who, _ := ws.Git(ctx, "log", "-1", "--format=%an"); strings.TrimSpace(who) != "Ana" {
		t.Errorf("the workspace's first commit is by %q, want the repository's person", who)
	}
	if after, _ := code.Git(ctx, "for-each-ref"); after != before {
		t.Error("opening changed the repository's refs")
	}
	again, err := Open(ctx, code.Dir, "main")
	if err != nil || again != e {
		t.Errorf("opening twice: %+v, %v", again, err)
	}
	l, _ := Load()
	if len(l.Projects) != 1 {
		t.Errorf("projects %+v", l.Projects)
	}

	// The workspace follows the repository's branch, starting new ones from its trunk.
	if _, err := code.Git(ctx, "switch", "-q", "-c", "feature/tip"); err != nil {
		t.Fatal(err)
	}
	if err := Follow(ctx, code, ws, "main"); err != nil {
		t.Fatal(err)
	}
	if b, _ := ws.Git(ctx, "branch", "--show-current"); strings.TrimSpace(b) != "feature/tip" {
		t.Errorf("workspace on %q", b)
	}
	// A workspace with changes nobody committed isn't switched under them.
	if err := os.WriteFile(filepath.Join(e.Workspace, "notes.txt"), []byte("draft"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := code.Git(ctx, "switch", "-q", "main"); err != nil {
		t.Fatal(err)
	}
	if err := Follow(ctx, code, ws, "main"); err == nil || !strings.Contains(err.Error(), "aren't committed") {
		t.Errorf("dirty workspace: %v", err)
	}
}
