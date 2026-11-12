#!/usr/bin/env bash
# Integration test T15 of the 1Password backend in CI (task
# test:onepassword:ci, macOS workflow). It reads the token of the CI service
# account from OP_SERVICE_ACCOUNT_TOKEN (the workflow sets it from the
# repository secret of the same name), writes it to a fresh token file with
# mode 0600, and runs the same test as test:onepassword. The token file is
# removed on exit.
#
# The script never prints the token: no `set -x`, no echo of the value. The
# masking of secrets in the GitHub log is a second line of defence only.
# A missing secret is a failure, not a skip: a skipped test looks green and
# hides breakage.
set -euo pipefail

if [ -z "${OP_SERVICE_ACCOUNT_TOKEN:-}" ]; then
  echo "FAIL: the repository secret OP_SERVICE_ACCOUNT_TOKEN is missing or empty." >&2
  echo "      The owner sets it once (see docs/reference/55-ci.md):" >&2
  echo "      op service-account create passierschein-ci --vault passierschein.dev:read_items --raw | gh secret set OP_SERVICE_ACCOUNT_TOKEN --repo seletz/passierschein" >&2
  exit 1
fi

dir=$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/a38-op.XXXXXX")
trap 'rm -rf "$dir"' EXIT
umask 077
printf '%s' "$OP_SERVICE_ACCOUNT_TOKEN" > "$dir/onepassword-token"
chmod 600 "$dir/onepassword-token"
# The test and the SDK get the token only through the file.
unset OP_SERVICE_ACCOUNT_TOKEN
export A38_OP_TOKEN_FILE="$dir/onepassword-token"

bash "$(cd "$(dirname "$0")" && pwd)/onepassword-integration.sh"
