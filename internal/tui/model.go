// Package tui is the terminal view "a38 tui" of the audit log (feature
// F13, use case UC6). It shows the figures of package auditstats, a table
// of the entries with filters and a detail pane, and the sessions with
// their grants.
//
// The view only reads. It gets the entries and the check of the hash chain
// from a [Source]. It never writes into the audit log and never talks to a
// socket of the daemon.
//
// Each string from the audit log is untrusted: the session name and the
// claims come from the agent. The view cleans each string with
// [auditstats.Clean] and cuts long values before it renders them.
package tui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/auditstats"
	"github.com/seletz/passierschein/internal/clock"
)

// Source gives the entries of the audit log.
type Source struct {
	// Name is the name of the audit file in the status line.
	Name string
	// Load returns all entries in the sequence of their numbers.
	Load func(ctx context.Context) ([]audit.Entry, error)
	// Verify checks the hash chain.
	Verify func(ctx context.Context) (audit.Verification, error)
}

// Options configure the view.
type Options struct {
	// Source gives the entries.
	Source Source
	// Clock gives the current time for the time ranges.
	Clock clock.Clock
	// Location gives the calendar days and the times on the screen.
	Location *time.Location
	// Range is the time range at the start. Empty means 7 days.
	Range auditstats.Range
	// Refresh reads the audit log again after this interval. Zero means
	// only on the key "r".
	Refresh time.Duration
}

// view is one of the three main views.
type view int

const (
	viewOverview view = iota
	viewEvents
	viewSessions
)

var viewNames = []string{"overview", "events", "sessions"}

// overlay is a pane in place of the main view.
type overlay int

const (
	overlayNone overlay = iota
	overlayDetail
	overlayTimeline
)

// input is the mode of the input line.
type input int

const (
	inputNone input = iota
	inputSearch
	inputFilter
)

// filter selects the entries. Project and session name also select the
// figures; the other fields select only the list of events.
type filter struct {
	project, session, ref, code string
	types                       []audit.EventType
}

func (f filter) empty() bool {
	return f.project == "" && f.session == "" && f.ref == "" && f.code == "" && len(f.types) == 0
}

// String returns the filter in the form of the input line.
func (f filter) String() string {
	var parts []string
	add := func(k, v string) {
		if v != "" {
			parts = append(parts, k+"="+v)
		}
	}
	add("project", f.project)
	add("session", f.session)
	add("ref", f.ref)
	var ts []string
	for _, t := range f.types {
		ts = append(ts, string(t))
	}
	add("type", strings.Join(ts, ","))
	add("code", f.code)
	return strings.Join(parts, " ")
}

// parseFilter reads "key=value" words. The keys are project, session, ref,
// type (a list separated by commas) and code.
func parseFilter(s string) (filter, error) {
	var f filter
	for _, w := range strings.Fields(s) {
		k, v, ok := strings.Cut(w, "=")
		if !ok || v == "" {
			return f, fmt.Errorf("%q is not key=value", auditstats.Clean(w))
		}
		switch k {
		case "project":
			f.project = v
		case "session":
			f.session = v
		case "ref":
			f.ref = v
		case "code":
			f.code = v
		case "type":
			for _, t := range strings.Split(v, ",") {
				et := audit.EventType(t)
				if !et.Valid() {
					return f, fmt.Errorf("unknown event type %q", auditstats.Clean(t))
				}
				f.types = append(f.types, et)
			}
		default:
			return f, fmt.Errorf("unknown key %q (project, session, ref, type, code)", auditstats.Clean(k))
		}
	}
	return f, nil
}

