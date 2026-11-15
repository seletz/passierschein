---
title: "Security Boundaries, Nachtrag Lauf 2"
description: "Nachtrag zu den Security Boundaries: der exec-Weg und die Auslieferung außerhalb von dev ab Version 0.3.0."
render_macros: true
weight: 6
tags:
  - Thema/Sicherheit
---
# Security Boundaries, Nachtrag Lauf 2

Diese Seite ergänzt [Security Boundaries](50-security-boundaries.md). Die Seite
aus Designlauf 1 ist eingefroren. Ihr Text ändert sich nicht. Dieser Nachtrag
gilt an den genannten Stellen vor ihr. Er trägt keine eigene Nummer. Die
Klassen sind dieselben: **enforced**, **inherited**, **not-provided**.

**Stand:** Designlauf 2. Stefan hat die Entscheidungen {{ ref("D60") }} bis
{{ ref("D68") }} am 2026-11-15 bestätigt. Der Nachtrag schließt
{{ ref("G38") }}.

## Was dieser Nachtrag ersetzt

| Stelle auf der Seite aus Lauf 1 | Neue Aussage |
|---|---|
| Schutzgut „Secret-Werte außerhalb von dev“: Der Wert erreicht nie den Agent als MCP-Antwort. | Das Schutzziel bleibt. Der Wert erreicht den Agent außerhalb von dev auch nicht als Datei. Der exec-Weg liefert ihn an ein Kommando, nicht an den Agent ({{ ref("D62") }}). |
| B1: „Eine Anfrage außerhalb von dev liefert im MVP keinen Wert“ ({{ ref("D24") }}). | Die Entscheidung {{ ref("D24") }} ist ersetzt. Außerhalb von dev liefert `a38` über den exec-Weg aus (Zeilen 1 und 2 der nächsten Tabelle). |
| Die Datei auf einem tmpfs ({{ ref("D10") }}) | Ersetzt. `a38` schreibt keinen Wert in eine Datei ({{ ref("D62") }}). |

## Maßnahmen des exec-Wegs

Der Weg gilt für den lokalen Einsatz ({{ ref("D60") }}). Die Maßnahmen gehören
zur Grenze B1 Agent-Socket.

| Maßnahme | Klasse | Mechanismus | Nachweis |
|---|---|---|---|
| Ein Wert außerhalb von dev steht nie in einer MCP-Antwort. | enforced | Code `exec_required`, keine Freigabeanfrage; die Konfiguration ändert die Regel nicht ({{ ref("D62") }}) | {{ ref("T35") }} |
| Ein Grant in test gilt nur für das freigegebene Programm. Prod braucht eine Freigabe je Kommando. | enforced | Bindung `program` in test, Bindung `command` und Laufzeit `0s` in prod ({{ ref("D63") }}) | {{ ref("T36") }} |
| Die Ausgabe des Kommandos enthält den exakten Wert nicht. | enforced in `a38 mcp`, Zone des Agents | Maskierung von stdout und stderr ({{ ref("D64") }}) | {{ ref("T33") }} |
| Der Wert erreicht den Agent beim exec-Weg nicht. | **not-provided** | Der Agent wählt das Kommando. Ein Kommando kann den Wert kodiert ausgeben ({{ ref("RI13") }}). | – |
| Ein verändertes `a38 mcp` gibt keinen Wert an den Agent. | **not-provided** | `a38 mcp` liegt in der Zone des Agents ({{ ref("D2") }}). | – |

## Was der exec-Weg leistet

Der exec-Weg verhindert im Normalfall, dass der Wert in den Kontext des Modells
und in das Transkript fließt. Das ist die Ursache von {{ ref("RI1") }}. Er
schützt nicht gegen einen gekaperten Agent. Lokal liest ein Prozess desselben
Benutzers die Umgebung des Kommandos ({{ ref("RI5") }}). Der Maßstab ist der
direkte Aufruf von `op` ({{ ref("D60") }}): Der exec-Weg bringt eine Freigabe
und einen Eintrag im Audit-Log, und der Wert steht im Normalfall nicht im
Kontext.

Der Weg in der VM ist offen ({{ ref("G35") }}).

**Quelle:** [Issue 91](https://github.com/seletz/passierschein/issues/91),
[Issue 102](https://github.com/seletz/passierschein/issues/102).
