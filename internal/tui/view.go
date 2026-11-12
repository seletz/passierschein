package tui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/auditstats"
	"github.com/seletz/passierschein/internal/protocol"
)

// The styles use the 16 colours of the terminal, so that the theme of the
// terminal decides the exact colour (light or dark). Colour is only an aid:
// each state also has a word. NO_COLOR removes the colours.
var (
	stBold  = lipgloss.NewStyle().Bold(true)
	stRev   = lipgloss.NewStyle().Reverse(true)
	stTab   = lipgloss.NewStyle().Reverse(true).Bold(true)
	stBad   = lipgloss.NewStyle().Foreground(lipgloss.Red).Bold(true)
	stWarn  = lipgloss.NewStyle().Foreground(lipgloss.Yellow)
	stGood  = lipgloss.NewStyle().Foreground(lipgloss.Green)
	stBar   = lipgloss.NewStyle().Foreground(lipgloss.Cyan)
	stFaint = lipgloss.NewStyle().Faint(true)
	stAlarm = lipgloss.NewStyle().Reverse(true).Bold(true).Foreground(lipgloss.Red)
)

// cut cleans s and shortens it to n cells with "…".
func cut(s string, n int) string {
	if n <= 0 {
		return ""
	}
	return ansi.Truncate(auditstats.Clean(s), n, "…")
}

// cutLeft cleans s and shortens it to n cells at the start with "…". The
// end of a reference (item and field) says more than its start.
func cutLeft(s string, n int) string {
	if n <= 0 {
		return ""
	}
	s = auditstats.Clean(s)
	if w := ansi.StringWidth(s); w > n {
		return ansi.TruncateLeft(s, w-n+1, "…")
	}
	return s
}

// pad cleans s, cuts it to n cells and fills it with spaces to n cells.
func pad(s string, n int) string {
	s = cut(s, n)
	return s + strings.Repeat(" ", max(n-ansi.StringWidth(s), 0))
}

// padLeft is pad with the spaces on the left side.
func padLeft(s string, n int) string {
	s = cut(s, n)
	return strings.Repeat(" ", max(n-ansi.StringWidth(s), 0)) + s
}

// dash returns "-" for an empty text.
func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func (m *Model) bodyHeight() int { return max(m.height-3, 1) }

// View renders the screen: the header, the body, the status line and the
// line for keys and input.
func (m *Model) View() tea.View {
	w, bh := m.width, m.bodyHeight()
	var body []string
	switch {
	case m.overlay == overlayHelp:
		body = m.scroll(m.helpLines(w), &m.paneTop, bh)
	case !m.loaded:
		body = []string{"", " Reading the audit log …"}
	case m.loadErr != nil:
		body = []string{"", stBad.Render(" Cannot read the audit log."), " " + cut(m.loadErr.Error(), w-2), "", " Press r to try again or q to quit."}
	case m.overlay == overlayDetail:
		body = m.scroll(m.detailLines(w), &m.paneTop, bh)
	case m.overlay == overlayTimeline:
		body = m.scroll(m.timelineLines(w), &m.paneTop, bh)
	case m.view == viewOverview:
		body = m.scroll(m.overviewLines(w), &m.overviewTop, bh)
	case m.view == viewEvents:
		body = m.eventLines(w, bh)
	case m.view == viewSessions:
		body = m.sessionLines(w, bh)
	}
	lines := make([]string, 0, m.height)
	lines = append(lines, m.headerLine(w))
	for i := 0; i < bh; i++ {
		l := ""
		if i < len(body) {
			l = body[i]
		}
		lines = append(lines, ansi.Truncate(l, w, ""))
	}
	lines = append(lines, m.statusLine(w), m.bottomLine(w))
	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	return v
}

