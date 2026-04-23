// ===== Toast Notifications =====
function showToast(message, type = 'info') {
    const container = document.getElementById('toast-container');
    if (!container) return;

    // Limit to max 2 toasts — remove oldest if needed
    while (container.children.length >= 2) {
        container.removeChild(container.firstChild);
    }

    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.textContent = message;

    container.appendChild(toast);

    // Trigger animation
    setTimeout(() => toast.classList.add('show'), 10);

    // Remove after 3 seconds
    setTimeout(() => {
        toast.classList.remove('show');
        setTimeout(() => toast.remove(), 300);
    }, 3000);
}

// ===== Modal System =====
function openModal(title, content, options = {}) {
    const modal = document.getElementById('modal');
    const modalTitle = document.getElementById('modal-title');
    const modalBody = document.getElementById('modal-body');
    const overlay = document.getElementById('modal-overlay');

    if (!modal || !modalTitle || !modalBody || !overlay) {
        console.error('[Modal] Modal elements not found in DOM');
        return;
    }

    // Set title
    if (title && title.includes('.')) {
        modalTitle.setAttribute('data-i18n', title);
        if (typeof t === 'function') {
            modalTitle.textContent = t(title);
        } else {
            modalTitle.textContent = title;
        }
    } else {
        modalTitle.removeAttribute('data-i18n');
        modalTitle.textContent = title;
    }

    // Set content
    modalBody.innerHTML = content;

    // Apply wide modal class if requested
    if (options.wide) {
        modal.classList.add('wide-modal');
    } else {
        modal.classList.remove('wide-modal');
    }

    // Show modal
    overlay.classList.add('active');
    document.body.style.overflow = 'hidden';
}

function hideModal() {
    // Block closing during forced password change
    const requirePwdChange = sessionStorage.getItem('nms_require_pwd_change') === 'true';
    if (requirePwdChange) {
        return;
    }

    const overlay = document.getElementById('modal-overlay');
    const modal = document.getElementById('modal');

    if (overlay) {
        overlay.classList.remove('active');
    }

    if (modal) {
        modal.classList.remove('wide-modal');
    }

    document.body.style.overflow = '';
}

function closeModal() {
    hideModal();
}

function resolveFrontendRoute(route) {
    const input = String(route || '');
    const match = input.match(/^([^?#]*)(.*)$/);
    const rawPath = match ? match[1] : input;
    const suffix = match ? match[2] : '';
    const normalizedPath = rawPath.startsWith('/') ? rawPath : `/${rawPath}`;

    if (!window.location.pathname.startsWith('/static/')) {
        return `${normalizedPath}${suffix}`;
    }

    const staticMap = {
        '/login': '/static/login.html',
        '/login.html': '/static/login.html',
        '/index': '/static/index.html',
        '/index.html': '/static/index.html',
        '/mode-selection': '/static/mode-selection.html',
        '/mode-selection.html': '/static/mode-selection.html',
        '/monitor': '/static/monitor.html',
        '/monitor.html': '/static/monitor.html',
        '/dashboard-view': '/static/dashboard-view.html',
        '/dashboard-view.html': '/static/dashboard-view.html',
        '/reset-password': '/static/reset-password.html',
        '/reset-password.html': '/static/reset-password.html'
    };

    return `${staticMap[normalizedPath] || `/static${normalizedPath}`}${suffix}`;
}
window.resolveFrontendRoute = resolveFrontendRoute;

function navigateToFrontendRoute(route, replace = false) {
    const target = resolveFrontendRoute(route);
    if (replace) {
        window.location.replace(target);
    } else {
        window.location.href = target;
    }
    return target;
}
window.navigateToFrontendRoute = navigateToFrontendRoute;

// Click outside to close
document.addEventListener('DOMContentLoaded', () => {
    const overlay = document.getElementById('modal-overlay');
    if (overlay) {
        overlay.addEventListener('click', (e) => {
            if (e.target === overlay) {
                // Check if this is a forced modal (password change)
                const isForced = sessionStorage.getItem('nms_require_pwd_change') === 'true';
                if (!isForced) {
                    hideModal();
                }
            }
        });
    }
});

// ===== Confirm Dialog =====
// Named 'showConfirm' to avoid overriding the native window.confirm (which returns boolean)
function showConfirm(message, onConfirm, onCancel) {
    const cancelLabel = (typeof t === 'function' && t('common.cancel')) || '取消';
    const confirmLabel = (typeof t === 'function' && t('common.confirm')) || '確定';
    const titleLabel = (typeof t === 'function' && t('common.confirm')) || '確認';

    const content = `
        <p style="margin-bottom:16px;">${message}</p>
        <div class="modal-actions">
            <button class="btn btn-secondary" id="confirm-cancel-btn">${cancelLabel}</button>
            <button class="btn btn-danger" id="confirm-ok-btn">${confirmLabel}</button>
        </div>
    `;
    openModal(titleLabel, content);

    // Attach listeners programmatically
    setTimeout(() => {
        const okBtn = document.getElementById('confirm-ok-btn');
        const cancelBtn = document.getElementById('confirm-cancel-btn');

        if (okBtn) {
            okBtn.onclick = () => {
                closeModal();
                if (typeof onConfirm === 'function') onConfirm();
            };
        }
        if (cancelBtn) {
            cancelBtn.onclick = () => {
                closeModal();
                if (typeof onCancel === 'function') onCancel();
            };
        }
    }, 50);
}
window.showConfirm = showConfirm;

// ===== Format Helpers =====
function formatUptime(seconds) {
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);

    if (days > 0) return `${days}天 ${hours}小時`;
    if (hours > 0) return `${hours}小時 ${minutes}分鐘`;
    return `${minutes}分鐘`;
}

function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
}

function formatDateTime(timestamp) {
    if (!timestamp) return '-';
    let date;
    // RFC3339 / ISO string (from backend)
    if (typeof timestamp === 'string') {
        date = new Date(timestamp);
    } else {
        // Legacy: Unix epoch number
        date = new Date(timestamp * 1000);
    }
    if (isNaN(date.getTime())) return String(timestamp);
    return date.toLocaleString('zh-TW', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false
    });
}

