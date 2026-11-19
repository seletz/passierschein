---
render_macros: true
id: 'D79'
titel: 'Der Daemon liest die Tokens der Konten beim Start nebeneinander aus dem Schlüsselbund'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-19'
order: 13
headless: true
status: gueltig
use_cases: [UC11]
beleg:
  wer: Stefan
  datum: '2026-11-19'
---
# D79 — Der Daemon liest die Tokens der Konten beim Start nebeneinander aus dem Schlüsselbund

{{ design_kopf(page) }}

**Entscheidung:** `a38 serve --backend 1password` liest die Tokens der Konten beim Start nebeneinander. Jedes Konto behält die Wartezeit von 2 Minuten für einen Dialog von macOS. Der Start wartet so höchstens einmal 2 Minuten, nicht 2 Minuten je Konto. Ergebnis, Log und Meldung behalten die Reihenfolge der Konfiguration. Alles andere aus {{ ref("D73") }} Punkt 1 gilt weiter: nur Konten mit Vault, jedes Token einmal.

**Gewählt aus zwei Wegen:**

| Weg | Vorteil | Nachteil |
|---|---|---|
| **Nebeneinander lesen** (gewählt) | Der Start wartet höchstens eine Frist. Jeder Dialog behält die volle Zeit für den Menschen. | Mehrere Dialoge von macOS können gleichzeitig erscheinen. |
| Wartezeit je Konto kürzen | Ein Dialog nach dem anderen. | Ein kurzer Dialog läuft ab, bevor der Mensch ihn liest. Das Konto bleibt dann bis zum Neustart gesperrt ({{ ref("D73") }} Punkt 5, kein Neuladen). Mit vielen Konten bleibt der Start trotzdem lang. |

**Begründung:** Ein abgelaufener Dialog kostet einen Neustart, und der Neustart beendet alle Grants ({{ ref("D33") }}). Eine kurze Wartezeit macht diesen Fall häufiger. Gleichzeitige Dialoge sind dagegen nur unbequem: Jeder Dialog nennt seinen Eintrag `a38.onepassword/<konto>`, und alle kommen vom selben Start, den der Mensch selbst ausgelöst hat. Dialoge kommen nur nach einem neuen Build ohne Developer ID ({{ ref("G5") }}); `a38 config onepassword-token set --account <name> --replace` vermeidet sie ganz.

**Quelle:** [Issue 119](https://github.com/seletz/passierschein/issues/119), Punkt 4 des Befunds. Vorschlag aus der Entwicklung; Stefan bestätigt ihn im Pull Request.
