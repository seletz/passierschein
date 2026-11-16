package approval

import (
	"errors"
	"fmt"
	"path"
	"unicode/utf8"

	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/protocol"
)

// MaxGrantLifetimeSeconds is the longest lifetime of a grant (D66: 24h).
const MaxGrantLifetimeSeconds = 24 * 3600

// ExecCommand is the command of an approval request of the exec way (D65,
// D61). The daemon copies Program, Args and Cwd from the message
// request_exec without a change. Env names the variables; each variable
// points to one entry of Request.Credentials, which holds the names of the
// backend.
type ExecCommand struct {
	// Environment is the strictest environment of the credentials: prod
	// before test before dev (D62). The app highlights test and prod.
	Environment config.Environment `json:"environment"`
	// Program is the absolute, clean path of the program.
	Program string `json:"program"`
	// Args lists the arguments without the program. The app shows each
	// argument in full.
	Args []string `json:"args"`
	// Cwd is the absolute, clean path of the working directory.
	Cwd string `json:"cwd"`
	// Env lists the environment variables in the order of the request.
	Env []ExecVariable `json:"env"`
	// Approval is the kind of approval that the human gives.
	Approval ExecApproval `json:"approval"`
}

// ExecVariable is one environment variable of a command.
type ExecVariable struct {
	// Name is the name of the variable (protocol.CheckEnvName).
	Name string `json:"name"`
	// CredentialID is the ID of the entry of Request.Credentials whose
	// value the variable gets. Two variables can point to one entry.
	CredentialID string `json:"credential_id"`
}

// ExecApprovalKind is the kind of approval for a command (D65, D63).
type ExecApprovalKind string

// The kinds of approval for a command.
const (
	// ExecApprovalGrant: the approval creates a grant with a lifetime and
	// a binding (dev, test with a lifetime more than 0).
	ExecApprovalGrant ExecApprovalKind = "grant"
	// ExecApprovalOnce: the approval is for this command only, one time
	// (lifetime 0: always in prod, in dev and test if the configuration
	// sets the lifetime 0). The human selects no lifetime.
	ExecApprovalOnce ExecApprovalKind = "once"
)

// Binding says for which commands a grant is valid (D63).
type Binding string

// The bindings of a grant.
const (
	// BindingNone: the grant is valid for each way and each command.
	BindingNone Binding = "none"
	// BindingProgram: the grant is valid for the same program path and the
	// same variable names; the arguments are free.
	BindingProgram Binding = "program"
	// BindingCommand: the grant is valid for the same program, arguments,
	// working directory and variable names.
	BindingCommand Binding = "command"
)

// ExecApproval is the kind of approval for a command.
//
// For the kind grant, Request.Grant.TTLSeconds is the configured lifetime
// and LifetimeChoicesSeconds lists the lifetimes that the human can
// select: the first value is the configured lifetime, the other values are
// shorter, in descending order. The daemon sends the effective binding: for
// a program without a statement (a shell, an interpreter, a starter) it
// sends command, not program (D63).
//
// For the kind once, Request.Grant.TTLSeconds is 0, the binding is command
// and the list of lifetimes is empty.
type ExecApproval struct {
	// Kind is grant or once.
	Kind ExecApprovalKind `json:"kind"`
	// Binding is the binding of the grant.
	Binding Binding `json:"binding"`
	// LifetimeChoicesSeconds lists the lifetimes in seconds that the
	// human can select. It is empty for the kind once.
	LifetimeChoicesSeconds []int64 `json:"lifetime_choices_seconds,omitempty"`
}

// ExecLifetimeChoices returns the lifetimes that the approval request of a
// command offers for the configured lifetime ttl: ttl, half of ttl
// (rounded down to minutes), 15 minutes and 5 minutes. Each value is at
// least five minutes and shorter than ttl, except ttl itself. The result is
// in descending order. It is nil for a ttl of 0 or less.
func ExecLifetimeChoices(ttl int64) []int64 {
	if ttl <= 0 {
		return nil
	}
	out := []int64{ttl}
	half := (ttl / 2) / 60 * 60
	for _, c := range []int64{half, 15 * 60, 5 * 60} {
		if c >= 5*60 && c < out[len(out)-1] {
			out = append(out, c)
		}
	}
	return out
}

// envRank orders the environments: prod is the strictest (D62).
func envRank(e config.Environment) int {
	switch e {
	case config.EnvDev:
		return 1
	case config.EnvTest:
		return 2
	case config.EnvProd:
		return 3
	}
	return 0
}

// Validate checks a request before a UI shows it. It checks the rules of
// the exec way (D62, D63, D65, D66): a request without Exec contains only
// credentials of dev; a request with Exec names the strictest environment,
// a complete command, one variable or more for each entry and a kind of
// approval that fits the environment.
//
// The error text names fields only, never a value of the request.
func (r Request) Validate() error {
	strictest := config.Environment("")
	for i, c := range r.Credentials {
		if envRank(c.Environment) == 0 {
			return fmt.Errorf("credentials[%d]: the environment is not dev, test or prod", i)
		}
		if envRank(c.Environment) > envRank(strictest) {
			strictest = c.Environment
		}
	}
	if r.Exec == nil {
		if strictest != config.EnvDev && strictest != "" {
			return errors.New("a request outside dev needs the exec way (D62)")
		}
		return nil
	}
	return r.Exec.validate(r, strictest)
}

