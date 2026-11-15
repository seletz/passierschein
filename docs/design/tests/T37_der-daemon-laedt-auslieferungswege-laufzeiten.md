---
render_macros: true
id: 'T37'
titel: 'Der Daemon lädt Auslieferungswege, Laufzeiten und Grenzen je Umgebung und lehnt verbotene Werte ab'
bereich: 'Betrieb und Release'
lauf: '2.0'
stand: '2026-11-15'
order: 3
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [F22, D66, UC16]
---
# T37 — Der Daemon lädt Auslieferungswege, Laufzeiten und Grenzen je Umgebung und lehnt verbotene Werte ab

{{ design_kopf(page) }}

**Form:** Go-Test, tabellengesteuert, über das Laden der Konfiguration.

**Schritte:** Je Zeile eine Datei:

1. ohne `environments` und ohne `exec`,
2. mit `environments.test.delivery: [mcp]`,
3. mit `environments.prod.grant_lifetime: 1h`,
4. mit `environments.prod.binding: program`,
5. mit `environments.test.binding: none`,
6. mit `grant.lifetime: 2h` und `environments.dev.grant_lifetime: 1h`,
7. mit `grant.lifetime: 2h` allein,
8. mit `exec.default_timeout: 20m` und `exec.max_timeout: 10m`,
9. mit `environments.staging`.

**Erwartung:** 1 lädt mit den Standards aus {{ ref("D66") }}. 2 bis 6, 8 und 9 brechen den Start ab; die Meldung nennt den Schlüssel. 7 lädt mit der Laufzeit 2 Stunden für dev; `a38 config validate` nennt den Schlüssel als veraltet.

**Scheitert, wenn:** eine verbotene Datei lädt oder ein Standard abweicht.
