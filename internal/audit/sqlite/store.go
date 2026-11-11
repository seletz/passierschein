// Package sqlite implements [audit.Store] on a SQLite file with a SHA-256
// hash chain.
//
// Each entry has a running number (1, 2, 3, ...) and a hash over the hash
// of the entry before it and the canonical form of the entry (see
// canonical). The first entry uses 64 zeros as the previous hash.
//
// Append writes in one transaction with full synchronisation, so the entry
// is on disk when Append returns. Verify reads all entries in the sequence
// of their numbers and finds the first entry that does not fit.
//
// Known limit (decision D37): the chain makes a change visible,
// but it cannot show that entries at the end are removed, and it cannot
// show a chain that somebody calculated again completely. The audit log is
// an aid for analysis, not a proof.
//
// The store never holds a secret value. [audit.Event] has no field for it.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite" // the pure Go driver, no cgo

	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/clock"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

const schema = `
CREATE TABLE IF NOT EXISTS entries (
	seq          INTEGER PRIMARY KEY,
	time_ns      INTEGER NOT NULL,
	type         TEXT    NOT NULL,
	session_id   TEXT    NOT NULL,
	origin       TEXT    NOT NULL,
	project      TEXT    NOT NULL,
	peer_pid     INTEGER NOT NULL,
	peer_uid     INTEGER NOT NULL,
	process      TEXT    NOT NULL,
	cwd          TEXT    NOT NULL,
	session_name TEXT    NOT NULL,
	agent        TEXT    NOT NULL,
	task         TEXT    NOT NULL,
	reason       TEXT    NOT NULL,
	refs         TEXT    NOT NULL,
	code         TEXT    NOT NULL,
	request_id   TEXT    NOT NULL,
	grant_id     TEXT    NOT NULL,
	lifetime_s   INTEGER NOT NULL,
	detail       TEXT    NOT NULL,
	prev_hash    TEXT    NOT NULL,
	hash         TEXT    NOT NULL
);`

const columns = `seq, time_ns, type, session_id, origin, project, peer_pid, peer_uid,
	process, cwd, session_name, agent, task, reason, refs, code, request_id,
	grant_id, lifetime_s, detail, prev_hash, hash`

// Option configures a [Store].
type Option func(*Store)

// WithClock sets the clock that gives the time of an entry. The default is
// the system clock.
func WithClock(c clock.Clock) Option {
	return func(s *Store) { s.clock = c }
}

// Store is the audit log on a SQLite file.
type Store struct {
	db       *sql.DB
	clock    clock.Clock
	readOnly bool

	// mu serialises Append, so that the numbers and the hashes follow
	// each other without a gap.
	mu       sync.Mutex
	lastSeq  uint64
	lastHash string
}

var _ audit.Store = (*Store)(nil)

// Open opens the audit file for reading and writing. It creates the file
// (mode 0600) and its directory (mode 0700) if they do not exist.
func Open(path string, opts ...Option) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("audit: cannot create the directory of the audit file: %w", err)
	}
	// Create the file with the right mode before SQLite does.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("audit: cannot create the audit file: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("audit: cannot create the audit file: %w", err)
	}
	s, err := open(path, false, opts)
	if err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(schema); err != nil {
		_ = s.db.Close()
		return nil, fmt.Errorf("audit: cannot prepare the audit file: %w", err)
	}
	if err := s.loadLast(context.Background()); err != nil {
		_ = s.db.Close()
		return nil, err
	}
	return s, nil
}

