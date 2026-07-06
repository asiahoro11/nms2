// Made by YTSworks
// YTS工作室製作
package iot

import "management-server/modules/contracts"

func Manifest() contracts.ModuleManifest {
	return contracts.ModuleManifest{
		ID:                  "iot",
		DisplayName:         "IoT / Modbus",
		Domain:              "iot",
		OwnerPackage:        "management-server/modules/iot",
		ExtractionStatus:    contracts.StatusExtracted,
		FeatureGate:         "iot_enabled",
		ImportableIntoShell: true,
		ShellMount: &contracts.ShellMount{
			Route:         "/iot",
			NavLabel:      "IoT / Modbus",
			NavI18nKey:    "nav.iot",
			FrontendEntry: "apps/frontend/js/iot.js",
			BackendEntry:  "management-server/modules/iot",
		},
		DependsOn: []string{"license", "devices"},
		IntegrationSurfaces: []contracts.IntegrationSurface{
			{Kind: "service", Name: "devices", Description: "Modbus TCP/RTU and REST ingest device CRUD with per-device polling schedules."},
			{Kind: "service", Name: "measurements", Description: "Time-series measurement store with event deduplication via UUID."},
			{Kind: "service", Name: "forwarder", Description: "Store-and-forward HTTP queue with retry backoff and configurable retention."},
		},
	}
}
