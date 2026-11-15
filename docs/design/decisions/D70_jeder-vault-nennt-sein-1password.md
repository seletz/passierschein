---
render_macros: true
id: 'D70'
titel: 'Jeder Vault nennt sein 1Password-Konto, es gibt kein Standardkonto'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-15'
order: 8
headless: true
status: gueltig
use_cases: [UC11, UC1]
beleg:
  wer: Stefan
  datum: '2026-11-15'
  quelle: ['#88']
  notiz: 'Stefan, 2026-11-15, wörtlich: immer den account mit angeben, better safe than sorry'
---
# D70 — Jeder Vault nennt sein 1Password-Konto, es gibt kein Standardkonto

{{ design_kopf(page) }}

**Entscheidung:** Jeder Vault der Konfiguration nennt das 1Password-Konto, zu dem er gehört (`vaults[].account`). Die Angabe ist Pflicht. Es gibt kein Standardkonto. Auch eine Konfiguration mit nur einem Konto nennt das Konto an jedem Vault. Ein Vault ohne Konto oder mit einem Konto, das die Konfiguration nicht nennt, ist ein Fehler der Konfiguration. Der Daemon startet dann nicht ({{ ref("D49") }}).

**Begründung:** Ein Service Account gehört zu genau einem Konto. Der Daemon muss für jeden Vault wissen, welches Token ihn liest. Ein Standardkonto wäre eine stille Annahme: Ein Vault, den der Mensch vergessen hat, ginge an das falsche Konto. Die Angabe an jedem Vault macht die Zuordnung sichtbar. Das folgt dem Gedanken aus {{ ref("D47") }}: Eine fehlende Angabe darf nie zu einem Standard führen.

**Quelle:** [Issue 88](https://github.com/seletz/passierschein/issues/88). Stefan, 2026-11-15, wörtlich: „immer den account mit angeben, better safe than sorry“. Die Form der Konfiguration steht in {{ ref("D71") }}.
