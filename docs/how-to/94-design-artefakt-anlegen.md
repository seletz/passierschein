---
title: Design-Artefakt anlegen
description: "Ein Artefakt des Design-Sets mit dg-register anlegen, ändern und prüfen."
---
# Design-Artefakt anlegen

Das Design-Set liegt in `docs/design/`. Das Werkzeug `dg-register` vergibt
die Nummern, legt Artefakte an und prüft das Set. Niemand vergibt eine
Nummer von Hand.

## Voraussetzung

`dg-register` ist installiert. Die verlangte Version steht in
`design-sets.yaml` unter `cli`.

## Artefakt anlegen

```sh
dg-register neu --familie D --bezug '#2' --zweck speicherort \
  --titel 'Das Audit-Log liegt in einer SQLite-Datei' \
  --bereich 'Audit' --format json
```

- `--familie` nennt die Familie: `UC`, `D`, `F`, `T`, `G` oder `RI`.
- `--bezug` nennt das Issue. Die Anführungszeichen sind nötig.
- `--zweck` unterscheidet mehrere Artefakte desselben Issues.
- `--bereich` nennt einen Bereich aus der `index.md` der Familie.

Das Werkzeug legt zwei Dateien an: das Artefakt und die Vergabedatei unter
`docs/design/register/`. Committe beide Dateien im selben Pull Request.

Schreibe danach den Text des Artefakts. Das Werkzeug ändert den Text nie.

## Status ändern

Den Status ändert nur das Werkzeug:

```sh
dg-register set G1 status=geschlossen beleg.durch=D4 beleg.wer=Stefan --format json
```

## Auf ein Artefakt verweisen

Im Text steht nie eine bloße ID. Ein Verweis nutzt das Makro `ref`, zum
Beispiel `ref("D1")` in doppelten geschweiften Klammern.

## Set prüfen

```sh
mise run design:check
```

Der Task läuft nur lokal. Der Grund steht in der
[Referenz der Tasks](../reference/40-tasks.md).
