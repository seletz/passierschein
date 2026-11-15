---
render_macros: true
id: 'RI13'
titel: 'Ein Agent holt den Wert über das Kommando in seinen Kontext'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-15'
order: 3
headless: true
status: offen
wahrscheinlichkeit: mittel
auswirkung: hoch
behandlung: [mindern, akzeptieren]
register:
  owner: Stefan
---
# RI13 — Ein Agent holt den Wert über das Kommando in seinen Kontext

{{ design_kopf(page) }}

**Beschreibung:** Der Agent wählt das Kommando beim exec-Weg selbst. Ein Kommando wie `printenv`, `echo "$PGPASSWORD" | base64` oder ein Programm, das seine Konfiguration ausgibt, bringt den Wert in die Ausgabe. Die Maskierung erkennt nur den exakten Wert ({{ ref("D64") }}). Lokal liest ein Prozess desselben Benutzers zudem die Umgebung des Kommandos. Ein verändertes `a38 mcp` gibt den Wert direkt zurück.

**Bewertung:** Ein ehrlicher Agent tut das selten und meist aus Versehen. Ein gekaperter Agent tut es gezielt. Die Auswirkung ist in test und prod höher als bei {{ ref("RI1") }}.

**Auslöser:** Ein Kommando mit dem Wert in der Ausgabe, eine kodierte Ausgabe, ein gekaperter Agent.

**Maßnahme:** Freigabeanfrage mit dem ganzen Kommando ({{ ref("D65") }}). Bindung an das Programm in test, Freigabe je Kommando in prod ({{ ref("D63") }}). Maskierung des exakten Werts. Audit-Log mit Kommando. Regel im Skill ({{ ref("D68") }}). Die Grenze steht offen in der Doku ({{ ref("D60") }}). Lokal bleibt der Rest akzeptiert, wie {{ ref("RI5") }}.

**Revisit:** mit der Stufe 5 ({{ ref("G35") }}) und nach dem ersten Monat mit test und prod.

**Quelle:** [Issue 91](https://github.com/seletz/passierschein/issues/91), Frage 5.
