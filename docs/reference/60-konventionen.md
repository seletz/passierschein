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
- Stefan gibt jeden Merge frei.

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

## Design vor Code

Das Design entsteht vor dem Code. Use Cases, Features, Entscheidungen,
Testfälle, Gaps und Risiken entstehen nur mit `dg-register`. Ein Spike klärt
eine offene Frage mit einem Experiment. Der Code eines Spikes ist
Wegwerfcode.
