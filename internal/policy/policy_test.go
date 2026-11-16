package policy

import (
	"testing"

	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/protocol"
)

const cfgYAML = `
version: 1
backend:
  onepassword:
    accounts:
      - name: privat
vaults:
  - {name: beispiel.app.k8s-dev, environment: dev, account: privat}
  - {name: beispiel.app.k8s-test, environment: test, account: privat}
  - {name: beispiel.app.k8s-prod, environment: prod, account: privat}
  - {name: beispiel.infra.k8s-dev, environment: dev, account: privat}
projects:
  - name: beispielprojekt
    paths: [/tmp/a38-test/beispielprojekt]
    vaults: [beispiel.app.k8s-dev, beispiel.app.k8s-test, beispiel.app.k8s-prod]
  - name: kaputtprojekt
    paths: [/tmp/a38-test/kaputtprojekt]
    vaults: [beispiel.app.k8s-dev, fehlt.k8s-dev]
`

// T3, T22, D12, D24, D28, D47: the rules of the policy, all or nothing.
func TestCheck(t *testing.T) {
	cfg, err := config.Parse([]byte(cfgYAML))
	if err != nil {
		t.Fatal(err)
	}
	ref := func(vault string) protocol.Reference {
		return protocol.Reference{Vault: vault, Item: "datenbank-dev", Field: "password"}
	}
	tests := []struct {
		name    string
		project string
		vaults  []string
		want    protocol.Code
	}{
		{"allowed dev vault", "beispielprojekt", []string{"beispiel.app.k8s-dev"}, ""},
		{"vault not in the configuration", "beispielprojekt", []string{"unbekannt.k8s-dev"}, protocol.CodeNotAllowed},
		{"vault not in the allowlist", "beispielprojekt", []string{"beispiel.infra.k8s-dev"}, protocol.CodeNotAllowed},
		{"environment test", "beispielprojekt", []string{"beispiel.app.k8s-test"}, protocol.CodeNotAllowed},
		{"environment prod", "beispielprojekt", []string{"beispiel.app.k8s-prod"}, protocol.CodeNotAllowed},
		{"exact name only", "beispielprojekt", []string{"Beispiel.app.k8s-dev"}, protocol.CodeNotAllowed},
		{"all or nothing", "beispielprojekt", []string{"beispiel.app.k8s-dev", "beispiel.app.k8s-prod"}, protocol.CodeNotAllowed},
		{"no project", "", []string{"beispiel.app.k8s-dev"}, protocol.CodeNotAllowed},
		{"unknown project", "fremd", []string{"beispiel.app.k8s-dev"}, protocol.CodeNotAllowed},
		{"invalid project", "kaputtprojekt", []string{"beispiel.app.k8s-dev"}, protocol.CodeConfigInvalid},
		{"no references", "beispielprojekt", nil, protocol.CodeNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var refs []protocol.Reference
			for _, v := range tt.vaults {
				refs = append(refs, ref(v))
			}
			got := Check(cfg, tt.project, refs)
			if (got == nil) != (tt.want == "") || got != nil && got.Code != tt.want {
				t.Fatalf("Check = %v, want %q", got, tt.want)
			}
		})
	}
	if Check(nil, "beispielprojekt", []protocol.Reference{ref("beispiel.app.k8s-dev")}) == nil {
		t.Error("a nil configuration allows a request")
	}
}
