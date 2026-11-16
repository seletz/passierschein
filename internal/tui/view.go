package tui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/auditstats"
	"github.com/seletz/passierschein/internal/protocol"
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

// The rows of the frame: the header and a rule above the body, a rule and
// the footer below it.
const chromeHeight = 4

// bodyHeight is the number of rows between the rules.
func (m *Model) bodyHeight() int { return max(m.height-chromeHeight, 2) }

// paneHeight is the number of rows of a view below the context line.
func (m *Model) paneHeight() int { return max(m.bodyHeight()-1, 1) }

// View renders the screen: the header with the program name and the tabs,
// a rule, the context line and the view, a rule and the footer with the
// key hints. The help is a panel over the body.
func (m *Model) View() tea.View {
	w, bh, ph := m.width, m.bodyHeight(), m.paneHeight()
	var pane []string
	switch {
	case !m.loaded:
		pane = []string{"", m.th.Muted.Render(" Reading the audit log …")}
	case m.loadErr != nil:
		pane = []string{"", m.th.BadText.Render(" Cannot read the audit log."), " " + cut(m.loadErr.Error(), w-2), "",
			m.th.Muted.Render(" Press R to try again or q to quit.")}
	case m.overlay == overlayDetail:
		pane = m.scroll(m.detailLines(w), &m.paneTop, ph)
	case m.overlay == overlayTimeline:
		pane = m.scroll(m.timelineLines(w), &m.paneTop, ph)
	case m.view == viewOverview:
		pane = m.scroll(m.overviewLines(w), &m.overviewTop, ph)
	case m.view == viewEvents:
		pane = m.eventLines(w, ph)
	case m.view == viewSessions:
		pane = m.sessionLines(w, ph)
	}
	body := make([]string, 0, bh)
	body = append(body, m.contextLine(w))
	for i := 0; i < ph; i++ {
		l := ""
		if i < len(pane) {
			l = pane[i]
		}
		body = append(body, l)
	}
	if m.help {
		body = strings.Split(m.helpPanel(w, bh), "\n")
	}
	rule := m.th.Rule.Render(strings.Repeat("─", w))
	lines := make([]string, 0, m.height)
	lines = append(lines, m.header(w), rule)
	for i := 0; i < bh; i++ {
		l := ""
		if i < len(body) {
			l = body[i]
		}
		lines = append(lines, ansi.Truncate(l, w, ""))
	}
	lines = append(lines, rule, m.footer(w))
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
		return append(out, m.th.Muted.Render(fmt.Sprintf(" ↓ %d more lines", len(lines)-end)))
	}
	return append([]string(nil), lines[*top:end]...)
}

// header renders the program name in the gradient of the theme on the
// left, the time of the last read (muted) in the middle and the tabs on
// the right. The active tab is in brackets on the accent colour, so it
// reads without colour too; an open pane follows it after "›". The time
// gives way to the tabs in a narrow window.
func (m *Model) header(w int) string {
	left := " " + m.th.gradient("a38 tui")
	right := strings.Join(m.tabs(), " ") + " "
	gap := w - ansi.StringWidth(left) - ansi.StringWidth(right)
	status := ""
	switch {
	case m.loadErr != nil:
		status = m.th.BadText.Render("read failed")
	case m.loaded:
		status = m.th.Muted.Render("updated " + m.now.In(m.opts.Location).Format("15:04:05"))
	}
	if gap < 1 {
		return ansi.Truncate(left+" "+right, w, "")
	}
	if status == "" || ansi.StringWidth(status)+2 > gap {
		return left + strings.Repeat(" ", gap) + right
	}
	pad := (gap - ansi.StringWidth(status)) / 2
	return left + strings.Repeat(" ", pad) + status + strings.Repeat(" ", gap-pad-ansi.StringWidth(status)) + right
}

