#!/usr/bin/env bash
# Fail unless the macOS signing and notarization secrets are set. Without
# MACOS_SIGN_P12 goreleaser silently builds unsigned darwin binaries, which the
# signed snapshot exists to rule out. Only presence is checked; values are
# never printed. Task release:secrets:check (signed-snapshot.yml).
set -euo pipefail
missing=0
for name in MACOS_SIGN_P12 MACOS_SIGN_PASSWORD MACOS_NOTARY_KEY MACOS_NOTARY_KEY_ID MACOS_NOTARY_ISSUER_ID; do
  if [ -z "${!name:-}" ]; then
    echo "::error::secret ${name} is not set" >&2
    missing=1
  fi
done
exit "$missing"
