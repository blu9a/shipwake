package main

// Measured requirements in the workspace. A metric proposal is a
// .metric file tagged @proposed. The page edits its sentences and approves it by removing the tag,
// each a commit of the person's. While the person types, the page asks how Shipwake reads each line.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blu9a/shipwake/internal/gitx"
	"github.com/blu9a/shipwake/internal/metricspec"
)

// metricPath checks that a path names a .metric file inside the repository.
func metricPath(dir, path string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(path))
	if !strings.HasSuffix(clean, ".metric") || filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("%q is not a .metric file in the repository", path)
	}
	if _, err := os.Stat(filepath.Join(dir, clean)); err != nil {
		return "", err
	}
	return filepath.ToSlash(clean), nil
}

// withoutProposed is the file as the person edits it: the @proposed tag is the state, not the text.
func withoutProposed(text string) string {
	var out []string
	for _, l := range strings.Split(text, "\n") {
		if t := strings.TrimSpace(l); strings.HasPrefix(t, "@") {
			kept := strings.Join(strings.FieldsFunc(t, func(r rune) bool { return r == ' ' || r == '\t' }), " ")
			kept = strings.TrimSpace(strings.ReplaceAll(" "+kept+" ", " @proposed ", " "))
			if kept == "" {
				continue
			}
			l = kept
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

func (w *workspace) metric(repo gitx.Repo, path string) (any, error) {
	path, err := metricPath(repo.Dir, path)
	if err != nil {
		return nil, err
	}
	text, err := w.committed(repo, path)
	if err != nil {
		return nil, err
	}
	s := metricspec.Parse(path, text)
	return map[string]any{"text": strings.TrimRight(withoutProposed(text), "\n"), "proposed": s.Proposed()}, nil
}

// checkMetric reads text as Shipwake would, for the editor: each line's meaning or its error.
func checkMetric(path, text string) (any, error) {
	return metricspec.Parse(path, text), nil
}

func (w *workspace) saveMetric(repo gitx.Repo, path, text string) (any, error) {
	path, err := metricPath(repo.Dir, path)
	if err != nil {
		return nil, err
	}
	if err := w.unsaved(repo, path); err != nil {
		return nil, err
	}
	old, err := w.committed(repo, path)
	if err != nil {
		return nil, err
	}
	s := metricspec.Parse(path, text)
	if err := s.Err(); err != nil {
		return nil, err
	}
	if s.Proposed() {
		return nil, fmt.Errorf("leave out @proposed: approving decides it")
	}
	if metricspec.Parse(path, old).Proposed() {
		text = "@proposed\n" + text // still a proposal until it is approved
	}
	text = strings.TrimRight(text, "\n") + "\n"
	if text == old {
		return map[string]string{"commit": ""}, nil
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, path), []byte(text), 0o644); err != nil {
		return nil, err
	}
	return w.commit(repo, path, fmt.Sprintf("spec: change the metric %q", s.Name))
}

func (w *workspace) approveMetric(repo gitx.Repo, path string) (any, error) {
	path, err := metricPath(repo.Dir, path)
	if err != nil {
		return nil, err
	}
	if err := w.unsaved(repo, path); err != nil {
		return nil, err
	}
	old, err := w.committed(repo, path)
	if err != nil {
		return nil, err
	}
	s := metricspec.Parse(path, old)
	if !s.Proposed() {
		return nil, fmt.Errorf("%s is not proposed", s.Name)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	text := strings.TrimRight(withoutProposed(old), "\n") + "\n"
	if err := os.WriteFile(filepath.Join(repo.Dir, path), []byte(text), 0o644); err != nil {
		return nil, err
	}
	user, email := w.person(repo)
	return w.commit(repo, path, fmt.Sprintf("spec: approve the metric %q\n\nApproved-by: %s <%s>", s.Name, strings.TrimSpace(user), strings.TrimSpace(email)))
}
