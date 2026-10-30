package highlight

import "testing"

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
	if got := Blob("abc", "a/main.go", src); len(got) != 1 || got[0] != "k7_5" {
		t.Fatalf("Go = %q", got)
	}
	// The same object under another name is read in that name's language.
	if got := Blob("abc", "notes.unknownext", src); got != nil {
		t.Errorf("unknown language = %q", got)
	}
	if got := Blob("abc", "b/main.go", src); len(got) != 1 || got[0] != "k7_5" {
		t.Errorf("Go again = %q", got)
	}
}
