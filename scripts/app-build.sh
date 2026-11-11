#!/usr/bin/env bash
# Build the approval app (app/, Swift package A38Approval) in both
# configurations and assemble the app bundle.
#
# - debug:   app/.build/debug/a38-approve, with the test switches (T14).
# - release: app/.build/release/a38-approve, without the test switches.
# - bundle:  app/.build/bundle/a38-approve.app from the release binary,
#            with Info.plist (LSUIElement) and an ad-hoc signature. The
#            Developer ID signature and the notarization are open (G5).
#
# Then the script runs scripts/app-release-check.sh on the release binary
# and on the bundle. macOS only: AppKit, SwiftUI and LocalAuthentication
# exist only there. `mise run app:build` runs this script with the pinned
# Swift toolchain.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
APP="$ROOT/app"

bash "$ROOT/scripts/app-swift-check.sh"

cd "$APP"
swift build -c debug
swift build -c release

BIN=$(swift build -c release --show-bin-path)/a38-approve
BUNDLE="$APP/.build/bundle/a38-approve.app"
VERSION=$(sed -nE 's/^public let appVersion = "([0-9]+\.[0-9]+\.[0-9]+)"$/\1/p' Sources/A38ApprovalCore/Launch.swift)
[ -n "$VERSION" ] || { echo "app-build: no appVersion in Launch.swift" >&2; exit 1; }
BUILD=$(git -C "$ROOT" rev-list --count HEAD 2>/dev/null || echo 1)

rm -rf "$BUNDLE"
mkdir -p "$BUNDLE/Contents/MacOS"
cp "$BIN" "$BUNDLE/Contents/MacOS/a38-approve"
cat > "$BUNDLE/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleIdentifier</key>
  <string>io.github.seletz.a38.approve</string>
  <key>CFBundleName</key>
  <string>a38</string>
  <key>CFBundleDisplayName</key>
  <string>a38</string>
  <key>CFBundleExecutable</key>
  <string>a38-approve</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
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

bash "$ROOT/scripts/app-release-check.sh" "$BIN" "$BUNDLE/Contents/MacOS/a38-approve"
echo "app-build: debug and release built; bundle $BUNDLE (ad-hoc signed)"
