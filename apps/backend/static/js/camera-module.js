// ============================================================
// Camera Module  v1.2.1
// License logic:
//   - Page tab is always visible (no hidden module)
//   - Without camera license: "???" button disabled + notice shown
//   - With camera license: notice hidden, full functionality
//   - Camera license is independent of Device / Alert licenses
//   v1.2.1: Added split-view monitoring (2x2 / 3x3 / 4x4)
// ============================================================

let currentGridMode = 'grid-4';
let currentLayoutValue = 4;
let cameras = [];
let cameraRecordingStatus = {}; // cameraId -> { is_recording, recording_enabled }
let gridSlots = new Array(64).fill(null); // supports up to 64 cameras across pages
let draggedCameraId = null;
let mjpegInterval = null;
let timeInterval = null;
let cameraLicensed = false;  // set by checkCameraLicense()
let cameraMaxCount = 4;
let cameraGpuAvailable = false; // true when server detects CUDA/QSV/VAAPI/D3D11
const _streamRetryDelay = {}; // camId_cellIdx ??current backoff ms for stream reconnect
let _cameraPreviewPaused = false;
let _cameraPreviewLifecycleBound = false;
const LIVE_STREAM_RECONNECT_MS = 70000;
// Max total cameras (pages ? 16) based on hardware capability
// Grid display is always max 16 per page; GPU unlocks more pages (32/64ch license)
const MAX_CAMS_GPU = 64;  // GPU: support up to 64ch license (4 pages of 16)
const MAX_CAMS_CPU = 16;  // CPU only: max 16ch (1 page)

const PAGE_SIZE = 16; // cameras per page (fixed)
let currentPage = 0;  // 0-based page index

const CAM_LAYOUT_KEY = 'cameraGridLayout';


function escapeHtml(str) {
    const d = document.createElement('div');
    d.textContent = str || '';
    return d.innerHTML;
}

function debounce(fn, ms) {
    let t;
    return (...args) => { clearTimeout(t); t = setTimeout(() => fn(...args), ms); };
}

function camText(key, fallback) {
    const text = typeof t === 'function' ? t(key) : null;
    if (typeof text !== 'string' || !text.trim()) return fallback;
    if (text === key) return fallback;
    if (text.includes('�') || text.includes('?')) return fallback;
    return text;
}

function camTextFmt(key, fallback, vars) {
    const text = camText(key, fallback);
    if (!vars || typeof text !== 'string') return text;
    return Object.entries(vars).reduce((acc, [name, value]) => {
        return acc.replace(new RegExp(`\\{${name}\\}`, 'g'), String(value));
    }, text);
}

// ???? License check ??????????????????????????????????????????????????????????????????????????????????????????

async function checkCameraLicense() {
    try {
        const res = await apiGet('/cameras/status');
        if (res && res.success && res.data) {
            cameraLicensed = !!res.data.licensed;
            cameraMaxCount = res.data.max || 4;
            cameraGpuAvailable = !!res.data.gpu_available;
        } else {
            cameraLicensed = false;
        }
    } catch (e) {
        cameraLicensed = false;
    }
    applyLicenseUI();
}

function applyLicenseUI() {
    const notice = document.getElementById('camera-license-notice');
    const addBtn = document.getElementById('cam-add-btn');
    const discoverBtns = document.querySelectorAll('[onclick="discoverCameras()"]');

    if (cameraLicensed) {
        if (notice) notice.style.display = 'none';
        if (addBtn) {
            addBtn.disabled = false;
            addBtn.classList.remove('btn-disabled');
            addBtn.title = '';
        }
        discoverBtns.forEach(b => { b.disabled = false; b.classList.remove('btn-disabled'); });
    } else {
        if (notice) notice.style.display = 'flex';
        if (addBtn) {
            addBtn.disabled = true;
            addBtn.classList.add('btn-disabled');
            addBtn.title = '請先啟用攝影機 License';
        }
        discoverBtns.forEach(b => {
            b.disabled = true;
            b.classList.add('btn-disabled');
            b.title = '請先啟用攝影機 License';
        });
    }

    // Update lock badge in sidebar nav
    const lockBadge = document.getElementById('camera-nav-lock');
    if (lockBadge) lockBadge.style.display = cameraLicensed ? 'none' : 'inline';

    // Disable grid layout buttons that exceed licensed camera count (max grid = 16)
    document.querySelectorAll('.cam-grid-btn').forEach(btn => {
        const gridSize = parseInt(btn.dataset.grid?.split('-')[1] || '0');
        if (gridSize > cameraMaxCount) {
            btn.disabled = true;
            btn.classList.add('btn-disabled');
            btn.title = `超過目前授權上限 (${gridSize} 格)`;
        } else {
            btn.disabled = false;
            btn.classList.remove('btn-disabled');
            btn.title = '';
        }
    });

    // Show GPU status hint (GPU unlocks 32/64ch license support via pagination)
    const gpuHint = document.getElementById('cam-gpu-hint');
    if (gpuHint) {
        gpuHint.textContent = cameraGpuAvailable ? t('cameras.gpu_accel') : t('cameras.cpu_decode');
        gpuHint.title = cameraGpuAvailable
            ? t('cameras.gpu_hint_on')
            : t('cameras.gpu_hint_off');
    }
}

// ???? Init ????????????????????????????????????????????????????????????????????????????????????????????????????????????

async function initNvrCameras() {
    console.log('[Camera] Initializing...');
    await checkCameraLicense(); // must run first so cameraMaxCount is set
    restoreLayout();            // then clamp saved layout to license limit
    if (cameraLicensed) {
        await fetchCameras();
    } else {
        renderGridCells(); // show empty grid with drop hint
    }
    startAutoRefresh();
    startTimeUpdate();
    if (!_cameraFullscreenListenersBound) {
        document.addEventListener('fullscreenchange', handleFullscreenChange);
        document.addEventListener('webkitfullscreenchange', handleFullscreenChange);
        document.addEventListener('keydown', handleCameraFullscreenKeydown);
        _cameraFullscreenListenersBound = true;
    }
    bindCameraPreviewLifecycle();
    window.resumeCameraPreview(true);
    window.addEventListener('resize', debounce(renderGridCells, 300));
}
window.initNvrCameras = initNvrCameras;

// Re-render on language change
window.addEventListener('languageChanged', () => {
    renderSidebar();
    if (window.nvrRenderCamList) window.nvrRenderCamList();
    // Update recording list headers (hardcoded Chinese) by re-fetching
    if (_nvrSelectedCamId) window.nvrLoadRecordings(_nvrPage);
});

// ???? Layout persistence ????????????????????????????????????????????????????????????????????????????????

function restoreLayout() {
    try {
        const saved = localStorage.getItem(CAM_LAYOUT_KEY);
        if (saved) {
            const p = JSON.parse(saved);
            if (p.layout && [4, 9, 16].includes(p.layout)) {
                // Clamp to license limit
                const allowed = p.layout <= cameraMaxCount ? p.layout : (cameraMaxCount >= 9 ? 9 : 4);
                currentLayoutValue = allowed;
                currentGridMode = `grid-${allowed}`;
                updateGridButtons();
            }
            if (Array.isArray(p.slots)) {
                for (let i = 0; i < 64; i++) {
                    gridSlots[i] = p.slots[i] !== undefined ? p.slots[i] : null;
                }
            }
            if (typeof p.page === 'number') currentPage = p.page;
        }
    } catch (e) { /* ignore */ }
    const grid = document.getElementById('camera-grid');
    if (grid) grid.className = 'cam-grid ' + currentGridMode;
}

function saveLayout() {
    localStorage.setItem(CAM_LAYOUT_KEY, JSON.stringify({ layout: currentLayoutValue, slots: gridSlots, page: currentPage }));
}

function updateGridButtons() {
    document.querySelectorAll('.cam-grid-btn').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.grid === currentGridMode);
    });
}

window.setGridMode = function (mode) {
    const requestedSize = parseInt(mode.split('-')[1]);
    // Enforce license limit: cannot select a grid larger than licensed camera count
    if (requestedSize > cameraMaxCount) {
        showToast(`目前授權最多僅支援 ${cameraMaxCount} 格畫面`, 'warning');
        return;
    }
    currentGridMode = mode;
    currentLayoutValue = requestedSize;
    const grid = document.getElementById('camera-grid');
    if (grid) grid.className = 'cam-grid ' + mode;
    updateGridButtons();
    saveLayout();
    renderGridCells();
};

window.toggleCameraSidebar = function () {
    const sb = document.getElementById('cam-sidebar');
    if (sb) sb.classList.toggle('collapsed');
};

// ??sidebar ?皝?????獢???
window.sidebarToggleRec = async function (cameraId, enable) {
    try {
        const res = await apiPut('/cameras/' + cameraId + '/recording', { enabled: enable });
        if (res && res.success) {
            // Optimistic local update
            cameraRecordingStatus[cameraId] = {
                is_recording: enable,
                recording_enabled: enable,
            };
            renderSidebar();
            if (typeof showToast === 'function')
                showToast(enable ? '已開始錄影' : '已停止錄影', enable ? 'success' : 'info');

            // After a short delay, re-poll backend to get true state
            // (nvrStop waits for ffmpeg to exit, so status is final by then)
            setTimeout(async () => {
                try {
                    const sr = await apiGet('/cameras/recording/status');
                    if (sr && sr.success && Array.isArray(sr.cameras)) {
                        cameraRecordingStatus = {};
                        sr.cameras.forEach(r => {
                            cameraRecordingStatus[r.id] = {
                                is_recording: r.is_recording,
                                recording_enabled: r.recording_enabled,
                            };
                        });
                        renderSidebar();
                        if (window.nvrRenderCamList) window.nvrRenderCamList();
                    }
                } catch(e) { /* ignore */ }
            }, 1500);

            if (window.nvrRenderCamList) window.nvrRenderCamList();
        } else {
            if (typeof showToast === 'function') showToast(res?.error || '操作失敗', 'error');
        }
    } catch (e) {
        if (typeof showToast === 'function') showToast('操作失敗', 'error');
    }
};

// ???? Data ??????????????????????????????????????????????????????????????????????????????????????????????????????????

async function fetchCameras() {
    if (!cameraLicensed) return;
    try {
        const [camRes, recRes] = await Promise.all([
            apiGet('/cameras'),
            apiGet('/cameras/recording/status').catch(() => null),
        ]);
        if (camRes && camRes.data && Array.isArray(camRes.data.cameras)) {
            cameras = camRes.data.cameras;
            // Remove stale slots
            const ids = cameras.map(c => c.id);
            for (let i = 0; i < 64; i++) {
                if (gridSlots[i] && !ids.includes(gridSlots[i])) gridSlots[i] = null;
            }
            if (cameras.length > 0 && gridSlots.every(slot => !slot)) {
                const seedCount = Math.min(cameras.length, Math.max(1, currentLayoutValue));
                for (let i = 0; i < seedCount; i++) {
                    gridSlots[i] = cameras[i].id;
                }
            }
            // Clamp page to valid range
            const totalPages = Math.max(1, Math.ceil(cameraMaxCount / PAGE_SIZE));
            if (currentPage >= totalPages) currentPage = totalPages - 1;
            saveLayout();
        }
        if (recRes && recRes.success && Array.isArray(recRes.cameras)) {
            cameraRecordingStatus = {};
            recRes.cameras.forEach(r => {
                cameraRecordingStatus[r.id] = { is_recording: r.is_recording, recording_enabled: r.recording_enabled };
            });
        }
        renderSidebar();
        renderGridCells();
    } catch (e) {
        console.error('[Camera] fetchCameras failed', e);
    }
}

// ???? Render ????????????????????????????????????????????????????????????????????????????????????????????????????????

function renderSidebar() {
    const list = document.getElementById('cam-sidebar-list');
    const count = document.getElementById('sidebar-cam-count');
    if (!list) return;
    if (count) count.innerText = cameras.length;

    if (!cameraLicensed) {
        list.innerHTML = `<div class="cam-no-license-hint">${camText('cameras.license_notice', '攝影機監控模組需加購 License 方可使用。')}</div>`;
        return;
    }

    if (cameras.length === 0) {
        list.innerHTML = `<div class="cam-empty-sidebar">${camText('cameras.no_cameras', '尚未新增攝影機，請先新增攝影機。')}</div>`;
        return;
    }

    const sb = document.getElementById('cam-sidebar');
    const isCollapsed = sb && sb.classList.contains('collapsed');

    list.innerHTML = cameras.map(cam => {
        const inUse = gridSlots.includes(cam.id);
        const statusCls = cam.status === 'online' ? 'online' : 'offline';
        const recState = cameraRecordingStatus[cam.id] || {};
        const isRecording = recState.is_recording || false;
        const snapUrl = `/api/v1/cameras/${cam.id}/snapshot?t=${Date.now()}&token=${getAuthToken()}`;
        const imgSrc = isCollapsed ? '' : snapUrl;
        const dataSrc = isCollapsed ? `data-src="${snapUrl}"` : '';
        const clickHandler = !inUse ? `onclick="window.tapAssignCamera(${cam.id})"` : '';
        return `
            <div class="cam-list-item ${inUse ? 'in-use' : ''}"
                 draggable="${!inUse}"
                 ondragstart="window.dragCamera(event, ${cam.id})"
                 ${clickHandler}>
                <div class="cam-list-thumb">
                    <img src="${imgSrc}" ${dataSrc} loading="lazy"
                         onerror="this.style.display='none'">
                </div>
                <div class="cam-list-info">
                    <div class="cam-list-name">${escapeHtml(cam.name)}</div>
                    <div class="cam-list-ip">${escapeHtml(cam.ip_address)}</div>
                </div>
                <div class="cam-status-dot ${statusCls}" title="${cam.status}"></div>
                ${!inUse ? `<div class="cam-tap-hint">${camText('cameras.tap_to_assign', '點擊指派到畫面')}</div>` : ''}
                <div style="display:flex;flex-direction:column;gap:3px;margin-left:auto;flex-shrink:0;align-items:center;">
                    <button class="cam-sidebar-rec-btn ${isRecording ? 'active' : ''}" 
                            onclick="event.stopPropagation();window.sidebarToggleRec(${cam.id}, ${!isRecording})" 
                            title="${isRecording ? camText('cameras.stop_recording', '停止錄影') : camText('cameras.start_recording', '開始錄影')}">
                        ${isRecording ? '&#9632;' : '&#9679;'}
                    </button>
                    <button class="cam-sidebar-edit-btn" onclick="event.stopPropagation();window.editCamera(${cam.id})" title="${camText('cameras.edit', '編輯攝影機')}">&#9881;</button>
                </div>
            </div>`;
    }).join('');
}

