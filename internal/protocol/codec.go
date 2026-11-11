package protocol

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

// maxDepth is the maximum nesting depth of a message.
const maxDepth = 6

// Set is a fixed set of message types: the messages that one side of one
// socket accepts. Decode with the set of the socket and the direction, so
// that a message of the other socket is invalid.
type Set struct {
	name  string
	types map[Type]func() Message
}

// The four sets of the protocol.
var (
	// AgentRequests are the messages that the daemon accepts on the agent
	// socket.
	AgentRequests = Set{"agent request", map[Type]func() Message{
		TypeHello:              func() Message { return new(Hello) },
		TypeRequestCredentials: func() Message { return new(RequestCredentials) },
		TypeGrantStatus:        func() Message { return new(GrantStatus) },
	}}
	// AgentResponses are the messages that the daemon sends on the agent
	// socket.
	AgentResponses = Set{"agent response", map[Type]func() Message{
		TypeWelcome:     func() Message { return new(Welcome) },
		TypeCredentials: func() Message { return new(Credentials) },
		TypeGrants:      func() Message { return new(Grants) },
		TypeError:       func() Message { return new(ErrorResponse) },
	}}
	// AdminRequests are the messages that the daemon accepts on the admin
	// socket.
	AdminRequests = Set{"admin request", map[Type]func() Message{
		TypeGrantsList:  func() Message { return new(GrantsList) },
		TypeGrantRevoke: func() Message { return new(GrantRevoke) },
		TypeStatus:      func() Message { return new(Status) },
		TypeFaultClear:  func() Message { return new(FaultClear) },
	}}
	// AdminResponses are the messages that the daemon sends on the admin
	// socket.
	AdminResponses = Set{"admin response", map[Type]func() Message{
		TypeGrants:           func() Message { return new(Grants) },
		TypeGrantRevoked:     func() Message { return new(GrantRevoked) },
		TypeStatusResult:     func() Message { return new(StatusResult) },
		TypeFaultClearResult: func() Message { return new(FaultClearResult) },
		TypeError:            func() Message { return new(ErrorResponse) },
	}}
)

// Types returns the message types of the set. The sequence is not defined.
func (s Set) Types() []Type {
	out := make([]Type, 0, len(s.types))
	for t := range s.types {
		out = append(out, t)
	}
	return out
}

// Decode reads one message from one line. The line end is optional.
//
// Decode is strict. It rejects: a line above the size limit, text that is
// not one JSON object, a key that is not in lower case or that appears
// twice, a wrong version, a type that is not in the set, an unknown field
// and content that fails Validate. Each failure is an *Error with the code
// invalid_request. The error text never repeats content of the line.
func (s Set) Decode(line []byte) (Message, error) {
	if len(line) > MaxMessageSize {
		return nil, invalid("the message is larger than %d bytes", MaxMessageSize)
	}
	line = bytes.TrimRight(line, "\r\n")
	if bytes.ContainsAny(line, "\r\n") {
		return nil, invalid("the message has a line break")
	}
	if !utf8.Valid(line) {
		return nil, invalid("the message is not valid UTF-8")
	}
	if err := checkShape(line); err != nil {
		return nil, err
	}
	var head struct {
		Version *json.Number `json:"version"`
		Type    *string      `json:"type"`
	}
	if err := json.Unmarshal(line, &head); err != nil {
		return nil, invalid("the fields version and type do not have the correct JSON types")
	}
	if head.Version == nil || head.Version.String() != "1" {
		return nil, invalid("the version of the message is not %d", Version)
	}
	if head.Type == nil {
		return nil, invalid("the message has no type")
	}
	create, ok := s.types[Type(*head.Type)]
	if !ok {
		return nil, invalid("the type of the message is not a known %s", s.name)
	}
	msg := create()
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.DisallowUnknownFields()
	if err := dec.Decode(msg); err != nil {
		// A field that failed its own check (a reference, a secret) has an
		// *Error with a safe text. The text of each other error can repeat
		// content of the line.
		var perr *Error
		if errors.As(err, &perr) {
			return nil, perr
		}
		return nil, invalid("the message has an unknown field or a field with a wrong JSON type")
	}
	if err := msg.Validate(); err != nil {
		return nil, err
	}
	return msg, nil
}

