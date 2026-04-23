package notifications

import "management-server/modules/contracts"

func Manifest() contracts.ModuleManifest {
	return contracts.ModuleManifest{
		ID:                  "notifications",
		DisplayName:         "Notifications",
		Domain:              "alerting",
		OwnerPackage:        "management-server/modules/notifications",
		ExtractionStatus:    contracts.StatusExtracted,
		FeatureGate:         "alerts_global_enabled",
		ImportableIntoShell: true,
		ShellMount: &contracts.ShellMount{
			Route:         "/notifications",
			NavLabel:      "通知與告警",
			NavI18nKey:    "nav.notifications",
			FrontendEntry: "apps/frontend/js/app.js",
			BackendEntry:  "management-server/modules/notifications",
		},
		IntegrationSurfaces: []contracts.IntegrationSurface{
			{Kind: "service", Name: "alert_settings", Description: "Alert channel settings, feature gating, and channel test dispatch."},
			{Kind: "service", Name: "in_app_notifications", Description: "In-app notification inbox read, acknowledge, and delete flows."},
			{Kind: "service", Name: "mail_transport_config", Description: "Shared email transport lookup for future auth and system channels."},
		},
		DependsOn: []string{"license", "logs"},
		Notes: []string{
			"This module extracts the latest alert settings, test dispatch, inbox, and shared mail transport flows without changing existing routes.",
			"Shared mail transport config lives here so future auth email OTP can reuse transport while staying logically separate from alert dispatch.",
		},
	}
}
