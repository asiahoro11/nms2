package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"management-server/cmd/internal/licensegen"
	"management-server/services/license"
)

func main() {
	machineID := flag.String("id", "", "Target Machine ID. Required.")
	licenseType := flag.String("type", "device", "Buyout type: device, alert, camera, camera_recording, access_control, pdu, iot, combined, full, custom.")
	featureCSV := flag.String("features", "", "Comma-separated feature keys for custom buyout licenses.")
	deviceCount := flag.Int("count", 0, "Device count. Required when device_management is selected.")
	cameraCount := flag.Int("cameras", 0, "Camera count. Required when camera_viewer or camera_recording is selected.")
	flag.Parse()

	if strings.TrimSpace(*machineID) == "" {
		log.Fatal("Error: Machine ID is required. Use -id <MACHINE_ID>")
	}

	result, err := licensegen.Generate(licensegen.GenerateInput{
		Mode:        license.FormalLicenseMode,
		MachineID:   *machineID,
		LicenseType: *licenseType,
		Features:    parseCSV(*featureCSV),
		DeviceCount: *deviceCount,
		CameraCount: *cameraCount,
		Years:       license.PermanentYearCut,
	})
	if err != nil {
		log.Fatalf("Error generating buyout license: %v", err)
	}

	fmt.Printf("Generating Management System %s Buyout License\n", licensegen.ProductVersion)
	fmt.Printf("Machine ID: %s\n", strings.ToLower(strings.TrimSpace(*machineID)))
	fmt.Printf("Type: %s\n", result.LicenseType)
	fmt.Printf("Features: %v\n", result.Features)
	fmt.Printf("Device Count: %d\n", result.DeviceCount)
	fmt.Printf("Camera Count: %d\n", result.CameraCount)
	fmt.Printf("Valid Until: Permanent\n")

	fmt.Println("\n================ BUYOUT LICENSE KEY ================")
	fmt.Println(result.Key)
	fmt.Println("====================================================")
}

func parseCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	return out
}
