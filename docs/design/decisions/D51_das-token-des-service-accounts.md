---
render_macros: true
id: 'D51'
titel: 'Das Token liegt im Schlüsselbund, eine Datei mit Modus 0600 ist der Ausweichweg'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-12'
order: 5
headless: true
status: gueltig
use_cases: [UC11]
beleg:
  notiz: 'Ziel und Bau: Stefan, 2026-11-12 (Issues 53, 59). Zu bestätigen durch Stefan: Befehlsnamen, Standardquelle keychain, Eintrag a38.onepassword, --replace, Lesen in token status'
  annahme: true
---
# D51 — Das Token liegt im Schlüsselbund, eine Datei mit Modus 0600 ist der Ausweichweg

{{ design_kopf(page) }}

**Entscheidung:**

1. **Ziel und Standard:** Das Token des Service Accounts liegt im Anmelde-Schlüsselbund von macOS, im Eintrag mit dem Dienst `a38.onepassword` und dem Konto `service-account`. Der Mensch fügt das Token im Terminal in `a38 token set` ein; `a38` legt den Eintrag selbst über Security.framework an, nie über `/usr/bin/security`. Der Schlüsselbund vertraut dann nur dem Build von `a38`, der den Eintrag angelegt hat. Stefan hat das Ziel am 2026-11-12 festgelegt und den Bau freigegeben. Umgesetzt in [Issue 59](https://github.com/seletz/passierschein/issues/59), nach dem [Spike in Issue 53](https://github.com/seletz/passierschein/issues/53).
2. **Quelle per Konfiguration:** Der Schlüssel `backend.onepassword.token_source` wählt die Quelle: `keychain` (Standard) oder `file`. Ein anderer Wert verhindert das Laden der Konfiguration.
3. **Ausweichweg:** Die Token-Datei bleibt für Entwicklung und CI. Die Konfiguration nennt sie im Schlüssel `backend.onepassword.token_file`. Der Standard ist `$XDG_CONFIG_HOME/a38/onepassword-token`, ohne die Variable `~/.config/a38/onepassword-token`. Die Datei ist eine reguläre Datei, gehört dem Benutzer des Daemons und hat genau den Modus `0600`. Ein symbolischer Link, ein Verzeichnis, ein anderer Besitzer, ein anderer Modus oder eine leere Datei verhindern den Start mit dem 1Password-Backend.
4. **Befehle:** `a38 token set` liest das Token ohne Echo vom Terminal oder von stdin, nie aus einem Argument oder einer Umgebungsvariable. Ein vorhandener Eintrag wird nur mit `--replace` ersetzt. `a38 token status` zeigt den Eintrag ohne Wert; den Wert liest er nur, wenn die Zugriffsliste genau diesen Build nennt. `a38 token delete` löscht den Eintrag. Jede Ausgabe nennt höchstens Länge und Fingerabdruck (die ersten 8 Hex-Zeichen von SHA-256), nie das Token.
5. Der Daemon liest das Token einmal beim Start, über die Schnittstelle `token.Source`, aus genau einer Quelle. Fehlt der Eintrag im Schlüsselbund, startet der Daemon mit dem 1Password-Backend nicht; die Meldung nennt `a38 token set`. Die Meldung nennt nie das Token.
6. Der Daemon hält das Token nur im Arbeitsspeicher, in einem Typ mit der Ausgabe `[redacted]`. Das Token steht nie im Log, in einer Fehlermeldung, im Audit-Log oder im Status.
7. Es gibt keine Umgebungsvariable für das Token.
8. `a38 doctor` prüft die Quelle, ohne das Token zu lesen: den Eintrag und seine Zugriffsliste, die Signatur von `a38` als Hinweis, eine übrige Token-Datei als Befund; bei der Quelle `file` Existenz, Besitzer und Modus der Datei.

**Grenze bis zur Signatur ({{ ref("G5") }}):** Ohne Developer ID bindet der Schlüsselbund den Eintrag an den Hash des Builds. Jeder neue Build von `a38` braucht eine Bestätigung in einem Dialog von macOS. Ein anderes Programm kann denselben Dialog auslösen; der Mensch kann beide nicht unterscheiden. Der Schutz gegen einen Prozess desselben Benutzers beginnt erst mit der Signatur mit Developer ID und Hardened Runtime. Bis dahin bleibt {{ ref("G3") }} im Teil der Signatur offen, und {{ ref("RI5") }} bleibt bestehen.

**Schwäche der Datei:** Jeder Prozess desselben Benutzers kann die Datei lesen, auch ein Agent, der auf dem Mac läuft. Der Modus `0600` schützt nur vor anderen Benutzern. Wer die Datei liest, liest alle Items der freigegebenen Vaults, bis der Mensch das Token widerruft. Die Datei kann in eine Datensicherung gelangen. Das ist ein Fall von {{ ref("RI5") }}. Die Rechte des Service Accounts (nur Lesen, nur die freigegebenen Vaults, {{ ref("D39") }}) begrenzen den Schaden.

**Begründung:** Ein Eintrag im Schlüsselbund, den `a38` selbst anlegt, ist für ein anderes Programm desselben Benutzers nicht still lesbar; das hat der Spike gemessen. Der Weg über `purego` braucht kein CGO. Die Datei ist einfach und reicht für Entwicklung und CI, wo jeder Neubau sonst einen Dialog auslöste. Ein Kindprozess erbt die Umgebung, auch ein Agent; eine Umgebungsvariable gäbe das Token weiter.

**Offen:** Die Namen der Befehle, die Quelle `keychain` als Standard, der Name des Eintrags, `--replace` und was `a38 token status` liest, sind Vorschläge aus der Entwicklung; Stefan bestätigt sie im Praxistest. Die Bindung an die Developer ID gehört zu {{ ref("G5") }}.

**Quelle:** Entwicklung der Stufe 3, [Issue 50](https://github.com/seletz/passierschein/issues/50); Spike [Issue 53](https://github.com/seletz/passierschein/issues/53); Umsetzung [Issue 59](https://github.com/seletz/passierschein/issues/59). Ziel Schlüsselbund und Freigabe des Baus: Stefan, 2026-11-12.
