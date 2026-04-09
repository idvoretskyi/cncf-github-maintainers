package config

import (
	"fmt"
	"os"
)

const (
	// OrgName is the GitHub organization for CNCF maintainers.
	OrgName = "cncf-maintainers"

	// TeamSlug is the team within OrgName where maintainers are added.
	TeamSlug = "cncf-maintainers"

	// MaintainersCSVURL is the raw URL of the CNCF project-maintainers.csv file.
	MaintainersCSVURL = "https://raw.githubusercontent.com/cncf/foundation/main/project-maintainers.csv"

	// CSV column indices (0-based).
	ColLevel      = 0
	ColProject    = 1
	ColName       = 2
	ColCompany    = 3
	ColGitHubName = 4
	ColOwnersLink = 5
)

// GetGitHubToken reads the GitHub personal access token from the environment.
// It returns an error if the token is not set.
func GetGitHubToken() (string, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return "", fmt.Errorf("GITHUB_TOKEN environment variable is not set\n" +
			"Set it with: export GITHUB_TOKEN=<your-pat>")
	}
	return token, nil
}