// scroll returns the visible part of lines. It keeps *top in the range and
// marks hidden lines below.
func (m *Model) scroll(lines []string, top *int, h int) []string {
	*top = clampInt(*top, 0, max(len(lines)-h, 0))
	end := min(*top+h, len(lines))
	if end < len(lines) && h > 1 {
		// The last line of the screen marks the hidden lines.
		end--
		out := append([]string(nil), lines[*top:end]...)
		return append(out, stFaint.Render(fmt.Sprintf(" ↓ %d more lines (j, pgdown)", len(lines)-end)))
	}
	return append([]string(nil), lines[*top:end]...)
}

func (m *Model) headerLine(w int) string {
	var b strings.Builder
	b.WriteString(stBold.Render(" a38 tui "))
	for i, name := range viewNames {
		label := fmt.Sprintf(" %d %s ", i+1, name)
		if view(i) == m.view && m.overlay != overlayHelp {
			label = stTab.Render(label)
		}
		b.WriteString(" " + label)
	}
	right := "? help  q quit "
	used := ansi.StringWidth(b.String())
	if gap := w - used - len(right); gap > 0 {
		b.WriteString(strings.Repeat(" ", gap) + stFaint.Render(right))
	}
	return ansi.Truncate(b.String(), w, "")
}

func (m *Model) statusLine(w int) string {
	line := m.statusText(m.opts.Source.Name)
	if ansi.StringWidth(line) > w {
		// Short of space: the base name of the file leaves room for the
		// filter.
		line = m.statusText(filepath.Base(m.opts.Source.Name))
	}
	if n := w - ansi.StringWidth(line); n > 0 {
		line += stRev.Render(strings.Repeat(" ", n))
	}
	return ansi.Truncate(line, w, "")
}

func (m *Model) statusText(name string) string {
	var segs []string
	add := func(s string) { segs = append(segs, stRev.Render(" "+s+" ")) }
	add(cut(name, 40))
	add(fmt.Sprintf("%d entries", len(m.entries)))
	switch {
	case !m.verified:
		add("chain: checking …")
	case m.verErr != nil:
		segs = append(segs, stAlarm.Render(" chain: CHECK FAILED "))
	case m.verify.OK:
		add("chain intact")
	default:
		segs = append(segs, stAlarm.Render(fmt.Sprintf(" CHAIN BROKEN at entry %d ", m.verify.FirstBadSeq)))
	}
	add(m.rng.Label())
	if !m.filter.empty() {
		add("filter: " + m.filter.String())
	}
	if m.search != "" {
		add("search: " + strconv.Quote(m.search))
	}
	return strings.Join(segs, stRev.Render("│"))
}

func (m *Model) bottomLine(w int) string {
	switch m.input {
	case inputSearch:
		return ansi.Truncate(stBold.Render(" Search: ")+m.inputText+"█"+stFaint.Render("  enter apply · esc cancel"), w, "")
	case inputFilter:
		return ansi.Truncate(stBold.Render(" Filter: ")+m.inputText+"█"+stFaint.Render("  project= session= ref= type= code="), w, "")
	}
	if m.message != "" {
		return " " + cut(m.message, w-2)
	}
	var hint string
	switch {
	case m.overlay != overlayNone:
		hint = "j/k scroll · esc back · q quit"
	case m.view == viewOverview:
		hint = "t range · f filter · j/k scroll · tab view · r refresh · ? help"
	case m.view == viewEvents:
		hint = "enter detail · / search · f filter · c clear · t range · r refresh"
	default:
		hint = "enter timeline · f filter · t range · r refresh · ? help"
	}
	return stFaint.Render(" " + cut(hint, w-2))
}

// ---- overview ----------------------------------------------------------

// percent returns n of total as a whole percent.
func percent(n, total int) string {
	if total == 0 {
		return "0%"
	}
	return fmt.Sprintf("%d%%", (n*100+total/2)/total)
}

