---
render_macros: true
id: 'T9'
titel: 'Der Daemon startet mit einem echten Backend nicht mit einer gefälschten Freigabe'
bereich: 'Freigabe'
lauf: '1.0'
stand: '2026-11-11'
order: 3
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [D40]
---
# T9 — Der Daemon startet mit einem echten Backend nicht mit einer gefälschten Freigabe

{{ design_kopf(page) }}

**Form:** Tabellengetriebener Go-Test über die Kombinationen von `--backend` und `--ui`.

**Erwartung:** `--backend 1password` startet nur mit `--ui app`. Jede andere Kombination endet vor dem Öffnen des Sockets mit einem Fehler.


Belegt: {{ ref("D40") }}.
