package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/protocol"
)

// fakeBackendExec adds a vault of test and a vault of prod to the fake
// backend of the tests. The values are fake.
const fakeBackendExec = `  - name: beispiel.app.k8s-test
    items:
      - name: datenbank-test
        fields:
          - name: password
            value: beispiel-wert-test
  - name: beispiel.app.k8s-prod
    items:
      - name: datenbank-prod
        fields:
          - name: password
            value: beispiel-wert-prod
`

// writeExecConfig writes the configuration of writeConfig with a vault of
// test and a vault of prod. The exec way for prod is switched on.
func writeExecConfig(t *testing.T) (path, socketDir, fakeFile string) {
	t.Helper()
	path, socketDir, fakeFile = writeConfig(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const devVault = "  - {name: beispiel.app.k8s-dev, environment: dev, account: privat}\n"
	if !strings.Contains(string(data), devVault) || !strings.Contains(string(data), "vaults: [beispiel.app.k8s-dev]") {
		t.Fatalf("the configuration of writeConfig changed:\n%s", data)
	}
	text := strings.Replace(string(data), devVault, devVault+
		"  - {name: beispiel.app.k8s-test, environment: test, account: privat}\n"+
		"  - {name: beispiel.app.k8s-prod, environment: prod, account: privat}\n", 1)
	text = strings.Replace(text, "vaults: [beispiel.app.k8s-dev]",
		"vaults: [beispiel.app.k8s-dev, beispiel.app.k8s-test, beispiel.app.k8s-prod]", 1)
	text += "environments:\n  prod:\n    delivery: [exec]\n"
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fakeFile, []byte(fakeBackend+fakeBackendExec), 0o600); err != nil {
		t.Fatal(err)
	}
	return path, socketDir, fakeFile
}

