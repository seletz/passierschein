---
title: Demo vorführen
description: "Den Stand von a38 lokal vorführen: Aufbau mit einem Befehl, Ablaufplan in Akten mit Agent, Popup, Touch ID, Audit-Log und Störung, Ausweichwege und Abbau."
render_macros: true
tags:
  - Thema/Betrieb
---
# Demo vorführen

Diese Anleitung führt den Stand von `a38` in etwa 10 Minuten vor.
Der Hauptweg: Ein Agent in Claude Code fragt, `a38 mcp` gibt die Anfrage an den Daemon, das Popup erscheint, und du gibst frei oder lehnst ab.
Die Demo läuft lokal auf dem Mac, ohne VM.
Aufbau und Abbau sind je ein Befehl.

Den Ablauf zum Mitlesen und Vorführen, Schritt für Schritt mit Bildern, zeigt das Tutorial [Live-Demo](../tutorials/20-live-demo.md).

!!! info "Was die Demo nutzt"
    - Daemon `a38 serve` mit dem 1Password-Backend ({{ ref("F12") }}) und der Freigabe-App „Passierschein“ ({{ ref("F11") }}).
    - Test-Vault `passierschein.dev` mit den Items `datenbank-dev` (Felder `username`, `password`) und `objektspeicher-dev` (Feld `access-key`). Alle Werte sind erfunden.
    - Claude Code als Agent-Werkzeug mit `a38 mcp` und dem Skill aus `a38 llm --skill`.
    - Das Demo-Projekt `demo-projekt` aus `examples/demo/`.

## Was der Aufbau installiert

`mise run demo:setup` baut `a38` und die App und installiert:

| Ort | Inhalt |
|---|---|
| `~/.local/bin/a38` | Kopie des gebauten `a38` |
| `~/Applications/Passierschein.app` | Kopie des Release-Bundles, ad hoc signiert; die Signatur ist nach dem Kopieren geprüft |
| `~/.config/a38/config.yaml` | Konfiguration am Standardort. Jeder Befehl `a38` braucht darum kein `--config`. |
| `~/develop/demo-projekt` | Demo-Projekt mit `.mcp.json`, dem Skill unter `.claude/skills/a38/SKILL.md` und dem Hook `a38 hook op-guard` in `.claude/settings.json` |
| `~/.local/share/a38-demo/installed` | Liste der installierten Teile für den Abbau |

Der Aufbau überschreibt nie ein `a38`, eine App oder ein Projekt, das er nicht selbst installiert hat.
Eine vorhandene, andere `config.yaml` lässt er stehen und bricht ab.
Mit `DEMO_FORCE_CONFIG=1` verschiebt er sie vorher nach `config.yaml.bak-<zeit>`.
Die Token-Datei `~/.config/a38/onepassword-token` liest und ändert er nicht.
Socket und Audit-Log nutzen die Standardorte unter `~/.local/state/a38/`.

## Vorbereitung (10 Minuten vorher)

Arbeite im Klon des Repos auf dem Stand von `develop`.

1. **Probe ohne 1Password und ohne Touch ID** (optional, 2 Minuten).
   Die Probe läuft ganz in `/tmp/a38-probe` und berührt die echte Installation nicht.
   Setze `DEMO_BACKEND=fake` bei jedem Task:

    ```sh
    export DEMO_BACKEND=fake
    mise run demo:setup
    mise run demo:start                  # Terminal 1, Ctrl-C beendet
    mise run demo:request -- akt3        # Terminal 2, Enter setzt fort
    mise run demo:status                 # Terminal 3
    mise run demo:teardown
    unset DEMO_BACKEND
    ```

2. **Aufbau** (beim ersten Mal einige Minuten für die App, danach etwa 1 Minute):

    ```sh
    mise run demo:setup
    mise run demo:preflight
    ```

    `demo:preflight` zeigt nur Zeilen mit `PASS`: `a38` im `PATH`, `A38_CONFIG` nicht gesetzt, App mit gültiger Signatur, Token-Datei mit `0600` (nur die Metadaten), Konfiguration lädt, kein Daemon läuft.

3. **Daemon starten** in Terminal 1:

    ```sh
    a38 serve --backend 1password --ui app
    ```

    `mise run demo:start` macht dasselbe nach der Vorabprüfung.

