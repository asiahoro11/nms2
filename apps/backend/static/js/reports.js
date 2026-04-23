// 進階報表匯出功能

// 匯出 Top Traffic 報表
function exportTrafficReport(format) {
    const limit = 10;
    if (format === 'pdf') {
        apiDownload(`/reports/traffic?format=pdf&limit=${limit}`);
    } else {
        apiDownload(`/reports/traffic?format=csv&limit=${limit}`);
    }
    showToast(t('admin.reports.traffic_exporting'), 'info');
}

// 匯出設備健康度報表
function exportHealthReport(format) {
    if (format === 'pdf') {
        apiDownload(`/reports/health?format=pdf`);
    } else {
        apiDownload(`/reports/health?format=csv`);
    }
    showToast(t('admin.reports.health_exporting'), 'info');
}

// 匯出可用性報表
function exportAvailabilityReport(format) {
    if (format === 'pdf') {
        apiDownload(`/reports/availability?format=pdf`);
    } else {
        apiDownload(`/reports/availability?format=csv`);
    }
    showToast(t('admin.reports.availability_exporting'), 'info');
}

// 匯出資產清單
function exportInventoryReport(format) {
    if (format === 'pdf') {
        apiDownload(`/reports/inventory?format=pdf`);
    } else {
        apiDownload(`/reports/inventory?format=csv`);
    }
    showToast(t('admin.reports.inventory_exporting'), 'info');
}
