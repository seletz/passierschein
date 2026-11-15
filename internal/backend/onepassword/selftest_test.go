package onepassword

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
)

func TestCheckSelfTestAnswer(t *testing.T) {
	// T30: the self-test passes only if the core of the SDK ran and
	// rejected the invalid token. Any other answer is a failure.
	tests := []struct {
		name    string
		err     error
		wantErr string
	}{
		{"the core rejects the token", errors.New("error initializing client: invalid service account token, please make sure you provide a valid service account token as parameter:  service account token base64 decoding failed"), ""},
		{"the core accepts the token", nil, "accepted the invalid token"},
		{"the core rejects the format", errors.New("error initializing client: invalid user input: encountered the following errors: service account token had invalid format"), ""},
		{"another error of the core", errors.New("error initializing client: failed to initialize plugin: x"), "did not reject the invalid token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkSelfTestAnswer(tt.err)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("checkSelfTestAnswer = %v, want nil", err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Fatalf("checkSelfTestAnswer = %v, want an error with %q", err, tt.wantErr)
			}
		})
	}
}

func TestSelfTest(t *testing.T) {
	// T30: the core of the SDK runs in this process with an invalid
	// token, without network. The unsigned test binary has no hardened
	// runtime; the signed binary is checked by scripts/sdk-core-check.sh.
	err := SelfTest(context.Background(), "0.1.0-SNAPSHOT-322e7ba")
	if runtime.GOOS != "darwin" {
		if !errors.Is(err, errNotAvailable) {
			t.Fatalf("SelfTest = %v, want errNotAvailable", err)
		}
		return
	}
	if err != nil {
		t.Fatalf("SelfTest = %v, want nil", err)
	}
}

func TestSDKRejectsRawSnapshotVersion(t *testing.T) {
	// Issue 89: the real SDK core rejects the unmapped version of a
	// snapshot, and classify gives the error its own class. Invalid token,
	// no network.
	if runtime.GOOS != "darwin" {
		t.Skip("the SDK is part of the macOS build only")
	}
	tests := []struct {
		version string
		class   string
	}{
		{"0.1.0-SNAPSHOT-322e7ba", classIntegration},
		{"1.2.3+build", classIntegration},
		{integrationVersion("0.1.0-SNAPSHOT-322e7ba"), classAuth},
		{integrationVersion("1.2.3+build"), classAuth},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			err := selfTestConnect(context.Background(), tt.version)
			if err == nil {
				t.Fatal("the SDK accepted the invalid token")
			}
			if got := classify("connect", err).Class; got != tt.class {
				t.Errorf("class = %s, want %s", got, tt.class)
			}
		})
	}
}