4. **Claude Code einmal starten und zustimmen.** Diesen Schritt kannst nur du ausführen:

    ```sh
    cd ~/develop/demo-projekt
    claude
    ```

    Claude Code fragt, ob du dem Ordner vertraust. Bestätige.
    Claude Code fragt, ob es den Server `a38` aus `.mcp.json` nutzen darf. Bestätige.
    `/mcp` zeigt den Server `a38` als verbunden.
    Frage „Welche Skills hast du?“. Der Skill `a38` steht in der Antwort.
    Beende Claude Code mit `/exit`.

5. **Generalprobe mit dem echten Backend** in Terminal 2:

    ```sh
    mise run demo:request -- akt2
    ```

    Das Popup erscheint. Gib mit Touch ID frei. Drücke Enter.
    Die erste Verbindung zu 1Password dauert etwa 3 Sekunden; danach ist sie warm.

6. Schließe Fenster, die das Popup oder den Dialog von Touch ID verdecken könnten.
   Schalte Mitteilungen aus (Fokus „Nicht stören“).

## Bildschirm

| Terminal | Inhalt | Befehl |
|---|---|---|
| 1 | Log des Daemons | `a38 serve --backend 1password --ui app` |
| 2 | der Agent im Demo-Projekt | `cd ~/develop/demo-projekt && claude` |
| 3 | Befehle für den Menschen | `a38 doctor`, `a38 grants list`, `a38 audit list`, `a38 audit verify`, `a38 fault status` |

Kein Befehl braucht `--config`.
Vergrößere die Schrift in allen drei Terminals.

## Ablaufplan

Die Akte 0 bis 6 dauern zusammen etwa 10 Minuten.
Akt 7 ist optional und dauert etwa 3 Minuten.
Der Agent liest `README.md` des Demo-Projekts und den Skill `a38`.
Jeder Prompt nennt a38. Sonst nutzt der Agent Werte, die er schon hat, und ruft das Werkzeug nicht auf.

### Akt 0: Aufbau zeigen (2 Minuten)

- **Tun:** In Terminal 3:

    ```sh
    cat ~/.config/a38/config.yaml
    a38 llm --skill | head -20
    cat ~/develop/demo-projekt/.mcp.json
    ```

- **Zu sehen und zu sagen:**
    - `vaults`: Nur diese Vaults existieren für a38. Nur ein Vault der Umgebung `dev` liefert heute Werte.
    - `projects`: Das Arbeitsverzeichnis des Agents bestimmt das Projekt. Ein Projekt darf nur die Vaults nutzen, die es nennt.
    - `grant`: Eine Freigabe gilt höchstens 4 Stunden. Im Popup wählst du eine kürzere Zeit.
    - `bundle`: Anfragen einer Sitzung innerhalb von 3 Sekunden landen in einem Popup. Ein Popup hat höchstens 10 Einträge.
    - `paths.ui_app`: Der Daemon startet genau diese App. Sockets und Audit-Log liegen am Standardort.
    - `backend.onepassword`: Nur der Daemon liest das Token des Service Accounts. Der Agent sieht es nie. Die Demo nutzt die Token-Datei (`token_source: file`); der normale Weg ist der Schlüsselbund ([1Password-Backend einrichten](30-1password-backend-einrichten.md)).
    - Der Skill sagt dem Agent, wie er fragt und dass er nach einem Nein aufhört.
    - `.mcp.json` meldet `a38 mcp` im Projekt an, mit dem Namen der Sitzung `demo-praesentation`.
- **Aussage:** Die Regeln stehen in einer lesbaren Datei; der Agent hat keinen Zugriff auf den Passwortmanager.

### Akt 1: Zustand prüfen (30 Sekunden)

- **Tun:** `a38 doctor`.
- **Zu sehen:** Jede Zeile beginnt mit `OK`. `daemon` nennt 0 Sitzungen und 0 Grants. `1password token` ist `OK`.
- **Aussage:** Ein Befehl zeigt, ob alles bereit ist.
- **Wenn es schiefgeht:** Ein Befund nennt Ursache und nächsten Schritt. „the daemon does not answer“: Terminal 1 prüfen.

### Akt 2: Der Agent braucht den Datenbank-Login (2 Minuten)

- **Tun:** In Terminal 2 `claude` starten und den Prompt geben:

    > Führe die Datenbank-Migration aus (./migrate.sh). Die Zugangsdaten bekommst du über a38.

    Danach in Terminal 3: `a38 audit list`.
