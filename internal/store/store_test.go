package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blu9a/shipwake/internal/model"
)

func TestRunSchema(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	run := model.Run{ID: "20261030T000000-new", Name: "Go tests", Start: time.Unix(2, 0)}
	if err := s.Save(&run, ""); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(s.Dir, "runs", run.ID+".json"))
	if !strings.Contains(string(data), `"schemaVersion": 1`) {
		t.Errorf("a saved run names its format:\n%s", data)
	}

	// A run recorded before formats had versions is format 1.
	old := `{"id": "20261029T000000-old", "name": "Go tests", "start": "1970-01-01T00:00:01Z"}`
	if err := os.WriteFile(filepath.Join(s.Dir, "runs", "20261029T000000-old.json"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	runs, err := s.List()
	if err != nil || len(runs) != 2 {
		t.Fatalf("runs = %d, %v", len(runs), err)
	}

	// One from a newer Shipwake is refused by name rather than misread.
	newer := `{"schemaVersion": 2, "id": "20261031T000000-next", "name": "Go tests"}`
	if err := os.WriteFile(filepath.Join(s.Dir, "runs", "20261031T000000-next.json"), []byte(newer), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.List(); err == nil || !strings.Contains(err.Error(), "20261031T000000-next.json") || !strings.Contains(err.Error(), "upgrade shipwake") {
		t.Errorf("newer run: %v", err)
	}
}
