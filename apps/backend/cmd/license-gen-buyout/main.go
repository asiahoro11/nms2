package main

import (
	"flag"
	"fmt"
	"log"
	"management-server/services/license"
	"strings"
)

func main() {
	machineIDPtr := flag.String("id", "", "Target Machine ID (Required)")
	typePtr := flag.String("type", "device", "License Type: device, alert, combined")
	deviceCountPtr := flag.Int("count", 0, "Device Count (Required for device/combined types)")
	flag.Parse()

	if *machineIDPtr == "" {
		log.Fatal("Error: Machine ID is required. Use -id <MACHINE_ID>")
	}

	machineID := strings.ToLower(strings.TrimSpace(*machineIDPtr))
	licenseType := strings.ToLower(*typePtr)

	// Derive the same secret key as the backend (NMS-LICENSE + MachineID)
	secretKey := license.DeriveKey("NMS-LICENSE-" + machineID)

	var features []string

	// Buyout version always uses Permanent validity (empty string)
	validUntil := ""

	// Determine features based on type
	switch licenseType {
	case "device":
		if *deviceCountPtr <= 0 {
			log.Fatal("Error: Device count must be > 0 for device license")
		}
		// Device buy-out: Only devices, no extra alert features (unless covered by separate alert license)
		features = []string{"device_management", "email"}

	case "alert":
		// Alert buy-out: Enables advanced features. Device count is irrelevant (usually 0)
		features = []string{"email", "line", "telegram", "whatsapp", "discord", "slack"}
		*deviceCountPtr = 0

	case "combined":
		// Combined: Devices + Alerts
		if *deviceCountPtr <= 0 {
			log.Fatal("Error: Device count must be > 0 for combined license")
		}
		features = []string{"device_management", "email", "line", "telegram", "whatsapp", "discord", "slack"}

	default:
		log.Fatalf("Unknown license type: %s", licenseType)
	}

	fmt.Printf("Generating Buyout (Permanent) License...\n")
	fmt.Printf("Machine ID: %s\n", machineID)
	fmt.Printf("Type: %s\n", licenseType)
	fmt.Printf("Device Count: %d\n", *deviceCountPtr)
	fmt.Printf("Features: %v\n", features)
	fmt.Printf("Valid Until: Permanent\n")

	key, err := license.GenerateLicenseKey(
		machineID,
		*deviceCountPtr,
		features,
		validUntil,
		secretKey,
	)
	if err != nil {
		log.Fatalf("Error generating key: %v", err)
	}

	fmt.Println("\n================ BUYOUT LICENSE KEY ================")
	fmt.Println(key)
	fmt.Println("====================================================")
}
