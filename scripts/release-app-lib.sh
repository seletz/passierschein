# Shared settings of the release bundle scripts (scripts/release-app-*.sh).
# Source it; it defines variables and functions only.
#
# The release bundle is Passierschein.app with the approval app and the
# darwin a38 of the same goreleaser run (D50):
#
#   Passierschein.app/Contents/Info.plist
#   Passierschein.app/Contents/MacOS/Passierschein   approval app, universal
#   Passierschein.app/Contents/MacOS/a38             Go CLI, universal (lipo)
#
# RELEASE_DIST    the goreleaser output with the darwin archives and the
#                 checksum file (default: dist)
# RELEASE_APP_OUT the work folder of the bundle (default: build/release-app)

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
DIST=${RELEASE_DIST:-$ROOT/dist}
OUT=${RELEASE_APP_OUT:-$ROOT/build/release-app}
BUNDLE="$OUT/Passierschein.app"
BUNDLE_ID=io.github.seletz.passierschein
# The fixed signing identifier of a38 inside the bundle (spike on the token
# in the keychain: the bundle ID with .a38).
A38_ID=$BUNDLE_ID.a38

die() {
  echo "::error::$*" >&2
  exit 1
}

# release_tag prints the tag of the goreleaser run in $DIST, read from the
# name of its single checksum file checksums-<tag>.txt.
release_tag() {
  local sums
  sums=$(cd "$DIST" 2>/dev/null && ls checksums-*.txt 2>/dev/null || true)
  [ -n "$sums" ] || die "no checksum file in $DIST; run goreleaser first (mise run release:snapshot)"
  [ "$(echo "$sums" | wc -l | tr -d ' ')" = 1 ] || die "more than one checksum file in $DIST"
  sums=${sums#checksums-}
  echo "${sums%.txt}"
}

# app_asset prints the base name of the release asset of the bundle.
app_asset() {
  echo "passierschein-app-$(release_tag)-darwin-universal"
}
