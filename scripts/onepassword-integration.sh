#!/usr/bin/env bash
# Integration test T15 of the 1Password backend (task test:onepassword).
# macOS only, opt-in. It reads only, and only from the vault
# passierschein.dev. A38_OP_TOKEN_FILE names the token file of the service
# account (mode 0600). The script never prints the token or a value.
# Optional: A38_OP_EXPECT_SHA256 is the SHA-256 (hex) of the password of
# the item datenbank-dev; the test then compares it.
set -euo pipefail

echo "1Password integration test (T15)"
echo "  vault:   passierschein.dev (the only vault the test touches)"
echo "  access:  read only; the test creates, changes and deletes nothing"
echo "  output:  no token and no value; each check prints PASS or FAIL"

if [ "$(uname -s)" != "Darwin" ]; then
  echo "FAIL: the 1Password backend is part of the macOS build only" >&2
  exit 1
fi
if [ -z "${A38_OP_TOKEN_FILE:-}" ]; then
  echo "FAIL: set A38_OP_TOKEN_FILE to the token file, for example:" >&2
  echo "  A38_OP_TOKEN_FILE=~/.config/a38/onepassword-token mise run test:onepassword" >&2
  exit 1
fi
echo "  token:   ${A38_OP_TOKEN_FILE} (content not shown)"
echo

# -v shows each check; the test logs only names, codes and PASS/FAIL.
go test -tags opintegration -count=1 -v -run '^TestIntegration$' ./internal/backend/onepassword/
