---
render_macros: true
id: 'G20'
titel: 'Form und Aktionen der Meldung bei einem Fehler des Audit-Logs sind offen'
bereich: 'Audit'
lauf: '1.0'
stand: '2026-11-11'
order: 3
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G20 — Form und Aktionen der Meldung bei einem Fehler des Audit-Logs sind offen

{{ design_kopf(page) }}

{{ ref("D36") }} verlangt ein Popup, wenn das Schreiben in das Audit-Log fehlschlägt. Der Mensch soll sofort handeln können. Dieselbe Art von Meldung nennt den Grund, wenn die Konfiguration eines Projekts fehlerhaft ist ({{ ref("D28") }}).

Festgelegt (Stefan, 2026-11-11): Das Popup informiert nur und bietet keine Aktionen. Der Mensch beseitigt die Ursache selbst. Die Fehlermeldungen der Kommandozeile und der Befehl `a38 doctor` helfen ihm dabei ({{ ref("F20") }}).

Offene Fragen:

- Zeigt die Freigabe-App die Meldung, oder nutzt der Daemon die Mitteilungen von macOS? Die Freigabe-App braucht dann eine zweite Art von Nachricht.
- Wie oft meldet der Daemon denselben Fehler, damit keine Flut von Popups entsteht?
- Wie erfährt der Mensch den Fehler, wenn er nicht am Mac sitzt?
- Entschieden: Die Auslieferung bleibt global gesperrt, solange der Daemon nicht fehlerfrei läuft ({{ ref("D48") }}). Offen ist, woran der Daemon das Ende der Störung erkennt.

**Quelle:** Rückmeldung von Stefan zum Design, 2026-11-11.
