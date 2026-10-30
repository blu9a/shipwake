package main

// The workspace: shipwake serve as the person's workspace in the requirements loop.
// The served site gains a small local API: read a feature file, save the person's changes as their
// commit, approve proposed scenarios with an Approved-by trailer, and tag an approved release. A
// published site has no API and stays read-only.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/blu9a/shipwake/internal/build"
	"github.com/blu9a/shipwake/internal/changelog"
	"github.com/blu9a/shipwake/internal/gherkin"
	"github.com/blu9a/shipwake/internal/gitx"
	"github.com/blu9a/shipwake/internal/model"
)

type workspace struct {
	mu      sync.Mutex
	ctx     context.Context
	opts    build.Options
	repos   map[string]string // project ID → repository directory
	files   http.Handler
	rebuild func() error
}

func newWorkspace(ctx context.Context, o build.Options) (*workspace, error) {
	w := &workspace{ctx: ctx, opts: o, files: http.FileServer(http.Dir(o.Out))}
	w.rebuild = func() error { return build.Run(ctx, w.opts) }
	if err := w.rebuild(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(o.Out, "data", "site.json"))
	if err != nil {
		return nil, err
	}
	var site model.Site
	if err := json.Unmarshal(data, &site); err != nil {
		return nil, err
	}
	w.repos = map[string]string{}
	for i, ref := range site.Projects {
		dir, err := filepath.Abs(o.Projects[i].Dir)
		if err != nil {
			return nil, err
		}
		w.repos[ref.ID] = dir
	}
	return w, nil
}

