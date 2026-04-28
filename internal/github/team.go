package github

import (
	"context"
	"errors"
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

// MembershipState describes a user's current membership state on a GitHub team.
type MembershipState int

const (
	// MembershipNone means the user has no membership record on the team
	// (they have never been invited or have been removed).
	MembershipNone MembershipState = iota

	// MembershipPending means the user has been invited to the team but has
	// not yet accepted the invitation.
	MembershipPending

	// MembershipActive means the user is a confirmed, active team member.
	MembershipActive
)

// GetTeamMembership returns the MembershipState for username on the given org
// team.  A 404 response is treated as MembershipNone (not an error).
func (c *Client) GetTeamMembership(ctx context.Context, org, teamSlug, username string) (MembershipState, error) {
	membership, resp, err := c.gh.Teams.GetTeamMembershipBySlug(ctx, org, teamSlug, username)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return MembershipNone, nil
		}
		return MembershipNone, fmt.Errorf("checking team membership for %q: %w", username, unwrapGitHubError(err))
	}
	switch membership.GetState() {
	case "active":
		return MembershipActive, nil
	case "pending":
		return MembershipPending, nil
	default:
		return MembershipNone, nil
	}
}

// TeamMember represents a single member of a GitHub team along with the role
// they hold on that team ("member" or "maintainer").
type TeamMember struct {
	Login string
	Role  string // "member" or "maintainer"
}

// ListTeamMembers returns all members of the given org team along with their
// role on that team.  It paginates through the GitHub API as needed.
//
// The "maintainer" role identifies team admins (typically org owners) who
// must be protected from automated removal during audits.
func (c *Client) ListTeamMembers(ctx context.Context, org, teamSlug string) ([]TeamMember, error) {
	var members []TeamMember

	// We must list members per role because the GitHub API returns logins only
	// (without role) when role="all". Querying each role separately lets us
	// reliably distinguish maintainers from regular members.
	for _, role := range []string{"member", "maintainer"} {
		opts := &github.TeamListTeamMembersOptions{
			Role:        role,
			ListOptions: github.ListOptions{PerPage: 100},
		}
		for {
			users, resp, err := c.gh.Teams.ListTeamMembersBySlug(ctx, org, teamSlug, opts)
			if err != nil {
				return nil, fmt.Errorf("listing %s team members for %s/%s: %w", role, org, teamSlug, unwrapGitHubError(err))
			}
			for _, u := range users {
				members = append(members, TeamMember{Login: u.GetLogin(), Role: role})
			}
			if resp.NextPage == 0 {
				break
			}
			opts.Page = resp.NextPage
		}
	}

	return members, nil
}

// RemoveFromTeam removes username from the given org team. It does not remove
// the user from the organisation itself — only their team membership.
func (c *Client) RemoveFromTeam(ctx context.Context, org, teamSlug, username string) error {
	resp, err := c.gh.Teams.RemoveTeamMembershipBySlug(ctx, org, teamSlug, username)
	if err != nil {
		if resp != nil {
			switch resp.StatusCode {
			case http.StatusForbidden:
				return fmt.Errorf("permission denied removing %q: ensure your token has 'admin:org' scope (%w)", username, unwrapGitHubError(err))
			case http.StatusNotFound:
				// Already not a member — treat as success.
				return nil
			}
		}
		return fmt.Errorf("removing %q from team %s/%s: %w", username, org, teamSlug, unwrapGitHubError(err))
	}
	return nil
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
	var ghErr *github.ErrorResponse
	if errors.As(err, &ghErr) {
		if ghErr.Message != "" {
			// Include rate-limit information when relevant.
			if ghErr.Response != nil && ghErr.Response.StatusCode == http.StatusForbidden {
				return fmt.Errorf("%s (HTTP 403 – check token scopes and org permissions)", ghErr.Message)
			}
			return errors.New(ghErr.Message)
		}
	}
	return err
}
