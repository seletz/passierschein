---
render_macros: true
id: 'D12'
titel: 'Nur die Vaults aus der Konfiguration sind für a38 sichtbar'
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
# D12 — Nur die Vaults aus der Konfiguration sind für a38 sichtbar

{{ design_kopf(page) }}

**Entscheidung:** Die Konfiguration nennt jeden Vault, den `a38` betrachtet, ausdrücklich. Ein Vault, der dort nicht steht, existiert für `a38` nicht. Der Daemon wendet die Liste an. Der Daemon vergleicht keine Namensmuster.

**Begründung:** Welche Vaults gelten, ist eine Policy des Menschen. Die übrigen Vaults sind Altbestand. Die Vaults der Anwendungen sind je Umgebung getrennt; ein dev-Vault enthält nur dev.

**Quelle:** Projektnotiz, Vault-Struktur. Die Projektnotiz nennt als Regel „nur Vaults mit `k8s` im Namen“. Stefan hat am 2026-11-11 im Review festgelegt: Die Regel steht als Liste in der Konfiguration und nicht als Namensvergleich im Daemon. Die Umgebung eines Vaults: {{ ref("D47") }}. Die erlaubten Vaults je Projekt: {{ ref("D28") }}.
