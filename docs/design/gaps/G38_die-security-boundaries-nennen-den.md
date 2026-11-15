---
render_macros: true
id: 'G38'
titel: 'Die Security Boundaries nennen den exec-Weg und die Auslieferung außerhalb von dev nicht'
bereich: 'Grenzen und Identität'
lauf: '2.0'
stand: '2026-11-15'
order: 8
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G38 — Die Security Boundaries nennen den exec-Weg und die Auslieferung außerhalb von dev nicht

{{ design_kopf(page) }}

Die Seite [Security Boundaries](../50-security-boundaries.md) stammt aus Designlauf 1. Sie nennt als Schutzziel für test und prod „Der Wert erreicht nie den Agent als MCP-Antwort“ und als Maßnahme die Ablehnung im MVP ({{ ref("D24") }}). Der exec-Weg ändert Maßnahmen und Klassen.

Nachzutragen nach der Bestätigung von {{ ref("D62") }}:

| Maßnahme | Klasse |
|---|---|
| Ein Wert außerhalb von dev steht nie in einer MCP-Antwort. | enforced ({{ ref("T35") }}) |
| Ein Grant in test gilt nur für das freigegebene Programm; prod braucht eine Freigabe je Kommando. | enforced ({{ ref("T36") }}) |
| Die Ausgabe des Kommandos enthält den exakten Wert nicht. | enforced in `a38 mcp`, Zone des Agents ({{ ref("T33") }}) |
| Der Wert erreicht den Agent beim exec-Weg nicht. | **not-provided**: Der Agent wählt das Kommando ({{ ref("RI13") }}). |
| Ein verändertes `a38 mcp` gibt keinen Wert an den Agent. | **not-provided**: `a38 mcp` liegt in der Zone des Agents ({{ ref("D2") }}). |

Offene Frage: Ändert der Nachtrag die Seite selbst, oder entsteht eine neue Seite für Lauf 2?

**Quelle:** [Issue 91](https://github.com/seletz/passierschein/issues/91).
