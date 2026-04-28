package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/idvoretskyi/cncf-github-maintainers/internal/config"
	"github.com/idvoretskyi/cncf-github-maintainers/internal/csv"
	gh "github.com/idvoretskyi/cncf-github-maintainers/internal/github"
	"github.com/spf13/cobra"
)

var (
	auditApply bool
	auditYes   bool
)

var auditCmd = &cobra.Command{
	Use:          "audit",
	SilenceUsage: true,
	Short:        "Audit the cncf-maintainers GitHub team against the CNCF maintainers CSV",
	Long: `Compares the membership of the cncf-maintainers/cncf-maintainers GitHub
team with the GitHub usernames listed in the CNCF project-maintainers.csv
and reports any discrepancies.

By default this command is read-only and only prints a report:
  • Users in the CSV but NOT on the team are reported (no action taken —
    additions are a manual step via 'cncf-maintainers add <username>').
  • Users on the team but NOT in the CSV are reported as candidates for
    removal.

Pass --apply to actually remove the extra team members. Users with the
'maintainer' role on the team (typically organisation owners) are NEVER
removed automatically.

Exit status:
  0  the team membership matches the CSV (or all requested removals
     succeeded with --apply)
  1  discrepancies were detected (read-only mode) or one or more
     operations failed (--apply mode)

Authentication (checked in order):
  1. GITHUB_TOKEN environment variable
  2. GH_TOKEN environment variable
  3. gh auth token (local GitHub CLI config)

The token must have 'admin:org' scope when --apply is used; read access
to the org and team is sufficient otherwise.

Examples:
  # Read-only audit (preview discrepancies)
  cncf-maintainers audit

  # Apply removals after a confirmation prompt
  cncf-maintainers audit --apply

  # Apply removals non-interactively (e.g. in CI)
  cncf-maintainers audit --apply --yes`,
	Args: cobra.NoArgs,
	RunE: runAudit,
}

func init() {
	rootCmd.AddCommand(auditCmd)
	auditCmd.Flags().BoolVar(&auditApply, "apply", false, "Apply removals (default is read-only/preview)")
	auditCmd.Flags().BoolVarP(&auditYes, "yes", "y", false, "Skip the confirmation prompt when --apply is set")
}

