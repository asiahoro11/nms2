package license

import "management-server/modules/contracts"

func Manifest() contracts.ModuleManifest {
	return contracts.ModuleManifest{
		ID:                  "license",
		DisplayName:         "License",
		Domain:              "entitlement",
		OwnerPackage:        "management-server/modules/license",
		ExtractionStatus:    contracts.StatusExtracted,
		FeatureGate:         "license_center",
		ImportableIntoShell: true,
		ShellMount: &contracts.ShellMount{
			Route:         "/license",
			NavLabel:      "授權管理",
			NavI18nKey:    "nav.license",
			FrontendEntry: "apps/frontend/js/admin.js",
			BackendEntry:  "management-server/modules/license",
		},
		IntegrationSurfaces: []contracts.IntegrationSurface{
			{Kind: "service", Name: "status", Description: "Current license status, limits, and active license inventory."},
			{Kind: "service", Name: "feature_gate", Description: "Feature enablement boundary shared by all add-on modules."},
			{Kind: "service", Name: "entitlement_runtime", Description: "License-aware device-management and module gating decisions."},
			{Kind: "service", Name: "activation", Description: "License activation, generation, reset, and reissue orchestration."},
		},
		Notes: []string{
			"Status queries, feature-gate calculations, activation, generation, reissue, and legacy admin license inventory now route through the module service.",
			"This module is the shared entitlement boundary for future shell-based systems such as nms_sync2.",
		},
	}
}
