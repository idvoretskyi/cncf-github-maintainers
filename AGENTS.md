# AGENTS.md

Instructions for OpenCode (and other coding agents) working in this repo.

## What this is

A Go CLI (binary `cncf-maintainers`, module `github.com/idvoretskyi/cncf-github-maintainers`)
that validates GitHub usernames against the CNCF `project-maintainers.csv`, adds confirmed
maintainers to the `cncf-maintainers/cncf-maintainers` GitHub team, and audits team membership
for drift against the CSV. Also shipped as a composite GitHub Action (`action.yml`).

## Commands

```bash
go build -o cncf-maintainers .    # build
go vet ./...                      # vet
go test -race ./...               # unit tests (currently none outside e2e; compiles, runs nothing)

# e2e tests: require network (hit the live CNCF CSV and, for some, the GitHub API).
# Gated behind the "e2e" build tag, so CI does NOT run them.
go test -v -tags e2e ./e2e/
go test -v -tags e2e ./e2e/ -run TestValidate_FileInput   # single test

goreleaser release --snapshot --clean   # local release dry-run, no publish
```

CI (`.github/workflows/ci.yml`) runs only: `go build ./...`, `go vet ./...`,
`go test -race -coverprofile=... ./...` — no `e2e` tag.

## Architecture

- `main.go` — sets version/commit/date via ldflags (injected by GoReleaser;
  `dev`/`none`/`unknown` on a plain local build), calls into `cmd`.
- `cmd/` (cobra) — `root.go` (shared arg-parsing), `validate.go`, `add.go`, `audit.go`.
  Each `RunE` fetches the CSV fresh and, for `add`/`audit`, talks to GitHub live — no caching
  or local fixtures.
- `internal/csv` — downloads/parses `project-maintainers.csv` from
  `raw.githubusercontent.com/cncf/foundation/main`. `FindByGitHubName` does case-insensitive
  lookup. CSV uses a "merged cell" convention: `Level`/`Project` are blank on continuation
  rows; the parser carries the last non-blank value forward.
- `internal/github` — wrapper (`Client`) around `go-github`'s Teams API. `ListTeamMembers`
  queries `role=member` and `role=maintainer` separately because the GitHub API only returns
  roles that way (not with `role=all`).
- `internal/config` — `OrgName`/`TeamSlug` constants (single source of truth — don't hardcode
  `"cncf-maintainers"` elsewhere) and `GetGitHubToken()`, resolved via
  `GITHUB_TOKEN` → `GH_TOKEN` → `gh auth token`.

## Conventions and gotchas

- New commands taking username input should reuse `readUsernames`/`splitUsernames`/`dedup` in
  `cmd/root.go` — they already handle `--file`, comma/space/newline-separated input, and
  case-insensitive dedup.
- Token scopes: `validate` and `add --dry-run` need no token; `add` (real) and `audit --apply`
  need `admin:org`; plain `audit` (read-only) needs only read access to the org/team.
- Two confirmation helpers exist on purpose: `promptConfirm` in `add.go` defaults to **yes**
  on blank input; `promptConfirmStrict` in `audit.go` defaults to **no** — used for the
  destructive team-removal path.
- Write output via `cmd.OutOrStdout()`, not `fmt.Println`, so tests/tooling can capture it.
- `audit`'s exit code is load-bearing: it returns a non-nil error (exit 1) whenever CSV/team
  are out of sync in read-only mode — this drives CI/cron guardrails and the Action's
  `fail-on-drift` input. `audit` never auto-adds missing users (report only) and never removes
  team members with the `maintainer` role (protects org owners).
- Changing CLI flags/subcommands generally also means updating `action.yml` inputs,
  `.github/action/run.sh`, and the README's usage/Action tables to keep them in sync.
- e2e tests hardcode known-stable CNCF maintainer logins (`thockin`, `liggitt`) as fixtures
  against the live CSV; if they start failing, check whether the CSV upstream still lists them.
