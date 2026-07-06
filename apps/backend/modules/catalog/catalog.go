// Made by YTSworks
// YTS工作室製作
package catalog

import (
	acmodule "management-server/modules/accesscontrol"
	adminmodule "management-server/modules/admin"
	authmodule "management-server/modules/auth"
	backupmodule "management-server/modules/backup"
	cameramodule "management-server/modules/camera"
	"management-server/modules/contracts"
	dashboardmodule "management-server/modules/dashboard"
	devicesmodule "management-server/modules/devices"
	iotmodule "management-server/modules/iot"
	licensemodule "management-server/modules/license"
	logsmodule "management-server/modules/logs"
	notificationsmodule "management-server/modules/notifications"
	pdumodule "management-server/modules/pdu"
	reportsmodule "management-server/modules/reports"
	toolsmodule "management-server/modules/tools"
	topologymodule "management-server/modules/topology"
	websshmodule "management-server/modules/webssh"
)

func PlannedManifests() []contracts.ModuleManifest {
	manifests := []contracts.ModuleManifest{
		authmodule.Manifest(),
		dashboardmodule.Manifest(),
		devicesmodule.Manifest(),
		topologymodule.Manifest(),
		cameramodule.Manifest(),
		acmodule.Manifest(),
		iotmodule.Manifest(),
		pdumodule.Manifest(),
		logsmodule.Manifest(),
		licensemodule.Manifest(),
		backupmodule.Manifest(),
		notificationsmodule.Manifest(),
		reportsmodule.Manifest(),
		toolsmodule.Manifest(),
		websshmodule.Manifest(),
		adminmodule.Manifest(),
	}

	return manifests
}

func ImportableManifests() []contracts.ModuleManifest {
	all := PlannedManifests()
	out := make([]contracts.ModuleManifest, 0, len(all))
	for _, manifest := range all {
		if manifest.ImportableIntoShell {
			out = append(out, manifest)
		}
	}
	return out
}

func ManifestByID(id string) (contracts.ModuleManifest, bool) {
	for _, manifest := range PlannedManifests() {
		if manifest.ID == id {
			return manifest, true
		}
	}
	return contracts.ModuleManifest{}, false
}
