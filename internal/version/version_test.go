package version

import "testing"

func TestString(t *testing.T) {
	tests := []struct {
		name                  string
		version, commit, date string
		want                  string
	}{
		{"default values", "dev", "unknown", "unknown", "a38 dev (commit unknown, built unknown)"},
		{"release values", "0.1.0", "abc1234", "2026-01-02T03:04:05Z", "a38 0.1.0 (commit abc1234, built 2026-01-02T03:04:05Z)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Version, Commit, Date = tt.version, tt.commit, tt.date
			if got := String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
