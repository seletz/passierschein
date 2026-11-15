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
	err := SelfTest(context.Background())
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