// match reports whether the filter fields that select only the list of
// events select e.
func (f filter) match(e audit.Entry) bool {
	if f.code != "" && string(e.Result.Code) != f.code {
		return false
	}
	if len(f.types) > 0 && !slices.Contains(f.types, e.Type) {
		return false
	}
	if f.ref != "" {
		found := false
		for _, r := range e.References {
			if strings.Contains(r.String(), f.ref) {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// matchText reports whether one text field of e contains q, without
// regard to case.
func matchText(e audit.Entry, q string) bool {
	if q == "" {
		return true
	}
	q = strings.ToLower(q)
	fields := []string{
		string(e.Type), string(e.Verified.SessionID), e.Verified.Project, e.Verified.Process, e.Verified.Cwd,
		e.Claims.SessionName, e.Claims.Agent, e.Claims.Task, e.Claims.Reason,
		string(e.Result.Code), e.Result.RequestID, e.Result.GrantID, e.Result.Detail,
	}
	for _, r := range e.References {
		fields = append(fields, r.String())
	}
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), q) {
			return true
		}
	}
	return false
}

// Messages of the commands.
type (
	loadedMsg struct {
		entries []audit.Entry
		err     error
	}
	verifiedMsg struct {
		v   audit.Verification
		err error
	}
	tickMsg struct{}
)

// Model is the Bubble Tea model of the view.
type Model struct {
	opts   Options
	width  int
	height int
	th     theme
	keys   keyMap

	loaded   bool
	loadErr  error
	entries  []audit.Entry
	verified bool
	verify   audit.Verification
	verErr   error

	view    view
	overlay overlay
	help    bool
	rng     auditstats.Range
	filter  filter
	search  string

	input     input
	inputText string
	message   string
	msgErr    bool

	// Derived from the entries, the range and the filter.
	now      time.Time
	scope    auditstats.Scope
	stats    auditstats.Stats
	events   []audit.Entry // newest first
	sessions []auditstats.Session

	// Positions in the lists and panes.
	overviewTop int
	evCursor    int
	evTop       int
	sesCursor   int
	sesTop      int
	paneTop     int
	detail      audit.Entry
	timeline    auditstats.Session
}

// New returns the model. The view reads the audit log in Init.
func New(o Options) *Model {
	if o.Clock == nil {
		o.Clock = clock.System{}
	}
	if o.Location == nil {
		o.Location = time.Local
	}
	if o.Range == "" {
		o.Range = auditstats.Range7Days
	}
	return &Model{opts: o, width: 80, height: 24, rng: o.Range, th: newTheme(true), keys: newKeyMap()}
}

// Init reads the audit log and checks the hash chain in the background,
// and asks the terminal for its background colour. The view uses the dark
// theme until the answer comes.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.load(), m.check(), m.tick(), tea.RequestBackgroundColor)
}

func (m *Model) load() tea.Cmd {
	return func() tea.Msg {
		entries, err := m.opts.Source.Load(context.Background())
		return loadedMsg{entries, err}
	}
}

func (m *Model) check() tea.Cmd {
	if m.opts.Source.Verify == nil {
		return nil
	}
	return func() tea.Msg {
		v, err := m.opts.Source.Verify(context.Background())
		return verifiedMsg{v, err}
	}
}

func (m *Model) tick() tea.Cmd {
	if m.opts.Refresh <= 0 {
		return nil
	}
	clk, d := m.opts.Clock, m.opts.Refresh
	return func() tea.Msg {
		t := clk.NewTimer(d)
		<-t.C()
		return tickMsg{}
	}
}

// Update handles a message.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = max(msg.Width, 20), max(msg.Height, 8)
		m.clamp()
	case loadedMsg:
		m.loaded = true
		m.loadErr = msg.err
		if msg.err == nil {
			m.entries = msg.entries
		}
		m.recompute()
	case verifiedMsg:
		m.verified, m.verify, m.verErr = true, msg.v, msg.err
	case tickMsg:
		return m, tea.Batch(m.load(), m.tick())
	case tea.BackgroundColorMsg:
		m.th = newTheme(msg.IsDark())
	case tea.KeyPressMsg:
		return m.key(msg)
	}
	return m, nil
}

// recompute derives the figures and the lists.
func (m *Model) recompute() {
	m.now = m.opts.Clock.Now()
	sc := m.rng.Scope(m.now, m.opts.Location)
	sc.Project, sc.SessionName = m.filter.project, m.filter.session
	m.scope = sc
	m.stats = auditstats.Compute(m.entries, sc, m.opts.Location, 5)
	m.events = m.events[:0]
	for i := len(m.entries) - 1; i >= 0; i-- {
		e := m.entries[i]
		if sc.Contains(e) && m.filter.match(e) && matchText(e, m.search) {
			m.events = append(m.events, e)
		}
	}
	m.sessions = auditstats.Sessions(m.entries, sc)
	m.clamp()
}

// clamp keeps the cursors in the lists.
func (m *Model) clamp() {
	m.evCursor = clampInt(m.evCursor, 0, len(m.events)-1)
	m.sesCursor = clampInt(m.sesCursor, 0, len(m.sessions)-1)
}

func clampInt(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	return min(max(v, lo), hi)
}

