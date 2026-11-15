---
title: "Spike: Hardened Runtime und der WebAssembly-Kern des 1Password-SDK"
description: "Der Spike belegt, warum das signierte a38 beim ersten Zugriff auf 1Password stirbt, und misst die Kosten der zwei Lösungen."
render_macros: true
tags:
  - Spike
  - Sicherheit
  - 1Password
---
# Spike: Hardened Runtime und der WebAssembly-Kern des 1Password-SDK

Stand: 2026-11-15. Bezug: [Issue 89](https://github.com/seletz/passierschein/issues/89).

Der Spike hat 1Password nicht benutzt. Er hat kein Token gelesen und keine Token-Datei geöffnet. Jeder Versuch nutzt das ungültige Token `ops_beispiel-kein-token`. Der Kern des SDK lehnt es ab, bevor er das Netz ruft. Jede Signatur ist ad hoc; der Spike hat die Developer ID nicht benutzt. Der Code liegt unter `spikes/hardened-runtime-wasm/`.

## Frage

Das signierte a38 aus dem Snapshot `0.1.0-SNAPSHOT-b79fc9c` startet mit `--backend 1password` und endet rund zwei Sekunden später. launchd meldet `OS_REASON_CODESIGNING`. Der Absturzbericht nennt `EXC_BAD_ACCESS`, `SIGKILL (Code Signature Invalid)`, `CODESIGNING` und `Invalid Page`. Ein lokaler Build ohne Hardened Runtime läuft.

Die Vermutung: Das SDK führt seinen Kern als WebAssembly aus (`extism/go-sdk`, `tetratelabs/wazero` v1.11.0). wazero übersetzt den Kern in Maschinencode. Die Hardened Runtime verbietet das ohne Entitlement.

## Ursache im Quellcode

1. `onepassword-sdk-go` v0.4.1 lädt den Kern in `internal/extism_core.go` mit `extism.PluginConfig{}`. Es gibt keine Laufzeit vor.
2. `extism/go-sdk` v1.7.1 wählt in `plugin.go` ohne Vorgabe `wazero.NewRuntimeConfig()`.
3. wazero wählt in `runtime.go` mit `engineKindAuto` auf arm64 und amd64 den Compiler.
4. Der Compiler legt Speicher mit `mmap` an, lesbar und schreibbar, ohne `MAP_JIT` (`internal/platform/mmap_other.go`). Er schreibt den Code hinein und schaltet ihn mit `mprotect` auf lesbar und ausführbar (`mmap_unix.go`).
5. Kein Teil bietet einen Schalter, eine Umgebungsvariable oder einen Build-Tag für den Interpreter. `go build -overlay` darf keine Datei im Modul-Cache ersetzen.

## Nachstellen

Das Programm `hrspike` ruft `sdk.NewClient` mit dem ungültigen Token. Der kleinste Nachweis:

```sh
cd spikes/hardened-runtime-wasm
CGO_ENABLED=0 go build -o bin/hrspike-compiler .
cp bin/hrspike-compiler bin/hrspike-hr
codesign --force --sign - --options runtime bin/hrspike-hr
./bin/hrspike-hr; echo $?    # 137
```

| Variante | Ergebnis |
|---|---|
| ohne Hardened Runtime (Signatur des Linkers) | läuft; der Kern lehnt das Token ab |
| ad hoc mit `--options runtime`, ohne Entitlement | `SIGKILL`, Exit-Status 137 |
| ad hoc mit Hardened Runtime und `allow-jit` | `SIGKILL`, Exit-Status 137 |
| ad hoc mit Hardened Runtime und `allow-unsigned-executable-memory` | läuft |
| ad hoc mit Hardened Runtime, extism mit dem Interpreter von wazero | läuft |

Der Absturzbericht des zweiten Versuchs gleicht dem aus dem Issue: `EXC_BAD_ACCESS`, `SIGKILL (Code Signature Invalid)`, Namensraum `CODESIGNING`, `Invalid Page`. Die Seite liegt in einem Bereich `r-x/rwx` ohne Datei. Ein LaunchAgent mit eigenem Label (`a38-test-89.hrspike`, Plist im temporären Verzeichnis) zeigt in `launchctl print` nur `last exit reason = OS_REASON_CODESIGNING`, keinen Exit-Code und kein Signal. Deshalb meldete `a38 service status` „no exit known“.

`allow-jit` reicht nicht, weil wazero kein `MAP_JIT` nutzt. Damit ist die Vermutung belegt.

## Kosten des Interpreters

Gemessen auf einem Mac mit Apple Silicon, je drei Läufe:

| Messung | Compiler | Interpreter |
|---|---|---|
| erster `sdk.NewClient` (Kern laden und starten) | 1,42 s | 0,14 s bis 0,16 s |
| `sdk.NewClient` mit geladenem Kern, Mittel aus 20 | 0,02 ms | 0,2 ms |
| Rechenlast: Potenz modulo einer Zahl mit 2048 Bit | 21 ms | 2,6 s |
| Rechenlast: AES-GCM über 1 MiB | 54 ms | 4,3 s |

Die Rechenlast stammt aus einem eigenen Modul (`bench/`, Go nach `wasip1`). Den Kern des SDK kann der Spike ohne Token nicht rechnen lassen.

Die echte Anmeldung hat der Integrationstest {{ ref("T15") }} im macOS-Job der CI gemessen, mit dem Service Account der CI. Für die Messung lag die Zeitgrenze bei 15 Minuten statt 60 Sekunden.

| Schritt von {{ ref("T15") }} | Compiler (Lauf 43700005590) | Interpreter (Lauf 43725336630) |
|---|---|---|
| Anmeldung (`connect`) | – | 63,9 s |
| Vault sichtbar, mit Anmeldung | 4,8 s | 65,5 s |
| drei Referenzen auflösen | 1,8 s | 9,7 s |
| drei Werte lesen | 0,7 s | 4,2 s |

Mit der normalen Zeitgrenze von 60 Sekunden scheiterte der Test im Interpreter.

## Empfehlung

Das Entitlement `com.apple.security.cs.allow-unsigned-executable-memory` für a38 allein. Der Compiler bleibt. Die Anmeldung im Interpreter dauert über eine Minute; das ist für den Daemon nicht nutzbar. Die Entscheidung ist {{ ref("D59") }}. Die Prüfung ist {{ ref("T30") }}.
