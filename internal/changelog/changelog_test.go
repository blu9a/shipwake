package changelog

import (
	"strings"
	"testing"
)

const keepAChangelog = `# Changelog

All notable changes to this project are documented here.

## [Unreleased]

### Added
- Something not released yet.

## [1.1.0] - 2026-09-01

### Fixed
- A bug.

## [1.0.0] - 2026-08-01

### Added
- The first version.

[1.1.0]: https://example.com/compare/v1.0.0...v1.1.0
`

func TestParse(t *testing.T) {
	cases := map[string][]string{
		keepAChangelog: {"1.1.0", "1.0.0"},
		"# Changelog\n\n## [1.2.0](https://x/compare/v1.1.0...v1.2.0) (2026-10-30)\n\n### Features\n\n* a\n": {"1.2.0"},
		"# pkg\n\n## 2.0.0\n\n### Major Changes\n\n- b\n\n## 1.0.0-rc.1\n\n- c\n":                            {"2.0.0", "1.0.0-rc.1"},
		"# Changelog\n\n## v0.1.0 (2026-10-30)\n\nSplit a bill.\n":                                           {"0.1.0"},
	}
	for text, want := range cases {
		got := Parse(text)
		if len(got) != len(want) {
			t.Fatalf("%d sections, want %v in:\n%s", len(got), want, text)
		}
		for i := range want {
			if got[i].Version != want[i] {
				t.Errorf("section %d is %q, want %q", i, got[i].Version, want[i])
			}
		}
	}
	if body, ok := Find(keepAChangelog, "v1.1.0"); !ok || body != "### Fixed\n- A bug." {
		t.Errorf("Find = %q, %v", body, ok)
	}
}

func TestInsert(t *testing.T) {
	got := Insert("", "v0.1.0", "2026-10-30", "Split a bill.\n\n### Features\n\n- split (7d5611a)")
	want := "# Changelog\n\n## v0.1.0 (2026-10-30)\n\nSplit a bill.\n\n### Features\n\n- split (7d5611a)\n"
	if got != want {
		t.Errorf("new file:\n%q\nwant\n%q", got, want)
	}
	next := Insert(keepAChangelog, "v1.2.0", "2026-10-30", "### Added\n- Something not released yet.")
	if s := Parse(next); len(s) != 3 || s[0].Version != "1.2.0" || s[1].Version != "1.1.0" {
		t.Fatalf("sections after insert: %+v", s)
	}
	if body, _ := Find(next, "1.2.0"); body != "### Added\n- Something not released yet." {
		t.Errorf("new section %q", body)
	}
	for _, kept := range []string{"All notable changes", "## [Unreleased]\n\n## v1.2.0", "[1.1.0]: https://example.com"} {
		if !contains(next, kept) {
			t.Errorf("lost %q:\n%s", kept, next)
		}
	}
	if contains(next, "## [Unreleased]\n\n### Added") {
		t.Errorf("Unreleased not emptied:\n%s", next)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
