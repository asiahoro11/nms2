// Made by YTSworks
// YTS工作室製作
package auth

import "management-server/modules/contracts"

func Manifest() contracts.ModuleManifest {
	return contracts.ModuleManifest{
		ID:               "auth",
		DisplayName:      "Authentication",
		Domain:           "identity",
		OwnerPackage:     "management-server/modules/auth",
		ExtractionStatus: contracts.StatusExtracted,
		IntegrationSurfaces: []contracts.IntegrationSurface{
			{Kind: "service", Name: "identity", Description: "Credential verification and current-user resolution."},
			{Kind: "service", Name: "session", Description: "JWT session issuance and authenticated context lookups."},
			{Kind: "service", Name: "password", Description: "Password change flow and future reset or recovery entry points."},
			{Kind: "service", Name: "twofactor", Description: "TOTP-first two-factor boundary with email fallback hooks."},
		},
		Notes: []string{
			"Auth now owns login, current-user, password change, password reset, and protected two-factor management routes while keeping external auth paths unchanged.",
			"TOTP enrollment, confirmation, disable, recovery-code regeneration, and password reset flows now live in the module. Email OTP remains a future fallback using shared mail transport.",
		},
	}
}
