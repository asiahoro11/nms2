// Made by YTSworks
// YTS工作室製作
package reports

import "management-server/modules/contracts"

func Manifest() contracts.ModuleManifest {
	return contracts.ModuleManifest{
		ID:                  "reports",
		DisplayName:         "Reports",
		Domain:              "reporting",
		OwnerPackage:        "management-server/modules/reports",
		ExtractionStatus:    contracts.StatusExtracted,
		ImportableIntoShell: true,
		ShellMount: &contracts.ShellMount{
			Route:         "/reports",
			NavLabel:      "報表中心",
			NavI18nKey:    "nav.reports",
			FrontendEntry: "apps/frontend/js/reports.js",
			BackendEntry:  "management-server/modules/reports",
		},
		DependsOn: []string{"devices", "logs"},
		IntegrationSurfaces: []contracts.IntegrationSurface{
			{Kind: "service", Name: "devices_report", Description: "Device inventory and status report exports in CSV, JSON, and PDF forms."},
			{Kind: "service", Name: "logs_report", Description: "System log and event report exports across CSV, JSON, and PDF outputs."},
			{Kind: "service", Name: "traffic_health_inventory", Description: "Traffic, health, availability, and inventory report generation."},
		},
		Notes: []string{
			"This module keeps the existing report routes stable while centralizing rendering in a reusable reporting service.",
		},
	}
}
