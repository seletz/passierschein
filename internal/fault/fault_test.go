package fault

import (
	"context"
	"testing"

	"github.com/seletz/passierschein/internal/protocol"
)

// NoLock never locks delivery.
func TestNoLock(t *testing.T) {
	var l Lock = NoLock{}
	l.Raise(protocol.CodeAuditFailed, "test")
	if l.Check() != nil || l.Fault() != nil {
		t.Fatal("NoLock locked")
	}
	if ok, finding := l.Clear(context.Background()); !ok || finding != "" {
		t.Fatalf("Clear = %v, %q", ok, finding)
	}
}