function getEffectiveSlotCount() {
    const w = window.innerWidth;
    if (w <= 768) return Math.min(currentLayoutValue, 4);
    if (w <= 1200 && currentLayoutValue === 16) return 9;
    return currentLayoutValue;
}

// Returns the global slot index range for the current page.
// Each page always shows PAGE_SIZE (16) slots maximum, mapped to global gridSlots.
function getPageSlotRange() {
    const start = currentPage * PAGE_SIZE;
    const end = start + PAGE_SIZE;
    return { start, end };
}

// Stop all active MJPEG streams on the current page before switching.
function stopCurrentPageStreams() {
    const n = getEffectiveSlotCount();
    const { start } = getPageSlotRange();
    for (let i = 0; i < n; i++) {
        const img = document.getElementById(`cam-media-${i}`);
        if (img && img.tagName === 'IMG' && img.src && img.src !== window.location.href) {
            img.dataset.streamStartedAt = '';
            img.src = '';
        }
    }
}

function renderGridCells() {
    const grid = document.getElementById('camera-grid');
    if (!grid) return;
    const n = getEffectiveSlotCount();
    const { start } = getPageSlotRange();
    // Clear retry backoff counters when grid is re-rendered (fresh start)
    Object.keys(_streamRetryDelay).forEach(k => delete _streamRetryDelay[k]);
    let html = '';
    for (let i = 0; i < n; i++) html += buildCellHtml(i, start + i);
    grid.innerHTML = html;
    updateTime();
    renderPagination();
}

function renderPagination() {
    const container = document.getElementById('cam-pagination');
    if (!container) return;
    const totalPages = Math.max(1, Math.ceil(cameraMaxCount / PAGE_SIZE));
    if (totalPages <= 1) {
        container.innerHTML = '';
        return;
    }
    let html = '<div class="cam-page-nav">';
    html += `<button class="cam-page-btn" onclick="window.switchCameraPage(${currentPage - 1})" ${currentPage === 0 ? 'disabled' : ''}>&#8249;</button>`;
    for (let p = 0; p < totalPages; p++) {
        html += `<button class="cam-page-btn ${p === currentPage ? 'active' : ''}" onclick="window.switchCameraPage(${p})">${p + 1}</button>`;
    }
    html += `<button class="cam-page-btn" onclick="window.switchCameraPage(${currentPage + 1})" ${currentPage >= totalPages - 1 ? 'disabled' : ''}>&#8250;</button>`;
    html += `<span class="cam-page-label">${t('cameras.page_label', { page: currentPage + 1, total: totalPages, max: cameraMaxCount })}</span>`;
    html += '</div>';
    container.innerHTML = html;
}

window.switchCameraPage = function (page) {
    const totalPages = Math.max(1, Math.ceil(cameraMaxCount / PAGE_SIZE));
    if (page < 0 || page >= totalPages) return;
    stopCurrentPageStreams(); // stop ffmpeg for current page
    currentPage = page;
    saveLayout();
    renderGridCells();       // render new page (new img.src ??ffmpeg starts)
};

// i = visual cell index (0..n-1), g = global gridSlots index (page * PAGE_SIZE + i)
function buildCellHtml(i, g) {
    const camId = gridSlots[g];
    const cam = camId ? cameras.find(c => c.id === camId) : null;
    const tl = (s, fb) => (typeof t === 'function' ? t(s) : null) || fb;

    let html = `<div class="cam-cell ${cam ? '' : 'empty'}" id="cam-cell-${i}"
        ondragover="window.allowDrop(event)"
        ondrop="window.dropCamera(event,${g})"
        ondragenter="this.classList.add('drag-over')"
        ondragleave="this.classList.remove('drag-over')"
        ondblclick="window.toggleCameraFullscreen(${i})">`;

    if (cam) {
        const online = cam.status === 'online';
        const sc = online ? 'online' : 'offline';
        const st = online ? tl('cameras.status_online', '\u5728\u7dda') : tl('cameras.status_offline', '\u96e2\u7dda');

        html += `<button class="cam-remove-btn" onclick="event.stopPropagation();window.removeFromGrid(${g})" title="\u79fb\u9664">&#10005;</button>`;
        html += `<button class="cam-edit-btn" onclick="event.stopPropagation();window.editCamera(${cam.id})" title="\u7de8\u8f2f">&#9881;</button>`;
        html += `<button class="cam-maximize-btn" onclick="event.stopPropagation();window.toggleCameraFullscreen(${i})" title="\u5168\u87a2\u5e55">&#9974;</button>`;

        html += `<div class="cam-info-overlay">
            <p><b>${escapeHtml(cam.name)}</b></p>
            <p>${escapeHtml(cam.ip_address)}:${cam.port || 554}</p>
            <p>${getCameraPreviewLabel(cam)} &nbsp;
               <span class="cam-status-dot ${sc}"></span> ${st}</p>
            ${cam.location ? `<p>${escapeHtml(cam.location)}</p>` : ''}
        </div>`;

        html += `<div class="fs-topbar" id="fs-topbar-${i}">
            <div class="fs-title"><span class="cam-status-dot ${sc}"></span> ${escapeHtml(cam.name)}</div>
            <div class="fs-time" id="fs-time-${i}"></div>
        </div>`;

        const token = getAuthToken();
        const previewMode = getCameraPreviewMode(cam);
        if (previewMode === 'live') {
            // MJPEG live stream ??one persistent HTTP connection, ~5fps
            html += `<div class="cam-media-container">
                <img id="cam-media-${i}" class="cam-stream-img" data-preview-mode="live" data-cam-id="${cam.id}" data-stream-started-at="${Date.now()}"
                    src="/api/v1/cameras/${cam.id}/stream/mjpeg?token=${token}"
                    onload="window._onStreamLoad(this,${cam.id},true)"
                    onerror="window._onStreamError(this,${cam.id},true)">
            </div>`;
        } else {
            html += `<img id="cam-media-${i}" class="cam-stream-img" data-preview-mode="snapshot" data-cam-id="${cam.id}" data-stream-started-at="${Date.now()}"
                src="/api/v1/cameras/${cam.id}/snapshot?t=${Date.now()}&token=${token}"
                onload="window._onStreamLoad(this,${cam.id},false)"
                onerror="window._onStreamError(this,${cam.id},false)">`;
        }
    } else {
        // No camera in slot
        if (!cameraLicensed) {
            html += `<div class="cam-empty-hint"><i>&#128274;</i><span>${camText('cameras.license_notice', '攝影機監控模組需加購 License 方可使用。')}</span></div>`;
        } else {
            html += `<div class="cam-empty-hint"><i>&#10133;</i><span>${tl('cameras.drop_hint', '拖曳攝影機至此')}</span></div>`;
        }
    }
    html += `</div>`;
    return html;
}

// g = global gridSlots index
window.removeFromGrid = function (g) {
    gridSlots[g] = null;
    saveLayout();
    renderSidebar();
    // Convert global index to visual cell index for this page
    const { start } = getPageSlotRange();
    const i = g - start;
    updateSingleCell(i, g);
};

// i = visual cell index, g = global slot index
function updateSingleCell(i, g) {
    const old = document.getElementById(`cam-cell-${i}`);
    if (!old) return;
    const tmp = document.createElement('div');
    tmp.innerHTML = buildCellHtml(i, g);
    old.replaceWith(tmp.firstElementChild);
    updateTime();
}

// ???? Drag & Drop ??????????????????????????????????????????????????????????????????????????????????????????????

window.dragCamera = function (event, id) {
    if (!cameraLicensed) return;
    draggedCameraId = id;
    if (event.dataTransfer) event.dataTransfer.setData('text/plain', id);
};

window.allowDrop = function (event) { event.preventDefault(); };

// Tap-to-assign: place camera into the first empty slot on current page (touch/RWD friendly)
window.tapAssignCamera = function (camId) {
    if (!cameraLicensed) return;
    if (gridSlots.includes(camId)) return; // already in use
    const { start, end } = getPageSlotRange();
    const n = getEffectiveSlotCount();
    // Find first empty slot on current page
    for (let g = start; g < start + n && g < end; g++) {
        if (!gridSlots[g]) {
            draggedCameraId = camId;
            gridSlots[g] = camId;
            saveLayout();
            renderSidebar();
            updateSingleCell(g - start, g);
            draggedCameraId = null;
            return;
        }
    }
    // No empty slot on current page ??show hint
    const tl = (s, fb) => (typeof t === 'function' ? t(s) : null) || fb;
    if (typeof showToast === 'function') showToast(tl('cameras.grid_full', '目前頁面已滿，請切換版面或移除其他攝影機。'), 'warning');
};

// slotG = global gridSlots index of the drop target
window.dropCamera = function (event, slotG) {
    event.preventDefault();
    if (!cameraLicensed) return;
    if (event.currentTarget) event.currentTarget.classList.remove('drag-over');
    if (draggedCameraId === null) return;

    const { start } = getPageSlotRange();
    const oldG = gridSlots.indexOf(draggedCameraId);
    gridSlots[slotG] = draggedCameraId;
    if (oldG !== -1 && oldG !== slotG) gridSlots[oldG] = null;
    saveLayout();
    renderSidebar();
    // Only update cells that are visible on this page
    if (oldG !== -1 && oldG !== slotG) {
        const oldI = oldG - start;
        if (oldI >= 0 && oldI < PAGE_SIZE) updateSingleCell(oldI, oldG);
    }
    updateSingleCell(slotG - start, slotG);
    draggedCameraId = null;
};

function isCameraPageActive() {
    return !!document.getElementById('cameras')?.classList.contains('active');
}

function getCameraStreamUrl(camId, isMjpeg) {
    const token = getAuthToken ? getAuthToken() : '';
    return isMjpeg
        ? `/api/v1/cameras/${camId}/stream/mjpeg?token=${token}&_r=${Date.now()}`
        : `/api/v1/cameras/${camId}/snapshot?t=${Date.now()}&token=${token}`;
}

function reconnectCameraImage(img, camId, isMjpeg) {
    if (!img) return;
    img.dataset.previewMode = isMjpeg ? 'live' : 'snapshot';
    img.dataset.streamStartedAt = String(Date.now());
    img.src = getCameraStreamUrl(camId, isMjpeg);
}

function restartVisibleCameraStreams(forceAll = false) {
    if (!cameraLicensed || _cameraPreviewPaused || document.hidden || !isCameraPageActive()) return;

    const n = getEffectiveSlotCount();
    const { start } = getPageSlotRange();
    const now = Date.now();

    for (let i = 0; i < n; i++) {
        const camId = gridSlots[start + i];
        if (!camId) continue;

        const img = document.getElementById(`cam-media-${i}`);
        if (!img || img.tagName !== 'IMG') continue;

        const isLive = img.dataset.previewMode === 'live';
        const startedAt = parseInt(img.dataset.streamStartedAt || '0', 10) || 0;
        const needsRenew = forceAll || !startedAt || (isLive && (now - startedAt) >= LIVE_STREAM_RECONNECT_MS);

        if (needsRenew) {
            reconnectCameraImage(img, camId, isLive);
        }
    }
}

function stopCameraTimers() {
    if (mjpegInterval) {
        clearInterval(mjpegInterval);
        mjpegInterval = null;
    }
    if (timeInterval) {
        clearInterval(timeInterval);
        timeInterval = null;
    }
}

function bindCameraPreviewLifecycle() {
    if (_cameraPreviewLifecycleBound) return;

    const tryResume = () => {
        if (isCameraPageActive()) {
            window.resumeCameraPreview(true);
        }
    };

    document.addEventListener('visibilitychange', () => {
        if (document.hidden) {
            window.pauseCameraPreview();
        } else {
            setTimeout(tryResume, 120);
        }
    });
    window.addEventListener('pageshow', tryResume);
    window.addEventListener('focus', tryResume);
    window.addEventListener('pagehide', () => window.pauseCameraPreview());

    _cameraPreviewLifecycleBound = true;
}

window.pauseCameraPreview = function () {
    _cameraPreviewPaused = true;
    stopCameraTimers();
    stopCurrentPageStreams();
};

window.resumeCameraPreview = function (forceReconnect = false) {
    _cameraPreviewPaused = false;
    if (!isCameraPageActive()) return;
    startAutoRefresh();
    startTimeUpdate();
    restartVisibleCameraStreams(forceReconnect);
};

window._onStreamLoad = function(img, camId, isMjpeg) {
    if (!img) return;
    img.style.opacity = '1';
    img.dataset.streamStartedAt = String(Date.now());
    if (img.id) {
        const idx = parseInt(img.id.replace('cam-media-', ''), 10);
        if (Number.isFinite(idx)) {
            delete _streamRetryDelay[`${camId}_${idx}`];
        }
    }
};