function formatDate(timestamp) {
    if (!timestamp) return '-';
    let date;
    if (typeof timestamp === 'string') {
        date = new Date(timestamp);
    } else {
        date = new Date(timestamp * 1000);
    }
    if (isNaN(date.getTime())) return String(timestamp);
    return date.toLocaleDateString('zh-TW');
}

function formatRelativeTime(dateStr) {
    if (!dateStr) return '-';
    const date = new Date(dateStr);
    if (isNaN(date.getTime())) return dateStr;
    const now = new Date();
    const diff = Math.floor((now - date) / 1000);
    if (diff < 60) return t('common.just_now') || '剛剛';
    if (diff < 3600) return `${Math.floor(diff / 60)} ${t('common.minutes_ago') || '分鐘前'}`;
    if (diff < 86400) return `${Math.floor(diff / 3600)} ${t('common.hours_ago') || '小時前'}`;
    if (diff < 604800) return `${Math.floor(diff / 86400)} ${t('common.days_ago') || '天前'}`;
    return date.toLocaleString('zh-TW', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' });
}

function getStatusBadge(status) {
    const statusMap = {
        'online': { class: 'success', text: '線上' },
        'offline': { class: 'danger', text: '離線' },
        'warning': { class: 'warning', text: '警告' },
        'unknown': { class: 'secondary', text: '未知' }
    };

    const s = statusMap[status] || statusMap['unknown'];
    return `<span class="badge badge-${s.class}">${s.text}</span>`;
}

// ===== Device Helpers (shared between devices.js and topology.js) =====

/**
 * Returns the preferred display name of a device.
 * Priority: User-defined name (if is_name_custom) > sys_name > auto-generated name > ip_address
 */
function getDeviceDisplayName(device) {
    if (!device) return '-';

    // 1. If user explicitly customized the name, use it
    if (device.is_name_custom && device.name) {
        return device.name;
    }

    // 2. Otherwise prefer System Name (SNMP name)
    if (device.sys_name) {
        return device.sys_name;
    }

    // 3. Fallback to name or IP
    return device.name || device.ip_address || '-';
}

/**
 * Returns an emoji icon string for a given device_type string.
 */
function getDeviceTypeIcon(deviceType) {
    const iconMap = {
        'router': '🌐',
        'switch': '🔀',
        'firewall': '🔥',
        'server': '🖥️',
        'ap': '📡',
        'camera': '📹',
        'printer': '🖨️',
        'phone': '📞',
        'ups': '🔋',
        'pdu': '🔌',
        'nas': '💾',
        'pc': '💻',
        'codec': '🎬',
        'access_point': '📡',
        'access_control': '🚪',
        'ipcam': '📹',
        'video_wall': '🖥',
        'other': '📦',
    };
    return iconMap[deviceType] || '📦';
}

/**
 * Returns an HTML string rendering the device icon.
 * Uses a custom uploaded image (image_path) if available,
 * otherwise falls back to the emoji from getDeviceTypeIcon().
 */
function getDeviceIcon(device) {
    if (!device) return '📦';
    if (device.image_path) {
        return `<img src="${device.image_path}" alt="" style="width:32px;height:32px;object-fit:contain;border-radius:4px;" onerror="this.style.display='none';this.nextSibling.style.display='inline'"><span style="display:none;font-size:22px;">${getDeviceTypeIcon(device.device_type)}</span>`;
    }
    return `<span style="font-size:22px;">${getDeviceTypeIcon(device.device_type)}</span>`;
}

window.getDeviceDisplayName = getDeviceDisplayName;
window.getDeviceTypeIcon = getDeviceTypeIcon;
window.getDeviceIcon = getDeviceIcon;
