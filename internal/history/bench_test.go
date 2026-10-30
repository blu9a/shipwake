package history

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/blu9a/shipwake/internal/gitx"
)

// bigRepo builds a trunk of merges, each bringing a branch of two commits, with git fast-import.
func bigRepo(b testing.TB, merges int) gitx.Repo {
	b.Helper()
	dir := b.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		b.Fatalf("git init: %v\n%s", err, out)
	}
	var s strings.Builder
	mark := 0
	commit := func(ref, msg string, from, merge int) int {
		mark++
		fmt.Fprintf(&s, "commit %s\nmark :%d\ncommitter t <t@example.invalid> %d +0000\ndata %d\n%s\n", ref, mark, 1767225600+mark*60, len(msg), msg)
		if from > 0 {
			fmt.Fprintf(&s, "from :%d\n", from)
		}
		if merge > 0 {
			fmt.Fprintf(&s, "merge :%d\n", merge)
		}
		file := fmt.Sprintf("src/file%d.go", mark%50)
		fmt.Fprintf(&s, "M 644 inline %s\ndata %d\n%s\n\n", file, len(msg), msg)
		return mark
	}
	tip := commit("refs/heads/main", "chore: start", 0, 0)
	for i := range merges {
		a := commit("refs/heads/work", fmt.Sprintf("feat(part%d): first step", i), tip, 0)
		c := commit("refs/heads/work", fmt.Sprintf("feat(part%d): second step\n\nRelease-note: Part %d works.", i, i), a, 0)
		tip = commit("refs/heads/main", fmt.Sprintf("Merge feat/part%d: part %d", i, i), tip, c)
	}
	s.WriteString("reset refs/heads/work\nfrom 0000000000000000000000000000000000000000\n\n")
	cmd := exec.Command("git", "-C", dir, "fast-import", "--quiet")
	cmd.Stdin = strings.NewReader(s.String())
	if out, err := cmd.CombinedOutput(); err != nil {
		b.Fatalf("git fast-import: %v\n%s", err, out)
	}
	return gitx.Repo{Dir: dir}
}

// BenchmarkHistoryLoad reads a 300-merge trunk (900 commits), as a build does for the timeline.
func BenchmarkHistoryLoad(b *testing.B) {
	ctx := context.Background()
	r := bigRepo(b, 300)
	for b.Loop() {
		h, err := Load(ctx, r, "main", 300)
		if err != nil {
			b.Fatal(err)
		}
		if len(h.Rows) != 300 {
			b.Fatalf("rows = %d", len(h.Rows))
		}
	}
}
