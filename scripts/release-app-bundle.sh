#!/usr/bin/env bash
# Assemble the release bundle Passierschein.app (task release:app, macOS
# only). The bundle is not signed here; release:app:sign signs it.
#
# 1. Take the darwin a38 of the goreleaser run in $RELEASE_DIST: check the
#    two darwin archives against the checksum file, unpack a38 (amd64 and
#    arm64), check that both come from the same release (version string,
#    commit) and join them with lipo into one universal binary.
# 2. Build the approval app as a universal release binary (arm64 and
#    x86_64), because the release ships a38 for both architectures. Check
#    that the release binary rejects the test switches (T14).
# 3. Put both into $RELEASE_APP_OUT/Passierschein.app with an Info.plist:
#    bundle ID io.github.seletz.passierschein, version from the tag,
#    minimum macOS 15 (the platform of the Swift package), and the app
#    icon of scripts/app-icon.sh in Contents/Resources (D69, T42).
set -euo pipefail
. "$(dirname "$0")/release-app-lib.sh"

bash "$ROOT/scripts/app-swift-check.sh"

TAG=$(release_tag)
VERSION=${TAG#v}
echo "release-app: tag $TAG from $DIST"

WORK="$OUT/work"
rm -rf "$OUT"
mkdir -p "$WORK"

# 1. a38 of the same release.
sums="$DIST/checksums-$TAG.txt"
for arch in amd64 arm64; do
  archive="passierschein-$TAG-darwin-$arch.tar.gz"
  [ -f "$DIST/$archive" ] || die "$archive is missing in $DIST"
  line=$(grep -E "  $archive\$" "$sums" || true)
  [ -n "$line" ] || die "$(basename "$sums") does not list $archive"
  (cd "$DIST" && echo "$line" | shasum -a 256 -c --status) || die "$archive does not match its checksum"
  mkdir -p "$WORK/$arch"
  tar -xzf "$DIST/$archive" -C "$WORK/$arch" a38
done
[ "$(lipo -archs "$WORK/amd64/a38")" = x86_64 ] || die "the darwin-amd64 archive has no x86_64 a38"
[ "$(lipo -archs "$WORK/arm64/a38")" = arm64 ] || die "the darwin-arm64 archive has no arm64 a38"

# The version string that goreleaser linked in. The runner is arm64, so
# only the arm64 binary runs; the amd64 binary must contain the same
# string. A release has exactly the tag version; a snapshot has
# <tag version>-SNAPSHOT-<commit>.
line=$("$WORK/arm64/a38" --version)
a38_version=$(echo "$line" | sed -nE 's/^a38 ([^ ]+) .*/\1/p')
case "$a38_version" in
  "$VERSION" | "$VERSION"-SNAPSHOT-*) ;;
  *) die "a38 reports version '${a38_version:-?}', the tag is $TAG" ;;
esac
LC_ALL=C grep -q -a -F "$a38_version" "$WORK/amd64/a38" || die "the amd64 a38 does not contain the version $a38_version"
# Both binaries come from the same commit; in CI also from the checked-out
# commit.
rev() { go version -m "$1" | sed -nE 's/^[[:space:]]*build[[:space:]]+vcs\.revision=([0-9a-f]+)$/\1/p'; }
rev_amd64=$(rev "$WORK/amd64/a38")
rev_arm64=$(rev "$WORK/arm64/a38")
[ -n "$rev_arm64" ] && [ "$rev_amd64" = "$rev_arm64" ] || die "the two a38 binaries name different commits"
head=$(git -C "$ROOT" rev-parse HEAD 2>/dev/null || true)
[ -z "$head" ] || [ "$head" = "$rev_arm64" ] || die "a38 is from commit $rev_arm64, the checkout is $head"
lipo -create "$WORK/amd64/a38" "$WORK/arm64/a38" -output "$WORK/a38"
echo "release-app: a38 $a38_version, commit ${rev_arm64:0:7}, $(lipo -archs "$WORK/a38")"

# 2. The approval app, universal.
cd "$ROOT/app"
swift build -c release --arch arm64 --arch x86_64
APP_BIN="$(swift build -c release --arch arm64 --arch x86_64 --show-bin-path)/Passierschein"
[ "$(lipo -archs "$APP_BIN")" = "x86_64 arm64" ] || die "the approval app is not universal: $(lipo -archs "$APP_BIN")"
bash "$ROOT/scripts/app-release-check.sh" "$APP_BIN"
bash "$ROOT/scripts/app-icon.sh"

# 3. The bundle. CFBundleShortVersionString takes X.Y.Z of the tag (no
# pre-release part: macOS allows only numbers there); CFBundleVersion is
# the number of commits, as in app:build.
short=$(echo "$VERSION" | sed -nE 's/^([0-9]+\.[0-9]+\.[0-9]+).*/\1/p')
[ -n "$short" ] || die "the tag $TAG has no version X.Y.Z"
build=$(git -C "$ROOT" rev-list --count HEAD 2>/dev/null || echo 1)
mkdir -p "$BUNDLE/Contents/MacOS" "$BUNDLE/Contents/Resources"
install -m 0755 "$APP_BIN" "$BUNDLE/Contents/MacOS/Passierschein"
install -m 0755 "$WORK/a38" "$BUNDLE/Contents/MacOS/a38"
install -m 0644 "$ROOT/app/.build/icon/AppIcon.icns" "$BUNDLE/Contents/Resources/AppIcon.icns"
cat > "$BUNDLE/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleIdentifier</key>
  <string>$BUNDLE_ID</string>
  <key>CFBundleName</key>
  <string>Passierschein</string>
  <key>CFBundleDisplayName</key>
  <string>Passierschein</string>
  <key>CFBundleExecutable</key>
  <string>Passierschein</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleIconFile</key>
  <string>AppIcon</string>
  <key>CFBundleInfoDictionaryVersion</key>
  <string>6.0</string>
  <key>CFBundleShortVersionString</key>
  <string>$short</string>
  <key>CFBundleVersion</key>
  <string>$build</string>
  <key>LSUIElement</key>
  <true/>
  <key>LSMinimumSystemVersion</key>
  <string>15.0</string>
</dict>
</plist>
PLIST
plutil -lint "$BUNDLE/Contents/Info.plist" >/dev/null
# No extended attributes: codesign rejects resource forks and Finder info.
xattr -cr "$BUNDLE"
echo "release-app: $BUNDLE (version $short, build $build, not signed yet)"
find "$BUNDLE" | sed "s|^$OUT/||" | sort
