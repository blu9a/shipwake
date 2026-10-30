// Package changelog reads and writes a project's changelog file: one "## <version>" section per
// release, newest first, whichever common way its headings are written.
//
//	# Changelog
//
//	## [Unreleased]
//
//	## [1.2.0] - 2026-10-30        with a date (keepachangelog.com)
//	## [1.2.0](https://…) (2026-10-30)   with a compare link
//	## 1.2.0                       bare
//	## v1.2.0 (2026-10-30)         what Shipwake writes
package changelog

import (
	"regexp"
	"strings"
)

// Section is one release in the file.
type Section struct {
	Version string // as in the heading, without brackets or a leading v
	Body    string // everything under the heading, trimmed
}

var (
	versionRe    = regexp.MustCompile(`^##\s+\[?v?(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)\]?`)
	unreleasedRe = regexp.MustCompile(`(?i)^##\s+\[?unreleased\]?\s*$`)
	headingRe    = regexp.MustCompile(`^##\s`)
)

// Parse reads the release sections of a changelog.
func Parse(text string) []Section {
	var out []Section
	var cur *Section
	var body []string
	flush := func() {
		if cur != nil {
			cur.Body = strings.TrimSpace(strings.Join(body, "\n"))
			out = append(out, *cur)
		}
		cur, body = nil, nil
	}
	for _, l := range strings.Split(text, "\n") {
		if headingRe.MatchString(l) {
			flush()
			if m := versionRe.FindStringSubmatch(l); m != nil {
				cur = &Section{Version: m[1]}
			}
			continue
		}
		if cur != nil {
			body = append(body, l)
		}
	}
	flush()
	return out
}

// Find is the section of a version ("v1.2.0" or "1.2.0"), if the file has one.
func Find(text, version string) (string, bool) {
	v := strings.TrimPrefix(version, "v")
	for _, s := range Parse(text) {
		if s.Version == v {
			return s.Body, true
		}
	}
	return "", false
}

// Insert adds a release above the others. An Unreleased section, if any, stays on top and is emptied:
// what it held is now the release, as the person approved it. A missing file starts with a title.
func Insert(text, version, date, body string) string {
	section := "## " + version
	if date != "" {
		section += " (" + date + ")"
	}
	section += "\n\n" + strings.TrimSpace(body) + "\n"
	if strings.TrimSpace(text) == "" {
		return "# Changelog\n\n" + section
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	// Where the first section starts; an Unreleased section is kept as an empty heading above.
	at := len(lines)
	for i, l := range lines {
		if headingRe.MatchString(l) {
			at = i
			break
		}
	}
	head := strings.TrimRight(strings.Join(lines[:at], "\n"), "\n")
	rest := lines[at:]
	keep := ""
	if len(rest) > 0 && unreleasedRe.MatchString(rest[0]) {
		keep = rest[0] + "\n\n"
		end := 1
		for end < len(rest) && !headingRe.MatchString(rest[end]) {
			end++
		}
		rest = rest[end:]
	}
	out := head + "\n\n" + keep + section
	if len(rest) > 0 {
		out += "\n" + strings.Join(rest, "\n") + "\n"
	}
	return out
}
