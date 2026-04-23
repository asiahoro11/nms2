package topology

import "management-server/modules/contracts"

func Manifest() contracts.ModuleManifest {
	return contracts.ModuleManifest{
		ID:                  "topology",
		DisplayName:         "Topology",
		Domain:              "graph",
		OwnerPackage:        "management-server/modules/topology",
		ExtractionStatus:    contracts.StatusExtracted,
		FeatureGate:         "device_management",
		ImportableIntoShell: true,
		ShellMount: &contracts.ShellMount{
			Route:         "/topology",
			NavLabel:      "網路拓樸",
			NavI18nKey:    "nav.topology",
			FrontendEntry: "apps/frontend/js/topology.js",
			BackendEntry:  "management-server/modules/topology",
		},
		DependsOn: []string{
			"devices",
			"interfaces",
		},
		IntegrationSurfaces: []contracts.IntegrationSurface{
			{
				Kind:        "internal_canonical_graph",
				Name:        "BuildCanonicalGraph",
				Description: "Canonical graph truth for future export, sync, and integration adapters.",
			},
			{
				Kind:        "legacy_projection",
				Name:        "LoadGraph",
				Description: "Legacy topology projection preserved for existing admin and monitor flows.",
			},
			{
				Kind:        "change_tracking",
				Name:        "topology_change_logs",
				Description: "Internal mutation trail for links and layout changes.",
			},
			{
				Kind:        "layout_snapshot",
				Name:        "topology_layout_snapshots",
				Description: "Internal layout snapshots for future restore/export workflows.",
			},
		},
		Notes: []string{
			"External routes stay unchanged while internals move into modules/topology.",
			"Canonical graph is the internal source of truth; legacy payloads are projected from it.",
		},
	}
}