// tabs renders the tabs of the header: the number of an inactive tab in
// the accent colour, the active tab in brackets on the accent colour.
func (m *Model) tabs() []string {
	tabs := make([]string, len(viewNames))
	for i, name := range viewNames {
		n := strconv.Itoa(i + 1)
		if view(i) != m.view {
			tabs[i] = " " + m.th.Mark.Render(n) + " " + name + " "
			continue
		}
		t := m.th.Active.Render("[" + n + " " + name + "]")
		var crumbs []string
		switch m.overlay {
		case overlayDetail:
			crumbs = append(crumbs, "entry "+strconv.FormatUint(m.detail.Seq, 10))
		case overlayTimeline:
			crumbs = append(crumbs, "session "+cut(dash(m.timeline.Name), 24))
		}
		if m.help {
			crumbs = append(crumbs, "help")
		}
		for _, c := range crumbs {
			t += m.th.AccentText.Render(" › " + c)
		}
		tabs[i] = t
	}
	return tabs
}

// contextLine is the first line of the body: the input line while it is
// open, else a message, else the state of the view: the audit file, the
// number of entries, the hash chain, the range, the filter and the search.
func (m *Model) contextLine(w int) string {
	switch m.input {
	case inputSearch:
		return ansi.Truncate(" "+m.th.Label.Render("search ›")+" "+m.inputText+"█", w, "")
	case inputFilter:
		return ansi.Truncate(" "+m.th.Label.Render("filter ›")+" "+m.inputText+"█"+m.th.Muted.Render("  project= session= ref= type= code="), w, "")
	}
	if m.message != "" {
		if m.msgErr {
			return " " + m.th.BadText.Render(cut(m.message, w-2))
		}
		return " " + m.th.Muted.Render(cut(m.message, w-2))
	}
	line := m.statusText(m.opts.Source.Name)
	if ansi.StringWidth(line) > w {
		// Short of space: the base name of the file leaves room for the
		// filter.
		line = m.statusText(filepath.Base(m.opts.Source.Name))
	}
	return ansi.Truncate(line, w, "…")
}

func (m *Model) statusText(name string) string {
	segs := []string{m.th.Muted.Render(cut(name, 40)), fmt.Sprintf("%d entries", len(m.entries))}
	switch {
	case !m.verified:
		segs = append(segs, m.th.Muted.Render("chain: checking …"))
	case m.verErr != nil:
		segs = append(segs, m.th.BadText.Render("chain: CHECK FAILED"))
	case m.verify.OK:
		segs = append(segs, m.th.GoodText.Render("chain intact"))
	default:
		segs = append(segs, m.th.Alarm.Render(fmt.Sprintf(" CHAIN BROKEN at entry %d ", m.verify.FirstBadSeq)))
	}
	segs = append(segs, m.rng.Label())
	if !m.filter.empty() {
		segs = append(segs, m.th.AccentText.Render("filter: "+m.filter.String()))
	}
	if m.search != "" {
		segs = append(segs, m.th.AccentText.Render("search: "+strconv.Quote(m.search)))
	}
	return " " + strings.Join(segs, m.th.Muted.Render(" · "))
}

// footerSeparator stands between the key hints of the footer.
const footerSeparator = " • "

// shortHelp returns the key hints of the footer: those of the current
// view, then the global ones.
func (m *Model) shortHelp() []key.Binding {
	k := m.keys
	switch {
	case m.input != inputNone:
		return []key.Binding{k.Apply, k.Cancel, k.ForceQuit}
	case m.help:
		return []key.Binding{k.Back, k.Quit}
	case m.overlay != overlayNone:
		return append([]key.Binding{k.Scroll, k.Back}, k.global()...)
	case m.view == viewOverview:
		return append([]key.Binding{k.Range, k.Filter, k.Scroll}, k.global()...)
	case m.view == viewEvents:
		return append([]key.Binding{k.Detail, k.Search, k.Filter, k.Range}, k.global()...)
	}
	return append([]key.Binding{k.Timeline, k.Filter, k.Range}, k.global()...)
}

// fullHelp returns the keys of the help: those of the current view in two
// columns, then the global ones.
func (m *Model) fullHelp() [][]key.Binding {
	k := m.keys
	nav := []key.Binding{k.Up, k.Down, k.PageUp, k.PageDown, k.Top, k.Bottom}
	var view [][]key.Binding
	switch {
	case m.overlay != overlayNone:
		view = [][]key.Binding{nav}
	case m.view == viewOverview:
		view = [][]key.Binding{{k.Up, k.Down, k.PageUp, k.PageDown, k.Top}, {k.Range, k.Filter, k.Clear, k.Search}}
	case m.view == viewEvents:
		view = [][]key.Binding{nav, {k.Detail, k.Search, k.Filter, k.Clear, k.Range}}
	default:
		view = [][]key.Binding{nav, {k.Timeline, k.Filter, k.Clear, k.Range}}
	}
	return append(view, k.fullGlobal())
}

