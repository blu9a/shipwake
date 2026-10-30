package highlight

import (
	"strings"
	"testing"
)

func TestLines(t *testing.T) {
	src := "package main\n\n// Hello says hi.\nfunc Hello() string {\n\treturn `two\nlines` // é\n}\n"
	got := Lines("hello.go", src)
	want := []string{
		"k7_5", // package main
		"",
		"c17",          // the comment
		"k4_1f5_3t6_2", // func Hello() string {
		"_1k6_1s4",     // the raw string starts here…
		"s6_1c4",       // …and ends here: still a string, then a comment with é (one UTF-16 unit)
		"_1",
	}
	if len(got) != len(want) {
		t.Fatalf("lines = %q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i+1, got[i], want[i])
		}
	}
	if Lines("notes.unknownext", "x") != nil {
		t.Error("unknown language should not be highlighted")
	}
}

func TestBlob(t *testing.T) {
	Forget()
	src := "package main\n"
	if got := Blob("abc", "a/main.go", src, 0); len(got) != 1 || got[0] != "k7_5" {
		t.Fatalf("Go = %q", got)
	}
	// The same object under another name is read in that name's language.
	if got := Blob("abc", "notes.unknownext", src, 0); got != nil {
		t.Errorf("unknown language = %q", got)
	}
	if got := Blob("abc", "b/main.go", src, 0); len(got) != 1 || got[0] != "k7_5" {
		t.Errorf("Go again = %q", got)
	}
}

func TestBlobUpTo(t *testing.T) {
	Forget()
	src := "package main\n\nvar s = `a\nb`\n" + strings.Repeat("// more\n", 500)
	whole := Lines("main.go", src)
	got := Blob("long", "main.go", src, 3)
	if len(got) >= len(whole) {
		t.Fatalf("read %d lines of %d for line 3", len(got), len(whole))
	}
	for i := range 4 {
		if got[i] != whole[i] {
			t.Errorf("line %d = %q, want %q as in the whole file", i+1, got[i], whole[i])
		}
	}
	// A later diff further down reads further.
	if got := Blob("long", "main.go", src, 450); len(got) < 450 || got[449] != whole[449] {
		t.Errorf("line 450 = %q", got[449])
	}
	if Blob("lock", "pnpm-lock.yaml", "lockfileVersion: '9.0'\n", 0) != nil {
		t.Error("a lockfile was colored")
	}
}
