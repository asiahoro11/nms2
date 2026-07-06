// Made by YTSworks
// YTS工作室製作
package camera

import "management-server/modules/contracts"

func Manifest() contracts.ModuleManifest {
	return contracts.ModuleManifest{
		ID:                  "camera",
		DisplayName:         "Camera",
		Domain:              "video",
		OwnerPackage:        "management-server/modules/camera",
		ExtractionStatus:    contracts.StatusExtracted,
		FeatureGate:         "camera_viewer_enabled",
		ImportableIntoShell: true,
		ShellMount: &contracts.ShellMount{
			Route:         "/cameras",
			NavLabel:      "攝影機監控",
			NavI18nKey:    "nav.cameras",
			FrontendEntry: "apps/frontend/js/camera-module.js",
			BackendEntry:  "management-server/modules/camera",
		},
		DependsOn: []string{"license", "logs"},
		IntegrationSurfaces: []contracts.IntegrationSurface{
			{
				Kind:        "service",
				Name:        "inventory",
				Description: "Camera inventory CRUD, schema migration, and camera module count checks.",
			},
			{
				Kind:        "service",
				Name:        "monitor_projection",
				Description: "Monitor camera list projection and monitor-display mutation under stable existing routes.",
			},
			{
				Kind:        "service",
				Name:        "discovery_health",
				Description: "Camera discovery candidate scan, single-camera health checks, and offline recovery probing.",
			},
			{
				Kind:        "service",
				Name:        "preview_runtime",
				Description: "Snapshot, MJPEG runtime, NVR metadata/config, and recording session lifecycle under stable existing routes.",
			},
		},
		Notes: []string{
			"Schema guard, license gate, inventory CRUD, monitor-display projection, recording metadata/config, batch credential updates, discovery, health probing, ffmpeg helper utilities, RTSP auth helpers, ONVIF probe helpers, snapshot execution, MJPEG runtime, and recording session lifecycle now live in the module.",
			"handlers/camera.go still keeps temporary legacy compatibility helpers, but active routes now delegate runtime ownership to the module.",
		},
	}
}
