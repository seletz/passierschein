# Hardened runtime spike

This folder contains throwaway code for issue #89.
It is not production code. Do not use it in the product.

`hrspike` calls `sdk.NewClient` of the 1Password SDK with an invalid token.
It reads no token and calls no network: the SDK core rejects the token first.
`bench/` runs a CPU load in wazero with the compiler and with the interpreter.

The report is in `docs/explanation/spikes/60-hardened-runtime-wasm.md`.

## Build and run

```sh
CGO_ENABLED=0 go build -o bin/hrspike-compiler .
cp bin/hrspike-compiler bin/hrspike-hr
codesign --force --sign - --options runtime bin/hrspike-hr
./bin/hrspike-hr; echo $?   # 137: killed for the code signature

cp bin/hrspike-compiler bin/hrspike-uem
codesign --force --sign - --options runtime --entitlements ent-allow-unsigned-executable-memory.plist bin/hrspike-uem
./bin/hrspike-uem           # runs

# interpreter: a copy of extism/go-sdk v1.7.1 in ./extism-interp (not
# committed) where plugin.go calls wazero.NewRuntimeConfigInterpreter()
# instead of wazero.NewRuntimeConfig()
go mod edit -replace github.com/extism/go-sdk=./extism-interp
CGO_ENABLED=0 go build -o bin/hrspike-interp .
go mod edit -dropreplace github.com/extism/go-sdk

GOOS=wasip1 GOARCH=wasm go build -o bin/guest.wasm ./bench/guest
CGO_ENABLED=0 go build -o bin/benchhost ./bench/host
./bin/benchhost compiler bin/guest.wasm
./bin/benchhost interp bin/guest.wasm
```