func (m *Model) rangeText() string {
	st := m.stats
	switch {
	case len(st.Days) == 0:
		return m.rng.Label()
	case len(st.Days) == 1:
		return m.rng.Label() + ", " + st.Days[0].Date
	}
	return fmt.Sprintf("%s, %s to %s", m.rng.Label(), st.Days[0].Date, st.Days[len(st.Days)-1].Date)
}

func (m *Model) overviewLines(w int) []string {
	st := m.stats
	row := func(label string, n int, style lipgloss.Style, note string) string {
		num := padLeft(strconv.Itoa(n), 5)
		if n > 0 {
			num = style.Render(num)
		}
		return " " + pad(label, 13) + num + "   " + cut(note, max(w-23, 10))
	}
	plain := lipgloss.NewStyle()
	var lines []string
	if m.verified && m.verErr == nil && !m.verify.OK {
		lines = append(lines, stAlarm.Render(" "+cut(fmt.Sprintf("The hash chain is broken at entry %d: %s.", m.verify.FirstBadSeq, m.verify.Finding), w-2)+" "),
			stBad.Render(" "+cut("The figures may be wrong. a38 audit verify shows the same finding.", w-2)))
	}
	lines = append(lines, " Range: "+m.rangeText()+stFaint.Render("  (t changes it)"))
	if st.Entries == 0 {
		lines = append(lines, "", " No entries in this range.", " Press t for a longer range, or c to clear the filter.")
		return lines
	}
	errs := make([]string, 0, len(st.ErrorCodes))
	for _, c := range st.ErrorCodes {
		errs = append(errs, fmt.Sprintf("%s %d", dash(c.Name), c.Count))
	}
	lines = append(lines,
		stBold.Render(row("POPUPS", st.Popups, plain, fmt.Sprintf("%.1f per day, busiest day %s with %d", st.PopupsPerDay, dash(st.BusiestDay.Date), st.BusiestDay.Popups))),
		row(" approved", st.Approved, stGood, fmt.Sprintf("%s (partial %d)", percent(st.Approved, st.Popups), st.Partial)),
		row(" denied", st.Denied, stBad, percent(st.Denied, st.Popups)+" denial share"),
		row(" timed out", st.TimedOut, stWarn, fmt.Sprintf("%s (UI failed %d, session ended %d)", percent(st.TimedOut, st.Popups), st.Failed, st.Abandoned)),
		row(" deviations", st.Deviations, stWarn, percent(st.Deviations, st.Popups)+" went beyond a live grant"),
		" "+pad(" size", 13)+padLeft(formatMedian(st.BundleMedian), 5)+"   "+cut("median entries per popup, largest "+strconv.Itoa(st.BundleMax), max(w-23, 10)),
		stBold.Render(row("DELIVERIES", st.Deliveries, plain, fmt.Sprintf("%s under a grant without popup (%d), %d after a popup",
			percent(st.DeliveriesUnderGrant, st.Deliveries), st.DeliveriesUnderGrant, st.DeliveriesAfterPopup))),
		stBold.Render(row("DENIED EARLY", st.PolicyDenials, plain, "by the policy, an unknown item or a full bundle")),
		" "+cut(fmt.Sprintf("REVOCATIONS %d · ERRORS %d · LOCKS %d (ended %d)", st.Revocations, st.Errors, st.LockBegins, st.LockEnds), w-2),
		"",
		stBold.Render(fmt.Sprintf(" POPUPS PER DAY (largest %d)", st.BusiestDay.Popups)),
	)
	lines = append(lines, chart(st.Days, w, 5)...)
	lines = append(lines, "")
	if len(errs) > 0 {
		lines = append(lines, " "+stBold.Render("ERRORS BY CODE")+"  "+cut(strings.Join(errs, ", "), max(w-18, 10)), "")
	}
	lines = append(lines, rankTables(st, w)...)
	return lines
}

func formatMedian(v float64) string {
	if v == float64(int(v)) {
		return strconv.Itoa(int(v))
	}
	return strconv.FormatFloat(v, 'f', 1, 64)
}

