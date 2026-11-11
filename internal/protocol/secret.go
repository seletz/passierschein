package protocol

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log/slog"
)

// redacted is the text that replaces a secret value in each output.
const redacted = "[redacted]"

// Secret holds one secret value. It protects the value against accidental
// output: formatting with package fmt, logging with package slog and
// json.Marshal all give "[redacted]". Only [Secret.Reveal] returns the
// value, and only [Encode] writes it to the wire.
//
// Call Reveal only at the point of delivery. Do not copy the result into a
// string that lives longer than the delivery.
type Secret struct {
	// A pointer, so that fmt prints an address and not the value when a
	// Secret is an unexported field of a different struct.
	v *string
}

// NewSecret returns a Secret that holds value.
func NewSecret(value string) Secret {
	return Secret{v: &value}
}

// Reveal returns the secret value.
func (s Secret) Reveal() string {
	if s.v == nil {
		return ""
	}
	return *s.v
}

// Equal reports whether s and other hold the same value. The comparison
// takes the same time for each content of equal length.
func (s Secret) Equal(other Secret) bool {
	return subtle.ConstantTimeCompare([]byte(s.Reveal()), []byte(other.Reveal())) == 1
}

// String returns "[redacted]".
func (s Secret) String() string { return redacted }

// GoString returns "[redacted]".
func (s Secret) GoString() string { return redacted }

// Format writes "[redacted]" for each verb.
func (s Secret) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte(redacted)) }

// LogValue returns "[redacted]" for package slog.
func (s Secret) LogValue() slog.Value { return slog.StringValue(redacted) }

// MarshalJSON writes "[redacted]". Only [Encode] writes the value.
func (s Secret) MarshalJSON() ([]byte, error) { return json.Marshal(redacted) }

// UnmarshalJSON reads the value from a JSON string.
func (s *Secret) UnmarshalJSON(data []byte) error {
	var value *string
	if err := json.Unmarshal(data, &value); err != nil || value == nil {
		return invalid("a secret value must be a JSON string")
	}
	s.v = value
	return nil
}
