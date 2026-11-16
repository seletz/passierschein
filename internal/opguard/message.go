package opguard

// Message returns the text for the agent when the guard blocks a command
// (D67, point 2). A model reads it. It names the two ways of a38, the MCP
// tools "run_with_credentials" and "request_credentials" of the server
// "a38", and it matches the rules of the skill (internal/skill/SKILL.md).
// It names no other way to get a credential, so that the agent does not
// look for one (D35, RI8).
func Message(rule Rule) string {
	var what string
	switch rule {
	case RuleTokenFile, RuleKeychainToken:
		what = "read the 1Password token of a38."
	default:
		what = "run the 1Password CLI \"op\"."
	}
	return "Blocked by a38: do not " + what + "\n" +
		"To run a command with a credential, use the MCP tool\n" +
		"run_with_credentials of the server a38. For a dev credential\n" +
		"value, you can also use request_credentials.\n"
}
