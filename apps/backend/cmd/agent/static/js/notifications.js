// Made by YTSworks
// YTS工作室製作
// ============================================================
// In-App Notification Center
// 鈴鐺告警中心：輪詢未讀數、彈窗列表、清除功能
// ============================================================

const NOTIF_POLL_INTERVAL = 30000; // 30 秒輪詢一次
let notifPollTimer = null;
let notifPanelOpen = false;
let globalIncidentShown = new Set();

// 初始化：登入後呼叫
function initNotifications() {
    fetchUnreadCount();
    notifPollTimer = setInterval(fetchUnreadCount, NOTIF_POLL_INTERVAL);

    // 點擊面板外部自動關閉
    document.addEventListener('click', (e) => {
        const wrapper = document.getElementById('notif-bell-wrapper');
        if (wrapper && !wrapper.contains(e.target)) {
            closeNotifPanel();
        }
    });
}

function stopNotifications() {
    if (notifPollTimer) {
        clearInterval(notifPollTimer);
        notifPollTimer = null;
    }
}

// 取得未讀數量（不開啟面板時只更新 badge）
async function fetchUnreadCount() {
    try {
        const res = await apiGet('/notifications?unread=1');
        if (res && res.success !== undefined) {
            updateBadge(res.unread_count || 0);
            evaluateGlobalIncidents(res.data || []);
            // 若面板開著，同步更新內容
            if (notifPanelOpen) {
                renderNotifList(res.data || []);
            }
        }
    } catch (e) {
        // 靜默失敗，不影響主介面
    }
}

function evaluateGlobalIncidents(items) {
    const critical = items.filter(item => item.severity === 'critical' || item.severity === 'alert');
    const warningBurst = items.filter(item => item.severity === 'warning').length >= 5;
    const incident = critical[0] || (warningBurst ? items.find(item => item.severity === 'warning') : null);
    if (!incident || globalIncidentShown.has(incident.id)) return;
    globalIncidentShown.add(incident.id);
    showGlobalIncidentModal(incident, critical.length, warningBurst);
}

function showGlobalIncidentModal(incident, criticalCount, warningBurst) {
    const existing = document.getElementById('global-incident-modal');
    if (existing) existing.remove();
    const modal = document.createElement('div');
    modal.id = 'global-incident-modal';
    modal.className = 'global-incident-modal';
    const title = escapeHtmlNotif(incident.title || 'Critical incident');
    const message = escapeHtmlNotif(incident.message || 'An abnormal condition requires attention.');
    const summary = criticalCount > 1 ? `${criticalCount} critical alerts are active.` : (warningBurst ? 'A burst of warning alerts was detected.' : 'A critical alert was detected.');
    const deviceAction = incident.device_id > 0 ? `<button class="btn btn-primary" onclick="openIncidentDevice(${Number(incident.device_id)})">Open device</button>` : '';
    modal.innerHTML = `<div class="global-incident-backdrop"></div><section class="global-incident-card" role="alertdialog" aria-modal="true"><div class="global-incident-icon">!</div><div><p class="global-incident-kicker">Immediate attention</p><h2>${title}</h2><p>${message}</p><p class="global-incident-summary">${summary}</p></div><div class="global-incident-actions">${deviceAction}<button class="btn btn-secondary" onclick="closeGlobalIncidentModal()">Dismiss</button></div></section>`;
    document.body.appendChild(modal);
}

function closeGlobalIncidentModal() { document.getElementById('global-incident-modal')?.remove(); }
function openIncidentDevice(id) { closeGlobalIncidentModal(); if (typeof viewDevice === 'function') viewDevice(id); }

function updateBadge(count) {
    const badge = document.getElementById('notif-badge');
    const btn = document.getElementById('notif-bell-btn');
    if (!badge) return;

    if (count > 0) {
        badge.style.display = 'flex';
        badge.textContent = count > 99 ? '99+' : count;
        btn && btn.classList.add('has-notif');
    } else {
        badge.style.display = 'none';
        btn && btn.classList.remove('has-notif');
    }
}

// 開關面板
async function toggleNotifPanel() {
    const panel = document.getElementById('notif-panel');
    if (!panel) return;

    if (notifPanelOpen) {
        closeNotifPanel();
    } else {
        panel.style.display = 'flex';
        notifPanelOpen = true;
        await loadNotifPanel();
    }
}

function closeNotifPanel() {
    const panel = document.getElementById('notif-panel');
    if (panel) panel.style.display = 'none';
    notifPanelOpen = false;
}

