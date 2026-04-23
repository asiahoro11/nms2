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
