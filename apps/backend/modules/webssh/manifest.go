// Made by YTSworks
// YTS工作室製作
package webssh

import "management-server/modules/contracts"

func Manifest() contracts.ModuleManifest {
	return contracts.ModuleManifest{
		ID:                  "webssh",
		DisplayName:         "WebSSH",
		Domain:              "operations",
		OwnerPackage:        "management-server/modules/webssh",
		ExtractionStatus:    contracts.StatusExtracted,
		ImportableIntoShell: true,
		ShellMount: &contracts.ShellMount{
			Route:         "/terminal",
			NavLabel:      "WebSSH",
			NavI18nKey:    "nav.webssh",
			FrontendEntry: "apps/frontend/js/app.js",
			BackendEntry:  "management-server/modules/webssh",
		},
		DependsOn: []string{"devices"},
		IntegrationSurfaces: []contracts.IntegrationSurface{
			{Kind: "service", Name: "terminal", Description: "Device-backed SSH terminal relay over WebSocket."},
		},
		Notes: []string{
			"This extraction keeps the WebSocket route stable while moving terminal session orchestration into a reusable module.",
		},
	}
}
