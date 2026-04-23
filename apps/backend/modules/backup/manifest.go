package backup

import "management-server/modules/contracts"

func Manifest() contracts.ModuleManifest {
	return contracts.ModuleManifest{
		ID:               "backup",
		DisplayName:      "Backup",
		Domain:           "system",
		OwnerPackage:     "management-server/modules/backup",
		ExtractionStatus: contracts.StatusExtracted,
		DependsOn:        []string{"logs"},
		IntegrationSurfaces: []contracts.IntegrationSurface{
			{Kind: "service", Name: "system_backup", Description: "System backup export, restore preparation, and restore-readiness checks."},
			{Kind: "service", Name: "encrypted_backup", Description: "Encrypted backup export, restore, and password rotation orchestration."},
			{Kind: "service", Name: "device_config_backup", Description: "Device configuration backup save, list, and download workflows."},
		},
		Notes: []string{
			"Backup module now owns system backup, encrypted backup, and device config backup workflows behind thin handlers.",
			"Restore restart orchestration remains in the existing handler layer, but the archive and persistence logic is module-owned.",
		},
	}
}
