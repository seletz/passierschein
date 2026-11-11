// Command fakeapp is the fake approval app of the tests. It never opens a
// window and never asks for Touch ID. It reads one JSON message from its
// standard input and acts as the flags say. The tests start it through a
// small shell script that holds the flags, because the daemon passes no
// argument to the app.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	record := flag.String("record", "", "append the input to this file")
	tick := flag.String("tick", "none", "entries to approve: none, all, a list of indexes, or literal IDs after \"id:\"")
	auth := flag.String("auth", "ok", "authentication: ok, none, failed, skipped, unknown")
	ttl := flag.String("ttl", "proposed", "lifetime: proposed, plus1, 0, or a number")
	outcome := flag.String("outcome", "", "outcome to write; empty: approved if entries are ticked, else denied")
	rid := flag.String("rid", "", "request ID to write instead of the right one")
	raw := flag.String("raw", "", "write this text instead of a decision")
	big := flag.Int("big", 0, "write this many bytes of output")
	twice := flag.Bool("twice", false, "write the decision twice")
	hang := flag.Bool("hang", false, "never answer")
	pidfile := flag.String("pidfile", "", "write the own PID to this file")
	spawn := flag.String("spawn", "", "start a child that hangs in the same group and write its PID to this file")
	sig := flag.Bool("signal", false, "kill the own process with SIGKILL")
	exit := flag.Int("exit", 0, "exit code")
	child := flag.Bool("child", false, "internal: hang as the child")
	flag.Parse()

	if *child {
		time.Sleep(time.Hour)
		return
	}
	in, _ := io.ReadAll(os.Stdin)
	if *record != "" {
		if f, err := os.OpenFile(*record, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
			_, _ = f.Write(append(append([]byte{}, in...), '\n'))
			_ = f.Close()
		}
	}
	if *pidfile != "" {
		_ = os.WriteFile(*pidfile, []byte(strconv.Itoa(os.Getpid())), 0o600)
	}
	if *spawn != "" {
		c := exec.Command(os.Args[0], "-child")
		_ = c.Start()
		_ = os.WriteFile(*spawn, []byte(strconv.Itoa(c.Process.Pid)), 0o600)
	}
	var msg struct {
		Type        string `json:"type"`
		RequestID   string `json:"request_id"`
		Credentials []struct {
			ID string `json:"id"`
		} `json:"credentials"`
		Grant struct {
			TTL int64 `json:"ttl_seconds"`
		} `json:"grant"`
	}
	_ = json.Unmarshal(in, &msg)
	if msg.Type == "notice" && !*hang {
		os.Exit(*exit)
	}
	if *sig {
		_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
	}
	if *hang {
		time.Sleep(time.Hour)
	}
	if *big > 0 {
		_, _ = os.Stdout.Write([]byte(strings.Repeat("x", *big)))
		os.Exit(*exit)
	}
	if *raw != "" {
		fmt.Print(*raw)
		os.Exit(*exit)
	}

	ids := []string{}
	switch {
	case *tick == "none":
	case *tick == "all":
		for _, c := range msg.Credentials {
			ids = append(ids, c.ID)
		}
	case strings.HasPrefix(*tick, "id:"):
		ids = strings.Split(strings.TrimPrefix(*tick, "id:"), ",")
	default:
		for _, s := range strings.Split(*tick, ",") {
			if i, err := strconv.Atoi(s); err == nil && i >= 0 && i < len(msg.Credentials) {
				ids = append(ids, msg.Credentials[i].ID)
			}
		}
	}
	life := msg.Grant.TTL
	switch *ttl {
	case "proposed":
	case "plus1":
		life++
	default:
		life, _ = strconv.ParseInt(*ttl, 10, 64)
	}
	out := *outcome
	if out == "" {
		out = "denied"
		if len(ids) > 0 {
			out = "approved"
		}
	}
	id := msg.RequestID
	if *rid != "" {
		id = *rid
	}
	d := map[string]any{
		"version": 1, "type": "approval_decision", "request_id": id,
		"outcome": out, "approved_credential_ids": ids, "elapsed_ms": 1,
	}
	if out == "approved" {
		d["ttl_seconds"] = life
	}
	switch *auth {
	case "ok":
		d["auth"] = map[string]any{"policy": "biometrics", "ok": true}
	case "failed":
		d["auth"] = map[string]any{"policy": "biometrics", "ok": false}
	case "skipped":
		d["auth"] = map[string]any{"policy": "skipped", "ok": true}
	case "unknown":
		d["auth"] = map[string]any{"policy": "magic", "ok": true}
	}
	enc := json.NewEncoder(os.Stdout)
	_ = enc.Encode(d)
	if *twice {
		_ = enc.Encode(d)
	}
	os.Exit(*exit)
}
