---
render_macros: true
id: 'D60'
titel: 'a38 unterscheidet den lokalen Einsatz und den Einsatz in der VM, und lokal deckt a38 ab, was Agenten heute mit op tun'
bereich: 'Grenzen und Identität'
lauf: '2.0'
stand: '2026-11-15'
order: 11
headless: true
status: gueltig
use_cases: [UC15, UC16]
beleg:
  notiz: Vorschlag aus Issue 91, zu bestätigen durch Stefan
  annahme: true
---
# D60 — a38 unterscheidet den lokalen Einsatz und den Einsatz in der VM, und lokal deckt a38 ab, was Agenten heute mit op tun

{{ design_kopf(page) }}

**Entscheidung:** Das Design unterscheidet zwei Einsatzfälle. Jede neue Regel nennt den Fall, für den sie gilt.

| Einsatzfall | Lage | Grenze | Meilenstein |
|---|---|---|---|
| lokal | Der Agent läuft unter demselben macOS-Benutzer wie der Daemon. | Keine technische Grenze. Ein Prozess des Benutzers erreicht 1Password, den Schlüsselbund und die Dateien des Daemons auch ohne `a38` ({{ ref("RI5") }}). | ab 0.3.0 |
| VM | Der Agent läuft in einer VM ohne `op`, ohne SDK und ohne Token ({{ ref("D2") }}). | Der Daemon ist der einzige Weg zu einem Secret. | Stufe 5 (M5) |

**Maßstab für den lokalen Einsatz:** Lokal misst sich `a38` an dem direkten Aufruf von `op`. Agenten rufen `op` heute für dev, test und prod auf. `a38` muss diese Aufrufe abdecken. Sonst bleibt `op` im Einsatz, und der Hook gegen `op` lässt sich nicht für alle Projekte einschalten ({{ ref("D67") }}).

**Ziel des lokalen Einsatzes:** Lokal schützt `a38` nicht gegen einen gekaperten Agent. `a38` leistet lokal drei Dinge:

1. Der Mensch gibt jeden Zugriff frei und sieht dabei, wofür der Agent den Wert braucht.
2. Das Audit-Log nennt jeden Zugriff.
3. Der Wert eines Vaults außerhalb von dev erreicht im Normalfall nicht den Kontext des Modells und nicht das Transkript ({{ ref("D62") }}).

**Regel für den Entwurf:** Eine Regel für den lokalen Einsatz darf den Einsatz in der VM nicht verbauen. Der Vertrag des exec-Wegs ({{ ref("D61") }}) hängt deshalb nicht vom Transport zum Daemon ab. Die offenen Fragen der VM stehen in {{ ref("G35") }}.

**Begründung:** Stefan, 2026-11-15, wörtlich: „Wir müssen denke ich zwichen VM use case und lokalem use case unterscheiden. Lokal, heute, rufen agenten bei mir "op" kommandos auf. Auch für test und prod -- agenten haben aber im cluster sehr beschränkte rechte. Schmaler grat.“ Der Praxistest von 0.2.0 zeigt: Mit einem scharfen Hook fehlt den Agenten ein Weg für test und prod ([Issue 83](https://github.com/seletz/passierschein/issues/83)). Ein Werkzeug, das nur dev abdeckt, verdrängt `op` nicht. Gemessen an `op` verbessert `a38` lokal die Lage auch ohne technische Grenze: `op` gibt den Wert dem Agent direkt, ohne Freigabe und ohne Eintrag.

**Folge:** Die Erklärung [Lokaler Einsatz und VM](../../explanation/lokal-und-vm.md) beschreibt beide Fälle und die Grenzen des exec-Wegs.

**Quelle:** [Issue 91](https://github.com/seletz/passierschein/issues/91), Frage 1 und 6. Vorschlag des Designlaufs 2; Stefan bestätigt ihn.