// footer renders the key hints as one line: key, space, description, the
// hints separated by footerSeparator. A hint that does not fit gives way
// to an ellipsis.
func (m *Model) footer(w int) string {
	st := m.th.help()
	limit := w - 1
	sep := st.ShortSeparator.Inline(true).Render(footerSeparator)
	var b strings.Builder
	total := 0
	for _, kb := range m.shortHelp() {
		item := st.ShortKey.Inline(true).Render(kb.Help().Key) + " " + st.ShortDesc.Inline(true).Render(kb.Help().Desc)
		if total > 0 {
			item = sep + item
		}
		iw := ansi.StringWidth(item)
		if total+iw > limit {
			if total+2 <= limit {
				b.WriteString(" " + st.Ellipsis.Inline(true).Render("…"))
			}
			break
		}
		total += iw
		b.WriteString(item)
	}
	return ansi.Truncate(" "+b.String(), w, "…")
}

// helpPanel renders the keys of the current view and the global keys in a
// panel with a rounded border in the accent colour, in the middle of the
// body. The notes follow if they fit.
func (m *Model) helpPanel(w, h int) string {
	hm := help.New()
	hm.Styles = m.th.help()
	hm.ShowAll = true
	title := viewNames[m.view]
	switch m.overlay {
	case overlayDetail:
		title = "entry"
	case overlayTimeline:
		title = "session"
	}
	keys := hm.FullHelpView(m.fullHelp())
	inner := max(min(lipgloss.Width(keys), w-4), 20)
	parts := []string{m.th.Title.Render("keys: " + title), "", keys}
	notes := []string{
		"The view only reads the audit log. It never writes and never talks to the daemon.",
		"The filter for project and session also selects the figures. The filter for ref, type and code and the search select only the events.",
		"Session name, agent, task and reason are claims of the agent and not verified.",
		"The how-to \"Freigaben auswerten\" explains each figure.",
	}
	var noteLines []string
	for _, n := range notes {
		for _, l := range wrap(n, inner, 3) {
			noteLines = append(noteLines, m.th.Muted.Render(l))
		}
	}
	closing := m.th.Muted.Render("esc or ? closes this help")
	// The borders, the title, three blank lines and the closing line take
	// 7 rows. The notes come only if they fit, else only the last one.
	switch fixed := 7 + lipgloss.Height(keys); {
	case fixed+len(noteLines) <= h:
		parts = append(parts, "")
		parts = append(parts, noteLines...)
	case fixed+1 <= h:
		parts = append(parts, "", m.th.Muted.Render(cut(notes[len(notes)-1], inner)))
	}
	parts = append(parts, "", closing)
	box := m.th.Border.Border(lipgloss.RoundedBorder()).Padding(0, 1).Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
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
	st, th := m.stats, m.th
	// row is one figure: the label, the number and a note. A main figure
	// has a bold label; a state has its colour on the number.
	row := func(label string, main bool, n int, style lipgloss.Style, note string) string {
		num := padLeft(strconv.Itoa(n), 5)
		if n > 0 {
			num = style.Render(num)
		}
		l := pad(label, 13)
		if main {
			l = th.Strong.Render(l)
		}
		return " " + l + num + "   " + th.Muted.Render(cut(note, max(w-23, 10)))
	}
	var lines []string
	if m.verified && m.verErr == nil && !m.verify.OK {
		lines = append(lines, th.Alarm.Render(" "+cut(fmt.Sprintf("The hash chain is broken at entry %d: %s.", m.verify.FirstBadSeq, m.verify.Finding), w-2)+" "),
			th.BadText.Render(" "+cut("The figures may be wrong. a38 audit verify shows the same finding.", w-2)))
	}
	lines = append(lines, " "+th.Label.Render("Range")+" "+m.rangeText()+th.Muted.Render("  (t changes it)"))
	lines = append(lines, m.accountLines(w)...)
	if st.Entries == 0 {
		lines = append(lines, "", th.Muted.Render(" No entries in this range."), th.Muted.Render(" Press t for a longer range, or c to clear the filter."))
		return lines
	}
	errs := make([]string, 0, len(st.ErrorCodes))
	for _, c := range st.ErrorCodes {
		errs = append(errs, fmt.Sprintf("%s %d", dash(c.Name), c.Count))
	}
	lines = append(lines,
		row("POPUPS", true, st.Popups, th.Figure, fmt.Sprintf("%.1f per day, busiest day %s with %d", st.PopupsPerDay, dash(st.BusiestDay.Date), st.BusiestDay.Popups)),
		row(" approved", false, st.Approved, th.GoodText, fmt.Sprintf("%s (partial %d)", percent(st.Approved, st.Popups), st.Partial)),
		row(" denied", false, st.Denied, th.BadText, percent(st.Denied, st.Popups)+" denial share"),
		row(" timed out", false, st.TimedOut, th.CautionText, fmt.Sprintf("%s (UI failed %d, session ended %d)", percent(st.TimedOut, st.Popups), st.Failed, st.Abandoned)),
		row(" deviations", false, st.Deviations, th.CautionText, percent(st.Deviations, st.Popups)+" went beyond a live grant"),
		" "+pad(" size", 13)+padLeft(formatMedian(st.BundleMedian), 5)+"   "+th.Muted.Render(cut("median entries per popup, largest "+strconv.Itoa(st.BundleMax), max(w-23, 10))),
		row("DELIVERIES", true, st.Deliveries, th.Figure, fmt.Sprintf("%s under a grant without popup (%d), %d after a popup",
			percent(st.DeliveriesUnderGrant, st.Deliveries), st.DeliveriesUnderGrant, st.DeliveriesAfterPopup)),
		row("DENIED EARLY", true, st.PolicyDenials, th.Figure, "by the policy, an unknown item or a full bundle"),
		" "+cut(fmt.Sprintf("REVOCATIONS %d · ERRORS %d · LOCKS %d (ended %d)", st.Revocations, st.Errors, st.LockBegins, st.LockEnds), w-2),
		"",
		th.Title.Render(fmt.Sprintf(" POPUPS PER DAY (largest %d)", st.BusiestDay.Popups)),
	)
	lines = append(lines, m.chart(st.Days, w, 5)...)
	lines = append(lines, "")
	if len(errs) > 0 {
		lines = append(lines, " "+th.Title.Render("ERRORS BY CODE")+"  "+cut(strings.Join(errs, ", "), max(w-18, 10)), "")
	}
	lines = append(lines, m.rankTables(st, w)...)
	return lines
}

