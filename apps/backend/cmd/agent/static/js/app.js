// Made by YTSworks
// YTS工作室製作
// 全域狀態
const state = {
    currentPage: 'dashboard',
    devices: {
        page: 1,
        limit: 20,
        search: '',
        type: '',
        status: ''
    },
    logs: {
        page: 1,
        limit: 50,
        type: 'system_logs',
        severity: '',
        search: '',
        scope: '',
        device: '',
        actor: '',
        status: '',
        dateFrom: '',
        dateTo: '',
        format: 'csv',
        bundleFormat: 'json'
    },
    idleTimeout: 5 * 60 * 1000, // 5 minutes (300,000 ms)
    lastActivity: Date.now(),
    licenseLocked: false,
    licenseLockReason: ''
};

// 初始化應用程式
document.addEventListener('DOMContentLoaded', async () => {
    // 清除舊版的 force_init_refresh 旗標 (不再使用 — login redirect 本身就是 fresh load)
    localStorage.removeItem('nms_force_init_refresh');

    // Strip cache-bust param (_cb=timestamp) from URL after it has done its job
    const _urlNow = new URL(window.location.href);
    if (_urlNow.searchParams.has('_cb')) {
        _urlNow.searchParams.delete('_cb');
        window.history.replaceState({}, '', _urlNow.pathname + (_urlNow.search || ''));
    }

    // Auto-logout on fresh page load (not from same session)
    // This ensures user always starts with login screen
    const sessionKey = 'nms_session_active';
    const isSessionActive = sessionStorage.getItem(sessionKey);
    const hasToken = sessionStorage.getItem('nms_token');
    const requirePasswordChange = sessionStorage.getItem('nms_require_pwd_change') === 'true';

    const urlParams = new URLSearchParams(window.location.search);
    const isRestored = urlParams.get('restored') === 'true';

    // If restored, we skip the fresh session check entirely
    // ALSO skip if user needs to change password (forced password change modal)
    if (!hasToken && window.location.pathname !== '/login' && window.location.pathname !== '/static/login.html') {
        navigateToFrontendRoute('/login');
        return;
    }

    // Mark session as active
    sessionStorage.setItem(sessionKey, 'true');

    // Initialize I18n
    if (typeof initI18n === 'function') {
        await initI18n();
    }

    if (!initAuth()) return; // Check auth first

    const systemInfo = await fetchSystemInfoSnapshot();
    const licenseLocked = systemInfo ? applySystemInfoSnapshot(systemInfo) : isLicenseLockActive();
    if (licenseLocked) {
        enterLicenseLockMode(getLicenseLockReason());
    }

    initTheme();
    initSidebarState();
    initNavigation();
    initAdminTabs();
    initFilters();
    initIdleTimer();    // Start idle monitoring
    if (!licenseLocked) {
        initVersionCheck(); // Start version/uptime polling
        initNotifications(); // Start in-app notification polling
        await updateModuleVisibility(); // Check which modules are enabled
    }

    // 載入初始頁面
    // 載入初始頁面 (恢復上次瀏覽的分頁)
    const lastPage = licenseLocked ? 'admin' : (sessionStorage.getItem('nms_active_page') || 'dashboard');
    loadPage(lastPage);

    // 自動重新整理 (每 15 秒)
    if (!licenseLocked) {
        setInterval(() => {
        if (state.currentPage === 'dashboard') {
            autoRefreshDashboard(); // 只更新數據，不重新載入頁面
        } else if (state.currentPage === 'devices') {
            // Auto-refresh devices list every 15s (silent mode)
            if (typeof loadDevices === 'function') {
                loadDevices(true);
            }
        }
        }, 15000); // 15 seconds
    }

    // 監聽語系切換事件，重新載入當前頁面
    window.addEventListener('languageChanged', async () => {
        console.log('[app.js] Language changed, reloading current page:', state.currentPage);
        if (!isLicenseLockActive()) {
            await updateModuleVisibility();
            loadPage(state.currentPage);
            return;
        }

        loadPage('admin');
    });
});

