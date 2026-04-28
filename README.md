# cncf-github-maintainers

[![Release](https://img.shields.io/github/v/release/idvoretskyi/cncf-github-maintainers)](https://github.com/idvoretskyi/cncf-github-maintainers/releases/latest)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

A CLI tool to validate GitHub usernames against the [CNCF project maintainers list](https://github.com/cncf/foundation/blob/main/project-maintainers.csv), add confirmed maintainers to the [cncf-maintainers](https://github.com/orgs/cncf-maintainers/teams/cncf-maintainers) GitHub team, and audit the team membership for drift against the CSV.

## Quick Start

```bash
# Install (macOS and Linux)
brew install idvoretskyi/tap/cncf-maintainers

# Validate a username
cncf-maintainers validate octocat

# Add a confirmed maintainer to the team
cncf-maintainers add octocat
```

## Installation

**Homebrew (macOS and Linux, recommended):**

```bash
brew install idvoretskyi/tap/cncf-maintainers
```

Or tap first, then install:

```bash
brew tap idvoretskyi/tap
brew install cncf-maintainers
```

**One-liner (macOS and Linux):**

```bash
curl -sSL https://raw.githubusercontent.com/idvoretskyi/cncf-github-maintainers/main/install.sh | sh
```

Auto-detects your OS and architecture and installs the binary to `/usr/local/bin`.

**Go install:**

```bash
go install github.com/idvoretskyi/cncf-github-maintainers@latest
```

**Build from source:**

```bash
git clone https://github.com/idvoretskyi/cncf-github-maintainers.git
cd cncf-github-maintainers
git checkout v0.1.0
go build -o cncf-maintainers .
```

## Authentication

A GitHub token with `admin:org` scope is required for the `add` command and for `audit --apply`. Read access to the org/team is sufficient for `audit` in its default (read-only) mode. The tool resolves the token in this order:

| Priority | Source |
|----------|--------|
| 1 | `GITHUB_TOKEN` env var |
| 2 | `GH_TOKEN` env var |
| 3 | `gh auth token` (GitHub CLI) |

The quickest way to authenticate:

```bash
gh auth login -s admin:org
```

The `validate` command does not require a token.

## Usage

```bash
# Validate a single user
cncf-maintainers validate <username>

# Validate multiple users or from a file
cncf-maintainers validate <user1> <user2>
cncf-maintainers validate --file usernames.txt

# Add a confirmed maintainer to the team
cncf-maintainers add <username>

# Preview without making changes
cncf-maintainers add <username> --dry-run

# Bulk add from a file
cncf-maintainers add --file usernames.txt

# Audit the team against the CSV (read-only by default)
cncf-maintainers audit

# Apply removals after a confirmation prompt
cncf-maintainers audit --apply

# Apply removals non-interactively (e.g. in CI)
cncf-maintainers audit --apply --yes
```

Input files accept one username per line. Lines starting with `#` and blank lines are ignored. Duplicates are deduplicated automatically.

## Audit mode

The `audit` command reconciles the GitHub team membership with the CNCF maintainers CSV.

- **Read-only by default.** Without `--apply`, it only prints a report and exits non-zero if the team and CSV are out of sync — useful as a CI guardrail.
- **Removals only.** Users on the team but absent from the CSV are listed (and removed when `--apply` is set).
- **Never auto-invites.** Users who appear in the CSV but are missing from the team are reported only — add them manually with `cncf-maintainers add <username>`.
- **Protects team maintainers.** Users with the `maintainer` role on the team (typically org owners) are never removed.

Exit codes:

| Code | Meaning |
|------|---------|
| 0 | Team membership matches the CSV (or all `--apply` removals succeeded) |
| 1 | Discrepancies detected (read-only mode) or one or more removals failed |

## License

[Apache License 2.0](LICENSE)
