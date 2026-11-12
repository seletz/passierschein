// Command gen writes a synthetic audit log for demos and screenshots of
// "a38 tui". It is a development program and not part of the release.
//
//	go run ./internal/auditdemo/gen -out /tmp/a38-tui-demo/audit.db
//
// It never writes into an existing file, so it cannot change a real audit
// log. With -break N it changes entry N afterwards, so that the hash chain
// is broken. With -empty it writes an audit log without entries.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	_ "modernc.org/sqlite" // the pure Go driver, no cgo

	"github.com/seletz/passierschein/internal/audit/sqlite"
	"github.com/seletz/passierschein/internal/auditdemo"
	"github.com/seletz/passierschein/internal/clock"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "gen: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("gen", flag.ContinueOnError)
	out := fs.String("out", "", "new audit file (must not exist)")
	days := fs.Int("days", 30, "number of calendar days, with the day of -end")
	endText := fs.String("end", "", "time of the newest possible event, RFC 3339 (default: now)")
	brk := fs.Uint64("break", 0, "change this entry afterwards, so that the chain is broken")
	empty := fs.Bool("empty", false, "write an audit log without entries")
	seed := fs.Uint64("seed", 0, "seed of the random events")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return errors.New("-out is necessary")
	}
	if _, err := os.Lstat(*out); err == nil {
		return fmt.Errorf("%s exists; gen writes only a new file", *out)
	}
	end := time.Now()
	if *endText != "" {
		var err error
		if end, err = time.Parse(time.RFC3339, *endText); err != nil {
			return fmt.Errorf("-end: %w", err)
		}
	}
	clk := clock.NewFake(end.AddDate(0, 0, -*days-1))
	store, err := sqlite.Open(*out, sqlite.WithClock(clk))
	if err != nil {
		return err
	}
	if !*empty {
		err = auditdemo.Generate(context.Background(), store, clk, auditdemo.Options{End: end, Days: *days, Location: time.Local, Seed: *seed})
	}
	if cerr := store.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if *brk > 0 {
		db, err := sql.Open("sqlite", "file:"+*out)
		if err != nil {
			return err
		}
		defer func() { _ = db.Close() }()
		if _, err := db.Exec(`UPDATE entries SET detail = 'changed afterwards' WHERE seq = ?`, *brk); err != nil {
			return err
		}
	}
	fmt.Println(*out)
	return nil
}
