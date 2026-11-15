---
render_macros: true
id: 'G40'
titel: 'Das Design nennt kein Ende der Prozessgruppe nach dem Kommando und keine Quelle für die Prüfung des Arbeitsverzeichnisses in a38 mcp'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-15'
order: 7
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G40 — Das Design nennt kein Ende der Prozessgruppe nach dem Kommando und keine Quelle für die Prüfung des Arbeitsverzeichnisses in a38 mcp

{{ design_kopf(page) }}

{{ ref("D64") }} nennt zwei Regeln für `a38 mcp`, die das Design nicht vollständig beschreibt.

- **Prozessgruppe nach dem Ende des Kommandos:** {{ ref("D64") }} beendet die Prozessgruppe nur am Ende der Zeitgrenze. Ein Kommando kann einen Kindprozess starten und selbst enden. Der Kindprozess erbt die Umgebung mit dem Wert. Er hält auch stdout oder stderr offen. `a38 mcp` wartet dann bis zur Zeitgrenze oder ohne Ende. Das Design sagt nicht, was mit diesem Kindprozess geschieht. Die Begründung von {{ ref("D64") }} will Prozesse verhindern, die mit dem Wert weiterlaufen.
- **Quelle für die Prüfung des Arbeitsverzeichnisses:** {{ ref("D64") }} sagt: `a38 mcp` prüft zuerst, ob das Arbeitsverzeichnis im Projekt der Sitzung liegt. Derselbe Text sagt: `a38 mcp` liest die Konfiguration nicht. Die Pfade der Projekte stehen aber nur in der Konfiguration ({{ ref("D49") }}). Die Antwort `welcome` nennt nur den Namen des Projekts.

**Vorschlag (umgesetzt in Issue 97, zu bestätigen durch Stefan):**

- Endet das Kommando und lebt noch ein Prozess seiner Prozessgruppe, sendet `a38 mcp` SIGTERM an die Gruppe und nach 5 Sekunden SIGKILL, wie am Ende der Zeitgrenze. Danach wartet `a38 mcp` höchstens 1 Sekunde auf das Ende der Ausgabe. Ein Prozess, der die Gruppe verlassen hat, bleibt eine Grenze ({{ ref("RI13") }}).
- `a38 mcp` liest die Konfiguration schon für den Pfad des Agent-Sockets. Es nutzt daraus nur die Pfade der Projekte. Ein Arbeitsverzeichnis gilt, wenn es nach dem Auflösen der symbolischen Links zum selben Projekt gehört wie das Arbeitsverzeichnis von `a38 mcp`. Die Grenzen für Zeit und Ausgabe kommen weiter nur vom Daemon. Der Daemon prüft das Arbeitsverzeichnis selbst noch einmal.

Offene Fragen:

- Bestätigt Stefan das Ende der Prozessgruppe nach dem Kommando, oder soll ein Kindprozess weiterlaufen dürfen?
- Bestätigt Stefan die Pfade der Projekte aus der Konfiguration als Quelle der Prüfung in `a38 mcp`?

**Quelle:** [Issue 97](https://github.com/seletz/passierschein/issues/97), Umsetzung des Werkzeugs `run_with_credentials`.
