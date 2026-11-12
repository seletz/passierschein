#!/usr/bin/env bash
# Fail unless the macOS signing and notarization secrets are set, and name
# each missing one. Without MACOS_SIGN_P12 goreleaser silently builds
# unsigned darwin binaries, and the app bundle cannot be signed: the release
# and the signed snapshot must stop here instead. Only presence is checked;
# values are never printed. Task release:secrets:check (release.yml,
# signed-snapshot.yml).
set -euo pipefail
missing=0
for name in MACOS_SIGN_P12 MACOS_SIGN_PASSWORD MACOS_NOTARY_KEY MACOS_NOTARY_KEY_ID MACOS_NOTARY_ISSUER_ID; do
  if [ -z "${!name:-}" ]; then
    echo "::error::secret ${name} is not set" >&2
    missing=1
  fi
done
if [ "$missing" = 1 ]; then
  echo "::error::a release needs all five secrets; an unsigned app is never released. Set them as in docs/how-to/60-release-signieren-und-pruefen.md" >&2
else
  echo "all five signing and notarization secrets are set"
fi
exit "$missing"
