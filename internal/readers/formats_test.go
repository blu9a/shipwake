package readers

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the expected runs of the built-in formats from their samples")

// TestBuiltinFormats converts each built-in converter's real sample report and compares the run with
// the reviewed one. After a deliberate change, run with -update and review the diff.
func TestBuiltinFormats(t *testing.T) {
	n := 0
	for _, r := range all {
		if r.Source != "built-in" {
			continue
		}
		n++
		read := func(name string) ([]byte, error) { return os.ReadFile(filepath.Join("formats", name)) }
		write := func(name string, data []byte) error { return os.WriteFile(filepath.Join("formats", name), data, 0o644) }
		if err := r.Spec.Check(read, write, *update); err != nil {
			t.Errorf("%s: %v", r.Format, err)
		}
	}
	if n == 0 {
		t.Fatal("no built-in formats")
	}
}
