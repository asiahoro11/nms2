// Dashboard 功能

async function loadDashboard() {
    try {
        const response = await apiGet('/dashboard');
        if (response.success) {
            renderDashboard(response.data);
        }
    } catch (error) {
        showToast(t('dashboard.toast.load_failed'), 'error');
    }
}

async function refreshDashboard() {
    // 手動重新整理：完整重新載入頁面
    window.location.reload(true);
}

async function autoRefreshDashboard() {
    // 自動重新整理：只更新數據，不重新載入頁面
    try {
        const response = await apiGet('/dashboard');
        if (response.success) {
            renderDashboard(response.data);
        }

        // 如果當前不是顯示流量，則需要額外更新當前的 Top 5 列表
        if (currentTopType !== 'traffic') {
            switchTopDevices(currentTopType);
        }
    } catch (error) {
        console.error('Auto refresh dashboard failed:', error);
        // 自動重新整理失敗時不顯示錯誤訊息，避免干擾使用者
    }
}

function renderDashboard(data) {
    // 更新系統版本
    document.getElementById('system-version').textContent = data.version || (typeof getAppVersion === 'function' ? getAppVersion() : '');

    // 更新統計卡片
    animateCounter('total-devices', data.total_devices);
    animateCounter('online-devices', data.online_count);
    animateCounter('offline-devices', data.offline_count);
    document.getElementById('memory-usage').textContent = `${data.system_stats.memory_usage.toFixed(1)} MB`;

    // 更新設備類型分布
    renderDeviceTypes(data.device_types);

    // 更新最近事件
    renderRecentEvents(data.recent_events);

    // 更新 Top 5 流量設備 (僅在當前為流量視圖時更新)
    if (currentTopType === 'traffic') {
        renderTopDevices(data.top_devices);
    }
}

function animateCounter(elementId, targetValue) {
    const element = document.getElementById(elementId);
    if (!element) return;
    const startValue = parseInt(element.textContent) || 0;
    const duration = 500;
    const startTime = performance.now();

    function update(currentTime) {
        const elapsed = currentTime - startTime;
        const progress = Math.min(elapsed / duration, 1);

        // Easing function
        const easeOutQuart = 1 - Math.pow(1 - progress, 4);
        const currentValue = Math.floor(startValue + (targetValue - startValue) * easeOutQuart);

        element.textContent = currentValue;

        if (progress < 1) {
            requestAnimationFrame(update);
        }
    }

    requestAnimationFrame(update);
}

// Local storage key for device type order
const STORAGE_KEY_DEVICE_TYPE_ORDER = 'nms_dashboard_device_type_order';

function renderDeviceTypes(types) {
    const container = document.getElementById('device-types-chart');
    if (!types || Object.keys(types).length === 0) {
        container.innerHTML = `<p class="empty-message">${t('devices.no_data')}</p>`;
        return;
    }

    const typeLabels = {
        router: t('devices.type_router'),
        switch: t('devices.type_switch'),
        access_point: t('devices.type_ap'),
        firewall: t('devices.type_firewall'),
        server: t('devices.type_server'),
        codec: t('devices.type_codec'),
        ipcam: t('devices.type_ipcam'),
        video_wall: t('devices.type_videowall'),
        access_control: t('devices.type_access_control'),
        ups: t('devices.type_ups'),
        pdu: t('devices.type_pdu'),
        other: t('devices.type_other'),
        unknown: t('devices.type_other')
    };

    // Determine order
    let order = [];
    try {
        const saved = localStorage.getItem(STORAGE_KEY_DEVICE_TYPE_ORDER);
        if (saved) order = JSON.parse(saved);
    } catch (e) { console.warn('Failed to load order', e); }

    // Get all current types
    const currentTypes = Object.keys(types);

    // Sort logic
    const validOrder = order.filter(t => currentTypes.includes(t));
    const newItems = currentTypes.filter(t => !validOrder.includes(t));
    const finalSort = [...validOrder, ...newItems];

    let html = '';
    for (const type of finalSort) {
        const count = types[type];
        const iconHtml = getDeviceTypeIcon(type);

        html += `
            <div class="device-type-item" draggable="true" data-type="${type}">
                <div class="device-type-icon">${iconHtml}</div>
                <div class="device-type-info">
                    <h4>${typeLabels[type] || type}</h4>
                    <p>${count}</p>
                </div>
            </div>
        `;
    }

    container.innerHTML = html;
    setupDeviceTypeDnD(container);
}

