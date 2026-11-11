---
render_macros: true
id: 'T7'
titel: 'a38 audit verify erkennt einen geänderten, gelöschten und vertauschten Eintrag'
bereich: 'Audit'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [F7, D36]
---
# T7 — a38 audit verify erkennt einen geänderten, gelöschten und vertauschten Eintrag

{{ design_kopf(page) }}

**Form:** Tabellengetriebener Go-Test auf einer Audit-Datei mit zehn Einträgen.

**Fälle:** unverändert; ein Feld geändert; ein Eintrag gelöscht; zwei Einträge vertauscht; letzter Eintrag entfernt.

**Erwartung:** Der unveränderte Fall besteht. Jeder andere Fall scheitert und nennt die Nummer des ersten falschen Eintrags.

**Bekannte Grenze:** Der Fall „letzter Eintrag entfernt“ ist ohne Verankerung außerhalb nicht erkennbar. Der Test hält das als erwartetes Verhalten fest ({{ ref("D37") }}).


Belegt: {{ ref("F7") }}, {{ ref("D36") }}.
