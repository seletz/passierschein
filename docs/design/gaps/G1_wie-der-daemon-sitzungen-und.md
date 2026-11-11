---
render_macros: true
id: 'G1'
titel: 'Wie der Daemon Sitzungen und VMs über den weitergeleiteten Socket erkennt, ist offen'
bereich: 'Grenzen und Identität'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G1 — Wie der Daemon Sitzungen und VMs über den weitergeleiteten Socket erkennt, ist offen

{{ design_kopf(page) }}

Über SSH-`RemoteForward` kommt jede Verbindung vom SSH-Prozess auf dem Mac. Die Peer-Daten nennen dann SSH und nicht den Agent. Ohne eine verlässliche Kennung gilt ein Grant für jeden Prozess in der VM. Das betrifft die Stufe 5.

Offene Fragen:

- Reicht eine VM je Projekt, sodass die VM die Sitzung ist?
- Kann der Daemon je VM einen eigenen Socket-Pfad anlegen und so die VM erkennen?
- Was zeigt die Freigabeanfrage als geprüfte Fakten?

**Quelle:** kritische Prüfung, Schwachstelle zur Identität; Projektnotiz, Ablauf Schritt 3.
