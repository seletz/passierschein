---
render_macros: true
id: 'G42'
titel: 'Die Entscheidung zum Service Account und die Security Boundaries nennen nur ein Konto'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-15'
order: 4
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G42 — Die Entscheidung zum Service Account und die Security Boundaries nennen nur ein Konto

{{ design_kopf(page) }}

{{ ref("D39") }} und die [Security Boundaries](../50-security-boundaries.md) sprechen von einem Service Account und einem Token. Beide stammen aus dem eingefrorenen Designlauf 1. Mit {{ ref("D71") }} hat jedes Konto einen eigenen Service Account und ein eigenes Token. Der Kern von {{ ref("D39") }} bleibt richtig: Service Account statt Desktop-Integration, nur Lesen, nur die freigegebenen Vaults.

Offene Fragen:

- Braucht {{ ref("D39") }} eine neue Fassung in Lauf 2, oder genügt der Satz in {{ ref("D71") }}, dass {{ ref("D39") }} für jedes Konto gilt?
- Der Nachtrag zu den Security Boundaries nennt nur den exec-Weg. Er muss die Tokens mehrerer Konten nennen: Jedes Token ist ein eigenes Schutzgut in der Zone des Mac. Ein Prozess, der einen Eintrag liest, liest nur die Vaults dieses Kontos. Mehr Konten heißt mehr Einträge und mehr Dialoge bei einem neuen Build ({{ ref("G5") }}).

**Quelle:** [Issue 88](https://github.com/seletz/passierschein/issues/88), Hinweis, dass die Änderung {{ ref("D51") }} und {{ ref("D39") }} berührt.
