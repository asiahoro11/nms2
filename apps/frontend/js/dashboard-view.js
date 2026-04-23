// Dashboard View - 戰情畫面邏輯
// Version: 1.2.1 — Camera Monitor added

let dashboardData = null;
let updateInterval = null;
let topologySimulation = null;

// Camera state
let dvCameras = [];        // all cameras
let dvCamLayout = 4;       // 4 | 9 | 16
let dvCamInterval = null;  // snapshot refresh timer (fallback)

// ========== 初始化 ==========
document.addEventListener('DOMContentLoaded', () => {
    // 檢查登入狀態
    checkAuth();

    // 初始化 i18n
    if (typeof initI18n === 'function') {
        initI18n();
    }

    // 自動進入全螢幕
    setTimeout(() => {
        enterFullscreen();
    }, 500);

    // 載入資料
    loadDashboardData();

    // 初始化攝影機監控
    dvInitCameras();

    // 設定自動更新（每 10 秒）
    updateInterval = setInterval(() => {
        loadDashboardData();
    }, 10000);

    // 監聽全螢幕變化
    document.addEventListener('fullscreenchange', updateFullscreenIcon);
    document.addEventListener('webkitfullscreenchange', updateFullscreenIcon);
});

// ========== 認證檢查 ==========
function checkAuth() {
    // Support both localStorage (legacy) and sessionStorage (current NMS)
    const token = sessionStorage.getItem('nms_token') || localStorage.getItem('token');
    if (!token) {
        navigateToFrontendRoute('/login');
        return false;
    }
    return true;
}

function dvGetToken() {
    return sessionStorage.getItem('nms_token') || localStorage.getItem('token') || '';
}

// ========== 全螢幕控制 ==========
function enterFullscreen() {
    const elem = document.documentElement;
    if (elem.requestFullscreen) {
        elem.requestFullscreen().catch(err => {
            console.log('Fullscreen request failed:', err);
        });
    } else if (elem.webkitRequestFullscreen) {
        elem.webkitRequestFullscreen();
    } else if (elem.msRequestFullscreen) {
        elem.msRequestFullscreen();
    }
}

function exitFullscreen() {
    if (document.exitFullscreen) {
        document.exitFullscreen();
    } else if (document.webkitExitFullscreen) {
        document.webkitExitFullscreen();
    } else if (document.msExitFullscreen) {
        document.msExitFullscreen();
    }
}

function toggleFullscreen() {
    if (isFullscreen()) {
        exitFullscreen();
    } else {
        enterFullscreen();
    }
}

function isFullscreen() {
    return !!(document.fullscreenElement || document.webkitFullscreenElement || document.msFullscreenElement);
}

function updateFullscreenIcon() {
    const icon = document.getElementById('fullscreen-icon');
    if (icon) {
        icon.textContent = isFullscreen() ? '⛶' : '⛶';
    }
}

// ========== 資料載入 ==========
async function loadDashboardData() {
    try {
        const [topologyRes, devicesRes] = await Promise.all([
            apiGet('/topology'),
            apiGet('/devices?page=1&limit=1000'),
        ]);

        if (topologyRes.success) {
            updateTopology(topologyRes.data);
        }

        if (devicesRes.success) {
            updateDeviceStatus(devicesRes.data);
        }

        updateLastUpdateTime();

    } catch (error) {
        console.error('[Dashboard] Load failed:', error);
    }
}

// ========== 攝影機監控 ==========

async function dvInitCameras() {
    try {
        // Check camera license
        const statusRes = await apiGet('/cameras/status');
        if (!statusRes || !statusRes.success || !statusRes.data || !statusRes.data.licensed) {
            return; // No license — hide camera section
        }

        // Load cameras
        const camRes = await apiGet('/cameras');
        if (!camRes || !camRes.success || !camRes.data) return;
        dvCameras = (camRes.data.cameras || camRes.data || []).filter(c => c.status === 'online' || c.is_enabled);

        if (dvCameras.length === 0) return;

        // Show camera section and layout controls
        const section = document.getElementById('camera-section');
        const ctrl    = document.getElementById('cam-layout-ctrl');
        if (section) section.style.display = '';
        if (ctrl)    ctrl.style.display = 'flex';

        // Add class to main for layout adjustment
        const main = document.getElementById('dashboard-main');
        if (main) main.classList.add('with-cameras');

        // Determine default layout based on camera count
        if (dvCameras.length <= 4)       dvCamLayout = 4;
        else if (dvCameras.length <= 9)  dvCamLayout = 9;
        else                              dvCamLayout = 16;

        dvRenderCamGrid();
    } catch (e) {
        console.error('[DV] Camera init failed:', e);
    }
}

function setCamLayout(n) {
    dvCamLayout = n;
    [4, 9, 16].forEach(x => {
        const btn = document.getElementById(`cam-btn-${x}`);
        if (btn) btn.classList.toggle('active', x === n);
    });
    dvRenderCamGrid();
}
window.setCamLayout = setCamLayout;

