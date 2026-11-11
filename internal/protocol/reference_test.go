package protocol

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestParseReference(t *testing.T) {
	// T24 (part): the invalid forms of a credential reference.
	tests := []struct {
		name    string
		in      string
		want    Reference
		wantErr bool
	}{
		{"valid", "op://beispiel.app.k8s-dev/datenbank-dev/password",
			Reference{Vault: "beispiel.app.k8s-dev", Item: "datenbank-dev", Field: "password"}, false},
		{"valid with spaces", "op://beispiel.app.k8s-dev/Datenbank dev/user name",
			Reference{Vault: "beispiel.app.k8s-dev", Item: "Datenbank dev", Field: "user name"}, false},
		{"no field", "op://beispiel.app.k8s-dev/datenbank-dev", Reference{}, true},
		{"one segment too many", "op://beispiel.app.k8s-dev/datenbank-dev/abschnitt/password", Reference{}, true},
		{"other scheme", "https://beispiel.app.k8s-dev/datenbank-dev/password", Reference{}, true},
		{"scheme in upper case", "OP://beispiel.app.k8s-dev/datenbank-dev/password", Reference{}, true},
		{"no scheme", "beispiel.app.k8s-dev/datenbank-dev/password", Reference{}, true},
		{"empty vault", "op:///datenbank-dev/password", Reference{}, true},
		{"empty item", "op://beispiel.app.k8s-dev//password", Reference{}, true},
		{"empty field", "op://beispiel.app.k8s-dev/datenbank-dev/", Reference{}, true},
		{"empty text", "", Reference{}, true},
		{"control character", "op://beispiel.app.k8s-dev/datenbank-dev/pass\nword", Reference{}, true},
		{"direction mark", "op://beispiel.app.k8s-dev/datenbank\u202edev/password", Reference{}, true},
		{"invalid UTF-8", "op://beispiel.app.k8s-dev/datenbank-dev/\xff", Reference{}, true},
		{"space around a segment", "op://beispiel.app.k8s-dev/ datenbank-dev/password", Reference{}, true},
		{"too long", "op://beispiel.app.k8s-dev/datenbank-dev/" + strings.Repeat("a", MaxReferenceLength), Reference{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseReference(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseReference(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseReference(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
			if err != nil {
				var perr *Error
				if !errors.As(err, &perr) || perr.Code != CodeInvalidRequest {
					t.Errorf("error = %v, want an *Error with code %q", err, CodeInvalidRequest)
				}
				return
			}
			if got.String() != tt.in {
				t.Errorf("String() = %q, want %q", got.String(), tt.in)
			}
		})
	}
}

func TestReferenceJSON(t *testing.T) {
	ref := Reference{Vault: "beispiel.app.k8s-dev", Item: "datenbank-dev", Field: "password"}
	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatal(err)
	}
	if want := `"op://beispiel.app.k8s-dev/datenbank-dev/password"`; string(data) != want {
		t.Errorf("Marshal = %s, want %s", data, want)
	}
	var back Reference
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back != ref {
		t.Errorf("round trip = %+v, want %+v", back, ref)
	}
	if err := json.Unmarshal([]byte(`"op://beispiel.app.k8s-dev/datenbank-dev"`), &back); err == nil {
		t.Error("Unmarshal of a reference without field: no error")
	}
	if _, err := json.Marshal(Reference{}); err == nil {
		t.Error("Marshal of the zero reference: no error")
	}
}
