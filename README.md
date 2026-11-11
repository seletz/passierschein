# Passierschein

Passierschein ist ein Credential-Broker für Coding Agents. Das
Kommandozeilenwerkzeug heißt `a38`.

**Stand:** Designphase. Das Repo enthält das Grundgerüst und das Design-Set.
Es enthält noch keine Funktionen.

## Zweck

- Ein Coding Agent fragt Zugangsdaten über einen MCP-Server an.
- Ein Daemon auf dem Mac bündelt die Anfragen und zeigt ein Popup mit Kontext.
- Ein Mensch gibt die Anfrage mit Touch ID frei oder lehnt sie ab.
- Eine Freigabe gilt für eine Sitzung und hat eine Laufzeit.
- Ein Audit-Log speichert jede Freigabe und jede Auslieferung.

Nur der Daemon hat Zugriff auf 1Password.

## Entwicklung

Voraussetzung: [mise](https://mise.jdx.dev/).

```sh
mise install            # Werkzeuge in den festgelegten Versionen
mise run hooks:install  # Git-Hook für den Secret-Scan, einmal pro Klon
mise run check          # alle Checks der CI
mise run docs:serve     # Doku lokal ansehen
```

Alle Tasks: `mise tasks`. Die Doku liegt unter `docs/` und beschreibt Tasks,
Release und Konventionen.

## Regeln in Kürze

- Design vor Code. Das Design-Set unter `docs/design/` entsteht mit
  `dg-register`.
- Jede Änderung entsteht in einem Worktree und kommt über einen Pull Request
  nach `develop`.
- Doku: Deutsch nach DIN 8579. Code: Englisch nach ASD-STE100.
- Testdaten sind neutral. Secrets stehen nie im Repo.

## Lizenz

MIT, siehe [LICENSE](LICENSE).
