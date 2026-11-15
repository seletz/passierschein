---
render_macros: true
id: 'D59'
titel: 'a38 trägt das Entitlement allow-unsigned-executable-memory für den WebAssembly-Kern des 1Password-SDK'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-15'
order: 7
headless: true
status: gueltig
use_cases: [UC1, UC11]
beleg:
  notiz: zu bestätigen durch Stefan
  annahme: true
---
# D59 — a38 trägt das Entitlement allow-unsigned-executable-memory für den WebAssembly-Kern des 1Password-SDK

{{ design_kopf(page) }}

**Entscheidung:** a38 trägt genau ein Entitlement der Hardened Runtime: `com.apple.security.cs.allow-unsigned-executable-memory`. Die App Passierschein trägt keines.

1. **Datei:** `scripts/a38.entitlements` legt das Entitlement fest. `scripts/release-app-sign.sh` signiert a38 im Bundle damit. Die Signatur mit quill in `.goreleaser.yml` signiert das einzelne a38 der tar-Archive damit (`notarize.macos.sign.entitlements`).
2. **Laufzeit:** Das 1Password-SDK führt seinen Kern weiter mit dem Compiler von wazero aus. Das Repository ändert weder das SDK noch extism noch wazero.
3. **Genau dieses Entitlement:** `release:app:check` und `release:signatures:check` schlagen fehl, wenn a38 andere Entitlements als die der Datei trägt oder die App überhaupt eines trägt. Diese statische Prüfung ist die verlässliche Prüfung von {{ ref("T30") }}.
4. **Prüfung zur Laufzeit:** `a38 selftest sdk-core` lädt den Kern und gibt ihm ein ungültiges Token. Der Kern lehnt es ab, bevor er das Netz ruft. {{ ref("T30") }} führt den Befehl mit einem a38 mit Hardened Runtime aus, dazu eine Kopie ohne Entitlement als Gegenprobe. Die macOS-Runner von GitHub setzen die Hardened Runtime für Code im Speicher nicht durch: Dort läuft auch die Kopie ohne Entitlement. Die Prüfung zur Laufzeit beweist die Wirkung deshalb nur auf einem Mac, der die Regel durchsetzt, etwa dem Mac der Entwicklung.

**Begründung:** Der Compiler von wazero legt Speicher mit `mmap` an (lesen und schreiben, ohne `MAP_JIT`), schreibt Maschinencode hinein und schaltet ihn mit `mprotect` auf ausführbar. Die Hardened Runtime tötet einen Prozess, der solchen Code ausführt (`SIGKILL`, `CODESIGNING`, `Invalid Page`). Das signierte a38 starb deshalb rund zwei Sekunden nach dem Start mit `--backend 1password` ([Issue 89](https://github.com/seletz/passierschein/issues/89)). Der Nachweis steht im [Spike Hardened Runtime und WebAssembly](../../explanation/spikes/60-hardened-runtime-wasm.md).

Zwei Wege lösen das Problem:

| Weg | Hardened Runtime | Anmeldung und erster Aufruf ({{ ref("T15") }}, CI) | drei Referenzen auflösen | drei Werte lesen |
|---|---|---|---|---|
| A: Interpreter von wazero | unverändert | 65,5 s (davon Anmeldung 63,9 s) | 9,7 s | 4,2 s |
| B: Entitlement `allow-unsigned-executable-memory` (gewählt) | ein Entitlement | 4,8 s | 1,8 s | 0,7 s |

`allow-jit` reicht nicht: wazero nutzt kein `MAP_JIT`. Der Versuch mit `allow-jit` endet ebenfalls mit `SIGKILL`.

Weg A lässt die Hardened Runtime unverändert. Er braucht aber eine Kopie von `extism/go-sdk` mit einer geänderten Zeile, und die Anmeldung dauert im Interpreter 63,9 s. Das liegt über der Zeitgrenze von 20 s für einen Aufruf des Backends. Der erste Agent nach dem Start des Daemons würde über eine Minute warten. Auch danach ist jeder Aufruf etwa fünfmal langsamer. Weg A ist damit nicht nutzbar.

Weg B schwächt die Hardened Runtime für a38 an einer Stelle: Der Prozess darf Speicher ohne Signatur ausführen. Die anderen Schutzmaßnahmen bleiben: Bibliotheken nur mit gültiger Signatur, keine Variablen `DYLD_*`, kein Debugger ohne Recht. a38 ist reines Go ohne CGO und lädt keine Bibliothek nach. Der einzige Erzeuger von ausführbarem Speicher ist wazero. Ein Angreifer bräuchte zuerst einen Fehler im Speicher des Prozesses; Go macht solche Fehler selten. Die App Passierschein, die den Dialog von Touch ID zeigt, bleibt ohne Entitlement.

Die Entscheidung betrifft {{ ref("G5") }} (Signatur mit der echten Developer ID) und {{ ref("RI9") }} (das SDK verhält sich anders als die Doku). Die Doku des SDK nennt die Hardened Runtime nicht. Ändert das SDK seinen Weg zum Kern, zeigt {{ ref("T30") }} das auf einem Mac an, der die Regel durchsetzt: Stirbt die Kopie ohne Entitlement dort nicht mehr, ist das Entitlement vielleicht überflüssig.

**Quelle:** [Issue 89](https://github.com/seletz/passierschein/issues/89), [Pull Request 90](https://github.com/seletz/passierschein/pull/90); Quellcode von `onepassword-sdk-go` v0.4.1 (`internal/extism_core.go`), `extism/go-sdk` v1.7.1 (`plugin.go`) und `tetratelabs/wazero` v1.11.0 (`config.go`, `runtime.go`, `internal/platform/mmap_unix.go`, `mmap_other.go`), gelesen am 2026-11-15. Messung des Interpreters im macOS-Job am 2026-11-15 (Lauf 43725336630), Vergleich mit dem Lauf 43700005590 auf `develop`. Annahme der Entwicklung; Stefan bestätigt sie im Review.
