// Made by YTSworks
// YTS工作室製作
package contracts

type ExtractionStatus string

const (
	StatusPlanned    ExtractionStatus = "planned"
	StatusInProgress ExtractionStatus = "in_progress"
	StatusExtracted  ExtractionStatus = "extracted"
)

type IntegrationSurface struct {
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type ShellMount struct {
	Route         string `json:"route"`
	NavLabel      string `json:"nav_label"`
	NavI18nKey    string `json:"nav_i18n_key"`
	FrontendEntry string `json:"frontend_entry"`
	BackendEntry  string `json:"backend_entry"`
}

type ModuleManifest struct {
	ID                  string               `json:"id"`
	DisplayName         string               `json:"display_name"`
	Domain              string               `json:"domain"`
	OwnerPackage        string               `json:"owner_package"`
	ExtractionStatus    ExtractionStatus     `json:"extraction_status"`
	FeatureGate         string               `json:"feature_gate,omitempty"`
	ImportableIntoShell bool                 `json:"importable_into_shell,omitempty"`
	ShellMount          *ShellMount          `json:"shell_mount,omitempty"`
	DependsOn           []string             `json:"depends_on,omitempty"`
	IntegrationSurfaces []IntegrationSurface `json:"integration_surfaces,omitempty"`
	Notes               []string             `json:"notes,omitempty"`
}