func runAudit(cmd *cobra.Command, _ []string) error {
	token, err := config.GetGitHubToken()
	if err != nil {
		return err
	}
	client := gh.NewClient(token)

	ctx := context.Background()

	fmt.Fprintf(cmd.OutOrStdout(), "Fetching CNCF maintainers list...\n")
	maintainers, err := csv.FetchMaintainers(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Fetching %s/%s team members...\n\n", config.OrgName, config.TeamSlug)
	teamMembers, err := client.ListTeamMembers(ctx, config.OrgName, config.TeamSlug)
	if err != nil {
		return err
	}

	// Build a lowercase-keyed CSV set, preserving the original casing for
	// display. A username that appears multiple times across projects is
	// counted once.
	csvSet := make(map[string]string)
	for _, m := range maintainers {
		key := strings.ToLower(strings.TrimSpace(m.GitHubName))
		if key == "" {
			continue
		}
		if _, ok := csvSet[key]; !ok {
			csvSet[key] = m.GitHubName
		}
	}

	// Partition the team into regular members and maintainers (admins).
	memberSet := make(map[string]string)     // role=member
	maintainerSet := make(map[string]string) // role=maintainer
	for _, tm := range teamMembers {
		key := strings.ToLower(tm.Login)
		if tm.Role == "maintainer" {
			maintainerSet[key] = tm.Login
		} else {
			memberSet[key] = tm.Login
		}
	}

	// Anything on the team in any role counts as "present".
	teamUnion := make(map[string]bool, len(memberSet)+len(maintainerSet))
	for k := range memberSet {
		teamUnion[k] = true
	}
	for k := range maintainerSet {
		teamUnion[k] = true
	}

	// In CSV but missing from the team — report only.
	var missingFromTeam []string
	for key, display := range csvSet {
		if !teamUnion[key] {
			missingFromTeam = append(missingFromTeam, display)
		}
	}

	// On the team (role=member) but not in the CSV — candidates for removal.
	var extraOnTeam []string
	for key, display := range memberSet {
		if _, ok := csvSet[key]; !ok {
			extraOnTeam = append(extraOnTeam, display)
		}
	}

	sort.Slice(missingFromTeam, func(i, j int) bool {
		return strings.ToLower(missingFromTeam[i]) < strings.ToLower(missingFromTeam[j])
	})
	sort.Slice(extraOnTeam, func(i, j int) bool {
		return strings.ToLower(extraOnTeam[i]) < strings.ToLower(extraOnTeam[j])
	})

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Audit summary:\n")
	fmt.Fprintf(out, "  CSV maintainers (unique GitHub usernames):  %d\n", len(csvSet))
	fmt.Fprintf(out, "  Team members (role=member):                 %d\n", len(memberSet))
	fmt.Fprintf(out, "  Team maintainers (role=maintainer, kept):   %d\n", len(maintainerSet))
	fmt.Fprintf(out, "  In CSV but NOT on team (report only):       %d\n", len(missingFromTeam))
	fmt.Fprintf(out, "  On team but NOT in CSV (to remove):         %d\n", len(extraOnTeam))
	fmt.Fprintln(out)

	if len(missingFromTeam) > 0 {
		fmt.Fprintf(out, "In CSV but missing from team (no action will be taken — add manually with 'cncf-maintainers add <username>'):\n")
		for _, u := range missingFromTeam {
			fmt.Fprintf(out, "  - %s\n", u)
		}
		fmt.Fprintln(out)
	}

	if len(extraOnTeam) > 0 {
		if auditApply {
			fmt.Fprintf(out, "On team but not in CSV (will be removed):\n")
		} else {
			fmt.Fprintf(out, "On team but not in CSV (would be removed with --apply):\n")
		}
		for _, u := range extraOnTeam {
			fmt.Fprintf(out, "  - %s\n", u)
		}
		fmt.Fprintln(out)
	}

	inSync := len(missingFromTeam) == 0 && len(extraOnTeam) == 0
	if inSync {
		fmt.Fprintf(out, "[✓] Team membership matches the CSV.\n")
		return nil
	}

	if !auditApply {
		fmt.Fprintf(out, "Read-only audit complete. Re-run with --apply to remove the extra team members.\n")
		return fmt.Errorf("team membership does not match the CSV (%d missing, %d extra)",
			len(missingFromTeam), len(extraOnTeam))
	}

	// --apply path: only removals are performed.
	if len(extraOnTeam) == 0 {
		fmt.Fprintf(out, "Nothing to remove. (%d user(s) are in the CSV but missing from the team — add manually if appropriate.)\n", len(missingFromTeam))
		return nil
	}

	if !auditYes {
		confirmed, err := promptConfirmStrict(cmd,
			fmt.Sprintf("Remove %d user(s) from %s/%s? [y/N]: ",
				len(extraOnTeam), config.OrgName, config.TeamSlug))
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Fprintln(out, "Aborted. No changes made.")
			return nil
		}
	}

	var failed int
	for _, username := range extraOnTeam {
		if err := client.RemoveFromTeam(ctx, config.OrgName, config.TeamSlug, username); err != nil {
			failed++
			fmt.Fprintf(out, "[!] Failed to remove %s: %v\n", username, err)
			continue
		}
		fmt.Fprintf(out, "[✓] Removed %s from %s/%s\n", username, config.OrgName, config.TeamSlug)
	}

	fmt.Fprintf(out, "\nDone. Removed: %d, Failed: %d\n", len(extraOnTeam)-failed, failed)
	if failed > 0 {
		return fmt.Errorf("%d removal(s) failed", failed)
	}
	return nil
}

// promptConfirmStrict writes prompt to cmd's stdout and reads a line from
// stdin. Unlike promptConfirm (used by 'add'), it defaults to NO on blank
// input and only returns true for explicit 'y'/'yes' answers. This is the
// safer default for destructive operations such as audit removals.
func promptConfirmStrict(cmd *cobra.Command, prompt string) (bool, error) {
	fmt.Fprint(cmd.OutOrStdout(), prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return false, fmt.Errorf("reading confirmation: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout())
		return false, nil
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return answer == "y" || answer == "yes", nil
}