function setupDeviceTypeDnD(container) {
    let draggedItem = null;
    const items = container.querySelectorAll('.device-type-item');
    items.forEach(item => {
        item.addEventListener('dragstart', function (e) {
            draggedItem = this;
            setTimeout(() => this.style.opacity = '0.5', 0);
            e.dataTransfer.effectAllowed = 'move';
        });

        item.addEventListener('dragend', function () {
            this.style.opacity = '1';
            draggedItem = null;
            items.forEach(i => i.classList.remove('drag-over'));
            saveDeviceTypeOrder(container);
        });

        item.addEventListener('dragover', function (e) {
            e.preventDefault();
            e.dataTransfer.dropEffect = 'move';
            this.classList.add('drag-over');
        });

        item.addEventListener('dragleave', function () {
            this.classList.remove('drag-over');
        });

        item.addEventListener('drop', function (e) {
            e.preventDefault();
            this.classList.remove('drag-over');
            if (this !== draggedItem) {
                const allItems = [...container.querySelectorAll('.device-type-item')];
                const draggedIdx = allItems.indexOf(draggedItem);
                const droppedIdx = allItems.indexOf(this);
                if (draggedIdx < droppedIdx) {
                    this.after(draggedItem);
                } else {
                    this.before(draggedItem);
                }
            }
        });
    });
}

function saveDeviceTypeOrder(container) {
    const order = [];
    container.querySelectorAll('.device-type-item').forEach(item => {
        order.push(item.dataset.type);
    });
    localStorage.setItem(STORAGE_KEY_DEVICE_TYPE_ORDER, JSON.stringify(order));
}