function dvRenderCamGrid() {
    const grid = document.getElementById('dv-camera-grid');
    if (!grid) return;

    // Stop existing MJPEG connections
    grid.querySelectorAll('img[data-mjpeg]').forEach(img => { img.src = ''; });

    const token  = dvGetToken();
    const cols   = dvCamLayout === 4 ? 2 : dvCamLayout === 9 ? 3 : 4;
    const count  = dvCamLayout;

    grid.className = `camera-grid cam-grid-${count}`;

    const slots = dvCameras.slice(0, count);
    let html = '';

    for (let i = 0; i < count; i++) {
        const cam = slots[i];
        if (cam) {
            const online   = cam.status === 'online';
            const dotCls   = online ? 'online' : '';
            const mjpegUrl = `/api/v1/cameras/${cam.id}/stream/mjpeg?token=${token}`;
            const snapUrl  = `/api/v1/cameras/${cam.id}/snapshot?t=${Date.now()}&token=${token}`;
            html += `
            <div class="camera-cell" id="dv-cell-${i}">
                <div class="camera-status-dot ${dotCls}"></div>
                <img data-mjpeg="1" id="dv-img-${i}"
                     src="${mjpegUrl}"
                     onerror="dvImgFallback(this,'${snapUrl.replace(/&t=\d+/, '')}',${cam.id})"
                     style="width:100%;height:100%;object-fit:cover;">
                <div class="camera-label">${escDV(cam.name)}</div>
            </div>`;
        } else {
            html += `<div class="camera-cell empty"><div class="cam-empty-hint">—</div></div>`;
        }
    }

    grid.innerHTML = html;
}

// Fallback: if MJPEG fails, refresh snapshot every 5s
window.dvImgFallback = function(img, baseSnap, camId) {
    if (img.dataset.fallback) return;
    img.dataset.fallback = '1';
    img.removeAttribute('data-mjpeg');
    img.src = baseSnap + '&t=' + Date.now();
    img.onerror = null;

    const cellId = img.closest('.camera-cell') ? img.closest('.camera-cell').id : null;
    const timer = setInterval(() => {
        if (!document.getElementById(cellId)) { clearInterval(timer); return; }
        img.src = baseSnap + '&t=' + Date.now();
    }, 5000);
};

function dvRefreshCameras() {
    dvRenderCamGrid();
}
window.dvRefreshCameras = dvRefreshCameras;

function escDV(s) {
    const d = document.createElement('div');
    d.textContent = s || '';
    return d.innerHTML;
}

// ========== 拓樸圖更新 ==========
function updateTopology(data) {
    // 使用 topology.js 的全局變數與函式，確保呈現一致
    window.topologyData = data;
    
    // 呼叫 topology.js 的處理函式
    if (typeof processTopologyData === 'function') {
        processTopologyData();
    }
}

// 實作 viewDevice 供 topology.js 點擊時呼叫 (Dashboard View 中僅顯示提示)
function viewDevice(deviceId) {
    console.log('[Dashboard] Device clicked:', deviceId);
}
window.viewDevice = viewDevice;

function refreshTopology() {
    loadDashboardData();
}

// ========== 設備狀態更新 ==========
function updateDeviceStatus(data) {
    const devices = data.devices || [];

    // 計算統計
    const total = devices.length;
    const online = devices.filter(d => d.is_online).length;
    const offline = total - online;
    const warning = devices.filter(d => d.cpu_usage > 80 || d.memory_usage > 80).length;

    // 更新數字
    document.getElementById('total-devices').textContent = total;
    document.getElementById('online-devices').textContent = online;
    document.getElementById('offline-devices').textContent = offline;
    document.getElementById('warning-devices').textContent = warning;

    // 更新設備清單
    renderDeviceList(devices);
}

function renderDeviceList(devices) {
    const container = document.getElementById('device-list-content');
    if (!container) return;

    if (devices.length === 0) {
        container.innerHTML = '<div class="device-item">尚無設備</div>';
        return;
    }

    // 排序：在線優先
    devices.sort((a, b) => (b.is_online ? 1 : 0) - (a.is_online ? 1 : 0));

    container.innerHTML = devices.map(device => {
        const statusClass = device.is_online ? 'online' : 'offline';
        const statusText = device.is_online ? '在線' : '離線';

        return `
            <div class="device-item ${statusClass}">
                <div class="device-info">
                    <div class="device-name">${device.name || 'Unknown'}</div>
                    <div class="device-ip">${device.ip_address}</div>
                </div>
                <div class="device-status">
                    <span class="status-dot ${statusClass}"></span>
                    ${statusText}
                </div>
            </div>
        `;
    }).join('');
}



// ========== 重新整理功能 ==========
function refreshTopology() {
    loadDashboardData();
}


// ========== 導航功能 ==========
function backToModeSelection() {
    if (confirm(t('mode_selection.confirm_back'))) {
        // 停止自動更新
        if (updateInterval) {
            clearInterval(updateInterval);
        }
        // 退出全螢幕
        exitFullscreen();
        // 返回登入頁
        navigateToFrontendRoute('/mode-selection.html');
    }
}

function logout() {
    if (confirm('確定要登出嗎？')) {
        if (updateInterval) clearInterval(updateInterval);
        // Clear both storages
        sessionStorage.removeItem('nms_token');
        sessionStorage.removeItem('nms_session_active');
        localStorage.removeItem('token');
        localStorage.removeItem('username');
        localStorage.removeItem('role');
        exitFullscreen();
        navigateToFrontendRoute('/login');
    }
}

// ========== 工具函數 ==========
function updateLastUpdateTime() {
    const now = new Date();
    const timeString = now.toLocaleTimeString('zh-TW', { hour12: false });
    const timeElement = document.getElementById('last-update');
    if (timeElement) {
        timeElement.textContent = timeString;
    }
}

function getAuthToken() {
    return localStorage.getItem('token') || '';
}

// ========== 清理 ==========
window.addEventListener('beforeunload', () => {
    if (updateInterval) {
        clearInterval(updateInterval);
    }
    if (topologySimulation) {
        topologySimulation.stop();
    }
});
