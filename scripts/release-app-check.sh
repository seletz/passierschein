#!/usr/bin/env bash
# Check the signed release bundle (task release:app:check, macOS only).
#
# APP_CHECK=developer-id (default; release and signed snapshot):
#   - the bundle verifies (codesign --verify --deep --strict),
#   - Passierschein and a38: fixed identifier, hardened runtime, not
#     ad-hoc, no get-task-allow, a secure timestamp, the chain
#     "Developer ID Application" -> "Developer ID Certification Authority"
#     -> "Apple Root CA", and the same team ID for both,
#   - both are universal (x86_64 arm64),
#   - Info.plist names AppIcon and Contents/Resources/AppIcon.icns has all
#     ten sizes (T42, scripts/app-icon-check.sh),
#   - Gatekeeper accepts the bundle as notarized (spctl -t exec),
#   - the stapled ticket is valid (stapler validate),
#   - a38 runs the WebAssembly core of the 1Password SDK under its hardened
#     runtime and has exactly the entitlements of scripts/a38.entitlements;
#     the app has no entitlements (T30, D59).
# APP_CHECK=adhoc (pull requests, develop, local tests): the same layout,
#   identifier and hardened runtime checks, an ad-hoc signature, no
#   Gatekeeper, no ticket; the SDK core check runs in both modes.
#
# The check prints flags, identifiers, the team ID and the verdicts, not
# the subject of the certificate.
set -euo pipefail
. "$(dirname "$0")/release-app-lib.sh"

[ "$(uname -s)" = Darwin ] || die "release:app:check runs on macOS only"
MODE=${APP_CHECK:-developer-id}
case "$MODE" in developer-id | adhoc) ;; *) die "APP_CHECK must be developer-id or adhoc" ;; esac
TARGET=${1:-$BUNDLE}
[ -d "$TARGET" ] || die "$TARGET is missing"

status=0
fail() {
  echo "::error::$*" >&2
  status=1
}

codesign --verify --deep --strict --verbose=2 "$TARGET" 2>&1 || fail "codesign --verify --deep --strict fails for $TARGET"

plist="$TARGET/Contents/Info.plist"
for kv in "CFBundleIdentifier=$BUNDLE_ID" CFBundleExecutable=Passierschein CFBundlePackageType=APPL LSMinimumSystemVersion=15.0; do
  got=$(plutil -extract "${kv%%=*}" raw -o - "$plist" 2>/dev/null || true)
  [ "$got" = "${kv#*=}" ] || fail "Info.plist: ${kv%%=*} is '${got:-?}', want '${kv#*=}'"
done
extra=$(cd "$TARGET/Contents/MacOS" && ls | grep -v -x -e Passierschein -e a38 || true)
[ -z "$extra" ] || fail "Contents/MacOS has extra files: $extra"
# T42: the app icon (D69), sealed by the signature of the bundle.
bash "$(dirname "$0")/app-icon-check.sh" "$TARGET" || fail "the app icon of $TARGET is missing or incomplete"

