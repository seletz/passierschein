// Package policy checks a request against the configuration before
// anything else happens (F3: D12, D24, D28, D47).
//
// The check is all or nothing: if one reference breaks a rule, the whole
// request fails. A request that fails the check never reaches the backend
// or the human.
package policy

import (
	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/protocol"
)

// notAllowedMessage is the one text of each not_allowed answer. An agent
// cannot learn from the answer whether a vault exists, whether the
// configuration lists it, or which rule failed (D27, T3).
const notAllowedMessage = "the policy does not allow the request"

// Check checks the references of one request for the project of the
// session. project is the verified project of the session; it never comes
// from a claim of the agent. An empty project means that the working
// directory belongs to no project.
//
// Check returns nil if each reference passes all rules:
//   - the configuration lists the vault (exact name, D12),
//   - the project allows the vault (D28),
//   - the environment of the vault is dev (D24, D47).
//
// It returns config_invalid for each request of a project with an invalid
// configuration, and not_allowed if one reference breaks a rule.
func Check(cfg *config.Config, project string, refs []protocol.Reference) *protocol.Error {
	if cfg == nil || len(refs) == 0 {
		return protocol.Errorf(protocol.CodeNotAllowed, notAllowedMessage)
	}
	p, ok := cfg.Project(project)
	if !ok || project == "" {
		return protocol.Errorf(protocol.CodeNotAllowed, notAllowedMessage)
	}
	if p.Invalid != "" {
		return protocol.Errorf(protocol.CodeConfigInvalid,
			"the configuration of the project is invalid; the human must correct it")
	}
	for _, ref := range refs {
		if !allowed(cfg, p, ref.Vault) {
			return protocol.Errorf(protocol.CodeNotAllowed, notAllowedMessage)
		}
	}
	return nil
}

// allowed reports whether the project can get values from the vault.
func allowed(cfg *config.Config, p config.Project, vault string) bool {
	v, ok := cfg.Vault(vault)
	if !ok {
		return false
	}
	// The environment check is exact. A missing or unknown environment
	// is never dev (D47).
	return p.Allows(vault) && v.Environment == config.EnvDev
}
