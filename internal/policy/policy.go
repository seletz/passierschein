// Package policy checks a request against the configuration before
// anything else happens (F3: D12, D28, D47; F22: D62).
//
// The check is all or nothing: if one reference breaks a rule, the whole
// request fails. A request that fails the check never reaches the backend
// or the human.
package policy

import (
	"slices"

	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/protocol"
)

// notAllowedMessage is the one text of each not_allowed answer. An agent
// cannot learn from the answer whether a vault exists, whether the
// configuration lists it, or which rule failed (D27, T3).
const notAllowedMessage = "the policy does not allow the request"

// execRequiredMessage is the text of the answer exec_required (D62).
const execRequiredMessage = "the vaults of the request deliver only through the exec way; ask again with run_with_credentials"

// Check checks the references of one request of the MCP way
// (request_credentials). It is [CheckWay] with the way mcp.
func Check(cfg *config.Config, project string, refs []protocol.Reference) *protocol.Error {
	return CheckWay(cfg, project, refs, config.DeliveryMCP)
}

// CheckWay checks the references of one request for the project of the
// session and the delivery way. project is the verified project of the
// session; it never comes from a claim of the agent. An empty project
// means that the working directory belongs to no project.
//
// CheckWay returns nil if each reference passes all rules:
//   - the configuration lists the vault (exact name, D12),
//   - the project allows the vault (D28),
//   - the environment of the vault is dev, test or prod (D47),
//   - the environment of each vault allows the way (D62, D66). The way mcp
//     is for dev only; the configuration cannot change this rule. The way
//     mcp needs a grant lifetime more than 0 (D76).
//
// It returns config_invalid for each request of a project with an invalid
// configuration. For the way mcp, it returns exec_required if each vault
// allows the exec way but one vault does not allow the MCP answer. Each
// other failure is not_allowed, with one text.
func CheckWay(cfg *config.Config, project string, refs []protocol.Reference, way config.Delivery) *protocol.Error {
	if cfg == nil || len(refs) == 0 || !way.Valid() {
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
	var envs []config.Environment
	for _, ref := range refs {
		env, ok := allowed(cfg, p, ref.Vault)
		if !ok {
			return protocol.Errorf(protocol.CodeNotAllowed, notAllowedMessage)
		}
		if !slices.Contains(envs, env) {
			envs = append(envs, env)
		}
	}
	if allowsAll(cfg, envs, way) {
		return nil
	}
	if way == config.DeliveryMCP && allowsAll(cfg, envs, config.DeliveryExec) {
		return protocol.Errorf(protocol.CodeExecRequired, execRequiredMessage)
	}
	return protocol.Errorf(protocol.CodeNotAllowed, notAllowedMessage)
}

// allowsAll reports whether each environment allows the way. The fixed
// rule of D62 holds also here: mcp only for dev. The way mcp also needs a
// lifetime more than 0 (D76).
func allowsAll(cfg *config.Config, envs []config.Environment, way config.Delivery) bool {
	for _, env := range envs {
		if way == config.DeliveryMCP && env != config.EnvDev {
			return false
		}
		// D76: the lifetime 0 gives one delivery for one command. The MCP
		// answer has no command, so it needs a grant with a lifetime.
		if way == config.DeliveryMCP && cfg.GrantLifetime(env) <= 0 {
			return false
		}
		if !cfg.DeliveryAllowed(env, way) {
			return false
		}
	}
	return true
}

// allowed reports whether the project can get values from the vault, and
// returns the environment of the vault.
func allowed(cfg *config.Config, p config.Project, vault string) (config.Environment, bool) {
	v, ok := cfg.Vault(vault)
	if !ok || !p.Allows(vault) {
		return "", false
	}
	// The environment check is exact. A missing or unknown environment
	// never delivers (D47).
	return v.Environment, v.Environment.Valid()
}

// Strictest returns the strictest environment of the vaults of refs: prod
// before test before dev (D62). It returns the empty environment if the
// configuration does not list a vault or gives it no valid environment.
func Strictest(cfg *config.Config, refs []protocol.Reference) config.Environment {
	var out config.Environment
	for _, ref := range refs {
		v, ok := cfg.Vault(ref.Vault)
		if !ok || !v.Environment.Valid() {
			return ""
		}
		if Rank(v.Environment) > Rank(out) {
			out = v.Environment
		}
	}
	return out
}

// Rank orders the environments by strictness: dev 1, test 2, prod 3. Each
// other value is 0.
func Rank(e config.Environment) int {
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
