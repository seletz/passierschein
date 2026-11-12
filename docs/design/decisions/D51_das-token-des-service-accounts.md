---
render_macros: true
id: 'D51'
titel: 'Das Token des Service Accounts liegt in einer Datei mit Modus 0600'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-12'
order: 5
headless: true
status: gueltig
use_cases: [UC11]
beleg:
  notiz: Vorschlag aus der Entwicklung, zu bestätigen durch Stefan
  annahme: true
---
# D51 — Das Token des Service Accounts liegt in einer Datei mit Modus 0600

{{ design_kopf(page) }}

**Entscheidung:** Die Konfiguration nennt die Token-Datei des Service Accounts im Schlüssel `backend.onepassword.token_file`. Der Standard ist `$XDG_CONFIG_HOME/a38/onepassword-token`, ohne die Variable `~/.config/a38/onepassword-token`. Die Datei ist eine reguläre Datei, gehört dem Benutzer des Daemons und hat genau den Modus `0600`. Ein symbolischer Link, ein Verzeichnis, ein anderer Besitzer, ein anderer Modus oder eine leere Datei verhindern den Start mit dem 1Password-Backend. Die Meldung nennt den Befund und nie den Inhalt. Der Daemon liest die Datei einmal beim Start. Er hält das Token nur im Arbeitsspeicher, in einem Typ mit der Ausgabe `[redacted]`. Das Token steht nie im Log, in einer Fehlermeldung, im Audit-Log oder im Status. Es gibt keine Umgebungsvariable für das Token. `a38 doctor` prüft Existenz, Besitzer und Modus der Datei und liest den Inhalt nicht.

**Begründung:** Die Datei ist einfach und braucht kein CGO. Der Zugriff auf den Schlüsselbund ohne CGO liefe über `/usr/bin/security`; die Zugriffsliste nennt dann `security` und nicht `a38` (Spike 10). Ein Kindprozess erbt die Umgebung, auch ein Agent; eine Umgebungsvariable gäbe das Token weiter. Eine Prüfung von Besitzer und Modus fängt eine falsch angelegte Datei vor dem ersten Start ab.

**Grenze:** Jeder Prozess des Benutzers kann die Datei lesen. Die Datei kann in eine Datensicherung gelangen. Der Schutz beruht auf den Rechten des Service Accounts: nur Lesen, nur die freigegebenen Vaults ({{ ref("D39") }}).

**Offen:** Ob der Schlüsselbund die Datei später ersetzt, entscheidet Stefan. Die Entscheidung engt {{ ref("G3") }} ein und schließt die Frage nach dem Ort des Tokens als Arbeitsannahme.

**Quelle:** Entwicklung der Stufe 3, [Issue 50](https://github.com/seletz/passierschein/issues/50). Vorschlag aus der Entwicklung, zu bestätigen durch Stefan.
