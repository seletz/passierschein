// Command hrspike is throwaway code for issue #89. It loads the WebAssembly
// core of the 1Password SDK with an invalid token. It never uses a real
// token and never reads a token file.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	sdk "github.com/1password/onepassword-sdk-go"
)

// fakeToken is not a service account token. The SDK core rejects it.
const fakeToken = "ops_beispiel-kein-token"

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	start := time.Now()
	fmt.Fprintln(os.Stderr, "hrspike: calling sdk.NewClient with an invalid token")
	_, err := sdk.NewClient(ctx,
		sdk.WithServiceAccountToken(fakeToken),
		sdk.WithIntegrationInfo("hrspike", "0"),
	)
	first := time.Since(start)
	fmt.Printf("first NewClient: %v err=%v\n", first, err)
	// A second client reuses the loaded core: this is the cost per call
	// without the load of the core.
	start = time.Now()
	const n = 20
	for i := 0; i < n; i++ {
		_, _ = sdk.NewClient(ctx, sdk.WithServiceAccountToken(fakeToken), sdk.WithIntegrationInfo("hrspike", "0"))
	}
	fmt.Printf("NewClient on loaded core, mean of %d: %v\n", n, time.Since(start)/n)
}