function renderRecentEvents(events) {
    const container = document.getElementById('recent-events');
    if (!events || events.length === 0) {
        container.innerHTML = `<p class="empty-message">${t('dashboard.events.empty')}</p>`;
        return;
    }

    let html = '';
    for (const event of events) {
        html += `
            <div class="event-item">
                <div class="event-severity ${event.severity}"></div>
                <div class="event-content">
                    <div class="event-message">${escapeHtml(event.message)}</div>
                    <div class="event-time">${formatRelativeTime(event.created_at)}</div>
                </div>
            </div>
        `;
    }
    container.innerHTML = html;
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function renderTopDevices(devices) {
    const container = document.getElementById('top-devices-list');
    if (!devices || devices.length === 0) {
        container.innerHTML = `<p class="empty-message">${t('dashboard.top5.empty')}</p>`;
        return;
    }

    let html = '<div class="top-list">';
    const maxTraffic = devices[0].total_traffic || 1;

    devices.forEach((device, index) => {
        const traffic = formatBits(device.total_traffic);
        const width = (device.total_traffic / maxTraffic) * 100;

        html += `
            <div class="top-item">
                <div class="top-item-row">
                    <div class="top-rank">#${index + 1}</div>
                    <div class="top-info">
                        <div class="top-name-ip">
                            <span class="top-name">${escapeHtml(device.name)}</span>
                            ${device.snmp_community ? `<span class="top-ip" style="cursor: pointer; color: var(--primary-color); text-decoration: underline;" onclick="window.open('//${device.ip_address}', '_blank')" title="${t('dashboard.top5.connect_hint')}">${device.ip_address}</span>` : `<span class="top-ip">${device.ip_address}</span>`}
                        </div>
                        <div class="top-bar-container">
                            <div class="top-bar-fill" style="width: ${width}%"></div>
                        </div>
                    </div>
                    <div class="top-value-group">
                        <div class="top-main-value">${traffic}</div>
                        <div class="top-secondary-values">
                            <span>CPU: ${device.cpu_usage.toFixed(1)}%</span>
                            <span>Mem: ${device.memory_usage.toFixed(1)}%</span>
                        </div>
                    </div>
                </div>
            </div>
        `;
    });
    html += '</div>';
    container.innerHTML = html;
}

function formatBits(bits) {
    if (!bits) return '0 bps';
    if (bits < 1000) return bits + " bps";
    if (bits < 1000000) return (bits / 1000).toFixed(1) + " Kbps";
    if (bits < 1000000000) return (bits / 1000000).toFixed(1) + " Mbps";
    return (bits / 1000000000).toFixed(1) + " Gbps";
}

let currentTopType = 'traffic';

async function switchTopDevices(type) {
    currentTopType = type;
    document.querySelectorAll('.top-switch-btn').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.type === type);
    });

    const title = document.getElementById('top-devices-title');
    const icons = { traffic: '🔥', cpu: '⚙️', memory: '💾' };
    const labels = {
        traffic: t('dashboard.top5.title_traffic'),
        cpu: t('dashboard.top5.title_cpu'),
        memory: t('dashboard.top5.title_memory')
    };
    if (title) {
        title.textContent = `${icons[type]} ${labels[type]}`;
        const i18nKeys = { traffic: 'dashboard.top5.title_traffic', cpu: 'dashboard.top5.title_cpu', memory: 'dashboard.top5.title_memory' };
        title.setAttribute('data-i18n', i18nKeys[type]);
    }

    try {
        let response;
        if (type === 'traffic') {
            response = await apiGet('/dashboard');
            if (response.success) renderTopDevices(response.data.top_devices);
        } else if (type === 'cpu') {
            response = await apiGet('/dashboard/top-cpu');
            if (response.success) renderTopMetricDevices(response.data, 'CPU');
        } else if (type === 'memory') {
            response = await apiGet('/dashboard/top-memory');
            if (response.success) renderTopMetricDevices(response.data, 'Memory');
        }
    } catch (error) {
        console.error('Failed to load top devices:', error);
        const container = document.getElementById('top-devices-list');
        if (container) container.innerHTML = `<p class="empty-message">${t('common.error')}</p>`;
    }
}

function renderTopMetricDevices(devices, metricType) {
    const container = document.getElementById('top-devices-list');
    if (!devices || devices.length === 0) {
        container.innerHTML = `<p class="empty-message">${t('dashboard.top5.empty_metric')}</p>`;
        return;
    }

    let html = '<div class="top-list">';
    const maxValue = devices[0].value || 1;
    devices.forEach((device, index) => {
        const value = device.value.toFixed(1);
        const width = (device.value / maxValue) * 100;
        html += `
            <div class="top-item">
                <div class="top-item-row">
                    <div class="top-rank">#${index + 1}</div>
                    <div class="top-info">
                        <div class="top-name-ip">
                            <span class="top-name">${escapeHtml(device.name)}</span>
                            ${device.snmp_community ? `<span class="top-ip" style="cursor: pointer; color: var(--primary-color); text-decoration: underline;" onclick="window.open('//${device.ip_address}', '_blank')" title="${t('dashboard.top5.connect_hint')}">${device.ip_address}</span>` : `<span class="top-ip">${device.ip_address}</span>`}
                        </div>
                        <div class="top-bar-container">
                            <div class="top-bar-fill" style="width: ${width}%"></div>
                        </div>
                    </div>
                    <div class="top-value-group">
                        <div class="top-main-value">${value}%</div>
                        <div class="top-secondary-values">
                            <span>Traffic: ${formatBits(device.total_traffic)}</span>
                            <span>${metricType === 'CPU' ? 'Mem: ' + device.memory_usage.toFixed(1) + '%' : 'CPU: ' + device.cpu_usage.toFixed(1) + '%'}</span>
                        </div>
                    </div>
                </div>
            </div>
        `;
    });
    html += '</div>';
    container.innerHTML = html;
}

window.addEventListener('languageChanged', (e) => {
    if (document.getElementById('dashboard') && document.getElementById('dashboard').classList.contains('active')) {
        autoRefreshDashboard();
        switchTopDevices(currentTopType);
    }
});
