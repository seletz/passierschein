package bundle

import (
	"slices"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/protocol"
)

func r(field string) protocol.Reference {
	return protocol.Reference{Vault: "beispiel.app.k8s-dev", Item: "datenbank-dev", Field: field}
}

// T2: the window has a fixed end.
func TestAccepts(t *testing.T) {
	start := time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)
	b := New(start, 3*time.Second, 10)
	tests := []struct {
		at   time.Duration
		want bool
	}{
		{0, true},
		{2999 * time.Millisecond, true},
		{3 * time.Second, false},
		{4 * time.Second, false},
	}
	for _, tt := range tests {
		if got := b.Accepts(start.Add(tt.at)); got != tt.want {
			t.Errorf("Accepts(+%v) = %v, want %v", tt.at, got, tt.want)
		}
	}
	b.Add([]protocol.Reference{r("a")})
	if !b.Deadline().Equal(start.Add(3 * time.Second)) {
		t.Errorf("the deadline moved")
	}
	b.Close()
	if b.Accepts(start) {
		t.Error("a closed bundle accepts")
	}
	if got := b.Add([]protocol.Reference{r("b")}); got[0] {
		t.Error("a closed bundle adds")
	}
}

// T19: the maximum keeps the first entries; a repeated entry uses no
// place.
func TestAddMaximum(t *testing.T) {
	tests := []struct {
		name   string
		max    int
		calls  [][]string
		want   [][]bool
		stored []string
	}{
		{"below", 3, [][]string{{"a", "b"}}, [][]bool{{true, true}}, []string{"a", "b"}},
		{"exactly", 2, [][]string{{"a", "b"}}, [][]bool{{true, true}}, []string{"a", "b"}},
		{"above", 2, [][]string{{"a", "b", "c"}}, [][]bool{{true, true, false}}, []string{"a", "b"}},
		{"over calls", 2, [][]string{{"a"}, {"b", "c"}, {"d"}}, [][]bool{{true}, {true, false}, {false}}, []string{"a", "b"}},
		{"repeat uses no place", 2, [][]string{{"a", "a"}, {"b", "a"}}, [][]bool{{true, true}, {true, true}}, []string{"a", "b"}},
		{"zero means one", 0, [][]string{{"a", "b"}}, [][]bool{{true, false}}, []string{"a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(time.Now(), time.Second, tt.max)
			for i, call := range tt.calls {
				var refs []protocol.Reference
				for _, f := range call {
					refs = append(refs, r(f))
				}
				if got := b.Add(refs); !slices.Equal(got, tt.want[i]) {
					t.Errorf("call %d = %v, want %v", i, got, tt.want[i])
				}
			}
			var stored []string
			for _, ref := range b.Close() {
				stored = append(stored, ref.Field)
			}
			if !slices.Equal(stored, tt.stored) || b.Len() != len(tt.stored) {
				t.Errorf("entries = %v, want %v", stored, tt.stored)
			}
		})
	}
}
