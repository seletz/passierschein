package protocol

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// referenceScheme is the prefix of each credential reference.
const referenceScheme = "op://"

// MaxReferenceLength is the maximum length of a credential reference in
// bytes.
const MaxReferenceLength = 512

// Reference names one credential: a field of an item in a vault. The text
// form is "op://<vault>/<item>/<field>".
//
// A Reference holds the text that the agent wrote. The names are not
// verified. The backend resolves a reference to the stored names; the
// approval request shows those, not this text.
type Reference struct {
	// Vault is the name of the vault.
	Vault string
	// Item is the name of the item.
	Item string
	// Field is the name of the field.
	Field string
}

// ParseReference parses the text form of a reference. The form has exactly
// three segments. A segment is not empty, has no "/" and no control
// character, and has no white space at its start or end. A failure is an
// *Error with the code invalid_request. The error text does not repeat the
// input.
func ParseReference(s string) (Reference, error) {
	if len(s) > MaxReferenceLength {
		return Reference{}, invalid("the reference is longer than %d bytes", MaxReferenceLength)
	}
	rest, ok := strings.CutPrefix(s, referenceScheme)
	if !ok {
		return Reference{}, invalid("a reference must start with %q", referenceScheme)
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 3 {
		return Reference{}, invalid("a reference must have the form op://<vault>/<item>/<field>")
	}
	ref := Reference{Vault: parts[0], Item: parts[1], Field: parts[2]}
	if err := ref.Validate(); err != nil {
		return Reference{}, err
	}
	return ref, nil
}

// Validate checks the segments of the reference. A failure is an *Error
// with the code invalid_request.
func (r Reference) Validate() error {
	total := len(referenceScheme) + 2
	for _, seg := range [...]struct{ name, value string }{
		{"vault", r.Vault}, {"item", r.Item}, {"field", r.Field},
	} {
		total += len(seg.value)
		switch {
		case seg.value == "":
			return invalid("the %s of a reference is empty", seg.name)
		case !utf8.ValidString(seg.value):
			return invalid("the %s of a reference is not valid UTF-8", seg.name)
		case strings.Contains(seg.value, "/"):
			return invalid("the %s of a reference contains a slash", seg.name)
		case strings.TrimSpace(seg.value) != seg.value:
			return invalid("the %s of a reference starts or ends with white space", seg.name)
		case strings.ContainsFunc(seg.value, IsControl):
			return invalid("the %s of a reference contains a control character", seg.name)
		}
	}
	if total > MaxReferenceLength {
		return invalid("the reference is longer than %d bytes", MaxReferenceLength)
	}
	return nil
}

// String returns the text form "op://<vault>/<item>/<field>".
func (r Reference) String() string {
	return referenceScheme + r.Vault + "/" + r.Item + "/" + r.Field
}

// MarshalText writes the text form. It fails for an invalid reference.
func (r Reference) MarshalText() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return []byte(r.String()), nil
}

// UnmarshalText parses the text form.
func (r *Reference) UnmarshalText(text []byte) error {
	ref, err := ParseReference(string(text))
	if err != nil {
		return err
	}
	*r = ref
	return nil
}

// IsControl reports whether r is a character that must not appear in a
// name or in a text that a human reads: a control character, a format
// character (this includes the marks for the writing direction), a line or
// paragraph separator, or the Unicode replacement character.
func IsControl(r rune) bool {
	return unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) || r == utf8.RuneError
}
