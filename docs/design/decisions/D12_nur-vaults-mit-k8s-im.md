---
render_macros: true
id: 'D12'
titel: 'Nur Vaults mit k8s im Namen sind für a38 sichtbar'
bereich: 'Backends'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: gueltig
use_cases: [UC1, UC11]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Projektnotiz vom 2026-11-11
---
# D12 — Nur Vaults mit k8s im Namen sind für a38 sichtbar

{{ design_kopf(page) }}

**Entscheidung:** `a38` betrachtet nur Vaults, deren Name `k8s` enthält. Alle anderen Vaults existieren für `a38` nicht.

**Begründung:** Die übrigen Vaults sind Altbestand. Die Vaults der Anwendungen sind je Umgebung getrennt; ein dev-Vault enthält nur dev.

**Quelle:** Projektnotiz, Vault-Struktur. Die Erkennung der Umgebung: {{ ref("D47") }}.

**Offen:** Stefan hat am 2026-11-11 gesagt: Das ist eine Policy. Die Konfiguration stellt sie sicher. Ein Vergleich des Namens im Daemon ist nicht nötig. Die Neufassung steht aus.
