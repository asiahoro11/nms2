package main

import (
	"flag"
	"fmt"
	"log"
	"management-server/services/license"
	"strings"
	"time"
)

func main() {
	machineIDPtr := flag.String("id", "", "Target Machine ID (Required)")
	typePtr := flag.String("type", "combined", "License Type: device, alert, combined, camera, trial")
	deviceCountPtr := flag.Int("count", 10, "Device Count (for device/combined types)")
	cameraCountPtr := flag.Int("cameras", 0, "Camera Count: 4 (free), 9 (standard), 16 (enterprise)")
	yearsPtr := flag.Int("years", 1, "Validity in Years (1-5)")
	flag.Parse()

	if *machineIDPtr == "" {
		log.Fatal("Error: Machine ID is required. Use -id <MACHINE_ID>")
	}

	machineID := strings.ToLower(strings.TrimSpace(*machineIDPtr))
	licenseType := strings.ToLower(*typePtr)

	// Derive the same secret key as the backend (NMS-LICENSE + MachineID)
	// This assumes the backend uses the default derived key strategy.
	secretKey := license.DeriveKey("NMS-LICENSE-" + machineID)

	var features []string
	validUntil := ""

	// Determine features and validity based on type
	switch licenseType {
	case "device":
		features = []string{"device_management", "email"} // Basic features
		validUntil = time.Now().AddDate(*yearsPtr, 0, 0).Format("2006-01-02")
	case "alert":
		features = []string{"email", "line", "telegram", "whatsapp", "discord", "slack"}
		// Alert license is permanent by default in this logic, but can be year-based if needed.
		// User requirement said "Alert-only license: permanent" in handler logic.
		validUntil = ""
		*deviceCountPtr = 0 // Alert license doesn't add devices
	case "combined":
		features = []string{"device_management", "email", "line", "telegram", "whatsapp", "discord", "slack", "camera_viewer"}
		validUntil = time.Now().AddDate(*yearsPtr, 0, 0).Format("2006-01-02")
		// If -cameras not specified, warn user
		if *cameraCountPtr == 0 {
			log.Println("Note: Combined license generated without camera count.")
			log.Println("      Use -cameras 4|9|16 to include Camera Viewer capacity.")
		}
	case "camera":
		// Camera/NVR license: enables Camera Viewer with specific camera count
		// Free: 4, Standard: 9, Enterprise: 16
		if *cameraCountPtr == 0 {
			*cameraCountPtr = 4 // Default to free tier
		}
		features = []string{"email", "camera_viewer"}
		validUntil = time.Now().AddDate(*yearsPtr, 0, 0).Format("2006-01-02")
		*deviceCountPtr = 0 // Camera license doesn't add device quota
	case "trial":
		features = []string{"email"}
		validUntil = time.Now().AddDate(0, 0, 14).Format("2006-01-02")
		*deviceCountPtr = 10
	default:
		log.Fatalf("Unknown license type: %s", licenseType)
	}

	fmt.Printf("Generating License...\n")
	fmt.Printf("Machine ID: %s\n", machineID)
	fmt.Printf("Type: %s\n", licenseType)
	fmt.Printf("Features: %v\n", features)
	if *deviceCountPtr > 0 {
		fmt.Printf("Device Count: %d\n", *deviceCountPtr)
	}
	if *cameraCountPtr > 0 {
		fmt.Printf("Camera Count: %d\n", *cameraCountPtr)
	}
	if validUntil != "" {
		fmt.Printf("Valid Until: %s\n", validUntil)
	} else {
		fmt.Printf("Valid Until: Permanent\n")
	}

	key, err := license.GenerateLicenseKeyWithCameras(
		machineID,
		*deviceCountPtr,
		*cameraCountPtr,
		features,
		validUntil,
		secretKey,
	)
	if err != nil {
		log.Fatalf("Error generating key: %v", err)
	}

	fmt.Println("\n================ LICENSE KEY ================")
	fmt.Println(key)
	fmt.Println("=============================================")
}
