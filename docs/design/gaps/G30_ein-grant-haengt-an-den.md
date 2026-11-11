---
render_macros: true
id: 'G30'
titel: 'Ein Grant hängt an den Namen des Backends und nicht an einer festen Kennung des Items'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-11'
order: 4
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G30 — Ein Grant hängt an den Namen des Backends und nicht an einer festen Kennung des Items

{{ design_kopf(page) }}

Der Daemon löst eine Referenz im Backend auf und zeigt dem Menschen die Namen ({{ ref("D27") }}). Der Grant und der spätere Lesezugriff nutzen dieselben Namen. Eine feste Kennung des Items nutzt der Daemon nicht.

Mit dem echten 1Password kann ein Item zwischen der Freigabe und dem Lesen umbenannt werden, oder zwei Items tragen denselben Namen. Dann liest der Daemon möglicherweise ein anderes Item als das freigegebene. Das Fake-Backend kann das nicht zeigen.

Das betrifft die Stufe 3 ({{ ref("F12") }}). Es hängt mit {{ ref("G23") }} und {{ ref("G25") }} zusammen.

Offene Fragen:

- Liefert das Backend beim Auflösen eine feste Kennung, die der Grant speichert?
- Wie verhält sich der Daemon bei zwei Items mit demselben Namen?

**Quelle:** Entwicklung von Stufe 1, Broker und MCP-Server (Issues 21, 22, 24), 2026-11-11.
