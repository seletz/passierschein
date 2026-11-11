#!/usr/bin/env bash
# Show a sample of app/Samples in the debug build of the approval app, with
# the fake authenticator (no Touch ID) and auto-close.
#
# Usage: scripts/app-run-sample.sh [sample] [more test switches...]
#   sample: name of a file in app/Samples without .json (default:
#           first-request). Without a name the script lists the samples.
# The decision goes to the standard output, the exit code is that of the
# app.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
APP="$ROOT/app"
sample=${1:-first-request}
shift || true

if [ ! -f "$APP/Samples/$sample.json" ]; then
  echo "app:run:sample: no sample $sample; the samples are:" >&2
  (cd "$APP/Samples" && ls -1 ./*.json | sed 's|^\./||; s|\.json$||') >&2
  exit 2
fi

bash "$ROOT/scripts/app-swift-check.sh"
cd "$APP"
swift build -c debug >&2
# --skip-auth: never the real Touch ID dialog. --auto-close: the popup ends
# with timeout (a notice: closes) after 30 s.
exec "$(swift build -c debug --show-bin-path)/a38-approve" --skip-auth --auto-close 30 "$@" < "Samples/$sample.json"
