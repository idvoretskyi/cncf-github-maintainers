package config

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	// OrgName is the GitHub organization for CNCF maintainers.
	OrgName = "cncf-maintainers"

	// TeamSlug is the team within OrgName where maintainers are added.
	TeamSlug = "cncf-maintainers"
)

// GetGitHubToken returns a GitHub personal access token using the following
// fallback chain:
//
//  1. GITHUB_TOKEN environment variable
//  2. GH_TOKEN environment variable (also recognised by the gh CLI)
//  3. Token from the local GitHub CLI config (`gh auth token`)
//
// When the token is obtained from the gh CLI, a reminder is printed to stderr
// because the token may not carry the 'admin:org' scope required by the add
// command.
func GetGitHubToken() (string, error) {
	// 1. GITHUB_TOKEN — the traditional / CI-friendly approach.
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token, nil
	}

	// 2. GH_TOKEN — respected by the gh CLI and many GitHub tools.
	if token := os.Getenv("GH_TOKEN"); token != "" {
		return token, nil
	}

	// 3. Fall back to the locally-installed gh CLI.
	if token, err := tokenFromGHCLI(); err == nil && token != "" {
		fmt.Fprintln(os.Stderr,
			"Note: using token from gh CLI — ensure it has 'admin:org' scope\n"+
				"      (run: gh auth refresh -s admin:org   to add the scope)")
		return token, nil
	}

	return "", fmt.Errorf("no GitHub token found\n" +
		"Provide a token using one of the following methods:\n" +
		"  1. export GITHUB_TOKEN=<your-pat>\n" +
		"  2. export GH_TOKEN=<your-pat>\n" +
		"  3. gh auth login  (GitHub CLI)")
}

// tokenFromGHCLI attempts to retrieve the current OAuth token from the gh CLI.
func tokenFromGHCLI() (string, error) {
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
