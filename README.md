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
# Users with a pending invitation are detected and skipped automatically
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

## Using as a GitHub Action

The tool is also available as a reusable composite GitHub Action. Reference it
in any workflow as `idvoretskyi/cncf-github-maintainers@v0` (floating major tag)
or pin to a specific release such as `@v0.2.0`.

### Inputs

| Input | Required | Default | Description |
|-------|----------|---------|-------------|
| `command` | yes | `audit` | Subcommand: `audit`, `validate`, or `add` |
| `args` | no | `''` | Extra CLI arguments (space-separated) |
| `version` | no | `latest` | Release tag to install (e.g. `v0.2.0`) |
| `github-token` | no | `''` | Token with `admin:org` scope for `add` / `audit --apply`; read access for `audit` preview; not needed for `validate` |
| `fail-on-drift` | no | `true` | For `audit`: when `false`, step succeeds on drift so the workflow can read `exit-code` and decide |

### Outputs

| Output | Description |
|--------|-------------|
| `exit-code` | Exit code returned by the CLI (`0` = success / in-sync, `1` = drift or not found) |

### Supported runners

| OS | amd64 | arm64 |
|----|-------|-------|
| `ubuntu-*` | ✓ | ✓ |
| `macos-*` | ✓ | ✓ |
| `windows-*` | ✗ | ✗ |

### Examples

**Scheduled audit (read-only, fails workflow on drift):**

```yaml
on:
  schedule:
    - cron: '0 12 * * 1'  # Mondays at 12:00 UTC
  workflow_dispatch:

jobs:
  audit:
    runs-on: ubuntu-latest
    steps:
      - uses: idvoretskyi/cncf-github-maintainers@v0
        with:
          command: audit
          github-token: ${{ secrets.CNCF_AUDIT_TOKEN }}  # read access to the team
```

**On-demand audit with automatic removals:**

```yaml
on: workflow_dispatch

jobs:
  audit-apply:
    runs-on: ubuntu-latest
    steps:
      - uses: idvoretskyi/cncf-github-maintainers@v0
        with:
          command: audit
          args: --apply --yes
          github-token: ${{ secrets.ORG_ADMIN_TOKEN }}   # admin:org scope
```

**Validate a GitHub username inside another workflow:**

```yaml
- uses: idvoretskyi/cncf-github-maintainers@v0
  with:
    command: validate
    args: ${{ github.event.pull_request.user.login }}
```

**Audit without failing the step (read exit-code yourself):**

```yaml
- id: audit
  uses: idvoretskyi/cncf-github-maintainers@v0
  with:
    command: audit
    fail-on-drift: 'false'
    github-token: ${{ secrets.CNCF_AUDIT_TOKEN }}

- name: Report drift
  if: steps.audit.outputs.exit-code == '1'
  run: echo "Drift detected — review the audit output above."
```

### Security

- The action downloads a pre-built binary from the GitHub Releases page and
  **verifies its SHA-256 checksum** against `checksums.txt` before executing.
- The `github-token` is passed via an environment variable and never appears
  as a CLI argument or in process listings.
- For sensitive workflows, consider pinning to an immutable commit SHA instead
  of a floating tag (e.g. `uses: idvoretskyi/cncf-github-maintainers@<sha>`).

## License

[Apache License 2.0](LICENSE)
