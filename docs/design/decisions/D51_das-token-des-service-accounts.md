---
render_macros: true
id: 'D51'
titel: 'Das Token liegt im Schlüsselbund, eine Datei mit Modus 0600 ist der Ausweichweg'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-16'
order: 5
headless: true
status: ersetzt
ersetzt_durch: D72
use_cases: [UC11]
beleg:
  wer: Stefan
  datum: '2026-11-16'
  quelle: ['#88', '#61']
  notiz: Von Stefan am 2026-11-12 in PR 61 abgenommen. Ersetzt durch D72 (Token je Konto, Issue 88); die Ersetzung hat Stefan am 2026-11-16 in PR 114 bestätigt.
---
# D51 — Das Token liegt im Schlüsselbund, eine Datei mit Modus 0600 ist der Ausweichweg

{{ design_kopf(page) }}

**Entscheidung:**

1. **Ziel und Standard:** Das Token des Service Accounts liegt im Anmelde-Schlüsselbund von macOS, im Eintrag mit dem Dienst `a38.onepassword` und dem Konto `service-account`. Der Mensch fügt das Token im Terminal in `a38 config onepassword-token set` ein; `a38` legt den Eintrag selbst über Security.framework an, nie über `/usr/bin/security`. Der Schlüsselbund vertraut dann nur dem Build von `a38`, der den Eintrag angelegt hat. Stefan hat das Ziel am 2026-11-12 festgelegt und den Bau freigegeben. Umgesetzt in [Issue 59](https://github.com/seletz/passierschein/issues/59), nach dem [Spike in Issue 53](https://github.com/seletz/passierschein/issues/53).
2. **Quelle per Konfiguration:** Der Schlüssel `backend.onepassword.token_source` wählt die Quelle: `keychain` (Standard) oder `file`. Ein anderer Wert verhindert das Laden der Konfiguration.
3. **Ausweichweg:** Die Token-Datei bleibt für Entwicklung und CI. Die Konfiguration nennt sie im Schlüssel `backend.onepassword.token_file`. Der Standard ist `$XDG_CONFIG_HOME/a38/onepassword-token`, ohne die Variable `~/.config/a38/onepassword-token`. Die Datei ist eine reguläre Datei, gehört dem Benutzer des Daemons und hat genau den Modus `0600`. Ein symbolischer Link, ein Verzeichnis, ein anderer Besitzer, ein anderer Modus oder eine leere Datei verhindern den Start mit dem 1Password-Backend.
4. **Befehle:** Die Gruppe `a38 config` enthält `a38 config onepassword-token set | status | delete`. `set` liest das Token nur vom Terminal, ohne Echo; ist die Standardeingabe kein Terminal, liest der Befehl nichts und endet mit dem Exit-Code 2. Das Token ist nie ein Argument, nie eine Umgebungsvariable und kommt nie aus einer Datei oder einer Pipe. Ein vorhandener Eintrag wird nur mit `--replace` ersetzt. `status` liest das Token nie und zeigt deshalb nie einen Dialog. `delete` löscht den Eintrag.
5. **Nur Metadaten:** Kein Befehl zeigt das Token oder einen daraus abgeleiteten Wert (Länge, Prüfsumme, Fingerabdruck). `status` und `a38 doctor` zeigen nur, was der Schlüsselbund ohne den Wert liefert: Existenz, Datum der Anlage und der Änderung, vertraute Programme, Partitionsliste; dazu die Signatur von `a38`. Die Token-Datei prüfen sie nur über Besitzer und Rechte.
6. Der Daemon liest das Token einmal beim Start, über die Schnittstelle `token.Source`, aus genau einer Quelle. Fehlt der Eintrag im Schlüsselbund, startet der Daemon mit dem 1Password-Backend nicht; die Meldung nennt `a38 config onepassword-token set`. Die Meldung nennt nie das Token.
7. Der Daemon hält das Token nur im Arbeitsspeicher, in einem Typ mit der Ausgabe `[redacted]`. Das Token steht nie im Log, in einer Fehlermeldung, im Audit-Log oder im Status.
8. Es gibt keine Umgebungsvariable für das Token.
9. `a38 doctor` prüft die Quelle, ohne das Token zu lesen: den Eintrag und seine Zugriffsliste, die Signatur von `a38` als Hinweis, eine übrige Token-Datei als Befund; bei der Quelle `file` Existenz, Besitzer und Modus der Datei.

**Grenze bis zur Signatur ({{ ref("G5") }}):** Ohne Developer ID bindet der Schlüsselbund den Eintrag an den Hash des Builds. Jeder neue Build von `a38` braucht eine Bestätigung in einem Dialog von macOS. Ein anderes Programm kann denselben Dialog auslösen; der Mensch kann beide nicht unterscheiden. Der Schutz gegen einen Prozess desselben Benutzers beginnt erst mit der Signatur mit Developer ID und Hardened Runtime. Bis dahin bleibt {{ ref("G3") }} im Teil der Signatur offen, und {{ ref("RI5") }} bleibt bestehen.

**Schwäche der Datei:** Jeder Prozess desselben Benutzers kann die Datei lesen, auch ein Agent, der auf dem Mac läuft. Der Modus `0600` schützt nur vor anderen Benutzern. Wer die Datei liest, liest alle Items der freigegebenen Vaults, bis der Mensch das Token widerruft. Die Datei kann in eine Datensicherung gelangen. Das ist ein Fall von {{ ref("RI5") }}. Die Rechte des Service Accounts (nur Lesen, nur die freigegebenen Vaults, {{ ref("D39") }}) begrenzen den Schaden.

**Begründung:** Ein Eintrag im Schlüsselbund, den `a38` selbst anlegt, ist für ein anderes Programm desselben Benutzers nicht still lesbar; das hat der Spike gemessen. Der Weg über `purego` braucht kein CGO. Die Datei ist einfach und reicht für Entwicklung und CI, wo jeder Neubau sonst einen Dialog auslöste. Ein Kindprozess erbt die Umgebung, auch ein Agent; eine Umgebungsvariable gäbe das Token weiter.

**Bestätigt:** Stefan hat am 2026-11-12 die Umsetzung abgenommen (PR 61): die Quelle `keychain` als Standard, das Lesen des Tokens nur vom Terminal, nur Metadaten in jeder Ausgabe, den Befehl `a38 config onepassword-token`, den Eintrag `a38.onepassword` mit dem Konto `service-account` und das Ersetzen mit `--replace`. Die Token-Datei bleibt für Entwicklung und CI.

**Offen:** Die Bindung an die Developer ID gehört zu {{ ref("G5") }}.

**Quelle:** Entwicklung der Stufe 3, [Issue 50](https://github.com/seletz/passierschein/issues/50); Spike [Issue 53](https://github.com/seletz/passierschein/issues/53); Umsetzung [Issue 59](https://github.com/seletz/passierschein/issues/59). Ziel Schlüsselbund und Freigabe des Baus: Stefan, 2026-11-12.
