---
title: Start
icon: lucide/stamp
render_macros: true
---
# Passierschein

{{ version_banner() }}

Passierschein ist ein Credential-Broker für Coding Agents. Das
Kommandozeilenwerkzeug heißt `a38`.

!!! warning "Stand: Stufe 1"
    Stufe 1 läuft lokal mit einem Fake-Backend und einer gefälschten Freigabe.
    Es gibt noch kein 1Password-Backend und kein Popup mit Touch ID.
    Das [Tutorial](tutorials/10-erste-schritte.md) führt durch den Ablauf.

## Zweck

Coding Agents brauchen Zugangsdaten, zum Beispiel für eine Datenbank der
Entwicklungsumgebung. Passierschein gibt diese Zugangsdaten kontrolliert
heraus:

- Der Agent fragt die Zugangsdaten über einen MCP-Server an.
  MCP ist das Model Context Protocol.
- Ein Daemon auf dem Mac bündelt die Anfragen einer Sitzung.
- Ein Popup zeigt die Anfrage mit Kontext. Der Mensch gibt die Anfrage mit
  Touch ID frei oder lehnt sie ab.
- Eine Freigabe gilt für eine Sitzung und hat eine Laufzeit.
- Ein Audit-Log speichert jede Freigabe und jede Auslieferung.

Nur der Daemon hat Zugriff auf 1Password. Der Agent hat keinen Zugriff auf
1Password.

## Einstieg

Die Doku folgt [Diátaxis](https://diataxis.fr/). Jeder Abschnitt beantwortet
eine Art von Frage.

<div class="grid cards" markdown>

<div markdown>
:lucide-graduation-cap:{ .lg .middle } **Tutorials**

Lernen durch Tun.

[:octicons-arrow-right-24: Tutorials](tutorials/index.md)
</div>

<div markdown>
:lucide-wrench:{ .lg .middle } **How-To**

Anleitungen für eine bestimmte Aufgabe.

[:octicons-arrow-right-24: How-To](how-to/index.md)
</div>

<div markdown>
:lucide-book-open:{ .lg .middle } **Referenz**

Fakten zum Nachschlagen: Tasks, Release, Konventionen.

[:octicons-arrow-right-24: Referenz](reference/index.md)
</div>

<div markdown>
:lucide-lightbulb:{ .lg .middle } **Erklärungen**

Hintergrund: Konzept und Spikes.

[:octicons-arrow-right-24: Erklärungen](explanation/index.md)
</div>

<div markdown>
:lucide-drafting-compass:{ .lg .middle } **Design**

Das Design-Set: Use Cases, Entscheidungen, Features, Testfälle, Gaps und
Risiken.

[:octicons-arrow-right-24: Design](design/index.md)
</div>

<div markdown>
:lucide-tags:{ .lg .middle } **Tags**

Alle Seiten nach Tag.

[:octicons-arrow-right-24: Tags](tags/index.md)
</div>

</div>
