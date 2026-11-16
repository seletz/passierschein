#!/usr/bin/env bash
# Build the approval app (app/, Swift package A38Approval) in both
# configurations and assemble the app bundle.
#
# - debug:   app/.build/debug/Passierschein, with the test switches (T14).
# - release: app/.build/release/Passierschein, without the test switches.
# - bundle:  app/.build/bundle/Passierschein.app from the release binary,
#            with Info.plist (LSUIElement) and an ad-hoc signature. The
#            Developer ID signature and the notarization are open (G5).
#
# Layout of the bundle (the daemon of PR 43 searches this path):
#
#   Passierschein.app/Contents/Info.plist
#   Passierschein.app/Contents/MacOS/Passierschein   the approval app
#   Passierschein.app/Contents/MacOS/a38             reserved for the Go CLI
#   Passierschein.app/Contents/Resources/AppIcon.icns
#                                                    the app icon (D69)
#
# This script does not copy a38 into the bundle. The release work puts it
# there and signs the whole bundle (G5).
#
# The app icon comes from scripts/app-icon.sh (task app:icon). Then the
# script runs scripts/app-release-check.sh on the release binary and on
# the bundle, and scripts/app-icon-check.sh on the bundle (T42). macOS
# only: AppKit, SwiftUI and LocalAuthentication exist only there. `mise run app:build` runs this script with the pinned
# Swift toolchain.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
APP="$ROOT/app"

bash "$ROOT/scripts/app-swift-check.sh"

cd "$APP"
swift build -c debug
swift build -c release

BIN=$(swift build -c release --show-bin-path)/Passierschein
BUNDLE="$APP/.build/bundle/Passierschein.app"
VERSION=$(sed -nE 's/^public let appVersion = "([0-9]+\.[0-9]+\.[0-9]+)"$/\1/p' Sources/A38ApprovalCore/Launch.swift)
[ -n "$VERSION" ] || { echo "app-build: no appVersion in Launch.swift" >&2; exit 1; }
BUILD=$(git -C "$ROOT" rev-list --count HEAD 2>/dev/null || echo 1)

bash "$ROOT/scripts/app-icon.sh"

# Start from an empty folder, so that no bundle of an older name stays.
rm -rf "$APP/.build/bundle"
mkdir -p "$BUNDLE/Contents/MacOS" "$BUNDLE/Contents/Resources"
cp "$BIN" "$BUNDLE/Contents/MacOS/Passierschein"
cp "$APP/.build/icon/AppIcon.icns" "$BUNDLE/Contents/Resources/AppIcon.icns"
cat > "$BUNDLE/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleIdentifier</key>
  <string>io.github.seletz.passierschein</string>
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
  <key>CFBundleShortVersionString</key>
  <string>$VERSION</string>
  <key>CFBundleVersion</key>
  <string>$BUILD</string>
  <key>LSUIElement</key>
  <true/>
  <key>LSMinimumSystemVersion</key>
  <string>15.0</string>
</dict>
</plist>
PLIST
plutil -lint "$BUNDLE/Contents/Info.plist" >/dev/null
# Ad-hoc signature with hardened runtime; no Developer ID (G5).
codesign --force --sign - --options runtime "$BUNDLE"
codesign --verify --strict "$BUNDLE"

bash "$ROOT/scripts/app-release-check.sh" "$BIN" "$BUNDLE/Contents/MacOS/Passierschein"
bash "$ROOT/scripts/app-icon-check.sh" "$BUNDLE"
echo "app-build: debug and release built; bundle $BUNDLE (ad-hoc signed)"
