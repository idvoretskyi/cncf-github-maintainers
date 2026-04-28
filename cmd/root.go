package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/idvoretskyi/cncf-github-maintainers/internal/csv"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "cncf-maintainers",
	Short: "Validate and manage CNCF project maintainers",
	Long: `cncf-maintainers validates GitHub usernames against the CNCF
project-maintainers spreadsheet, optionally adds confirmed
maintainers to the cncf-maintainers GitHub organisation team, and
audits the team membership for drift against the CSV.

Sources:
  CSV  – https://github.com/cncf/foundation/blob/main/project-maintainers.csv
  Team – https://github.com/orgs/cncf-maintainers/teams/cncf-maintainers

Authentication:
  Set the GITHUB_TOKEN environment variable to a Personal Access Token
  with the 'admin:org' scope before running the 'add' or 'audit --apply'
  commands.`,
}

// SetVersion injects build-time version information into the root command so
// that `cncf-maintainers --version` prints meaningful output. It is called
// from main.go with variables populated by GoReleaser ldflags.
func SetVersion(version, commit, date string) {
	rootCmd.Version = fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date)
}

// Execute is the entry point called from main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// printMaintainerDetails writes the Name, Company, Project, and optionally
// OWNERS link for a single Maintainer entry to w.
func printMaintainerDetails(w io.Writer, m csv.Maintainer, includeOwners bool) {
	fmt.Fprintf(w, "    Name:    %s\n", m.Name)
	if m.Company != "" {
		fmt.Fprintf(w, "    Company: %s\n", m.Company)
	}
	fmt.Fprintf(w, "    Project: %s (%s)\n", m.Project, strings.ToLower(m.Level))
	if includeOwners && m.OwnersLink != "" {
		fmt.Fprintf(w, "    OWNERS:  %s\n", m.OwnersLink)
	}
}

// readUsernames returns a deduplicated, non-empty list of GitHub usernames
// from positional arguments and/or a file specified by --file.
// Each positional argument may itself contain multiple usernames separated
// by commas, spaces, or newlines (so copy-pasted lists work directly).
// args should be the positional arguments passed to the cobra command.
func readUsernames(args []string, file string) ([]string, error) {
	var positional []string
	for _, arg := range args {
		positional = append(positional, splitUsernames(arg)...)
	}
	hasPositional := len(positional) > 0

	switch {
	case hasPositional && file != "":
		return nil, fmt.Errorf("use either positional username arguments or --file, not both")

	case hasPositional:
		return dedup(positional), nil

	case file != "":
		return readUsernamesFromFile(file)

	default:
		return nil, fmt.Errorf("one or more GitHub usernames or --file is required")
	}
}

// splitUsernames splits a single string on commas, spaces, and newlines,
// returning non-empty tokens. This allows copy-pasted lists to be passed
// as a single quoted argument or as multiple shell words.
func splitUsernames(s string) []string {
	// Normalise commas to spaces, then split on whitespace.
	return strings.Fields(strings.ReplaceAll(s, ",", " "))
}

// dedup returns a case-insensitively deduplicated copy of names,
// preserving the original casing of the first occurrence.
func dedup(names []string) []string {
	seen := make(map[string]bool, len(names))
	out := make([]string, 0, len(names))
	for _, n := range names {
		lower := strings.ToLower(n)
		if !seen[lower] {
			seen[lower] = true
			out = append(out, n)
		}
	}
	return out
}

// readUsernamesFromFile reads one GitHub username per line from path.
// Blank lines and lines starting with '#' are ignored.
// Each line may also contain comma/space-separated usernames.
func readUsernamesFromFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file %q: %w", path, err)
	}
	defer f.Close()

	var names []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		names = append(names, splitUsernames(line)...)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading file %q: %w", path, err)
	}
	names = dedup(names)
	if len(names) == 0 {
		return nil, fmt.Errorf("file %q contains no usernames", path)
	}
	return names, nil
}
