---
render_macros: true
id: 'D59'
titel: 'a38 führt den WebAssembly-Kern des 1Password-SDK im Interpreter von wazero aus und bleibt ohne Entitlement'
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
# D59 — a38 führt den WebAssembly-Kern des 1Password-SDK im Interpreter von wazero aus und bleibt ohne Entitlement

{{ design_kopf(page) }}

**Entscheidung:** a38 führt den WebAssembly-Kern des 1Password-SDK im Interpreter von wazero aus. a38 trägt kein Entitlement der Hardened Runtime.

1. **Weg:** Das SDK lädt seinen Kern über `extism/go-sdk` und gibt keine Laufzeit vor. extism wählt dann `wazero.NewRuntimeConfig()`, und wazero wählt auf arm64 und amd64 den Compiler. Weder das SDK noch extism noch wazero bieten einen Schalter, keine Umgebungsvariable und keinen Build-Tag. Das Repository enthält deshalb eine Kopie von `extism/go-sdk` unter `third_party/extism-go-sdk` mit einer Änderung in einer Zeile: Ohne Vorgabe wählt extism `wazero.NewRuntimeConfigInterpreter()`. `go.mod` ersetzt das Modul mit `replace` durch die Kopie.
2. **Prüfung der Kopie:** Der Task `mise run deps:extism:check` ist Teil von `mise run check`. Er prüft, dass die Kopie das Modul der Version ist, die das SDK verlangt, mit der Prüfsumme aus `third_party/extism-go-sdk.sum`, plus dem Patch `third_party/extism-go-sdk.patch`, und sonst nichts. Verlangt ein neues SDK eine andere Version von extism, schlägt der Task fehl.
3. **Prüfung des Binarys:** `a38 selftest sdk-core` lädt den Kern und gibt ihm ein ungültiges Token. Der Kern lehnt es ab, bevor er das Netz ruft. {{ ref("T30") }} führt den Befehl mit einem a38 mit Hardened Runtime aus.
4. **Kein Entitlement:** `scripts/release-app-sign.sh` signiert a38 weiter ohne Entitlements. Die Prüfung des Bundles schlägt fehl, wenn a38 `allow-jit`, `allow-unsigned-executable-memory` oder `disable-executable-page-protection` trägt.

**Begründung:** Der Compiler von wazero legt Speicher mit `mmap` an (lesen und schreiben, ohne `MAP_JIT`), schreibt Maschinencode hinein und schaltet ihn mit `mprotect` auf ausführbar. Die Hardened Runtime tötet einen Prozess, der solchen Code ausführt (`SIGKILL`, `CODESIGNING`, `Invalid Page`). Das signierte a38 starb deshalb rund zwei Sekunden nach dem Start mit `--backend 1password` ([Issue 89](https://github.com/seletz/passierschein/issues/89)). Der Nachweis steht im [Spike Hardened Runtime und WebAssembly](../../explanation/spikes/60-hardened-runtime-wasm.md).

Zwei Wege lösen das Problem:

| Weg | Hardened Runtime | Start des Kerns | Aufruf ohne Rechenlast | Rechenlast (Kryptographie) |
|---|---|---|---|---|
| A: Interpreter (gewählt) | unverändert | 0,15 s | 0,2 ms | 80- bis 125-mal langsamer |
| B: Entitlement `allow-unsigned-executable-memory` | geschwächt | 1,4 s | 0,02 ms | Maßstab |

`allow-jit` reicht für Weg B nicht: wazero nutzt kein `MAP_JIT`. Der Versuch mit `allow-jit` endet ebenfalls mit `SIGKILL`. Weg B braucht `allow-unsigned-executable-memory`. Dieses Entitlement erlaubt dem ganzen Prozess, Speicher ohne Signatur auszuführen. a38 hält das Token des Service Accounts und liefert Secrets aus ({{ ref("D1") }}). Eine eingeschleuste Schwachstelle hätte damit einen Weg, eigenen Code auszuführen. Weg A lässt die Hardened Runtime unverändert. Das wiegt schwerer als die Kosten des Interpreters.

Die Kosten des Interpreters sind gemessen, soweit es ohne Token geht. Der Start des Kerns ist schneller als mit dem Compiler, weil das Übersetzen entfällt. Ein Aufruf des Kerns ohne Rechenlast dauert 0,2 ms statt 0,02 ms. Rechenlast wie die Anmeldung mit SRP oder das Entschlüsseln eines Schlüssels mit RSA ist im Interpreter 80- bis 125-mal langsamer. Die Schätzung für die Anmeldung ist einige Sekunden, einmal je Start des Daemons. Ein Aufruf über das Netz dauert ohnehin 0,1 s und mehr. Der Integrationstest {{ ref("T15") }} protokolliert die Zeit der Anmeldung.

Die Kopie von extism ist eine Abweichung von der Lieferkette ({{ ref("D18") }}). Der Patch ist eine Zeile, und der Task aus Punkt 2 bindet die Kopie an die geprüfte Version. `govulncheck` sieht das ersetzte Modul nicht mehr unter seiner Version. Deshalb nennt der Task die Version, und ein Update von extism folgt dem Update des SDK.

Die Entscheidung betrifft {{ ref("G5") }} (Signatur mit der echten Developer ID) und {{ ref("RI9") }} (das SDK verhält sich anders als die Doku). Die Doku des SDK nennt die Hardened Runtime nicht. Ändert das SDK seinen Weg zum Kern, zeigt {{ ref("T30") }} das an.

**Wenn der Interpreter zu langsam ist:** Weg B bleibt möglich. Dann trägt a38 allein das Entitlement `com.apple.security.cs.allow-unsigned-executable-memory`, die App Passierschein keines. Die Prüfung aus Punkt 4 ändert sich mit.

**Quelle:** [Issue 89](https://github.com/seletz/passierschein/issues/89); Quellcode von `onepassword-sdk-go` v0.4.1 (`internal/extism_core.go`), `extism/go-sdk` v1.7.1 (`plugin.go`) und `tetratelabs/wazero` v1.11.0 (`config.go`, `runtime.go`, `internal/platform/mmap_unix.go`, `mmap_other.go`), gelesen am 2026-11-15. Annahme der Entwicklung; Stefan bestätigt sie im Review.
