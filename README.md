# Passierschein

Passierschein ist ein Credential-Broker für Coding Agents. Das
Kommandozeilenwerkzeug heißt `a38`.

## Stand

**Das aktuelle Release ist `v0.2.0` (2026-11-15).** Es liefert
`Passierschein.app` mit `a38` im Bundle als ZIP für macOS, signiert mit der
Developer ID, notarisiert und mit angeheftetem Ticket. Dazu kommen tar-Archive
mit dem einzelnen `a38` für macOS und Linux, SBOMs und Prüfsummen. Das erste Release
`v0.1.0` (2026-11-12) arbeitet nicht mit 1Password: Die gehärtete Laufzeit
(Hardened Runtime) blockiert das 1Password-SDK. `v0.2.0` behebt das. Nutze
`v0.1.0` nicht. Die Anleitung
[a38 aus dem Release installieren](docs/how-to/15-a38-aus-dem-release-installieren.md)
zeigt den Weg. Die Doku steht unter <https://seletz.github.io/passierschein/>
und zeigt den Stand von `develop`.
Agenten lesen die Doku als Markdown: <https://seletz.github.io/passierschein/llms.txt> listet alle Seiten, `llms-full.txt` enthält alle Seiten in einer Datei.

**Stufe 1 ist fertig.** Sie läuft lokal mit einem Fake-Backend und einer
gefälschten Freigabe.

**Stufe 2 ist gebaut, abgenommen und im Praxistest bestanden.** Die
Freigabe-App „Passierschein“ (Swift, Ordner `app/`) zeigt das Popup und fragt
Touch ID. Der Daemon startet sie mit `a38 serve --ui app`. Der Ablauf über
echte Prozesse läuft mit der Debug-App und gefälschter Authentifizierung
(`mise run app:e2e`). Stefan hat am 2026-11-12 das echte Touch ID und die Sicht
unter OmniWM geprüft. Offen sind Vollbild, Wechsel des Arbeitsbereichs,
Zeitablauf und die Meldungen am Bildschirm. Die
Anleitung [Freigabe-App lokal ausprobieren](docs/how-to/40-freigabe-app-ausprobieren.md)
zeigt, wie du die App ohne Touch ID startest.

**Stufe 3 ist gebaut und im Praxistest bestanden.** Das 1Password-Backend
liest über das Go-SDK mit dem Token eines Service Accounts
(`a38 serve --backend 1password --ui app`, nur macOS). Der Integrationstest
gegen den Test-Vault und der Praxistest mit der echten App sind am
2026-11-12 bestanden. Das Token liegt im Schlüsselbund von macOS
(`a38 config onepassword-token set`); die Token-Datei bleibt als Ausweichweg für die Entwicklung.
Die Anleitung [1Password-Backend einrichten](docs/how-to/30-1password-backend-einrichten.md)
beschreibt Service Account, Schlüsselbund, Token-Datei und Test.

Eine Demo des Stands baust du mit einem Befehl auf und ab.
Die Anleitung [Demo vorführen](docs/how-to/50-demo-vorfuehren.md) enthält
den Ablaufplan mit Agent, Popup, Touch ID, Audit-Log und Störung.
Das Tutorial [Live-Demo](docs/tutorials/20-live-demo.md) zeigt den Ablauf Schritt für Schritt mit Bildern.

Was in Stufe 1 funktioniert:

- `a38 serve`: der Daemon mit Agent-Socket, Admin-Socket, Bündelung, Policy,
  Grants, Audit-Log und globaler Sperre bei einer Störung
- `a38 mcp`: der MCP-Server mit den Werkzeugen `request_credentials` und
  `grant_status`
- `a38 grants list` und `a38 grants revoke`: Grants ansehen und widerrufen
- `a38 audit list`, `a38 audit verify` und `a38 audit stats`: das Audit-Log
  lesen, die Hash-Kette prüfen und die Kennzahlen ausgeben
- `a38 tui`: Freigaben und Auslieferungen im Terminal auswerten (nur macOS,
  Entwurf zur Abnahme durch Stefan)
- `a38 doctor`, `a38 fault status` und `a38 fault clear`: Fehlersuche und
  Aufheben der Sperre
- `a38 config onepassword-token set`, `status` und `delete`: das Token des
  Service Accounts im Schlüsselbund von macOS (nur macOS)
- `a38 llm --skill`: die Anleitung für Agents
- `a38 hook op-guard`: der Hook gegen den direkten Aufruf von `op` (Stufe 4)
- Backend aus einer YAML-Datei (`--backend fake`) und Freigabe ohne Mensch
  (`--ui auto-approve`, `auto-deny`, `script`) für Tests und zum Lernen

**Das Release ist signiert und notarisiert.** Der Daemon prüft die Signatur
der App, wenn `a38` selbst mit der Developer ID signiert ist. Die Anleitung
[Release signieren und prüfen](docs/how-to/60-release-signieren-und-pruefen.md)
beschreibt für Maintainer die Secrets und die Prüfung eines Release.

**Stufe 4 ist gebaut, der Praxistest ist offen.** Der Hook `a38 hook op-guard`
blockiert im Agent-Werkzeug einen Befehl, der `op` startet, und verweist auf
`request_credentials`. Er ist ein Hinweis und kein Schutz. Die Anleitung
[Hook gegen den direkten Aufruf von op einrichten](docs/how-to/23-hook-gegen-op-einrichten.md)
zeigt den Eintrag in Claude Code.

Noch nicht vorhanden: `a38 service install`, der Betrieb mit Agents
in der VM und Homebrew (das folgt, wenn das Repo öffentlich ist).

28 von 41 Testfällen des Designs sind belegt, 3 teilweise, 10 zu bauen (exec-Weg für 0.3.0). Die App, der darwin-Code und der 1Password-Integrationstest laufen in einem eigenen macOS-Workflow ([CI](docs/reference/55-ci.md)). Die
Übersicht steht in der [Testabdeckung](docs/reference/90-testabdeckung.md).

## Zweck

- Ein Coding Agent fragt Zugangsdaten über einen MCP-Server an.
- Ein Daemon auf dem Mac bündelt die Anfragen und zeigt ein Popup mit Kontext.
- Ein Mensch gibt die Anfrage mit Touch ID frei oder lehnt sie ab.
- Eine Freigabe gilt für eine Sitzung und hat eine Laufzeit.
- Ein Audit-Log speichert jede Freigabe und jede Auslieferung.

Nur der Daemon hat Zugriff auf 1Password.

## Installation

Die Anleitung [a38 aus dem Release installieren](docs/how-to/15-a38-aus-dem-release-installieren.md)
beschreibt den Weg für macOS (App mit `a38`, Prüfung der Signatur, Token im
Schlüsselbund, Update) und für Linux (nur der Client `a38 mcp`). Mit `mise`
installierst du das einzelne `a38` aus dem tar-Archiv.

## Einstieg

Zum Bauen und Ausprobieren aus dem Quelltext: [mise](https://mise.jdx.dev/).

```sh
mise install        # Werkzeuge in den festgelegten Versionen
mise run build      # baut bin/a38
```

Das [Tutorial](docs/tutorials/10-erste-schritte.md) führt in wenigen Schritten
durch den ganzen Ablauf mit dem Fake-Backend. Die [How-tos](docs/how-to/index.md)
lösen einzelne Aufgaben. Das
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