// accountLines renders the state of each 1Password account from the
// status of the daemon (D78): one line with all accounts and one line for
// each account that does not work. Each text from the daemon is cleaned.
func (m *Model) accountLines(w int) []string {
	if m.opts.Source.Accounts == nil || !m.accLoaded {
		return nil
	}
	th := m.th
	head := " " + th.Label.Render("1PASSWORD") + " "
	if m.accErr != nil {
		return []string{head + th.Muted.Render(cut("the daemon does not answer: "+auditstats.Clean(m.accErr.Error()), max(w-12, 10)))}
	}
	if len(m.accStates) == 0 {
		// Another backend or an old daemon: no line.
		return nil
	}
	var parts, notes []string
	for _, a := range m.accStates {
		name := auditstats.Clean(a.Name)
		switch a.State {
		case "ok":
			parts = append(parts, th.GoodText.Render(fmt.Sprintf("%s ok (%d of %d vaults)", name, a.Visible, a.Configured)))
			continue
		case "locked":
			parts = append(parts, th.BadText.Render(name+" LOCKED"))
		default:
			parts = append(parts, th.CautionText.Render(name+" UNREACHABLE"))
		}
		notes = append(notes, th.Muted.Render("   "+cut(name+": "+auditstats.Clean(a.Reason), max(w-4, 10))))
	}
	return append([]string{ansi.Truncate(head+strings.Join(parts, " · "), max(w-1, 10), "…")}, notes...)
}

