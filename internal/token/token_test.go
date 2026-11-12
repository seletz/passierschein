package token

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

// marker is a fake token. No output may contain it.
const marker = "ops_beispiel-marker-7f3a"

func TestTokenNeverPrints(t *testing.T) {
	// D34, G3: the token prints as [redacted] in each form.
	tok := New(marker)
	var logged strings.Builder
	slog.New(slog.NewTextHandler(&logged, nil)).Info("start", "token", tok)
	js, err := json.Marshal(struct{ T Token }{tok})
	if err != nil {
		t.Fatal(err)
	}
	txt, err := tok.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	outs := []string{
		fmt.Sprint(tok), fmt.Sprintf("%v %+v %#v %s %q %x", tok, tok, tok, tok, tok, tok),
		fmt.Sprintf("%v", struct{ T Token }{tok}), fmt.Sprintf("%+v", &struct{ t Token }{tok}),
		logged.String(), string(js), string(txt), tok.String(), tok.GoString(),
	}
	for _, out := range outs {
		if strings.Contains(out, marker) || strings.Contains(out, "beispiel-marker") {
			t.Errorf("output contains the token: %q", out)
		}
	}
	if !strings.Contains(outs[0], "[redacted]") {
		t.Errorf("fmt output = %q, want [redacted]", outs[0])
	}
}
