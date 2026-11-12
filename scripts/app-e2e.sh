#!/usr/bin/env bash
# End-to-end run over real processes (T14, part 1): the real "a38 serve
# --ui app" starts the debug build of the approval app. macOS only.
#
# The script builds the debug app, then runs the Go test with the tags
# "darwin appe2e" (cmd/a38/app_e2e_test.go). The test starts a38 with a
# wrapper script as the app; the wrapper adds the test switches and records
# what the app gets and writes.
#
# NO Touch ID: each wrapper uses --skip-auth (fake authenticator) or ends by
# --auto-close before any confirmation.
# Two windows appear on the screen for about two seconds each.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)

bash "$ROOT/scripts/app-swift-check.sh"
cd "$ROOT/app"
swift build -c debug >&2
APP_DEBUG="$(swift build -c debug --show-bin-path)/Passierschein"
[ -x "$APP_DEBUG" ] || { echo "app-e2e: no debug app at $APP_DEBUG" >&2; exit 1; }

cd "$ROOT"
echo "app-e2e: two short windows open on the screen; no Touch ID is used." >&2
A38_APP_DEBUG="$APP_DEBUG" go test -tags appe2e -run '^TestAppE2E$' -count=1 -v ./cmd/a38
