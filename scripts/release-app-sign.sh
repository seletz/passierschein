#!/usr/bin/env bash
# Sign the release bundle (task release:app:sign, macOS only).
#
# Developer ID (default; the release and the signed snapshot):
#   MACOS_SIGN_P12 (base64 PKCS#12 with the "Developer ID Application"
#   certificate and its key) and MACOS_SIGN_PASSWORD come from the
#   environment. The script imports the certificate into a new temporary
#   keychain under $RUNNER_TEMP (never the login keychain), signs, verifies
#   and deletes the keychain and the files in a trap. The search list of
#   keychains is restored.
# Ad-hoc (APP_SIGN_ADHOC=1; pull requests, develop, local tests):
#   the same steps with the identity "-". Never on a tag.
#
# Order: nested code first, then the bundle (Apple: sign inside out; no
# --deep for signing). Both with the hardened runtime, a secure timestamp
# and a fixed identifier. No entitlements: the app and a38 need none, and
# get-task-allow is never set.
#
# The script prints no secret: no set -x, the values go only through
# environment variables and files with mode 0600. It prints the team ID and
# the SHA-1 of the certificate, not its subject.
set -euo pipefail
. "$(dirname "$0")/release-app-lib.sh"

[ "$(uname -s)" = Darwin ] || die "release:app:sign runs on macOS only"
[ -d "$BUNDLE/Contents/MacOS" ] || die "$BUNDLE is missing; run mise run release:app first"
for f in Passierschein a38; do
  [ -f "$BUNDLE/Contents/MacOS/$f" ] || die "$BUNDLE/Contents/MacOS/$f is missing"
done

sign_args=()
if [ "${APP_SIGN_ADHOC:-}" = 1 ]; then
  case "${GITHUB_REF:-}" in
    refs/tags/*) die "APP_SIGN_ADHOC is not allowed for a tag: a release is never ad-hoc signed" ;;
  esac
  echo "release-app-sign: ad-hoc signature (APP_SIGN_ADHOC=1), no Developer ID"
  sign_args=(--sign - --timestamp=none)
else
  missing=()
  for name in MACOS_SIGN_P12 MACOS_SIGN_PASSWORD; do
    [ -n "${!name:-}" ] || missing+=("$name")
  done
  [ "${#missing[@]}" = 0 ] || die "secret(s) not set: ${missing[*]}; an unsigned app is never released (docs/how-to/60-release-signieren-und-pruefen.md)"

  umask 077
  tmp=$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/a38-sign.XXXXXX")
  keychain="$tmp/signing.keychain-db"
  old_keychains=$(security list-keychains -d user | sed -E 's/^[[:space:]]*"(.*)"$/\1/')
  cleanup() {
    # Restore the search list first, then delete the keychain and files.
    # shellcheck disable=SC2086 # one path per line, no blanks in the paths
    security list-keychains -d user -s $old_keychains >/dev/null 2>&1 || true
    security delete-keychain "$keychain" >/dev/null 2>&1 || true
    rm -rf "$tmp"
  }
  trap cleanup EXIT

  printf '%s' "$MACOS_SIGN_P12" | base64 --decode > "$tmp/cert.p12" 2>/dev/null ||
    die "MACOS_SIGN_P12 is not valid base64"
  [ -s "$tmp/cert.p12" ] || die "MACOS_SIGN_P12 decodes to an empty file"
  keychain_password=$(openssl rand -hex 32)

  security create-keychain -p "$keychain_password" "$keychain"
  security set-keychain-settings -lut 3600 "$keychain"
  security unlock-keychain -p "$keychain_password" "$keychain"
  # Only codesign may use the key (-T), not every program (-A).
  security import "$tmp/cert.p12" -k "$keychain" -f pkcs12 -P "$MACOS_SIGN_PASSWORD" \
    -T /usr/bin/codesign >/dev/null 2>&1 || die "cannot import MACOS_SIGN_P12 (wrong MACOS_SIGN_PASSWORD or not a PKCS#12 file)"
  rm -f "$tmp/cert.p12"
  # Lets codesign use the key without a dialog. The output lists key
  # attributes; it is dropped.
  security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$keychain_password" "$keychain" >/dev/null 2>&1 ||
    die "cannot set the partition list of the temporary keychain"
  # codesign looks for the intermediate certificates in the search list.
  # The runner image has "Developer ID Certification Authority" (G2) in
  # the system keychain.
  # shellcheck disable=SC2086
  security list-keychains -d user -s "$keychain" $old_keychains

  identities=$(security find-identity -v -p codesigning "$keychain" | grep '"Developer ID Application: ' || true)
  [ "$(printf '%s' "$identities" | grep -c . || true)" = 1 ] ||
    die "MACOS_SIGN_P12 must hold exactly one valid Developer ID Application identity"
  identity=$(echo "$identities" | sed -nE 's/^[[:space:]]*[0-9]+\) ([0-9A-F]{40}) .*/\1/p')
  team=$(echo "$identities" | sed -nE 's/.*\(([A-Z0-9]{10})\)"$/\1/p')
  [ -n "$identity" ] && [ -n "$team" ] || die "cannot read the identity of MACOS_SIGN_P12"
  echo "release-app-sign: Developer ID Application, team $team, certificate SHA-1 $identity"
  sign_args=(--sign "$identity" --keychain "$keychain" --timestamp)
fi

# Inside out: a38 (nested code in Contents/MacOS), then the bundle, which
# signs the main executable and seals Info.plist and the nested code. a38
# gets a new signature here, also when quill signed it before (one tool,
# one identifier, both slices of the universal binary).
codesign --force "${sign_args[@]}" --options runtime --identifier "$A38_ID" "$BUNDLE/Contents/MacOS/a38"
codesign --force "${sign_args[@]}" --options runtime --identifier "$BUNDLE_ID" "$BUNDLE"
codesign --verify --deep --strict --verbose=2 "$BUNDLE"
echo "release-app-sign: $BUNDLE signed"
