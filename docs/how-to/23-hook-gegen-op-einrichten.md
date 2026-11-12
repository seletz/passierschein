---
title: Hook gegen den direkten Aufruf von op einrichten
description: "a38 hook op-guard in Claude Code eintragen, von Hand prüfen und wieder entfernen."
render_macros: true
---
# Hook gegen den direkten Aufruf von op einrichten

Ein Agent soll Zugangsdaten nur über a38 anfragen.
Der Skill sagt ihm das.
Der Hook `a38 hook op-guard` erinnert ihn daran, wenn er es vergisst ({{ ref("F16") }}, {{ ref("UC12") }}).
Diese Anleitung trägt den Hook in Claude Code ein.

## Was der Hook tut

Claude Code startet den Hook vor jedem Befehl des Werkzeugs `Bash`.
Der Hook liest den Befehl und entscheidet.

- Er **blockiert** einen Befehl, der die 1Password-CLI `op` startet. Der Befehl läuft nicht.
- Er blockiert auch einen Befehl, der das Token von a38 liest: die Token-Datei (`…/onepassword-token`) und den Eintrag `a38.onepassword` im Schlüsselbund mit `security find-generic-password -w`.
- Der Agent erhält eine kurze Meldung. Sie nennt nur einen Weg: das MCP-Werkzeug `request_credentials` des Servers `a38`.
- Jeden anderen Befehl **lässt er durch** und schreibt nichts.
- Andere Werkzeuge als `Bash` prüft er nicht.

Die Meldung an den Agent lautet:

```text
a38 blocked this command. This command runs the 1Password CLI "op". The command did not run.
Do not run "op" or any other password manager tool yourself.
Get credentials only through a38: call the MCP tool "request_credentials" of the MCP server "a38".
Put all credentials that the task needs into one request. Give each as a reference of the form op://vault/item/field, and give "task" and "reason".
If the request fails or the human denies it, stop the work that needs the credential and tell the human the error code.
Do not look for another way to get the credential.
```

## Was der Hook nicht tut

!!! warning "Ein Hinweis, kein Schutz"
    Der Hook ist ein Hinweis für einen Agent, der sich an die Regeln hält ({{ ref("D15") }}).
    Er verhindert Versehen.
    Er hält keinen Agent auf, der a38 umgehen will.
    Der Agent läuft unter deinem Benutzer.
    Er kann ein Secret auf vielen anderen Wegen lesen, zum Beispiel über ein Skript, eine Variable oder ein anderes Programm ({{ ref("RI5") }}).
    Den Schutz liefert erst die VM ohne 1Password (Stufe 5).

- **Im Zweifel lässt der Hook durch.** Eine Eingabe, die kein JSON ist, die größer als 1 MiB ist oder deren Befehl kein Parser versteht, passiert. Der Rest von a38 schließt im Zweifel. Der Hook nicht: Er ist keine Grenze, und ein Fehler im Hook darf die Arbeit des Agents nicht anhalten ({{ ref("D58") }}).
- Ein Befehl, dessen Kommandowort erst zur Laufzeit entsteht, passiert, zum Beispiel `$TOOL read …` oder `bash -c "$CMD"`.
- Ein Skript in einer Datei (`bash ./skript.sh`) und ein Here-Document für eine Shell prüft der Hook nicht.
- Er prüft Zeichenketten von `bash -c`, `sh -c`, `zsh -c`, `eval` und `env -S` bis zur Tiefe 3. Eine tiefere Verschachtelung passiert.
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

Vollständige Beispiele der Eingabe liegen im Repository unter `internal/opguard/testdata/`.

## Im Agent prüfen

1. Starte Claude Code im Projekt.
2. Bitte den Agent: „Lies mit op das Passwort aus op://beispiel.app.k8s-dev/datenbank-dev/password.“
3. Claude Code zeigt `PreToolUse:Bash hook error` mit der Meldung. Der Befehl läuft nicht.
4. Der Agent nutzt danach `request_credentials`. Das Popup von a38 erscheint.

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
