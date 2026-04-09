package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/idvoretskyi/cncf-github-maintainers/internal/config"
	"github.com/idvoretskyi/cncf-github-maintainers/internal/csv"
	gh "github.com/idvoretskyi/cncf-github-maintainers/internal/github"
	"github.com/spf13/cobra"
)

var (
	addUsername string
	addFile     string
	addDryRun   bool
)

// addResult captures what happened for a single username during "add".
type addResult struct {
	username   string
	found      bool  // present in the CNCF CSV
	alreadyMem bool  // already an active team member
	added      bool  // newly added in this run
	dryRun     bool  // would have been added (--dry-run)
	err        error // API or lookup error
}

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Validate CNCF maintainer(s) and add them to the cncf-maintainers team",
	Long: `Fetches the CNCF project-maintainers.csv, validates the supplied
GitHub username(s), and adds confirmed maintainers to the
cncf-maintainers/cncf-maintainers team on GitHub.

Authentication:
  Requires the GITHUB_TOKEN environment variable to be set to a PAT
  with 'admin:org' scope (unless --dry-run is used).

Examples:
  # Single user
  cncf-maintainers add --username dims

  # Dry-run: validate but do not add
  cncf-maintainers add --username dims --dry-run

  # Bulk from file
  cncf-maintainers add --file usernames.txt

  # Bulk dry-run
  cncf-maintainers add --file usernames.txt --dry-run`,
	RunE: runAdd,
}

func init() {
	rootCmd.AddCommand(addCmd)
	addCmd.Flags().StringVarP(&addUsername, "username", "u", "", "GitHub username to add")
	addCmd.Flags().StringVarP(&addFile, "file", "f", "", "Path to a file with one GitHub username per line")
	addCmd.Flags().BoolVar(&addDryRun, "dry-run", false, "Validate only – do not add to the team")
}

func runAdd(cmd *cobra.Command, _ []string) error {
	usernames, err := readUsernames(addUsername, addFile)
	if err != nil {
		return err
	}

	// Require a token upfront unless we are just doing a dry-run.
	var client *gh.Client
	if !addDryRun {
		token, err := config.GetGitHubToken()
		if err != nil {
			return err
		}
		client = gh.NewClient(token)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Fetching CNCF maintainers list...\n\n")

	ctx := context.Background()
	maintainers, err := csv.FetchMaintainers(ctx)
	if err != nil {
		return err
	}

	if addDryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "Dry-run mode enabled – no changes will be made.\n\n")
	}

	results := make([]addResult, 0, len(usernames))

	for _, username := range usernames {
		res := addResult{username: username}

		matches := csv.FindByGitHubName(maintainers, username)
		if len(matches) == 0 {
			// Not a CNCF maintainer – print and move on.
			fmt.Fprintf(cmd.OutOrStdout(), "[✗] %s — NOT found in the CNCF maintainers list, skipping\n", username)
			results = append(results, res)
			continue
		}

		res.found = true
		fmt.Fprintf(cmd.OutOrStdout(), "[✓] %s — confirmed CNCF maintainer\n", username)
		for _, m := range matches {
			fmt.Fprintf(cmd.OutOrStdout(), "    Name:    %s\n", m.Name)
			if m.Company != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "    Company: %s\n", m.Company)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "    Project: %s (%s)\n", m.Project, strings.ToLower(m.Level))
		}

		if addDryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "    [!] Would add to %s/%s (dry-run)\n", config.OrgName, config.TeamSlug)
			res.dryRun = true
			results = append(results, res)
			continue
		}

		// Check current membership.
		isMember, err := client.IsTeamMember(ctx, config.OrgName, config.TeamSlug, username)
		if err != nil {
			res.err = err
			fmt.Fprintf(cmd.OutOrStdout(), "    [!] Error checking membership: %v\n", err)
			results = append(results, res)
			continue
		}
		if isMember {
			fmt.Fprintf(cmd.OutOrStdout(), "    [~] Already an active member of %s/%s\n", config.OrgName, config.TeamSlug)
			res.alreadyMem = true
			results = append(results, res)
			continue
		}

		// Add to team.
		if err := client.AddToTeam(ctx, config.OrgName, config.TeamSlug, username); err != nil {
			res.err = err
			fmt.Fprintf(cmd.OutOrStdout(), "    [!] Failed to add: %v\n", err)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "    [✓] Added to %s/%s\n", config.OrgName, config.TeamSlug)
			res.added = true
		}
		results = append(results, res)
	}

	// Print summary when processing more than one username.
	if len(usernames) > 1 {
		printAddSummary(cmd, results)
	}

	// Non-zero exit if any user failed (API error).
	for _, r := range results {
		if r.err != nil {
			return fmt.Errorf("one or more operations failed (see output above)")
		}
	}
	return nil
}

func printAddSummary(cmd *cobra.Command, results []addResult) {
	var notFound, dryRun, alreadyMem, added, failed int
	for _, r := range results {
		switch {
		case r.err != nil:
			failed++
		case !r.found:
			notFound++
		case r.dryRun:
			dryRun++
		case r.alreadyMem:
			alreadyMem++
		case r.added:
			added++
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nSummary (%d user(s) processed):\n", len(results))
	if added > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  [✓] Added:            %d\n", added)
	}
	if dryRun > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  [!] Would add (dry):  %d\n", dryRun)
	}
	if alreadyMem > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  [~] Already members:  %d\n", alreadyMem)
	}
	if notFound > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  [✗] Not in CSV:       %d\n", notFound)
	}
	if failed > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  [!] Errors:           %d\n", failed)
	}
}
