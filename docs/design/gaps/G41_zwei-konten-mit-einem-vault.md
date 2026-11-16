---
render_macros: true
id: 'G41'
titel: 'Zwei Konten mit einem Vault desselben Titels'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-15'
order: 3
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G41 — Zwei Konten mit einem Vault desselben Titels

{{ design_kopf(page) }}

Der Name eines Vaults ist in der Konfiguration eindeutig ({{ ref("D49") }}, {{ ref("D71") }}). Die Referenz des Agents nennt kein Konto ({{ ref("D27") }}). Zwei Konten können aber einen Vault mit demselben Titel haben, zum Beispiel `datenbank-dev` im Konto `privat` und im Konto `firma`. Die Konfiguration kann dann nur einen der beiden nennen.

Offene Fragen:

- Gehört das Konto in die Referenz des Agents, zum Beispiel `op://firma/datenbank-dev/…`? Das ändert das Protokoll ({{ ref("D27") }}) und den Skill.
- Oder bekommt der Vault in `a38` einen eigenen, eindeutigen Namen und ein Feld für den Titel in 1Password (zum Beispiel `name: datenbank-dev-firma`, `title: datenbank-dev`)? Das ändert nur die Konfiguration und das Backend.

**Vorschlag:** Das Feld für den Titel. Es lässt Referenz, Protokoll und Skill unverändert. Bis zur Entscheidung benennt der Mensch einen der Vaults in 1Password um.

**Quelle:** [Issue 88](https://github.com/seletz/passierschein/issues/88), Abschnitt „Offen für das Design“.