// checkShape checks the JSON structure of a line: exactly one object, each
// key in lower-case ASCII, no key twice in one object, limited depth.
// Package encoding/json matches keys without regard to case and accepts a
// repeated key; this check closes both.
func checkShape(line []byte) error {
	type frame struct {
		object  bool
		wantKey bool
		keys    map[string]struct{}
	}
	var stack []frame
	started, done := false, false
	valueDone := func() {
		if len(stack) == 0 {
			done = true
			return
		}
		if top := &stack[len(stack)-1]; top.object {
			top.wantKey = true
		}
	}
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.UseNumber()
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return invalid("the message is not valid JSON")
		}
		if done {
			return invalid("the line has text after the JSON object")
		}
		if !started {
			if tok != json.Delim('{') {
				return invalid("the message is not a JSON object")
			}
			started = true
		}
		switch t := tok.(type) {
		case json.Delim:
			switch t {
			case '{', '[':
				if len(stack) >= maxDepth {
					return invalid("the message is nested too deep")
				}
				stack = append(stack, frame{object: t == '{', wantKey: t == '{', keys: map[string]struct{}{}})
			default:
				stack = stack[:len(stack)-1]
				valueDone()
			}
		case string:
			top := &stack[len(stack)-1]
			if !top.object || !top.wantKey {
				valueDone()
				continue
			}
			if !isFieldName(t) {
				return invalid("a key of the message is not a lower-case field name")
			}
			if _, twice := top.keys[t]; twice {
				return invalid("a key of the message appears twice")
			}
			top.keys[t] = struct{}{}
			top.wantKey = false
		default:
			valueDone()
		}
	}
	if !done {
		return invalid("the message is not a complete JSON object")
	}
	return nil
}

// isFieldName reports whether s has the form of a field name of the
// protocol: lower-case ASCII letters, digits and "_".
func isFieldName(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9' || i == 0) && (c != '_' || i == 0) {
			return false
		}
	}
	return true
}

// wireCredentials is the wire form of Credentials. It is the only place
// that writes a secret value.
type wireCredentials struct {
	Header
	ID      string      `json:"id"`
	Entries []wireEntry `json:"entries"`
}

type wireEntry struct {
	Reference Reference `json:"reference"`
	Value     *string   `json:"value,omitempty"`
	Error     *Error    `json:"error,omitempty"`
}

// Encode returns the message as one line of JSON without the line end. It
// sets the version and the type. It fails if the message is invalid or
// larger than the size limit.
//
// Encode is the only function that writes the value of a [Secret].
func Encode(m Message) ([]byte, error) {
	if m == nil {
		return nil, errors.New("protocol: cannot encode a nil message")
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	h := m.header()
	h.Version, h.Type = Version, m.MessageType()
	var payload any = m
	if c, ok := m.(*Credentials); ok {
		w := wireCredentials{Header: c.Header, ID: c.ID, Entries: make([]wireEntry, len(c.Entries))}
		for i, e := range c.Entries {
			w.Entries[i] = wireEntry{Reference: e.Reference, Error: e.Error}
			if e.Value != nil {
				v := e.Value.Reveal()
				w.Entries[i].Value = &v
			}
		}
		payload = w
	}
	line, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.New("protocol: cannot encode the message")
	}
	if len(line)+1 > MaxMessageSize {
		return nil, invalid("the message is larger than %d bytes", MaxMessageSize)
	}
	return line, nil
}

// Write encodes the message and writes it with the line end in one call to
// w.Write. A caller that shares w between goroutines must serialize the
// calls.
func Write(w io.Writer, m Message) error {
	line, err := Encode(m)
	if err != nil {
		return err
	}
	_, err = w.Write(append(line, '\n'))
	return err
}

// ReadLine reads the next line from r and returns it without the line end.
//
// It returns io.EOF at a clean end of the stream and io.ErrUnexpectedEOF if
// the stream ends in the middle of a line. A line above the size limit is
// an *Error with the code invalid_request; the stream position is then not
// defined, and the caller must close the connection.
func ReadLine(r *bufio.Reader) ([]byte, error) {
	var line []byte
	for {
		part, err := r.ReadSlice('\n')
		if len(line)+len(part) > MaxMessageSize {
			return nil, invalid("the message is larger than %d bytes", MaxMessageSize)
		}
		line = append(line, part...)
		switch {
		case err == nil:
			return bytes.TrimRight(line, "\r\n"), nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF):
			if len(line) > 0 {
				return nil, io.ErrUnexpectedEOF
			}
			return nil, io.EOF
		default:
			return nil, err
		}
	}
}
