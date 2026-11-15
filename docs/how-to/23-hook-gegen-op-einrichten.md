---
title: Hook gegen den direkten Aufruf von op einrichten
description: "a38 hook op-guard in Claude Code für alle Projekte eintragen, von Hand prüfen und wieder entfernen."
render_macros: true
---
# Hook gegen den direkten Aufruf von op einrichten

Ein Agent soll Zugangsdaten nur über a38 anfragen.
Der Skill sagt ihm das.
Der Hook `a38 hook op-guard` erinnert ihn daran, wenn er es vergisst ({{ ref("F16") }}, {{ ref("F24") }}, {{ ref("UC12") }}, {{ ref("UC16") }}).
Diese Anleitung trägt den Hook in Claude Code ein.
Sie empfiehlt den Benutzerbereich: Dann gilt der Hook für alle deine Projekte ({{ ref("D67") }}).

## Was der Hook tut

Claude Code startet den Hook vor jedem Befehl des Werkzeugs `Bash`.
Der Hook liest den Befehl und entscheidet.

- Er **blockiert** einen Befehl, der die 1Password-CLI `op` startet. Der Befehl läuft nicht.
- Er blockiert auch einen Befehl, der das Token von a38 liest: die Token-Datei (`…/onepassword-token`) und den Eintrag `a38.onepassword` im Schlüsselbund mit `security find-generic-password -w`.
- Der Agent erhält eine kurze Meldung. Sie nennt die beiden Wege von a38 und keinen Umweg: die MCP-Werkzeuge `run_with_credentials` und `request_credentials` des Servers `a38`.
- Jeden anderen Befehl **lässt er durch** und schreibt nichts.
- Andere Werkzeuge als `Bash` prüft er nicht.

Die Meldung an den Agent lautet:

```text
Blocked by a38: do not run the 1Password CLI "op".
To run a command with a credential, use the MCP tool
run_with_credentials of the server a38. For a dev credential
value, you can also use request_credentials.
```

Liest der Befehl das Token von a38, lautet die erste Zeile `Blocked by a38: do not read the 1Password token of a38.`
Der Rest bleibt gleich.

Mit dem exec-Weg hat jeder Aufruf von `op` einen Ersatz, auch für test und prod.
Der Agent startet sein Kommando über `run_with_credentials` ([Befehl mit Zugangsdaten ausführen lassen](25-befehl-mit-zugangsdaten-ausfuehren.md)).
Deshalb passt der Hook jetzt für alle Projekte.

## Was der Hook nicht tut

!!! warning "Ein Hinweis, kein Schutz"
    Der Hook ist ein Hinweis für einen Agent, der sich an die Regeln hält ({{ ref("D15") }}).
    Er verhindert Versehen.
    Er hält keinen Agent auf, der a38 umgehen will.
    Der Agent läuft unter deinem Benutzer.
    Er kann ein Secret auf vielen anderen Wegen lesen, zum Beispiel über ein Skript, eine Variable oder ein anderes Programm ({{ ref("RI5") }}).
    Den Schutz liefert erst die VM ohne 1Password (Stufe 5).

