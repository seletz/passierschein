---
render_macros: true
id: 'G47'
titel: 'Eine Laufzeit 0 in dev oder test lässt sich nicht freigeben'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-15'
order: 9
headless: true
type: Gap
kind: design
status: offen
register:
  owner: Stefan
---
# G47 — Eine Laufzeit 0 in dev oder test lässt sich nicht freigeben

{{ design_kopf(page) }}

{{ ref("D66") }} erlaubt für `environments.<env>.grant_lifetime` Werte von `0s` bis `24h`, auch bei dev und test. {{ ref("D63") }} sagt: Laufzeit 0 heißt ein Grant für genau eine Auslieferung. Die Freigabeanfrage kennt diese Art aber nur für prod: Die Art `once` gilt nach der Prüfung aus Issue 98 nur bei prod, und die Art `grant` braucht eine Laufzeit größer als 0. Auch die gewöhnliche Freigabeanfrage für `request_credentials` braucht eine Laufzeit größer als 0.

Eine Konfiguration mit `grant_lifetime: 0s` bei dev oder test lädt also, aber keine Freigabe gelingt. Der Daemon meldet dann `ui_failed`, weil die Freigabeanfrage ungültig ist.

**Umsetzung in Issue 99:** Der Daemon baut die Freigabeanfrage nach den Regeln aus Issue 98. Bei Laufzeit 0 außerhalb von prod scheitert die Prüfung der Freigabeanfrage; der Agent erhält `ui_failed`, das Audit-Log einen Fehler.

Offene Fragen:

- Soll die Konfiguration `0s` nur bei prod erlauben?
- Oder soll die Art `once` auch für dev und test gelten, wenn die Laufzeit 0 ist?

**Quelle:** [Issue 99](https://github.com/seletz/passierschein/issues/99), Umsetzung des Brokers für den exec-Weg.
