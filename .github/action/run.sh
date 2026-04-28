#!/usr/bin/env bash
# .github/action/run.sh
# Downloads the cncf-maintainers binary from GitHub Releases, verifies its
# checksum, and executes the requested subcommand.
#
# Environment variables (set by action.yml):
#   INPUT_COMMAND       – subcommand: audit | validate | add
#   INPUT_ARGS          – extra CLI arguments (space-separated string)
#   INPUT_VERSION       – release tag ("latest" resolves at run time)
#   INPUT_FAIL_ON_DRIFT – "true"/"false" (audit only)
#   GITHUB_TOKEN        – forwarded to the CLI for authenticated commands
#   GITHUB_OUTPUT       – path to the GitHub Actions output file

set -euo pipefail

# ── Helpers ────────────────────────────────────────────────────────────────────

log()  { echo "[cncf-maintainers] $*"; }
die()  { echo "[cncf-maintainers] ERROR: $*" >&2; exit 1; }

# ── Platform detection ─────────────────────────────────────────────────────────

case "$(uname -s)" in
  Linux)  OS="linux"  ;;
  Darwin) OS="darwin" ;;
  *)      die "Unsupported OS: $(uname -s). Only Linux and macOS runners are supported." ;;
esac

case "$(uname -m)" in
  x86_64)          ARCH="amd64" ;;
  aarch64|arm64)   ARCH="arm64" ;;
  *)               die "Unsupported architecture: $(uname -m)." ;;
esac

log "Runner: ${OS}/${ARCH}"

# ── Resolve version ────────────────────────────────────────────────────────────

REPO="idvoretskyi/cncf-github-maintainers"
VERSION="${INPUT_VERSION:-latest}"

if [ "$VERSION" = "latest" ]; then
  log "Resolving latest release tag..."
  VERSION="$(curl -sSfL \
    -H "Accept: application/vnd.github+json" \
    "https://api.github.com/repos/${REPO}/releases/latest" \
    | jq -r '.tag_name')"
  [ -n "$VERSION" ] || die "Could not resolve latest release tag."
fi

log "Installing cncf-maintainers ${VERSION}"

# ── Download & verify ──────────────────────────────────────────────────────────

TARBALL="cncf-maintainers_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

curl -sSfL "${BASE_URL}/${TARBALL}"       -o "${TMP}/${TARBALL}"
curl -sSfL "${BASE_URL}/checksums.txt"    -o "${TMP}/checksums.txt"

log "Verifying checksum..."
cd "$TMP"
if command -v sha256sum &>/dev/null; then
  grep "${TARBALL}" checksums.txt | sha256sum -c --status \
    || die "Checksum verification failed for ${TARBALL}."
elif command -v shasum &>/dev/null; then
  grep "${TARBALL}" checksums.txt | shasum -a 256 -c --status \
    || die "Checksum verification failed for ${TARBALL}."
else
  die "Neither sha256sum nor shasum found on this runner."
fi
cd - > /dev/null

log "Checksum OK"

# ── Extract binary ─────────────────────────────────────────────────────────────

tar -xzf "${TMP}/${TARBALL}" -C "$TMP" cncf-maintainers
BIN="${TMP}/cncf-maintainers"
chmod +x "$BIN"

# ── Build argument array (safe word-splitting, no eval) ───────────────────────

CMD="${INPUT_COMMAND:-audit}"
read -ra ARGS_ARR <<< "${INPUT_ARGS:-}"

# ── Execute ────────────────────────────────────────────────────────────────────

log "Running: cncf-maintainers ${CMD} ${ARGS_ARR[*]+"${ARGS_ARR[*]}"}"
set +e
"$BIN" "$CMD" "${ARGS_ARR[@]+"${ARGS_ARR[@]}"}"
rc=$?
set -e

# ── Emit output ────────────────────────────────────────────────────────────────

if [ -n "${GITHUB_OUTPUT:-}" ]; then
  echo "exit-code=${rc}" >> "$GITHUB_OUTPUT"
fi

# ── Exit-code policy ───────────────────────────────────────────────────────────
# For 'audit' with fail-on-drift=false, exit code 1 means "drift detected"
# (not an error). The step should succeed; callers read the exit-code output.
# Any other non-zero exit, or non-audit commands, propagate as failures.

FAIL_ON_DRIFT="${INPUT_FAIL_ON_DRIFT:-true}"

if [ "$CMD" = "audit" ] && [ "$FAIL_ON_DRIFT" = "false" ] && [ "$rc" -eq 1 ]; then
  log "Drift detected (exit code 1) but fail-on-drift=false — marking step as success."
  exit 0
fi

exit "$rc"
