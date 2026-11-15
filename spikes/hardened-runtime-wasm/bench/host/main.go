// Command host runs bench/guest.wasm in wazero with the compiler or the
// interpreter.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func main() {
	ctx := context.Background()
	cfg := wazero.NewRuntimeConfigCompiler()
	if os.Args[1] == "interp" {
		cfg = wazero.NewRuntimeConfigInterpreter()
	}
	code, err := os.ReadFile(os.Args[2])
	if err != nil {
		panic(err)
	}
	start := time.Now()
	r := wazero.NewRuntimeWithConfig(ctx, cfg)
	wasi_snapshot_preview1.MustInstantiate(ctx, r)
	m, err := r.CompileModule(ctx, code)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s compile %v; ", os.Args[1], time.Since(start))
	_, err = r.InstantiateModule(ctx, m, wazero.NewModuleConfig().WithStdout(os.Stdout).WithSysWalltime().WithSysNanotime())
	if err != nil {
		panic(err)
	}
}