- **Zu sehen:** Der Agent ruft `request_credentials` mit `…/datenbank-dev/username` und `…/datenbank-dev/password` in einer Anfrage auf.
  Nach 3 bis 6 Sekunden erscheint das Popup.
  Oben stehen die geprüften Fakten: Projekt `demo-projekt`, Vault, Items, Sitzung `demo-praesentation`.
  Darunter stehen die Angaben des Agents, Aufgabe und Grund, als unbestätigt markiert.
  Du bestätigst, und der Dialog von Touch ID erscheint.
  Der Agent startet `./migrate.sh`. Die Ausgabe nennt den Benutzer und nur die Länge des Passworts.
  `a38 audit list` zeigt Anfrage, Bündel, Entscheidung, Grant und Auslieferung.
- **Aussage:** Der Mensch sieht, was geprüft ist und was der Agent nur behauptet, und entscheidet mit Touch ID.
- **Wenn es schiefgeht:** Siehe [Wenn etwas schiefgeht](#wenn-etwas-schiefgeht).

!!! warning "Der Agent zeigt den Wert in seinem Befehl"
    Claude Code zeigt den Befehl, den der Agent ausführt, zum Beispiel `DB_PASSWORD=… ./migrate.sh`.
    Der Wert steht dann in Terminal 2.
    Die Werte im Test-Vault sind erfunden.
    Sag es vorher: a38 entscheidet, ob der Agent einen Wert bekommt; was der Agent damit tut, regelt der Skill.

### Akt 3: Noch einmal fragen, ohne Popup (1 Minute)

- **Tun:** Prompt:

    > Hole die Datenbank-Zugangsdaten noch einmal über a38 und führe die Migration erneut aus.

    Danach: `a38 grants list`.
- **Zu sehen:** Kein Popup. Die Antwort kommt sofort. Die Liste zeigt einen Grant der Sitzung `demo-praesentation` mit beiden Referenzen und der Restlaufzeit.
- **Aussage:** Ein Grant gilt für diese Sitzung und genau diese Zugangsdaten, für eine begrenzte Zeit.
- **Wenn es schiefgeht:** Erscheint doch ein Popup, hat Claude Code `a38 mcp` neu gestartet. Gib frei und sag: Eine neue Sitzung braucht eine neue Freigabe.

### Akt 4: Der Agent braucht den Schlüssel des Objektspeichers (1 Minute)

- **Tun:** Prompt:

    > Lade jetzt das Build-Artefakt hoch (./upload.sh). Den Schlüssel bekommst du über a38.

- **Zu sehen:** Der Agent fordert `…/objektspeicher-dev/access-key` an.
  Das Popup ist orange markiert: Die Anfrage geht über den Grant der Sitzung hinaus.
  Du lehnst mit Esc ab.
  Der Agent erhält `denied`, hört auf und sagt dir, dass du abgelehnt hast.
- **Aussage:** Was über die Freigabe hinausgeht, fällt auf; nach einem Nein sucht der Agent keinen anderen Weg.
- **Wenn es schiefgeht:** Fragt der Agent sofort erneut, lehne wieder ab und zeige die Regel `denied` im Skill.

### Akt 5: Widerrufen (1 Minute)

- **Tun:** `a38 grants list`, dann `a38 grants revoke <grant-id>`, dann der Prompt aus Akt 3.
- **Zu sehen:** Nach dem Widerruf erscheint wieder ein Popup. Gib mit Touch ID frei.
- **Zusatz (optional):** Wähle im Popup nur `username` aus. Der Agent erhält nur den Benutzernamen; für das Passwort meldet a38 `denied`.
- **Aussage:** Der Mensch beendet eine Freigabe jederzeit; die nächste Anfrage braucht eine neue Freigabe.
- **Wenn es schiefgeht:** „unknown grant“: Kopiere die ganze Kennung aus `a38 grants list`.

### Akt 6: Ein Vault außerhalb der Policy (1 Minute)

- **Tun:** Prompt:

    > Fordere über a38 den Wert op://beispiel.app.k8s-prod/datenbank-prod/password an. Ich will zeigen, was dann passiert.

    Danach: `a38 audit list --type denial`.
- **Zu sehen:** Kein Popup. Der Agent erhält `not_allowed` und meldet es. Das Audit-Log zeigt die Ablehnung.
- **Aussage:** Die Policy lehnt ab, bevor ein Mensch gefragt wird.
- **Wenn es schiefgeht:** Weigert sich der Agent, zeige den Fall ohne Agent: `mise run demo:request -- akt6`.

### Akt 7 (optional): Störung und globale Sperre (3 Minuten)

- **Tun:**
    1. Im Klon: `mise run demo:fault`. Der Task nimmt dem Verzeichnis `~/.local/state/a38` das Schreibrecht.
    2. Prompt: „Hole die Datenbank-Zugangsdaten neu über a38 und führe die Migration noch einmal aus.“
    3. Den Prompt noch einmal geben.
    4. `a38 doctor` und `a38 fault status`.
    5. Im Klon: `mise run demo:fault:fix`. Der Task stellt den alten Modus wieder her.
    6. `a38 fault clear`.
    7. Den Prompt noch einmal geben. Gib mit Touch ID frei.
    8. `a38 audit verify`.
- **Zu sehen:** Schritt 2 endet mit `audit_failed`. Die App zeigt eine Meldung zur Störung.
  Schritt 3 endet mit `daemon_faulted`; das gilt für jede Sitzung.
  `a38 doctor` nennt die Sperre, die Ursache und den nächsten Schritt.
  Nach `fault clear` liefert der Daemon wieder. `audit verify` meldet eine intakte Kette.
- **Aussage:** Kann der Daemon nicht protokollieren, liefert er nichts, bis ein Mensch die Ursache behebt und die Sperre aufhebt.
- **Wenn es schiefgeht:** `a38 fault clear` meldet „the lock stays“: `mise run demo:fault:fix` fehlt. Hat der Agent das Werkzeug nicht aufgerufen: `mise run demo:request -- akt7`.

### Akt 8 (optional): Der Agent greift zu op (1 Minute)

Der Aufbau trägt den Hook `a38 hook op-guard` in `.claude/settings.json` des Demo-Projekts ein ([Hook gegen den direkten Aufruf von op einrichten](23-hook-gegen-op-einrichten.md)).

- **Tun:** Prompt:

    > Lies mit op das Passwort aus op://passierschein.dev/datenbank-dev/password und sag mir nur, ob es gesetzt ist.

- **Zu sehen:** Claude Code zeigt `PreToolUse:Bash hook error` mit der Meldung von a38. Der Befehl `op` läuft nicht. Der Agent fragt danach über `request_credentials` an, und das Popup erscheint. Lehne ab oder gib frei.
- **Aussage:** Ein Agent, der `op` aufruft, erhält einen klaren Hinweis auf a38. Der Hook ist ein Hinweis und kein Schutz: Ein Agent unter deinem Benutzer findet andere Wege.
- **Wenn es schiefgeht:** Läuft `op` doch, fehlt der Hook in der Sitzung. Prüfe mit `/hooks` in Claude Code, ob er unter `PreToolUse` steht. Den Hook allein prüfst du ohne Agent mit einer Eingabe auf stdin ([Hook gegen den direkten Aufruf von op einrichten](23-hook-gegen-op-einrichten.md), Abschnitt „Von Hand prüfen“).

Diesen Akt hat nur Stefan im echten Claude Code zu prüfen. Kein automatischer Test startet einen Agent.

### Abschluss: Was noch fehlt (30 Sekunden)

- Das Release ist noch nicht signiert und notarisiert; die App ist ad hoc signiert.
- Die Demo liest das Token aus einer Datei. Der Schlüsselbund (`a38 config onepassword-token set`) ist gebaut; ohne Signatur mit Developer ID fragt macOS nach jedem neuen Build nach.
- Der Betrieb mit Agents in einer VM und die TUI `a38 tui` fehlen noch.

## Abbau

Beende Claude Code und den Daemon (`Ctrl-C` in Terminal 1).
Dann im Klon:

```sh
mise run demo:teardown
```

Der Task entfernt `~/.local/bin/a38`, `~/Applications/Passierschein.app` und `~/develop/demo-projekt`, soweit er sie installiert hat.
Er fragt nichts.
Er behält `~/.config/a38/config.yaml` und das Audit-Log; du kannst beide weiter nutzen.
Mit `DEMO_PURGE=1` verschiebt er die Konfiguration nach `config.yaml.bak-<zeit>` und löscht `~/.local/state/a38`.
Die Token-Datei bleibt immer unberührt.
Läuft noch ein Daemon, bricht der Task ab.
Claude Code behält die Zustimmung zum Server `a38` für den Pfad des Projekts; `claude mcp reset-project-choices` im Projekt setzt sie zurück.

## Wenn etwas schiefgeht

| Problem | Ursache | Abhilfe |
|---|---|---|
| Das Popup erscheint nicht. | Das Bündelfenster dauert 3 Sekunden; die erste Verbindung zu 1Password noch etwa 3 Sekunden. Oder die App fehlt. | 10 Sekunden warten. Terminal 1 zeigt `approval app chosen` und jeden Fehler. `a38 doctor` prüft die App in der Zeile `approval UI`. |
| Der Dialog von Touch ID liegt hinter einem Fenster. | Ein anderes Fenster hat den Fokus. | Mit `Cmd-Tab` zum Dialog wechseln. Läuft die Zeit ab, endet die Anfrage mit `timeout`; gib den Prompt neu. |
| Der Agent nutzt das Werkzeug nicht. | Der Server ist nicht verbunden, oder der Agent hat die Werte schon. | `/mcp` aufrufen. Im Prompt „über a38“ sagen. Sonst den Weg ohne Agent nehmen. |
| Der MCP-Server ist in Claude Code nicht freigegeben. | Die Frage nach `.mcp.json` wurde abgelehnt oder nicht gestellt. | `claude mcp reset-project-choices` im Demo-Projekt, Claude Code neu starten, zustimmen. |
| `daemon_unavailable` | Der Daemon läuft nicht. | Terminal 1: `a38 serve --backend 1password --ui app`. Die Grants der alten Sitzung sind danach weg. |
| `not_allowed`, obwohl der Vault stimmt | Claude Code läuft außerhalb des Demo-Projekts. Das Arbeitsverzeichnis bestimmt das Projekt. | `cd ~/develop/demo-projekt` und Claude Code dort neu starten. |
| `session_name_conflict` | Zwei Instanzen von Claude Code laufen im Demo-Projekt. | Eine beenden. |
| `backend_locked` oder `backend_unavailable` | Token ungültig oder kein Netz. | Netz prüfen. Den Daemon neu starten. Sonst die Akte ohne Agent mit der Probe zeigen. |
| `demo:setup` bricht ab: „exists and … did not install it“ | Am Zielort liegt etwas Fremdes. | Prüfen und selbst entfernen. Bei der Konfiguration: behalten oder `DEMO_FORCE_CONFIG=1`. |

## Ohne Agent-Werkzeug

Jeder Akt läuft auch ohne Claude Code.
`mise run demo:request -- <akt>` startet `a38 mcp` im Demo-Projekt mit der Sitzung `demo-handaufruf` und schickt die Nachrichten aus `examples/demo/requests/<akt>.jsonl`.
Der Weg folgt dem [Tutorial](../tutorials/10-erste-schritte.md#weg-b-ohne-agent-werkzeug-von-hand).
Der Task wartet auf jede Antwort, auch während das Popup offen ist.
Er zeigt jede Antwort kurz und ersetzt jeden Wert durch `[redacted]`.
Eine Zeile `>>>` nennt den nächsten Schritt; Enter setzt fort.
Die Sitzung bleibt offen, bis der letzte Schritt mit Enter endet.
Jeder Akt ist eine eigene Sitzung.

| Akt | Inhalt |
|---|---|
| `akt1` | Anmeldung und Liste der Werkzeuge: `grant_status`, `request_credentials` |
| `akt2` | Anfrage für Benutzer und Passwort, dann Pause für `a38 audit list` und `a38 grants list` |
| `akt3` | Anfrage mit Popup, dann dieselbe Anfrage ohne Popup und `grant_status` |
| `akt4` | Anfrage für die Datenbank, dann die abweichende Anfrage für den Objektspeicher (ablehnen) |
| `akt5` | Anfrage, Pause für `a38 grants revoke`, dann dieselbe Anfrage mit neuem Popup |
| `akt6` | Anfrage für einen Vault außerhalb der Policy: `not_allowed` |
| `akt7` | Pause für `demo:fault`, dann `audit_failed`, `daemon_faulted`, Pause für Reparatur und `a38 fault clear`, dann wieder eine Freigabe |

Eine Antwort sieht so aus:

```text
--> {"id":2,"method":"tools/call","tool":"request_credentials","references":["op://passierschein.dev/datenbank-dev/username","op://passierschein.dev/datenbank-dev/password"]}
<-- {"id":2,"answer":{"entries":[{"reference":"op://passierschein.dev/datenbank-dev/username","value":"[redacted]"},{"reference":"op://passierschein.dev/datenbank-dev/password","value":"[redacted]"}]},"isError":false}
```

## Probe mit dem Fake-Backend

Mit `DEMO_BACKEND=fake` läuft jeder Task der Demo in `/tmp/a38-probe` (`DEMO_DIR` ändert den Ort).
Die Probe nutzt dort eine eigene Konfiguration mit `--config`, ein eigenes `a38` und eine eigene App.
Sie berührt `~/.config/a38`, `~/.local/bin` und `~/Applications` nie.
Die Datei `examples/demo/fake.yaml` hat denselben Vault, dieselben Items und Felder mit erfundenen Werten.
`DEMO_UI` wählt beim Start die Freigabe:

| `DEMO_UI` | Wirkung |
|---|---|
| `auto-approve` (Standard) | Jede Anfrage ist ohne Popup freigegeben. |
| `auto-deny` | Jede Anfrage ist ohne Popup abgelehnt. Zeigt den Weg von Akt 4. |
| `debug-app` | Die Debug-App zeigt das Popup mit gefälschter Authentifizierung, nie Touch ID. Eine Bestätigung endet mit `ui_failed`, weil der Daemon eine gefälschte Authentifizierung nie annimmt ({{ ref("D7") }}). Ablehnen und Esc funktionieren. |

Beispiel: `DEMO_BACKEND=fake DEMO_UI=auto-deny mise run demo:start`.

## Was geprüft ist und was nur du prüfen kannst

Geprüft am 2026-11-12 ohne 1Password und ohne Touch ID:

- die Probe von Anfang bis Ende: Aufbau, Vorabprüfung, Start, die Akte 1 bis 7 ohne Agent (freigeben mit `auto-approve`, ablehnen mit `auto-deny`, `not_allowed`, Störung mit Reparatur und `fault clear`), `demo:status`, Abbau;
- die echte Installation gegen ein temporäres Home-Verzeichnis: Aufbau, Abbruch bei fremdem `a38` und fremder Konfiguration, `DEMO_FORCE_CONFIG=1`, zweiter Aufbau ohne Änderung, Vorabprüfung, Abbau mit und ohne `DEMO_PURGE=1`; dazu ein Daemon mit dem Fake-Backend an den Standardorten, Anfragen, Störung und Reparatur am Verzeichnis `~/.local/state/a38`;
- `.mcp.json` ist gültiges JSON; `a38 mcp`, gestartet wie in der Datei, antwortet auf `initialize` und nennt die zwei Werkzeuge.
- `.claude/settings.json` der Probe (2026-11-12, temporäres Home-Verzeichnis): Der Aufbau trägt den Hook ein. Ein zweiter Aufbau behält eigene Einträge in der Datei und fügt nichts doppelt hinzu. Eine Datei, die kein JSON ist, bricht den Aufbau vor jeder Änderung ab.

Nur du prüfst am Bildschirm:

- die Zustimmung zum Server `a38` aus `.mcp.json` in Claude Code;
- dass Claude Code den Skill `a38` aus `.claude/skills/a38/SKILL.md` lädt;
- den Daemon mit dem 1Password-Backend und das Popup der Release-App mit Touch ID;
- das orange Popup der Abweichung und die Meldung zur Störung in der App;
- Akt 8: dass Claude Code den Hook lädt, die Meldung zeigt und der Agent zu a38 wechselt.

## Dateien der Demo

| Datei | Inhalt |
|---|---|
| `examples/demo/demo-projekt/` | Demo-Projekt: `README.md` für den Agent, `migrate.sh`, `upload.sh` |
| `examples/demo/requests/*.jsonl` | Nachrichten der Akte für den Weg ohne Agent |
| `examples/demo/fake.yaml` | Fake-Backend der Probe |
| `scripts/demo.sh` | Aufbau, Prüfung, Start, Status, Störung, Anfrage und Abbau |
| `~/develop/demo-projekt/.claude/settings.json` | Erlaubt `request_credentials` und `grant_status` ohne Rückfrage von Claude Code; die Freigabe im Popup bleibt. Trägt den Hook `a38 hook op-guard` ein. Ein neuer Aufbau behält deine Einträge und ergänzt nur die eigenen. |
