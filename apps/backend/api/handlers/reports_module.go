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
