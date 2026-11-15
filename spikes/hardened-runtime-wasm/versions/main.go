// Command versions is throwaway code for issue #89: it shows how the core
// of the 1Password SDK answers to integration versions. It uses only
// invalid tokens and calls no network.
package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	sdk "github.com/1password/onepassword-sdk-go"
)

func main() {
	ctx := context.Background()
	tokens := map[string]string{
		"invalid": "ops_beispiel-kein-token",
		"base64":  "ops_" + base64.StdEncoding.EncodeToString([]byte(`{"signInAddress":"beispiel.1password.eu"}`)),
	}
	names := []string{"a38"}
	versions := os.Args[1:]
	for tn, tok := range tokens {
		for _, n := range names {
			for _, v := range versions {
				_, err := sdk.NewClient(ctx, sdk.WithServiceAccountToken(tok), sdk.WithIntegrationInfo(n, v))
				fmt.Printf("token=%s name=%q version=%q (%d)\n  err=%v\n", tn, n, v, len(v), err)
			}
		}
	}
}
