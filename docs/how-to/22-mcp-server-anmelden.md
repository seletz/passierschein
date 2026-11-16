---
title: a38 als MCP-Server anmelden
description: "a38 mcp im Agent-Werkzeug anmelden und den Namen der Sitzung je Sitzung festlegen."
render_macros: true
---
# a38 als MCP-Server anmelden

Das Agent-Werkzeug startet `a38 mcp` als MCP-Server über stdio.
Der Server bietet die Werkzeuge `request_credentials`, `grant_status` und `run_with_credentials`.
`run_with_credentials` startet ein Kommando mit den Werten in dessen Umgebung und gibt nur das Ergebnis zurück.
Außerhalb von dev ist `run_with_credentials` der einzige Weg ({{ ref("D62") }}).
Diese Anleitung zeigt die Anmeldung in Claude Code.

## Voraussetzungen

- `a38` ist installiert und im `PATH` des Agent-Werkzeugs.
- Die Konfigurationsdatei von a38 existiert.
- `a38 serve` läuft auf dem Mac.
- Der Skill ist installiert ([Skill für Agents installieren](20-skill-installieren.md)).

## Name der Sitzung

Der Name der Sitzung ordnet eine Freigabeanfrage einer Sitzung zu.
Der Name steht in der Freigabeanfrage, im Audit-Log und in der TUI.
Wähle denselben Namen, den du in deinem Agent-Werkzeug siehst, zum Beispiel `beispiel-koordinator`.

`a38 mcp` liest den Namen aus zwei Quellen.

| Quelle | Vorrang |
|---|---|
| Argument `--session-name <name>` | höher |
| Umgebungsvariable `A38_SESSION_NAME` | niedriger |

Ohne beide Quellen hat die Sitzung keinen Namen.
Der Daemon prüft den Namen gegen die Regeln der Konfiguration.
Ein ungültiger Name endet mit dem Code `invalid_request`.
Zwei verbundene Sitzungen mit demselben Namen enden mit `session_name_conflict`.

## Anmelden in Claude Code

Die Syntax stammt aus der Dokumentation von Claude Code
([MCP](https://code.claude.com/docs/en/mcp), Stand 2026-11-11).

### Mit `claude mcp add`

Der Name steht fest in der Anmeldung:

```sh
claude mcp add --transport stdio a38 \
  -- a38 mcp --session-name beispiel-koordinator
```

Die Optionen von `claude mcp add` stehen vor dem Namen des Servers.
Nach `--` folgen Befehl und Argumente von `a38 mcp`.

Eine abweichende Konfigurationsdatei übergibst du mit `--config`:

```sh
claude mcp add --transport stdio a38 \
  -- a38 mcp --config /pfad/zu/config.yaml --session-name beispiel-koordinator
```

Ohne `--config` gilt `A38_CONFIG`, danach der Standardpfad.

### Mit `.mcp.json`

Lege die Datei `.mcp.json` im Projekt an.
Sie teilt die Anmeldung mit dem Team.
Der Name kommt dann je Sitzung aus der Umgebung:

```json
{
  "mcpServers": {
    "a38": {
      "command": "a38",
      "args": ["mcp"],
      "env": {
        "A38_SESSION_NAME": "${A38_SESSION_NAME}"
      }
    }
  }
}
```

Claude Code ersetzt `${A38_SESSION_NAME}` beim Start durch den Wert aus deiner Umgebung.
Ist die Variable nicht gesetzt, bleibt der Text `${A38_SESSION_NAME}` stehen.
Der Daemon lehnt diesen Namen ab, weil `$`, `{` und `}` nicht erlaubt sind.
Setze die Variable deshalb vor dem Start:

```sh
A38_SESSION_NAME=beispiel-koordinator claude
```

Mit `${A38_SESSION_NAME:-}` ist der Name ohne Variable leer.
Dann hat die Sitzung keinen Namen.

## Name des Agent-Werkzeugs

`a38 mcp` sendet auch den Namen des MCP-Clients an den Daemon.
Der Name stammt aus der Anfrage `initialize`.
Der Daemon zeigt ihn als unbestätigte Angabe des Agents.
Er braucht keine Einstellung.

Die Anleitung [Befehl mit Zugangsdaten ausführen lassen](25-befehl-mit-zugangsdaten-ausfuehren.md) beschreibt `run_with_credentials`.

## Verhalten ohne Daemon

`a38 mcp` verbindet sich beim ersten Aufruf eines Werkzeugs.
Ist `a38 serve` nicht erreichbar, endet jeder Aufruf mit dem Code `daemon_unavailable`.
`a38 mcp` beendet sich dabei nicht.
Es versucht den nächsten Aufruf erneut.
Eine neue Verbindung ist eine neue Sitzung.
Die Grants der alten Sitzung sind dann weg.

## Prüfen

1. Starte `a38 serve`.
2. Starte Claude Code mit gesetztem Namen.
3. Rufe `/mcp` auf. Der Server `a38` ist verbunden.
4. Bitte den Agent, `grant_status` aufzurufen. Die Antwort ist eine leere Liste.