const LICENSE_LOCK_STORAGE_KEY = 'nms_license_locked';
const LICENSE_LOCK_REASON_STORAGE_KEY = 'nms_license_lock_reason';

function isLicenseLockActive() {
    return state.licenseLocked || sessionStorage.getItem(LICENSE_LOCK_STORAGE_KEY) === 'true';
}

function getLicenseLockReason() {
    return state.licenseLockReason || sessionStorage.getItem(LICENSE_LOCK_REASON_STORAGE_KEY) || '';
}

function setLicenseLockState(locked, reason = '') {
    state.licenseLocked = Boolean(locked);
    state.licenseLockReason = (reason || '').trim();

    if (state.licenseLocked) {
        sessionStorage.setItem(LICENSE_LOCK_STORAGE_KEY, 'true');
        if (state.licenseLockReason) {
            sessionStorage.setItem(LICENSE_LOCK_REASON_STORAGE_KEY, state.licenseLockReason);
        } else {
            sessionStorage.removeItem(LICENSE_LOCK_REASON_STORAGE_KEY);
        }
        return;
    }

    sessionStorage.removeItem(LICENSE_LOCK_STORAGE_KEY);
    sessionStorage.removeItem(LICENSE_LOCK_REASON_STORAGE_KEY);
}

function clearLicenseLockState() {
    setLicenseLockState(false, '');
}

function getPreferredLockedAdminTab() {
    return (typeof isAdmin === 'function' && isAdmin()) ? 'users' : 'licenses';
}

function isAllowedLockedAdminTab(tabName) {
    if (tabName === 'licenses') {
        return true;
    }
    if (tabName === 'users') {
        return typeof isAdmin === 'function' && isAdmin();
    }
    return false;
}

function activateAdminTab(tabName) {
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.tab === tabName);
    });

    document.querySelectorAll('.admin-tab-content').forEach(content => {
        const currentTab = content.id.replace(/-tab$/, '');
        content.classList.toggle('active', currentTab === tabName);
    });
}

function applyLicenseLockUI() {
    const activeTab = getPreferredLockedAdminTab();

    document.querySelectorAll('.sidebar .nav-item, .bottom-nav-item').forEach(item => {
        item.style.display = item.dataset.page === 'admin' ? '' : 'none';
    });

    document.querySelectorAll('.admin-tabs .tab-btn').forEach(btn => {
        btn.style.display = isAllowedLockedAdminTab(btn.dataset.tab) ? '' : 'none';
    });

    document.querySelectorAll('.admin-tab-content').forEach(content => {
        const tabName = content.id.replace(/-tab$/, '');
        content.style.display = isAllowedLockedAdminTab(tabName) ? '' : 'none';
    });

    activateAdminTab(activeTab);
}

function enterLicenseLockMode(reason = '') {
    setLicenseLockState(true, reason || getLicenseLockReason());
    applyLicenseLockUI();
}

async function fetchSystemInfoSnapshot() {
    try {
        const token = sessionStorage.getItem('nms_token');
        const headers = { 'Pragma': 'no-cache', 'Cache-Control': 'no-cache' };
        if (token) headers['Authorization'] = `Bearer ${token}`;

        const response = await fetch(`/api/v1/system/info?_ts=${Date.now()}`, {
            cache: 'no-store',
            headers
        });
        if (!response.ok) {
            return null;
        }

        const payload = await response.json();
        if (!payload.success || !payload.data) {
            return null;
        }

        return payload.data;
    } catch (error) {
        console.warn('[app.js] Failed to fetch system info:', error);
        return null;
    }
}

