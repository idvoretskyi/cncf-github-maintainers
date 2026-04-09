package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/v62/github"
	"golang.org/x/oauth2"
)

// Client wraps the go-github client with convenience methods for team management.
type Client struct {
	gh *github.Client
}

// NewClient creates an authenticated GitHub API client using the provided token.
func NewClient(token string) *Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	return &Client{gh: github.NewClient(tc)}
}

// IsTeamMember reports whether username is already an active member of the
// given org team.  It returns false (not an error) when the user is simply
// not a member yet.
func (c *Client) IsTeamMember(ctx context.Context, org, teamSlug, username string) (bool, error) {
	membership, resp, err := c.gh.Teams.GetTeamMembershipBySlug(ctx, org, teamSlug, username)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return false, fmt.Errorf("checking team membership for %q: %w", username, unwrapGitHubError(err))
	}
	// A membership can be "pending" (invited but not accepted) or "active".
	return membership.GetState() == "active", nil
}

// AddToTeam adds username to the given org team as a regular member.
// If the user is not yet a GitHub organisation member they will receive an
// invitation email; if they are already a member they will be added directly.
func (c *Client) AddToTeam(ctx context.Context, org, teamSlug, username string) error {
	opts := &github.TeamAddTeamMembershipOptions{Role: "member"}
	_, resp, err := c.gh.Teams.AddTeamMembershipBySlug(ctx, org, teamSlug, username, opts)
	if err != nil {
		if resp != nil {
			switch resp.StatusCode {
			case http.StatusUnprocessableEntity:
				return fmt.Errorf("cannot add %q: the user may not exist on GitHub or the org has restrictions (%w)", username, unwrapGitHubError(err))
			case http.StatusForbidden:
				return fmt.Errorf("permission denied adding %q: ensure your token has 'admin:org' scope (%w)", username, unwrapGitHubError(err))
			}
		}
		return fmt.Errorf("adding %q to team %s/%s: %w", username, org, teamSlug, unwrapGitHubError(err))
	}
	return nil
}

// unwrapGitHubError extracts the human-readable message from a *github.ErrorResponse
// when available, otherwise returns the original error.
func unwrapGitHubError(err error) error {
	if ghErr, ok := err.(*github.ErrorResponse); ok {
		if ghErr.Message != "" {
			// Include rate-limit information when relevant.
			if ghErr.Response != nil && ghErr.Response.StatusCode == http.StatusForbidden {
				return fmt.Errorf("%s (HTTP 403 – check token scopes and org permissions)", ghErr.Message)
			}
			return fmt.Errorf("%s", ghErr.Message)
		}
	}
	return err
}
