package admin

import "management-server/modules/contracts"

func Manifest() contracts.ModuleManifest {
	return contracts.ModuleManifest{
		ID:               "admin",
		DisplayName:      "Admin",
		Domain:           "control_plane",
		OwnerPackage:     "management-server/modules/admin",
		ExtractionStatus: contracts.StatusExtracted,
		DependsOn:        []string{"auth", "license", "logs"},
		IntegrationSurfaces: []contracts.IntegrationSurface{
			{Kind: "service", Name: "user_management", Description: "User account CRUD and role administration."},
			{Kind: "service", Name: "branding", Description: "Company branding settings and logo management."},
			{Kind: "service", Name: "security_settings", Description: "Security policy orchestration including global 2FA and password expiry."},
			{Kind: "service", Name: "system_config", Description: "System configuration list and targeted config updates."},
			{Kind: "service", Name: "system_info", Description: "Runtime system identity and version metadata."},
		},
		Notes: []string{
			"Admin module now owns user management, branding, security settings, system config orchestration, and system info projection behind thin handlers.",
			"Legacy helper methods remain only as compatibility references while active routes delegate to the module service.",
		},
	}
}
