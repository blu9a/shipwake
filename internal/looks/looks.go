// Package looks is visual review: screenshots recorded with a run, each compared pixel by pixel with the
// look a person approved for it.
//
// A screenshot takes part when its attachment is named "look: <subject>" or "look: <subject> [<variant>]",
// such as "look: Button [dark, phone]". The comparison is deterministic: no model, a fixed threshold, and
// anti-aliased pixels tolerated (see Compare).
package looks

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"

	"github.com/blu9a/shipwake/internal/model"
)

// Prefix starts the name of a screenshot that takes part in visual review.
const Prefix = "look:"

var nameRe = regexp.MustCompile(`^look:\s*(.*?)(?:\s*\[([^\[\]]*)\])?\s*$`)

// Parse reads a screenshot's name: its subject and variant. ok is false for any other attachment.
func Parse(name string) (subject, variant string, ok bool) {
	m := nameRe.FindStringSubmatch(name)
	if m == nil {
		return "", "", false
	}
	subject = strings.Join(strings.Fields(m[1]), " ")
	if subject == "" {
		return "", "", false
	}
	var parts []string
	for _, p := range strings.Split(m[2], ",") {
		if p = strings.Join(strings.Fields(p), " "); p != "" {
			parts = append(parts, p)
		}
	}
	return subject, strings.Join(parts, ", "), true
}

// Name is how a look is named in checks and commands: the subject, and its variant in brackets.
func Name(subject, variant string) string {
	if variant == "" {
		return subject
	}
	return subject + " [" + variant + "]"
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// ID is a look's key in addresses: readable, and distinct for distinct subjects and variants.
func ID(subject, variant string) string {
	sum := sha256.Sum256([]byte(subject + "\x00" + variant))
	slug := strings.Trim(slugRe.ReplaceAllString(strings.ToLower(Name(subject, variant)), "-"), "-")
	if len(slug) > 60 {
		slug = strings.Trim(slug[:60], "-")
	}
	if slug != "" {
		slug += "-"
	}
	return slug + hex.EncodeToString(sum[:3])
}

// Shot is a screenshot found in a run.
type Shot struct {
	Subject, Variant string
	Run              string
	Path             string // in the run's files
	Type             string
}

// decodable tells the media types a look can be: what Go decodes without help.
func decodable(t string) bool {
	return t == "image/png" || t == "image/jpeg" || t == "image/gif"
}

// Collect finds the screenshots in runs, in the order they were recorded. When a subject and variant
// appears more than once, the first run's last one counts: a test that took it again took the newer look.
func Collect(runs []*model.Run) []Shot {
	var res []Shot
	at := map[string]int{}
	add := func(r *model.Run, a model.Attachment) {
		subject, variant, ok := Parse(a.Name)
		if !ok || !decodable(a.Type) {
			return
		}
		key := subject + "\x00" + variant
		s := Shot{Subject: subject, Variant: variant, Run: r.ID, Path: a.Path, Type: a.Type}
		if i, seen := at[key]; seen {
			if res[i].Run == r.ID {
				res[i] = s
			}
			return
		}
		at[key] = len(res)
		res = append(res, s)
	}
	for _, r := range runs {
		var steps func([]model.Step)
		steps = func(list []model.Step) {
			for _, st := range list {
				for _, a := range st.Attachments {
					add(r, a)
				}
				steps(st.Steps)
			}
		}
		for _, t := range r.Tests {
			steps(t.Steps)
			for _, a := range t.Attachments {
				add(r, a)
			}
		}
	}
	return res
}

// Baseline is the approval a commit's look is compared with: of those for its subject and variant made
// on the commit or one before it (applies says which), the newest.
func Baseline(approvals []model.LookApproval, subject, variant string, applies func(model.LookApproval) bool) *model.LookApproval {
	var best *model.LookApproval
	for i := range approvals {
		a := &approvals[i]
		if a.Subject != subject || a.Variant != variant || !applies(*a) {
			continue
		}
		if best == nil || a.At.After(best.At) || a.At.Equal(best.At) && a.ID > best.ID {
			best = a
		}
	}
	return best
}
