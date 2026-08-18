package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"management-server/services/license"
)

// Run this only in the isolated license issuer environment.
func main() {
	privateOutput := flag.String("private-output", "issuer_private.key", "new private-key output file")
	publicOutput := flag.String("public-output", "issuer_public.key", "new public-key output file")
	flag.Parse()

	pub, priv, err := license.GenerateEd25519KeyPair()
	if err != nil {
		log.Fatal(err)
	}
	if err := writeKeyPairFiles(*privateOutput, *publicOutput, pub, priv); err != nil {
		log.Fatal(err)
	}
	fingerprint := sha256.Sum256(pub)
	fmt.Printf("License issuer key pair created.\n")
	fmt.Printf("Private key: %s\n", mustAbs(*privateOutput))
	fmt.Printf("Public key:  %s\n", mustAbs(*publicOutput))
	fmt.Printf("Public-key SHA-256: %x\n", fingerprint)
	fmt.Printf("public_key_bytes=%d private_key_bytes=%d\n", ed25519.PublicKeySize, ed25519.PrivateKeySize)
	fmt.Println("The private key was not printed. Keep it offline and never package it with NMS.")
}

func writeKeyPairFiles(privatePath, publicPath string, publicKey, privateKey []byte) error {
	privatePath = filepath.Clean(strings.TrimSpace(privatePath))
	publicPath = filepath.Clean(strings.TrimSpace(publicPath))
	if privatePath == "." || publicPath == "." {
		return fmt.Errorf("key output paths cannot be empty")
	}
	privateAbs, err := filepath.Abs(privatePath)
	if err != nil {
		return err
	}
	publicAbs, err := filepath.Abs(publicPath)
	if err != nil {
		return err
	}
	if strings.EqualFold(privateAbs, publicAbs) {
		return fmt.Errorf("private and public key output paths must differ")
	}
	if len(publicKey) != ed25519.PublicKeySize || len(privateKey) != ed25519.PrivateKeySize {
		return fmt.Errorf("invalid Ed25519 key-pair size")
	}

	privateFile, err := os.OpenFile(privateAbs, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create private key without overwrite: %w", err)
	}
	privateCreated := true
	defer func() {
		_ = privateFile.Close()
		if privateCreated {
			_ = os.Remove(privateAbs)
		}
	}()

	publicFile, err := os.OpenFile(publicAbs, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create public key without overwrite: %w", err)
	}
	publicCreated := true
	defer func() {
		_ = publicFile.Close()
		if publicCreated {
			_ = os.Remove(publicAbs)
		}
	}()

	privateLine := "NMS_LICENSE_PRIVATE_KEY_B64=" + base64.StdEncoding.EncodeToString(privateKey) + "\n"
	publicLine := "NMS_LICENSE_PUBLIC_KEY_B64=" + base64.StdEncoding.EncodeToString(publicKey) + "\n"
	if _, err := privateFile.WriteString(privateLine); err != nil {
		return fmt.Errorf("write private key: %w", err)
	}
	if err := privateFile.Sync(); err != nil {
		return fmt.Errorf("sync private key: %w", err)
	}
	if _, err := publicFile.WriteString(publicLine); err != nil {
		return fmt.Errorf("write public key: %w", err)
	}
	if err := publicFile.Sync(); err != nil {
		return fmt.Errorf("sync public key: %w", err)
	}
	if err := privateFile.Close(); err != nil {
		return fmt.Errorf("close private key: %w", err)
	}
	if err := publicFile.Close(); err != nil {
		return fmt.Errorf("close public key: %w", err)
	}
	privateCreated = false
	publicCreated = false
	return nil
}

func mustAbs(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}
