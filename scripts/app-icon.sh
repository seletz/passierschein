#!/usr/bin/env bash
# Build the app icon of Passierschein.app from its source (task app:icon,
# macOS only; issue 104, D69, T42).
#
# Source: app/Icon/render-icon.swift draws three drafts with Core
# Graphics. This script compiles it with the pinned Swift toolchain, draws
# each draft into an icon set (10 PNG files, 16 to 1024 px) and joins the
# set with iconutil into an icns file:
#
#   app/.build/icon/draft-<n>/AppIcon.iconset
#   app/.build/icon/draft-<n>/AppIcon.icns
#   app/.build/icon/AppIcon.icns              the icon of the bundle
#
# APP_ICON_DRAFT selects the draft of the bundle (D69: draft 2).
#
# T42: the script draws the selected draft a second time and fails if the
# two icns files differ. The icon depends only on the source.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
OUT="$ROOT/app/.build/icon"
SELECTED=${APP_ICON_DRAFT:-2}

bash "$ROOT/scripts/app-swift-check.sh"

rm -rf "$OUT"
mkdir -p "$OUT"
SDK=$(xcrun --sdk macosx --show-sdk-path)
swiftc -O -sdk "$SDK" -o "$OUT/render-icon" "$ROOT/app/Icon/render-icon.swift"

for draft in 1 2 3; do
  dir="$OUT/draft-$draft"
  "$OUT/render-icon" "$draft" "$dir/AppIcon.iconset"
  iconutil --convert icns --output "$dir/AppIcon.icns" "$dir/AppIcon.iconset"
done

# The same source gives the same icns (T42).
"$OUT/render-icon" "$SELECTED" "$OUT/again/AppIcon.iconset"
iconutil --convert icns --output "$OUT/again/AppIcon.icns" "$OUT/again/AppIcon.iconset"
if ! cmp -s "$OUT/draft-$SELECTED/AppIcon.icns" "$OUT/again/AppIcon.icns"; then
  echo "app-icon: two runs of the same source give different icns files" >&2
  exit 1
fi
rm -rf "$OUT/again"

cp "$OUT/draft-$SELECTED/AppIcon.icns" "$OUT/AppIcon.icns"
echo "app-icon: $OUT/AppIcon.icns (draft $SELECTED, $(shasum -a 256 "$OUT/AppIcon.icns" | cut -c1-12)), drafts 1 to 3 in $OUT"
