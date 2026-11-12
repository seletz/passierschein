---
render_macros: true
id: 'D51'
titel: 'Das Token liegt künftig im Schlüsselbund, bis dahin in einer Datei mit Modus 0600'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-12'
order: 5
headless: true
status: gueltig
use_cases: [UC11]
beleg:
  notiz: Ziel Schlüsselbund von Stefan am 2026-11-12 festgelegt (Spike Issue 53); Token-Datei als Übergang ist ein Vorschlag aus der Entwicklung, zu bestätigen durch Stefan
  annahme: true
---
# D51 — Das Token liegt künftig im Schlüsselbund, bis dahin in einer Datei mit Modus 0600

{{ design_kopf(page) }}

**Entscheidung:**

1. **Ziel:** Das Token des Service Accounts liegt im Schlüsselbund von macOS, gebunden an das signierte Programm `a38`. Der Mensch fügt das Token im Terminal in einen Befehl von `a38` ein; `a38` legt es im Schlüsselbund ab. Name und Einzelheiten des Befehls klärt der [Spike in Issue 53](https://github.com/seletz/passierschein/issues/53). Stefan hat das Ziel am 2026-11-12 festgelegt.
2. **Übergang:** Bis dahin ist eine Token-Datei die einzige umgesetzte Quelle, für die Entwicklung und für Tests. Die Konfiguration nennt sie im Schlüssel `backend.onepassword.token_file`. Der Standard ist `$XDG_CONFIG_HOME/a38/onepassword-token`, ohne die Variable `~/.config/a38/onepassword-token`.
3. Die Datei ist eine reguläre Datei, gehört dem Benutzer des Daemons und hat genau den Modus `0600`. Ein symbolischer Link, ein Verzeichnis, ein anderer Besitzer, ein anderer Modus oder eine leere Datei verhindern den Start mit dem 1Password-Backend. Die Meldung nennt den Befund und nie den Inhalt.
4. Der Daemon liest das Token einmal beim Start, über die Schnittstelle `token.Source`. Eine Quelle im Schlüsselbund ersetzt nur den Konstruktor der Quelle; das Backend bleibt unverändert.
5. Der Daemon hält das Token nur im Arbeitsspeicher, in einem Typ mit der Ausgabe `[redacted]`. Das Token steht nie im Log, in einer Fehlermeldung, im Audit-Log oder im Status.
6. Es gibt keine Umgebungsvariable für das Token.
7. `a38 doctor` prüft Existenz, Besitzer und Modus der Datei und liest den Inhalt nicht.

**Schwäche der Datei:** Jeder Prozess desselben Benutzers kann die Datei lesen, auch ein Agent, der auf dem Mac läuft. Der Modus `0600` schützt nur vor anderen Benutzern. Wer die Datei liest, liest alle Items der freigegebenen Vaults, bis der Mensch das Token widerruft. Die Datei kann in eine Datensicherung gelangen. Das ist ein Fall von {{ ref("RI5") }} und der Grund für das Ziel Schlüsselbund ({{ ref("G3") }}). Die Rechte des Service Accounts (nur Lesen, nur die freigegebenen Vaults, {{ ref("D39") }}) begrenzen den Schaden.

**Begründung:** Die Datei ist einfach, braucht kein CGO und reicht für Entwicklung und Test. Ein Eintrag im Schlüsselbund, gebunden an das signierte Programm, ist für einen anderen Prozess desselben Benutzers nicht ohne Rückfrage lesbar. Ein Kindprozess erbt die Umgebung, auch ein Agent; eine Umgebungsvariable gäbe das Token weiter. Eine Prüfung von Besitzer und Modus fängt eine falsch angelegte Datei vor dem ersten Start ab.

**Offen:** Wie `a38` ohne CGO den Schlüsselbund nutzt und wie die Bindung an das signierte Programm aussieht, klärt Issue 53. Die Entscheidung engt {{ ref("G3") }} ein.

**Quelle:** Entwicklung der Stufe 3, [Issue 50](https://github.com/seletz/passierschein/issues/50). Ziel Schlüsselbund: Stefan, 2026-11-12. Die Token-Datei als Übergang ist ein Vorschlag aus der Entwicklung, zu bestätigen durch Stefan.
