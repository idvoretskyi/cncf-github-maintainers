package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "cncf-maintainers",
	Short: "Validate and manage CNCF project maintainers",
	Long: `cncf-maintainers validates GitHub usernames against the CNCF
project-maintainers spreadsheet and optionally adds confirmed
maintainers to the cncf-maintainers GitHub organisation team.

Sources:
  CSV  – https://github.com/cncf/foundation/blob/main/project-maintainers.csv
  Team – https://github.com/orgs/cncf-maintainers/teams/cncf-maintainers

Authentication:
  Set the GITHUB_TOKEN environment variable to a Personal Access Token
  with the 'admin:org' scope before running the 'add' command.`,
}

// Execute is the entry point called from main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// readUsernames returns a deduplicated, non-empty list of GitHub usernames
// from either the --username flag or a file specified by --file.
func readUsernames(username, file string) ([]string, error) {
	switch {
	case username != "" && file != "":
		return nil, fmt.Errorf("use either --username or --file, not both")

	case username != "":
		return []string{strings.TrimSpace(username)}, nil

	case file != "":
		return readUsernamesFromFile(file)

	default:
		return nil, fmt.Errorf("one of --username or --file is required")
	}
}

// readUsernamesFromFile reads one GitHub username per line from path.
// Blank lines and lines starting with '#' are ignored.
func readUsernamesFromFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file %q: %w", path, err)
	}
	defer f.Close()

	seen := make(map[string]bool)
	var names []string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lower := strings.ToLower(line)
		if !seen[lower] {
			seen[lower] = true
			names = append(names, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading file %q: %w", path, err)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("file %q contains no usernames", path)
	}
	return names, nil
}
