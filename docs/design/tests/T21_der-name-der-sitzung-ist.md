---
render_macros: true
id: 'T21'
titel: 'Der Name der Sitzung ist ein Startwert von a38 mcp und steht nur bei den Angaben'
bereich: 'Grenzen und Identität'
lauf: '2.0'
stand: '2026-11-12'
order: 4
headless: true
status: aktiv
umsetzung: teilweise
beweist: [D25, D4, D36]
---
# T21 — Der Name der Sitzung ist ein Startwert von a38 mcp und steht nur bei den Angaben

{{ design_kopf(page) }}

**Form:** Tabellengetriebener Go-Test mit `--ui script`. Das Skript speichert die Freigabeanfrage.

**Fälle und Erwartung:**

- `a38 mcp` startet mit dem Namen `beispiel-koordinator-7f3a`. Die Freigabeanfrage enthält den Namen genau einmal, im Abschnitt der Angaben. Der Abschnitt der geprüften Fakten enthält den Namen nicht und nennt die Kennung des Daemons.
- Die Einträge des Audit-Logs für Anfrage, Bündel, Entscheidung, Grant und Auslieferung tragen den Namen im Feld der Angaben, nie im Feld der geprüften Fakten. `a38 audit list` zeigt den Namen.
- Kein Name gesetzt: Die Verbindung arbeitet. Die Freigabeanfrage zeigt eine ausdrückliche Leermarke und keinen erfundenen Wert.
- Ein Name mit 80 Zeichen aus Buchstaben, Ziffern, Leerzeichen, Bindestrich, Unterstrich, runden Klammern und Doppelpunkt gilt, zum Beispiel `beispiel: koordinator (2)`. Die Freigabeanfrage zeigt ihn ungekürzt.
- Ein Name mit 81 Zeichen: `invalid_request` beim Aufbau der Verbindung. Der Daemon bedient andere Sitzungen weiter.
- Ein Name mit einem anderen Zeichen (Punkt, Umlaut, Schrägstrich): `invalid_request`.
- Ein Name mit einem Steuerzeichen (Zeilenumbruch, Escape-Folge, NUL, Zeichen für die Schreibrichtung): `invalid_request`, auch wenn die Konfiguration das Zeichen erlaubt. Weder die Freigabeanfrage noch `a38 audit list` enthält ein rohes Steuerzeichen.
- Die Konfiguration setzt die Höchstlänge auf 20 und erlaubt zusätzlich den Punkt: Ein Name mit 21 Zeichen ist ungültig, ein Name mit Punkt gilt.
- Eine Höchstlänge von 0 oder eine leere Zeichenmenge in der Konfiguration: Der Daemon startet nicht.
- Zwei Verbindungen mit demselben Namen: Jede Anfrage beider Sitzungen endet mit `session_name_conflict`. Es entsteht keine Freigabeanfrage. Ein bestehender Grant der ersten Sitzung liefert nicht mehr aus, solange beide verbunden sind. Nach dem Ende einer der beiden Verbindungen arbeitet die andere wieder.
- Der Name trägt keinen Grant: Nach dem Ende der Verbindung braucht eine neue Verbindung mit demselben Namen eine neue Freigabe.
- Der Name steht je Verbindung fest: Eine zweite Anmeldung oder ein Feld für den Namen in `request_credentials` endet mit `invalid_request`.

**Offen:** Die Form des Startwerts ({{ ref("G19") }}).


Belegt: {{ ref("D25") }}, {{ ref("D4") }}, {{ ref("D36") }}.
