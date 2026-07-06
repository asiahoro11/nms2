// Made by YTSworks
// YTS工作室製作
// 進階報表匯出功能

function reportLangParam() {
    const lang = (typeof currentLang !== 'undefined' && currentLang) ? currentLang : 'zh-TW';
    return `lang=${encodeURIComponent(lang)}`;
}

// 匯出 Top Traffic 報表
function exportTrafficReport(format) {
    const limit = 10;
    if (format === 'pdf') {
        apiDownload(`/reports/traffic?format=pdf&limit=${limit}&${reportLangParam()}`);
    } else {
        apiDownload(`/reports/traffic?format=csv&limit=${limit}&${reportLangParam()}`);
    }
    showToast(t('admin.reports.traffic_exporting'), 'info');
}

// 匯出設備健康度報表
function exportHealthReport(format) {
    if (format === 'pdf') {
        apiDownload(`/reports/health?format=pdf&${reportLangParam()}`);
    } else {
        apiDownload(`/reports/health?format=csv&${reportLangParam()}`);
    }
    showToast(t('admin.reports.health_exporting'), 'info');
}

// 匯出可用性報表
function exportAvailabilityReport(format) {
    if (format === 'pdf') {
        apiDownload(`/reports/availability?format=pdf&${reportLangParam()}`);
    } else {
        apiDownload(`/reports/availability?format=csv&${reportLangParam()}`);
    }
    showToast(t('admin.reports.availability_exporting'), 'info');
}

// 匯出資產清單
function exportInventoryReport(format) {
    if (format === 'pdf') {
        apiDownload(`/reports/inventory?format=pdf&${reportLangParam()}`);
    } else {
        apiDownload(`/reports/inventory?format=csv&${reportLangParam()}`);
    }
    showToast(t('admin.reports.inventory_exporting'), 'info');
}

function exportStandardReport(report, format) {
    const safeFormat = format === 'pdf' || format === 'json' ? format : 'csv';
    apiDownload(`/reports/${report}?format=${safeFormat}&${reportLangParam()}`);
    showToast(t('app.toast.exporting'), 'info');
}

function exportInterfaceReport(format) {
    exportStandardReport('interfaces', format);
}

function exportHealthTrendReport(format) {
    exportStandardReport('health-trend', format);
}

function exportSLAReport(format) {
    exportStandardReport('sla', format);
}

function exportAuditReport(format) {
    exportStandardReport('audit', format);
}

function exportLicenseCapacityReport(format) {
    exportStandardReport('license-capacity', format);
}

function exportCameraReport(format) {
    exportStandardReport('cameras', format);
}

function exportPDUReport(format) {
    exportStandardReport('pdu', format);
}

function exportAccessControlReport(format) {
    exportStandardReport('access-control', format);
}
