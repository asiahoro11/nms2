package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"log"

	"management-server/services/license"
)

// Run this only in the isolated license issuer environment.
func main() {
	pub, priv, err := license.GenerateEd25519KeyPair()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("NMS_LICENSE_PUBLIC_KEY_B64=%s\n", base64.StdEncoding.EncodeToString(pub))
	fmt.Printf("NMS_LICENSE_PRIVATE_KEY_B64=%s\n", base64.StdEncoding.EncodeToString(priv))
	fmt.Printf("public_key_bytes=%d private_key_bytes=%d\n", ed25519.PublicKeySize, ed25519.PrivateKeySize)
}