function applySystemInfoSnapshot(info) {
    if (!info) {
        return false;
    }

    const verSpan = document.getElementById('system-version');
    if (verSpan && info.version) {
        verSpan.textContent = info.version;
    }
    if (info.start_time) {
        currentSystemStartTime = info.start_time;
    }

    if (info.license_locked) {
        enterLicenseLockMode(info.license_lock_reason || 'license_required');
        return true;
    }

    clearLicenseLockState();
    return false;
}

window.handleLicenseLocked = function(reason) {
    enterLicenseLockMode(reason || getLicenseLockReason());
    loadPage('admin');
};

window.clearLicenseLockState = clearLicenseLockState;

// Version Check State
let currentSystemStartTime = null;

// Initialize Version/Uptime Check
function initVersionCheck() {
    const verSpan = document.getElementById('system-version');
    if (verSpan && typeof getAppVersion === 'function') {
        verSpan.textContent = getAppVersion();
    }

    const checkVersion = async () => {
        try {
            // Version check is public in SetupRouter, but we add token just in case
            const token = sessionStorage.getItem('nms_token');
            const headers = { 'Pragma': 'no-cache', 'Cache-Control': 'no-cache' };
            if (token) headers['Authorization'] = `Bearer ${token}`;

            const response = await fetch('/api/v1/system/info?_ts=' + new Date().getTime(), {
                headers: headers
            });

            if (!response.ok) return;

            const data = await response.json();
            if (data.success && data.data) {
                const newStartTime = data.data.start_time;
                const newVersion = data.data.version;

                // Update system version display globally
                if (verSpan && newVersion) {
                    verSpan.textContent = newVersion;
                }

                if (currentSystemStartTime === null) {
                    currentSystemStartTime = newStartTime;
                } else if (newStartTime !== currentSystemStartTime) {
                    // Server restarted or updated
                    console.log('Server restart detected. Forcing logout/reload.');
                    showToast(t('app.toast.updating'), 'warning');

                    // Force logout after short delay
                    setTimeout(() => {
                        logout(); // Ensures token is cleared and redirects to login
                    }, 2000);
                }
            }
        } catch (error) {
            console.error('Version check failed:', error);
        }
    };

    // Check immediately
    checkVersion();

    // Poll every 30 seconds
    setInterval(checkVersion, 30000);
}

// ----- 閒置登出功能 (Idle Timeout) -----
function initIdleTimer() {
    // 監聽各種使用者活動
    const activities = ['mousedown', 'mousemove', 'keypress', 'scroll', 'touchstart'];

    activities.forEach(name => {
        document.addEventListener(name, () => {
            state.lastActivity = Date.now();
        }, { passive: true });
    });

    // 每分鐘檢查一次是否閒置過久
    setInterval(() => {
        // 如果未登入則不檢查
        if (!getToken()) return;

        const idleTime = Date.now() - state.lastActivity;
        if (idleTime > state.idleTimeout) {
            console.log('User idle for too long. Logging out...');
            showToast(t('app.toast.idle_logout'), 'warning');
            setTimeout(() => {
                logout();
            }, 2000);
        }
    }, 60000); // Check every minute
}

// ----- 視窗切換檢查 (Visibility Check) -----
function initVisibilityCheck() {
    document.addEventListener('visibilitychange', () => {
        if (document.visibilityState === 'visible') {
            // Skip session check if modal is currently open
            if (window.isModalOpen) {
                console.log('[App] Skipping session check - modal is open');
                return;
            }

            // 當使用者回到分頁時，檢查登入狀態
            console.log('Tab visible again. Checking session...');

            // 1. 如果 Token 消失了（例如在別的分頁登出），直接重整
            if (!getToken()) {
                window.location.reload();
                return;
            }

            // 2. 檢查閒置時間是否在背景中超過了
            const idleTime = Date.now() - state.lastActivity;
            if (idleTime > state.idleTimeout) {
                showToast(t('app.toast.idle_relogin'), 'warning');
                setTimeout(() => logout(), 1500);
            }
        }
    });
}

