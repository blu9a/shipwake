package history

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"

	"github.com/blu9a/shipwake/internal/gitx"
	"github.com/blu9a/shipwake/internal/highlight"
	"github.com/blu9a/shipwake/internal/model"
)

// highlightDiffs colors each shown diff line from its file: added and context lines from the new
// version, removed lines from the old one. Files whose language isn't known stay plain.
func highlightDiffs(ctx context.Context, r gitx.Repo, base, sha string, files []model.FileChange, diffs map[string]model.FileDiff) {
	var specs []string
	for _, f := range files {
		if len(diffs[f.Path].Lines) == 0 {
			continue
		}
		if f.Status != "D" {
			specs = append(specs, sha+":"+f.Path)
		}
		if f.Status != "A" && base != gitx.EmptyTree {
			specs = append(specs, base+":"+cmp(f.Old, f.Path))
		}
	}
	if len(specs) == 0 {
		return
	}
	blobs, err := catFiles(ctx, r, specs)
	if err != nil {
		return // plain diffs are still correct
	}
	for _, f := range files {
		d := diffs[f.Path]
		if len(d.Lines) == 0 {
			continue
		}
		newer := highlight.Lines(f.Path, blobs[sha+":"+f.Path])
		older := highlight.Lines(cmp(f.Old, f.Path), blobs[base+":"+cmp(f.Old, f.Path)])
		if newer == nil && older == nil {
			continue
		}
		d.HL = mapLines(d.Lines, older, newer)
		diffs[f.Path] = d
	}
}

func cmp(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// mapLines walks a patch as the page does, numbering old and new lines from each hunk header.
func mapLines(lines, older, newer []string) []string {
	at := func(hl []string, n int) string {
		if n >= 1 && n <= len(hl) {
			return hl[n-1]
		}
		return ""
	}
	out := make([]string, len(lines))
	o, n := 0, 0
	for i, l := range lines {
		switch {
		case strings.HasPrefix(l, "@@"):
			o, n = hunkStart(l)
		case strings.HasPrefix(l, "\\"):
		case strings.HasPrefix(l, "+"):
			out[i] = at(newer, n)
			n++
		case strings.HasPrefix(l, "-"):
			out[i] = at(older, o)
			o++
		default:
			out[i] = at(newer, n)
			o++
			n++
		}
	}
	return out
}

// hunkStart reads "@@ -o,l +n,l @@".
func hunkStart(h string) (o, n int) {
	f := strings.Fields(h)
	num := func(s string) int {
		s = strings.TrimLeft(s, "-+")
		s, _, _ = strings.Cut(s, ",")
		v, _ := strconv.Atoi(s)
		return v
	}
	if len(f) >= 3 {
		o, n = num(f[1]), num(f[2])
	}
	return o, n
}

// catFiles reads blobs by "rev:path" in one git process. Missing ones are left out.
func catFiles(ctx context.Context, r gitx.Repo, specs []string) (map[string]string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", r.Dir, "cat-file", "--batch")
	cmd.Stdin = strings.NewReader(strings.Join(specs, "\n") + "\n")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git cat-file: %w", err)
	}
	res := make(map[string]string, len(specs))
	rd := bufio.NewReader(&out)
	for _, spec := range specs {
		header, err := rd.ReadString('\n')
		if err != nil {
			return res, nil
		}
		f := strings.Fields(header)
		if len(f) != 3 || f[1] != "blob" {
			continue // "<spec> missing", or not a file
		}
		size, err := strconv.Atoi(f[2])
		if err != nil {
			return res, nil
		}
		body := make([]byte, size+1) // content and its trailing newline
		if _, err := io.ReadFull(rd, body); err != nil {
			return res, nil
		}
		if size <= highlight.MaxBytes {
			res[spec] = string(body[:size])
		}
	}
	return res, nil
}
