# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Go CLI (binary name `cncf-maintainers`, module `github.com/idvoretskyi/cncf-github-maintainers`)
that validates GitHub usernames against the CNCF `project-maintainers.csv`, adds confirmed
maintainers to the `cncf-maintainers/cncf-maintainers` GitHub team, and audits team membership
for drift against the CSV. It's also shipped as a composite GitHub Action (`action.yml`).

## Commands

```bash
# Build
go build -o cncf-maintainers .

# Vet
go vet ./...

# Unit/integration tests (there are currently none outside e2e — this compiles but runs nothing)
go test -race ./...

# End-to-end tests: require network access (they hit the live CNCF CSV and, for
# some, the GitHub API) and are gated behind the "e2e" build tag, so CI does NOT run them.
go test -v -tags e2e ./e2e/

# Run a single e2e test
go test -v -tags e2e ./e2e/ -run TestValidate_FileInput

# Local release dry-run (validates .goreleaser.yml without publishing)
goreleaser release --snapshot --clean
```

CI (`.github/workflows/ci.yml`) runs: `go build ./...`, `go vet ./...`, and
`go test -race -coverprofile=... ./...` — no `e2e` tag, so e2e tests never run there.

## Architecture

Layering, top to bottom:

- `main.go` — sets version/commit/date (injected by GoReleaser via ldflags; `dev`/`none`/`unknown`
  on a plain local build) and calls into `cmd`.
- `cmd/` (cobra) — `root.go` (shared arg-parsing helpers), `validate.go`, `add.go`, `audit.go`.
  Each `RunE` fetches the CSV fresh and, for `add`/`audit`, talks to GitHub — there is no caching
  or local fixture; every invocation is a live network call.
- `internal/csv` — downloads and parses `project-maintainers.csv` from
  `raw.githubusercontent.com/cncf/foundation/main`. `FindByGitHubName` does the case-insensitive
  lookup used by all three commands. The CSV uses a "merged cell" convention where `Level` and
  `Project` are blank on continuation rows; the parser carries the last non-blank value forward.
- `internal/github` — thin wrapper (`Client`) around `go-github`'s Teams API: membership lookup,
  list-members-by-role, add, remove. `ListTeamMembers` queries `role=member` and `role=maintainer`
  separately because the GitHub API only returns roles that way (not with `role=all`).
- `internal/config` — `OrgName`/`TeamSlug` constants (single source of truth — don't hardcode
  `"cncf-maintainers"` elsewhere) and `GetGitHubToken()`, which resolves a token via
  `GITHUB_TOKEN` → `GH_TOKEN` → `gh auth token`.

## Conventions worth knowing

- New commands taking username input should reuse `readUsernames`/`splitUsernames`/`dedup` in
  `cmd/root.go` — they already handle `--file`, comma/space/newline-separated input, and
  case-insensitive dedup.
- `validate` and `add --dry-run` require no GitHub token; `add` (real) and `audit --apply` need
  `admin:org` scope; plain `audit` (read-only) needs only read access to the org/team.
- Two separate confirmation helpers exist on purpose: `promptConfirm` in `add.go` defaults to
  **yes** on blank input, while `promptConfirmStrict` in `audit.go` defaults to **no** — used
  for the destructive team-removal path.
- Commands write output via `cmd.OutOrStdout()`, not `fmt.Println`, so tests/tooling can capture it.
- `audit`'s exit code is load-bearing: it returns a non-nil error (exit 1) whenever CSV/team are
  out of sync in read-only mode, which is what CI/cron guardrails and the Action's
  `fail-on-drift` input rely on. `audit` never auto-adds missing users (report only) and never
  removes team members with the `maintainer` role (protects org owners).
- Changing CLI flags or subcommands generally also means updating `action.yml` inputs,
  `.github/action/run.sh`, and the README's usage/Action tables to keep them in sync.
- e2e tests hardcode known-stable CNCF maintainer logins (`thockin`, `liggitt`) as fixtures
  against the live CSV; if they start failing, check whether the CSV upstream still lists them.