// 初始化主題
function initTheme() {
    const savedTheme = localStorage.getItem('nms-theme');
    document.body.classList.toggle('dark-theme', savedTheme !== 'light');
}

// 切換主題
function toggleTheme() {
    const isDark = document.body.classList.toggle('dark-theme');
    localStorage.setItem('nms-theme', isDark ? 'dark' : 'light');
    showToast(isDark ? t('app.toast.theme_dark') : t('app.toast.theme_light'), 'info');
}

// 初始化導航
function initNavigation() {
    const navItems = document.querySelectorAll('.nav-item');

    navItems.forEach(item => {
        item.addEventListener('click', (e) => {
            e.preventDefault();
            const page = item.dataset.page;

            // 更新導航狀態
            navItems.forEach(i => i.classList.remove('active'));
            item.classList.add('active');

            // 載入頁面
            loadPage(page);

            // 在移動裝置上關閉側邊欄
            closeSidebar();
        });
    });
}

// 切換側邊欄 (用於移動裝置)
function toggleSidebar() {
    const sidebar = document.querySelector('.sidebar');
    const overlay = document.querySelector('.sidebar-overlay');

    if (sidebar.classList.contains('open')) {
        closeSidebar();
    } else {
        sidebar.classList.add('open');
        // 創建或顯示覆蓋層
        if (!overlay) {
            const newOverlay = document.createElement('div');
            newOverlay.className = 'sidebar-overlay active';
            newOverlay.onclick = closeSidebar;
            document.body.appendChild(newOverlay);
        } else {
            overlay.classList.add('active');
        }
    }
}

// 關閉側邊欄
function closeSidebar() {
    const sidebar = document.querySelector('.sidebar');
    const overlay = document.querySelector('.sidebar-overlay');

    sidebar.classList.remove('open');
    if (overlay) {
        overlay.classList.remove('active');
    }
}

// 收縮/展開側邊欄
function toggleSidebarCollapse() {
    const sidebar = document.getElementById('sidebar');
    if (!sidebar) return;

    const isCollapsed = sidebar.classList.toggle('collapsed');
    document.body.classList.toggle('sidebar-collapsed', isCollapsed);
    localStorage.setItem('nms-sidebar-collapsed', isCollapsed ? 'true' : 'false');
}

// 初始化側邊欄收縮狀態
function initSidebarState() {
    const sidebar = document.getElementById('sidebar');
    const savedState = localStorage.getItem('nms-sidebar-collapsed');

    if (savedState === 'true' && sidebar) {
        sidebar.classList.add('collapsed');
        document.body.classList.add('sidebar-collapsed');
    }
}

// 載入頁面
function loadPage(page) {
    if (state.currentPage === 'cameras' && page !== 'cameras' && typeof window.pauseCameraPreview === 'function') {
        window.pauseCameraPreview();
    }

    // Viewer 不得進入 admin 頁面，強制導回 dashboard
    if (isLicenseLockActive() && page !== 'admin') {
        page = 'admin';
    }

    if (page === 'admin' && typeof isAdmin === 'function' && !isAdmin() && !isLicenseLockActive()) {
        page = 'dashboard';
    }
    // Editor 以下不得進入 logs/reports，強制導回 dashboard
    if ((page === 'logs') && typeof isViewer === 'function' && isViewer()) {
        page = 'dashboard';
    }

    state.currentPage = page;
    sessionStorage.setItem('nms_active_page', page); // 儲存當前頁面

    // 隱藏所有頁面
    document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));

    // 顯示目標頁面
    const targetPage = document.getElementById(page);
    if (targetPage) {
        targetPage.classList.add('active');
    }

    // 更新側邊欄導航狀態
    document.querySelectorAll('.sidebar .nav-item').forEach(item => {
        item.classList.toggle('active', item.dataset.page === page);
    });

    // 更新底部導航欄狀態
    document.querySelectorAll('.bottom-nav-item').forEach(item => {
        item.classList.toggle('active', item.dataset.page === page);
    });

    // 載入頁面資料
    switch (page) {
        case 'dashboard':
            loadDashboard();
            break;
        case 'devices':
            loadDevices();
            break;
        case 'topology':
            loadTopology();
            break;
        case 'logs':
            loadLogs();
            break;
        case 'admin':
            if (isLicenseLockActive()) {
                enterLicenseLockMode(getLicenseLockReason());
                activateAdminTab(getPreferredLockedAdminTab());
                break;
            }
            break;
        case 'audit':
            if (typeof loadAuditLogs === 'function') loadAuditLogs(1);
            break;
        case 'cameras':
            if (typeof initNvrCameras === 'function') {
                initNvrCameras();
            }
            break;
        case 'access-control':
            if (typeof acInit === 'function') {
                acInit();
            }
            break;
        case 'pdu':
            if (typeof pduInit === 'function') {
                pduInit();
            }
            break;
        case 'iot':
            if (typeof iotLoad === 'function') {
                iotLoad();
            }
            break;
    }
}



