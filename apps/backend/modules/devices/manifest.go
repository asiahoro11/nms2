// Made by YTSworks
// YTS工作室製作
package devices

import "management-server/modules/contracts"

func Manifest() contracts.ModuleManifest {
	return contracts.ModuleManifest{
		ID:                  "devices",
		DisplayName:         "Devices",
		Domain:              "inventory",
		OwnerPackage:        "management-server/modules/devices",
		ExtractionStatus:    contracts.StatusExtracted,
		FeatureGate:         "device_management",
		ImportableIntoShell: true,
		ShellMount: &contracts.ShellMount{
			Route:         "/devices",
			NavLabel:      "設備清單",
			NavI18nKey:    "nav.devices",
			FrontendEntry: "apps/frontend/js/app.js",
			BackendEntry:  "management-server/modules/devices",
		},
		DependsOn: []string{
			"license",
			"logs",
		},
		IntegrationSurfaces: []contracts.IntegrationSurface{
			{
				Kind:        "inventory_service",
				Name:        "ListDevices/GetDevice",
				Description: "Shared inventory and device detail queries for downstream systems.",
			},
			{
				Kind:        "interfaces_service",
				Name:        "ListInterfaces",
				Description: "Interface inventory shared with topology and port-oriented modules.",
			},
			{
				Kind:        "crud_service",
				Name:        "Create/Update/Delete",
				Description: "Core device mutation layer under the existing HTTP wrapper.",
			},
			{
				Kind:        "operations_service",
				Name:        "Image/Metrics/Bulk/Poll",
				Description: "Image upload persistence, metrics read path, poll queueing checks, and bulk mutation support.",
			},
		},
		Notes: []string{
			"Inventory CRUD, image upload persistence, metrics read path, and bulk mutation flows now live in the module behind stable handlers.",
			"Thin handlers still own audit, event insertion, and alert dispatch side effects around the extracted device service.",
		},
	}
}