func formatMedian(v float64) string {
	if v == float64(int(v)) {
		return strconv.Itoa(int(v))
	}
	return strconv.FormatFloat(v, 'f', 1, 64)
}

// rankTables renders the busiest sessions and projects side by side and
// the most requested credentials below them.
func (m *Model) rankTables(st auditstats.Stats, w int) []string {
	th := m.th
	half := max((w-3)/2, 30)
	table := func(title string, ranks []auditstats.Rank, width int) []string {
		nameW := max(width-16, 8)
		out := []string{th.Label.Render(" " + pad(title, nameW) + padLeft("REQ", 5) + padLeft("POP", 5) + padLeft("DEL", 5))}
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
			lines = append(lines, l+strings.Repeat(" ", max(half-ansi.StringWidth(l), 0))+th.Rule.Render(" │ ")+r)
		}
	} else {
		lines = append(append(lines, sess...), proj...)
	}
	lines = append(lines, "")
	nameW := max(w-19, 20)
	lines = append(lines, th.Label.Render(" "+pad("MOST REQUESTED CREDENTIALS", nameW)+padLeft("REQ", 5)+padLeft("DEL", 5)+padLeft("DENY", 6)))
	for _, r := range st.TopReferences {
		lines = append(lines, "  "+pad(r.Name, nameW-1)+padLeft(strconv.Itoa(r.Requests), 5)+padLeft(strconv.Itoa(r.Deliveries), 5)+padLeft(strconv.Itoa(r.Denials), 6))
	}
	return lines
}

// blocks are the eighths of a cell for the bars.
var blocks = []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

