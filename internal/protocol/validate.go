package protocol

import (
	"unicode/utf8"
)

// invalid returns an *Error with the code invalid_request.
func invalid(format string, args ...any) *Error {
	return Errorf(CodeInvalidRequest, format, args...)
}

// checkID checks a correlation ID, a session ID or a grant ID: 1 to
// MaxIDLength characters from letters, digits, ".", "_", ":" and "-".
func checkID(name, value string) error {
	if value == "" {
		return invalid("the field %s is empty", name)
	}
	if len(value) > MaxIDLength {
		return invalid("the field %s is longer than %d characters", name, MaxIDLength)
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '.', c == '_', c == ':', c == '-':
		default:
			return invalid("the field %s contains a character that is not allowed", name)
		}
	}
	return nil
}

// checkText checks a text that a human reads. multiline permits line feed
// and tab.
func checkText(name, value string, maxChars int, multiline bool) error {
	if !utf8.ValidString(value) {
		return invalid("the field %s is not valid UTF-8", name)
	}
	if utf8.RuneCountInString(value) > maxChars {
		return invalid("the field %s is longer than %d characters", name, maxChars)
	}
	for _, r := range value {
		if multiline && (r == '\n' || r == '\t') {
			continue
		}
		if IsControl(r) {
			return invalid("the field %s contains a control character", name)
		}
	}
	return nil
}

// checkReferences checks a list of references.
func checkReferences(refs []Reference, minimum int) error {
	if len(refs) < minimum {
		return invalid("the field references is empty")
	}
	if len(refs) > MaxReferences {
		return invalid("the field references has more than %d entries", MaxReferences)
	}
	for _, ref := range refs {
		if err := ref.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// firstError returns the first error that is not nil.
func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// Validate checks the message. The rules of the configuration for the
// session name are not part of this check; a control character in the name
// is always invalid.
func (m *Hello) Validate() error {
	return firstError(
		checkText("session_name", m.SessionName, MaxSessionNameLength, false),
		checkText("agent", m.Agent, MaxAgentLength, false),
	)
}

// Validate checks the message.
func (m *Welcome) Validate() error {
	return firstError(
		checkID("session_id", m.SessionID),
		checkText("project", m.Project, MaxClaimLength, false),
	)
}

// Validate checks the message.
func (m *RequestCredentials) Validate() error {
	return firstError(
		checkID("id", m.ID),
		checkReferences(m.References, 1),
		checkText("task", m.Task, MaxClaimLength, true),
		checkText("reason", m.Reason, MaxClaimLength, true),
	)
}

// Validate checks the message.
func (m *Credentials) Validate() error {
	if err := checkID("id", m.ID); err != nil {
		return err
	}
	if len(m.Entries) == 0 || len(m.Entries) > MaxReferences {
		return invalid("the field entries must have 1 to %d entries", MaxReferences)
	}
	for _, e := range m.Entries {
		if err := e.Reference.Validate(); err != nil {
			return err
		}
		if (e.Value == nil) == (e.Error == nil) {
			return invalid("an entry must have exactly one of value and error")
		}
		if e.Error != nil {
			if err := e.Error.validate(); err != nil {
				return err
			}
		}
	}
	return nil
}

// Validate checks the message.
func (m *GrantStatus) Validate() error { return checkID("id", m.ID) }

// Validate checks the message.
func (m *Grants) Validate() error {
	if err := checkID("id", m.ID); err != nil {
		return err
	}
	for _, g := range m.Grants {
		err := firstError(
			checkID("grant_id", g.GrantID),
			checkReferences(g.References, 0),
			checkText("session_name", g.SessionName, MaxSessionNameLength, false),
			checkText("project", g.Project, MaxClaimLength, false),
		)
		if err == nil && g.SessionID != "" {
			err = checkID("session_id", g.SessionID)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Validate checks the message.
func (m *ErrorResponse) Validate() error {
	if m.ID != "" {
		if err := checkID("id", m.ID); err != nil {
			return err
		}
	}
	return m.Error.validate()
}

// Validate checks the message.
func (m *GrantsList) Validate() error { return checkID("id", m.ID) }

// Validate checks the message.
func (m *GrantRevoke) Validate() error {
	return firstError(checkID("id", m.ID), checkID("grant_id", m.GrantID))
}

// Validate checks the message.
func (m *GrantRevoked) Validate() error {
	return firstError(checkID("id", m.ID), checkID("grant_id", m.GrantID))
}

// Validate checks the message.
func (m *Status) Validate() error { return checkID("id", m.ID) }

// Validate checks the message.
func (m *StatusResult) Validate() error {
	if err := checkID("id", m.ID); err != nil {
		return err
	}
	if m.Fault != nil {
		if !m.Fault.Code.Valid() {
			return invalid("the fault code is not a known code")
		}
		if err := firstError(
			checkText("reason", m.Fault.Reason, MaxClaimLength, true),
			checkText("finding", m.Fault.Finding, MaxClaimLength, true),
		); err != nil {
			return err
		}
	}
	for _, p := range m.InvalidProjects {
		err := firstError(
			checkText("project", p.Project, MaxClaimLength, false),
			checkText("reason", p.Reason, MaxClaimLength, true),
		)
		if err != nil {
			return err
		}
	}
	if m.Backend != nil {
		switch m.Backend.State {
		case BackendOK, BackendLocked, BackendUnreachable:
		default:
			return invalid("the backend state is not a known state")
		}
		if err := checkText("backend reason", m.Backend.Reason, MaxClaimLength, true); err != nil {
			return err
		}
		for _, a := range m.Backend.Accounts {
			switch a.State {
			case BackendOK, BackendLocked, BackendUnreachable:
			default:
				return invalid("the state of an account is not a known state")
			}
			if a.Name == "" {
				return invalid("an account of the backend has no name")
			}
			if err := firstError(
				checkText("account name", a.Name, MaxClaimLength, false),
				checkText("account reason", a.Reason, MaxClaimLength, true),
			); err != nil {
				return err
			}
		}
	}
	if m.UI != nil {
		if err := firstError(
			checkText("ui mode", m.UI.Mode, MaxClaimLength, false),
			checkText("ui reason", m.UI.Reason, MaxClaimLength, true),
			checkText("ui app_path", m.UI.AppPath, MaxClaimLength, true),
		); err != nil {
			return err
		}
	}
	return checkText("daemon_version", m.DaemonVersion, MaxClaimLength, false)
}

// Validate checks the message.
func (m *FaultClear) Validate() error { return checkID("id", m.ID) }

// Validate checks the message.
func (m *FaultClearResult) Validate() error {
	return firstError(checkID("id", m.ID), checkText("finding", m.Finding, MaxClaimLength, true))
}
