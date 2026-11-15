package policy

import (
	"testing"

	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/protocol"
)

const cfgYAML = `
version: 1
vaults:
  - {name: beispiel.app.k8s-dev, environment: dev}
  - {name: beispiel.app.k8s-test, environment: test}
  - {name: beispiel.app.k8s-prod, environment: prod}
  - {name: beispiel.infra.k8s-dev, environment: dev}
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
		// T35: outside dev the MCP answer is never allowed (D62).
		{"environment test", "beispielprojekt", []string{"beispiel.app.k8s-test"}, protocol.CodeExecRequired},
		{"environment prod", "beispielprojekt", []string{"beispiel.app.k8s-prod"}, protocol.CodeNotAllowed},
		{"exact name only", "beispielprojekt", []string{"Beispiel.app.k8s-dev"}, protocol.CodeNotAllowed},
		{"all or nothing", "beispielprojekt", []string{"beispiel.app.k8s-dev", "beispiel.app.k8s-prod"}, protocol.CodeNotAllowed},
		{"mixed dev and test", "beispielprojekt", []string{"beispiel.app.k8s-dev", "beispiel.app.k8s-test"}, protocol.CodeExecRequired},
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

// T35, D62, D66: the delivery way follows the environment of the vaults.
// The strictest environment of a mixed request decides; a way that the
// configuration switches off gives not_allowed.
func TestCheckWay(t *testing.T) {
	ref := func(vault string) protocol.Reference {
		return protocol.Reference{Vault: vault, Item: "datenbank-dev", Field: "password"}
	}
	const prodOn = `
environments:
  prod:
    delivery: [exec]
`
	const devExecOnly = `
environments:
  dev:
    delivery: [exec]
  test:
    delivery: []
`
	tests := []struct {
		name   string
		extra  string
		way    config.Delivery
		vaults []string
		want   protocol.Code
	}{
		{"dev mcp", "", config.DeliveryMCP, []string{"beispiel.app.k8s-dev"}, ""},
		{"dev exec", "", config.DeliveryExec, []string{"beispiel.app.k8s-dev"}, ""},
		{"test mcp", "", config.DeliveryMCP, []string{"beispiel.app.k8s-test"}, protocol.CodeExecRequired},
		{"test exec", "", config.DeliveryExec, []string{"beispiel.app.k8s-test"}, ""},
		{"prod mcp, off", "", config.DeliveryMCP, []string{"beispiel.app.k8s-prod"}, protocol.CodeNotAllowed},
		{"prod exec, off", "", config.DeliveryExec, []string{"beispiel.app.k8s-prod"}, protocol.CodeNotAllowed},
		{"prod exec, on", prodOn, config.DeliveryExec, []string{"beispiel.app.k8s-prod"}, ""},
		{"prod mcp, on", prodOn, config.DeliveryMCP, []string{"beispiel.app.k8s-prod"}, protocol.CodeExecRequired},
		{"mixed dev and test mcp", "", config.DeliveryMCP, []string{"beispiel.app.k8s-dev", "beispiel.app.k8s-test"}, protocol.CodeExecRequired},
		{"mixed dev and test exec", "", config.DeliveryExec, []string{"beispiel.app.k8s-dev", "beispiel.app.k8s-test"}, ""},
		{"mixed test and prod exec, prod off", "", config.DeliveryExec, []string{"beispiel.app.k8s-test", "beispiel.app.k8s-prod"}, protocol.CodeNotAllowed},
		{"dev mcp switched off", devExecOnly, config.DeliveryMCP, []string{"beispiel.app.k8s-dev"}, protocol.CodeExecRequired},
		{"test switched off", devExecOnly, config.DeliveryMCP, []string{"beispiel.app.k8s-test"}, protocol.CodeNotAllowed},
		{"not allowed for the project", "", config.DeliveryExec, []string{"beispiel.infra.k8s-dev"}, protocol.CodeNotAllowed},
		{"unknown way", "", config.Delivery("file"), []string{"beispiel.app.k8s-dev"}, protocol.CodeNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.Parse([]byte(cfgYAML + tt.extra))
			if err != nil {
				t.Fatal(err)
			}
			var refs []protocol.Reference
			for _, v := range tt.vaults {
				refs = append(refs, ref(v))
			}
			got := CheckWay(cfg, "beispielprojekt", refs, tt.way)
			if (got == nil) != (tt.want == "") || got != nil && got.Code != tt.want {
				t.Fatalf("CheckWay = %v, want %q", got, tt.want)
			}
		})
	}
}

// T35, D62: Strictest returns the strictest environment of the references.
func TestStrictest(t *testing.T) {
	cfg, err := config.Parse([]byte(cfgYAML))
	if err != nil {
		t.Fatal(err)
	}
	ref := func(vault string) protocol.Reference {
		return protocol.Reference{Vault: vault, Item: "datenbank-dev", Field: "password"}
	}
	tests := []struct {
		vaults []string
		want   config.Environment
	}{
		{[]string{"beispiel.app.k8s-dev"}, config.EnvDev},
		{[]string{"beispiel.app.k8s-dev", "beispiel.app.k8s-test"}, config.EnvTest},
		{[]string{"beispiel.app.k8s-prod", "beispiel.app.k8s-test", "beispiel.app.k8s-dev"}, config.EnvProd},
		{[]string{"unbekannt.k8s-dev"}, ""},
	}
	for _, tt := range tests {
		var refs []protocol.Reference
		for _, v := range tt.vaults {
			refs = append(refs, ref(v))
		}
		if got := Strictest(cfg, refs); got != tt.want {
			t.Errorf("Strictest(%v) = %q, want %q", tt.vaults, got, tt.want)
		}
	}
}