// OpenReadOnly opens an existing audit file for reading. Append fails on
// such a store. A missing file is an error.
func OpenReadOnly(path string, opts ...Option) (*Store, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("audit: cannot read the audit file: %w", err)
	}
	s, err := open(path, true, opts)
	if err != nil {
		return nil, err
	}
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name = 'entries'`).Scan(&n); err != nil || n == 0 {
		_ = s.db.Close()
		return nil, errors.New("audit: the file is not an audit log")
	}
	return s, nil
}

func open(path string, readOnly bool, opts []Option) (*Store, error) {
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "synchronous(FULL)")
	if readOnly {
		q.Set("mode", "ro")
	}
	dsn := (&url.URL{Scheme: "file", Opaque: escapePath(path), RawQuery: q.Encode()}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("audit: cannot open the audit file: %w", err)
	}
	// One connection: Append is serialised anyway, and it keeps the
	// pragmas of the connection in force.
	db.SetMaxOpenConns(1)
	s := &Store{db: db, clock: clock.System{}, readOnly: readOnly, lastHash: genesisHash}
	for _, o := range opts {
		o(s)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("audit: cannot open the audit file: %w", err)
	}
	return s, nil
}

// escapePath escapes the characters of a path that have a meaning in a
// file URI.
func escapePath(p string) string {
	return strings.NewReplacer("%", "%25", "?", "%3f", "#", "%23").Replace(p)
}

func (s *Store) loadLast(ctx context.Context) error {
	err := s.db.QueryRowContext(ctx, `SELECT seq, hash FROM entries ORDER BY seq DESC LIMIT 1`).Scan(&s.lastSeq, &s.lastHash)
	if errors.Is(err, sql.ErrNoRows) {
		s.lastSeq, s.lastHash = 0, genesisHash
		return nil
	}
	if err != nil {
		return fmt.Errorf("audit: cannot read the last entry: %w", err)
	}
	return nil
}

// Close closes the audit file.
func (s *Store) Close() error { return s.db.Close() }

// Append writes ev as the next entry. It is durable when Append returns.
// An invalid event is an error and writes nothing.
func (s *Store) Append(ctx context.Context, ev audit.Event) (audit.Entry, error) {
	if s.readOnly {
		return audit.Entry{}, errors.New("audit: the audit file is open for reading only")
	}
	if !ev.Type.Valid() {
		return audit.Entry{}, fmt.Errorf("audit: unknown event type %q", ev.Type)
	}
	for _, r := range ev.References {
		if err := r.Validate(); err != nil {
			return audit.Entry{}, fmt.Errorf("audit: invalid reference in event: %w", err)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// The time has the resolution of a nanosecond and is UTC. Strip the
	// monotonic reading, so that the stored and the returned entry match.
	e := audit.Entry{
		Seq:      s.lastSeq + 1,
		Time:     time.Unix(0, s.clock.Now().UnixNano()).UTC(),
		Event:    cloneEvent(ev),
		PrevHash: s.lastHash,
	}
	e.Hash = entryHash(e)

	refs, err := encodeRefs(e.References)
	if err != nil {
		return audit.Entry{}, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO entries (`+columns+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		e.Seq, e.Time.UnixNano(), string(e.Type), string(e.Verified.SessionID), string(e.Verified.Origin),
		e.Verified.Project, e.Verified.PeerPID, e.Verified.PeerUID, e.Verified.Process, e.Verified.Cwd,
		e.Claims.SessionName, e.Claims.Agent, e.Claims.Task, e.Claims.Reason, refs,
		string(e.Result.Code), e.Result.RequestID, e.Result.GrantID, e.Result.LifetimeSeconds,
		e.Result.Detail, e.PrevHash, e.Hash)
	if err != nil {
		return audit.Entry{}, fmt.Errorf("audit: cannot write the entry: %w", err)
	}
	s.lastSeq, s.lastHash = e.Seq, e.Hash
	return e, nil
}

// Probe checks that the store can write an entry, without a change to the
// file or the chain. It writes a row in a transaction and rolls it back.
// The daemon uses it at the start and for the check of the cause of the
// global delivery lock (D48). A full disk can pass the probe and still
// fail the next Append.
func (s *Store) Probe(ctx context.Context) (err error) {
	if s.readOnly {
		return errors.New("audit: the audit file is open for reading only")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("audit: cannot write the audit file: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); err == nil && rbErr != nil {
			err = fmt.Errorf("audit: cannot write the audit file: %w", rbErr)
		}
	}()
	// The number 0 is never the number of an entry.
	if _, err := tx.ExecContext(ctx, `INSERT INTO entries (`+columns+`) VALUES (0,0,'','','','',0,0,'','','','','','','','','','',0,'','','')`); err != nil {
		return fmt.Errorf("audit: cannot write the audit file: %w", err)
	}
	return nil
}

func cloneEvent(ev audit.Event) audit.Event {
	if len(ev.References) == 0 {
		ev.References = nil
		return ev
	}
	ev.References = append([]protocol.Reference(nil), ev.References...)
	return ev
}

type storedRef struct {
	Vault string `json:"vault"`
	Item  string `json:"item"`
	Field string `json:"field"`
}

func encodeRefs(refs []protocol.Reference) (string, error) {
	out := make([]storedRef, len(refs))
	for i, r := range refs {
		out[i] = storedRef{r.Vault, r.Item, r.Field}
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("audit: cannot encode the references: %w", err)
	}
	return string(b), nil
}

func decodeRefs(text string) ([]protocol.Reference, error) {
	var in []storedRef
	if err := json.Unmarshal([]byte(text), &in); err != nil {
		return nil, err
	}
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]protocol.Reference, len(in))
	for i, r := range in {
		out[i] = protocol.Reference{Vault: r.Vault, Item: r.Item, Field: r.Field}
	}
	return out, nil
}

// scanner is the part of sql.Rows that scanEntry needs.
type scanner interface{ Scan(dest ...any) error }

