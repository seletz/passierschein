---
render_macros: true
id: 'D29'
titel: 'Das Bündelfenster dauert drei Sekunden, und ein Bündel hat höchstens zehn Einträge'
bereich: 'Anfrage und Bündelung'
lauf: '1.0'
stand: '2026-11-11'
order: 3
headless: true
status: gueltig
use_cases: [UC1]
beleg:
  notiz: bleibt Annahme (Stefan, 2026-11-11); der Praxistest prüft die Werte
  annahme: true
---
# D29 — Das Bündelfenster dauert drei Sekunden, und ein Bündel hat höchstens zehn Einträge

{{ design_kopf(page) }}

**Entscheidung:** Das Bündelfenster beginnt mit der ersten Anfrage einer Sitzung ohne passenden Grant. Es dauert drei Sekunden. Jede weitere Anfrage der Sitzung in dieser Zeit kommt in dasselbe Bündel. Ein Bündel hat höchstens zehn Einträge. Der Daemon lehnt jeden weiteren Eintrag mit dem Code `bundle_too_large` ab. Beide Werte sind konfigurierbar.

**Begründung:** Niemand liest ein langes Bündel gründlich ({{ ref("RI12") }}). Drei Sekunden sind für den Agent kaum spürbar.