teams=()
for f in Passierschein a38; do
  path="$TARGET/Contents/MacOS/$f"
  want_id=$BUNDLE_ID
  [ "$f" = a38 ] && want_id=$A38_ID
  info=$(codesign -d --verbose=4 "$path" 2>&1) || { fail "$f: no signature"; continue; }
  id=$(echo "$info" | sed -nE 's/^Identifier=(.*)$/\1/p')
  flags=$(echo "$info" | sed -nE 's/^CodeDirectory .* flags=0x[0-9a-f]+\(([^)]*)\).*/\1/p')
  team=$(echo "$info" | sed -nE 's/^TeamIdentifier=(.*)$/\1/p')
  timestamp=$(echo "$info" | grep -c '^Timestamp=' || true)
  authorities=$(echo "$info" | sed -nE 's/^Authority=(.*)$/\1/p')
  archs=$(lipo -archs "$path")
  ents=$(codesign -d --entitlements - --xml "$path" 2>/dev/null || true)
  echo "$f: identifier=$id flags=$flags team=$team timestamp=$timestamp archs=$archs"
  [ "$id" = "$want_id" ] || fail "$f: identifier $id, want $want_id"
  [ "$archs" = "x86_64 arm64" ] || fail "$f: architectures $archs, want x86_64 arm64"
  case ",$flags," in *,runtime,*) ;; *) fail "$f: no hardened runtime flag" ;; esac
  case "$ents" in *get-task-allow*) fail "$f: has the entitlement get-task-allow" ;; esac
  # D59: only a38 has an entitlement (sdk-core-check.sh checks its set).
  if [ "$f" = Passierschein ] && echo "$ents" | grep -q '<key>'; then fail "$f: has entitlements; the app needs none (D59)"; fi
  if [ "$MODE" = adhoc ]; then
    case ",$flags," in *,adhoc,*) ;; *) fail "$f: not ad-hoc signed in adhoc mode" ;; esac
    continue
  fi
  case ",$flags," in *,adhoc,*) fail "$f: only ad-hoc signed" ;; esac
  [ "$timestamp" = 1 ] || fail "$f: no secure timestamp"
  first=$(echo "$authorities" | sed -n 1p)
  case "$first" in "Developer ID Application: "*) ;; *) fail "$f: the leaf certificate is not a Developer ID Application certificate" ;; esac
  echo "$authorities" | grep -qx "Developer ID Certification Authority" || fail "$f: no Developer ID Certification Authority in the chain"
  echo "$authorities" | grep -qx "Apple Root CA" || fail "$f: the chain does not end at Apple Root CA"
  if echo "$team" | grep -qE '^[A-Z0-9]{10}$'; then teams+=("$team"); else fail "$f: no team ID"; fi
  echo "$f: authorities=Developer ID Application > $(echo "$authorities" | sed -n '2,$p' | paste -sd '>' - | sed 's/>/ > /g')"
done

if [ "$MODE" = developer-id ]; then
  [ "${#teams[@]}" = 2 ] && [ "${teams[0]}" = "${teams[1]}" ] || fail "Passierschein and a38 do not have the same team ID"
  # The Developer ID requirement with the team, as the daemon checks it
  # (appui.SystemCodeSigner.VerifyTeam).
  if [ "${#teams[@]}" -ge 1 ]; then
    req="=anchor apple generic and certificate 1[field.1.2.840.113635.100.6.2.6] exists and certificate leaf[field.1.2.840.113635.100.6.1.13] exists and certificate leaf[subject.OU] = \"${teams[0]}\""
    for f in Passierschein a38; do
      codesign --verify --strict --deep -R "$req" "$TARGET/Contents/MacOS/$f" 2>/dev/null ||
        fail "$f does not satisfy the Developer ID requirement of team ${teams[0]}"
    done
  fi
  verdict=$(spctl -a -vv -t exec "$TARGET" 2>&1 || true)
  echo "$verdict" | grep -E ': accepted$|^source=' || true
  echo "$verdict" | grep -q ': accepted$' || fail "Gatekeeper rejects the bundle"
  echo "$verdict" | grep -qx 'source=Notarized Developer ID' || fail "Gatekeeper does not see a notarization (want source=Notarized Developer ID)"
  xcrun stapler validate "$TARGET" || fail "the stapled ticket is missing or not valid"
fi

# T30: the signed a38 runs the WebAssembly core of the 1Password SDK under
# its hardened runtime with exactly the entitlements of a38.entitlements (D59).
bash "$(dirname "$0")/sdk-core-check.sh" "$TARGET/Contents/MacOS/a38" || fail "a38 in the bundle cannot run the 1Password SDK core"

if [ "$status" = 0 ]; then
  echo "release-app-check: $TARGET passes ($MODE)"
fi
exit "$status"