// 載入完整通知列表
async function loadNotifPanel() {
    try {
        const res = await apiGet('/notifications');
        if (res && res.success !== undefined) {
            updateBadge(res.unread_count || 0);
            renderNotifList(res.data || []);
        }
    } catch (e) {
        document.getElementById('notif-list').innerHTML =
            '<div class="notif-empty">載入失敗，請稍後再試</div>';
    }
}

function renderNotifList(items) {
    const list = document.getElementById('notif-list');
    if (!list) return;

    if (!items || items.length === 0) {
        list.innerHTML = '<div class="notif-empty">目前沒有告警通知</div>';
        return;
    }

    list.innerHTML = items.map(n => `
        <div class="notif-item ${n.is_read ? 'notif-read' : 'notif-unread'} notif-sev-${n.severity}" data-id="${n.id}">
            <div class="notif-item-left">
                <span class="notif-sev-dot"></span>
                <div class="notif-item-body">
                    <div class="notif-item-title">${escapeHtmlNotif(n.title)}</div>
                    <div class="notif-item-msg">${escapeHtmlNotif(n.message)}</div>
                    <div class="notif-item-time">${formatNotifTime(n.created_at)}</div>
                </div>
            </div>
            <div class="notif-item-actions">
                ${!n.is_read ? `<button class="notif-item-btn" onclick="markOneRead(${n.id})" title="標記已讀">✓</button>` : ''}
                <button class="notif-item-btn notif-item-del" onclick="deleteOneNotif(${n.id})" title="清除">✕</button>
            </div>
        </div>
    `).join('');
}

// 標記單筆已讀
async function markOneRead(id) {
    try {
        await apiPut(`/notifications/${id}/read`, {});
        await loadNotifPanel();
    } catch (e) {}
}

// 全部標記已讀
async function markAllNotifsRead() {
    try {
        await apiPut('/notifications/read-all', {});
        await loadNotifPanel();
    } catch (e) {}
}

// 刪除單筆
async function deleteOneNotif(id) {
    try {
        await apiDelete(`/notifications/${id}`);
        await loadNotifPanel();
    } catch (e) {}
}

// 全部清除
async function clearAllNotifs() {
    if (!confirm('確定要清除所有告警通知？')) return;
    try {
        await apiDelete('/notifications/all');
        await loadNotifPanel();
    } catch (e) {}
}

// 顯示即時彈出告警（由 WebSocket 或輪詢差異觸發）
function showNotifToast(severity, title, message) {
    const toast = document.createElement('div');
    toast.className = `notif-toast notif-toast-${severity}`;
    toast.innerHTML = `
        <div class="notif-toast-header">
            <span class="notif-toast-icon">${severity === 'warning' ? '⚠️' : severity === 'info' ? 'ℹ️' : '🔴'}</span>
            <strong>${escapeHtmlNotif(title)}</strong>
            <button class="notif-toast-close" onclick="this.parentElement.parentElement.remove()">✕</button>
        </div>
        <div class="notif-toast-msg">${escapeHtmlNotif(message)}</div>
        <div class="notif-toast-footer">
            <button class="notif-toast-ack" onclick="acknowledgeToast(this)">確認清除</button>
        </div>
    `;

    let container = document.getElementById('notif-toast-container');
    if (!container) {
        container = document.createElement('div');
        container.id = 'notif-toast-container';
        document.body.appendChild(container);
    }
    container.appendChild(toast);
}

function acknowledgeToast(btn) {
    const toast = btn.closest('.notif-toast');
    if (toast) toast.remove();
}

// 輔助
function escapeHtmlNotif(text) {
    if (!text) return '';
    const d = document.createElement('div');
    d.textContent = text;
    return d.innerHTML;
}

function formatNotifTime(ts) {
    if (!ts) return '';
    const d = new Date(ts.replace(' ', 'T') + (ts.includes('+') ? '' : 'Z'));
    if (isNaN(d)) return ts;
    return d.toLocaleString('zh-TW', { hour12: false });
}

// apiDelete / apiPut helpers（若 api.js 未定義則補上）
if (typeof apiDelete === 'undefined') {
    window.apiDelete = async function(url) {
        const token = localStorage.getItem('nms_token');
        const res = await fetch(`/api/v1${url}`, {
            method: 'DELETE',
            headers: { 'Authorization': `Bearer ${token}`, 'Cache-Control': 'no-cache' }
        });
        return res.json();
    };
}

if (typeof apiPut === 'undefined') {
    window.apiPut = async function(url, body) {
        const token = localStorage.getItem('nms_token');
        const res = await fetch(`/api/v1${url}`, {
            method: 'PUT',
            headers: {
                'Authorization': `Bearer ${token}`,
                'Content-Type': 'application/json',
                'Cache-Control': 'no-cache'
            },
            body: JSON.stringify(body)
        });
        return res.json();
    };
}
