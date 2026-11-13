#!/usr/bin/env bash
# Check that the linux build of a38 has no code for 1Password (test T13)
# and no code for the macOS keychain (D42, D53, issue 59).
# The list of packages must not contain:
# - a package of the 1Password SDK (github.com/1password/),
# - a package of the WebAssembly runtime of the SDK (github.com/extism/,
#   github.com/tetratelabs/wazero, github.com/dylibso/),
# - a package of this module whose path contains "onepassword" or "1password",
# - purego (github.com/ebitengine/purego), which binds Security.framework,
# - the keychain package of this module (internal/keychain),
# - the LaunchAgent package of this module (internal/service, issue 70),
# - the terminal view "a38 tui" (internal/tui) and its libraries
#   (charm.land/...): the audit log lives on the Mac (D53, issue 71).
# Task deps:linux:check. With "--self-test" the script checks itself with
# faked package lists, and then with real package lists: the darwin build
# must contain the 1Password backend, the SDK, the keychain package and
# purego, and a linux build with the
# build constraint "darwin" switched on (as if the constraint were removed)
# must be caught. So the check is known to guard something.
set -euo pipefail

MODULE=github.com/seletz/passierschein

# forbidden reads a package list on stdin and prints each forbidden package.
forbidden() {
  grep -iE "^github\.com/1password/|^github\.com/extism/|^github\.com/tetratelabs/wazero|^github\.com/dylibso/|^github\.com/ebitengine/purego|^${MODULE}/.*(onepassword|1password)|^${MODULE}/internal/keychain(/|$)|^${MODULE}/internal/service(/|$)|^${MODULE}/internal/tui(/|$)|^charm\.land/" || true
}

if [ "${1:-}" = "--self-test" ]; then
  fail=0
  expect() { # expect <name> <want-count> <list>
    got=$(printf '%s\n' "$3" | forbidden | grep -c . || true)
    if [ "$got" != "$2" ]; then
      echo "self-test '$1': found $got forbidden packages, want $2" >&2
      fail=1
    fi
  }
  expect clean 0 $'fmt\ngithub.com/seletz/passierschein/internal/protocol'
  expect sdk 1 $'fmt\ngithub.com/1password/onepassword-sdk-go'
  expect backend 1 $'fmt\ngithub.com/seletz/passierschein/internal/backend/onepassword'
  expect backend-digit 1 $'github.com/seletz/passierschein/internal/1password'
  expect other-module 0 $'github.com/other/onepassword'
  expect wasm-runtime 2 $'github.com/extism/go-sdk\ngithub.com/tetratelabs/wazero/api'
  expect purego 2 $'github.com/ebitengine/purego\ngithub.com/ebitengine/purego/internal/strings'
  expect keychain 1 $'fmt\ngithub.com/seletz/passierschein/internal/keychain'
  expect keychain-like 0 $'github.com/seletz/passierschein/internal/keychainx\ngithub.com/other/keychain'
  expect tui 2 $'github.com/seletz/passierschein/internal/tui\ncharm.land/bubbletea/v2'
  expect tui-like 0 $'github.com/seletz/passierschein/internal/tuix\ngithub.com/other/charm.land'
  expect service 2 $'github.com/seletz/passierschein/internal/service\ngithub.com/seletz/passierschein/internal/service/fakelaunchctl'
  expect service-like 0 $'github.com/seletz/passierschein/internal/servicex\ngithub.com/other/service'

  # must_catch <name> <package list>: the list must hold the backend, the
  # SDK, the keychain package, purego and the terminal view, and the filter
  # must find them.
  must_catch() {
    found=$(printf '%s\n' "$2" | forbidden)
    for pkg in "${MODULE}/internal/backend/onepassword" "github.com/1password/onepassword-sdk-go" \
      "${MODULE}/internal/keychain" "github.com/ebitengine/purego" "${MODULE}/internal/service" \
      "${MODULE}/internal/tui" "charm.land/bubbletea/v2"; do
      if ! printf '%s\n' "$found" | grep -qxF "$pkg"; then
        echo "self-test '$1': $pkg is not found; the check would guard nothing" >&2
        fail=1
      fi
    done
  }
  # Control: the darwin build contains the backend, the SDK, the keychain
  # package and purego.
  must_catch darwin-build "$(GOOS=darwin go list -deps ./cmd/a38)"
  # Negative test: with the constraint "darwin" switched on in a linux
  # build (as if the build constraint were removed), the check fails.
  must_catch linux-without-constraint "$(GOOS=linux go list -tags darwin -deps ./cmd/a38)"
  [ "$fail" = 0 ] && echo "self-test ok"
  exit "$fail"
fi

list=$(GOOS=linux go list -deps ./cmd/a38)
found=$(printf '%s\n' "$list" | forbidden)
if [ -n "$found" ]; then
  echo "::error::the linux build of a38 contains code for 1Password, the macOS keychain or the terminal view:" >&2
  printf '%s\n' "$found" >&2
  exit 1
fi
echo "the linux build of a38 has no 1Password, keychain, LaunchAgent or terminal view packages ($(printf '%s\n' "$list" | wc -l | tr -d ' ') packages checked)"
