// Made by YTSworks
// YTS工作室製作
package logs

import "management-server/modules/contracts"

func Manifest() contracts.ModuleManifest {
	return contracts.ModuleManifest{
		ID:                  "logs",
		DisplayName:         "Logs And Audit",
		Domain:              "observability",
		OwnerPackage:        "management-server/modules/logs",
		ExtractionStatus:    contracts.StatusExtracted,
		FeatureGate:         "logs_center",
		ImportableIntoShell: true,
		ShellMount: &contracts.ShellMount{
			Route:         "/logs",
			NavLabel:      "日誌與稽核",
			NavI18nKey:    "nav.logs",
			FrontendEntry: "apps/frontend/js/logs.js",
			BackendEntry:  "management-server/modules/logs",
		},
		IntegrationSurfaces: []contracts.IntegrationSurface{
			{Kind: "service", Name: "audit", Description: "Unified audit log queries and review workflow."},
			{Kind: "service", Name: "system_logs", Description: "Unified system log queries and review workflow."},
			{Kind: "service", Name: "device_logs", Description: "Unified device log queries and acknowledge workflow."},
			{Kind: "service", Name: "config_change_logs", Description: "Unified config change queries and review workflow."},
			{Kind: "service", Name: "evidence_bundle", Description: "Export and evidence bundle data access for the latest log center."},
		},
		Notes: []string{
			"The latest consolidated log center is extracted here while legacy syslogs and events compatibility routes remain available in handlers.",
			"Future shell pages can mount this module directly by manifest without reimplementing filters, review workflows, or export logic.",
		},
	}
}
