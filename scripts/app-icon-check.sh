#!/usr/bin/env bash
# T42: check the app icon of a bundle Passierschein.app (macOS only).
#
#   bash scripts/app-icon-check.sh <path>/Passierschein.app
#
# The check fails if
#   - Info.plist does not name the icon (CFBundleIconFile = AppIcon),
#   - Contents/Resources/AppIcon.icns is missing,
#   - the icns does not hold all ten images of the macOS icon set
#     (16, 32, 128, 256 and 512 points, each at 1x and 2x) with the
#     correct pixel size.
#
# scripts/app-build.sh and scripts/release-app-check.sh run this check.
set -euo pipefail

BUNDLE=${1:-}
[ -n "$BUNDLE" ] || { echo "usage: app-icon-check.sh <Passierschein.app>" >&2; exit 2; }
[ -d "$BUNDLE" ] || { echo "app-icon-check: $BUNDLE is missing" >&2; exit 1; }

status=0
fail() {
  echo "app-icon-check: $*" >&2
  status=1
}

name=$(plutil -extract CFBundleIconFile raw -o - "$BUNDLE/Contents/Info.plist" 2>/dev/null || true)
[ "$name" = AppIcon ] || fail "Info.plist: CFBundleIconFile is '${name:-?}', want 'AppIcon'"

icns="$BUNDLE/Contents/Resources/AppIcon.icns"
if [ ! -f "$icns" ]; then
  fail "$icns is missing"
  exit "$status"
fi

tmp=$(mktemp -d "${TMPDIR:-/tmp}/a38-icon-check.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
iconutil --convert iconset --output "$tmp/AppIcon.iconset" "$icns" || fail "iconutil cannot read $icns"

for size in 16 32 128 256 512; do
  for scale in 1 2; do
    file="icon_${size}x${size}.png"
    [ "$scale" = 2 ] && file="icon_${size}x${size}@2x.png"
    want=$((size * scale))
    path="$tmp/AppIcon.iconset/$file"
    if [ ! -f "$path" ]; then
      fail "AppIcon.icns has no $file"
      continue
    fi
    w=$(sips -g pixelWidth "$path" | sed -nE 's/.*pixelWidth: ([0-9]+).*/\1/p')
    h=$(sips -g pixelHeight "$path" | sed -nE 's/.*pixelHeight: ([0-9]+).*/\1/p')
    [ "$w" = "$want" ] && [ "$h" = "$want" ] || fail "$file is ${w:-?}x${h:-?}, want ${want}x${want}"
  done
done

if [ "$status" = 0 ]; then
  echo "app-icon-check: $BUNDLE has AppIcon.icns with 10 images (16 to 1024 px)"
fi
exit "$status"
