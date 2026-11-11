package main

import (
	"fmt"
	"io"

	"github.com/seletz/passierschein/internal/skill"
)

// runLLM prints the instructions for agents. "--skill" is the only mode.
func runLLM(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || args[0] != "--skill" {
		_, _ = fmt.Fprintln(stderr, "a38 llm: usage: a38 llm --skill")
		return exitUsage
	}
	if _, err := io.WriteString(stdout, skill.Text()); err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 llm: %v\n", err)
		return exitFailure
	}
	return exitOK
}
