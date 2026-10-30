package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blu9a/shipwake/internal/changelog"
	"github.com/blu9a/shipwake/internal/gitx"
)

const featureFile = `Feature: Splitting a bill
  Friends paying together see what each of them owes.

  Scenario: Splitting evenly
    Given a bill of 90.00
    When we split it between 3 people
    Then each person pays 30.00

  @proposed
  Scenario: Adding a tip
    Given a bill of 100.00
    And a tip of 15%
    Then each person pays 57.50

  @slow @proposed
  Scenario: Nobody to split with
    Given a bill of 50.00
    Then I'm asked how many people are paying
`

func TestProposalRoundTrip(t *testing.T) {
	text := proposalOf(featureFile)
	if strings.Contains(text, "@proposed") || !strings.HasPrefix(text, "  Scenario: Adding a tip") || !strings.Contains(text, "@slow\n  Scenario: Nobody") {
		t.Fatalf("proposal = %q", text)
	}
	same, err := withProposal(featureFile, text)
	if err != nil || same != featureFile {
		t.Fatalf("unchanged proposal should give the same file:\n%s", same)
	}
	edited := strings.Replace(text, "a tip of 15%", "a tip of 10%", 1) + "\n\n  Scenario: Rounding\n    Then nobody pays a fraction of a cent\n"
	got, err := withProposal(featureFile, edited)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"  Scenario: Splitting evenly\n    Given a bill of 90.00", // approved: untouched
		"  @proposed\n  Scenario: Adding a tip\n    Given a bill of 100.00\n    And a tip of 10%",
		"  @slow @proposed\n  Scenario: Nobody to split with",
		"  @proposed\n  Scenario: Rounding\n    Then nobody pays a fraction of a cent\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if _, err := withProposal(featureFile, "Given a stray step\n"); err == nil {
		t.Error("a proposal must start with a Scenario")
	}
}

func TestRelease(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo := gitx.Repo{Dir: dir}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.name", "Ana"},
		{"config", "user.email", "ana@example.com"},
		{"commit", "-q", "--allow-empty", "-m", "first"},
	} {
		if _, err := repo.Git(ctx, args...); err != nil {
			t.Fatal(err)
		}
	}
	w := &workspace{ctx: ctx, rebuild: func() error { return nil }}
	if err := os.WriteFile(filepath.Join(dir, ".shipwake.yaml"), []byte("changelog: none\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Git(ctx, "add", "."); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Git(ctx, "commit", "-q", "-m", "tags only"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.release(repo, "v0.1.0-rc.1", "HEAD", "# Splitting a bill\n\nEach person's share.", nil); err != nil {
		t.Fatal(err)
	}
	msg, err := repo.Git(ctx, "tag", "-l", "--format=%(contents)", "v0.1.0-rc.1")
	if err != nil {
		t.Fatal(err)
	}
	want := "# Splitting a bill\n\nEach person's share.\n\nApproved-by: Ana <ana@example.com>\n"
	if strings.TrimRight(msg, "\n") != strings.TrimRight(want, "\n") {
		t.Errorf("tag message = %q, want %q", msg, want)
	}
	for _, c := range []struct{ version, commit string }{
		{"v0.1.0-rc.1", "HEAD"}, // exists
		{"0.1.0", "HEAD"},       // not a version
		{"v0.1", "HEAD"},
		{"v0.1.0", "nope"}, // not a commit
	} {
		if _, err := w.release(repo, c.version, c.commit, "notes", nil); err == nil {
			t.Errorf("release(%q, %q) succeeded, want an error", c.version, c.commit)
		}
	}
}

