---
render_macros: true
id: 'D78'
titel: 'Der Status nennt jedes 1Password-Konto mit eigenem Zustand, und die Konten antworten nebeneinander mit eigener Frist'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-16'
order: 12
headless: true
status: gueltig
use_cases: [UC11, UC6]
beleg:
  notiz: zu bestätigen durch Stefan (Issue 119)
  annahme: true
---
# D78 — Der Status nennt jedes 1Password-Konto mit eigenem Zustand, und die Konten antworten nebeneinander mit eigener Frist

{{ design_kopf(page) }}

**Entscheidung:**

1. **Feld `accounts`:** Die Antwort `status` am Admin-Socket nennt im Zustand des Backends je 1Password-Konto Name, Zustand (`ok`, `locked`, `unreachable`), Grund und die Zahl der Vaults (konfiguriert und sichtbar). Die Reihenfolge ist die der Konfiguration. Das Feld enthält nie ein Token und nie einen Wert. Der Agent-Socket ändert sich nicht.
2. **Zustand des ganzen Backends:** Er ist `ok`, solange ein Konto antwortet. Antwortet kein Konto, ist er `locked`, wenn mindestens ein Konto gesperrt ist, sonst `unreachable`.
3. **Nebeneinander, eigene Frist:** Der Daemon fragt die Konten nebeneinander. Jedes Konto hat eine eigene Frist: 3 Sekunden für den Status, 30 Sekunden für die Prüfung nach dem Start. Ein Konto ohne Antwort in seiner Frist ist `unreachable`; der Grund nennt die Frist. Ein hängendes oder gesperrtes Konto ändert den Zustand eines anderen Kontos nicht.
4. **`a38 doctor`:** Je Konto eine Zeile `1password account <name>`. Ein gesperrtes oder nicht erreichbares Konto ist ein Befund mit dem nächsten Schritt für dieses Konto. Sieht der Service Account nicht jeden Vault des Kontos, ist das ein Hinweis.
5. **`a38 tui`:** Die Übersicht zeigt eine Zeile `1PASSWORD` mit dem Zustand jedes Kontos und je gestörtem Konto den Grund. Die Ansicht holt den Status über den Admin-Socket, nur lesend, und nur, wenn die Konfiguration Konten nennt. Läuft der Daemon nicht, sagt die Zeile das; die Auswertung des Audit-Logs bleibt unverändert.

**Begründung:** Mit mehreren Konten ({{ ref("D73") }}) reicht ein Zustand für das ganze Backend nicht. Lehnt 1Password das Token eines Kontos ab, sah `a38 doctor` das bisher nicht ({{ ref("G45") }}). Eine gemeinsame Frist ließ ein langsames Konto die Zeit der anderen verbrauchen; dann hießen alle Konten „nicht erreichbar“. Die Frist von 3 Sekunden liegt unter der Frist von `a38 doctor` für den ganzen Status (5 Sekunden). Ein Backend bleibt `ok`, solange ein Konto antwortet, weil die anderen Konten weiter ausliefern; die Zeilen der Konten nennen den Rest. `a38 tui` zeigt den Zustand, weil der Mensch dort die Freigaben auswertet und ein gesperrtes Konto die Ablehnungen erklärt. Bisher sprach `a38 tui` nicht mit dem Daemon; die Statusabfrage ist nur lesend und ändert die Auswertung nicht.

**Quelle:** [Issue 119](https://github.com/seletz/passierschein/issues/119), Vorschlag in {{ ref("G45") }}. Vorschlag aus der Entwicklung; Stefan bestätigt ihn im Pull Request.
