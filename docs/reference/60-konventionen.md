---
title: Konventionen
description: "Regeln für Sprache, Commits, Branches, Testdaten und Secrets."
---
# Konventionen

## Sprache

| Ort | Sprache | Regelwerk |
|---|---|---|
| Doku unter `docs/`, `README.md` | Deutsch | DIN 8579:2022-07 |
| Code: Bezeichner, Kommentare, Fehlermeldungen | Englisch | ASD-STE100 |
| Commits, Pull Requests, Issues | Commits englisch, Issues und Pull Requests deutsch | Conventional Commits |

DIN 8579 beschreibt übersetzungsgerechtes Schreiben. Die wichtigsten Regeln:

- Ein Satz enthält eine Aussage.
- Die Sätze sind kurz und vollständig.
- Die Sätze stehen im Aktiv und im Präsens.
- Ein Begriff bezeichnet immer dieselbe Sache.
- Eine Abkürzung ist bei der ersten Nennung erklärt.
- Aufzählungen stehen als Liste.
- Füllwörter und Redewendungen fehlen.

ASD-STE100 ist Simplified Technical English. Die wichtigsten Regeln:

- Ein Wort hat eine Bedeutung.
- Die Sätze sind kurz und stehen im Aktiv.
- Eine Anweisung enthält einen Schritt.

## Branches und Pull Requests

- Der Standard-Branch ist `develop`.
- Jede Änderung entsteht in einem Worktree auf einem eigenen Branch.
- Der Branchname hat die Form `<art>/<issue>-<kurzname>`, zum Beispiel
  `feat/12-fake-backend`.
- Eine Änderung kommt nur über einen Pull Request nach `develop`.
- Der Koordinator prüft jeden Pull Request und merged ihn danach selbst
  (Stefan, 2026-11-11). Die Prüfung umfasst die CI, die Tests und den Diff.
- Ein Pull Request mit einer Auswahl oder einer Abnahme für Stefan bleibt
  Entwurf. Das gilt für jede Entscheidung über die Oberfläche. Er ist Stefan
  zugewiesen. Erst nach der Freigabe in der Diskussion wird er bereit zum
  Merge.
- Der Titel eines Pull Requests nennt den Endstand. GitHub übernimmt ihn als
  Nachricht des Merge-Commits.

### Pflichtteile eines Pull Requests

Die Vorlage `.github/pull_request_template.md` fragt diese Teile ab:

| Teil | Inhalt |
|---|---|
| Features | die Features des Design-Sets, die der Pull Request umsetzt |
| Use Cases | die Use Cases, die der Pull Request berührt |
| Testfälle | die umgesetzten Testfälle mit dem Namen der Testfunktion |
| Screenshots | mindestens ein Bild des Ergebnisses |

- Ein Test nennt seinen Testfall im Namen oder im Kommentar, zum Beispiel
  `// T2: ...`.
- Ein Pull Request ohne Bezug zum Design-Set schreibt „keine“.
- Ein Screenshot einer Ausgabe der Kommandozeile entsteht mit
  `mise run pr:screenshot -- bild.png -- <befehl>`.
- Die Bilder liegen auf dem Branch `review-assets`. Niemand merged diesen
  Branch. `mise run pr:screenshots:upload -- <nr> bild.png` legt sie ab und
  gibt die Zeilen für den Text des Pull Requests aus.
- Ein Screenshot zeigt nie einen Secret-Wert.

## Commits

Commits folgen [Conventional Commits](https://www.conventionalcommits.org/de/):
`<art>(<bereich>): <beschreibung>`.

| Art | Bedeutung |
|---|---|
| `feat` | neue Funktion |
| `fix` | Fehlerbehebung |
| `docs` | Doku und Design-Set |
| `test` | Tests |
| `ci` | Workflows |
| `chore` | Sonstiges, zum Beispiel Abhängigkeiten |

Der Text des Release entsteht aus diesen Commits.

## Testdaten

Testdaten sind neutral:

- keine echten Namen von Kunden, Firmen oder Personen,
- keine echten Secrets,
- keine echten Namen von Vaults oder Items.

Beispiele: Vault `beispiel.app.k8s-dev`, Item `datenbank-dev`, Projekt
`beispielprojekt`.

## Secrets

- Ein Secret-Wert steht nie in einem Log, in einer Fehlermeldung oder in
  einer Testausgabe.
- gitleaks prüft jede Änderung vor dem Commit und in der CI.

## Design nach dem Freeze

- Designlauf 1 ist seit dem 2026-11-11 eingefroren. Der Text seiner
  Artefakte ändert sich nicht mehr.
- Findet die Entwicklung einen Fehler im Design, entsteht ein neues Artefakt
  in Lauf 2. Eine ersetzte Entscheidung erhält den Status „ersetzt“.
- Der Status eines Gaps, eines Risikos und eines Testfalls ändert sich
  weiter mit `dg-register set`.
- Eine Änderung an einer abgenommenen Entscheidung braucht die Freigabe von
  Stefan. Der Pull Request bleibt bis dahin Entwurf.

## Design vor Code

Das Design entsteht vor dem Code. Use Cases, Features, Entscheidungen,
Testfälle, Gaps und Risiken entstehen nur mit `dg-register`. Ein Spike klärt
eine offene Frage mit einem Experiment. Der Code eines Spikes ist
Wegwerfcode.