// chart renders one bar per day (or per group of days) with h rows.
func (m *Model) chart(days []auditstats.Day, w, h int) []string {
	if len(days) == 0 {
		return []string{m.th.Muted.Render("  (no days)")}
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
		prefix := "     " + m.th.Rule.Render("│")
		if r == h-1 {
			prefix = fmt.Sprintf(" %3d ", top) + m.th.Rule.Render("┤")
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
		lines = append(lines, prefix+m.th.Bar.Render(b.String()))
	}
	n := len(vals) * colW
	lines = append(lines, "   0 "+m.th.Rule.Render("└"+strings.Repeat("─", n)))
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
	lines = append(lines, m.th.Muted.Render(strings.TrimRight(string(axis), " ")))
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
func (m *Model) resultStyle(e audit.Entry) lipgloss.Style {
	switch {
	case e.Result.Code == protocol.CodeTimeout:
		return m.th.CautionText
	case e.Result.Code != "", e.Type == audit.EventDenial:
		return m.th.BadText
	case e.Type == audit.EventDelivery:
		return m.th.GoodText
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
	lines := []string{m.title(title, n == 0), m.th.Label.Render(head)}
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
			lines = append(lines, m.selected(strings.Join(cells, " ")+" "+result+ref, w))
			continue
		}
		lines = append(lines, " "+strings.Join(cells, " ")+" "+m.resultStyle(e).Render(result)+ref)
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
		out := []string{"  " + m.th.Label.Render(pad(label, 13)) + ls[0]}
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
	add(m.th.Title.Render(fmt.Sprintf(" ENTRY %d", e.Seq)) + m.th.Muted.Render("  (esc back)"))
	when := local.Format("2006-01-02 15:04:05 MST")
	if _, off := local.Zone(); off != 0 {
		when += " (" + e.Time.UTC().Format("15:04:05") + " UTC)"
	}
	add(field("Time", when)...)
	add(field("Type", string(e.Type))...)
	add("  " + m.th.Label.Render(pad("Result", 13)) + m.resultStyle(e).Render(cut(resultText(e), vw)))
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
		add("  " + m.th.Label.Render(pad(label, 13)) + cut(r.String(), vw))
	}
	add("", m.th.Title.Render(" VERIFIED BY THE DAEMON"))
	add(field("Session ID", dash(string(e.Verified.SessionID)))...)
	add(field("Project", dash(e.Verified.Project))...)
	origin := dash(string(e.Verified.Origin))
	if e.Verified.SessionID != "" {
		origin += fmt.Sprintf(", user %d, process %d", e.Verified.PeerUID, e.Verified.PeerPID)
	}
	add(field("Origin", origin)...)
	add(field("Program", dash(e.Verified.Process))...)
	add(field("Directory", dash(e.Verified.Cwd))...)
	add("", m.th.Title.Render(" CLAIMS OF THE AGENT (NOT VERIFIED)"))
	add(field("Session name", dash(e.Claims.SessionName))...)
	add(field("Agent", dash(e.Claims.Agent))...)
	add(field("Task", dash(e.Claims.Task))...)
	add(field("Reason", dash(e.Claims.Reason))...)
	add("", m.th.Title.Render(" HASH CHAIN"))
	add(field("Position", fmt.Sprintf("entry %d of %d", e.Seq, last))...)
	chain := m.chainText(e.Seq)
	if m.verified && m.verErr == nil && !m.verify.OK && e.Seq >= m.verify.FirstBadSeq {
		add("  " + m.th.Label.Render(pad("Chain", 13)) + m.th.BadText.Render(cut(chain, vw)))
	} else {
		add(field("Chain", chain)...)
	}
	add("  " + m.th.Label.Render(pad("Prev hash", 13)) + cut(e.PrevHash, vw))
	add("  " + m.th.Label.Render(pad("Hash", 13)) + cut(e.Hash, vw))
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
	lines := []string{m.title(title, n == 0), m.th.Label.Render(head)}
	for i := m.sesTop; i < min(m.sesTop+rows, n); i++ {
		s := m.sessions[i]
		line := " " + pad(dash(s.Name), nameW) + " " + pad(dash(s.Project), projW) + " " + pad(s.Last.In(m.opts.Location).Format("01-02 15:04"), 11)
		if idW > 0 {
			line += " " + pad(s.ID, idW)
		}
		line += num(strconv.Itoa(s.Requests)) + num(strconv.Itoa(s.Popups)) + num(strconv.Itoa(s.Grants)) + num(strconv.Itoa(s.Deliveries)) + num(strconv.Itoa(s.Denials))
		if i == m.sesCursor {
			line = m.selected(line[1:], w)
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
	add(m.th.Title.Render(" SESSION "+cut(dash(s.Name), max(w-30, 10))) + m.th.Muted.Render(" (claimed name, esc back)"))
	add(" " + cut(fmt.Sprintf("ID %s · project %s · agent %s (claimed)", s.ID, dash(s.Project), dash(s.Agent)), w-2))
	add(" " + cut(fmt.Sprintf("%s to %s · requests %d · popups %d · grants %d · deliveries %d · denials %d",
		s.First.In(loc).Format("2006-01-02 15:04"), s.Last.In(loc).Format("15:04"), s.Requests, s.Popups, s.Grants, s.Deliveries, s.Denials), w-2))
	add("", m.th.Title.Render(" GRANTS"))
	grants := auditstats.Grants(m.entries, s.ID)
	if len(grants) == 0 {
		add(m.th.Muted.Render("  none"))
	}
	for _, g := range grants {
		end := "no end event"
		if !g.Ended.IsZero() {
			end = "ended " + g.Ended.In(loc).Format("15:04") + " " + string(g.End)
		}
		add("  " + cut(fmt.Sprintf("%s  %s for %s  %s  %d deliveries  %d credentials",
			g.ID, g.Granted.In(loc).Format("15:04"), shortDuration(g.Lifetime), end, g.Deliveries, len(g.References)), w-3))
	}
	add("", m.th.Title.Render(" TIMELINE"))
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
			m.resultStyle(e).Render(pad(resultText(e), resW)) + " " + cut(extra, refW))
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

// title renders the title of a list; the title of an empty list is muted.
func (m *Model) title(s string, empty bool) string {
	if empty {
		return m.th.Muted.Render(s)
	}
	return m.th.Title.Render(s)
}

// selected renders the selected row of a list: the mark ▸ and the row on
// the selection background, filled to the width. The row is plain text, so
// that the background runs through.
func (m *Model) selected(row string, w int) string {
	line := "▸" + ansi.Strip(row)
	line += strings.Repeat(" ", max(w-ansi.StringWidth(line), 0))
	return m.th.Mark.Inherit(m.th.Selected).Render("▸") + m.th.Selected.Render(line[len("▸"):])
}
