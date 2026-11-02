package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/blu9a/shipwake/internal/build"
	"github.com/blu9a/shipwake/internal/gitx"
	"github.com/blu9a/shipwake/internal/looks"
)

// looksCmd lists a commit's screenshots in visual review; looks approve approves them.
func looksCmd() *cobra.Command {
	var commit, results string
	var asJSON bool
	c := &cobra.Command{
		Use:   "looks [repo]",
		Short: "List a commit's screenshots and how each compares with its approved look: same, changed or new",
		Long: "A screenshot takes part in visual review when its attachment is named \"look: <subject>\" or\n" +
			"\"look: <subject> [<variant>]\", such as \"look: Button [dark, phone]\". Each is compared pixel by pixel\n" +
			"with the newest look approved for it on this commit or one before it. One that differs, or that has\n" +
			"no approved look yet, holds the commit in shipwake check until someone approves it.",
		Example: "  shipwake looks\n" +
			"  shipwake looks approve \"Button [dark]\"\n" +
			"  shipwake looks approve --all --commit 1a2b3c4",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			l, err := build.Load(ctx, projectsFrom(args, results)[0], -1, logf(cmd))
			if err != nil {
				return err
			}
			defer l.Close()
			sha, err := l.Repo.Resolve(ctx, commit)
			if err != nil {
				return err
			}
			ls, _, err := l.ReviewLooks(ctx, sha)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if asJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(ls)
			}
			if len(ls.Looks) == 0 {
				fmt.Fprintf(out, "%.7s: no screenshots recorded (attachments named \"look: <subject> [<variant>]\")\n", sha)
				return nil
			}
			fmt.Fprintf(out, "%.7s, compared with the approved looks\n", sha)
			for _, lk := range ls.Looks {
				switch lk.Status {
				case "same":
					fmt.Fprintf(out, "  ✓ %s: as approved by %s\n", lk.Name, name(lk.Approved.By))
				case "changed":
					fmt.Fprintf(out, "  ✗ %s: %s differ from the look approved on %.7s\n", lk.Name, plural(lk.Pixels, "pixel"), lk.Approved.Commit)
				default:
					fmt.Fprintf(out, "  + %s: new, waits for a first approval\n", lk.Name)
				}
			}
			return nil
		},
	}
	c.Flags().StringVar(&commit, "commit", "HEAD", "the commit whose screenshots to list")
	c.Flags().StringVarP(&results, "results", "r", "", resultsHelp)
	c.Flags().BoolVar(&asJSON, "json", false, "print as JSON")
	c.AddCommand(approveLooksCmd())
	return c
}

func approveLooksCmd() *cobra.Command {
	var commit, results, repoDir string
	var all, push bool
	c := &cobra.Command{
		Use:   "approve [look]...",
		Short: "Approve how a commit's screenshots look, as you: later commits are compared with them",
		Long: "Approves the named screenshots of a commit that changed or are new (a name such as \"Button [dark]\",\n" +
			"or a subject for all its variants), or all of them with --all. The approval is kept with the runs\n" +
			"(looks/ on the results branch), committed with Approved-by and your Git identity. --push sends the\n" +
			"results branch to origin; nothing is pushed otherwise.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !all {
				return fmt.Errorf("name the looks to approve, or approve every changed and new one with --all")
			}
			ctx := cmd.Context()
			p := projectsFrom([]string{repoDir}, results)[0]
			repo := gitx.Repo{Dir: repoDir}
			n, _ := repo.Git(ctx, "config", "user.name")
			e, _ := repo.Git(ctx, "config", "user.email")
			by := strings.TrimSpace(n) + " <" + strings.TrimSpace(e) + ">"
			approved, err := build.ApproveLooks(ctx, p, commit, args, all, by, push)
			if err != nil {
				return err
			}
			for _, a := range approved {
				fmt.Fprintf(cmd.OutOrStdout(), "Approved %s on %.7s\n", looks.Name(a.Subject, a.Variant), a.Commit)
			}
			return nil
		},
	}
	c.Flags().StringVar(&commit, "commit", "HEAD", "the commit whose screenshots to approve")
	c.Flags().StringVarP(&results, "results", "r", "", resultsHelp)
	c.Flags().StringVar(&repoDir, "repo", ".", "the repository")
	c.Flags().BoolVar(&all, "all", false, "approve every screenshot of the commit that changed or is new")
	c.Flags().BoolVar(&push, "push", false, "push the results branch to origin")
	return c
}

// name is a person's name without their email.
func name(by string) string {
	n, _, _ := strings.Cut(by, " <")
	return n
}