// key handles a key press.
func (m *Model) key(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	km := m.keys
	if key.Matches(k, km.ForceQuit) {
		return m, tea.Quit
	}
	if m.input != inputNone {
		m.editInput(k)
		return m, nil
	}
	m.message, m.msgErr = "", false
	if m.help {
		switch {
		case key.Matches(k, km.Help, km.Back):
			m.help = false
		case key.Matches(k, km.Quit):
			return m, tea.Quit
		}
		return m, nil
	}
	switch {
	case key.Matches(k, km.Quit):
		return m, tea.Quit
	case key.Matches(k, km.Help):
		m.help = true
	case key.Matches(k, km.Overview, km.Events, km.Sessions):
		m.view = view(k.String()[0] - '1')
		m.overlay, m.paneTop = overlayNone, 0
	case key.Matches(k, km.NextView):
		m.view = (m.view + 1) % view(len(viewNames))
		m.overlay, m.paneTop = overlayNone, 0
	case key.Matches(k, km.PrevView):
		m.view = (m.view + view(len(viewNames)) - 1) % view(len(viewNames))
		m.overlay, m.paneTop = overlayNone, 0
	case key.Matches(k, km.Reload):
		m.message = "Reading the audit log again."
		return m, tea.Batch(m.load(), m.check())
	case m.overlay != overlayNone:
		m.paneKey(k)
	case key.Matches(k, km.Range):
		m.rng = m.rng.Next()
		m.recompute()
	case key.Matches(k, km.Search):
		m.input, m.inputText = inputSearch, m.search
		if m.view == viewOverview {
			m.view = viewEvents
		}
	case key.Matches(k, km.Filter):
		m.input, m.inputText = inputFilter, m.filter.String()
	case key.Matches(k, km.Clear):
		m.filter, m.search = filter{}, ""
		m.recompute()
		m.message = "Filter and search are cleared."
	case key.Matches(k, km.Open):
		switch {
		case m.view == viewEvents && len(m.events) > 0:
			m.overlay, m.paneTop, m.detail = overlayDetail, 0, m.events[m.evCursor]
		case m.view == viewSessions && len(m.sessions) > 0:
			m.overlay, m.paneTop, m.timeline = overlayTimeline, 0, m.sessions[m.sesCursor]
		}
	default:
		m.move(k)
	}
	return m, nil
}

// paneKey handles a key in the detail or the timeline.
func (m *Model) paneKey(k tea.KeyPressMsg) {
	km := m.keys
	page := max(m.paneHeight()-1, 1)
	switch {
	case key.Matches(k, km.Back):
		m.overlay, m.paneTop = overlayNone, 0
	case key.Matches(k, km.Down):
		m.paneTop++
	case key.Matches(k, km.Up):
		m.paneTop = max(m.paneTop-1, 0)
	case key.Matches(k, km.PageDown):
		m.paneTop += page
	case key.Matches(k, km.PageUp):
		m.paneTop = max(m.paneTop-page, 0)
	case key.Matches(k, km.Top):
		m.paneTop = 0
	case key.Matches(k, km.Bottom):
		m.paneTop = 1 << 30
	}
}

// move moves the cursor of the current view.
func (m *Model) move(k tea.KeyPressMsg) {
	km := m.keys
	page := max(m.paneHeight()-2, 1)
	delta := 0
	switch {
	case key.Matches(k, km.Down):
		delta = 1
	case key.Matches(k, km.Up):
		delta = -1
	case key.Matches(k, km.PageDown):
		delta = page
	case key.Matches(k, km.PageUp):
		delta = -page
	case key.Matches(k, km.Top):
		delta = -1 << 30
	case key.Matches(k, km.Bottom):
		delta = 1 << 30
	default:
		return
	}
	switch m.view {
	case viewOverview:
		m.overviewTop = max(m.overviewTop+clampInt(delta, -1<<20, 1<<20), 0)
	case viewEvents:
		m.evCursor = clampInt(m.evCursor+delta, 0, len(m.events)-1)
	case viewSessions:
		m.sesCursor = clampInt(m.sesCursor+delta, 0, len(m.sessions)-1)
	}
}

// editInput handles a key in the input line.
func (m *Model) editInput(k tea.KeyPressMsg) {
	switch k.String() {
	case "esc":
		m.input = inputNone
	case "enter":
		switch m.input {
		case inputSearch:
			m.search = strings.TrimSpace(m.inputText)
			m.evCursor = 0
		case inputFilter:
			f, err := parseFilter(m.inputText)
			if err != nil {
				m.message, m.msgErr = "Filter not changed: "+err.Error(), true
				m.input = inputNone
				return
			}
			m.filter = f
			m.evCursor, m.sesCursor = 0, 0
		}
		m.input = inputNone
		m.recompute()
	case "backspace":
		if r := []rune(m.inputText); len(r) > 0 {
			m.inputText = string(r[:len(r)-1])
		}
	case "ctrl+u":
		m.inputText = ""
	default:
		if t := k.Text; t != "" && len([]rune(m.inputText)) < 200 {
			m.inputText += auditstats.Clean(t)
		}
	}
}
