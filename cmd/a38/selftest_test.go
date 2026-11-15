package main

import (
	"runtime"
	"strings"
	"testing"
)

// T30: "a38 selftest sdk-core" runs the WebAssembly core of the
// 1Password SDK with an invalid token and names the result. The unsigned
// test binary has no hardened runtime; scripts/sdk-core-check.sh runs the
// same command on a binary with hardened runtime.
func TestSelftestCommand(t *testing.T) {
	coreCode, coreOut := exitOK, "the WebAssembly core of the 1Password SDK runs in this a38"
	if runtime.GOOS != "darwin" {
		coreCode, coreOut = exitFailure, "not available on this platform"
	}
	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  string
	}{
		{"sdk-core", []string{"selftest", "sdk-core"}, coreCode, coreOut},
		{"help", []string{"selftest", "--help"}, exitOK, "a38 selftest sdk-core"},
		{"usage: no check", []string{"selftest"}, exitUsage, "a38 selftest sdk-core"},
		{"usage: unknown check", []string{"selftest", "network"}, exitUsage, `unknown check "network"`},
		{"usage: extra argument", []string{"selftest", "sdk-core", "extra"}, exitUsage, `unexpected argument "extra"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runCmd(tt.args...)
			if code != tt.wantCode {
				t.Fatalf("exit code %d, want %d; stdout %q, stderr %q", code, tt.wantCode, stdout, stderr)
			}
			if !strings.Contains(stdout+stderr, tt.wantOut) {
				t.Fatalf("output %q %q lacks %q", stdout, stderr, tt.wantOut)
			}
		})
	}
}
