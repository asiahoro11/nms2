// Made by YTSworks
// YTS工作室製作
package handlers

import "github.com/gin-gonic/gin"

func (h *Handler) ExportDevicesReportPDF(c *gin.Context) { h.reports.ExportDevicesReportPDF(c) }
func (h *Handler) ExportLogsReportPDF(c *gin.Context)    { h.reports.ExportLogsReportPDF(c) }
func (h *Handler) ExportDevicesReport(c *gin.Context)    { h.reports.ExportDevicesReport(c) }
func (h *Handler) ExportLogsReport(c *gin.Context)       { h.reports.ExportLogsReport(c) }
func (h *Handler) ExportTopTrafficReport(c *gin.Context) { h.reports.ExportTopTrafficReport(c) }
func (h *Handler) ExportDeviceHealthReport(c *gin.Context) {
	h.reports.ExportDeviceHealthReport(c)
}
func (h *Handler) ExportAvailabilityReport(c *gin.Context) { h.reports.ExportAvailabilityReport(c) }
func (h *Handler) ExportInventoryReport(c *gin.Context)    { h.reports.ExportInventoryReport(c) }
func (h *Handler) ExportInterfaceReport(c *gin.Context)    { h.reports.ExportInterfaceReport(c) }
func (h *Handler) ExportHealthTrendReport(c *gin.Context)  { h.reports.ExportHealthTrendReport(c) }
func (h *Handler) ExportSLAReport(c *gin.Context)          { h.reports.ExportSLAReport(c) }
func (h *Handler) ExportAuditReport(c *gin.Context)        { h.reports.ExportAuditReport(c) }
func (h *Handler) ExportLicenseCapacityReport(c *gin.Context) {
	h.reports.ExportLicenseCapacityReport(c)
}
func (h *Handler) ExportCameraReport(c *gin.Context)        { h.reports.ExportCameraReport(c) }
func (h *Handler) ExportPDUReport(c *gin.Context)           { h.reports.ExportPDUReport(c) }
func (h *Handler) ExportAccessControlReport(c *gin.Context) { h.reports.ExportAccessControlReport(c) }
func (h *Handler) ExportTopologyReport(c *gin.Context)      { h.reports.ExportTopologyReport(c) }
func (h *Handler) ExportEventReport(c *gin.Context)         { h.reports.ExportEventReport(c) }
func (h *Handler) ExportSyslogReport(c *gin.Context)        { h.reports.ExportSyslogReport(c) }
func (h *Handler) ExportNotificationReport(c *gin.Context)  { h.reports.ExportNotificationReport(c) }
func (h *Handler) ExportIoTDeviceReport(c *gin.Context)     { h.reports.ExportIoTDeviceReport(c) }
func (h *Handler) ExportIoTMeasurementReport(c *gin.Context) {
	h.reports.ExportIoTMeasurementReport(c)
}
func (h *Handler) ExportCameraRecordingReport(c *gin.Context) {
	h.reports.ExportCameraRecordingReport(c)
}
func (h *Handler) ExportAccessEventReport(c *gin.Context) { h.reports.ExportAccessEventReport(c) }
func (h *Handler) ExportAccessCardReport(c *gin.Context)  { h.reports.ExportAccessCardReport(c) }
func (h *Handler) ExportAccessScheduleReport(c *gin.Context) {
	h.reports.ExportAccessScheduleReport(c)
}
func (h *Handler) ExportConfigBackupReport(c *gin.Context) {
	h.reports.ExportConfigBackupReport(c)
}
