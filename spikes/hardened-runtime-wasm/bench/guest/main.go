// Command guest is a CPU load for the benchmark: one RSA-sized modular
// exponentiation (similar to an SRP step) and AES-GCM over 1 MiB.
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"fmt"
	"math/big"
	"time"
)

func main() {
	start := time.Now()
	p, _ := new(big.Int).SetString("FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74020BBEA63B139B22514A08798E3404DDEF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7EDEE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3DC2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F83655D23DCA3AD961C62F356208552BB9ED529077096966D670C354E4ABC9804F1746C08CA18217C32905E462E36CE3BE39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9DE2BCBF6955817183995497CEA956AE515D2261898FA051015728E5A8AACAA68FFFFFFFFFFFFFFFF", 16)
	x := new(big.Int).SetBytes(sha256.New().Sum([]byte("beispielprojekt")))
	r := new(big.Int).Exp(big.NewInt(2), x.Lsh(x, 1800), p)
	t1 := time.Since(start)
	start = time.Now()
	key := make([]byte, 32)
	b, _ := aes.NewCipher(key)
	g, _ := cipher.NewGCM(b)
	buf := make([]byte, 1<<20)
	out := g.Seal(nil, make([]byte, 12), buf, nil)
	fmt.Println("modexp", t1, "aesgcm-1MiB", time.Since(start), r.BitLen()%2, len(out))
}
