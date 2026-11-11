---
render_macros: true
id: 'D49'
titel: 'Die Konfiguration ist eine YAML-Datei unter ~/.config/a38, und der Pfad bestimmt das Projekt'
bereich: 'Betrieb und Release'
lauf: '2.0'
stand: '2026-11-11'
order: 3
headless: true
status: gueltig
use_cases: [UC11]
beleg:
  notiz: Vorschlag aus der Entwicklung, zu bestätigen durch Stefan
  annahme: true
---
# D49 — Die Konfiguration ist eine YAML-Datei unter ~/.config/a38, und der Pfad bestimmt das Projekt

{{ design_kopf(page) }}

**Entscheidung:** Die Konfiguration des Daemons ist eine YAML-Datei. Sie liegt unter `$XDG_CONFIG_HOME/a38/config.yaml`. Ohne die Variable liegt sie unter `~/.config/a38/config.yaml`. Der Schalter `--config` und die Umgebungsvariable `A38_CONFIG` nennen einen anderen Pfad; der Schalter hat Vorrang. Die Datei trägt eine Versionsnummer (`version: 1`). Der Daemon lehnt einen unbekannten Schlüssel ab.

**Inhalt:** Die Datei nennt die Liste der Vaults mit ihrer Umgebung ({{ ref("D12") }}, {{ ref("D47") }}), je Projekt die Pfade und die erlaubten Vaults ({{ ref("D28") }}), die Laufzeit eines Grants ({{ ref("D32") }}), das Bündelfenster und die Obergrenze eines Bündels ({{ ref("D29") }}), die Regeln für den Namen der Sitzung ({{ ref("D25") }}) und die Pfade für die Sockets und die Audit-Datei ({{ ref("D43") }}, {{ ref("D11") }}). Nur die Versionsnummer ist Pflicht. Jeder andere Wert hat einen Standard. Eine Dauer steht mit Einheit, zum Beispiel `4h`, `30m` oder `3s`.

**Erkennung des Projekts:** Ein Projekt nennt einen oder mehrere absolute Pfade. Eine Sitzung gehört zu dem Projekt, wenn ihr Arbeitsverzeichnis einer dieser Pfade ist oder darunter liegt. Passen mehrere Projekte, gilt der längste Pfad. Ein Pfad gehört zu höchstens einem Projekt. Der Daemon löst symbolische Links im Arbeitsverzeichnis auf, bevor er vergleicht. Passt kein Projekt, hat die Sitzung kein Projekt; jede Anfrage endet dann mit `not_allowed`. Der Daemon wertet das Git-Remote nicht aus.

**Dateien des Daemons:** Die Sockets liegen im Verzeichnis `$XDG_STATE_HOME/a38/run`, ohne die Variable unter `~/.local/state/a38/run`. Die Audit-Datei liegt unter `$XDG_STATE_HOME/a38/audit.db`, ohne die Variable unter `~/.local/state/a38/audit.db`. Die Konfiguration kann beide Pfade ändern. `a38 mcp` liest die Konfiguration nicht. `a38 mcp` findet den Agent-Socket über denselben Standardpfad oder über einen eigenen Startwert.

**Kein Neuladen im Betrieb:** Der Daemon liest die Datei einmal beim Start. Eine Änderung wirkt erst nach einem Neustart des Daemons. Der Neustart beendet alle Grants ({{ ref("D33") }}).

**Fehler:** Eine Datei, die fehlt, kein gültiges YAML ist oder einen ungültigen Wert enthält, bricht den Start ab. Die Fehlermeldung nennt jeden Befund. Die Ausnahme steht in {{ ref("D28") }}: Ein Projekt, das einen Vault außerhalb der Liste der Vaults nennt, ist ungültig, und der Daemon startet trotzdem. Derselbe Vault zweimal in der Liste ist ein Fehler, auch mit derselben Umgebung.

**Begründung:**

- YAML ist das Format des Fake-Backends ({{ ref("D38") }}). Ein Format für beide Dateien braucht nur eine Bibliothek. YAML erlaubt Kommentare; die Liste der Vaults ist eine Policy des Menschen und braucht Erläuterungen.
- Der Pfad `~/.config/a38` ist kurz, ohne Leerzeichen und unter macOS und Linux gleich. `~/Library/Application Support` enthält ein Leerzeichen und fehlt unter Linux.
- Der Pfad eines Unix-Sockets ist unter macOS auf 103 Byte begrenzt. Der Standardpfad bleibt weit darunter. Der Daemon prüft die Länge beim Laden.
- Der Pfad als Merkmal des Projekts ist ein geprüfter Fakt: Der Daemon liest das Arbeitsverzeichnis aus den Peer-Daten ({{ ref("D25") }}). Das Git-Remote steht in einer Datei des Arbeitsverzeichnisses. Der Agent kann diese Datei ändern. Der Daemon müsste dafür Dateien aus der Zone des Agents lesen und auswerten.
- Mehrere Pfade je Projekt decken Worktrees ab, die neben dem Projekt liegen.
- Ohne Neuladen wechselt der Daemon nie ohne Zutun des Menschen sein Verhalten. Das folgt dem Grundsatz aus {{ ref("D48") }}. Ein Neuladen bräuchte Regeln für bestehende Grants und für eine Datei, die mitten im Betrieb ungültig wird.

**Folge:** Nach einer Änderung der Policy startet der Mensch den Daemon neu und gibt danach neu frei. Erweist sich das im Alltag als störend, ist ein Neuladen eine neue Entscheidung.

**Offen:** Ob das Arbeitsverzeichnis unter herdr und in Worktrees immer das richtige Projekt ergibt, klärt {{ ref("G2") }}.

**Quelle:** Entwicklung der Stufe 1, [Issue 16](https://github.com/seletz/passierschein/issues/16). Die Entscheidung schließt {{ ref("G17") }}. Sie ist ein Vorschlag aus der Entwicklung; Stefan bestätigt sie.
