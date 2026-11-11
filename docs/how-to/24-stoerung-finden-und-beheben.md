---
title: Störung finden und beheben
description: "Mit a38 doctor den Zustand prüfen, die Sperre der Auslieferung lesen und aufheben, typische Befunde beheben."
---
# Störung finden und beheben

Diese Anleitung zeigt, wie du eine Störung von `a38` findest und behebst.
Du brauchst nur `a38 doctor` und `a38 fault`.
Beide Befehle zeigen keinen Secret-Wert.
Die Befehle sind in der [Referenz der Befehle für den Menschen](../reference/80-befehle-fuer-den-menschen.md) beschrieben.

## Zustand prüfen

Starte `a38 doctor`.

```console
$ a38 doctor
OK       configuration      loaded, 1 projects, 1 vaults
OK       socket modes       directory 0700, sockets 0600
OK       daemon             version v0.1.0, 1 sessions, 2 grants
OK       delivery lock      delivery is open
FINDING  audit file         the audit file is not writable
         Cause:     open /Users/beispiel/.local/state/a38/audit.db: permission denied
         Next step: fix the rights of /Users/beispiel/.local/state/a38/audit.db; without the audit log the daemon locks delivery

1 finding.
```

Jede Zeile beginnt mit `OK` oder `FINDING`.
Ein Befund nennt die Ursache (`Cause`) und den nächsten Schritt (`Next step`).
Der Exit-Code ist 0 ohne Befund und 1 mit mindestens einem Befund.
Mit `--json` gibt der Befehl dieselben Daten für Programme aus.

Mit `--config <datei>` wählst du eine andere Konfigurationsdatei.
Ohne das Argument gilt die Reihenfolge von `A38_CONFIG` und dem Standardort.

## Sperre der Auslieferung lesen und aufheben

Erkennt der Daemon eine Störung an sich selbst, sperrt er die Auslieferung für alle Sitzungen.
Jede Anfrage endet dann mit `daemon_faulted`.
Die Sperre endet nie von selbst.
Du hebst sie auf.

1. Lies den Zustand mit `a38 fault status`. Der Befehl nennt die Ursache und den Beginn der Sperre, soweit der Daemon sie meldet.
2. Behebe die Ursache.
3. Hebe die Sperre mit `a38 fault clear` auf.

Der Daemon prüft die Ursache, bevor er die Sperre aufhebt.
Besteht die Ursache noch, bleibt die Sperre.
Der Befehl zeigt dann den Befund und endet mit dem Exit-Code 1.
Du kannst auch den Daemon neu starten.

`a38 fault status` endet mit dem Exit-Code 1, solange die Sperre besteht.
Ein Skript kann damit auf die Sperre prüfen.

## Typische Befunde

| Befund | Nächster Schritt |
|---|---|
| Die Konfiguration lädt nicht | Die Ursache nennt die Zeile oder den Schlüssel. Korrigiere die Datei und starte `a38 doctor` erneut. |
| Die Konfiguration eines Projekts ist ungültig | Die Zeile nennt das Projekt und den Grund, zum Beispiel einen Vault, der nicht in der Vault-Liste steht. Korrigiere das Projekt und starte den Daemon neu. Andere Projekte laufen weiter. |
| Das Socket-Verzeichnis hat die falschen Rechte | `chmod 700 <verzeichnis>` und den Daemon neu starten. Der Daemon korrigiert die Rechte nicht selbst. |
| Der Daemon antwortet nicht | `a38 serve` starten. Prüfe, dass er dieselbe Konfiguration nutzt. |
| Die Auslieferung ist gesperrt | Ursache beheben, dann `a38 fault clear`. |
| Die Audit-Datei fehlt oder ist nicht schreibbar | Rechte der Datei und des Verzeichnisses korrigieren. Ohne Audit-Log sperrt der Daemon die Auslieferung. |
| Wenig Platz im Verzeichnis der Audit-Datei | Platz schaffen. Die Schwelle ist 100 MiB. |
| Die Hash-Kette des Audit-Logs ist beschädigt | Die Datei als Beweis aufbewahren und mit `a38 audit list` ansehen. Dann aus einer Sicherung wiederherstellen oder die Datei wegschieben, damit der Daemon ein neues Log beginnt. |

## Grenzen

Der Daemon meldet den Zustand des Backends und der Freigabe-App heute nicht in der Antwort auf `status`.
`a38 doctor` zeigt für beide `state not reported`.
Ein gesperrtes oder nicht erreichbares Backend ist ein Fehler der einzelnen Anfrage und keine Sperre der Auslieferung.
