package skill

import (
	"strings"
	"testing"

	"github.com/seletz/passierschein/internal/protocol"
)

func TestFrontMatter(t *testing.T) {
	for _, want := range []string{"---\nname: a38\n", "\ndescription: "} {
		if !strings.Contains(Text(), want) {
			t.Errorf("the front matter lacks %q", want)
		}
	}
	if !strings.HasPrefix(Text(), "---\n") {
		t.Error("the text does not start with a front matter")
	}
}

// T20 (part): the text names every error code of the protocol and every
// MCP tool.
func TestTextNamesCodesAndTools(t *testing.T) {
	for _, c := range protocol.Codes() {
		if !strings.Contains(Text(), "`"+string(c)+"`") {
			t.Errorf("the skill does not name the error code %q", c)
		}
	}
	for _, tool := range []protocol.Type{protocol.TypeRequestCredentials, protocol.TypeGrantStatus} {
		if !strings.Contains(Text(), "`"+string(tool)+"`") {
			t.Errorf("the skill does not name the tool %q", tool)
		}
	}
	for _, want := range []string{"op://", "references", "task", "reason", "`op`"} {
		if !strings.Contains(Text(), want) {
			t.Errorf("the skill lacks %q", want)
		}
	}
}
