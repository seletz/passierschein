---
render_macros: true
id: 'D47'
titel: 'Die Umgebung eines Vaults folgt aus seinem Namen'
bereich: 'Backends'
lauf: '1.0'
stand: '2026-11-11'
order: 4
headless: true
status: gueltig
use_cases: [UC5, UC1]
beleg:
  notiz: Vorschlag des Designlaufs, zu bestätigen durch Stefan
  annahme: true
---
# D47 — Die Umgebung eines Vaults folgt aus seinem Namen

{{ design_kopf(page) }}

**Entscheidung:** Ein Vault gehört zur Umgebung dev, wenn ein Segment seines Namens genau `k8s-dev` ist. Jeder andere Vault gilt als nicht dev. Die Regel ist konfigurierbar.

**Begründung:** Die Vaults folgen dem Muster `<tenant>.<app>.k8s[-dev|-test]`. Im Zweifel gilt ein Vault als nicht dev. Das ist die sichere Richtung.
