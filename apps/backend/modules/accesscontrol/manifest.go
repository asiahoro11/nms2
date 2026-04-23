package accesscontrol

import "management-server/modules/contracts"

func Manifest() contracts.ModuleManifest {
	return contracts.ModuleManifest{
		ID:                  "access_control",
		DisplayName:         "Access Control",
		Domain:              "security",
		OwnerPackage:        "management-server/modules/accesscontrol",
		ExtractionStatus:    contracts.StatusExtracted,
		FeatureGate:         "access_control_enabled",
		ImportableIntoShell: true,
		ShellMount: &contracts.ShellMount{
			Route:         "/access-control",
			NavLabel:      "門禁管理",
			NavI18nKey:    "nav.access_control",
			FrontendEntry: "apps/frontend/js/access-control-module.js",
			BackendEntry:  "management-server/modules/accesscontrol",
		},
		DependsOn: []string{"license", "logs"},
		IntegrationSurfaces: []contracts.IntegrationSurface{
			{Kind: "service", Name: "doors", Description: "Door inventory, control, and status operations."},
			{Kind: "service", Name: "cards", Description: "Card inventory and holder lifecycle management."},
			{Kind: "service", Name: "events", Description: "Access event query and manual event injection."},
			{Kind: "service", Name: "schedules", Description: "Card access schedules, approval, and runtime access checks."},
		},
		Notes: []string{
			"Access control keeps current routes and payloads stable while door, card, event, and schedule logic live in the module.",
			"Thin handlers still perform HTTP binding and license freshness checks before calling the module.",
		},
	}
}
