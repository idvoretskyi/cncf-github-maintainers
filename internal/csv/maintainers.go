package csv

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/idvoretskyi/cncf-github-maintainers/internal/config"
)

// Maintainer represents a single row from the CNCF project-maintainers.csv.
// The Level and Project fields are propagated forward when they are blank in
// subsequent rows (the CSV uses a "merged cell" convention).
type Maintainer struct {
	Level      string // e.g. "Graduated", "Incubating", "Sandbox"
	Project    string // e.g. "Kubernetes maintainers"
	Name       string // Full name
	Company    string
	GitHubName string // GitHub username (column "Github Name")
	OwnersLink string // URL to OWNERS/MAINTAINERS file
}

// FetchMaintainers downloads the CNCF project-maintainers.csv and returns
// a slice of parsed Maintainer entries.  It retries once on transient errors.
func FetchMaintainers(ctx context.Context) ([]Maintainer, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		maintainers, err := fetchAndParse(ctx)
		if err == nil {
			return maintainers, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("fetching CNCF maintainers CSV: %w", lastErr)
}

func fetchAndParse(ctx context.Context) ([]Maintainer, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, config.MaintainersCSVURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status %d fetching CSV", resp.StatusCode)
	}

	return parseCSV(resp.Body)
}

// parseCSV reads the CSV content and returns Maintainer entries.
// It handles the "carry-forward" pattern where Level and Project columns
// are empty for rows that belong to the same project.
func parseCSV(r io.Reader) ([]Maintainer, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1 // allow variable number of fields
	reader.LazyQuotes = true

	var (
		maintainers   []Maintainer
		currentLevel  string
		currentProj   string
		headerSkipped bool
	)

	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parsing CSV: %w", err)
		}

		// Skip the header row (first non-empty record whose first field is empty
		// and second field is "Project").
		if !headerSkipped {
			headerSkipped = true
			// The real header row starts with an empty cell then "Project"
			if len(record) > 1 && strings.TrimSpace(record[1]) == "Project" {
				continue
			}
		}

		// Ensure we have at least the GitHubName column.
		if len(record) <= config.ColGitHubName {
			continue
		}

		// Carry forward Level and Project when the columns are blank.
		if lv := strings.TrimSpace(record[config.ColLevel]); lv != "" {
			currentLevel = lv
		}
		if proj := strings.TrimSpace(record[config.ColProject]); proj != "" {
			currentProj = proj
		}

		ghName := strings.TrimSpace(record[config.ColGitHubName])
		if ghName == "" {
			continue
		}

		var ownersLink string
		if len(record) > config.ColOwnersLink {
			ownersLink = strings.TrimSpace(record[config.ColOwnersLink])
		}

		maintainers = append(maintainers, Maintainer{
			Level:      currentLevel,
			Project:    currentProj,
			Name:       strings.TrimSpace(record[config.ColName]),
			Company:    strings.TrimSpace(record[config.ColCompany]),
			GitHubName: ghName,
			OwnersLink: ownersLink,
		})
	}

	if len(maintainers) == 0 {
		return nil, fmt.Errorf("CSV parsed successfully but contained no maintainer entries")
	}

	return maintainers, nil
}

// FindByGitHubName returns all Maintainer entries whose GitHubName matches
// the given username (case-insensitive).  A person can be listed across
// multiple projects.
func FindByGitHubName(maintainers []Maintainer, username string) []Maintainer {
	needle := strings.ToLower(strings.TrimSpace(username))
	var results []Maintainer
	for _, m := range maintainers {
		if strings.ToLower(strings.TrimSpace(m.GitHubName)) == needle {
			results = append(results, m)
		}
	}
	return results
}
