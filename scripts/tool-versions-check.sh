#!/usr/bin/env bash
# Check the pins of the build and release tools and the shape of the workflows.
# mise.toml is the source of truth; the workflows only install mise and run
# mise tasks, so the tool versions cannot drift between local runs and CI.
#
# - mise.toml pins go (equal to the go directive in go.mod), uv, goreleaser,
#   gitleaks and cyclonedx-gomod to exact X.Y.Z versions.
# - Each app task (approval app, macOS only) and release:app pin the same
#   exact swift.
# - mise verifies aqua downloads with cosign and SLSA provenance: aqua.cosign
#   and aqua.slsa must stay true.
# - Every `run:` of a workflow is a single `mise run <task>`.
# - Every action is pinned to a full commit hash, with the version as comment:
#   `uses: owner/name@<40 hex digits> # vX.Y.Z`.
#
# - Every job of a workflow has `timeout-minutes`, so that a hanging job does
#   not burn minutes (macOS minutes count ten times).
#
# A finding is printed and the script exits 1. `mise run tools:check` runs it.
# TOOLS_CHECK_ROOT overrides the repository root (for tests).
set -euo pipefail

ROOT=${TOOLS_CHECK_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}
MISE="$ROOT/mise.toml"
status=0

fail() {
  echo "::error::$*" >&2
  status=1
}

semver='[0-9]+\.[0-9]+\.[0-9]+'
pin() { sed -nE "s/^$1 = \"($semver)\"\$/\1/p" "$MISE" | head -n 1; }

go_mise=$(pin go)
go_mod=$(sed -nE 's/^go ([0-9.]+)$/\1/p' "$ROOT/go.mod")
[ -n "$go_mise" ] || fail "mise.toml: no exact go version under [tools] (go = \"X.Y.Z\")"
[ "$go_mise" = "$go_mod" ] || fail "mise.toml pins go ${go_mise:-?}, go.mod says ${go_mod:-?}"

for tool in uv goreleaser gitleaks; do
  [ -n "$(pin "$tool")" ] || fail "mise.toml: no exact $tool version under [tools] ($tool = \"X.Y.Z\")"
done

cyclonedx=$(sed -nE "s/.*\"github:CycloneDX\/cyclonedx-gomod\" = \"($semver)\".*/\1/p" "$MISE" | sort -u)
[ -n "$cyclonedx" ] || fail "mise.toml: no exact cyclonedx-gomod pin in the release tasks"
[ "$(echo "$cyclonedx" | wc -l | tr -d ' ')" = "1" ] || fail "mise.toml: the release tasks pin different cyclonedx-gomod versions"

# The app tasks (macOS only) pin the Swift toolchain at the task.
swift=$(sed -nE "s/.*tools = \{ \"swift\" = \"($semver)\" \}.*/\1/p" "$MISE" | sort -u)
# The app tasks and release:app (the release bundle) build Swift.
app_tasks=$(grep -cE '^\[tasks\."(app:|release:app"\])' "$MISE" || true)
swift_pins=$(grep -cE 'tools = \{ "swift" = "' "$MISE" || true)
[ -n "$swift" ] || fail "mise.toml: no exact swift pin in the app tasks (tools = { \"swift\" = \"X.Y.Z\" })"
[ "$(echo "$swift" | wc -l | tr -d ' ')" = "1" ] || fail "mise.toml: the app tasks pin different swift versions"
[ "$app_tasks" = "$swift_pins" ] || fail "mise.toml: $app_tasks app tasks, but $swift_pins swift pins; each app task pins swift"

for s in aqua.cosign aqua.slsa; do
  grep -qE "^${s//./\\.} = true\$" "$MISE" || fail "mise.toml: [settings] must keep $s = true (verified tool downloads)"
done

for wf in "$ROOT"/.github/workflows/*.yml; do
  name=$(basename "$wf")
  while IFS= read -r line; do
    cmd=${line#*run:}
    cmd=${cmd# }
    case "$cmd" in
      "mise run "*) ;;
      *) fail "$name: every step runs one mise task (run: mise run <task>), found: run: $cmd" ;;
    esac
  done < <(grep -E '^[[:space:]]*(- )?run:' "$wf" || true)
  jobs=$(grep -cE '^    runs-on:' "$wf" || true)
  timeouts=$(grep -cE '^    timeout-minutes: [0-9]+' "$wf" || true)
  [ "$jobs" = "$timeouts" ] || fail "$name: $jobs jobs, but $timeouts with timeout-minutes; each job needs timeout-minutes"
  while IFS= read -r line; do
    action=${line#*uses: }
    [[ "$action" =~ @[0-9a-f]{40}\ #\ v$semver$ ]] || fail "$name: $action is not pinned to a commit hash with a version comment (@<hash> # vX.Y.Z)"
  done < <(grep -E '^[[:space:]]*(- )?uses:' "$wf" || true)
done

if [ "$status" -eq 0 ]; then
  echo "pinned: go $go_mise, uv $(pin uv), goreleaser $(pin goreleaser), gitleaks $(pin gitleaks), cyclonedx-gomod $cyclonedx, swift $swift (app tasks); workflows run only mise tasks and pinned actions, each job has a timeout"
fi
exit "$status"