func (w *workspace) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		w.files.ServeHTTP(rw, r)
		return
	}
	// Only the page itself may write: same origin, JSON bodies.
	if r.Method == http.MethodPost {
		if o := r.Header.Get("Origin"); o != "" && o != "http://"+r.Host {
			http.Error(rw, "cross-origin", http.StatusForbidden)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(rw, "want JSON", http.StatusUnsupportedMediaType)
			return
		}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	reply := func(v any, err error) {
		rw.Header().Set("Content-Type", "application/json")
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(rw).Encode(map[string]string{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(rw).Encode(v)
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/"), "/"), "/")
	if parts[0] == "session" {
		user := ""
		checkedOut := map[string]string{} // project → the branch its requirements are edited on
		for id, dir := range w.repos {
			repo := gitx.Repo{Dir: dir}
			if out, err := repo.Git(w.ctx, "config", "user.name"); err == nil && user == "" {
				user = strings.TrimSpace(out)
			}
			checkedOut[id] = w.checkedOut(repo)
		}
		reply(map[string]any{"edit": true, "user": user, "checkedOut": checkedOut}, nil)
		return
	}
	if len(parts) != 2 {
		http.NotFound(rw, r)
		return
	}
	dir, ok := w.repos[parts[0]]
	if !ok {
		http.NotFound(rw, r)
		return
	}
	repo := gitx.Repo{Dir: dir}
	switch {
	case parts[1] == "feature" && r.Method == http.MethodGet:
		path, err := featurePath(dir, r.URL.Query().Get("path"))
		if err != nil {
			reply(nil, err)
			return
		}
		text, err := w.committed(repo, path)
		reply(map[string]string{"text": text}, err)
	case parts[1] == "feature" && r.Method == http.MethodPost:
		var req struct{ Path, Text, Branch string }
		if err := w.decode(r, repo, &req, &req.Branch); err != nil {
			reply(nil, err)
			return
		}
		reply(w.save(repo, req.Path, req.Text))
	case parts[1] == "proposal" && r.Method == http.MethodGet:
		path, err := featurePath(dir, r.URL.Query().Get("path"))
		if err != nil {
			reply(nil, err)
			return
		}
		text, err := w.committed(repo, path)
		if err != nil {
			reply(nil, err)
			return
		}
		reply(map[string]string{"text": proposalOf(text)}, nil)
	case parts[1] == "proposal" && r.Method == http.MethodPost:
		var req struct{ Path, Text, Branch string }
		if err := w.decode(r, repo, &req, &req.Branch); err != nil {
			reply(nil, err)
			return
		}
		reply(w.saveProposal(repo, req.Path, req.Text))
	case parts[1] == "approve" && r.Method == http.MethodPost:
		var req struct {
			Path, Branch string
			Scenarios    []string
		}
		if err := w.decode(r, repo, &req, &req.Branch); err != nil {
			reply(nil, err)
			return
		}
		reply(w.approve(repo, req.Path, req.Scenarios))
	case parts[1] == "metric" && r.Method == http.MethodGet:
		reply(w.metric(repo, r.URL.Query().Get("path")))
	case parts[1] == "metric" && r.Method == http.MethodPost:
		var req struct{ Path, Text, Branch string }
		if err := w.decode(r, repo, &req, &req.Branch); err != nil {
			reply(nil, err)
			return
		}
		reply(w.saveMetric(repo, req.Path, req.Text))
	case parts[1] == "check-metric" && r.Method == http.MethodPost:
		var req struct{ Path, Text string }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			reply(nil, err)
			return
		}
		reply(checkMetric(req.Path, req.Text))
	case parts[1] == "approve-metric" && r.Method == http.MethodPost:
		var req struct{ Path, Branch string }
		if err := w.decode(r, repo, &req, &req.Branch); err != nil {
			reply(nil, err)
			return
		}
		reply(w.approveMetric(repo, req.Path))
	case parts[1] == "release" && r.Method == http.MethodPost:
		var req struct {
			Version, Commit, Notes string
			Sections               map[string]string // the notes per language, for the changelog files
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			reply(nil, err)
			return
		}
		reply(w.release(repo, req.Version, req.Commit, req.Notes, req.Sections))
	default:
		http.NotFound(rw, r)
	}
}

// checkedOut is the branch requirements are edited on: the one checked out, where the person's
// commits go. None when HEAD is detached.
func (w *workspace) checkedOut(repo gitx.Repo) string {
	out, err := repo.Git(w.ctx, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// decode reads a request that changes requirements, made on the page of branch. It is refused unless
// that branch is checked out: the commit would land on another branch than the one the person saw.
func (w *workspace) decode(r *http.Request, repo gitx.Repo, req any, branch *string) error {
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		return err
	}
	switch here := w.checkedOut(repo); {
	case *branch == "":
		return fmt.Errorf("say which branch this change is for")
	case here == "":
		return fmt.Errorf("no branch is checked out; check out %s to change its requirements", *branch)
	case here != *branch:
		return fmt.Errorf("%s is checked out, not %s; check out %s to change its requirements", here, *branch, *branch)
	}
	return nil
}

// featurePath checks that a path names a .feature file inside the repository.
func featurePath(dir, path string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(path))
	if !strings.HasSuffix(clean, ".feature") || filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("%q is not a feature file in the repository", path)
	}
	if _, err := os.Stat(filepath.Join(dir, clean)); err != nil {
		return "", err
	}
	return filepath.ToSlash(clean), nil
}

// save writes the person's version of a feature file and commits it as theirs.
func (w *workspace) save(repo gitx.Repo, path, text string) (any, error) {
	path, err := featurePath(repo.Dir, path)
	if err != nil {
		return nil, err
	}
	if err := w.unsaved(repo, path); err != nil {
		return nil, err
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	old, _ := os.ReadFile(filepath.Join(repo.Dir, path))
	if string(old) == text {
		return map[string]string{"commit": ""}, nil
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, path), []byte(text), 0o644); err != nil {
		return nil, err
	}
	name := gherkin.Parse(text).Name
	return w.commit(repo, path, fmt.Sprintf("spec: change the scenarios of %q", name))
}

var proposedTag = regexp.MustCompile(`(^|\s)@proposed(\s|$)`)

// approve removes @proposed from the named scenarios and commits with Approved-by.
func (w *workspace) approve(repo gitx.Repo, path string, scenarios []string) (any, error) {
	path, err := featurePath(repo.Dir, path)
	if err != nil {
		return nil, err
	}
	if err := w.unsaved(repo, path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(repo.Dir, path))
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	scenario := regexp.MustCompile(`^\s*(?:Scenario|Scenario Outline|Scenario Template|Example):\s*(.+)$`)
	approved := 0
	for i, l := range lines {
		m := scenario.FindStringSubmatch(l)
		if m == nil || !slices.Contains(scenarios, strings.TrimSpace(m[1])) {
			continue
		}
		// The scenario's tags are on the lines just above it.
		for j := i - 1; j >= 0 && strings.HasPrefix(strings.TrimSpace(lines[j]), "@"); j-- {
			if proposedTag.MatchString(lines[j]) {
				lines[j] = strings.TrimRight(proposedTag.ReplaceAllString(lines[j], "$1"), " \t")
				approved++
			}
		}
	}
	if approved == 0 {
		return nil, fmt.Errorf("none of these scenarios is proposed")
	}
	// Drop tag lines left empty.
	var out []string
	for i, l := range lines {
		if strings.TrimSpace(l) == "" && i+1 < len(lines) && scenario.MatchString(lines[i+1]) && i > 0 && strings.TrimSpace(lines[i-1]) == "" {
			continue
		}
		out = append(out, l)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, path), []byte(strings.Join(out, "\n")), 0o644); err != nil {
		return nil, err
	}
	user, _ := repo.Git(w.ctx, "config", "user.name")
	email, _ := repo.Git(w.ctx, "config", "user.email")
	msg := fmt.Sprintf("spec: approve %d %s of %q\n\nApproved-by: %s <%s>", approved, map[bool]string{true: "scenario", false: "scenarios"}[approved == 1],
		gherkin.Parse(string(data)).Name, strings.TrimSpace(user), strings.TrimSpace(email))
	return w.commit(repo, path, msg)
}