func (e *ExecCommand) validate(r Request, strictest config.Environment) error {
	if envRank(e.Environment) == 0 {
		return errors.New("exec.environment: the environment is not dev, test or prod")
	}
	if e.Environment != strictest {
		return errors.New("exec.environment: the environment is not the strictest environment of the credentials")
	}
	if err := checkExecPath("exec.program", e.Program); err != nil {
		return err
	}
	if err := checkExecPath("exec.cwd", e.Cwd); err != nil {
		return err
	}
	if err := checkExecArgs(e.Args); err != nil {
		return err
	}
	if err := e.checkEnv(r.Credentials); err != nil {
		return err
	}
	return e.Approval.check(e.Environment, r.Grant.TTLSeconds)
}

func checkExecPath(field, p string) error {
	if p == "" || len(p) > protocol.MaxPathLength || !utf8.ValidString(p) {
		return fmt.Errorf("%s: the path is empty, too long or not UTF-8", field)
	}
	for _, c := range p {
		if protocol.IsControl(c) {
			return fmt.Errorf("%s: the path contains a control character", field)
		}
	}
	if !path.IsAbs(p) || path.Clean(p) != p {
		return fmt.Errorf("%s: the path is not absolute and clean", field)
	}
	return nil
}

func checkExecArgs(args []string) error {
	if len(args) > protocol.MaxExecArgs {
		return fmt.Errorf("exec.args: more than %d arguments", protocol.MaxExecArgs)
	}
	size := 0
	for i, a := range args {
		size += len(a)
		if size > protocol.MaxExecArgsSize {
			return fmt.Errorf("exec.args: the arguments are longer than %d bytes together", protocol.MaxExecArgsSize)
		}
		if !utf8.ValidString(a) {
			return fmt.Errorf("exec.args[%d]: the argument is not UTF-8", i)
		}
		for _, c := range a {
			if c != '\n' && c != '\t' && protocol.IsControl(c) {
				return fmt.Errorf("exec.args[%d]: the argument contains a control character", i)
			}
		}
	}
	return nil
}

func (e *ExecCommand) checkEnv(creds []Credential) error {
	if len(e.Env) == 0 || len(e.Env) > protocol.MaxReferences {
		return fmt.Errorf("exec.env: the command needs 1 to %d variables", protocol.MaxReferences)
	}
	known := make(map[string]bool, len(creds))
	for _, c := range creds {
		known[c.ID] = true
	}
	names := make(map[string]bool, len(e.Env))
	used := make(map[string]bool, len(creds))
	for i, v := range e.Env {
		if err := protocol.CheckEnvName(v.Name); err != nil {
			return fmt.Errorf("exec.env[%d]: %w", i, err)
		}
		if names[v.Name] {
			return fmt.Errorf("exec.env[%d]: the variable name appears twice", i)
		}
		names[v.Name] = true
		if !known[v.CredentialID] {
			return fmt.Errorf("exec.env[%d]: the credential is not in the request", i)
		}
		used[v.CredentialID] = true
	}
	for i, c := range creds {
		if !used[c.ID] {
			return fmt.Errorf("credentials[%d]: the credential has no variable", i)
		}
	}
	return nil
}

func (a ExecApproval) check(env config.Environment, ttl int64) error {
	switch a.Kind {
	case ExecApprovalOnce:
		if a.Binding != BindingCommand {
			return errors.New("exec.approval.binding: once needs the binding command")
		}
		if ttl != 0 || len(a.LifetimeChoicesSeconds) != 0 {
			return errors.New("exec.approval: once has no lifetime and no lifetime choices")
		}
		return nil
	case ExecApprovalGrant:
		if env == config.EnvProd {
			return errors.New("exec.approval.kind: prod allows only the kind once")
		}
	default:
		return errors.New("exec.approval.kind: the kind is not grant or once")
	}
	switch a.Binding {
	case BindingNone, BindingProgram, BindingCommand:
	default:
		return errors.New("exec.approval.binding: the binding is not none, program or command")
	}
	if env == config.EnvTest && a.Binding == BindingNone {
		return errors.New("exec.approval.binding: test does not allow the binding none")
	}
	if ttl <= 0 || ttl > MaxGrantLifetimeSeconds {
		return fmt.Errorf("grant.ttl_seconds: a grant needs a lifetime of 1 to %d seconds", MaxGrantLifetimeSeconds)
	}
	c := a.LifetimeChoicesSeconds
	if len(c) == 0 || c[0] != ttl {
		return errors.New("exec.approval.lifetime_choices_seconds: the first choice is not the lifetime of the grant")
	}
	for i := 1; i < len(c); i++ {
		if c[i] <= 0 || c[i] >= c[i-1] {
			return errors.New("exec.approval.lifetime_choices_seconds: the choices are not shorter lifetimes in descending order")
		}
	}
	return nil
}

// validateExecDecision checks an approval of a command: it covers every
// entry, and the lifetime is one of the offered choices (grant) or 0
// (once).
func validateExecDecision(req Request, d Decision) error {
	if len(d.ApprovedCredentialIDs) != len(req.Credentials) {
		return errors.New("the approval of a command does not cover every variable")
	}
	switch req.Exec.Approval.Kind {
	case ExecApprovalOnce:
		if d.TTLSeconds != 0 {
			return fmt.Errorf("the lifetime %d is not valid for one command", d.TTLSeconds)
		}
		return nil
	case ExecApprovalGrant:
		for _, c := range req.Exec.Approval.LifetimeChoicesSeconds {
			if c == d.TTLSeconds && c > 0 {
				return nil
			}
		}
		return fmt.Errorf("the lifetime %d is not one of the offered lifetimes", d.TTLSeconds)
	}
	return errors.New("the request has no valid kind of approval")
}