- **Im Zweifel lässt der Hook durch.** Eine Eingabe, die kein JSON ist, die größer als 1 MiB ist oder deren Befehl kein Parser versteht, passiert. Der Rest von a38 schließt im Zweifel. Der Hook nicht: Er ist keine Grenze, und ein Fehler im Hook darf die Arbeit des Agents nicht anhalten ({{ ref("D58") }}).
- Ein Befehl, dessen Kommandowort erst zur Laufzeit entsteht, passiert, zum Beispiel `$TOOL read …` oder `bash -c "$CMD"`. Der Schalter `--strict` ändert das (siehe [Strenger Modus](#strenger-modus)).
- Ein Skript in einer Datei (`bash ./skript.sh`) und ein Here-Document für eine Shell prüft der Hook nicht.
- Er prüft Zeichenketten von `bash -c`, `sh -c`, `zsh -c`, `eval` und `env -S` bis zur Tiefe 3. Eine tiefere Verschachtelung passiert.
- Das MCP-Werkzeug `run_with_credentials` ist kein Befehl von `Bash`. Der Hook prüft es nicht ({{ ref("D67") }}).
- Ein Alias oder eine Funktion mit anderem Namen erkennt er nicht.
- `which op`, `command -v op`, `type op` und `brew install 1password-cli` lesen kein Secret. Sie passieren.

## Voraussetzungen

- `a38` ist installiert ([a38 aus dem Release installieren](15-a38-aus-dem-release-installieren.md)).
- `a38` ist als MCP-Server angemeldet ([a38 als MCP-Server anmelden](22-mcp-server-anmelden.md)).
- Der Skill ist installiert ([Skill für Agents installieren](20-skill-installieren.md)).
- Der Agent ist Claude Code.

## Den Block ausgeben

`a38 hook op-guard --print-config` gibt den Block für die Datei `settings.json` aus.
Der Befehl ändert keine Datei.

```sh
a38 hook op-guard --print-config
```

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "a38",
            "args": [
              "hook",
              "op-guard"
            ],
            "timeout": 10
          }
        ]
      }
    ]
  }
}
```

Der Block nutzt die Form von Claude Code
([Hooks reference](https://code.claude.com/docs/en/hooks), Stand 2026-11-12):

- `PreToolUse` läuft vor dem Aufruf eines Werkzeugs und kann ihn blockieren.
- `"matcher": "Bash"` wählt nur das Werkzeug `Bash`.
- `args` startet `a38` ohne Shell (Exec-Form). Claude Code sucht `command` im `PATH`.
- `timeout` ist in Sekunden. Der Hook braucht Millisekunden. Läuft die Zeit ab, lässt Claude Code den Befehl durch.

## Projekt oder Benutzer

**Empfehlung:** Trage den Hook im Benutzerbereich ein (`~/.claude/settings.json`).
Dann gilt er für alle deine Projekte ({{ ref("D67") }}).

Claude Code liest Hooks aus mehreren Dateien und führt sie zusammen
([Hooks reference](https://code.claude.com/docs/en/hooks), Tabelle der Orte von Hooks).

| Datei | Gilt für | Variante |
|---|---|---|
| `.claude/settings.json` im Projekt | dieses Projekt; die Datei kann ins Repository | `--scope project` (Standard): Name `a38` |
| `.claude/settings.local.json` im Projekt | dieses Projekt, nur für dich | `--scope project` |
| `~/.claude/settings.json` | alle deine Projekte | `--scope user`: absoluter Pfad des laufenden `a38` |

Der Unterschied ist nur das Feld `command`.

- **Name `a38`:** Die Datei passt auf jedem Rechner, auf dem `a38` im `PATH` liegt. Ein anderes Programm mit dem Namen `a38` weiter vorn im `PATH` liefe an seiner Stelle. Fehlt `a38` im `PATH`, startet der Hook nicht, und Claude Code lässt jeden Befehl durch.
- **Absoluter Pfad:** Der Hook hängt nicht vom `PATH` des Agent-Werkzeugs ab. Die Datei passt nur auf diesem Rechner. Nach einem Umzug von `a38` gibst du den Block neu aus.

```sh
a38 hook op-guard --print-config --scope user
```

## Für alle Projekte einrichten

1. Gib den Block mit dem absoluten Pfad aus:

    ```sh
    a38 hook op-guard --print-config --scope user
    ```

2. Öffne `~/.claude/settings.json` in einem Editor.
3. Füge den Block von Hand ein, wie im nächsten Abschnitt unter Schritt 3 beschrieben.
   Überschreibe die Datei nicht mit `>`: Sie enthält meist schon andere Einstellungen.
4. Steht der Hook schon in `.claude/settings.json` eines Projekts, entferne ihn dort.
   Sonst läuft er in diesem Projekt zweimal.
5. Starte die Sitzungen von Claude Code neu.
6. Gib in Claude Code `/hooks` ein. Der Hook steht unter `PreToolUse`.

Kein Befehl von a38 ändert `~/.claude/settings.json`. Die Datei gehört dir ({{ ref("D58") }}).

## Den Block eintragen

1. Öffne die gewählte Datei, zum Beispiel `.claude/settings.json` im Projekt.
2. Fehlt die Datei, lege sie mit dem ausgegebenen Block an:

    ```sh
    mkdir -p .claude
    a38 hook op-guard --print-config > .claude/settings.json
    ```

    Der Befehl überschreibt eine vorhandene Datei. Nutze ihn nur, wenn es die Datei noch nicht gibt.

3. Hat die Datei schon Inhalt, füge den Block von Hand ein:
    - Fehlt der Schlüssel `hooks`, kopiere ihn mit seinem Inhalt auf die oberste Ebene.
    - Gibt es `hooks.PreToolUse` schon, hänge die Gruppe mit `"matcher": "Bash"` an die Liste an.
4. Starte die Sitzung von Claude Code neu.
5. Gib in Claude Code `/hooks` ein. Der Hook steht unter `PreToolUse`.

Claude Code führt einen Hook aus einer Datei erst aus, wenn du dem Ordner vertraust.
Das gilt in einer interaktiven Sitzung auch für `~/.claude/settings.json`
([Hooks reference](https://code.claude.com/docs/en/hooks), Abschnitt „Workspace trust“).

## Strenger Modus

Ohne Schalter lässt der Hook einen Befehl durch, den er nicht prüfen kann.
Mit `--strict` blockiert er einen solchen Befehl, wenn der Text des Befehls `op` als eigenes Wort enthält ({{ ref("D67") }}).
Das gilt in drei Fällen:

- Der Parser versteht den Befehl nicht.
- Ein Kommandowort entsteht erst zur Laufzeit, zum Beispiel `TOOL=op; $TOOL read …` oder `bash -c "$CMD"`.
- Die Schachtelung von `bash -c` und `eval` ist tiefer als 3.

Ein eigenes Wort heißt: Vor und nach `op` steht kein Buchstabe, keine Ziffer und keines der Zeichen `_`, `.` und `-`.
`top`, `opam`, `op.txt` und `op-check` enthalten `op` nicht als eigenes Wort.
`TOOL=op` und `op://…` enthalten es.

