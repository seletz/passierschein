// Package skill holds the instructions for agents. The text is part of the
// binary, so it always matches the installed version of a38.
package skill

import _ "embed"

//go:embed SKILL.md
var text string

// Text returns the instructions for agents: a Markdown file with the front
// matter of a Claude Code skill.
func Text() string {
	return text
}