// ???? Stream auto-reconnect ??????????????????????????????????????????????????????????????????????????
// Called from img onerror. Retries with exponential backoff (3s??s??2s, max 30s).
window._onStreamError = function(img, camId, isMjpeg) {
    if (_cameraPreviewPaused) return;
    img.style.opacity = '0.3';
    const idx = img.id ? parseInt(img.id.replace('cam-media-', ''), 10) : -1;
    const key = camId + '_' + idx;
    const delay = Math.min((_streamRetryDelay[key] || 3000) * 1, 30000);
    _streamRetryDelay[key] = Math.min(delay * 2, 30000);
    setTimeout(() => {
        if (_cameraPreviewPaused || document.hidden || !isCameraPageActive()) return;
        if (!document.getElementById(img.id)) return;
        reconnectCameraImage(img, camId, isMjpeg);
        img.style.opacity = '1';
        img.onerror = function() { window._onStreamError(this, camId, isMjpeg); };
    }, delay);
};

// ???? Fullscreen ????????????????????????????????????????????????????????????????????????????????????????????????

// ???? Fullscreen overlay ????????????????????????????????????????????????????????????????????????????????
// Uses a separate fixed overlay + independent <img> for MJPEG so that
// the grid cell's stream is never interrupted when entering fullscreen.

let _fsEntering = false;
let _fsCellId = null;
let _cameraFullscreenListenersBound = false;

function getCameraPreviewMode(cam) {
    return 'live';
}

function getCameraPreviewLabel(cam) {
    return camText('cameras.preview_snapshot', '\u5feb\u7167\u9810\u89bd');
}

function ensureCameraMonitorFields(form, cam) {
    if (!form || form.querySelector('[data-monitor-fields="1"]')) return;

    const ptzLabel = form.querySelector('input[name="supports_ptz"]')?.closest('label');
    if (!ptzLabel) return;

    const wrap = document.createElement('div');
    wrap.dataset.monitorFields = '1';
    wrap.style.display = 'grid';
    wrap.style.gridTemplateColumns = '1fr 140px';
    wrap.style.gap = '12px';
    wrap.style.marginBottom = '8px';
    wrap.innerHTML = `
        <label style="display:flex;align-items:center;gap:8px;cursor:pointer;">
            <input type="checkbox" name="monitor_display" value="1" ${cam?.monitor_display ? 'checked' : ''}>
            <span style="font-size:0.9rem;">${camText('cameras.monitor_display', '顯示於監控模式')}</span>
        </label>
        <div class="form-group" style="margin-bottom:0;">
            <label style="font-size:0.85rem;">${camText('cameras.monitor_order', '監控排序')}</label>
            <input type="number" name="monitor_order" value="${cam?.monitor_order || 0}" min="0" class="input-field">
        </div>
    `;
    ptzLabel.parentNode.insertBefore(wrap, ptzLabel);
}

function enhanceCameraSettingsForm(mode, cam = null) {
    const form = document.getElementById(mode === 'edit' ? 'camera-edit-form' : 'camera-add-form');
    if (!form) return;

    const streamSelect = form.querySelector('select[name="stream_type"]');
    if (streamSelect) {
        const label = streamSelect.closest('.form-group')?.querySelector('label');
        if (label) label.textContent = camText('cameras.preview_snapshot', '\u5feb\u7167\u9810\u89bd');
        const snapshotOpt = streamSelect.querySelector('option[value="mjpeg"]');
        const liveOpt = streamSelect.querySelector('option[value="webrtc"]');
        if (snapshotOpt) snapshotOpt.textContent = camText('cameras.preview_snapshot', '\u5feb\u7167\u9810\u89bd');
        if (liveOpt) liveOpt.remove();
        streamSelect.value = 'mjpeg';
        streamSelect.disabled = true;
    }

    ensureCameraMonitorFields(form, cam);
}

function ensureCameraFullscreenBackdrop() {
    let backdrop = document.getElementById('camera-fullscreen-backdrop');
    if (backdrop) return backdrop;

    backdrop = document.createElement('div');
    backdrop.id = 'camera-fullscreen-backdrop';
    backdrop.className = 'camera-fullscreen-backdrop';
    backdrop.hidden = true;
    backdrop.addEventListener('click', () => {
        exitCameraFullscreen();
    });
    document.body.appendChild(backdrop);
    return backdrop;
}

function exitCameraFullscreen() {
    if (_fsCellId) {
        const activeCell = document.getElementById(_fsCellId);
        if (activeCell) {
            const img = activeCell.querySelector('.cam-stream-img');
            if (img && img.dataset.fullscreenTempLive === '1') {
                const camId = img.dataset.camId;
                img.dataset.fullscreenTempLive = '';
                img.dataset.previewMode = 'snapshot';
                img.src = `/api/v1/cameras/${camId}/snapshot?t=${Date.now()}&token=${getAuthToken()}`;
            }
            activeCell.classList.remove('cam-cell-inline-fullscreen');
        }
    }
    const backdrop = document.getElementById('camera-fullscreen-backdrop');
    if (backdrop) backdrop.hidden = true;
    document.body.classList.remove('cam-fullscreen-active');
    _fsEntering = false;
    _fsCellId = null;
}

function handleCameraFullscreenKeydown(event) {
    if (event.key === 'Escape' && _fsCellId) {
        exitCameraFullscreen();
    }
}

window.toggleCameraFullscreen = function (idx) {
    const cell = document.getElementById(`cam-cell-${idx}`);
    if (!cell || cell.classList.contains('empty')) return;

    if (_fsCellId === cell.id) {
        exitCameraFullscreen();
        return;
    }

    exitCameraFullscreen();
    _fsEntering = true;
    _fsCellId = cell.id;
    ensureCameraFullscreenBackdrop().hidden = false;
    document.body.classList.add('cam-fullscreen-active');
    cell.classList.add('cam-cell-inline-fullscreen');
    setTimeout(() => { _fsEntering = false; }, 50);
};

function handleFullscreenChange() {
    if (!_fsEntering && !document.fullscreenElement && !document.webkitFullscreenElement && _fsCellId) {
        exitCameraFullscreen();
    }
}

function startTimeUpdate() {
    if (timeInterval) clearInterval(timeInterval);
    timeInterval = setInterval(updateTime, 1000);
    updateTime();
}

function updateTime() {
    const n = new Date();
    const s = n.getFullYear() + '-' +
        String(n.getMonth() + 1).padStart(2, '0') + '-' +
        String(n.getDate()).padStart(2, '0') + ' ' +
        String(n.getHours()).padStart(2, '0') + ':' +
        String(n.getMinutes()).padStart(2, '0') + ':' +
        String(n.getSeconds()).padStart(2, '0');
    for (let i = 0; i < 16; i++) {
        const el = document.getElementById(`fs-time-${i}`);
        if (el) el.innerText = s;
    }
}

// ???? Auto-refresh (MJPEG snapshot) ????????????????????????????????????????????????????????

function startAutoRefresh() {
    if (mjpegInterval) clearInterval(mjpegInterval);
    mjpegInterval = setInterval(() => {
        if (!cameraLicensed || _cameraPreviewPaused || document.hidden || !isCameraPageActive()) return;
        const n = getEffectiveSlotCount();
        const { start } = getPageSlotRange();
        const now = Date.now();
        for (let i = 0; i < n; i++) {
            const camId = gridSlots[start + i]; // only current page
            if (!camId) continue;
            const cam = cameras.find(c => c.id === camId);
            if (!cam) continue;
            const img = document.getElementById(`cam-media-${i}`);
            if (!img || img.tagName !== 'IMG') continue;

            if (img.dataset.previewMode === 'live') {
                const startedAt = parseInt(img.dataset.streamStartedAt || '0', 10) || 0;
                if (!startedAt || (now - startedAt) >= LIVE_STREAM_RECONNECT_MS) {
                    reconnectCameraImage(img, camId, true);
                }
                continue;
            }

            if (img.dataset.previewMode !== 'live') {
                const url = `/api/v1/cameras/${camId}/snapshot?t=${Date.now()}&token=${getAuthToken()}`;
                const tmp = new Image();
                tmp.onload = () => {
                    img.dataset.streamStartedAt = String(Date.now());
                    img.src = url;
                };
                tmp.src = url;
            }
        }
    }, 2000);
}

// ???? Discovery (scan device list for RTSP port) ????????????????????????????????

window.discoverCameras = async function () {
    if (!cameraLicensed) {
        if (typeof showToast === 'function') showToast('請先啟用攝影機 License', 'warning');
        return;
    }
    if (typeof openModal !== 'function') return;

    openModal('搜尋攝影機', `
        <div style="text-align:center;padding:30px;">
            <div class="loading-spinner" style="margin:0 auto 16px;"></div>
            <p>正在掃描區域網路中的攝影機與 RTSP 服務，請稍候...</p>
        </div>`);

    try {
        const res = await apiPost('/cameras/discover', {});
        if (!res || !res.success) throw new Error(res?.error || '?嚗貉?剜??');
        const list = (res.data && res.data.data) || [];
        if (list.length === 0) {
            document.getElementById('modal-body').innerHTML = `
                <div style="text-align:center;padding:30px;">
                    <p>未找到可直接加入的設備 (Port 554 closed)</p>
                    <button class="btn btn-secondary" onclick="hideModal()" style="margin-top:16px;">關閉</button>
                </div>`;
            return;
        }
        let rows = list.map(d => `
            <tr>
                <td style="padding:10px;">${escapeHtml(d.name)}</td>
                <td style="padding:10px;"><code>${escapeHtml(d.ip_address)}</code></td>
                <td style="padding:10px;text-align:right;">
                    <button class="btn btn-sm" style="background:var(--success-color);color:#fff;border:none;"
                        onclick="window.quickAddDiscoveredCamera(${d.device_id},'${escapeHtml(d.ip_address)}','${escapeHtml(d.name)}',this)">
                        立即新增
                    </button>
                </td>
            </tr>`).join('');
        document.getElementById('modal-body').innerHTML = `
            <p style="margin-bottom:12px;color:var(--text-secondary);">找到 ${list.length} 台設備開啟 RTSP Port 554，可直接加入攝影機清單。</p>
            <div style="max-height:360px;overflow-y:auto;border:1px solid var(--border-color);border-radius:6px;">
                <table style="width:100%;border-collapse:collapse;">
                    <thead style="background:var(--bg-tertiary);"><tr>
                        <th style="padding:10px;text-align:left;">設備名稱</th>
                        <th style="padding:10px;text-align:left;">IP</th>
                        <th style="padding:10px;text-align:right;">操作</th>
                    </tr></thead>
                    <tbody>${rows}</tbody>
                </table>
            </div>
            <div style="text-align:right;margin-top:16px;">
                <button class="btn btn-secondary" onclick="hideModal()">?謚?</button>
            </div>`;
    } catch (e) {
        document.getElementById('modal-body').innerHTML = `
            <div style="text-align:center;padding:30px;color:var(--danger-color);">
                <p>?蹎? ${escapeHtml(e.message)}</p>
                <button class="btn btn-secondary" onclick="hideModal()" style="margin-top:16px;">?謚?</button>
            </div>`;
    }
};

window.quickAddDiscoveredCamera = async function (deviceId, ip, name, btn) {
    if (!cameraLicensed) return;
    if (btn) btn.disabled = true;
    const payload = {
        name, ip_address: ip, port: 554,
        rtsp_url: `rtsp://admin:password@${ip}:554/stream1`,
        stream_type: 'mjpeg'
    };
    try {
        const res = await apiPost('/cameras', payload);
        if (res && res.success) {
            if (btn) { btn.textContent = '?啣???'; btn.style.background = 'var(--text-muted)'; }
            if (typeof showToast === 'function') showToast('?蔣璈歇?啣?', 'success');
            await fetchCameras();
        } else {
            if (btn) btn.disabled = false;
            if (typeof showToast === 'function') showToast(res?.error || '?啣?憭望?', 'error');
        }
    } catch (e) {
        if (btn) btn.disabled = false;
        if (typeof showToast === 'function') showToast('新增失敗', 'error');
    }
};

// ???? Add / Edit modals ??????????????????????????????????????????????????????????????????????????????????

