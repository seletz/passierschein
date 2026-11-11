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

- Woher liest `a38 mcp` den Namen: aus einer Umgebungsvariable, aus einer Datei des Agent-Werkzeugs oder aus einem Argument beim Start?
- Was zeigt die Freigabeanfrage, wenn die Sitzung keinen Namen hat oder der Mensch sie später umbenennt?
- Kann der Daemon den Namen prüfen, zum Beispiel über die Sitzungsdaten des Agent-Werkzeugs zur Prozess-ID? Ohne Prüfung steht der Name bei den Angaben des Agents ({{ ref("D4") }}).
- Wie heißt das Gegenstück bei anderen Agent-Werkzeugen?
- Ein gekaperter Agent kann den Namen einer anderen Sitzung nennen ({{ ref("RI2") }}). Wie macht die Freigabeanfrage das erkennbar?

**Quelle:** Rückmeldung von Stefan zum Design, 2026-11-11.
