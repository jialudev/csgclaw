#!/usr/bin/env bash
#
# Local upgrade test helper for csgclaw.
#
# Usage:
#   scripts/test-upgrade-local.sh [options]
#
# Options:
#   --version <semver>  Build the local test bundle with this version.
#                       Default: v0.0.1
#   --with-daemon       Also test `csgclaw upgrade` with daemon restart.
#   --keep-temp         Keep the temporary workspace after the script exits.
#   --skip-package      Reuse an existing archive in dist/ and skip packaging.
#   --help              Show command help.
#
# What it does:
#   1. Runs `make package VERSION=<semver>` unless `--skip-package` is used.
#   2. Extracts the generated bundle into a temporary directory.
#   3. Runs:
#        - `csgclaw --version`
#        - `csgclaw upgrade --check`
#        - `csgclaw upgrade --no-restart`
#   4. Optionally starts a daemon under an isolated HOME and runs
#      `csgclaw upgrade` to verify the restart path.
#
# Safety notes:
#   - The script uses an isolated temp directory as the install root.
#   - The daemon test uses an isolated HOME, so it will not touch your real
#     `~/.csgclaw/server.pid`.
#   - `upgrade` still talks to the real release endpoint unless your
#     environment redirects it elsewhere.
#
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/test-upgrade-local.sh [options]

Options:
  --version <semver>      Local package version to build for the test bundle.
                          Default: v0.0.1
  --with-daemon           Also test `csgclaw upgrade` with daemon restart.
  --keep-temp             Keep the temporary workspace instead of deleting it.
  --skip-package          Reuse an existing archive in dist/ and skip packaging.
  --help                  Show this help message.

Behavior:
  1. Build a local release bundle with a semver version.
  2. Extract it into a temporary directory.
  3. Run:
       - `csgclaw --version`
       - `csgclaw upgrade --check`
       - `csgclaw upgrade --no-restart`
  4. Optionally start a daemon under an isolated HOME and run `csgclaw upgrade`.

Notes:
  - The script uses an isolated temp directory and isolated HOME for safety.
  - `upgrade` still fetches the latest release metadata and bundle from the real
    release endpoint unless your environment overrides that behavior elsewhere.
EOF
}

log() {
  printf '\n==> %s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

VERSION="v0.0.1"
WITH_DAEMON=0
KEEP_TEMP=0
SKIP_PACKAGE=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      [[ $# -ge 2 ]] || fail "--version requires a value"
      VERSION="$2"
      shift 2
      ;;
    --with-daemon)
      WITH_DAEMON=1
      shift
      ;;
    --keep-temp)
      KEEP_TEMP=1
      shift
      ;;
    --skip-package)
      SKIP_PACKAGE=1
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      fail "unknown option: $1"
      ;;
  esac
done

case "$VERSION" in
  v[0-9]*.[0-9]*.[0-9]*|[0-9]*.[0-9]*.[0-9]*) ;;
  *)
    fail "version must be a semver like v0.0.1"
    ;;
esac

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GOOS="$(go env GOOS)"
GOARCH="$(go env GOARCH)"
ARCHIVE_BASENAME="csgclaw_${VERSION}_${GOOS}_${GOARCH}.tar.gz"
ARCHIVE_PATH="${ROOT_DIR}/dist/${ARCHIVE_BASENAME}"

TMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/csgclaw-upgrade-test.XXXXXX")"
WORK_DIR="${TMP_ROOT}/work"
HOME_DIR="${TMP_ROOT}/home"
INSTALL_ROOT="${WORK_DIR}/csgclaw"
BIN_PATH="${INSTALL_ROOT}/bin/csgclaw"

cleanup() {
  if [[ "$KEEP_TEMP" -eq 1 ]]; then
    printf '\nkept temp dir: %s\n' "$TMP_ROOT"
    return
  fi
  rm -rf "$TMP_ROOT"
}
trap cleanup EXIT

mkdir -p "$WORK_DIR" "$HOME_DIR"

if [[ "$SKIP_PACKAGE" -eq 0 ]]; then
  log "Packaging local bundle as ${VERSION}"
  make -C "$ROOT_DIR" package VERSION="$VERSION"
fi

[[ -f "$ARCHIVE_PATH" ]] || fail "archive not found: $ARCHIVE_PATH"

log "Extracting ${ARCHIVE_BASENAME}"
tar -xzf "$ARCHIVE_PATH" -C "$WORK_DIR"

[[ -x "$BIN_PATH" ]] || fail "binary not found after extraction: $BIN_PATH"

log "Printing bundled version"
"$BIN_PATH" --version

log "Running upgrade --check"
"$BIN_PATH" upgrade --check

log "Running upgrade --no-restart"
"$BIN_PATH" upgrade --no-restart

if [[ "$WITH_DAEMON" -eq 1 ]]; then
  log "Starting daemon with isolated HOME at ${HOME_DIR}"
  HOME="$HOME_DIR" "$BIN_PATH" serve --daemon

  log "Running upgrade with automatic restart"
  HOME="$HOME_DIR" "$BIN_PATH" upgrade

  log "Stopping daemon"
  HOME="$HOME_DIR" "$BIN_PATH" stop || true
fi

log "Done"
printf 'install root: %s\n' "$INSTALL_ROOT"
printf 'isolated home: %s\n' "$HOME_DIR"
