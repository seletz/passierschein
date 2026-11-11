---
render_macros: true
id: 'D14'
titel: 'Der Zugang zum Cluster läuft über die Tailscale-Identität und nicht über a38'
bereich: 'Grenzen und Identität'
lauf: '1.0'
stand: '2026-11-11'
order: 5
headless: true
status: gueltig
use_cases: [UC13]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Projektnotiz vom 2026-11-11
---
# D14 — Der Zugang zum Cluster läuft über die Tailscale-Identität und nicht über a38

{{ design_kopf(page) }}

**Entscheidung:** Der Zugang zum Cluster hängt an der Tailscale-Identität. 1Password enthält dafür keine Zugangsdaten. `a38` vermittelt keinen Zugang zum Cluster.

**Begründung:** Die Rechte liegen in der Rollenvergabe des Clusters. Die Persona für Coding Agents ist offen ({{ ref("G10") }}).

**Quelle:** Projektnotiz, Cluster.
