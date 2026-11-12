#!/usr/bin/env bash
# Pack the signed (and, for a release, stapled) bundle for the release
# (task release:app:package, macOS only).
#
# - $RELEASE_DIST/passierschein-app-<tag>-darwin-universal.zip, made with
#   ditto -c -k --keepParent (Apple's way; keeps the signature and the
#   ticket). The script unpacks the zip into a temporary folder and runs
#   release:app:check on the copy.
# - $RELEASE_DIST/passierschein-app-<tag>-darwin-universal.cdx.json, the
#   CycloneDX SBOM of the zip (D18: one SBOM per archive). Main component:
#   the bundle. Components: the two executables with their SHA-256, and
#   the Go modules of a38 from cyclonedx-gomod (the same modules as in the
#   darwin archives).
# - Two lines for the zip and its SBOM in $RELEASE_DIST/checksums-<tag>.txt.
# APP_CHECK as for release:app:check.
set -euo pipefail
. "$(dirname "$0")/release-app-lib.sh"

[ "$(uname -s)" = Darwin ] || die "release:app:package runs on macOS only"
[ -d "$BUNDLE" ] || die "$BUNDLE is missing"
TAG=$(release_tag)
NAME=$(app_asset)
ZIP="$DIST/$NAME.zip"
SBOM="$DIST/$NAME.cdx.json"
SUMS="$DIST/checksums-$TAG.txt"

rm -f "$ZIP" "$SBOM"
ditto -c -k --keepParent "$BUNDLE" "$ZIP"

tmp=$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/a38-package.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
ditto -x -k "$ZIP" "$tmp/unpacked"
bash "$ROOT/scripts/release-app-check.sh" "$tmp/unpacked/Passierschein.app"

# SBOM. a38 in the bundle is the universal binary of the two release
# binaries; cyclonedx-gomod reads the module list of the arm64 binary of
# the darwin-arm64 archive (amd64 links the same modules).
version=${TAG#v}
a38_version=$("$BUNDLE/Contents/MacOS/a38" --version | sed -nE 's/^a38 ([^ ]+) .*/\1/p')
cyclonedx-gomod bin -json -version "$a38_version" -output "$tmp/a38.cdx.json" "$OUT/work/arm64/a38"
sha() { shasum -a 256 "$1" | cut -d' ' -f1; }
app_sha=$(sha "$BUNDLE/Contents/MacOS/Passierschein")
a38_sha=$(sha "$BUNDLE/Contents/MacOS/a38")
short=$(plutil -extract CFBundleShortVersionString raw -o - "$BUNDLE/Contents/Info.plist")
jq --arg version "$version" --arg short "$short" --arg app_sha "$app_sha" --arg a38_sha "$a38_sha" \
   --arg bundle_id "$BUNDLE_ID" '
  (.metadata.component) as $a38
  | ($a38 + {
      "bom-ref": ($a38["bom-ref"] // "a38"),
      name: "a38",
      description: "Contents/MacOS/a38: the Go CLI, universal binary (x86_64 arm64), signed in the bundle",
      hashes: [{alg: "SHA-256", content: $a38_sha}]
    }) as $a38c
  | {
      "bom-ref": "app:Passierschein",
      type: "application",
      name: "Passierschein",
      version: $short,
      description: "Contents/MacOS/Passierschein: the approval app (Swift, no package dependencies), universal binary",
      hashes: [{alg: "SHA-256", content: $app_sha}],
      purl: ("pkg:swift/github.com/seletz/passierschein/app@" + $version)
    } as $app
  | .metadata.component = {
      "bom-ref": "bundle:Passierschein.app",
      type: "application",
      name: "Passierschein.app",
      version: $version,
      description: ("macOS app bundle " + $bundle_id + " with the approval app and a38")
    }
  | .components = [$app, $a38c] + (.components // [])
  | .dependencies = ((.dependencies // []) + [{ref: "bundle:Passierschein.app", dependsOn: ["app:Passierschein", $a38c["bom-ref"]]}])
' "$tmp/a38.cdx.json" > "$SBOM"

# Checksums: replace the lines of a former run, then add the two files.
grep -v -E "  ($NAME\\.zip|$NAME\\.cdx\\.json)\$" "$SUMS" > "$tmp/sums" || true
(cd "$DIST" && shasum -a 256 "$NAME.zip" "$NAME.cdx.json") >> "$tmp/sums"
cat "$tmp/sums" > "$SUMS"
echo "release-app-package: $(basename "$ZIP"), $(basename "$SBOM"); $(basename "$SUMS") lists both"