func TestMCPRunWithCredentialsEndToEnd(t *testing.T) {
	// T35, T36, T38, T39 (end to end): the real processes "a38 serve
	// --backend fake --ui auto-approve" and "a38 mcp". An MCP client runs
	// a command with run_with_credentials for dev, test and prod (prod
	// switched on). The command checks the value in its environment and
	// prints it; the result shows only the mask. request_credentials for
	// test ends with exec_required. The audit log names the command, holds
	// the event execution and never a value or the output.
	if testing.Short() {
		t.Skip("builds the binary")
	}
	bin := filepath.Join(t.TempDir(), "a38")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	cfgPath, socketDir, fakeFile := writeExecConfig(t)
	stop := startServe(t, bin, cfgPath, socketDir, "--backend", "fake", "--fake-file", fakeFile, "--ui", "auto-approve")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := a38Cmd(ctx, t, bin, "mcp", "--config", cfgPath, "--session-name", "beispiel-koordinator")
	var mcpErr syncBuffer
	cmd.Stderr = &mcpErr
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "coding-agent", Version: "1"}, nil).
		Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect: %v; stderr:\n%s", err, mcpErr.String())
	}

	// T35 step 1: no value of test as MCP answer.
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "request_credentials",
		Arguments: map[string]any{
			"references": []string{"op://beispiel.app.k8s-test/datenbank-test/password"},
			"task":       "Check the migration", "reason": "The check needs the database of test.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sc, _ := json.Marshal(res.StructuredContent); !res.IsError || !strings.Contains(string(sc), string(protocol.CodeExecRequired)) || strings.Contains(string(sc), "beispiel-wert") {
		t.Errorf("request_credentials for test = %s; want exec_required and no value", sc)
	}

	run := func(env, ref, value string, args []string) (map[string]any, bool) {
		t.Helper()
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{
			Name: "run_with_credentials",
			Arguments: map[string]any{
				"command": "/bin/sh",
				"args":    args,
				"env":     map[string]string{"DB_PASSWORD": ref},
				"task":    "Check the database of " + env,
				"reason":  "Verify migration 42 in " + env,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		sc, _ := json.Marshal(res.StructuredContent)
		if strings.Contains(string(sc), value) {
			t.Errorf("%s: the result contains the value", env)
		}
		var got map[string]any
		if err := json.Unmarshal(sc, &got); err != nil {
			t.Fatal(err)
		}
		return got, res.IsError
	}
	for _, tt := range []struct {
		env, ref, value string
	}{
		{"dev", "op://beispiel.app.k8s-dev/datenbank-dev/password", "beispiel-wert-serve"},
		{"test", "op://beispiel.app.k8s-test/datenbank-test/password", "beispiel-wert-test"},
		{"prod", "op://beispiel.app.k8s-prod/datenbank-prod/password", "beispiel-wert-prod"},
	} {
		script := `test "$DB_PASSWORD" = "` + tt.value + `" || exit 7; echo "value: $DB_PASSWORD"`
		got, isErr := run(tt.env, tt.ref, tt.value, []string{"-c", script})
		// The script contains the value: the daemon refuses (T39).
		if !isErr || !strings.Contains(mustJSONText(t, got), string(protocol.CodeInvalidRequest)) {
			t.Errorf("%s: a command with the value = %v; want invalid_request", tt.env, got)
		}
		// The output text appears only at run time, never in the arguments.
		script = `test -n "$DB_PASSWORD" || exit 7; printf 'value: %s\n' "$DB_PASSWORD"; printf 'beispiel-ausgabe-%s\n' 4711`
		got, isErr = run(tt.env, tt.ref, tt.value, []string{"-c", script})
		stdout, _ := got["stdout"].(string)
		if isErr || got["exit_code"] != float64(0) || !strings.Contains(stdout, "value: [masked:DB_PASSWORD]") ||
			!strings.Contains(stdout, outputText) || got["masked"] != float64(1) {
			t.Errorf("%s: result = %v; want exit code 0 and the mask", tt.env, got)
		}
	}
	_ = cs.Close()
	stop()

	entries := auditEntries(t, bin, cfgPath)
	for _, e := range entries {
		text := mustJSONText(t, e)
		if strings.Contains(text, outputText) {
			t.Errorf("the event %s contains the output", e.Type)
		}
		for _, value := range []string{"beispiel-wert-serve", "beispiel-wert-test", "beispiel-wert-prod"} {
			if !strings.Contains(text, value) {
				continue
			}
			// RI14: the agent put the value into an argument. Only the
			// entries before the read of the values repeat the command
			// of the agent.
			agent := e.Command != nil && strings.Contains(strings.Join(e.Command.Args, " "), value)
			before := e.Type == audit.EventRequest || e.Type == audit.EventDecision || e.Type == audit.EventGrant
			if !agent || !before {
				t.Errorf("the event %s contains a value", e.Type)
			}
		}
	}
	var executions, deliveries int
	for _, e := range entries {
		switch e.Type {
		case audit.EventDelivery:
			deliveries++
			if e.Command == nil || e.Command.Way != audit.WayExec || e.Command.Program != "/bin/sh" {
				t.Errorf("delivery without the command: %+v", e)
			}
		case audit.EventExecution:
			executions++
			if x := e.Execution; x == nil || x.ExitCode != 0 || x.Masked != 1 || e.Command == nil {
				t.Errorf("execution = %+v", e)
			}
		}
	}
	if deliveries != 3 || executions != 3 {
		t.Errorf("deliveries %d, executions %d; want 3 and 3", deliveries, executions)
	}
	if out, err := a38Cmd(context.Background(), t, bin, "audit", "verify", "--config", cfgPath).CombinedOutput(); err != nil {
		t.Errorf("a38 audit verify: %v\n%s", err, out)
	}
}

// outputText is a text that only the output of the command contains.
const outputText = "beispiel-ausgabe-4711"

// mustJSONText returns v as JSON text.
func mustJSONText(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
