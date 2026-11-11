---
title: Design
description: "Das Design-Set von Passierschein: Use Cases, Entscheidungen, Features, Testfälle, Gaps und Risiken."
icon: lucide/drafting-compass
weight: 50
---
# Design

Das Design-Set beschreibt, was Passierschein leistet und warum. Es entsteht
vor dem Code. Das Werkzeug `dg-register` vergibt die Nummern und prüft das
Set.

**Stand:** Designlauf 1 „Design bis MVP“ ist offen
([Issue 2](https://github.com/seletz/passierschein/issues/2)). Das Set hat
noch keine Artefakte.

## Familien

- [Use Cases](use-cases/index.md): Ablauf je Akteur.
- [Entscheidungen](decisions/index.md): gültige, ersetzte und zurückgezogene
  Entscheidungen.
- [Features](features/index.md): Funktionen, je auf Entscheidungen
  abgebildet.
- [Testfälle](tests/index.md): Tests, die eine Eigenschaft des Designs
  belegen.
- [Gaps](gaps/index.md): offene Fragen.
- [Risiken](risks/index.md): bekannte Risiken.

Die Läufe stehen in `docs/design/laeufe.yaml`. Die Vergabedateien der
Nummern stehen unter `docs/design/register/`.

## Regeln

- Eine Nummer entsteht nur über `dg-register reserve` oder `dg-register neu`.
- Den Status ändert nur `dg-register set`.
- Ein Verweis auf ein Artefakt nutzt das Makro `ref`.

Die Schritte stehen in der Anleitung
[Design-Artefakt anlegen](../how-to/94-design-artefakt-anlegen.md).
