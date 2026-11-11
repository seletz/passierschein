---
title: Fake-Backend
description: "Format der YAML-Datei für das Fake-Backend, Zähler und Fehlerzustände für Tests."
tags:
  - Thema/Backend
---
# Fake-Backend

Das Fake-Backend liest Vaults, Items und Felder aus einer YAML-Datei.
Es dient den Tests und der lokalen Entwicklung.
Das Paket heißt `internal/backend/fake`.
`a38 serve --backend fake --fake-file <datei>` nutzt es.
Der Schalter `--fake-file` nennt die YAML-Datei.
Ohne den Schalter startet der Daemon nicht.

## Format der Datei

Die Datei enthält ein YAML-Dokument mit dem Schlüssel `vaults`.

```yaml
vaults:
  - name: beispiel.app.k8s-dev
    items:
      - name: datenbank-dev
        fields:
          - name: username
            value: beispiel-wert-1
          - name: password
            value: beispiel-wert-2
```

| Ebene | Schlüssel | Bedeutung |
|---|---|---|
| Vault | `name`, `items` | Name des Vaults und seine Items |
| Item | `name`, `fields` | Titel des Items und seine Felder |
| Feld | `name`, `value` | Bezeichnung des Feldes und sein Wert |

Die Regeln:

- Die Listen behalten die Reihenfolge der Datei.
- Ein Name ist nicht leer.
- Ein Vault-Name kommt in der Datei einmal vor.
- Ein Item-Name kommt in seinem Vault einmal vor.
- Ein Feldname kommt in seinem Item einmal vor.
- Ein unbekannter Schlüssel ist ein Fehler.
- Die Datei enthält genau ein YAML-Dokument und mindestens einen Vault.
- Ein Fehler beim Laden nennt nie einen Wert.

Eine Beispieldatei liegt unter
`internal/backend/fake/testdata/beispiel.yaml`.

## Namen

Das Backend vergleicht Namen exakt.
Es normalisiert nichts.
Ein anderer Name ist `not_found`.
Das gilt für Vault, Item und Feld.
Die Auflösung liefert die Namen so, wie die Datei sie schreibt.

## Zähler

Das Fake-Backend zählt jeden Aufruf von `Resolve` und `Read`.
Es zählt je Referenz und insgesamt.

| Methode | Ergebnis |
|---|---|
| `Resolves()`, `ResolvesOf(ref)` | Anzahl der Auflösungen von Namen |
| `Reads()`, `ReadsOf(ref)` | Anzahl der Lesezugriffe auf Werte |
| `ResetCounters()` | setzt alle Zähler auf null |

- Ein Aufruf zählt auch bei einem unbekannten Namen oder einem Fehlerzustand.
- Ein Aufruf mit abgebrochenem Kontext zählt nicht.
- `Vaults` zählt nicht.
- Ein Test prüft mit `Reads() == 0`, dass kein Lesezugriff stattfand.

## Fehlerzustände

| Methode | Wirkung |
|---|---|
| `SetLocked(true)` | Jeder Aufruf scheitert mit `backend.ErrLocked` (`backend_locked`). |
| `SetUnavailable(true)` | Jeder Aufruf scheitert mit `backend.ErrUnavailable` (`backend_unavailable`). |
| `SetValue(ref, wert)` | Ändert den Wert eines Feldes zwischen zwei Lesezugriffen. |

- `SetLocked` hat Vorrang vor `SetUnavailable`.
- `SetValue` zählt nicht als Lesezugriff.
- Das Backend hält keinen Cache. Der nächste `Read` liefert den neuen Wert.

## Secrets

Das Fake-Backend schreibt nie einen Wert in ein Log oder auf die Ausgabe.
Kein Fehler enthält einen Wert.
`fmt` gibt für das Backend einen festen Text aus.
Die Werte der Beispieldatei sind erfunden.