// versionRe is a release tag: v1.2.3, or a pre-release such as v1.2.3-rc.1. The page orders them.
var versionRe = regexp.MustCompile(`^v\d+\.\d+\.\d+(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)

// release records an approved release. With a changelog (the default), it adds the release's section
// to each language's file, commits that as the person ("chore(release): v1.2.0") and tags that commit;
// the commit only adds the notes, so the release is still the one that was checked. Without one, it
// tags the commit itself. The tag holds the notes, then Approved-by with the person's Git identity. It
// creates no remote state; pushing is the person's choice.
func (w *workspace) release(repo gitx.Repo, version, commit, notes string, sections map[string]string) (any, error) {
	if !versionRe.MatchString(version) {
		return nil, fmt.Errorf("%q is not a version like v1.2.3 or v1.2.3-rc.1", version)
	}
	if _, err := repo.Git(w.ctx, "rev-parse", "-q", "--verify", "refs/tags/"+version); err == nil {
		return nil, fmt.Errorf("%s already exists", version)
	}
	sha, err := repo.Git(w.ctx, "rev-parse", "-q", "--verify", commit+"^{commit}")
	if err != nil {
		return nil, fmt.Errorf("%q is not a commit", commit)
	}
	user, _ := repo.Git(w.ctx, "config", "user.name")
	email, _ := repo.Git(w.ctx, "config", "user.email")
	if strings.TrimSpace(user) == "" {
		return nil, fmt.Errorf("set your name in Git first: git config user.name")
	}
	if files, err := w.changelogs(repo); err != nil {
		return nil, err
	} else if len(files) > 0 {
		if sha, err = w.writeChangelogs(repo, files, version, strings.TrimSpace(sha), sections); err != nil {
			return nil, err
		}
	}
	msg := fmt.Sprintf("%s\n\nApproved-by: %s <%s>\n", strings.TrimSpace(notes), strings.TrimSpace(user), strings.TrimSpace(email))
	f, err := os.CreateTemp("", "shipwake-notes-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(msg); err != nil {
		f.Close()
		return nil, err
	}
	f.Close()
	if _, err := repo.Git(w.ctx, "tag", "-a", version, strings.TrimSpace(sha), "-F", f.Name(), "--cleanup=verbatim"); err != nil {
		return nil, err
	}
	if err := w.rebuild(); err != nil {
		return nil, err
	}
	return map[string]string{"tag": version, "commit": strings.TrimSpace(sha)}, nil
}

// changelogs are the repository's changelog files by language, as its config says.
func (w *workspace) changelogs(repo gitx.Repo) (map[string]string, error) {
	cfg, err := build.LoadConfig(repo.Dir)
	if err != nil {
		return nil, err
	}
	if len(cfg.Languages) == 0 {
		cfg.Languages = []string{"en"}
	}
	return build.ChangelogFiles(cfg), nil
}

// writeChangelogs adds the release to each changelog file and commits them on top of the released
// commit, which must be the newest one: the release commit goes on top of the branch.
func (w *workspace) writeChangelogs(repo gitx.Repo, files map[string]string, version, sha string, sections map[string]string) (string, error) {
	head, err := repo.Git(w.ctx, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(head) != sha {
		return "", fmt.Errorf("only the newest commit can be released: its changelog is committed on top of it")
	}
	var paths []string
	for lang, file := range files {
		text := strings.TrimSpace(sections[lang])
		if text == "" {
			continue // no notes in this language: its file stays as it is
		}
		if err := w.unsaved(repo, file); err != nil {
			return "", err
		}
		old, _ := os.ReadFile(filepath.Join(repo.Dir, file))
		next := changelog.Insert(string(old), version, time.Now().Format("2006-01-02"), text)
		if err := os.WriteFile(filepath.Join(repo.Dir, file), []byte(next), 0o644); err != nil {
			return "", err
		}
		paths = append(paths, file)
	}
	if len(paths) == 0 {
		return sha, nil
	}
	if _, err := repo.Git(w.ctx, append([]string{"add", "--"}, paths...)...); err != nil {
		return "", err
	}
	if _, err := repo.Git(w.ctx, append([]string{"commit", "-q", "-m", "chore(release): " + version, "--"}, paths...)...); err != nil {
		return "", err
	}
	out, err := repo.Git(w.ctx, "rev-parse", "HEAD")
	return strings.TrimSpace(out), err
}

func (w *workspace) commit(repo gitx.Repo, path, msg string) (any, error) {
	if _, err := repo.Git(w.ctx, "add", "--", path); err != nil {
		return nil, err
	}
	if _, err := repo.Git(w.ctx, "commit", "-q", "-m", msg, "--", path); err != nil {
		return nil, err
	}
	sha, _ := repo.Git(w.ctx, "rev-parse", "HEAD")
	if err := w.rebuild(); err != nil {
		return nil, err
	}
	return map[string]string{"commit": strings.TrimSpace(sha)}, nil
}

var scenarioLine = regexp.MustCompile(`^\s*(?:Scenario|Scenario Outline|Scenario Template|Example):`)

type block struct {
	lines    []string // its tag lines, the Scenario line and its steps, trailing blank lines removed
	proposed bool
}

// splitFeature cuts a feature file into its header (feature, description, background) and scenario
// blocks, each starting at its tags.
func splitFeature(text string) (header []string, blocks []block) {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	start := -1 // where the current block began
	flush := func(end int) {
		if start < 0 {
			return
		}
		b := block{lines: trimBlank(lines[start:end])}
		for _, l := range b.lines {
			if strings.HasPrefix(strings.TrimSpace(l), "@") && proposedTag.MatchString(l) {
				b.proposed = true
			}
		}
		blocks = append(blocks, b)
	}
	for i := 0; i < len(lines); i++ {
		if !scenarioLine.MatchString(lines[i]) {
			continue
		}
		begin := i
		for begin > 0 && strings.HasPrefix(strings.TrimSpace(lines[begin-1]), "@") {
			begin--
		}
		if start < 0 {
			header = trimBlank(lines[:begin])
		} else {
			flush(begin)
		}
		start = begin
	}
	if start < 0 {
		return trimBlank(lines), nil
	}
	flush(len(lines))
	return header, blocks
}

func trimBlank(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// untagged drops @proposed from a block, and tag lines it leaves empty.
func untagged(lines []string) []string {
	var out []string
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "@") {
			l = strings.TrimRight(proposedTag.ReplaceAllString(l, "$1"), " \t")
			if strings.TrimSpace(l) == "" {
				continue
			}
		}
		out = append(out, l)
	}
	return out
}

// saveProposal replaces the proposed scenarios with the person's text and commits it.
func (w *workspace) saveProposal(repo gitx.Repo, path, text string) (any, error) {
	path, err := featurePath(repo.Dir, path)
	if err != nil {
		return nil, err
	}
	if err := w.unsaved(repo, path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(repo.Dir, path))
	if err != nil {
		return nil, err
	}
	next, err := withProposal(string(data), text)
	if err != nil {
		return nil, err
	}
	if next == string(data) {
		return map[string]string{"commit": ""}, nil
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, path), []byte(next), 0o644); err != nil {
		return nil, err
	}
	return w.commit(repo, path, fmt.Sprintf("spec: change the proposal for %q", gherkin.Parse(next).Name))
}

// proposalOf is the proposed scenarios as plain Gherkin, without their @proposed tags.
func proposalOf(file string) string {
	_, blocks := splitFeature(file)
	var text []string
	for _, b := range blocks {
		if b.proposed {
			text = append(text, strings.Join(untagged(b.lines), "\n"))
		}
	}
	return strings.Join(text, "\n\n")
}

// withProposal puts the person's text in place of the proposed scenarios. Every scenario in it stays
// proposed until approved; the other scenarios are left exactly as they are.
func withProposal(file, text string) (string, error) {
	header, blocks := splitFeature(file)
	mine, rest := splitFeature("Feature: x\n" + text)
	if len(trimBlank(mine[1:])) > 0 {
		return "", fmt.Errorf("the proposal must start with a Scenario")
	}
	var out []string
	out = append(out, header...)
	for _, b := range blocks {
		if !b.proposed {
			out = append(out, "")
			out = append(out, b.lines...)
		}
	}
	for _, b := range rest {
		lines := b.lines
		indent := ""
		for _, l := range lines {
			if scenarioLine.MatchString(l) {
				indent = l[:len(l)-len(strings.TrimLeft(l, " \t"))]
				break
			}
		}
		if strings.HasPrefix(strings.TrimSpace(lines[0]), "@") {
			lines = append([]string{lines[0] + " @proposed"}, lines[1:]...)
		} else {
			lines = append([]string{indent + "@proposed"}, lines...)
		}
		out = append(out, "")
		out = append(out, lines...)
	}
	return strings.Join(out, "\n") + "\n", nil
}

// committed reads a file as the branch's last commit has it. Only committed scenarios count: a
// proposal is what the commit says, the same thing a published site shows; text that is only in
// the working tree is nobody's proposal yet.
func (w *workspace) committed(repo gitx.Repo, path string) (string, error) {
	return repo.Git(w.ctx, "show", "HEAD:"+path)
}

// unsaved refuses to write a file that has changes nobody committed: saving from the page would
// commit them as part of the person's change.
func (w *workspace) unsaved(repo gitx.Repo, path string) error {
	out, err := repo.Git(w.ctx, "status", "--porcelain", "--", path)
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "" {
		return fmt.Errorf("%s has changes that aren't committed; commit or discard them first, so they aren't saved as part of this change", path)
	}
	return nil
}
