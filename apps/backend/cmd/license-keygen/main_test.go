package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteKeyPairFilesCreatesMatchingKeysWithoutOverwrite(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	privatePath := filepath.Join(dir, "issuer_private.key")
	publicPath := filepath.Join(dir, "issuer_public.key")
	if err := writeKeyPairFiles(privatePath, publicPath, publicKey, privateKey); err != nil {
		t.Fatal(err)
	}

	privateData, err := os.ReadFile(privatePath)
	if err != nil {
		t.Fatal(err)
	}
	publicData, err := os.ReadFile(publicPath)
	if err != nil {
		t.Fatal(err)
	}
	gotPrivate, err := decodeKeyLine(string(privateData), "NMS_LICENSE_PRIVATE_KEY_B64=")
	if err != nil {
		t.Fatal(err)
	}
	gotPublic, err := decodeKeyLine(string(publicData), "NMS_LICENSE_PUBLIC_KEY_B64=")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotPrivate, privateKey) || !bytes.Equal(gotPublic, publicKey) {
		t.Fatal("persisted key pair does not match generated key pair")
	}
	if !bytes.Equal(gotPrivate[ed25519.SeedSize:], gotPublic) {
		t.Fatal("public key does not match private key")
	}
	if err := writeKeyPairFiles(privatePath, publicPath, publicKey, privateKey); err == nil {
		t.Fatal("expected existing key files to be preserved")
	}
}

func decodeKeyLine(value, prefix string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(strings.TrimPrefix(strings.TrimSpace(value), prefix))
}
