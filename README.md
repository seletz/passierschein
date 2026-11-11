# Passierschein

Passierschein ist ein Credential-Broker für Coding Agents. Das
Kommandozeilenwerkzeug heißt `a38`.

## Stand

**Stufe 1 ist fertig.** Sie läuft lokal mit einem Fake-Backend und einer
gefälschten Freigabe. Es gibt noch kein 1Password-Backend und kein Popup mit
Touch ID. Beides gehört zu Stufe 2.

Was in Stufe 1 funktioniert:

- `a38 serve`: der Daemon mit Agent-Socket, Admin-Socket, Bündelung, Policy,
  Grants, Audit-Log und globaler Sperre bei einer Störung
- `a38 mcp`: der MCP-Server mit den Werkzeugen `request_credentials` und
  `grant_status`
- `a38 grants list` und `a38 grants revoke`: Grants ansehen und widerrufen
- `a38 audit list` und `a38 audit verify`: das Audit-Log lesen und die
  Hash-Kette prüfen
- `a38 doctor`, `a38 fault status` und `a38 fault clear`: Fehlersuche und
  Aufheben der Sperre
- `a38 llm --skill`: die Anleitung für Agents
- Backend aus einer YAML-Datei (`--backend fake`) und Freigabe ohne Mensch
  (`--ui auto-approve`, `auto-deny`, `script`) für Tests und zum Lernen

Noch nicht vorhanden: das 1Password-Backend, die Freigabe-App in Swift,
`a38 tui`, `a38 service install`, der Hook gegen `op`, der Betrieb mit Agents
in der VM und die Auslieferung außerhalb von dev.

22 von 28 Testfällen des Designs sind durch automatische Tests belegt. Die
Übersicht steht in der [Testabdeckung](docs/reference/90-testabdeckung.md).

## Zweck

- Ein Coding Agent fragt Zugangsdaten über einen MCP-Server an.
- Ein Daemon auf dem Mac bündelt die Anfragen und zeigt ein Popup mit Kontext.
- Ein Mensch gibt die Anfrage mit Touch ID frei oder lehnt sie ab.
- Eine Freigabe gilt für eine Sitzung und hat eine Laufzeit.
- Ein Audit-Log speichert jede Freigabe und jede Auslieferung.

Nur der Daemon hat Zugriff auf 1Password.

## Einstieg

Voraussetzung: [mise](https://mise.jdx.dev/).

```sh
mise install        # Werkzeuge in den festgelegten Versionen
mise run build      # baut bin/a38
```

Das [Tutorial](docs/tutorials/10-erste-schritte.md) führt in wenigen Schritten
durch den ganzen Ablauf mit dem Fake-Backend. Das
[Design-Set](docs/design/index.md) ist die Spezifikation.

## Entwicklung

```sh
mise run hooks:install  # Git-Hook für den Secret-Scan, einmal pro Klon
mise run check          # alle Checks der CI
mise run docs:serve     # Doku lokal ansehen
```

Alle Tasks: `mise tasks`. Die Doku liegt unter `docs/` und beschreibt Tasks,
Release und Konventionen.

## Regeln in Kürze

- Design vor Code. Das Design-Set unter `docs/design/` entsteht mit
  `dg-register`. Lauf 1 ist eingefroren.
- Jede Änderung entsteht in einem Worktree und kommt über einen Pull Request
  nach `develop`.
- Doku: Deutsch nach DIN 8579. Code: Englisch nach ASD-STE100.
- Testdaten sind neutral. Secrets stehen nie im Repo.

## Lizenz

MIT, siehe [LICENSE](LICENSE).
