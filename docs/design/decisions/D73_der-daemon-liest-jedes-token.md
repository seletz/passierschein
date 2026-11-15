---
render_macros: true
id: 'D73'
titel: 'Der Daemon liest jedes Token beim Start, und ein Konto ohne Token sperrt nur seine Vaults'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-15'
order: 11
headless: true
status: gueltig
use_cases: [UC11, UC1]
beleg:
  notiz: Vorschlag aus der Entwicklung (Issue 88), zu bestätigen durch Stefan
  annahme: true
---
# D73 — Der Daemon liest jedes Token beim Start, und ein Konto ohne Token sperrt nur seine Vaults

{{ design_kopf(page) }}

**Entscheidung:**

1. **Lesen beim Start:** `a38 serve --backend 1password` liest das Token jedes Kontos einmal beim Start, in der Reihenfolge der Konfiguration. Er liest nur die Tokens der Konten, die mindestens ein Vault nennt. Ein Konto ohne Vault löst so keinen Dialog aus.
2. **Ein Konto scheitert:** Fehlt das Token eines Kontos (kein Eintrag im Schlüsselbund, eine unsichere oder fehlende Datei, eine Zeitüberschreitung beim Schlüsselbund), startet der Daemon trotzdem. Das Konto ist dann gesperrt. Jede Anfrage an einen seiner Vaults endet mit `backend_locked`. Das Log des Daemons nennt das Konto und den nächsten Schritt, nie ein Token.
3. **Alle Konten scheitern:** Hat kein Konto ein Token, startet der Daemon nicht. Die Meldung nennt jedes Konto mit seinem Befund und dem nächsten Schritt.
4. **Ein Client je Konto:** Das 1Password-Backend hält einen Client des SDK je Konto. Es leitet jede Anfrage über den Vault an das Konto des Vaults. Der Client entsteht beim ersten Aufruf ({{ ref("D52") }} bleibt unverändert). Lehnt 1Password das Token eines Kontos ab, sperrt das nur die Vaults dieses Kontos.
5. **Kein Neuladen:** Der Daemon liest kein Token im Betrieb nach. Nach `a38 config onepassword-token set` startet der Mensch den Daemon neu ({{ ref("D49") }}). Der Neustart beendet alle Grants ({{ ref("D33") }}).
6. Die Prüfung im Hintergrund nach dem Start läuft je Konto. Das Log nennt je Konto, ob der Client arbeitet und wie viele Vaults der Service Account sieht.

**Begründung:** Ein gesperrtes Konto liefert nichts aus ({{ ref("D35") }}). Die anderen Konten arbeiten weiter; der Mensch verliert nicht alle Freigaben wegen eines Kontos. Der Code `backend_locked` sagt dem Agent, dass nur der Mensch helfen kann. Ohne jedes Token hat ein laufender Daemon keinen Zweck; der Abbruch beim Start ist dann die klarere Meldung.

**Wahl von Stefan:** Ob ein Konto ohne Token den Start abbricht (wie bisher in {{ ref("D51") }}) oder nur seine Vaults sperrt. Empfehlung: nur seine Vaults sperren, wie in der Skizze des Issues.

**Quelle:** [Issue 88](https://github.com/seletz/passierschein/issues/88), Skizze: „Ein Konto ohne gültiges Token sperrt nur seine Vaults.“ Vorschlag aus der Entwicklung, zu bestätigen durch Stefan.