// rankTables renders the busiest sessions and projects side by side and
// the most requested credentials below them.
func rankTables(st auditstats.Stats, w int) []string {
	half := max((w-3)/2, 30)
	table := func(title string, ranks []auditstats.Rank, width int) []string {
		nameW := max(width-16, 8)
		out := []string{stBold.Render(" "+pad(title, nameW)) + stBold.Render(padLeft("REQ", 5)+padLeft("POP", 5)+padLeft("DEL", 5))}
		for _, r := range ranks {
			out = append(out, "  "+pad(r.Name, nameW-1)+padLeft(strconv.Itoa(r.Requests), 5)+padLeft(strconv.Itoa(r.Popups), 5)+padLeft(strconv.Itoa(r.Deliveries), 5))
		}
		return out
	}
	var lines []string
	sess := table("BUSIEST SESSIONS", st.TopSessions, half)
	proj := table("BUSIEST PROJECTS", st.TopProjects, half)
	if w >= 2*30+3 {
		for i := 0; i < max(len(sess), len(proj)); i++ {
			l, r := "", ""
			if i < len(sess) {
				l = sess[i]
			}
			if i < len(proj) {
				r = proj[i]
			}
			lines = append(lines, l+strings.Repeat(" ", max(half-ansi.StringWidth(l), 0))+" │ "+r)
		}
	} else {
		lines = append(append(lines, sess...), proj...)
	}
	lines = append(lines, "")
	nameW := max(w-19, 20)
	lines = append(lines, stBold.Render(" "+pad("MOST REQUESTED CREDENTIALS", nameW)+padLeft("REQ", 5)+padLeft("DEL", 5)+padLeft("DENY", 6)))
	for _, r := range st.TopReferences {
		lines = append(lines, "  "+pad(r.Name, nameW-1)+padLeft(strconv.Itoa(r.Requests), 5)+padLeft(strconv.Itoa(r.Deliveries), 5)+padLeft(strconv.Itoa(r.Denials), 6))
	}
	return lines
}

// blocks are the eighths of a cell for the bars.
var blocks = []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

// chart renders one bar per day (or per group of days) with h rows.
func chart(days []auditstats.Day, w, h int) []string {
	if len(days) == 0 {
		return []string{"  (no days)"}
	}
	avail := max(w-8, 10)
	per := 1
	for (len(days)+per-1)/per > avail {
		per++
	}
	var vals []int
	var labels []string
	for i := 0; i < len(days); i += per {
		v := 0
		for j := i; j < min(i+per, len(days)); j++ {
			v += days[j].Popups
		}
		vals = append(vals, v)
		labels = append(labels, days[i].Date[5:])
	}
	// One cell of gap between bars; a few days get wider bars.
	colW := clampInt(avail/len(vals), 1, 6)
	top := 0
	for _, v := range vals {
		top = max(top, v)
	}
	var lines []string
	for r := h - 1; r >= 0; r-- {
		prefix := "     │"
		if r == h-1 {
			prefix = fmt.Sprintf(" %3d ┤", top)
		}
		var b strings.Builder
		for _, v := range vals {
			cell := " "
			if top > 0 {
				eighths := v*h*8/top - r*8
				if v > 0 && r == 0 && eighths <= 0 {
					eighths = 1
				}
				cell = blocks[clampInt(eighths, 0, 8)]
			}
			b.WriteString(strings.Repeat(cell, max(colW-1, 1)) + strings.Repeat(" ", min(colW-1, 1)))
		}
		lines = append(lines, prefix+stBar.Render(b.String()))
	}
	n := len(vals) * colW
	lines = append(lines, "   0 └"+strings.Repeat("─", n))
	axis := []rune(strings.Repeat(" ", n+6))
	put := func(at int, s string) {
		for i, c := range s {
			if at+i < len(axis) {
				axis[at+i] = c
			}
		}
	}
	if colW >= 6 {
		// Wide bars: a date under each bar.
		for i, l := range labels {
			put(6+i*colW, l)
		}
	} else {
		put(6, labels[0])
		if last := labels[len(labels)-1]; n >= 12 {
			put(6+n-len(last)-colW+1, last)
		}
	}
	if per > 1 {
		put(6+n/2-4, fmt.Sprintf("(%d days per bar)", per))
	}
	lines = append(lines, strings.TrimRight(string(axis), " "))
	return lines
}