// 初始化管理員頁籤
function initAdminTabs() {
    const tabBtns = document.querySelectorAll('.admin-tabs .tab-btn');

    tabBtns.forEach(btn => {
        btn.addEventListener('click', () => {
            if (btn.classList.contains('disabled-tab')) {
                showToast(btn.getAttribute('title') || t('common.disabled'), 'warning');
                return;
            }
            const tab = btn.dataset.tab;

            if (isLicenseLockActive() && !isAllowedLockedAdminTab(tab)) {
                activateAdminTab(getPreferredLockedAdminTab());
                return;
            }

            // 更新按鈕狀態
            tabBtns.forEach(b => b.classList.remove('active'));
            tabBtns.forEach(b => b.classList.remove('active'));
            btn.classList.add('active');

            // 更新內容
            document.querySelectorAll('.admin-tab-content').forEach(c => c.classList.remove('active'));
            const tabContent = document.getElementById(`${tab}-tab`);
            if (tabContent) {
                tabContent.classList.add('active');
            }
            loadAdminTabData(tab);
        });
    });
}

// 初始化過濾器
function loadAdminTabData(tab) {
    if (typeof isAdmin === 'function' && !isAdmin() && !isLicenseLockActive()) return;

    switch (tab) {
        case 'host-status':
            if (typeof loadHostStatus === 'function') loadHostStatus();
            break;
        case 'users':
            if (typeof loadUsers === 'function') loadUsers();
            break;
        case 'audit':
            if (typeof loadAuditLogs === 'function') loadAuditLogs(1);
            break;
        case 'licenses':
            if (typeof loadLicenses === 'function') loadLicenses();
            break;
        case 'alerts':
            if (typeof loadAlertSettings === 'function') loadAlertSettings();
            if (typeof loadModuleConfigs === 'function') loadModuleConfigs();
            break;
        case 'security':
            if (typeof loadSecuritySettings === 'function') loadSecuritySettings();
            break;
        case 'branding':
            if (typeof loadBrandingSettings === 'function') loadBrandingSettings();
            break;
    }
}

