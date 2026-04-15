# cncf-github-maintainers

[![Release](https://img.shields.io/github/v/release/idvoretskyi/cncf-github-maintainers)](https://github.com/idvoretskyi/cncf-github-maintainers/releases/latest)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

A CLI tool to validate GitHub usernames against the [CNCF project maintainers list](https://github.com/cncf/foundation/blob/main/project-maintainers.csv) and add confirmed maintainers to the [cncf-maintainers](https://github.com/orgs/cncf-maintainers/teams/cncf-maintainers) GitHub team.

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
go build -o cncf-maintainers .
```

## Authentication

A GitHub token with `admin:org` scope is required for the `add` command. The tool resolves it in this order:

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
```

Input files accept one username per line. Lines starting with `#` and blank lines are ignored. Duplicates are deduplicated automatically.

## License

[Apache License 2.0](LICENSE)