// ---- events ------------------------------------------------------------

// resultText returns the short result of an entry: the code, a detail
// that code compares, "approved" for a decision, "-" for a request and a
// bundle, or "ok".
func resultText(e audit.Entry) string {
	switch e.Result.Detail {
	case audit.DetailFaultBegin, audit.DetailFaultEnd, audit.DetailSessionEnded, audit.DetailDaemonStopped, audit.DetailLifetimeEnded:
		return e.Result.Detail
	}
	switch {
	case e.Result.Code != "":
		return string(e.Result.Code)
	case e.Type == audit.EventDecision:
		return "approved"
	case e.Type == audit.EventRequest, e.Type == audit.EventBundle:
		return "-"
	}
	return "ok"
}

// resultStyle colours a result: a denial or a fault in red, a timeout in
// yellow.
func resultStyle(e audit.Entry) lipgloss.Style {
	switch {
	case e.Result.Code == protocol.CodeTimeout:
		return stWarn
	case e.Result.Code != "", e.Type == audit.EventDenial:
		return stBad
	case e.Type == audit.EventDelivery:
		return stGood
	}
	return lipgloss.NewStyle()
}

func refsText(refs []protocol.Reference) string {
	if len(refs) == 0 {
		return "-"
	}
	s := refs[0].String()
	if len(refs) > 1 {
		s += fmt.Sprintf(" +%d", len(refs)-1)
	}
	return s
}

// eventColumns returns the widths of project, session, result and
// references for the width of the screen.
func eventColumns(w int) (proj, sess, res, refs int) {
	rest := w - 1 - 5 - 1 - 11 - 1 - 10 - 1
	if w >= 110 {
		proj, sess, res = 16, 20, 14
		refs = rest - proj - sess - res - 3
		return
	}
	proj, sess = 15, 19
	res = max(rest-proj-sess-2, 8)
	return
}

func (m *Model) eventLines(w, h int) []string {
	proj, sess, res, refs := eventColumns(w)
	n := len(m.events)
	rows := max(h-2, 1)
	if m.evCursor < m.evTop {
		m.evTop = m.evCursor
	}
	if m.evCursor >= m.evTop+rows {
		m.evTop = m.evCursor - rows + 1
	}
	m.evTop = clampInt(m.evTop, 0, max(n-rows, 0))
	title := fmt.Sprintf(" EVENTS %d–%d of %d, newest first", min(m.evTop+1, n), min(m.evTop+rows, n), n)
	if n == 0 {
		title = " EVENTS: none match the range, the filter and the search"
	}
	head := " " + padLeft("SEQ", 5) + " " + pad("TIME", 11) + " " + pad("TYPE", 10) + " " + pad("PROJECT", proj) + " " + pad("SESSION", sess) + " " + pad("RESULT", res)
	if refs > 0 {
		head += " " + pad("REFERENCES", refs)
	}
	lines := []string{stBold.Render(title), stBold.Render(head)}
	for i := m.evTop; i < min(m.evTop+rows, n); i++ {
		e := m.events[i]
		cells := []string{
			padLeft(strconv.FormatUint(e.Seq, 10), 5),
			pad(e.Time.In(m.opts.Location).Format("01-02 15:04"), 11),
			pad(string(e.Type), 10),
			pad(dash(e.Verified.Project), proj),
			pad(dash(e.Claims.SessionName), sess),
		}
		result := pad(resultText(e), res)
		ref := ""
		if refs > 0 {
			ref = " " + cutLeft(refsText(e.References), refs)
		}
		if i == m.evCursor {
			line := " " + strings.Join(cells, " ") + " " + result + ref
			lines = append(lines, stRev.Render(line+strings.Repeat(" ", max(w-ansi.StringWidth(line), 0))))
			continue
		}
		lines = append(lines, " "+strings.Join(cells, " ")+" "+resultStyle(e).Render(result)+ref)
	}
	return lines
}