function initFilters() {
    // 設備搜尋
    const deviceSearch = document.getElementById('device-search');
    if (deviceSearch) {
        let searchTimeout;
        deviceSearch.addEventListener('input', (e) => {
            clearTimeout(searchTimeout);
            searchTimeout = setTimeout(() => {
                state.devices.search = e.target.value;
                state.devices.page = 1;
                loadDevices();
            }, 300);
        });
    }

    // 設備類型過濾
    const deviceTypeFilter = document.getElementById('device-type-filter');
    if (deviceTypeFilter) {
        deviceTypeFilter.addEventListener('change', (e) => {
            state.devices.type = e.target.value;
            state.devices.page = 1;
            loadDevices();
        });
    }

    // 設備狀態過濾
    const deviceStatusFilter = document.getElementById('device-status-filter');
    if (deviceStatusFilter) {
        deviceStatusFilter.addEventListener('change', (e) => {
            state.devices.status = e.target.value;
            state.devices.page = 1;
            loadDevices();
        });
    }

    // 日誌類型選擇
    const logTypeSelect = document.getElementById('log-type-select');
    if (logTypeSelect) {
        logTypeSelect.addEventListener('change', (e) => {
            state.logs.type = e.target.value;
            state.logs.page = 1;
            state.logs.scope = '';
            state.logs.device = '';
            state.logs.severity = '';
            state.logs.actor = '';
            state.logs.status = '';
            loadLogs();
        });
    }

    // 日誌嚴重程度過濾
    const logSeverityFilter = document.getElementById('log-severity-filter');
    if (logSeverityFilter) {
        logSeverityFilter.addEventListener('change', (e) => {
            state.logs.severity = e.target.value;
            state.logs.page = 1;
            loadLogs();
        });
    }
}

// 全螢幕切換
function toggleFullscreen() {
    const btn = document.getElementById('fullscreen-btn');
    if (!document.fullscreenElement) {
        document.documentElement.requestFullscreen().then(() => {
            if (btn) btn.textContent = '⊠';
            if (btn) btn.title = '離開全螢幕';
        }).catch(err => {
            console.warn('Fullscreen failed:', err);
        });
    } else {
        document.exitFullscreen().then(() => {
            if (btn) btn.textContent = '⛶';
            if (btn) btn.title = '全螢幕';
        });
    }
}

// 監聽全螢幕狀態改變 (F11 或 Esc 退出時同步按鈕圖示)
document.addEventListener('fullscreenchange', () => {
    const btn = document.getElementById('fullscreen-btn');
    if (!btn) return;
    if (document.fullscreenElement) {
        btn.textContent = '⊠';
        btn.title = '離開全螢幕';
    } else {
        btn.textContent = '⛶';
        btn.title = '全螢幕';
    }
});

// 匯出報表
function exportReport(type, format) {
    const lang = (typeof currentLang !== 'undefined' && currentLang) ? currentLang : 'zh-TW';
    apiDownload(`/reports/${type}?format=${encodeURIComponent(format)}&lang=${encodeURIComponent(lang)}`);
    showToast(t('app.toast.exporting'), 'info');
}

