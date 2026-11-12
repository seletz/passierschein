#!/usr/bin/env bash
# Notarize the signed release bundle and staple the ticket (task
# release:app:notarize, macOS only, CI).
#
# MACOS_NOTARY_KEY (base64 .p8 of the App Store Connect API key),
# MACOS_NOTARY_KEY_ID and MACOS_NOTARY_ISSUER_ID come from the environment.
# The key goes into a file with mode 0600 under $RUNNER_TEMP, which a trap
# removes. The script zips the bundle with ditto (Apple's way), submits it
# with notarytool, waits, prints only the submission ID, the status and the
# issues of the log (paths and messages), staples the ticket to the bundle
# and validates it.
set -euo pipefail
. "$(dirname "$0")/release-app-lib.sh"

[ "$(uname -s)" = Darwin ] || die "release:app:notarize runs on macOS only"
[ -d "$BUNDLE" ] || die "$BUNDLE is missing; run release:app and release:app:sign first"
missing=()
for name in MACOS_NOTARY_KEY MACOS_NOTARY_KEY_ID MACOS_NOTARY_ISSUER_ID; do
  [ -n "${!name:-}" ] || missing+=("$name")
done
[ "${#missing[@]}" = 0 ] || die "secret(s) not set: ${missing[*]}; an app that is not notarized is never released (docs/how-to/60-release-signieren-und-pruefen.md)"

umask 077
tmp=$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/a38-notary.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
printf '%s' "$MACOS_NOTARY_KEY" | base64 --decode > "$tmp/AuthKey.p8" 2>/dev/null ||
  die "MACOS_NOTARY_KEY is not valid base64"
[ -s "$tmp/AuthKey.p8" ] || die "MACOS_NOTARY_KEY decodes to an empty file"
auth=(--key "$tmp/AuthKey.p8" --key-id "$MACOS_NOTARY_KEY_ID" --issuer "$MACOS_NOTARY_ISSUER_ID")

ditto -c -k --keepParent "$BUNDLE" "$tmp/submit.zip"
echo "release-app-notarize: submit $(basename "$BUNDLE") ($(du -h "$tmp/submit.zip" | cut -f1)), wait up to 30 minutes"
xcrun notarytool submit "$tmp/submit.zip" "${auth[@]}" --wait --timeout 30m \
  --output-format json > "$tmp/submit.json" 2> "$tmp/submit.err" || true
id=$(jq -r '.id // empty' "$tmp/submit.json" 2>/dev/null || true)
status=$(jq -r '.status // empty' "$tmp/submit.json" 2>/dev/null || true)
echo "release-app-notarize: submission ${id:-?}, status ${status:-?}"
if [ -z "$id" ]; then
  # notarytool failed before a submission: show its message. It names
  # the problem (for example an invalid key), not the key.
  sed -E 's/[A-Za-z0-9+\/=]{40,}/[redacted]/g' "$tmp/submit.err" | tail -n 5 >&2
  die "the submission to the notary service failed"
fi
if xcrun notarytool log "$id" "${auth[@]}" "$tmp/log.json" >/dev/null 2>&1; then
  jq -c '{status, statusSummary, issues: [(.issues // [])[] | {severity, path, message}]}' "$tmp/log.json"
fi
[ "$status" = Accepted ] || die "the notary service did not accept the bundle (status ${status:-?})"

xcrun stapler staple "$BUNDLE"
xcrun stapler validate "$BUNDLE"
echo "release-app-notarize: notarized and stapled"