window.showAddCameraModal = async function () {
    if (!cameraLicensed) {
        if (typeof showToast === 'function') showToast('請先啟用攝影機 License', 'warning');
        return;
    }
    // Check limit
    const count = cameras.length;
    if (count >= cameraMaxCount) {
        if (typeof showToast === 'function')
            showToast(`攝影機數量已達授權上限 (${cameraMaxCount} 台)`, 'warning');
        return;
    }

    let deviceOptions = `<option value="">-- 選擇設備 --</option>`;
    try {
        const r = await apiGet('/devices');
        if (r && r.success && Array.isArray(r.data)) {
            r.data.forEach(d => {
                deviceOptions += `<option value="${d.id}" data-ip="${escapeHtml(d.ip_address)}" data-name="${escapeHtml(d.name)}">${escapeHtml(d.name)} (${escapeHtml(d.ip_address)})</option>`;
            });
        }
    } catch (e) { /* ignore */ }

    openModal('\u65b0\u589e\u651d\u5f71\u6a5f', `
        <form id="camera-add-form" onsubmit="window.handleCameraSubmit(event)">
            <div class="form-group" style="margin-bottom:12px;">
                <label>\u9078\u64c7\u5df2\u5b58\u5728\u8a2d\u5099 (\u9078\u7528)</label>
                <select class="select-input" style="width:100%;" onchange="window.handleCameraDeviceSelect(this)">
                    ${deviceOptions}
                </select>
                <small class="text-muted">${t('cameras.connection_mode_hint')||'\u652f\u63f4\u76f4\u63a5 RTSP \u6216 ONVIF \u81ea\u52d5\u63a2\u6e2c'}</small>
            </div>
            <hr style="margin-bottom:12px;">
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:12px;">
                <div class="form-group" style="margin-bottom:0;">
                    <label>${t('cameras.name')} *</label>
                    <input type="text" id="cam-add-name" name="name" required placeholder="${t('cameras.name')}" class="input-field">
                </div>
                <div class="form-group" style="margin-bottom:0;">
                    <label>${t('cameras.location')}</label>
                    <input type="text" name="location" placeholder="${t('cameras.location')}" class="input-field">
                </div>
            </div>
            <div style="display:grid;grid-template-columns:1fr 80px;gap:12px;margin-bottom:12px;">
                <div class="form-group" style="margin-bottom:0;">
                    <label>${t('cameras.ip_address')} *</label>
                    <input type="text" id="cam-add-ip" name="ip_address" required placeholder="192.168.1.100" class="input-field" oninput="window.updateCamRtspPreview()">
                </div>
                <div class="form-group" style="margin-bottom:0;">
                    <label>Port</label>
                    <input type="number" id="cam-add-port" name="port" value="554" class="input-field" oninput="window.updateCamRtspPreview()">
                </div>
            </div>
            <div class="form-group" style="margin-bottom:12px;">
                <label>${t('cameras.rtsp_url')} *</label>
                <input type="text" id="cam-add-rtsp" name="rtsp_url" required
                    placeholder="rtsp://admin:password@192.168.1.100:554/stream1" class="input-field">
            </div>
            <div style="display:grid;grid-template-columns:1fr 1fr 1fr;gap:12px;margin-bottom:12px;">
                <div class="form-group" style="margin-bottom:0;">
                    <label>${t('cameras.username')}</label>
                    <input type="text" id="cam-add-user" name="username" placeholder="admin" class="input-field" oninput="window.updateCamRtspPreview()">
                </div>
                <div class="form-group" style="margin-bottom:0;">
                    <label>${t('cameras.password')}</label>
                    <input type="password" id="cam-add-pwd" name="password" placeholder="${t('cameras.password_hint')}" class="input-field" oninput="window.updateCamRtspPreview()">
                </div>
                <div class="form-group" style="margin-bottom:0;">
                    <label>${camText('cameras.preview_snapshot', '\u5feb\u7167\u9810\u89bd')}</label>
                    <input type="hidden" name="stream_type" value="mjpeg">
                    <div class="input-field" style="display:flex;align-items:center;min-height:42px;opacity:0.85;">${camText('cameras.preview_snapshot', '\u5feb\u7167\u9810\u89bd')}</div>
                </div>
            </div>
            <details style="margin-bottom:12px;border:1px solid var(--border-color);border-radius:6px;padding:8px 12px;">
                <summary style="cursor:pointer;font-size:0.85rem;color:var(--text-secondary);">&#9881; ${camText('cameras.advanced_settings', '\u9032\u968e\u8a2d\u5b9a')} (${camText('cameras.recording_settings', '\u9304\u5f71\u8a2d\u5b9a')} / ONVIF / ${camText('cameras.manufacturer', '\u5ee0\u724c')} / PTZ)</summary>
                <div style="margin-top:12px;">
                    <div style="padding:10px 12px;margin-bottom:12px;background:var(--bg-tertiary);border:1px solid var(--border-color);border-radius:8px;">
                        <div style="font-size:0.85rem;font-weight:600;margin-bottom:4px;">${camText('cameras.recording_settings', '\u9304\u5f71\u8a2d\u5b9a')}</div>
                        <div style="font-size:0.78rem;color:var(--text-muted);">${camText('cameras.recording_settings_hint', '\u9304\u5f71\u4f86\u6e90\u8207\u8f49\u78bc\u7b56\u7565\u7d71\u4e00\u7531\u9032\u968e\u8a2d\u5b9a\u7ba1\u7406\uff0c\u8207\u756b\u9762\u9810\u89bd\u65b9\u5f0f\u5206\u958b\u3002')}</div>
                    </div>
                    <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:10px;">
                        <div><label style="font-size:0.85rem;">${t('cameras.onvif_url')}</label>
                            <div style="display:flex;gap:6px;align-items:center;">
                                <input type="text" id="cam-add-onvif" name="onvif_url" placeholder="http://192.168.1.x:80/onvif/device_service" class="input-field" style="flex:1;">
                                <button type="button" class="btn btn-xs btn-secondary" onclick="window.onvifProbe('add')" title="${camText('cameras.onvif_probe', '\u900f\u904e ONVIF \u81ea\u52d5\u53d6\u5f97 RTSP \u4f4d\u5740')}" style="white-space:nowrap;flex-shrink:0;">${camText('cameras.onvif_probe_button', '\u81ea\u52d5\u53d6 RTSP')}</button>
                            </div>
                        </div>
                        <div><label style="font-size:0.85rem;">${t('cameras.manufacturer')}</label>
                            <input type="text" name="manufacturer" placeholder="Hikvision / Dahua / QCTek" class="input-field"></div>
                    </div>
                    <div class="form-group" style="margin-bottom:8px;"><label style="font-size:0.85rem;">${t('cameras.model')}</label>
                        <input type="text" name="model" placeholder="DS-2CD" class="input-field"></div>
                    <div class="form-group" style="margin-bottom:8px;">
                        <label style="font-size:0.85rem;">${camText('cameras.recording_bitrate', '\u9304\u5f71 Bitrate \u9650\u5236')} (kbps) <span style="color:var(--text-muted);font-size:0.75rem;">(0=Stream Copy)</span></label>
                        <input type="number" name="recording_bitrate_kbps" min="0" max="102400" placeholder="0" value="0" class="input-field">
                    </div>
                    <div class="form-group" style="margin-bottom:8px;">
                        <label style="font-size:0.85rem;">${camText('cameras.recording_source_label', '\u9304\u5f71\u4f86\u6e90')}</label>
                        <div style="display:flex;gap:16px;align-items:center;margin-top:4px;">
                            <label style="cursor:pointer;display:flex;align-items:center;gap:6px;">
                                <input type="radio" name="recording_source" value="rtsp" checked>
                                <span>${camText('cameras.recording_source_rtsp', 'RTSP \u76f4\u63a5\u9304\u5f71')}</span>
                            </label>
                            <label style="cursor:pointer;display:flex;align-items:center;gap:6px;">
                                <input type="radio" name="recording_source" value="onvif">
                                <span>${camText('cameras.recording_source_onvif', 'ONVIF \u53d6\u6d41\u9304\u5f71')}</span>
                            </label>
                        </div>
                        <small style="color:var(--text-muted);">${camText('cameras.recording_source_hint', 'ONVIF \u6a21\u5f0f\u6bcf\u6b21\u9304\u5f71\u524d\u52d5\u614b\u53d6\u5f97\u6700\u65b0 RTSP \u4f4d\u5740\u3002\u82e5\u8a2d\u5b9a Bitrate \u5247\u6703\u9032\u884c\u8f49\u78bc\u9304\u5f71\u3002')}</small>
                    </div>
                    <label style="display:flex;align-items:center;gap:8px;cursor:pointer;">
                        <input type="checkbox" name="supports_ptz" id="cam-add-ptz">
                        <span style="font-size:0.9rem;">${t('cameras.supports_ptz')}</span>
                    </label>
                </div>
            </details>
            <div style="display:flex;justify-content:flex-end;gap:10px;padding-top:10px;border-top:1px solid var(--border-color);">
                <button type="button" class="btn btn-secondary" onclick="hideModal()">${t('common.cancel')||'\u53d6\u6d88'}</button>
                <button type="submit" class="btn btn-primary">${t('cameras.add')}</button>
            </div>
        </form>`, { wide: true });
    enhanceCameraSettingsForm('add');
};

window.handleCameraDeviceSelect = function (sel) {
    const opt = sel.options[sel.selectedIndex];
    if (!opt.value) return;
    const ip = opt.getAttribute('data-ip');
    const name = opt.getAttribute('data-name');
    const nameEl = document.getElementById('cam-add-name');
    const ipEl = document.getElementById('cam-add-ip');
    if (nameEl) nameEl.value = name;
    if (ipEl) ipEl.value = ip;
    window.updateCamRtspPreview();
};

window.updateCamRtspPreview = function () {
    const ip = (document.getElementById('cam-add-ip') || {}).value || '';
    const port = (document.getElementById('cam-add-port') || {}).value || '554';
    const user = (document.getElementById('cam-add-user') || {}).value || '';
    const pwd = (document.getElementById('cam-add-pwd') || {}).value || '';
    const rtsp = document.getElementById('cam-add-rtsp');
    if (!rtsp || document.activeElement === rtsp || !ip) return;
    
    let path = '/stream1';
    try {
        if (rtsp.value.includes('://')) {
            const url = new URL(rtsp.value.replace(/^rtsp/i, 'http'));
            if (url.pathname && url.pathname !== '/') path = url.pathname + url.search + url.hash;
        }
    } catch(e) {}
    
    let auth = '';
    if (user) {
        const encUser = encodeURIComponent(user);
        const encPwd = pwd ? encodeURIComponent(pwd) : '';
        auth = encPwd ? `${encUser}:${encPwd}@` : `${encUser}@`;
    }
    rtsp.value = `rtsp://${auth}${ip}:${port}${path}`;
};

window.updateEditRtspPreview = function () {
    const ip = (document.getElementById('cam-edit-ip') || {}).value || '';
    const port = (document.getElementById('cam-edit-port') || {}).value || '554';
    const user = (document.getElementById('cam-edit-user') || {}).value || '';
    const pwd = (document.getElementById('cam-edit-pwd') || {}).value || '';
    const rtsp = document.getElementById('cam-edit-rtsp');
    if (!rtsp || document.activeElement === rtsp || !ip) return;
    
    let path = '/stream1';
    let currentPwd = pwd;
    
    try {
        if (rtsp.value.includes('://')) {
            const url = new URL(rtsp.value.replace(/^rtsp/i, 'http'));
            if (!pwd && url.password) currentPwd = url.password;
            if (url.pathname && url.pathname !== '/') path = url.pathname + url.search + url.hash;
        }
    } catch(e) {}
    
    let auth = '';
    if (user) {
        const encUser = encodeURIComponent(user);
        const encPwd = currentPwd ? encodeURIComponent(currentPwd) : '';
        auth = encPwd ? `${encUser}:${encPwd}@` : `${encUser}@`;
    }
    rtsp.value = `rtsp://${auth}${ip}:${port}${path}`;
};

window.handleCameraSubmit = async function (event) {
    event.preventDefault();
    if (!cameraLicensed) return;
    const form = event.target;
    const fd = new FormData(form);
    const data = {
        name: fd.get('name'), location: fd.get('location') || '',
        ip_address: fd.get('ip_address'), port: parseInt(fd.get('port')) || 554,
        rtsp_url: fd.get('rtsp_url'), onvif_url: fd.get('onvif_url') || '',
        username: fd.get('username') || '', password: fd.get('password') || '',
        manufacturer: fd.get('manufacturer') || '', model: fd.get('model') || '',
        supports_ptz: form.querySelector('#cam-add-ptz')?.checked ?? false,
        stream_type: 'mjpeg',
        monitor_display: fd.get('monitor_display') ? 1 : 0,
        monitor_order: parseInt(fd.get('monitor_order')) || 0,
        recording_bitrate_kbps: parseInt(fd.get('recording_bitrate_kbps')) || 0,
        recording_source: fd.get('recording_source') || 'rtsp',
    };
    const btn = form.querySelector('button[type="submit"]');
    if (btn) btn.disabled = true;
    try {
        const res = await apiPost('/cameras', data);
        if (res && res.success) {
            if (typeof showToast === 'function') showToast('攝影機已新增', 'success');
            hideModal();
            await fetchCameras();
        } else {
            if (typeof showToast === 'function') showToast(res?.error || '新增失敗', 'error');
            if (btn) btn.disabled = false;
        }
    } catch (e) {
        if (typeof showToast === 'function') showToast(e.message || '新增失敗', 'error');
        if (btn) btn.disabled = false;
    }
};

