---
title: Design
description: "Das Design-Set von Passierschein: Security Boundaries, Use Cases, Entscheidungen, Features, Testfälle, Gaps und Risiken."
icon: lucide/drafting-compass
weight: 50
render_macros: true
---
# Design

Das Design-Set beschreibt, was Passierschein leistet und warum. Es entsteht
vor dem Code. Das Werkzeug `dg-register` vergibt die Nummern und prüft das
Set.

**Stand:** Designlauf 1 „Design bis MVP“, Teilstand 1.1. Stefan hat das
Design am 2026-11-11 im Review abgenommen. Der Lauf ist seit dem 2026-11-11
eingefroren ([Issue 2](https://github.com/seletz/passierschein/issues/2)).

**Eingefroren heißt:** Der Text eines Artefakts dieses Laufs ändert sich
nicht mehr. Findet die Entwicklung einen Fehler im Design, oder wirft Stefan
bei einer Abnahme eine Entscheidung um, entsteht in einem neuen Lauf eine
neue Entscheidung. Die alte Entscheidung erhält den Status „ersetzt“.

## Lesereihenfolge

1. [Security Boundaries](50-security-boundaries.md): Zonen, Grenzen und
   Maßnahmen. Diese Seite ist das Bedrohungsmodell.
2. [Use Cases](use-cases/index.md): Ablauf je Akteur. Der Hauptablauf ist
   {{ ref("UC1") }}.
3. [Entscheidungen](decisions/index.md)
4. [Features](features/index.md)
5. [Testfälle](tests/index.md)
6. [Gaps](gaps/index.md)
7. [Risiken](risks/index.md)

## Familien

| Familie | Bereich |
|---|---|
| UC | {{ ref("UC1") }}–{{ ref("UC14") }} |
| D | {{ ref("D1") }}–{{ ref("D49") }} |
| F | {{ ref("F1") }}–{{ ref("F20") }} |
| T | {{ ref("T1") }}–{{ ref("T28") }} |
| G | {{ ref("G1") }}–{{ ref("G20") }} |
| RI | {{ ref("RI1") }}–{{ ref("RI12") }} |

## Bestätigt und angenommen

Eine Entscheidung hat eine von zwei Herkünften:

- **Bestätigt:** {{ ref("D1") }} bis {{ ref("D22") }} stammen aus der
  Projektnotiz vom 2026-11-11. Stefan hat sie getroffen.
- **Im Review abgenommen:** {{ ref("D23") }} bis {{ ref("D47") }} sind
  Vorschläge dieses Designlaufs. Stefan hat sie am 2026-11-11 abgenommen,
  mit den Ausnahmen der nächsten Zeile.
- **Annahme:** Die Seite zeigt eine Annahme mit dem Zusatz „(Annahme)“.
  Offen ist {{ ref("D29") }} (bleibt Annahme bis zum Praxistest).
- **Im Review neu entstanden:** {{ ref("D48") }} (eine Störung des Daemons
  sperrt die Auslieferung global).
- **Im Review neu gefasst:** {{ ref("D12") }} und {{ ref("D47") }}. Die
  Konfiguration nennt die Vaults und ihre Umgebung ausdrücklich. Der Daemon
  vergleicht keine Namensmuster.
- **Aus der Projektnotiz, wieder offen:** {{ ref("D3") }} (Transport in die
  VM).

Drei Entscheidungen ruhen auf Spikes: {{ ref("D39") }} (1Password-SDK),
{{ ref("D31") }} und {{ ref("D45") }} (Freigabe-UI), {{ ref("D46") }}
(Signatur der Swift-App). Kein Spike hat 1Password oder die echte
Developer ID benutzt. Die offenen Praxistests stehen in {{ ref("G3") }},
{{ ref("G4") }} und {{ ref("G5") }}.

## Umfang des MVP

Das MVP umfasst die Stufen 1 bis 3 ({{ ref("D23") }}). Das Feld `stufe` an
Use Cases und Features nennt die Stufe des Stufenplans:

| Stufe | Inhalt | Meilenstein |
|---|---|---|
| 1 | lokal mit Fake-Backend und Fake-UI | M1 |
| 2 | Freigabe-App in Swift | M2 |
| 3 | echtes 1Password-Backend | M3 |
| 4 | Hook gegen den direkten Aufruf von `op` | M4 |
| 5 | Agents in der VM | M5 |

Das MVP isoliert nicht ({{ ref("RI5") }}). Es macht Freigaben
nachvollziehbar.

## Was mit Absicht fehlt

- **Szenarien** gibt es nicht. Das Set hat noch keine Durchläufe mit Daten,
  aus denen ein Szenario entsteht.
- **Release Scope** hat keine eigene Seite. Der Umfang steht im Feld `stufe`.

## Regeln

- Eine Nummer entsteht nur über `dg-register reserve` oder `dg-register neu`.
- Den Status ändert nur `dg-register set`.
- Ein Verweis auf ein Artefakt nutzt das Makro `ref`.

Die Schritte stehen in der Anleitung
[Design-Artefakt anlegen](../how-to/94-design-artefakt-anlegen.md). Die
Läufe stehen in `docs/design/laeufe.yaml`. Die Vergabedateien der Nummern
stehen unter `docs/design/register/`.
