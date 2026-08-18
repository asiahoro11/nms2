package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
)

// Offline issuer: keep the private key outside the NMS host and server packages.
func main() {
	payloadPath := flag.String("payload", "", "JSON payload file (automation mode)")
	outputPath := flag.String("output", "", "output License file; defaults to a timestamped file")
	privateKeyPath := flag.String("private-key", "", "private-key file; defaults to issuer_private.key beside the executable")
	flag.Parse()

	privateKey, err := loadPrivateKey(*privateKeyPath)
	if err != nil {
		fail("讀取 Ed25519 私鑰失敗：%v", err)
	}

	if strings.TrimSpace(*payloadPath) == "" {
		runWizard(bufio.NewReader(os.Stdin), privateKey, *outputPath)
		return
	}

	payload, err := readPayload(*payloadPath)
	if err != nil {
		fail("讀取 payload 失敗：%v", err)
	}
	key, err := issue(payload, privateKey)
	if err != nil {
		fail("簽發失敗：%v", err)
	}
	if strings.TrimSpace(*outputPath) == "" {
		fmt.Println(key)
		return
	}
	if err := writeLicense(*outputPath, key); err != nil {
		fail("寫入 License 失敗：%v", err)
	}
	fmt.Printf("License 已建立：%s\n", mustAbs(*outputPath))
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
