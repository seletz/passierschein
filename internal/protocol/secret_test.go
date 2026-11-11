package protocol

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

func TestSecretDoesNotPrint(t *testing.T) {
	// T8 (part): a secret value does not get into formatted text or a log.
	const marker = "kennwert-nur-fuer-tests"
	s := NewSecret(marker)
	var logged strings.Builder
	logger := slog.New(slog.NewTextHandler(&logged, nil))
	logger.Info("delivery", "value", s, "pointer", &s)
	outputs := []string{
		fmt.Sprint(s), fmt.Sprintf("%s", s), fmt.Sprintf("%v", s), fmt.Sprintf("%+v", s),
		fmt.Sprintf("%#v", s), fmt.Sprintf("%q", s), fmt.Sprintf("%x", s), fmt.Sprintf("%d", s),
		fmt.Sprintf("%v", &s), fmt.Sprintf("%+v", struct{ V Secret }{s}),
		fmt.Sprintf("%v", fmt.Errorf("read failed: %v", s)),
		logged.String(),
	}
	for i, out := range outputs {
		if strings.Contains(out, marker) {
			t.Errorf("output %d contains the secret value", i)
		}
	}
	if got := s.Reveal(); got != marker {
		t.Error("Reveal() does not return the value")
	}
}

func TestSecretJSON(t *testing.T) {
	const marker = "kennwert-nur-fuer-tests"
	data, err := json.Marshal(NewSecret(marker))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), marker) {
		t.Error("Marshal writes the value: only Encode writes it")
	}
	var back Secret
	if err := json.Unmarshal([]byte(`"`+marker+`"`), &back); err != nil {
		t.Fatal(err)
	}
	if back.Reveal() != marker {
		t.Error("Unmarshal does not read the value")
	}
	if !back.Equal(NewSecret(marker)) || back.Equal(NewSecret("anderer-wert")) || back.Equal(Secret{}) {
		t.Error("Equal does not compare the values")
	}
	if (Secret{}).Reveal() != "" {
		t.Error("the zero Secret is not empty")
	}
}