window.editCamera = async function (id) {
    if (!cameraLicensed) return;
    const cam = cameras.find(c => c.id === id);
    if (!cam) return;

    openModal(t('cameras.edit'), `
        <form id="camera-edit-form" onsubmit="window.submitEditCamera(event,${id})">
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:12px;">
                <div class="form-group" style="margin-bottom:0;"><label>${t('cameras.name')} *</label><input type="text" name="name" required value="${escapeHtml(cam.name)}" class="input-field"></div>
                <div class="form-group" style="margin-bottom:0;"><label>${t('cameras.location')}</label><input type="text" name="location" value="${escapeHtml(cam.location||'')}" class="input-field"></div>
            </div>
            <div style="display:grid;grid-template-columns:1fr 80px;gap:12px;margin-bottom:12px;">
                <div class="form-group" style="margin-bottom:0;"><label>${t('cameras.ip_address')} *</label><input type="text" id="cam-edit-ip" name="ip_address" required value="${escapeHtml(cam.ip_address)}" class="input-field" oninput="window.updateEditRtspPreview()"></div>
                <div class="form-group" style="margin-bottom:0;"><label>Port</label><input type="number" id="cam-edit-port" name="port" value="${cam.port||554}" class="input-field" oninput="window.updateEditRtspPreview()"></div>
            </div>
            <div class="form-group" style="margin-bottom:12px;">
                <label>${t('cameras.rtsp_url')} *</label>
                <input type="text" id="cam-edit-rtsp" name="rtsp_url" required value="${escapeHtml(cam.rtsp_url||'')}" class="input-field">
            </div>
            <div style="display:grid;grid-template-columns:1fr 1fr 1fr;gap:12px;margin-bottom:12px;">
                <div class="form-group" style="margin-bottom:0;"><label>${t('cameras.username')}</label><input type="text" id="cam-edit-user" name="username" value="${escapeHtml(cam.username||'')}" class="input-field" oninput="window.updateEditRtspPreview()"></div>
                <div class="form-group" style="margin-bottom:0;"><label>${t('cameras.password')}</label><input type="password" id="cam-edit-pwd" name="password" placeholder="${t('cameras.password_hint')}" class="input-field" oninput="window.updateEditRtspPreview()"></div>
                                <div class="form-group" style="margin-bottom:0;"><label>${camText('cameras.preview_snapshot', '\u5feb\u7167\u9810\u89bd')}</label>
                    <input type="hidden" name="stream_type" value="mjpeg">
                    <div class="input-field" style="display:flex;align-items:center;min-height:42px;opacity:0.85;">${camText('cameras.preview_snapshot', '\u5feb\u7167\u9810\u89bd')}</div>
                </div>
            </div>
            <details style="margin-bottom:12px;border:1px solid var(--border-color);border-radius:6px;padding:8px 12px;">
                <summary style="cursor:pointer;font-size:0.85rem;color:var(--text-secondary);">&#9881; ${camText('cameras.advanced_settings', '\u9032\u968e\u8a2d\u5b9a')} (${camText('cameras.recording_settings', '\u9304\u5f71\u8a2d\u5b9a')} / ONVIF / ${camText('cameras.manufacturer', '\u5ee0\u724c')} / PTZ)</summary>
                <div style="margin-top:12px;">
                    <div style="padding:10px 12px;margin-bottom:12px;background:var(--bg-tertiary);border:1px solid var(--border-color);border-radius:8px;">
                        <div style="font-size:0.85rem;font-weight:600;margin-bottom:4px;">${camText('cameras.recording_settings', '\u9304\u5f71\u8a2d\u5b9a')}</div>
                        <div style="font-size:0.78rem;color:var(--text-muted);">${camText('cameras.recording_settings_hint', '\u9304\u5f71\u4f86\u6e90\u8207\u8f49\u78bc\u7b56\u7565\u7d71\u4e00\u7531\u9032\u968e\u8a2d\u5b9a\u7ba1\u7406\uff0c\u8207\u756b\u9762\u9810\u89bd\u65b9\u5f0f\u5206\u958b\u3002')}</div>
                    </div>
                    <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:10px;">
                        <div class="form-group" style="margin-bottom:0;"><label style="font-size:0.85rem;">${t('cameras.onvif_url')}</label>
                            <div style="display:flex;gap:6px;align-items:center;">
                                <input type="text" id="cam-edit-onvif" name="onvif_url" value="${escapeHtml(cam.onvif_url||'')}" class="input-field" style="flex:1;">
                                <button type="button" class="btn btn-xs btn-secondary" onclick="window.onvifProbe('edit')" title="${camText('cameras.onvif_probe', '\u900f\u904e ONVIF \u81ea\u52d5\u53d6\u5f97 RTSP \u4f4d\u5740')}" style="white-space:nowrap;flex-shrink:0;">${camText('cameras.onvif_probe_button', '\u81ea\u52d5\u53d6 RTSP')}</button>
                            </div>
                        </div>
                        <div class="form-group" style="margin-bottom:0;"><label style="font-size:0.85rem;">${t('cameras.manufacturer')}</label><input type="text" name="manufacturer" value="${escapeHtml(cam.manufacturer||'')}" class="input-field"></div>
                    </div>
                    <div class="form-group" style="margin-bottom:8px;">
                        <label style="font-size:0.85rem;">${camText('cameras.recording_bitrate', '\u9304\u5f71 Bitrate \u9650\u5236')} (kbps) <span style="color:var(--text-muted);font-size:0.75rem;">(0=Stream Copy)</span></label>
                        <input type="number" name="recording_bitrate_kbps" min="0" max="102400" value="${cam.recording_bitrate_kbps||0}" class="input-field">
                    </div>
                    <div class="form-group" style="margin-bottom:8px;">
                        <label style="font-size:0.85rem;">${camText('cameras.recording_source_label', '\u9304\u5f71\u4f86\u6e90')}</label>
                        <div style="display:flex;gap:16px;align-items:center;margin-top:4px;">
                            <label style="cursor:pointer;display:flex;align-items:center;gap:6px;">
                                <input type="radio" name="recording_source" value="rtsp" ${cam.recording_source!=='onvif'?'checked':''}>
                                <span>${camText('cameras.recording_source_rtsp', 'RTSP \u76f4\u63a5\u9304\u5f71')}</span>
                            </label>
                            <label style="cursor:pointer;display:flex;align-items:center;gap:6px;">
                                <input type="radio" name="recording_source" value="onvif" ${cam.recording_source==='onvif'?'checked':''}>
                                <span>${camText('cameras.recording_source_onvif', 'ONVIF \u53d6\u6d41\u9304\u5f71')}</span>
                            </label>
                        </div>
                        <small style="color:var(--text-muted);">${camText('cameras.recording_source_hint', 'ONVIF \u6a21\u5f0f\u6bcf\u6b21\u9304\u5f71\u524d\u52d5\u614b\u53d6\u5f97\u6700\u65b0 RTSP \u4f4d\u5740\u3002\u82e5\u8a2d\u5b9a Bitrate \u5247\u6703\u9032\u884c\u8f49\u78bc\u9304\u5f71\u3002')}</small>
                    </div>
                    <label style="display:flex;align-items:center;gap:8px;cursor:pointer;">
                        <input type="checkbox" name="supports_ptz" id="cam-edit-ptz" ${cam.supports_ptz?'checked':''}>
                        <span style="font-size:0.9rem;">${t('cameras.supports_ptz')}</span>
                    </label>
                </div>
            </details>
            <div style="display:flex;justify-content:flex-end;gap:10px;padding-top:10px;border-top:1px solid var(--border-color);">
                <button type="button" class="btn btn-danger" onclick="window.deleteCamera(${id})" style="margin-right:auto;">&#128465; ${camText('cameras.delete', '刪除攝影機')}</button>
                <button type="button" class="btn btn-secondary" onclick="hideModal()">${t('common.cancel')||'\u53d6\u6d88'}</button>
                <button type="submit" class="btn btn-primary">${t('cameras.edit')}</button>
            </div>
        </form>`, { wide: true });
    enhanceCameraSettingsForm('edit', cam);
};

window.submitEditCamera = async function (event, id) {
    event.preventDefault();
    if (!cameraLicensed) return;
    const form = event.target;
    const fd = new FormData(form);
    const data = {
        name: fd.get('name'), location: fd.get('location') || '',
        ip_address: fd.get('ip_address'), port: parseInt(fd.get('port')) || 554,
        rtsp_url: fd.get('rtsp_url'), onvif_url: fd.get('onvif_url') || '',
        username: fd.get('username') || '', manufacturer: fd.get('manufacturer') || '',
        model: fd.get('model') || '',
        supports_ptz: form.querySelector('#cam-edit-ptz')?.checked ?? false,
        stream_type: 'mjpeg',
        monitor_display: fd.get('monitor_display') ? 1 : 0,
        monitor_order: parseInt(fd.get('monitor_order')) || 0,
        recording_bitrate_kbps: parseInt(fd.get('recording_bitrate_kbps')) || 0,
        recording_source: fd.get('recording_source') || 'rtsp',
    };
    const pwd = fd.get('password');
    if (pwd) data.password = pwd;

    const btn = form.querySelector('button[type="submit"]');
    if (btn) btn.disabled = true;
    try {
        const res = await apiPut('/cameras/' + id, data);
        if (res && res.success) {
            if (typeof showToast === 'function') showToast(t('cameras.updated_success'), 'success');
            hideModal();
            await fetchCameras();
        } else {
            if (typeof showToast === 'function') showToast(res?.error || t('cameras.updated_error'), 'error');
            if (btn) btn.disabled = false;
        }
    } catch (e) {
        if (typeof showToast === 'function') showToast(e.message || '新增失敗', 'error');
        if (btn) btn.disabled = false;
    }
};

// ???? Bulk Scan (subnet / IP range / single) ??????????????????????????????????????

window.openBulkScanModal = function () {
    if (!cameraLicensed) {
        if (typeof showToast === 'function') showToast('請先啟用攝影機 License', 'warning');
        return;
    }
    openModal('批次搜尋攝影機', `
        <div style="display:flex;flex-direction:column;gap:14px;">
            <!-- Scan type tabs -->
            <div style="display:flex;gap:8px;border-bottom:1px solid var(--border-color);padding-bottom:10px;">
                <button class="btn btn-sm btn-primary" id="bscan-tab-range"   onclick="window._bscanTab('range')"  >IP 範圍</button>
                <button class="btn btn-sm btn-secondary" id="bscan-tab-subnet" onclick="window._bscanTab('subnet')">子網 CIDR</button>
                <button class="btn btn-sm btn-secondary" id="bscan-tab-single" onclick="window._bscanTab('single')">單一 IP</button>
            </div>
            <!-- Range -->
            <div id="bscan-panel-range">
                <div style="display:grid;grid-template-columns:1fr 1fr;gap:10px;">
                    <div class="form-group" style="margin:0;"><label>起始 IP</label><input id="bscan-start" class="input-field" placeholder="192.168.1.1"></div>
                    <div class="form-group" style="margin:0;"><label>結束 IP</label><input id="bscan-end" class="input-field" placeholder="192.168.1.254"></div>
                </div>
            </div>
            <!-- Subnet -->
            <div id="bscan-panel-subnet" style="display:none;">
                <div class="form-group" style="margin:0;"><label>子網 CIDR，可輸入多段以逗號分隔</label><input id="bscan-subnet" class="input-field" placeholder="192.168.1.0/24, 10.0.0.0/24"></div>
            </div>
            <!-- Single -->
            <div id="bscan-panel-single" style="display:none;">
                <div class="form-group" style="margin:0;"><label>IP 位址</label><input id="bscan-single" class="input-field" placeholder="192.168.1.100"></div>
            </div>

            <!-- Options -->
            <div style="display:grid;grid-template-columns:1fr 80px 80px;gap:10px;align-items:end;">
                <div class="form-group" style="margin:0;">
                    <label>掃描模式</label>
                    <select id="bscan-mode" class="select-input">
                        <option value="both">RTSP + ONVIF</option>
                        <option value="rtsp">僅 RTSP</option>
                        <option value="onvif">僅 ONVIF</option>
                    </select>
                </div>
                <div class="form-group" style="margin:0;"><label>RTSP Port</label><input id="bscan-rport" class="input-field" value="554" type="number"></div>
                <div class="form-group" style="margin:0;"><label>ONVIF Port</label><input id="bscan-oport" class="input-field" value="80" type="number"></div>
            </div>
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:10px;">
                <div class="form-group" style="margin:0;"><label>登入帳號</label><input id="bscan-user" class="input-field" placeholder="admin"></div>
                <div class="form-group" style="margin:0;"><label>登入密碼</label><input id="bscan-pass" class="input-field" type="password"></div>
            </div>
            <label style="display:flex;align-items:center;gap:8px;cursor:pointer;font-size:0.9rem;">
                <input type="checkbox" id="bscan-autoadd"> 掃描完成後自動新增到攝影機清單
            <div id="bscan-result" style="display:none;max-height:260px;overflow-y:auto;border:1px solid var(--border-color);border-radius:6px;"></div>
            <div style="display:flex;justify-content:flex-end;gap:10px;padding-top:8px;border-top:1px solid var(--border-color);">
                <button class="btn btn-secondary" onclick="hideModal()">取消</button>
                <button class="btn btn-primary" id="bscan-submit-btn" onclick="window._submitBulkScan()">開始掃描</button>
            </div>
        </div>`, { wide: true });
    window._bscanType = 'range';
};

window._bscanTab = function (type) {
    window._bscanType = type;
    ['range','subnet','single'].forEach(t => {
        document.getElementById('bscan-panel-' + t).style.display = (t === type) ? '' : 'none';
        const btn = document.getElementById('bscan-tab-' + t);
        if (btn) {
            btn.className = (t === type) ? 'btn btn-sm btn-primary' : 'btn btn-sm btn-secondary';
        }
    });
};

