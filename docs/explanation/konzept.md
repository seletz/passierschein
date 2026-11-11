---
title: Konzept
description: "Ausgangslage, Ziele, Bestandteile und Stufenplan von Passierschein."
---
# Konzept

Diese Seite fasst die Idee von Passierschein zusammen. Die verbindlichen
Aussagen stehen im [Design-Set](../design/index.md).

## Ausgangslage

Viele Coding Agents laufen direkt auf dem Mac unter der Identität des
Menschen. Die Agents holen Zugangsdaten aus 1Password. Der Dialog von Touch ID
nennt weder den Agent noch die Aufgabe noch die Zugangsdaten. Der Mensch
bestätigt deshalb, ohne zu prüfen.

## Ziele

- Die Coding Agents laufen in einer Linux-VM. Eine VM ist eine virtuelle
  Maschine.
- Die VM enthält keine Werkzeuge von 1Password.
- Jede Anfrage nach Zugangsdaten ist nachvollziehbar: Das Popup zeigt den
  Kontext, der Daemon bündelt die Anfragen, das Audit-Log speichert jede
  Freigabe.
- Die Agents haben eine eigene Identität für Git und für den Cluster.

## Bestandteile

| Bestandteil | Ort | Aufgabe |
|---|---|---|
| `a38 serve` | Mac | Daemon. Hält als einziger Bestandteil den Zugriff auf 1Password. Stellt einen Unix-Socket bereit, bündelt Anfragen, vergibt Freigaben und schreibt das Audit-Log. |
| Freigabe-App | Mac | Zeigt das Popup und fragt Touch ID ab. Sieht keine Secrets. |
| `a38 mcp` | VM oder Mac | MCP-Server für die Agents. Spricht nur mit dem Unix-Socket. |
| `a38 tui` | Mac | Wertet die Freigaben aus. |
| `a38 llm --skill` | VM oder Mac | Gibt die Anleitung für Agents aus. |

MCP ist das Model Context Protocol. TUI ist eine textbasierte
Benutzeroberfläche im Terminal.

## Ablauf

1. Der Agent fragt über MCP an. Die Angaben des Agents gelten als
   unbestätigt.
2. Der Daemon sammelt die Anfragen einer Sitzung für einige Sekunden.
3. Das Popup zeigt oben die Fakten, die der Daemon geprüft hat. Das Popup
   zeigt unten die Angaben des Agents.
4. Der Mensch wählt die Zugangsdaten aus und bestätigt einmal mit Touch ID.
5. Der Daemon vergibt eine Freigabe für die Sitzung. Die Freigabe hat eine
   Laufzeit.
6. Der Daemon liefert die Zugangsdaten aus und schreibt das Audit-Log.

## Stufenplan

| Stufe | Inhalt |
|---|---|
| 0 | Design bis zum MVP, Spikes für offene Fragen |
| 1 | Lokal mit gefälschtem Backend und gefälschter Freigabe |
| 2 | Freigabe-App in Swift |
| 3 | Echtes 1Password-Backend |
| 4 | Hook gegen den direkten Aufruf von `op` |
| 5 | Agents in der VM |

MVP bedeutet Minimum Viable Product: die kleinste nutzbare Fassung. Die
Stufen entsprechen den Meilensteinen in GitHub.
