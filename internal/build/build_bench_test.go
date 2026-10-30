package build

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// fixture is a real repository at a pinned commit, from tools/fixtures.sh. Without it the benchmark
// is skipped: it needs the network once to fetch it.
func fixture(b *testing.B, name string) string {
	b.Helper()
	dir := os.Getenv("SHIPWAKE_FIXTURES")
	if dir == "" {
		dir = filepath.Join("..", "..", ".cache", "fixtures")
	}
	d := filepath.Join(dir, name)
	if _, err := os.Stat(filepath.Join(d, ".git")); err != nil {
		b.Skipf("no fixture %s: run tools/fixtures.sh", name)
	}
	return d
}

// BenchmarkBuildRealRepository builds the site of charmbracelet/log as a project would: its whole
// history up to the default 300 rows, every commit's files and diffs, its releases. This is the time
// someone waits for shipwake build or serve.
func BenchmarkBuildRealRepository(b *testing.B) {
	ctx := context.Background()
	repo := fixture(b, "charmbracelet-log")
	for b.Loop() {
		if err := Run(ctx, Options{Projects: []Project{{Dir: repo}}, Out: b.TempDir()}); err != nil {
			b.Fatal(err)
		}
	}
}
