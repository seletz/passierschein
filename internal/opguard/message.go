package opguard

// Message returns the text for the agent when the guard blocks a command.
// A model reads it. It names one way only, the MCP tool
// "request_credentials" of the server "a38", and it matches the rules of
// the skill (internal/skill/SKILL.md). It names no other way to get a
// credential, so that the agent does not look for one (D35, RI8).
func Message(rule Rule) string {
	var what string
	switch rule {
	case RuleOpCLI:
		what = "This command runs the 1Password CLI \"op\"."
	case RuleTokenFile, RuleKeychainToken:
		what = "This command reads the 1Password token of a38."
	default:
		what = "This command reads a credential directly."
	}
	return "a38 blocked this command. " + what + " The command did not run.\n" +
		"Do not run \"op\" or any other password manager tool yourself.\n" +
		"Get credentials only through a38: call the MCP tool \"request_credentials\" of the MCP server \"a38\".\n" +
		"Put all credentials that the task needs into one request. Give each as a reference of the form op://vault/item/field, and give \"task\" and \"reason\".\n" +
		"If the request fails or the human denies it, stop the work that needs the credential and tell the human the error code.\n" +
		"Do not look for another way to get the credential.\n"
}