window._submitBulkScan = async function () {
    const btn = document.getElementById('bscan-submit-btn');
    if (btn) { btn.disabled = true; btn.textContent = '掃描中...'; }

    const type = window._bscanType || 'range';
    const payload = {
        scan_type: type,
        scan_mode: document.getElementById('bscan-mode')?.value || 'both',
        rtsp_port: parseInt(document.getElementById('bscan-rport')?.value) || 554,
        onvif_port: parseInt(document.getElementById('bscan-oport')?.value) || 80,
        username: document.getElementById('bscan-user')?.value || '',
        password: document.getElementById('bscan-pass')?.value || '',
        auto_add: document.getElementById('bscan-autoadd')?.checked || false,
        name_prefix: 'Camera-',
    };
    if (type === 'range') {
        payload.start_ip = document.getElementById('bscan-start')?.value?.trim();
        payload.end_ip   = document.getElementById('bscan-end')?.value?.trim();
    } else if (type === 'subnet') {
        payload.subnet = document.getElementById('bscan-subnet')?.value?.trim();
    } else {
        payload.single_ip = document.getElementById('bscan-single')?.value?.trim();
    }

    try {
        const res = await apiPost('/cameras/bulk-scan', payload);
        const resultEl = document.getElementById('bscan-result');
        if (!res || !res.success) throw new Error(res?.error || '?謚??剜??');

        const hits = res.results || [];
        if (hits.length === 0) {
            resultEl.style.display = '';
            resultEl.innerHTML = `<div style="padding:20px;text-align:center;color:var(--text-muted);">沒有找到可加入的攝影機</div>`;
        } else {
            const rows = hits.map(h => `
                <div style="padding:8px 12px;border-bottom:1px solid var(--border-color);display:flex;align-items:center;gap:10px;">
                    <code style="flex:1;font-size:0.85rem;">${escapeHtml(h.ip_address)}</code>
                    <span style="font-size:0.75rem;color:var(--text-muted);">
                        ${h.rtsp_found ? 'RTSP' : ''}${h.onvif_found ? ' / ONVIF' : ''}
                    </span>
                    ${h.already_in ? '<span style="font-size:0.75rem;color:var(--text-muted);">已在清單</span>' :
                      h.added     ? '<span style="font-size:0.75rem;color:var(--success-color);">已新增</span>' :
                      `<button class="btn btn-sm" style="background:var(--success-color);color:#fff;border:none;padding:3px 10px;"
                         onclick="window.quickAddScannedCamera('${escapeHtml(h.ip_address)}',${payload.rtsp_port},'${escapeHtml(payload.username)}','${escapeHtml(payload.password)}',this)">立即新增</button>`}
                </div>`).join('');
            resultEl.style.display = '';
            resultEl.innerHTML = `<div style="padding:8px 12px;font-size:0.85rem;font-weight:600;background:var(--bg-tertiary);">
                找到 ${hits.length} 筆結果 / 已掃描 ${res.scanned} 個 IP${res.added > 0 ? ` / 自動新增 ${res.added} 台` : ''}
            </div>${rows}`;
        }
        if (res.added > 0) await fetchCameras();
    } catch (e) {
        const resultEl = document.getElementById('bscan-result');
        resultEl.style.display = '';
        resultEl.innerHTML = `<div style="padding:16px;color:var(--danger-color);">錯誤: ${escapeHtml(e.message)}</div>`;
    } finally {
        if (btn) { btn.disabled = false; btn.textContent = '開始掃描'; }
    }
};

window.quickAddScannedCamera = async function (ip, port, username, password, btn) {
    if (!cameraLicensed) return;
    if (btn) btn.disabled = true;
    const rtspUrl = username
        ? `rtsp://${encodeURIComponent(username)}:${encodeURIComponent(password)}@${ip}:${port}/stream1`
        : `rtsp://${ip}:${port}/stream1`;
    try {
        const res = await apiPost('/cameras', {
            name: 'Camera-' + ip.replace(/\./g, '-'),
            ip_address: ip, port,
            rtsp_url: rtspUrl,
            username, password,
            stream_type: 'mjpeg',
        });
        if (res && res.success) {
            if (btn) { btn.textContent = '已新增'; btn.style.background = 'var(--text-muted)'; }
            if (typeof showToast === 'function') showToast('已新增到攝影機清單', 'success');
            await fetchCameras();
        } else {
            if (btn) btn.disabled = false;
            if (typeof showToast === 'function') showToast(res?.error || '新增失敗', 'error');
        }
    } catch (e) {
        if (btn) btn.disabled = false;
        if (typeof showToast === 'function') showToast('新增失敗', 'error');
    }
};

// ???? Batch Update Credentials ??????????????????????????????????????????????????????????????????

window.openBatchCredentialsModal = function () {
    if (!cameraLicensed) {
        if (typeof showToast === 'function') showToast('請先啟用攝影機 License', 'warning');
        return;
    }
    const camRows = cameras.map(c => `
        <label style="display:flex;align-items:center;gap:8px;padding:6px 4px;border-bottom:1px solid var(--border-color);cursor:pointer;font-size:0.9rem;">
            <input type="checkbox" class="bcred-cam-cb" value="${c.id}" checked>
            <code style="min-width:110px;">${escapeHtml(c.ip_address)}</code>
            <span style="color:var(--text-muted);">${escapeHtml(c.name)}</span>
        </label>`).join('');

    openModal('批次修改帳密', `
        <div style="display:flex;flex-direction:column;gap:14px;">
            <p style="font-size:0.85rem;color:var(--text-muted);">可一次更新多台攝影機的登入帳號、密碼與 RTSP Port。留空的欄位會維持原值。</p>
            <div style="max-height:200px;overflow-y:auto;border:1px solid var(--border-color);border-radius:6px;padding:4px 8px;">
                <label style="display:flex;align-items:center;gap:8px;padding:4px;font-size:0.85rem;font-weight:600;cursor:pointer;color:var(--text-muted);">
                    <input type="checkbox" id="bcred-all" checked onchange="document.querySelectorAll('.bcred-cam-cb').forEach(cb=>cb.checked=this.checked)">
                    全選 / 取消全選
                </label>
                ${camRows}
            </div>
            <div style="display:grid;grid-template-columns:1fr 1fr 80px;gap:10px;align-items:end;">
                <div class="form-group" style="margin:0;"><label>新帳號</label><input id="bcred-user" class="input-field" placeholder="admin"></div>
                <div class="form-group" style="margin:0;"><label>新密碼</label><input id="bcred-pass" class="input-field" type="password" placeholder="留空則不變更"></div>
                <div class="form-group" style="margin:0;"><label>RTSP Port</label><input id="bcred-port" class="input-field" type="number" placeholder="554"></div>
            </div>
            <label style="display:flex;align-items:center;gap:8px;cursor:pointer;font-size:0.9rem;">
                <input type="checkbox" id="bcred-update-rtsp" checked>
                同步更新 RTSP URL 中的帳號、密碼與 Port
            <div style="display:flex;justify-content:flex-end;gap:10px;padding-top:8px;border-top:1px solid var(--border-color);">
                <button class="btn btn-secondary" onclick="hideModal()">取消</button>
                <button class="btn btn-primary" id="bcred-submit-btn" onclick="window._submitBatchCredentials()">套用變更</button>
            </div>
        </div>`, { wide: true });
};

window._submitBatchCredentials = async function () {
    const btn = document.getElementById('bcred-submit-btn');
    if (btn) { btn.disabled = true; btn.textContent = '套用中...'; }

    const ids = Array.from(document.querySelectorAll('.bcred-cam-cb:checked')).map(cb => parseInt(cb.value));
    if (ids.length === 0) {
        if (typeof showToast === 'function') showToast('請至少選擇一台攝影機', 'warning');
        if (btn) { btn.disabled = false; btn.textContent = '套用變更'; }
        return;
    }

    const username    = document.getElementById('bcred-user')?.value?.trim() || '';
    const password    = document.getElementById('bcred-pass')?.value || '';
    const rtspPort    = parseInt(document.getElementById('bcred-port')?.value) || 0;
    const updateRTSP  = document.getElementById('bcred-update-rtsp')?.checked ?? true;

    if (!username && !password && !rtspPort) {
        if (typeof showToast === 'function') showToast('請至少填寫一個要更新的欄位', 'warning');
        if (btn) { btn.disabled = false; btn.textContent = '套用變更'; }
        return;
    }

    try {
        const res = await apiPut('/cameras/batch/credentials', {
            camera_ids: ids, username, password,
            rtsp_port: rtspPort, update_rtsp: updateRTSP,
        });
        if (res && res.success) {
            if (typeof showToast === 'function')
                showToast("已更新 " + res.updated + " 台" + (res.failed > 0 ? "，失敗 " + res.failed + " 台" : ""), 'success');
            hideModal();
            await fetchCameras();
        } else {
            if (typeof showToast === 'function') showToast(res?.error || '批次更新失敗', 'error');
        }
    } catch (e) {
        if (typeof showToast === 'function') showToast(e.message || '批次更新失敗', 'error');
    } finally {
        if (btn) { btn.disabled = false; btn.textContent = '套用變更'; }
    }
};

// ???? Camera Tab Switch ??????????????????????????????????????????????????????????????????????????????????

window.switchCameraTab = function (tab) {
    const isLive = (tab === 'live');
    document.getElementById('cam-panel-live').style.display = isLive ? '' : 'none';
    document.getElementById('cam-panel-nvr').style.display  = isLive ? 'none' : '';
    const tabLive = document.getElementById('cam-tab-live');
    const tabNvr  = document.getElementById('cam-tab-nvr');
    if (tabLive) { tabLive.className = isLive ? 'btn btn-sm btn-primary' : 'btn btn-sm btn-secondary'; }
    if (tabNvr)  { tabNvr.className  = isLive ? 'btn btn-sm btn-secondary' : 'btn btn-sm btn-primary'; }
    if (!isLive) {
        window.nvrInit();
    }
};

// ???? NVR Management UI ??????????????????????????????????????????????????????????????????????????????????

let _nvrPage = 1;
let _nvrTotalPages = 1;

// ???? NVR state ??????????????????????????????????????????????????????????????????????????????????????????????????
let _nvrSelectedCamId = null; // currently selected camera in NVR panel

window.nvrInit = async function () {
    await window.nvrLoadStatus();
    // Auto-select first camera if none selected
    if (!_nvrSelectedCamId && cameras.length > 0) {
        await window.nvrSelectCamera(cameras[0].id);
    }
};

// Render left camera list with recording status + on/off buttons
window.nvrLoadStatus = async function () {
    try {
        const res = await apiGet('/cameras/recording/status');
        if (res && res.success && Array.isArray(res.cameras)) {
            cameraRecordingStatus = {};
            res.cameras.forEach(r => {
                cameraRecordingStatus[r.id] = { is_recording: r.is_recording, recording_enabled: r.recording_enabled };
            });
        }
    } catch(e) { /* ignore */ }
    window.nvrRenderCamList();
};

window.nvrRenderCamList = function () {
    const listEl = document.getElementById('nvr-cam-list');
    if (!listEl) return;
    if (cameras.length === 0) {
        listEl.innerHTML = `<div style="padding:20px;text-align:center;color:var(--text-muted);font-size:0.85rem;">${camText('cameras.no_cameras', '尚無攝影機')}</div>`;
        return;
    }
    listEl.innerHTML = cameras.map(cam => {
        const recState = cameraRecordingStatus[cam.id] || {};
        const isRec  = recState.is_recording || false;
        const isSelected = cam.id === _nvrSelectedCamId;
        const recDotColor = isRec ? '#ef4444' : '#6b7280';
        const selBg = isSelected
            ? 'background:var(--primary-color);color:#fff;'
            : 'background:var(--bg-card);';
        const textColor = isSelected ? 'color:#fff;' : 'color:var(--text-primary);';
        const subColor  = isSelected ? 'color:rgba(255,255,255,0.7);' : 'color:var(--text-muted);';
        return `
        <div onclick="window.nvrSelectCamera(${cam.id})" style="cursor:pointer;display:flex;align-items:center;gap:10px;padding:10px 12px;border-bottom:1px solid var(--border-color);${selBg}transition:background 0.15s;">
            <span style="width:8px;height:8px;border-radius:50%;flex-shrink:0;background:${recDotColor};${isRec?'box-shadow:0 0 6px #ef4444;':''}"></span>
            <div style="flex:1;min-width:0;">
                <div style="font-size:0.85rem;font-weight:500;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;${textColor}">${escapeHtml(cam.name)}</div>
                <div style="font-size:0.75rem;${subColor}">${escapeHtml(cam.ip_address)}</div>
            </div>
            <button onclick="event.stopPropagation();window.nvrToggle(${cam.id}, ${!isRec})" 
                    style="background:transparent;border:1px solid ${isSelected?'rgba(255,255,255,0.4)':'var(--border-color)'};border-radius:4px;width:28px;height:24px;display:flex;align-items:center;justify-content:center;color:${isSelected?'#fff':'var(--text-secondary)'};cursor:pointer;font-size:0.75rem;"
                    title="${isRec ? camText('cameras.nvr_stop_recording', '停止錄影') : camText('cameras.nvr_start_recording', '開始錄影')}">
                ${isRec ? '&#9632;' : '&#9679;'}
            </button>
        </div>`;
    }).join('');
};

window.nvrSelectCamera = async function (camId) {
    _nvrSelectedCamId = camId;
    window.nvrRenderCamList();
    await window.nvrLoadDates();
    await window.nvrLoadRecordings(1);
};

window.nvrToggle = async function (cameraId, enable) {
    try {
        const res = await apiPut('/cameras/' + cameraId + '/recording', { enabled: enable });
        if (res && res.success) {
            // Optimistic update
            cameraRecordingStatus[cameraId] = { is_recording: enable, recording_enabled: enable };
            window.nvrRenderCamList();
            renderSidebar();
            if (typeof showToast === 'function')
                showToast(enable ? '已開始錄影' : '已停止錄影', enable ? 'success' : 'info');

            // Re-poll after 1.5s to get authoritative state from backend
            setTimeout(async () => {
                try {
                    const sr = await apiGet('/cameras/recording/status');
                    if (sr && sr.success && Array.isArray(sr.cameras)) {
                        cameraRecordingStatus = {};
                        sr.cameras.forEach(r => {
                            cameraRecordingStatus[r.id] = {
                                is_recording: r.is_recording,
                                recording_enabled: r.recording_enabled,
                            };
                        });
                        window.nvrRenderCamList();
                        renderSidebar();
                        // Refresh recording list if stopping
                        if (!enable) window.nvrLoadRecordings(_nvrPage);
                    }
                } catch(e) { /* ignore */ }
            }, 1500);
        } else {
            if (typeof showToast === 'function') showToast(res?.error || '操作失敗', 'error');
        }
    } catch(e) {
        if (typeof showToast === 'function') showToast('操作失敗', 'error');
    }
};

window.nvrLoadDates = async function () {
    const camId = _nvrSelectedCamId || '';
    const url = '/recordings/dates' + (camId ? '?camera_id=' + camId : '');
    try {
        const res = await apiGet(url);
        if (!res || !res.success) return;
        const sel = document.getElementById('nvr-filter-date');
        if (!sel) return;
        const cur = sel.value;
        sel.innerHTML = `<option value="">${camText('cameras.nvr_all_dates', '全部日期')}</option>`;
        (res.dates || []).forEach(d => {
            const o = document.createElement('option');
            o.value = d; o.textContent = d;
            if (d === cur) o.selected = true;
            sel.appendChild(o);
        });
    } catch(e) { /* ignore */ }
};

