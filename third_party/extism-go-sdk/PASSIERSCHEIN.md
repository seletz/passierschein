# Patched copy of github.com/extism/go-sdk v1.7.1

This directory is the module `github.com/extism/go-sdk` v1.7.1 (BSD 3-Clause,
see `LICENSE`) without its tests, plus one change in `plugin.go`
(`../extism-go-sdk.patch`). `go.mod` of Passierschein replaces the module
with this directory.

The change: when the caller passes no runtime configuration, extism uses the
interpreter of wazero, not the compiler. The 1Password SDK passes none. The
compiler of wazero maps memory, writes machine code into it and makes it
executable (`mprotect` with `PROT_EXEC`, no `MAP_JIT`). The hardened runtime
of macOS kills a process that runs such code unless it has the entitlement
`com.apple.security.cs.allow-unsigned-executable-memory`. The interpreter
runs no generated machine code, so a38 needs no entitlement (decision D59,
issue #89).

`mise run deps:extism:check` checks that this directory is the upstream
module plus the patch, and nothing else. To update: copy the files of the
new upstream version that the 1Password SDK requires, apply the patch,
record the checksum of the new version in `../extism-go-sdk.sum`, and run the
check.