// chainText says where an entry stands in the hash chain.
func (m *Model) chainText(seq uint64) string {
	switch {
	case !m.verified:
		return "the check of the chain is running"
	case m.verErr != nil:
		return "the check of the chain failed: " + m.verErr.Error()
	case m.verify.OK:
		if seq <= m.verify.Entries {
			return "intact up to this entry"
		}
		return "written after the check (r checks again)"
	case seq < m.verify.FirstBadSeq:
		return fmt.Sprintf("intact up to this entry; the chain breaks at entry %d", m.verify.FirstBadSeq)
	case seq == m.verify.FirstBadSeq:
		return "BROKEN HERE: " + m.verify.Finding
	}
	return fmt.Sprintf("not checked: the chain breaks before, at entry %d", m.verify.FirstBadSeq)
}

// wrap cleans s and breaks it into lines of at most n cells, at most
// maxLines lines; the last line ends with "…" if text is cut.
func wrap(s string, n, maxLines int) []string {
	s = auditstats.Clean(s)
	if s == "" {
		return []string{"-"}
	}
	var out []string
	for _, l := range strings.Split(ansi.Wrap(s, n, " /-"), "\n") {
		out = append(out, ansi.Truncate(l, n, "…"))
	}
	if len(out) > maxLines {
		out = out[:maxLines]
		out[maxLines-1] = ansi.Truncate(out[maxLines-1], n-1, "") + "…"
	}
	return out
}

func (m *Model) detailLines(w int) []string {
	e := m.detail
	vw := max(w-16, 20)
	field := func(label, value string) []string {
		ls := wrap(value, vw, 4)
		out := []string{"  " + pad(label, 13) + ls[0]}
		for _, l := range ls[1:] {
			out = append(out, "  "+strings.Repeat(" ", 13)+l)
		}
		return out
	}
	var lines []string
	add := func(l ...string) { lines = append(lines, l...) }
	last := uint64(0)
	if len(m.entries) > 0 {
		last = m.entries[len(m.entries)-1].Seq
	}
	local := e.Time.In(m.opts.Location)
	add(stBold.Render(fmt.Sprintf(" ENTRY %d", e.Seq)) + stFaint.Render("  (esc back)"))
	when := local.Format("2006-01-02 15:04:05 MST")
	if _, off := local.Zone(); off != 0 {
		when += " (" + e.Time.UTC().Format("15:04:05") + " UTC)"
	}
	add(field("Time", when)...)
	add(field("Type", string(e.Type))...)
	add("  " + pad("Result", 13) + resultStyle(e).Render(cut(resultText(e), vw)))
	add(field("Detail", dash(e.Result.Detail))...)
	add(field("Request ID", dash(e.Result.RequestID))...)
	add(field("Grant ID", dash(e.Result.GrantID))...)
	if e.Result.LifetimeSeconds > 0 {
		add(field("Lifetime", (time.Duration(e.Result.LifetimeSeconds) * time.Second).String())...)
	}
	if len(e.References) == 0 {
		add(field("References", "-")...)
	}
	for i, r := range e.References {
		label := ""
		if i == 0 {
			label = "References"
		}
		add("  " + pad(label, 13) + cut(r.String(), vw))
	}
	add("", stBold.Render(" VERIFIED BY THE DAEMON"))
	add(field("Session ID", dash(string(e.Verified.SessionID)))...)
	add(field("Project", dash(e.Verified.Project))...)
	origin := dash(string(e.Verified.Origin))
	if e.Verified.SessionID != "" {
		origin += fmt.Sprintf(", user %d, process %d", e.Verified.PeerUID, e.Verified.PeerPID)
	}
	add(field("Origin", origin)...)
	add(field("Program", dash(e.Verified.Process))...)
	add(field("Directory", dash(e.Verified.Cwd))...)
	add("", stBold.Render(" CLAIMS OF THE AGENT (NOT VERIFIED)"))
	add(field("Session name", dash(e.Claims.SessionName))...)
	add(field("Agent", dash(e.Claims.Agent))...)
	add(field("Task", dash(e.Claims.Task))...)
	add(field("Reason", dash(e.Claims.Reason))...)
	add("", stBold.Render(" HASH CHAIN"))
	add(field("Position", fmt.Sprintf("entry %d of %d", e.Seq, last))...)
	chain := m.chainText(e.Seq)
	if m.verified && m.verErr == nil && !m.verify.OK && e.Seq >= m.verify.FirstBadSeq {
		add("  " + pad("Chain", 13) + stBad.Render(cut(chain, vw)))
	} else {
		add(field("Chain", chain)...)
	}
	add("  " + pad("Prev hash", 13) + cut(e.PrevHash, vw))
	add("  " + pad("Hash", 13) + cut(e.Hash, vw))
	return lines
}