window.nvrFilterChange = function () {
    window.nvrLoadRecordings(1);
};

// ???? Batch select state ????????????????????????????????????????????????????????????????????????????????
let _nvrBatchMode = false;
let _nvrSelectedIds = new Set();

window.nvrToggleBatchMode = function() {
    _nvrBatchMode = !_nvrBatchMode;
    _nvrSelectedIds.clear();
    const btn = document.getElementById('nvr-batch-select-btn');
    const delBtn = document.getElementById('nvr-batch-delete-btn');
    const countEl = document.getElementById('nvr-batch-count');
    const saWrap = document.getElementById('nvr-select-all-wrap');
    if (btn) btn.style.background = _nvrBatchMode ? 'var(--primary-color)' : '';
    if (btn) btn.style.color = _nvrBatchMode ? '#fff' : '';
    if (delBtn) delBtn.style.display = _nvrBatchMode ? '' : 'none';
    if (countEl) {
        countEl.style.display = _nvrBatchMode ? '' : 'none';
        countEl.textContent = camTextFmt('cameras.nvr_selected', '已選取 {count} 筆', { count: 0 });
    }
    if (saWrap) saWrap.style.display = _nvrBatchMode ? '' : 'none';
    window.nvrLoadRecordings(_nvrPage);
};

window.nvrSelectAll = function(checked) {
    _nvrSelectedIds.clear();
    document.querySelectorAll('.nvr-rec-cb').forEach(cb => {
        cb.checked = checked;
        if (checked) _nvrSelectedIds.add(parseInt(cb.dataset.id));
    });
    const countEl = document.getElementById('nvr-batch-count');
    if (countEl) countEl.textContent = camTextFmt('cameras.nvr_selected', '已選取 {count} 筆', { count: _nvrSelectedIds.size });
};

window.nvrToggleRecSelect = function(id, checked) {
    if (checked) _nvrSelectedIds.add(id);
    else _nvrSelectedIds.delete(id);
    const countEl = document.getElementById('nvr-batch-count');
    if (countEl) countEl.textContent = camTextFmt('cameras.nvr_selected', '已選取 {count} 筆', { count: _nvrSelectedIds.size });
};

window.nvrBatchDelete = async function() {
    if (_nvrSelectedIds.size === 0) { if (typeof showToast === 'function') showToast(t('cameras.nvr_select_first'), 'warning'); return; }
    if (!confirm(t('cameras.nvr_confirm_delete', { count: _nvrSelectedIds.size }))) return;
    try {
        const res = await apiDelete('/recordings/batch', { ids: [..._nvrSelectedIds] });
        if (res && res.success) {
            if (typeof showToast === 'function') showToast(t('cameras.nvr_deleted', { count: res.deleted }), 'success');
            _nvrSelectedIds.clear();
            await window.nvrLoadRecordings(1);
        } else {
            if (typeof showToast === 'function') showToast(res?.error || t('cameras.nvr_delete_failed'), 'error');
        }
    } catch(e) {
        if (typeof showToast === 'function') showToast(t('cameras.nvr_connect_failed'), 'error');
    }
};

window.nvrLoadRecordings = async function (page) {
    _nvrPage = page || 1;
    const camId = _nvrSelectedCamId || '';
    const date  = document.getElementById('nvr-filter-date')?.value || '';

    const listEl = document.getElementById('nvr-recording-list');

    if (!camId) {
        if (listEl) listEl.innerHTML = '<div style="padding:40px;text-align:center;color:var(--text-muted);">請先選擇要查看錄影的攝影機</div>';
        // hide day timeline
        const dtw = document.getElementById('nvr-day-timeline-wrap');
        if (dtw) dtw.style.display = 'none';
        return;
    }

    let url = `/recordings?page=${_nvrPage}&limit=20&camera_id=${camId}`;
    if (date) url += '&date=' + encodeURIComponent(date);

    if (listEl) listEl.innerHTML = `<div style="padding:30px;text-align:center;color:var(--text-muted);">${t('common.loading') || '載入中...'}</div>`;

    try {
        const res = await apiGet(url);
        if (!res || !res.success) throw new Error(res?.error || '讀取錄影清單失敗');
        const recs = res.recordings || [];
        _nvrTotalPages = Math.max(1, Math.ceil(res.total / 20));

        if (recs.length === 0) {
            if (listEl) listEl.innerHTML = `<div style="padding:40px;text-align:center;color:var(--text-muted);">${camText('cameras.nvr_no_recordings', '此條件無錄影記錄')}</div>`;
            const dtw = document.getElementById('nvr-day-timeline-wrap');
            if (dtw) dtw.style.display = 'none';
        } else {
            const locale = (typeof currentLang !== 'undefined' ? currentLang : 'zh-TW').replace('_','-');
            const rows = recs.map(r => {
                const startDt = new Date(r.started_at);
                const startTime = startDt.toLocaleTimeString(locale);
                const startDate = startDt.toLocaleDateString(locale);
                const dur   = r.duration_sec > 0 ? fmtDur(r.duration_sec) : (t('cameras.nvr_recording') || '錄影中');
                const size  = r.file_size > 0 ? fmtBytes(r.file_size) : '-';
                const isRecording = r.status === 'recording';
                const statusBadge = isRecording
                    ? `<span style="color:#ef4444;font-size:0.78rem;">● ${t('cameras.nvr_recording') || '錄影中'}</span>`
                    : `<span style="color:var(--text-muted);font-size:0.78rem;">${t('cameras.nvr_done') || '已完成'}</span>`;
                const cbCol = _nvrBatchMode
                    ? `<input type="checkbox" class="nvr-rec-cb" data-id="${r.id}" ${_nvrSelectedIds.has(r.id)?'checked':''} onchange="window.nvrToggleRecSelect(${r.id},this.checked)" style="width:16px;height:16px;cursor:pointer;">`
                    : `<div></div>`;
                return `<div style="display:grid;grid-template-columns:${_nvrBatchMode?'28px ':''}60px 1fr 70px 60px 110px;gap:6px;align-items:center;padding:8px 14px;border-bottom:1px solid var(--border-color);">
                    ${cbCol}
                    <div style="font-size:0.78rem;color:var(--text-muted);">${startTime}</div>
                    <div style="font-size:0.78rem;">${escapeHtml(r.camera_name||'')} <span style="color:var(--text-muted)">${startDate}</span></div>
                    <div style="font-size:0.8rem;">${dur}</div>
                    <div style="font-size:0.8rem;">${size}</div>
                    <div style="display:flex;gap:4px;align-items:center;flex-wrap:wrap;">
                        ${!isRecording ? `
                        <button title="${camText('cameras.view', '檢視')}" style="padding:2px 7px;background:var(--primary-color);color:#fff;border:none;border-radius:4px;font-size:0.75rem;cursor:pointer;"
                            onclick="window.nvrPlay(${r.id},'${escapeHtml(r.camera_name)} ${startDate} ${startTime}')">檢視</button>
                        <a title="${camText('common.upload', '匯出')}" style="padding:2px 7px;background:var(--bg-tertiary);border:1px solid var(--border-color);border-radius:4px;font-size:0.75rem;text-decoration:none;color:var(--text-primary);cursor:pointer;"
                            href="/api/v1/recordings/${r.id}/export?token=${getAuthToken()}" download>匯出</a>
                        ` : statusBadge}
                        <button title="${camText('common.delete', '刪除')}" style="padding:2px 6px;background:transparent;border:1px solid #ef4444;color:#ef4444;border-radius:4px;font-size:0.75rem;cursor:pointer;"
                            onclick="window.nvrDeleteRec(${r.id})">刪除</button>
                    </div>
                </div>`;
            }).join('');
            if (listEl) listEl.innerHTML = rows;
        }

        // Pagination
        const pagEl = document.getElementById('nvr-pagination');
        if (pagEl) {
            if (_nvrTotalPages <= 1) { pagEl.innerHTML = ''; }
            else {
                let btns = '';
                for (let p = 1; p <= _nvrTotalPages; p++) {
                    btns += `<button class="btn btn-sm ${p === _nvrPage ? 'btn-primary' : 'btn-secondary'}" onclick="window.nvrLoadRecordings(${p})">${p}</button>`;
                }
                pagEl.innerHTML = btns;
            }
        }
        // Load day timeline when date is selected
        if (date) {
            window.nvrLoadDayTimeline();
        } else {
            const dtw = document.getElementById('nvr-day-timeline-wrap');
            if (dtw) dtw.style.display = 'none';
        }
        window.nvrLoadTimeline();
    } catch(e) {
        if (listEl) listEl.innerHTML = `<div style="padding:30px;text-align:center;color:var(--danger-color);">${camText('common.error', '錯誤')}：${escapeHtml(e.message)}</div>`;
    }
};

// ???? Day Timeline (in recording list panel) ????????????????????????????????????????
window.nvrLoadDayTimeline = async function() {
    const camId = _nvrSelectedCamId;
    const date = document.getElementById('nvr-filter-date')?.value;
    const wrap = document.getElementById('nvr-day-timeline-wrap');
    if (!camId || !date || !wrap) return;

    try {
        const res = await apiGet(`/recordings?limit=9999&camera_id=${camId}&date=${encodeURIComponent(date)}`);
        if (!res || !res.success) { wrap.style.display = 'none'; return; }
        const recs = (res.recordings || []).filter(r => r.status === 'done' || r.status === 'recording');
        if (recs.length === 0) { wrap.style.display = 'none'; return; }
        wrap.style.display = '';
        _nvrDayTimelineRecs = recs;
        nvrDrawDayTimeline(recs, date);
    } catch(e) {
        if (wrap) wrap.style.display = 'none';
    }
};

let _nvrDayTimelineRecs = [];

function nvrDrawDayTimeline(recs, date) {
    const ticksEl = document.getElementById('nvr-day-timeline-ticks');
    const blocksEl = document.getElementById('nvr-day-timeline-blocks');
    const nowEl = document.getElementById('nvr-day-timeline-now');
    if (!ticksEl || !blocksEl) return;

    // Draw hour ticks
    let tHTML = '';
    for (let h = 0; h <= 24; h += 3) {
        const left = (h / 24) * 100;
        tHTML += `<div style="position:absolute;left:${left}%;top:0;height:100%;border-left:1px solid rgba(255,255,255,0.12);pointer-events:none;">
            <span style="position:absolute;bottom:2px;left:2px;font-size:0.55rem;color:rgba(255,255,255,0.35);">${String(h).padStart(2,'0')}</span>
        </div>`;
    }
    ticksEl.innerHTML = tHTML;

    // Draw recording blocks
    let bHTML = '';
    const today = new Date().toISOString().slice(0,10);
    recs.forEach(r => {
        const dt = new Date(r.started_at);
        const secOfDay = dt.getHours()*3600 + dt.getMinutes()*60 + dt.getSeconds();
        let dur = r.duration_sec > 0 ? r.duration_sec : Math.max(60, (Date.now() - dt.getTime())/1000);
        const left = (secOfDay / 86400) * 100;
        const width = Math.max((dur / 86400) * 100, 0.3);
        const isActive = r.status === 'recording';
        const color = isActive ? '#ef4444' : 'var(--primary-color,#3b82f6)';
        bHTML += `<div style="position:absolute;left:${left}%;width:${width}%;top:15%;height:70%;background:${color};border-radius:2px;opacity:0.85;box-shadow:0 0 3px rgba(0,0,0,0.4);"
            title="${new Date(r.started_at).toLocaleTimeString()} | ${r.camera_name||''} | ${r.duration_sec>0 ? fmtDur(r.duration_sec) : '錄影中'}"
            data-recid="${r.id}"></div>`;
    });
    blocksEl.innerHTML = bHTML;

    // Show "now" marker if viewing today
    if (nowEl) {
        if (date === today) {
            const now = new Date();
            const nowSec = now.getHours()*3600 + now.getMinutes()*60 + now.getSeconds();
            nowEl.style.left = ((nowSec/86400)*100) + '%';
            nowEl.style.display = '';
        } else {
            nowEl.style.display = 'none';
        }
    }
}

window.nvrDayTimelineClick = function(e) {
    if (!_nvrDayTimelineRecs || _nvrDayTimelineRecs.length === 0) return;
    const rect = e.currentTarget.getBoundingClientRect();
    const clickX = e.clientX - rect.left;
    const clickSec = (clickX / rect.width) * 86400;

    let bestRec = null, minDiff = 999999;
    for (const r of _nvrDayTimelineRecs) {
        const dt = new Date(r.started_at);
        const rStart = dt.getHours()*3600 + dt.getMinutes()*60 + dt.getSeconds();
        const dur = r.duration_sec > 0 ? r.duration_sec : 60;
        if (clickSec >= rStart - 30 && clickSec <= rStart + dur + 30) {
            const offset = Math.max(0, Math.min(clickSec - rStart, dur - 1));
            window.nvrPlayObj(r, offset);
            return;
        }
        const diff = Math.abs(clickSec - (rStart + dur/2));
        if (diff < minDiff) { minDiff = diff; bestRec = r; }
    }
    if (minDiff < 3600 && bestRec) window.nvrPlayObj(bestRec, 0);
};

window._nvrTimelineRecs = [];
window._nvrCurrentPlayingRec = null;

