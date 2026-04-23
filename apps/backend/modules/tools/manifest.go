package tools

import "management-server/modules/contracts"

func Manifest() contracts.ModuleManifest {
	return contracts.ModuleManifest{
		ID:                  "tools",
		DisplayName:         "Tools",
		Domain:              "operations",
		OwnerPackage:        "management-server/modules/tools",
		ExtractionStatus:    contracts.StatusExtracted,
		ImportableIntoShell: true,
		ShellMount: &contracts.ShellMount{
			Route:         "/tools",
			NavLabel:      "工具箱",
			NavI18nKey:    "nav.tools",
			FrontendEntry: "apps/frontend/js/tools.js",
			BackendEntry:  "management-server/modules/tools",
		},
		IntegrationSurfaces: []contracts.IntegrationSurface{
			{Kind: "service", Name: "ping", Description: "Validated multi-target ping execution for operator diagnostics."},
			{Kind: "service", Name: "traceroute", Description: "Validated traceroute execution with normalized output decoding."},
		},
		Notes: []string{
			"This extraction keeps existing routes stable while moving execution and validation logic into a reusable diagnostics module.",
		},
	}
}