// scanEntry reads one row. refsErr is set if the references column is not
// valid; Verify reports that as a finding and List as an error.
func scanEntry(r scanner) (e audit.Entry, refsErr error, err error) {
	var timeNS int64
	var typ, origin, sid, code, refs string
	err = r.Scan(&e.Seq, &timeNS, &typ, &sid, &origin, &e.Verified.Project, &e.Verified.PeerPID,
		&e.Verified.PeerUID, &e.Verified.Process, &e.Verified.Cwd, &e.Claims.SessionName,
		&e.Claims.Agent, &e.Claims.Task, &e.Claims.Reason, &refs, &code, &e.Result.RequestID,
		&e.Result.GrantID, &e.Result.LifetimeSeconds, &e.Result.Detail, &e.PrevHash, &e.Hash)
	if err != nil {
		return e, nil, err
	}
	e.Time = time.Unix(0, timeNS).UTC()
	e.Type = audit.EventType(typ)
	e.Verified.SessionID = session.ID(sid)
	e.Verified.Origin = session.Origin(origin)
	e.Result.Code = protocol.Code(code)
	e.References, refsErr = decodeRefs(refs)
	return e, refsErr, nil
}

// List returns the entries that f selects, in the sequence of their
// numbers. With a limit, it returns the newest entries.
func (s *Store) List(ctx context.Context, f audit.Filter) ([]audit.Entry, error) {
	var where []string
	var args []any
	add := func(cond string, a ...any) {
		where = append(where, cond)
		args = append(args, a...)
	}
	if f.Project != "" {
		add("project = ?", f.Project)
	}
	if f.SessionName != "" {
		add("session_name = ?", f.SessionName)
	}
	if f.Reference != nil {
		add(`EXISTS (SELECT 1 FROM json_each(entries.refs) j WHERE
			json_extract(j.value, '$.vault') = ? AND json_extract(j.value, '$.item') = ?
			AND json_extract(j.value, '$.field') = ?)`, f.Reference.Vault, f.Reference.Item, f.Reference.Field)
	}
	if len(f.Types) > 0 {
		ph := make([]string, len(f.Types))
		ta := make([]any, len(f.Types))
		for i, t := range f.Types {
			ph[i], ta[i] = "?", string(t)
		}
		add("type IN ("+strings.Join(ph, ",")+")", ta...)
	}
	if f.Code != "" {
		add("code = ?", string(f.Code))
	}
	if !f.Since.IsZero() {
		add("time_ns >= ?", f.Since.UnixNano())
	}
	if !f.Until.IsZero() {
		add("time_ns < ?", f.Until.UnixNano())
	}
	q := `SELECT ` + columns + ` FROM entries`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY seq DESC"
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", f.Limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("audit: cannot read the entries: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []audit.Entry
	for rows.Next() {
		e, refsErr, err := scanEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("audit: cannot read an entry: %w", err)
		}
		if refsErr != nil {
			return nil, fmt.Errorf("audit: entry %d has invalid references", e.Seq)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("audit: cannot read the entries: %w", err)
	}
	// The query read the newest first. Return the sequence of the numbers.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// Verify calculates the chain again. For each entry, in the sequence of
// the stored numbers, it checks three things: the number is the expected
// one (1, 2, 3, ...), PrevHash is the hash of the entry before, and Hash
// matches the canonical form. The result names the first entry that fails.
//
// A gap in the numbers (a deleted entry) is reported with the number of the
// missing entry, because that is the first place where the log is wrong.
// Entries that are removed at the end cannot be seen (known limit).
func (s *Store) Verify(ctx context.Context) (audit.Verification, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+columns+` FROM entries ORDER BY seq`)
	if err != nil {
		return audit.Verification{}, fmt.Errorf("audit: cannot read the entries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var v audit.Verification
	expected, prev := uint64(1), genesisHash
	for rows.Next() {
		e, refsErr, err := scanEntry(rows)
		if err != nil {
			return audit.Verification{}, fmt.Errorf("audit: cannot read an entry: %w", err)
		}
		bad := func(finding string, seq uint64) (audit.Verification, error) {
			v.FirstBadSeq, v.Finding = seq, finding
			return v, nil
		}
		switch {
		case e.Seq != expected:
			return bad(fmt.Sprintf("the number is wrong: expected entry %d, found entry %d", expected, e.Seq), expected)
		case refsErr != nil:
			return bad("the references are not readable", e.Seq)
		case e.PrevHash != prev:
			return bad("the previous hash does not match the entry before", e.Seq)
		case e.Hash != entryHash(e):
			return bad("the hash does not match the content", e.Seq)
		}
		v.Entries++
		expected, prev = e.Seq+1, e.Hash
	}
	if err := rows.Err(); err != nil {
		return audit.Verification{}, fmt.Errorf("audit: cannot read the entries: %w", err)
	}
	v.OK = true
	return v, nil
}
