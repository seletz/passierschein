package token

import (
	"strings"
	"testing"
)

func TestCheckShape(t *testing.T) {
	// D51, issue 59: the form check of "a38 config onepassword-token set". No error and no
	// warning contains the token.
	long := ExpectedPrefix + strings.Repeat("a", MaxLength)
	tests := []struct {
		name     string
		in       string
		wantErr  string
		wantWarn bool
	}{
		{"good", marker, "", false},
		{"no prefix", "beispiel-marker-ohne-praefix", "", true},
		{"empty", "", "empty", false},
		{"too short", "ops_kurz", "too short", false},
		{"too long", long, "too long", false},
		{"space", "ops_beispiel marker-7f3a", "white space", false},
		{"tab", "ops_beispiel\tmarker-7f3a", "white space", false},
		{"inner newline", "ops_beispiel\nmarker-7f3a", "white space", false},
		{"control character", "ops_beispiel\x01marker-7f3a", "control character", false},
		{"escape", "ops_beispiel\x1b[2Jmarker", "control character", false},
		{"invalid utf-8", "ops_beispiel\xffmarker-7f3a", "UTF-8", false},
		{"non-breaking space", "ops_beispiel marker-7f3a", "white space", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warns, err := CheckShape(tt.in)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("error = %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
			if got := len(warns) > 0; got != tt.wantWarn {
				t.Errorf("warnings = %q, want some: %v", warns, tt.wantWarn)
			}
			text := strings.Join(warns, " ")
			if err != nil {
				text += err.Error()
			}
			if tt.in != "" && strings.Contains(text, "beispiel") {
				t.Errorf("output contains the token: %q", text)
			}
		})
	}
}

func TestSignatureStable(t *testing.T) {
	for kind, want := range map[SignatureKind]bool{
		SignatureDeveloperID: true, SignatureAdhoc: false, SignatureNone: false,
		SignatureOther: false, SignatureUnknown: false,
	} {
		if got := (Signature{Kind: kind}).Stable(); got != want {
			t.Errorf("%s: Stable() = %v", kind, got)
		}
	}
}
