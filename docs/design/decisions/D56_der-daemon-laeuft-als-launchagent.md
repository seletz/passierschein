---
render_macros: true
id: 'D56'
titel: 'Der Daemon läuft als LaunchAgent ohne automatischen Neustart'
bereich: 'Betrieb und Release'
lauf: '2.0'
stand: '2026-11-13'
order: 6
headless: true
status: gueltig
use_cases: [UC11]
beleg:
  wer: Stefan
  datum: '2026-11-13'
  quelle: ['#75']
---
# D56 — Der Daemon läuft als LaunchAgent ohne automatischen Neustart

{{ design_kopf(page) }}

**Entscheidung:**

1. **Label und Datei:** `a38 service install` legt einen LaunchAgent mit dem Label `io.github.seletz.passierschein.daemon` an. Das Label beginnt mit dem Bundle-Bezeichner der App ({{ ref("D50") }}). Die plist liegt unter `~/Library/LaunchAgents/io.github.seletz.passierschein.daemon.plist`. Ein Kommentar in der plist kennzeichnet sie als Datei von `a38`. Eine plist ohne diese Kennzeichnung ändert und entfernt `a38` nur mit `--force`.
2. **Programm:** `ProgramArguments` nennt den echten Pfad des laufenden `a38`, nach Auflösung der symbolischen Links. Aus dem Release ist das `Passierschein.app/Contents/MacOS/a38`. Der Standard ist `serve --backend 1password --ui app` ({{ ref("D40") }}). `--config` steht nur in der plist, wenn der Mensch die Datei mit `--config` oder `A38_CONFIG` nennt.
3. **Kein automatischer Neustart:** `RunAtLoad` ist `true`, `KeepAlive` ist `false`. launchd startet den Daemon bei der Anmeldung und nie wieder von selbst. Der Mensch startet ihn mit `a38 service restart` neu.
4. **Sitzung und Rang:** `LimitLoadToSessionType` ist `Aqua`. `ProcessType` ist `Interactive`. `ExitTimeOut` ist 20 Sekunden. `Umask` ist `077`. `AssociatedBundleIdentifiers` nennt `io.github.seletz.passierschein`.
5. **Log:** stdout und stderr gehen nach `~/Library/Logs/a38/serve.log`. launchd und `a38` rotieren die Datei nicht. Die Datei enthält keinen Secret-Wert.
6. **Umgebung:** `EnvironmentVariables` enthält höchstens `XDG_CONFIG_HOME` und `XDG_STATE_HOME` der Shell, die installiert. Ein Secret steht nie in der plist ({{ ref("D51") }}).
7. **Exit-Codes von `a38 serve`:** 0 heißt normales Ende nach SIGTERM. 1 heißt Fehler beim Start oder im Betrieb, zum Beispiel Konfiguration, Quelle des Tokens oder Verzeichnis der Sockets. 2 heißt falscher Aufruf oder Absturz der Go-Laufzeit. Kein Code führt zu einem Neustart.
8. **Vorprüfung:** `a38 service install` installiert nichts, was nicht starten kann. Die Konfiguration muss laden, die Freigabe-App muss die Prüfung der Datei bestehen, die Quelle des Tokens muss bereit sein (nur Metadaten, das Token liest der Befehl nie), die Pfade der Sockets müssen kurz genug sein, und kein anderer Daemon darf auf dem Admin-Socket antworten. Das Programm muss dem Benutzer oder `root` gehören, und weder Gruppe noch andere dürfen es oder sein Verzeichnis schreiben.

**Begründung:**

- Ein Neustart hebt die globale Sperre auf. Nur der Mensch darf das ({{ ref("D48") }}). Ein Neustart durch launchd wäre ein Wechsel ohne Zutun des Menschen.
- Ein Fehler beim Start bleibt, bis der Mensch ihn behebt. Eine Schleife von Neustarts hilft nicht ({{ ref("D49") }}).
- Mit der Quelle `keychain` kann jeder Start einen Dialog von macOS zeigen. Eine Schleife von Dialogen gewöhnt den Menschen an das Bestätigen ({{ ref("D51") }}).
- Der Daemon braucht die Sitzung am Bildschirm für das Popup und den Anmelde-Schlüsselbund. `Aqua` lädt ihn nur dort.
- Die Freigabe-App ist ein Kindprozess des Daemons und teilt seine Grenzen für Rechenzeit. Ein Mensch wartet auf sie. Ohne `ProcessType` drosselt macOS den Prozess leicht.
- launchd gibt einem Agent 5 Sekunden zwischen SIGTERM und SIGKILL. Der Daemon braucht die Zeit, um die Grants mit einem Eintrag im Audit-Log zu beenden ({{ ref("T16") }}).
- launchd gibt die Umgebung der Shell nicht weiter. Ohne `--config` und die XDG-Variablen nutzte der Daemon andere Pfade als `a38 mcp` in der Shell.
- `~/Library/Logs` ist der Ort, den die Konsole von macOS zeigt.

**Folge:** Nach einem Absturz läuft kein Daemon. Jeder Agent erhält dann `daemon_unavailable` ({{ ref("RI8") }}). `a38 doctor` und `a38 service status` nennen den letzten Exit-Code und das Log. Der Mensch startet den Daemon mit `a38 service restart`.

**Offen:** Was macOS in den Einstellungen unter „Anmeldeobjekte“ zeigt, prüft der Praxistest mit dem signierten Release ({{ ref("G5") }}).

**Quelle:** Entwicklung der Stufe 3, [Issue 70](https://github.com/seletz/passierschein/issues/70). Die Entscheidung ist ein Vorschlag aus der Entwicklung; Stefan bestätigt sie.
