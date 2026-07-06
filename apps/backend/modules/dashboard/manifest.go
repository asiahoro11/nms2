// Made by YTSworks
// YTS工作室製作
package dashboard

import "management-server/modules/contracts"

func Manifest() contracts.ModuleManifest {
	return contracts.ModuleManifest{
		ID:                  "dashboard",
		DisplayName:         "Dashboard",
		Domain:              "workspace",
		OwnerPackage:        "management-server/modules/dashboard",
		ExtractionStatus:    contracts.StatusExtracted,
		ImportableIntoShell: true,
		ShellMount: &contracts.ShellMount{
			Route:         "/dashboard",
			NavLabel:      "儀表板",
			NavI18nKey:    "nav.dashboard",
			FrontendEntry: "apps/frontend/js/dashboard.js",
			BackendEntry:  "management-server/modules/dashboard",
		},
		DependsOn: []string{"devices", "topology", "camera", "logs"},
		IntegrationSurfaces: []contracts.IntegrationSurface{
			{Kind: "projection", Name: "dashboard_summary", Description: "Operational summary projection for the main workspace shell."},
			{Kind: "projection", Name: "top_cpu", Description: "Top CPU device projection for dashboard widgets."},
			{Kind: "projection", Name: "top_memory", Description: "Top memory device projection for dashboard widgets."},
		},
		Notes: []string{
			"This module keeps the existing dashboard routes stable while summary queries and widget projections live in the module.",
		},
	}
}
