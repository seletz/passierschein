---
render_macros: true
id: 'D57'
titel: 'a38 tui läuft nur auf macOS, nutzt Bubble Tea und leitet die Kennzahlen aus den Ereignissen ab'
bereich: 'Audit'
lauf: '2.0'
stand: '2026-11-13'
order: 4
headless: true
status: gueltig
use_cases: [UC6]
beleg:
  notiz: Vorschlag aus der Entwicklung, zu bestätigen durch Stefan; Stil nach dem Review in PR 76 angepasst
  annahme: true
---
# D57 — a38 tui läuft nur auf macOS, nutzt Bubble Tea und leitet die Kennzahlen aus den Ereignissen ab

{{ design_kopf(page) }}

**Entscheidung:** Drei Punkte gelten zusammen.

1. **Plattform:** `a38 tui` gibt es nur im macOS-Build von `a38`. Der linux-Build meldet „not available on this platform“ und enthält weder `internal/tui` noch die Bibliotheken von Charm. `mise run deps:linux:check` prüft das. `a38 audit list` und `a38 audit stats` gibt es auf jeder Plattform.
2. **Bibliothek:** Die Oberfläche nutzt Bubble Tea v2 (`charm.land/bubbletea/v2` v2.1.1), Lip Gloss v2 (`charm.land/lipgloss/v2` v2.0.6) und aus Bubbles v2 (`charm.land/bubbles/v2` v2.2.1) die Pakete `key` und `help`, alle mit festem Stand in `go.mod`. Stil und Tasten folgen Stefans anderen Oberflächen mit Bubble Tea: ein Theme mit festen Farben für hellen und dunklen Hintergrund, das dem Hintergrund des Terminals folgt. Jeder Zustand hat zusätzlich ein Wort, Zeichen oder Schriftschnitt. `NO_COLOR` schaltet die Farben ab.
3. **Kennzahlen:** Das Paket `internal/auditstats` leitet die Kennzahlen aus den Ereignissen von {{ ref("D36") }} ab. Das Audit-Log hat kein Feld für ein Popup oder eine Abweichung.
    - Ein **Popup** ist ein Ereignis „Bündel“ mit Kennung der Freigabeanfrage, dem eine Entscheidung, eine Ablehnung mit `denied`, ein Ablauf mit `timeout` oder ein Fehler mit derselben Kennung folgt. Ein Bündel ohne ein solches Ereignis hatte kein Popup: Ein Grant deckte jeden Eintrag ab.
    - Eine **Teilfreigabe** ist ein Popup mit Entscheidung und Ablehnung. Sie zählt als Freigabe.
    - Der **Anteil der Ablehnungen** ist die Zahl der abgelehnten Popups geteilt durch die Zahl der Popups.
    - Eine **Abweichung** ist ein Popup einer Sitzung, die zur Zeit des Bündels einen lebenden Grant hatte. Ein Grant lebt von seinem Ereignis bis zum Ende seiner Laufzeit oder bis zu einem Ablauf oder Widerruf mit seiner Kennung.
    - Die **Größe** eines Popups ist die Zahl der Referenzen in seinen Ereignissen des Ausgangs, also der offenen Einträge, die der Mensch sah.
    - Eine **Auslieferung nach Popup** hat einen Grant, dessen Ereignis nach der letzten Anfrage der Sitzung vor der Auslieferung steht. Jede andere Auslieferung lief **unter einem Grant ohne Popup**.
    - Ein Tag ist ein Kalendertag in der lokalen Zeit. Der Zeitraum beim Start ist 7 Tage.

**Begründung:** Das Audit-Log liegt beim Daemon auf dem Mac ({{ ref("D53") }}). In der VM des Agents gibt es kein Audit-Log, das die Ansicht lesen könnte; dort wäre die Oberfläche nur zusätzlicher Code im Client. Stefan nutzt Bubble Tea in anderen Projekten; v2 ist die gepflegte Hauptversion. Die Definitionen brauchen keine Änderung am Audit-Log und gelten auch für alte Einträge. Ob die Zahlen {{ ref("G12") }} beantworten, entscheidet Stefan.

**Quelle:** Entwicklung von {{ ref("F13") }}, Issue 71.