func TestMetricProposal(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo := gitx.Repo{Dir: dir}
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.name", "Ana"}, {"config", "user.email", "ana@example.com"}} {
		if _, err := repo.Git(ctx, args...); err != nil {
			t.Fatal(err)
		}
	}
	path := "metrics/totals.exact.metric"
	if err := os.MkdirAll(filepath.Join(dir, "metrics"), 0o755); err != nil {
		t.Fatal(err)
	}
	proposal := "@proposed @eval\nMetric: Exact totals\n  Bills that add up.\n\n  Higher is better\n  Blocks a release below 100%\n"
	if err := os.WriteFile(filepath.Join(dir, path), []byte(proposal), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-q", "-m", "propose"}} {
		if _, err := repo.Git(ctx, args...); err != nil {
			t.Fatal(err)
		}
	}
	w := &workspace{ctx: ctx, rebuild: func() error { return nil }}
	got, err := w.metric(repo, path)
	if err != nil {
		t.Fatal(err)
	}
	if text := got.(map[string]any)["text"]; text != "@eval\nMetric: Exact totals\n  Bills that add up.\n\n  Higher is better\n  Blocks a release below 100%" {
		t.Errorf("text = %q", text)
	}
	if _, err := w.saveMetric(repo, path, "@eval\nMetric: Exact totals\n  Higher is better\n  Blocks a release eventually\n"); err == nil {
		t.Error("a line Shipwake can't read was saved")
	}
	if _, err := w.saveMetric(repo, path, "@eval\nMetric: Exact totals\n  Higher is better\n  Blocks a release below 99.9%\n"); err != nil {
		t.Fatal(err)
	}
	saved, _ := os.ReadFile(filepath.Join(dir, path))
	if !strings.HasPrefix(string(saved), "@proposed\n@eval\n") || !strings.Contains(string(saved), "99.9%") {
		t.Errorf("saved:\n%s", saved)
	}
	if _, err := w.approveMetric(repo, path); err != nil {
		t.Fatal(err)
	}
	approved, _ := os.ReadFile(filepath.Join(dir, path))
	if strings.Contains(string(approved), "@proposed") || !strings.HasPrefix(string(approved), "@eval\n") {
		t.Errorf("approved:\n%s", approved)
	}
	msg, _ := repo.Git(ctx, "log", "-1", "--format=%B")
	if !strings.Contains(msg, "Approved-by: Ana <ana@example.com>") {
		t.Errorf("message %q", msg)
	}
	if _, err := w.approveMetric(repo, path); err == nil {
		t.Error("approving twice succeeded")
	}
}

func TestReleaseWithChangelog(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo := gitx.Repo{Dir: dir}
	git := func(args ...string) string {
		t.Helper()
		out, err := repo.Git(ctx, args...)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(out)
	}
	git("init", "-q")
	git("config", "user.name", "Ana")
	git("config", "user.email", "ana@example.com")
	git("commit", "-q", "--allow-empty", "-m", "feat: split a bill")
	checked := git("rev-parse", "HEAD")
	w := &workspace{ctx: ctx, rebuild: func() error { return nil }}
	notes := "Split a bill.\n\n### Features\n\n- split a bill (" + checked[:7] + ")"
	got, err := w.release(repo, "v0.1.0", checked, notes, map[string]string{"en": notes})
	if err != nil {
		t.Fatal(err)
	}
	tagged := got.(map[string]string)["commit"]
	if tagged == checked || git("rev-parse", tagged+"^") != checked {
		t.Errorf("the tag should be on a release commit on top of %s, is on %s", checked, tagged)
	}
	if s := git("log", "-1", "--format=%s", tagged); s != "chore(release): v0.1.0" {
		t.Errorf("release commit %q", s)
	}
	if files := git("show", "--name-only", "--format=", tagged); files != "CHANGELOG.md" {
		t.Errorf("release commit changes %q", files)
	}
	file, _ := os.ReadFile(filepath.Join(dir, "CHANGELOG.md"))
	if body, ok := changelog.Find(string(file), "v0.1.0"); !ok || body != notes {
		t.Errorf("changelog section %q in:\n%s", body, file)
	}
	if !strings.Contains(git("tag", "-l", "--format=%(contents)", "v0.1.0"), "Approved-by: Ana <ana@example.com>") {
		t.Error("the tag has no Approved-by")
	}
	// The release commit is now the newest: releasing an older commit can't put a changelog on it.
	git("commit", "-q", "--allow-empty", "-m", "fix: later")
	if _, err := w.release(repo, "v0.1.1", checked, notes, map[string]string{"en": notes}); err == nil {
		t.Error("released a commit that isn't the newest")
	}
}