// ---- sessions ----------------------------------------------------------

func (m *Model) sessionLines(w, h int) []string {
	n := len(m.sessions)
	rows := max(h-2, 1)
	if m.sesCursor < m.sesTop {
		m.sesTop = m.sesCursor
	}
	if m.sesCursor >= m.sesTop+rows {
		m.sesTop = m.sesCursor - rows + 1
	}
	m.sesTop = clampInt(m.sesTop, 0, max(n-rows, 0))
	nameW, projW, idW := 20, 15, 0
	if w >= 100 {
		nameW, projW, idW = 24, 18, 10
	}
	title := fmt.Sprintf(" SESSIONS %d–%d of %d, latest activity first", min(m.sesTop+1, n), min(m.sesTop+rows, n), n)
	if n == 0 {
		title = " SESSIONS: none in the range and the filter"
	}
	num := func(s string) string { return padLeft(s, 5) }
	head := " " + pad("SESSION NAME", nameW) + " " + pad("PROJECT", projW) + " " + pad("LAST", 11)
	if idW > 0 {
		head += " " + pad("ID", idW)
	}
	head += num("REQ") + num("POP") + num("GRNT") + num("DEL") + num("DENY")
	lines := []string{stBold.Render(title), stBold.Render(head)}
	for i := m.sesTop; i < min(m.sesTop+rows, n); i++ {
		s := m.sessions[i]
		line := " " + pad(dash(s.Name), nameW) + " " + pad(dash(s.Project), projW) + " " + pad(s.Last.In(m.opts.Location).Format("01-02 15:04"), 11)
		if idW > 0 {
			line += " " + pad(s.ID, idW)
		}
		line += num(strconv.Itoa(s.Requests)) + num(strconv.Itoa(s.Popups)) + num(strconv.Itoa(s.Grants)) + num(strconv.Itoa(s.Deliveries)) + num(strconv.Itoa(s.Denials))
		if i == m.sesCursor {
			line = stRev.Render(line + strings.Repeat(" ", max(w-ansi.StringWidth(line), 0)))
		}
		lines = append(lines, line)
	}
	return lines
}

