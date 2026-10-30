// Package store keeps normalized runs on disk: one JSON file per run and its attachments.
//
//	<dir>/runs/<id>.json
//	<dir>/files/<id>/<name>
//
// The directory can live anywhere: an orphan branch's work tree, a CI artifact, a separate repository.
// Nothing is written into the project's own source.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/blu9a/shipwake/internal/model"
)

// Store is a results directory.
type Store struct{ Dir string }

// NewID returns a sortable, unique run ID: time first, then randomness.
func NewID(t time.Time) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return t.UTC().Format("20060102T150405") + "-" + hex.EncodeToString(b)
}

// Save writes a run and copies its attachments into the store. Attachment paths are rewritten
// to be relative to the run's files; srcDir resolves relative source paths.
func (s Store) Save(run *model.Run, srcDir string) error {
	files := filepath.Join(s.Dir, "files", run.ID)
	used := map[string]int{}
	copyOne := func(a *model.Attachment) error {
		src := a.Path
		if !filepath.IsAbs(src) {
			src = filepath.Join(srcDir, src)
		}
		base := filepath.Base(src)
		if n := used[base]; n > 0 { // keep names unique within a run
			base = fmt.Sprintf("%d-%s", n, base)
		}
		used[filepath.Base(src)]++
		size, err := copyFile(src, filepath.Join(files, base))
		if err != nil {
			return fmt.Errorf("attachment %s: %w", a.Name, err)
		}
		a.Path, a.Size = base, size
		return nil
	}
	var walkSteps func([]model.Step) error
	walkSteps = func(steps []model.Step) error {
		for i := range steps {
			for j := range steps[i].Attachments {
				if err := copyOne(&steps[i].Attachments[j]); err != nil {
					return err
				}
			}
			if err := walkSteps(steps[i].Steps); err != nil {
				return err
			}
		}
		return nil
	}
	for i := range run.Tests {
		for j := range run.Tests[i].Attachments {
			if err := copyOne(&run.Tests[i].Attachments[j]); err != nil {
				return err
			}
		}
		if err := walkSteps(run.Tests[i].Steps); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Join(s.Dir, "runs"), 0o755); err != nil {
		return err
	}
	run.Schema = model.RunSchema
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.Dir, "runs", run.ID+".json"), data, 0o644)
}

// List reads every run, oldest first.
func (s Store) List() ([]model.Run, error) {
	entries, err := os.ReadDir(filepath.Join(s.Dir, "runs"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var runs []model.Run
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.Dir, "runs", e.Name()))
		if err != nil {
			return nil, err
		}
		var r model.Run
		if err := json.Unmarshal(data, &r); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if err := r.CheckSchema(); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		runs = append(runs, r)
	}
	slices.SortFunc(runs, func(a, b model.Run) int { return a.Start.Compare(b.Start) })
	return runs, nil
}

// FilesDir is where a run's attachments are.
func (s Store) FilesDir(id string) string { return filepath.Join(s.Dir, "files", id) }

func copyFile(src, dst string) (int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return 0, err
	}
	out, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return n, err
}
