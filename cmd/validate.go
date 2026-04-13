package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/idvoretskyi/cncf-github-maintainers/internal/csv"
	"github.com/spf13/cobra"
)

var validateFile string

var validateCmd = &cobra.Command{
	Use:   "validate [username]",
	Short: "Check whether a GitHub username is a CNCF project maintainer",
	Long: `Fetches the CNCF project-maintainers.csv and checks whether the
supplied GitHub username appears in the "Github Name" column.

For bulk operations use --file to supply one username per line.

Examples:
  # Single user
  cncf-maintainers validate dims

  # Bulk from file (one username per line)
  cncf-maintainers validate --file usernames.txt`,
	Args: cobra.MaximumNArgs(1),
	RunE: runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)
	validateCmd.Flags().StringVarP(&validateFile, "file", "f", "", "Path to a file with one GitHub username per line")
}

func runValidate(cmd *cobra.Command, args []string) error {
	usernames, err := readUsernames(args, validateFile)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Fetching CNCF maintainers list...\n\n")

	ctx := context.Background()
	maintainers, err := csv.FetchMaintainers(ctx)
	if err != nil {
		return err
	}

	allFound := true
	for _, username := range usernames {
		matches := csv.FindByGitHubName(maintainers, username)
		if len(matches) == 0 {
			allFound = false
			fmt.Fprintf(cmd.OutOrStdout(), "[✗] %s — NOT found in the CNCF maintainers list\n", username)
			continue
		}

		fmt.Fprintf(cmd.OutOrStdout(), "[✓] %s — confirmed CNCF project maintainer\n", username)
		for _, m := range matches {
			fmt.Fprintf(cmd.OutOrStdout(), "    Name:    %s\n", m.Name)
			if m.Company != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "    Company: %s\n", m.Company)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "    Project: %s (%s)\n", m.Project, strings.ToLower(m.Level))
			if m.OwnersLink != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "    OWNERS:  %s\n", m.OwnersLink)
			}
		}
	}

	if len(usernames) > 1 {
		found := 0
		for _, u := range usernames {
			if len(csv.FindByGitHubName(maintainers, u)) > 0 {
				found++
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\nSummary: %d/%d username(s) are active CNCF maintainers\n", found, len(usernames))
	}

	if !allFound {
		os.Exit(1)
	}
	return nil
}
