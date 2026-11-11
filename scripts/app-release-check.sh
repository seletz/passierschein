#!/usr/bin/env bash
# T14: the test switches must not work in the shipped bundle.
#
# For each release binary given as argument:
# - each test switch ends with exit code 64 and an error decision, before
#   the app reads the standard input or creates a window;
# - the binary contains no fake authenticator and no headless runner;
# - --help works and writes nothing to the standard output.
#
# The check never starts the app with a valid request: a release build
# would show the real Touch ID path.
set -euo pipefail

[ "$#" -ge 1 ] || { echo "usage: $0 <release-binary>..." >&2; exit 2; }

switches=(
  "--request /dev/null" "--skip-auth" "--fake-auth success" "--auto-close 1" "--no-window"
  "--demo-select all" "--demo-scroll-end" "--demo-lifetime-open" "--demo-arm-delay 0"
  "--demo-confirm" "--debug-window"
)
status=0
fail() { echo "app-release-check: $*" >&2; status=1; }

for bin in "$@"; do
  [ -x "$bin" ] || { fail "$bin is not an executable"; continue; }
  for sw in "${switches[@]}"; do
    # shellcheck disable=SC2086 # the switch and its value are two words
    out=$("$bin" $sw </dev/null 2>/dev/null) && code=0 || code=$?
    [ "$code" = 64 ] || fail "$bin $sw: exit code $code, want 64"
    case "$out" in
      *'"outcome":"error"'*) ;;
      *) fail "$bin $sw: no error decision on the standard output" ;;
    esac
  done
  # Control: the symbol table is readable, so a missing symbol means
  # something.
  symbols=$(nm "$bin" 2>/dev/null || true)
  texts=$(strings "$bin" 2>/dev/null || true)
  grep -q TouchIDAuthenticator <<<"$symbols" || fail "$bin: no symbol table to check"
  for symbol in FakeAuthenticator runHeadless applyDemo parseTestSwitch; do
    if grep -q "$symbol" <<<"$symbols"; then
      fail "$bin contains the test code $symbol"
    fi
  done
  for text in "fake authenticator" "Test switches (debug build only)"; do
    if grep -q "$text" <<<"$texts"; then
      fail "$bin contains the test text: $text"
    fi
  done
  out=$("$bin" --help </dev/null 2>/dev/null) && code=0 || code=$?
  [ "$code" = 0 ] || fail "$bin --help: exit code $code, want 0"
  [ -z "$out" ] || fail "$bin --help: output on the standard output"
done

if [ "$status" -eq 0 ]; then
  echo "app-release-check: ${#switches[@]} test switches rejected with exit code 64, no test code in $# binaries"
fi
exit "$status"
