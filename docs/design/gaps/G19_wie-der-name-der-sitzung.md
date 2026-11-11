---
render_macros: true
id: 'G19'
titel: 'Wie der Name der Sitzung zu a38 kommt und ob der Daemon ihn prüfen kann, ist offen'
bereich: 'Grenzen und Identität'
lauf: '1.0'
stand: '2026-11-11'
order: 5
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G19 — Wie der Name der Sitzung zu a38 kommt und ob der Daemon ihn prüfen kann, ist offen

{{ design_kopf(page) }}

{{ ref("D25") }} verlangt den Namen der Sitzung aus dem Agent-Werkzeug in der Freigabeanfrage. Der Mensch ordnet eine Anfrage über diesen Namen zu.

Offene Fragen:

- Festgelegt (Stefan, 2026-11-11): Die Anmeldung des MCP-Servers legt den Namen je Sitzung fest. Offen ist die Form: Argument oder Umgebungsvariable von `a38 mcp`. Offen ist auch, wer den Wert bei jeder neuen Sitzung setzt, ohne dass der Mensch ihn von Hand einträgt.
- Was zeigt die Freigabeanfrage, wenn die Sitzung keinen Namen hat oder der Mensch sie später umbenennt?
- Kann der Daemon den Namen prüfen, zum Beispiel über die Sitzungsdaten des Agent-Werkzeugs zur Prozess-ID? Ohne Prüfung steht der Name bei den Angaben des Agents ({{ ref("D4") }}).
- Wie heißt das Gegenstück bei anderen Agent-Werkzeugen?
- Entschieden (Stefan, 2026-11-11): Zwei Sitzungen mit demselben Namen sind ein Fehler; der Daemon lehnt beide ab ({{ ref("D25") }}). Offen bleiben die Längengrenze und die erlaubten Zeichen des Namens.

**Quelle:** Rückmeldung von Stefan zum Design, 2026-11-11.
