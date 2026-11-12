---
render_macros: true
id: 'D52'
titel: 'Das 1Password-Backend liest über feste Kennungen und lehnt doppelte Titel ab'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-12'
order: 6
headless: true
status: gueltig
use_cases: [UC1]
beleg:
  notiz: Vorschlag aus der Entwicklung, zu bestätigen durch Stefan
  annahme: true
---
# D52 — Das 1Password-Backend liest über feste Kennungen und lehnt doppelte Titel ab

{{ design_kopf(page) }}

**Entscheidung:**

1. `Resolve` des 1Password-Backends sucht die festen Kennungen von Vault, Item und Feld und bindet sie an die Referenz. Die Bindung gilt bis zum Ende des Daemons.
2. `Read` liest nur über diese Kennungen. Der Daemon liest so das Item, das der Mensch freigegeben hat, auch nach einer Umbenennung.
3. Findet ein späteres `Resolve` derselben Referenz ein anderes Item, endet die Anfrage mit `backend_unavailable`. Ein Neustart des Daemons beendet alle Grants ({{ ref("D33") }}) und nimmt das neue Item an.
4. Zwei Items, zwei Felder oder zwei Vaults mit demselben Titel enden mit `backend_unavailable`, nicht mit `not_found`. Das Log des Daemons nennt Vault und Titel.
5. Das Segment des Items ist ein Titel und nie die Kennung eines Items. Vault, Item und Feld vergleicht das Backend genau, ohne Angleichung. Ein Feld darf in einem Abschnitt liegen; seine Bezeichnung muss im Item eindeutig sein.
6. Eine Referenz mit Attribut (zum Beispiel `?attribute=otp`) endet mit `not_found` ({{ ref("G25") }}).
7. Die Schnittstelle `backend.Backend` und der Grant bleiben unverändert. Die Bindung liegt im Backend.

**Begründung:** `not_found` sagte dem Agent, dass es den Eintrag nicht gibt; er gibt es aber zweimal. Der Agent kann das nicht beheben, nur der Mensch durch Umbenennen. `backend_unavailable` liefert nichts aus und lädt nicht zu einem Umweg ein ({{ ref("D35") }}). Ein Item über seine Kennung zu nennen hieße, dass der Agent Kennungen kennt; die Freigabeanfrage zeigt Titel ({{ ref("D27") }}). Die Bindung bis zum Neustart folgt dem Grundsatz aus {{ ref("D48") }}: Der Dienst wechselt sein Verhalten nicht ohne Zutun des Menschen.

**Folge:** Die Entscheidung beantwortet die Fragen aus {{ ref("G30") }} für das 1Password-Backend und die Frage nach der Kennung aus {{ ref("G23") }}.

**Quelle:** Entwicklung der Stufe 3, [Issue 50](https://github.com/seletz/passierschein/issues/50). Vorschlag aus der Entwicklung, zu bestätigen durch Stefan.