func (m *Model) timelineLines(w int) []string {
	s := m.timeline
	loc := m.opts.Location
	var lines []string
	add := func(l ...string) { lines = append(lines, l...) }
	add(stBold.Render(" SESSION "+cut(dash(s.Name), max(w-30, 10))) + stFaint.Render(" (claimed name, esc back)"))
	add(" " + cut(fmt.Sprintf("ID %s · project %s · agent %s (claimed)", s.ID, dash(s.Project), dash(s.Agent)), w-2))
	add(" " + cut(fmt.Sprintf("%s to %s · requests %d · popups %d · grants %d · deliveries %d · denials %d",
		s.First.In(loc).Format("2006-01-02 15:04"), s.Last.In(loc).Format("15:04"), s.Requests, s.Popups, s.Grants, s.Deliveries, s.Denials), w-2))
	add("", stBold.Render(" GRANTS"))
	grants := auditstats.Grants(m.entries, s.ID)
	if len(grants) == 0 {
		add("  none")
	}
	for _, g := range grants {
		end := "no end event"
		if !g.Ended.IsZero() {
			end = "ended " + g.Ended.In(loc).Format("15:04") + " " + string(g.End)
		}
		add("  " + cut(fmt.Sprintf("%s  %s for %s  %s  %d deliveries  %d credentials",
			g.ID, g.Granted.In(loc).Format("15:04"), shortDuration(g.Lifetime), end, g.Deliveries, len(g.References)), w-3))
	}
	add("", stBold.Render(" TIMELINE"))
	resW := 16
	refW := max(w-2-14-1-10-1-resW-1, 10)
	for _, e := range m.entries {
		if string(e.Verified.SessionID) != s.ID || !m.scope.Contains(e) {
			continue
		}
		extra := refsText(e.References)
		switch {
		case e.Type == audit.EventBundle:
			extra = fmt.Sprintf("%s, %d entries", e.Result.RequestID, len(e.References))
		case e.Type == audit.EventGrant:
			extra = fmt.Sprintf("%s for %s, %s", e.Result.GrantID, shortDuration(time.Duration(e.Result.LifetimeSeconds)*time.Second), refsText(e.References))
		case e.Result.GrantID != "":
			extra = e.Result.GrantID + ", " + refsText(e.References)
		}
		add("  " + e.Time.In(loc).Format("01-02 15:04:05") + " " + pad(string(e.Type), 10) + " " +
			resultStyle(e).Render(pad(resultText(e), resW)) + " " + cut(extra, refW))
	}
	return lines
}

// shortDuration writes a lifetime such as "4h" or "90m".
func shortDuration(d time.Duration) string {
	switch {
	case d <= 0:
		return "-"
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", int(d/time.Hour))
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	}
	return d.String()
}

// ---- help --------------------------------------------------------------

func (m *Model) helpLines(w int) []string {
	keys := [][2]string{
		{"1 2 3, tab", "switch the view: overview, events, sessions"},
		{"j k, ↓ ↑", "move or scroll; pgdown pgup, g G: start and end"},
		{"enter", "detail of an event, timeline of a session"},
		{"esc", "back"},
		{"/", "search text in the events"},
		{"f", "filter: project= session= ref= type= code="},
		{"c", "clear the filter and the search"},
		{"t", "time range: today, 7 days, 30 days, all"},
		{"r", "read the audit log again and check the chain"},
		{"?", "this help"},
		{"q, ctrl+c", "quit"},
	}
	lines := []string{stBold.Render(" KEYS") + stFaint.Render("  (esc back)")}
	for _, k := range keys {
		lines = append(lines, "  "+pad(k[0], 13)+cut(k[1], w-16))
	}
	notes := []string{
		"The view only reads the audit log. It never writes and never talks to the daemon.",
		"The filter for project and session also selects the figures. The filter for ref, type and code and the search select only the events.",
		"Session name, agent, task and reason are claims of the agent and not verified.",
		"The how-to \"Freigaben auswerten\" explains each figure.",
	}
	lines = append(lines, "", stBold.Render(" NOTES"))
	for _, n := range notes {
		for _, l := range wrap(n, w-4, 3) {
			lines = append(lines, "  "+l)
		}
	}
	return lines
}