window.nvrLoadTimeline = async function (forceCamId, forceDate) {
    const camId = forceCamId || _nvrSelectedCamId;
    const date  = forceDate  || document.getElementById('nvr-filter-date')?.value;
    if (!camId || !date) return;
    try {
        const res = await apiGet(`/recordings?limit=9999&camera_id=${camId}&date=${encodeURIComponent(date)}`);
        if (res && res.success) {
            window._nvrTimelineRecs = res.recordings || [];
            window.nvrDrawTimelineBlocks();
        }
    } catch(e) {}
};

window.nvrDrawTimelineBlocks = function() {
    const ticksEl = document.getElementById('nvr-timeline-ticks');
    const blocksEl = document.getElementById('nvr-timeline-blocks');
    if (!ticksEl || !blocksEl) return;
    
    if (ticksEl.children.length === 0) {
        let tHTML = '';
        for (let i=0; i<=24; i+=2) {
            const left = (i/24)*100;
            tHTML += `<div style="position:absolute;left:${left}%;top:0;height:100%;border-left:1px solid rgba(255,255,255,0.1);">
                <span style="position:absolute;bottom:0px;left:2px;font-size:0.6rem;color:var(--text-muted);">${i}</span>
            </div>`;
        }
        ticksEl.innerHTML = tHTML;
    }
    
    let bHTML = '';
    window._nvrTimelineRecs.forEach(r => {
        const dt = new Date(r.started_at);
        const secOfDay = dt.getHours()*3600 + dt.getMinutes()*60 + dt.getSeconds();
        let dur = r.duration_sec > 0 ? r.duration_sec : ((Date.now() - dt.getTime())/1000);
        if (dur <= 0) dur = 10;
        const left = (secOfDay / 86400) * 100;
        const width = (dur / 86400) * 100;
        bHTML += `<div style="position:absolute; left:${left}%; width:max(${width}%, 1px); top:25%; height:50%; background:var(--primary-color); border-radius:2px; box-shadow:0 0 2px rgba(0,0,0,0.5);" title="${dt.toLocaleTimeString()} (${Math.round(dur)}s)"></div>`;
    });
    blocksEl.innerHTML = bHTML;
};

window.nvrTimelineClick = function(e) {
    if (!window._nvrTimelineRecs || window._nvrTimelineRecs.length === 0) return;
    const rect = e.currentTarget.getBoundingClientRect();
    const clickX = e.clientX - rect.left;
    const clickSec = (clickX / rect.width) * 86400;

    let bestRec = null;
    let minDiff = 999999;

    for (const r of window._nvrTimelineRecs) {
        const dt = new Date(r.started_at);
        const rStart = dt.getHours()*3600 + dt.getMinutes()*60 + dt.getSeconds();
        const dur = r.duration_sec > 0 ? r.duration_sec : ((Date.now() - dt.getTime())/1000);
        // Expand target slightly mapping
        if (clickSec >= rStart - 60 && clickSec <= rStart + dur + 60) {
            let offset = clickSec - rStart;
            if (offset < 0) offset = 0;
            if (offset > dur) offset = dur - 1;
            window.nvrPlayObj(r, offset);
            return;
        }
        // find nearest
        const diff = Math.abs(clickSec - (rStart + dur/2));
        if (diff < minDiff) {
            minDiff = diff;
            bestRec = r;
        }
    }
    if (minDiff < 3600 && bestRec) {
        window.nvrPlayObj(bestRec, 0); // Jump to nearest if within 1 hr
    }
};

window.nvrPlay = async function (recId, title) {
    // Try to find full recording object from current timeline data first
    let r = (window._nvrTimelineRecs || []).find(x => x.id === recId);
    if (!r) {
        // Fetch recording details from API to get started_at, camera_id, etc.
        try {
            const res = await apiGet(`/recordings?limit=9999`);
            if (res && res.success) {
                r = (res.recordings || []).find(x => x.id === recId);
            }
        } catch(e) {}
    }
    if (!r) r = { id: recId, camera_name: title };
    window.nvrPlayObj(r, 0);
};

window.nvrPlayObj = function (rec, offsetSec = 0) {
    const wrap  = document.getElementById('nvr-player-wrap');
    const video = document.getElementById('nvr-player-video');
    const titleEl = document.getElementById('nvr-player-title');
    if (!wrap || !video || !rec.id) return;

    window._nvrCurrentPlayingRec = rec;
    const token = getAuthToken();
    video.src = `/api/v1/recordings/${rec.id}/play?token=${token}`;

    if (rec.started_at) {
        const startObj = new Date(rec.started_at);
        if (titleEl) titleEl.textContent = `${rec.camera_name || 'Camera'} - ${startObj.toLocaleString()}`;

        // Auto-load timeline for this recording's date if not already loaded for same cam+date
        const recDate = startObj.toISOString().slice(0, 10);
        const camId = rec.camera_id || _nvrSelectedCamId;
        const alreadyLoaded = window._nvrTimelineRecs && window._nvrTimelineRecs.length > 0 &&
            window._nvrTimelineRecs.some(r => r.camera_id === camId &&
                new Date(r.started_at).toISOString().slice(0,10) === recDate);
        if (!alreadyLoaded) {
            window.nvrLoadTimeline(camId, recDate);
        } else {
            window.nvrDrawTimelineBlocks();
        }
    }

    wrap.style.display = '';
    wrap.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    
    video.onloadedmetadata = () => {
        if (offsetSec > 0 && offsetSec < video.duration) {
            video.currentTime = offsetSec;
        }
        video.play().catch(() => {});
    };
    
    video.ontimeupdate = () => {
        if (!window._nvrCurrentPlayingRec || !window._nvrCurrentPlayingRec.started_at) return;
        const dt = new Date(window._nvrCurrentPlayingRec.started_at);
        const currentTotalSec = (dt.getHours()*3600 + dt.getMinutes()*60 + dt.getSeconds()) + video.currentTime;
        
        const cEl = document.getElementById('nvr-timeline-cursor');
        const cLbl = document.getElementById('nvr-timeline-cursor-label');
        const timeEl = document.getElementById('nvr-player-time');
        
        if (cEl) {
            cEl.style.display = 'block';
            cEl.style.left = ((currentTotalSec / 86400) * 100) + '%';
        }
        
        const h = Math.floor(currentTotalSec / 3600)%24;
        const m = Math.floor((currentTotalSec % 3600) / 60);
        const s = Math.floor(currentTotalSec % 60);
        const text = `${h.toString().padStart(2,'0')}:${m.toString().padStart(2,'0')}:${s.toString().padStart(2,'0')}`;
        if (cLbl) cLbl.textContent = text;
        if (timeEl) timeEl.textContent = text;
    };
    
    video.onended = () => {
        if (!window._nvrTimelineRecs || window._nvrTimelineRecs.length === 0) return;
        const sorted = [...window._nvrTimelineRecs].sort((a,b) => new Date(a.started_at) - new Date(b.started_at));
        const idx = sorted.findIndex(x => x.id === rec.id);
        if (idx !== -1 && idx + 1 < sorted.length) {
            const nextRec = sorted[idx + 1];
            // Auto play next clip to ensure continuous playback
            window.nvrPlayObj(nextRec, 0); 
        }
    };
};

window.nvrClosePlayer = function () {
    const wrap  = document.getElementById('nvr-player-wrap');
    const video = document.getElementById('nvr-player-video');
    const cEl = document.getElementById('nvr-timeline-cursor');
    if (video) { video.pause(); video.src = ''; }
    if (wrap)  wrap.style.display = 'none';
    if (cEl) cEl.style.display = 'none';
    window._nvrCurrentPlayingRec = null;
};

window.nvrDeleteRec = async function (recId) {
    if (!confirm('確定要刪除此錄影片段嗎？刪除後將無法復原。')) return;
    try {
        const res = await apiDelete('/recordings/' + recId);
        if (res && res.success) {
            if (typeof showToast === 'function') showToast('錄影片段已刪除', 'success');
            await window.nvrLoadRecordings(_nvrPage);
        } else {
            if (typeof showToast === 'function') showToast(res?.error || '刪除失敗', 'error');
        }
    } catch(e) {
        if (typeof showToast === 'function') showToast('刪除失敗', 'error');
    }
};

// ???? NVR Settings ????????????????????????????????????????????????????????????????????????????????????????????

window.openNVRSettings = async function () {
    if (typeof openModal !== 'function') return;

    // Load current config
    let storageDir = 'recordings';
    let usedBytes = 0;
    try {
        const res = await apiGet('/nvr/config');
        if (res && res.success) {
            storageDir = res.storage_dir || 'recordings';
            usedBytes  = res.used_bytes  || 0;
        }
    } catch(e) { /* ignore */ }

    openModal('NVR 錄影設定', `
        <div style="display:flex;flex-direction:column;gap:16px;">
            <div class="form-group" style="margin:0;">
                <label style="font-weight:600;">錄影儲存路徑</label>
                <p style="font-size:0.8rem;color:var(--text-muted);margin-bottom:6px;">
                    每台攝影機的錄影會儲存在此目錄下的 <code>cam{ID}/{YYYY-MM-DD}/</code> 子資料夾。<br>
                    可輸入本機或掛載磁碟路徑，例如 <code>D:/recordings</code> 或 <code>/mnt/nvr</code>。                </p>
                <input id="nvr-storage-dir-input" class="input-field" value="${escapeHtml(storageDir)}" placeholder="recordings">
            </div>
            <div style="padding:10px 14px;background:var(--bg-secondary);border-radius:6px;border:1px solid var(--border-color);font-size:0.85rem;">
                <div style="color:var(--text-muted);margin-bottom:4px;">目前已使用的錄影空間</div>
                <div style="font-size:1.1rem;font-weight:600;">${fmtBytes(usedBytes)}</div>
            </div>
            <div style="display:flex;justify-content:flex-end;gap:10px;padding-top:8px;border-top:1px solid var(--border-color);">
                <button class="btn btn-secondary" onclick="hideModal()">取消</button>
                <button class="btn btn-primary" onclick="window._saveNVRSettings()">儲存</button>
            </div>
        </div>`);
};

window._saveNVRSettings = async function () {
    const dir = document.getElementById('nvr-storage-dir-input')?.value?.trim();
    if (!dir) {
        if (typeof showToast === 'function') showToast('請輸入有效的錄影路徑', 'warning');
        return;
    }
    try {
        const res = await apiPut('/nvr/config', { storage_dir: dir });
        if (res && res.success) {
            if (typeof showToast === 'function') showToast('NVR 設定已儲存', 'success');
            hideModal();
        } else {
            if (typeof showToast === 'function') showToast(res?.error || '儲存失敗', 'error');
        }
    } catch(e) {
        if (typeof showToast === 'function') showToast('儲存失敗', 'error');
    }
};

function fmtDur(sec) {
    const h = Math.floor(sec / 3600);
    const m = Math.floor((sec % 3600) / 60);
    const s = sec % 60;
    if (h > 0) return `${h}h${String(m).padStart(2,'0')}m`;
    if (m > 0) return `${m}m${String(s).padStart(2,'0')}s`;
    return `${s}s`;
}

function fmtBytes(bytes) {
    if (bytes >= 1e9) return (bytes/1e9).toFixed(1) + ' GB';
    if (bytes >= 1e6) return (bytes/1e6).toFixed(1) + ' MB';
    if (bytes >= 1e3) return (bytes/1e3).toFixed(0) + ' KB';
    return bytes + ' B';
}

window.deleteCamera = async function (id) {
    if (!cameraLicensed) return;
    if (!confirm(t('cameras.delete_confirm'))) return;
    try {
        const res = await apiDelete('/cameras/' + id);
        if (res && res.success) {
            if (typeof showToast === 'function') showToast(t('cameras.deleted_success'), 'success');
            hideModal();
            // Remove from grid
            gridSlots = gridSlots.map(s => s === id ? null : s);
            saveLayout();
            await fetchCameras();
        } else {
            if (typeof showToast === 'function') showToast(res?.error || t('cameras.deleted_error'), 'error');
        }
    } catch (e) {
        if (typeof showToast === 'function') showToast(e.message || '刪除失敗', 'error');
    }
};



// ???? ONVIF probe ??auto-fetch RTSP URL ??????????????????????????????????????????????????
// mode: 'add' (??? modal) | 'edit' (?箏??modal)
window.onvifProbe = async function(mode) {
    const onvifInput = document.getElementById(mode === 'add' ? 'cam-add-onvif' : 'cam-edit-onvif');
    const rtspInput  = document.getElementById(mode === 'add' ? 'cam-add-rtsp'  : 'cam-edit-rtsp');
    const userInput  = document.getElementById(mode === 'add' ? 'cam-add-user'  : 'cam-edit-user');
    const pwdInput   = document.getElementById(mode === 'add' ? 'cam-add-pwd'   : 'cam-edit-pwd');

    const onvifUrl = onvifInput ? onvifInput.value.trim() : '';
    if (!onvifUrl) {
        showToast(t('cameras.onvif_url_required') || '請先輸入 ONVIF 位址', 'warning');
        return;
    }

    const btn = document.querySelector(`[onclick="window.onvifProbe('${mode}')"]`);
    if (btn) { btn.disabled = true; btn.textContent = '取得中...'; }

    try {
        const res = await apiPost('/cameras/onvif/probe', {
            onvif_url: onvifUrl,
            username:  userInput ? userInput.value.trim() : '',
            password:  pwdInput  ? pwdInput.value         : '',
        });
        if (res && res.success && res.data && res.data.rtsp_url) {
            if (rtspInput) rtspInput.value = res.data.rtsp_url;
            showToast('已取得 RTSP 位址', 'success');
        } else {
            showToast((res && res.error) || 'ONVIF 取得失敗', 'error');
        }
    } catch (e) {
        showToast('ONVIF 取得失敗: ' + e.message, 'error');
    } finally {
        if (btn) { btn.disabled = false; btn.innerHTML = '自動取 RTSP'; }
    }
};
