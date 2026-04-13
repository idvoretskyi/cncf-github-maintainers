package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/idvoretskyi/cncf-github-maintainers/internal/config"
	"github.com/idvoretskyi/cncf-github-maintainers/internal/csv"
	gh "github.com/idvoretskyi/cncf-github-maintainers/internal/github"
	"github.com/spf13/cobra"
)

var (
	addFile   string
	addDryRun bool
)

// addResult captures what happened for a single username during "add".
type addResult struct {
	username   string
	found      bool  // present in the CNCF CSV
	alreadyMem bool  // already an active team member
	added      bool  // newly added in this run
	dryRun     bool  // would have been added (--dry-run)
	skipped    bool  // user declined the confirmation prompt
	err        error // API or lookup error
}

var addCmd = &cobra.Command{
	Use:   "add [username...]",
	Short: "Validate CNCF maintainer(s) and add them to the cncf-maintainers team",
	Long: `Fetches the CNCF project-maintainers.csv, validates the supplied
GitHub username(s), and — after showing their details and asking for
confirmation — adds confirmed maintainers to the
cncf-maintainers/cncf-maintainers team on GitHub.

Multiple usernames can be passed as separate arguments or as a single
quoted string with names separated by spaces, commas, or newlines.
For file-based bulk operations use --file. The confirmation prompt is
only shown when a single username is resolved; bulk runs are automatic.

Authentication (checked in order):
  1. GITHUB_TOKEN environment variable
  2. GH_TOKEN environment variable
  3. gh auth token (local GitHub CLI config)

The token must have 'admin:org' scope (unless --dry-run is used).

Examples:
  # Single user (interactive confirmation)
  cncf-maintainers add johnsmith

  # Multiple users as separate arguments
  cncf-maintainers add johnsmith janedoe janesmith

  # Multiple users as a copy-pasted comma/space-separated list
  cncf-maintainers add "johnsmith, janedoe, janesmith"

  # Dry-run: validate but do not add
  cncf-maintainers add johnsmith --dry-run

  # Bulk from file (no confirmation prompt)
  cncf-maintainers add --file usernames.txt

  # Bulk dry-run
  cncf-maintainers add --file usernames.txt --dry-run`,
	Args: cobra.ArbitraryArgs,
	RunE: runAdd,
}

func init() {
	rootCmd.AddCommand(addCmd)
	addCmd.Flags().StringVarP(&addFile, "file", "f", "", "Path to a file with one GitHub username per line")
	addCmd.Flags().BoolVar(&addDryRun, "dry-run", false, "Validate only – do not add to the team")
}

func runAdd(cmd *cobra.Command, args []string) error {
	usernames, err := readUsernames(args, addFile)
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

	// Single-user flow: interactive with confirmation prompt.
	// Bulk flow (multiple usernames or --file): non-interactive, no prompt.
	interactive := len(usernames) == 1 && addFile == ""

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
		res, err := processUsername(ctx, cmd, client, username, maintainers, interactive, addDryRun)
		if err != nil {
			return err
		}
		results = append(results, res)
	}

	// Print summary when processing more than one username.
	if len(usernames) > 1 {
		printAddSummary(cmd, results)
	}

	// Non-zero exit if any user had an API error.
	for _, r := range results {
		if r.err != nil {
			return fmt.Errorf("one or more operations failed (see output above)")
		}
	}
	return nil
}

// processUsername validates and optionally adds a single username to the team.
// A non-nil error signals that the entire run should be aborted (e.g. a failed
// confirmation read); per-user API errors are recorded in addResult.err instead.
func processUsername(ctx context.Context, cmd *cobra.Command, client *gh.Client,
	username string, maintainers []csv.Maintainer, interactive, dryRun bool) (addResult, error) {

	res := addResult{username: username}

	if interactive {
		fmt.Fprintf(cmd.OutOrStdout(), "Checking if %s is an active CNCF project maintainer...\n\n", username)
	}

	matches := csv.FindByGitHubName(maintainers, username)
	if len(matches) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "[✗] %s — NOT found in the CNCF maintainers list, skipping\n", username)
		return res, nil
	}

	res.found = true
	fmt.Fprintf(cmd.OutOrStdout(), "[✓] %s is a confirmed CNCF project maintainer:\n", username)
	for _, m := range matches {
		printMaintainerDetails(cmd.OutOrStdout(), m, false)
	}
	fmt.Fprintln(cmd.OutOrStdout())

	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "    [!] Would add to %s/%s (dry-run)\n", config.OrgName, config.TeamSlug)
		res.dryRun = true
		return res, nil
	}

	if interactive {
		confirmed, err := promptConfirm(cmd, fmt.Sprintf("Add %s to the %s/%s GitHub team? [Y/n]: ", username, config.OrgName, config.TeamSlug))
		if err != nil {
			return res, err
		}
		if !confirmed {
			fmt.Fprintf(cmd.OutOrStdout(), "Skipped.\n")
			res.skipped = true
			return res, nil
		}
	}

	isMember, err := client.IsTeamMember(ctx, config.OrgName, config.TeamSlug, username)
	if err != nil {
		res.err = err
		fmt.Fprintf(cmd.OutOrStdout(), "    [!] Error checking membership: %v\n", err)
		return res, nil
	}
	if isMember {
		fmt.Fprintf(cmd.OutOrStdout(), "[~] %s is already an active member of %s/%s\n", username, config.OrgName, config.TeamSlug)
		res.alreadyMem = true
		return res, nil
	}

	if err := client.AddToTeam(ctx, config.OrgName, config.TeamSlug, username); err != nil {
		res.err = err
		fmt.Fprintf(cmd.OutOrStdout(), "[!] Failed to add %s: %v\n", username, err)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "[✓] %s added to %s/%s\n", username, config.OrgName, config.TeamSlug)
		res.added = true
	}
	return res, nil
}

// promptConfirm writes prompt to cmd's stdout and reads a line from stdin.
// Returns true for blank input (default yes) or any input starting with 'y'/'Y'.
func promptConfirm(cmd *cobra.Command, prompt string) (bool, error) {
	fmt.Fprint(cmd.OutOrStdout(), prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return false, fmt.Errorf("reading confirmation: %w", err)
		}
		// EOF – treat as "no"
		fmt.Fprintln(cmd.OutOrStdout())
		return false, nil
	}
	answer := strings.TrimSpace(scanner.Text())
	return answer == "" || strings.HasPrefix(strings.ToLower(answer), "y"), nil
}

func printAddSummary(cmd *cobra.Command, results []addResult) {
	var notFound, dryRun, alreadyMem, added, skipped, failed int
	for _, r := range results {
		switch {
		case r.err != nil:
			failed++
		case !r.found:
			notFound++
		case r.dryRun:
			dryRun++
		case r.skipped:
			skipped++
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
	if skipped > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  [-] Skipped:          %d\n", skipped)
	}
	if notFound > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  [✗] Not in CSV:       %d\n", notFound)
	}
	if failed > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  [!] Errors:           %d\n", failed)
	}
}