Eine Eingabe, die kein JSON ist, die größer als 1 MiB ist oder die keinen Befehl hat, passiert auch mit `--strict`.
Das ist ein Fehler im Aufruf, kein Zweifel am Befehl.

`--strict` hilft, wenn ein Agent nach einer Blockade `op` in einer Variable versteckt.
Ein falscher Treffer hält aber die Arbeit des Agents an.
Der Standard bleibt deshalb ohne Schalter.

Den Block mit dem Schalter gibt `--print-config --strict` aus:

```sh
a38 hook op-guard --print-config --scope user --strict
```

Im Block steht dann `"args": ["hook", "op-guard", "--strict"]`.

## Von Hand prüfen

Du prüfst den Hook ohne Agent.
Du gibst ihm eine Eingabe in der Form von Claude Code auf stdin.
Der Hook führt den Befehl der Eingabe nie aus.

```sh
printf '%s' '{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"op read op://beispiel.app.k8s-dev/datenbank-dev/password"}}' \
  | a38 hook op-guard
echo "exit=$?"
```

Erwartet: die Meldung auf stderr und `exit=2`.

```sh
printf '%s' '{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git commit -m \"fix op\""}}' \
  | a38 hook op-guard
echo "exit=$?"
```

Erwartet: keine Ausgabe und `exit=0`.

Den strengen Modus prüfst du mit einem Kommandowort aus einer Variable:

```sh
printf '%s' '{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"TOOL=op; $TOOL whoami"}}' \
  | a38 hook op-guard --strict
echo "exit=$?"
```

Erwartet: die Meldung auf stderr und `exit=2`.
Ohne `--strict` passiert derselbe Befehl mit `exit=0`.
Auf stderr steht dann nur der Grund: `allowed without inspection: a command word is built at run time; not inspected`.

Vollständige Beispiele der Eingabe liegen im Repository unter `internal/opguard/testdata/`.

## Im Agent prüfen

1. Starte Claude Code im Projekt.
2. Bitte den Agent: „Lies mit op das Passwort aus op://beispiel.app.k8s-dev/datenbank-dev/password.“
3. Claude Code zeigt `PreToolUse:Bash hook error` mit der Meldung. Der Befehl läuft nicht.
4. Der Agent nutzt danach `run_with_credentials` oder `request_credentials`. Das Popup von a38 erscheint.

Bleibt der Agent bei `op`, prüfe mit `/hooks`, ob der Hook geladen ist.
Mit `claude --debug` schreibt Claude Code jeden Lauf eines Hooks in das Debug-Log
([Hooks reference](https://code.claude.com/docs/en/hooks), Abschnitt „Debug hooks“).

## Exit-Codes

| Code | Bedeutung | Wirkung in Claude Code |
|---|---|---|
| 0 | durchlassen; keine Ausgabe | Der Befehl geht in die normale Prüfung der Rechte. |
| 2 | blockieren; Meldung auf stderr | Der Befehl läuft nicht. Der Agent liest die Meldung. |
| 1 | Fehler im Aufruf, zum Beispiel ein unbekannter Schalter | Der Befehl läuft. Claude Code zeigt einen Hinweis auf den Fehler. |

Der Hook nutzt für einen Fehler im Aufruf den Code 1 und nicht 2.
Ein falscher Eintrag in der Datei blockiert so nicht jeden Befehl.
Setze in der Datei nicht `"onFailure": "block"`: Dann blockiert ein fehlendes `a38` jeden Befehl.

## Andere Agent-Werkzeuge

Der Vertrag ist schlicht: JSON auf stdin, Exit-Code 0 oder 2, Meldung auf stderr.
Jedes Werkzeug mit einem Hook vor einem Befehl kann ihn nutzen, wenn es das Feld `tool_input.command` sendet.
Der Hook prüft heute nur den Namen `Bash`.

Geprüft ist die Gemini CLI ([Hooks reference](https://geminicli.com/docs/hooks/reference/), Stand 2026-11-12).
Ihr Ereignis `BeforeTool` sendet `tool_name` und `tool_input` und behandelt Exit-Code 2 mit stderr als Ablehnung.
Ihr Werkzeug für die Shell heißt aber `run_shell_command`.
Der Hook lässt es deshalb heute durch.
Die Unterstützung ist eine Option für später.

## Entfernen

1. Öffne die Datei, in die du den Block eingetragen hast.
2. Entferne die Gruppe mit `"matcher": "Bash"` und `"args": ["hook", "op-guard"]` aus `hooks.PreToolUse`.
3. Ist `hooks.PreToolUse` danach leer, entferne den Schlüssel.
4. Starte die Sitzung von Claude Code neu.

Alle Hooks schaltest du mit `"disableAllHooks": true` in der Datei ab
([Hooks guide](https://code.claude.com/docs/en/hooks-guide)).

## Später

Ein Befehl, der den Block selbst in die Datei schreibt, fehlt mit Absicht.
Die Datei gehört dir.
Ein solcher Befehl ist ein möglicher späterer Schritt.