// 匯出日誌
function exportLogs() {
    const type = (state.logs && state.logs.type) ? state.logs.type : 'system_logs';
    const format = (state.logs && state.logs.format) ? state.logs.format : 'csv';
    const params = new URLSearchParams({
        format,
        type,
        lang: (typeof currentLang !== 'undefined' && currentLang) ? currentLang : 'zh-TW',
    });
    if (state.logs && state.logs.severity) {
        params.set('severity', state.logs.severity);
    }
    if (state.logs && state.logs.search) {
        params.set('search', state.logs.search);
    }
    if (state.logs && state.logs.scope) {
        params.set('scope', state.logs.scope);
    }
    if (state.logs && state.logs.device) {
        params.set('device', state.logs.device);
    }
    if (state.logs && state.logs.actor) {
        params.set('actor', state.logs.actor);
    }
    if (state.logs && state.logs.status) {
        params.set('status', state.logs.status);
    }
    if (state.logs && state.logs.dateFrom) {
        params.set('start', state.logs.dateFrom);
    }
    if (state.logs && state.logs.dateTo) {
        params.set('end', state.logs.dateTo);
    }
    apiDownload(`/log-center/export?${params.toString()}`);
    showToast(t('app.toast.logs_exporting'), 'info');
}
async function updateModuleVisibility() {
    try {
        console.log('[App] Updating module visibility...');
        const token = sessionStorage.getItem('nms_token');
        const headers = { 'Pragma': 'no-cache', 'Cache-Control': 'no-cache' };
        if (token) headers['Authorization'] = `Bearer ${token}`;

        const [configResponse, featureResponse] = await Promise.all([
            fetch('/api/v1/system/config?_ts=' + new Date().getTime(), { headers }),
            fetch('/api/v1/license/features?_ts=' + new Date().getTime(), { headers }),
        ]);

        if (!configResponse.ok || !featureResponse.ok) {
            console.warn('[App] module visibility APIs returned non-OK, preserving current nav state');
            return;
        }

        const [configResult, featureResult] = await Promise.all([
            configResponse.json(),
            featureResponse.json(),
        ]);

        if (configResult.success && featureResult.success) {
            const features = featureResult.data || {};
            const deviceManagementEnabled = !!features.device_management;
            const pageKeys = ['devices', 'topology'];

            setModuleLock('camera', !features.camera_viewer);
            setModuleLock('access_control', !features.access_control);
            setModuleLock('pdu', !features.pdu);
            setModuleLock('iot', !features.iot);

            pageKeys.forEach((pageKey) => {
                const nav = document.querySelector(`.nav-item[data-page="${pageKey}"]`);
                const page = document.getElementById(pageKey);
                if (nav) {
                    nav.style.display = deviceManagementEnabled ? '' : 'none';
                }
                if (page) {
                    page.style.display = deviceManagementEnabled || !page.classList.contains('active') ? '' : 'none';
                }
            });

            const blockedPages = new Set(['#devices', '#topology']);
            if (!deviceManagementEnabled && blockedPages.has(window.location.hash)) {
                window.location.hash = '#dashboard';
                if (typeof showPage === 'function') {
                    showPage('dashboard');
                }
            }
        }
    } catch (error) {
        console.error('Failed to update module visibility:', error);
    }
}

function setModuleLock(moduleKey, locked) {
    const modules = {
        camera: {
            navId: 'nav-cameras',
            lockId: 'camera-nav-lock',
            bottomId: 'bottom-nav-cameras',
            bottomLockId: 'bottom-camera-nav-lock',
            title: '需要攝影機授權'
        },
        access_control: {
            navId: 'nav-access-control',
            lockId: 'ac-nav-lock',
            bottomId: 'bottom-nav-access-control',
            bottomLockId: 'bottom-ac-nav-lock',
            title: '需要門禁管理授權'
        },
        pdu: {
            navId: 'nav-pdu',
            lockId: 'pdu-nav-lock',
            bottomId: 'bottom-nav-pdu',
            bottomLockId: 'bottom-pdu-nav-lock',
            title: '需要 PDU/UPS 授權'
        },
        iot: {
            navId: 'nav-iot',
            lockId: 'iot-nav-lock',
            bottomId: 'bottom-nav-iot',
            bottomLockId: 'bottom-iot-nav-lock',
            title: '需要 IoT / Modbus 授權'
        }
    };

    const config = modules[moduleKey];
    if (!config) return;

    const ensureLock = (containerId, lockId, className) => {
        const container = document.getElementById(containerId);
        if (!container) return null;

        let lock = document.getElementById(lockId);
        if (!lock) {
            lock = document.createElement('span');
            lock.id = lockId;
            lock.className = className;
            lock.textContent = '🔒';
            container.appendChild(lock);
        }

        lock.title = config.title;
        lock.setAttribute('aria-label', config.title);
        return lock;
    };

    const navLock = ensureLock(config.navId, config.lockId, 'nav-badge-lock');
    const bottomLock = ensureLock(config.bottomId, config.bottomLockId, 'bottom-nav-lock');

    [navLock, bottomLock].forEach((lock) => {
        if (lock) lock.style.display = locked ? 'inline-flex' : 'none';
    });

    [document.getElementById(config.navId), document.getElementById(config.bottomId)].forEach((item) => {
        if (item) {
            item.classList.toggle('module-locked', locked);
            item.title = locked ? config.title : '';
        }
    });
}

window.setModuleLock = setModuleLock;
