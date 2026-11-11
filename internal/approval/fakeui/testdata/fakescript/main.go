// Command fakescript is the helper program of the script mode tests. It
// reads one JSON message from its standard input and acts as the flags say.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	record := flag.String("record", "", "append the input to this file")
	tick := flag.String("tick", "none", "entries to approve: none, all, a list of indexes, or the list of literal IDs after \"id:\"")
	ttl := flag.String("ttl", "proposed", "lifetime: proposed, half, plus1, or a number")
	outcome := flag.String("outcome", "", "outcome to write; empty: approved if entries are ticked, else denied")
	raw := flag.String("raw", "", "write this text instead of a decision")
	hang := flag.Bool("hang", false, "never answer")
	exit := flag.Int("exit", 0, "exit code")
	flag.Parse()

	in, _ := io.ReadAll(os.Stdin)
	if *record != "" {
		f, err := os.OpenFile(*record, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = f.Write(append(append([]byte{}, in...), '\n'))
			_ = f.Close()
		}
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
	if msg.Type == "notice" {
		os.Exit(*exit)
	}
	if *hang {
		time.Sleep(time.Hour)
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
			i, err := strconv.Atoi(s)
			if err == nil && i >= 0 && i < len(msg.Credentials) {
				ids = append(ids, msg.Credentials[i].ID)
			}
		}
	}
	var life int64
	switch *ttl {
	case "proposed":
		life = msg.Grant.TTL
	case "half":
		life = msg.Grant.TTL / 2
	case "plus1":
		life = msg.Grant.TTL + 1
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
	d := map[string]any{
		"version": 1, "type": "approval_decision", "request_id": msg.RequestID,
		"outcome": out, "approved_credential_ids": ids, "elapsed_ms": 1,
	}
	if out == "approved" {
		d["ttl_seconds"] = life
	}
	_ = json.NewEncoder(os.Stdout).Encode(d)
	os.Exit(*exit)
}
