package pdu

import "management-server/modules/contracts"

func Manifest() contracts.ModuleManifest {
	return contracts.ModuleManifest{
		ID:                  "pdu",
		DisplayName:         "PDU / UPS",
		Domain:              "power",
		OwnerPackage:        "management-server/modules/pdu",
		ExtractionStatus:    contracts.StatusExtracted,
		FeatureGate:         "pdu_enabled",
		ImportableIntoShell: true,
		ShellMount: &contracts.ShellMount{
			Route:         "/pdu",
			NavLabel:      "PDU/UPS 監控",
			NavI18nKey:    "nav.pdu",
			FrontendEntry: "apps/frontend/js/pdu-module.js",
			BackendEntry:  "management-server/modules/pdu",
		},
		DependsOn: []string{"license", "logs", "devices"},
		IntegrationSurfaces: []contracts.IntegrationSurface{
			{Kind: "service", Name: "inventory", Description: "PDU and UPS inventory CRUD, license status, and module visibility."},
			{Kind: "service", Name: "polling", Description: "On-demand SNMP polling with live UPS status projection."},
			{Kind: "service", Name: "runtime_status", Description: "Current battery, power, and alarm readings projected for UI or future sync adapters."},
		},
		Notes: []string{
			"This module keeps routes and payloads stable while PDU and UPS inventory, status, and poll logic live in the module.",
			"Outlet control and richer vendor-specific runtime projections can be added later without changing the external route surface.",
		},
	}
}
