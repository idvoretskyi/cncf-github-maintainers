# cncf-github-maintainers

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

A CLI tool that validates GitHub usernames against the
[CNCF project maintainers list](https://github.com/cncf/foundation/blob/main/project-maintainers.csv)
and adds confirmed maintainers to the
[cncf-maintainers](https://github.com/orgs/cncf-maintainers/teams/cncf-maintainers) GitHub team.

## Features

- **Validate** -- check whether a GitHub username is an active CNCF project maintainer
- **Add** -- validate and then add the user to the `cncf-maintainers/cncf-maintainers` team
- **Bulk mode** -- process a file of usernames (one per line)
- **Dry-run** -- preview what `add` would do without making changes
- Multi-project detection (shows all projects a person maintains)
- Case-insensitive username matching

## Prerequisites

- Go 1.26+
- A GitHub token with `admin:org` scope (required for the `add` command)

### Authentication

The tool looks for a GitHub token using the following fallback chain:

| Priority | Source | Notes |
|----------|--------|-------|
| 1 | `GITHUB_TOKEN` env var | Traditional / CI-friendly approach |
| 2 | `GH_TOKEN` env var | Also respected by the GitHub CLI |
| 3 | `gh auth token` | Reads the token from your local GitHub CLI config |

The easiest way to authenticate locally is to use the [GitHub CLI](https://cli.github.com/):

```bash
# Log in (first time) — include the admin:org scope for the "add" command
gh auth login -s admin:org

# If you already have gh installed and logged in, add the scope:
gh auth refresh -s admin:org
```

The `validate` command does not require a token.

## Installation

```bash
# Clone and build
git clone https://github.com/idvoretskyi/cncf-github-maintainers.git
cd cncf-github-maintainers
go build -o cncf-maintainers .

# Or install directly
go install github.com/idvoretskyi/cncf-github-maintainers@latest
```

## Usage

### Validate a username

```bash
# Single user
cncf-maintainers validate --username <github-username>

# Bulk from file
cncf-maintainers validate --file usernames.txt
```

Example output:

```
[✓] <github-username> -- confirmed CNCF maintainer
    Name:    Jane Doe
    Company: Example Corp
    Project: projectname (graduated)
```

### Add a maintainer to the team

```bash
# If using gh CLI auth, no extra setup is needed.
# Otherwise, set your token explicitly:
export GITHUB_TOKEN=ghp_...

# Single user
cncf-maintainers add --username <github-username>

# Dry-run (validate only, no changes)
cncf-maintainers add --username <github-username> --dry-run

# Bulk from file
cncf-maintainers add --file usernames.txt

# Bulk dry-run
cncf-maintainers add --file usernames.txt --dry-run
```

### Input file format

One GitHub username per line. Blank lines and lines starting with `#` are ignored.
Duplicates are automatically deduplicated.

```
# Project A maintainers
username1
username2

# Project B maintainers
username3
```

## How it works

1. Fetches [`project-maintainers.csv`](https://github.com/cncf/foundation/blob/main/project-maintainers.csv) from the `cncf/foundation` repository
2. Parses the CSV (handling the carry-forward pattern for Level and Project columns)
3. Matches the input username(s) against the `Github Name` column (case-insensitive)
4. For the `add` command: calls the GitHub API to add validated users to the `cncf-maintainers/cncf-maintainers` team

## Project structure

```
.
├── main.go                      # Entry point
├── cmd/
│   ├── root.go                  # Root command, shared helpers
│   ├── validate.go              # "validate" subcommand
│   └── add.go                   # "add" subcommand with --dry-run
└── internal/
    ├── config/config.go         # Constants, token resolution (env / gh CLI)
    ├── csv/maintainers.go       # CSV fetch, parse, lookup
    └── github/team.go           # GitHub team membership API
```

## License

[Apache License 2.0](LICENSE)
