// 管理功能

// 使用者管理
async function loadUsers() {
    if (typeof isAdmin === 'function' && !isAdmin()) return;
    try {
        const response = await apiGet('/users');
        if (response.success) {
            renderUsers(response.data);
        }
    } catch (error) {
        showToast(t('admin.users.error_load'), 'error');
    }
}

function renderUsers(users) {
    const tbody = document.getElementById('users-tbody');

    if (!users || users.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="5" class="empty-message">${t('admin.users.empty')}</td>
            </tr>
        `;
        return;
    }

    let html = '';
    for (const user of users) {
        html += `
            <tr>
                <td><strong>${escapeHtml(user.username)}</strong></td>
                <td><span class="role-badge ${user.role}">${user.role}</span></td>
                <td><span class="status-badge ${user.is_active ? 'active' : 'inactive'}">${user.is_active ? t('admin.users.status_enabled') : t('admin.users.status_disabled')}</span></td>
                <td>${formatDateTime(user.created_at)}</td>
                <td>
                    <div class="action-buttons">
                        <button class="action-btn" onclick="editUser(${user.id})" title="${t('admin.users.action_edit')}">✏️</button>
                        <button class="action-btn danger" onclick="confirmDeleteUser(${user.id}, '${escapeHtml(user.username)}')" title="${t('admin.users.action_delete')}">🗑️</button>
                    </div>
                </td>
            </tr>
        `;
    }

    tbody.innerHTML = html;
}

function showAddUserModal() {
    const content = `
        <form id="add-user-form" onsubmit="createUser(event)">
            <div class="form-group">
                <label data-i18n="admin.users.username">使用者名稱 *</label>
                <input type="text" name="username" required placeholder="例如: admin" data-i18n="[placeholder]admin.users.username_placeholder">
            </div>
            <div class="form-group">
                <label data-i18n="login.password">密碼 *</label>
                <input type="password" name="password" required minlength="8">
                <small style="color: var(--text-secondary, #64748b); display: block; margin-top: 4px; font-size: 0.8rem;">
                    * 至少 8 碼，需含大寫字母、小寫字母，以及數字或特殊字元
                </small>
            </div>
            <div class="form-group">
                <label data-i18n="admin.users.role">角色</label>
                <select name="role">
                    <option value="viewer" data-i18n="admin.users.role_viewer">檢視者</option>
                    <option value="admin" data-i18n="admin.users.role_admin">管理員</option>
                </select>
            </div>
            <div class="form-actions">
                <button type="button" class="btn btn-secondary" onclick="hideModal()" data-i18n="common.cancel">取消</button>
                <button type="submit" class="btn btn-primary" data-i18n="common.add">新增</button>
            </div>
        </form>
    `;
    openModal('admin.users.add', content);
    applyTranslations();
}

async function createUser(event) {
    event.preventDefault();
    const form = event.target;
    const formData = new FormData(form);

    const password = formData.get('password');

    // Client-side password strength check
    if (password.length < 8 || !/[A-Z]/.test(password) || !/[a-z]/.test(password) || (!/[0-9]/.test(password) && !/[^A-Za-z0-9]/.test(password))) {
        showToast(t('common.password_policy'), 'error');
        return;
    }

    const user = {
        username: formData.get('username'),
        password: password,
        role: formData.get('role')
    };

    try {
        const response = await apiPost('/users', user);
        if (response.success) {
            showToast(t('admin.users.success_add'), 'success');
            hideModal();
            loadUsers();
        }
    } catch (error) {
        showToast(error.message || t('admin.users.error_add'), 'error');
    }
}

async function editUser(id) {
    const content = `
        <form id="edit-user-form" onsubmit="updateUser(event, ${id})">
            <div class="form-group">
                <label data-i18n="admin.users.new_password">新密碼 (留空不修改)</label>
                <input type="password" name="password" minlength="8">
                <small style="color: var(--text-secondary, #64748b); display: block; margin-top: 4px; font-size: 0.8rem;">
                    * 至少 8 碼，需含大寫字母、小寫字母，以及數字或特殊字元
                </small>
            </div>
            <div class="form-group">
                <label data-i18n="admin.users.role">角色</label>
                <select name="role">
                    <option value="viewer" data-i18n="admin.users.role_viewer">檢視者</option>
                    <option value="admin" data-i18n="admin.users.role_admin">管理員</option>
                </select>
            </div>
            <div class="form-group">
                <label data-i18n="common.status">狀態</label>
                <select name="is_active">
                    <option value="true" data-i18n="common.enabled">啟用</option>
                    <option value="false" data-i18n="common.disabled">停用</option>
                </select>
            </div>
            <div class="form-actions">
                <button type="button" class="btn btn-secondary" onclick="hideModal()" data-i18n="common.cancel">取消</button>
                <button type="submit" class="btn btn-primary" data-i18n="common.save">儲存</button>
            </div>
        </form>
    `;
    openModal('admin.users.edit_title', content);
    applyTranslations();
}

async function updateUser(event, id) {
    event.preventDefault();
    const form = event.target;
    const formData = new FormData(form);

    const user = {};
    if (formData.get('password')) {
        const pwd = formData.get('password');
        if (pwd.length < 8 || !/[A-Z]/.test(pwd) || !/[a-z]/.test(pwd) || (!/[0-9]/.test(pwd) && !/[^A-Za-z0-9]/.test(pwd))) {
            showToast(t('common.password_policy'), 'error');
            return;
        }
        user.password = pwd;
    }
    user.role = formData.get('role');
    user.is_active = formData.get('is_active') === 'true';

    try {
        const response = await apiPut(`/users/${id}`, user);
        if (response.success) {
            showToast(t('admin.users.success_update'), 'success');
            hideModal();
            loadUsers();
        }
    } catch (error) {
        showToast(error.message || t('admin.users.error_update'), 'error');
    }
}

function confirmDeleteUser(id, username) {
    showConfirm(
        t('admin.users.confirm_delete', { username: username }),
        () => deleteUser(id)
    );
}

async function deleteUser(id) {
    try {
        const response = await apiDelete(`/users/${id}`);
        if (response.success) {
            showToast(t('admin.users.success_delete'), 'success');
            loadUsers();
        }
    } catch (error) {
        showToast(error.message || t('admin.users.error_delete'), 'error');
    }
}

// ===== 授權管理 (Simplified License Management) =====

async function loadLicenseStatus() {
    try {
        const response = await apiGet('/license/status');
        if (response.success) {
            renderLicenseStatus(response.data);
        }
    } catch (error) {
        showToast(error.message || t('admin.licenses.error_load'), 'error');
    }
}

async function loadMachineID() {
    try {
        const response = await apiGet('/license/machine-id');
        if (response.success) {
            const machineIdEl = document.getElementById('machine-id-display');
            if (machineIdEl) {
                machineIdEl.textContent = response.data.raw_id;
            }
        }
    } catch (error) {
        console.error('Failed to load machine ID:', error);
    }
}

function renderLicenseStatus(status) {
    // 更新設備使用量
    const currentEl = document.getElementById('current-device-count');
    const maxEl = document.getElementById('max-device-count');

    if (currentEl) currentEl.textContent = status.current_devices;
    if (maxEl) maxEl.textContent = status.max_devices;

    // 更新進度條
    const percentage = status.max_devices > 0
        ? Math.min(100, (status.current_devices / status.max_devices) * 100)
        : 0;
    const progressBar = document.getElementById('device-usage-bar');
    if (progressBar) {
        progressBar.style.width = percentage + '%';
        progressBar.className = 'progress-fill ' + (percentage >= 90 ? 'danger' : percentage >= 70 ? 'warning' : 'normal');
    }

    // 更新試用版卡片
    const trialCard = document.getElementById('trial-card');
    const trialBtn = document.getElementById('trial-activate-btn');
    if (trialCard && trialBtn) {
        if (!status.trial_available) {
            trialCard.classList.add('used');
            trialBtn.textContent = t('admin.licenses.trial_used');
            trialBtn.disabled = true;
        }
    }

    // 渲染授權列表
    renderLicenseList(status.licenses);
}

function renderLicenseList(licenses) {
    const tbody = document.getElementById('licenses-tbody');

    if (!licenses || licenses.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="6" class="empty-message">${t('admin.licenses.empty')}</td>
            </tr>
        `;
        return;
    }

    let html = '';
    for (const license of licenses) {
        const statusClass = license.status === 'active' ? 'active' :
            license.status === 'trial' ? 'trial' :
                license.status === 'expired' ? 'expired' : 'inactive';
        const statusText = license.status === 'active' ? t('admin.licenses.status_active') :
            license.status === 'trial' ? t('admin.licenses.status_trial') :
                license.status === 'expired' ? t('admin.licenses.status_expired') : t('admin.licenses.status_invalid');

        const features = license.features && license.features.length > 0
            ? license.features.map(f => getFeatureLabel(f)).join(', ')
            : '-';

        // Build capacity display: devices and/or cameras
        let capacityParts = [];
        if (license.device_count > 0) capacityParts.push(`設備 +${license.device_count}`);
        if (license.camera_count > 0) capacityParts.push(`攝影機 ${license.camera_count} 路`);
        const capacityText = capacityParts.length > 0 ? capacityParts.join(' / ') : '-';
        const validUntilText = license.is_permanent
            ? t('admin.licenses.valid_forever')
            : ((license.valid_until && !license.valid_until.startsWith('9999')) ? formatDate(license.valid_until) : t('admin.licenses.valid_forever'));

        html += `
            <tr class="${statusClass === 'expired' ? 'license-expired' : ''}">
                <td><code class="license-key">${escapeHtml(license.license_key)}</code></td>
                <td>${getLicenseTypeLabel(license.license_type)}</td>
                <td class="device-count">${capacityText}</td>
                <td>${features}</td>
                <td>${validUntilText}</td>
                <td><span class="license-status-badge ${statusClass}">${statusText}</span></td>
            </tr>
        `;
    }

    tbody.innerHTML = html;
}

function getFeatureLabel(feature) {
    const labels = {
        'email': '📧 Email',
        'line': '💬 LINE',
        'telegram': '✈️ Telegram',
        'whatsapp': '📱 WhatsApp',
        'camera_viewer': '📷 攝影機監控'
    };
    return labels[feature] || feature;
}

function getLicenseTypeLabel(type) {
    const labels = {
        'trial': `🎁 ${t('admin.licenses.type_trial')}`,
        'standard': `📋 ${t('admin.licenses.type_standard')}`,
        'professional': `⭐ ${t('admin.licenses.type_professional')}`,
        'enterprise': `🏢 ${t('admin.licenses.type_enterprise')}`
    };
    return labels[type] || type;
}

async function activateTrial() {
    showConfirm(
        t('admin.licenses.confirm_trial'),
        async () => {
            try {
                const response = await apiPost('/license/activate', { license_key: 'TRIAL' });
                if (response.success) {
                    showToast(response.message || t('admin.licenses.trial_activated'), 'success');
                    loadLicenseStatus();
                }
            } catch (error) {
                showToast(error.message || t('admin.licenses.activate_error'), 'error');
            }
        }
    );
}

function showActivateLicenseModal() {
    const content = `
        <form id="activate-license-form" onsubmit="activateLicense(event)">
            <div class="form-group">
                <label>${t('admin.licenses.license_key_label')}</label>
                <textarea name="license_key" required 
                    placeholder="${t('admin.licenses.license_key_placeholder')}" 
                    rows="4" 
                    style="width: 100%; font-family: monospace; font-size: 12px;"></textarea>
            </div>
            <div class="license-how-to">
                <p><strong>${t('admin.licenses.how_to_get_key_title')}</strong></p>
                <ol>
                    <li>${t('admin.licenses.how_to_step1')}</li>
                    <li>${t('admin.licenses.how_to_step2')}</li>
                    <li>${t('admin.licenses.how_to_step3')}</li>
                    <li>${t('admin.licenses.how_to_step4')}</li>
                </ol>
            </div>
            <div class="form-actions">
                <button type="button" class="btn btn-secondary" onclick="hideModal()">${t('common.cancel')}</button>
                <button type="submit" class="btn btn-primary">${t('admin.licenses.activate_button')}</button>
            </div>
        </form>
    `;
    openModal('admin.licenses.activate_modal_title', content);
}

async function activateLicense(event) {
    event.preventDefault();
    const form = event.target;
    const formData = new FormData(form);
    const licenseKey = formData.get('license_key').trim();
    const wasLocked = typeof isLicenseLockActive === 'function' && isLicenseLockActive();

    if (!licenseKey) {
        showToast(t('admin.licenses.enter_key'), 'error');
        return;
    }

    try {
        const response = await apiPost('/license/activate', { license_key: licenseKey });
        if (response.success) {
            // Updated feedback messages
            const msg = response.message || t('admin.licenses.activate_success');
            if (licenseKey.startsWith('DEV-')) {
                showToast(t('admin.licenses.device_license_added'), 'success');
            } else if (licenseKey.startsWith('ALM-')) {
                showToast(t('admin.licenses.alarm_license_added'), 'success');
            } else {
                showToast(msg, 'success');
            }

            hideModal();

            if (wasLocked) {
                if (typeof clearLicenseLockState === 'function') {
                    clearLicenseLockState();
                } else {
                    sessionStorage.removeItem('nms_license_locked');
                    sessionStorage.removeItem('nms_license_lock_reason');
                }
                setTimeout(() => window.location.reload(), 300);
                return;
            }

            // Critical Refresh: Refresh both license list AND module availability
            if (typeof loadLicenses === 'function') loadLicenses();
            if (typeof loadLicenseStatus === 'function') loadLicenseStatus();
            if (typeof loadModuleConfigs === 'function') loadModuleConfigs();

            // Refresh camera viewer nav if relevant
            if (typeof updateModuleVisibility === 'function') {
                setTimeout(updateModuleVisibility, 500);
            }
        }
    } catch (error) {
        showToast(error.message || t('admin.licenses.activate_error'), 'error');
    }
}

// ===== Unlock Hidden Features =====
// Secret click counter for "系統管理" title
let adminTitleClickCount = 0;
let adminTitleClickTimer = null;

// Initialize click listener when page loads
document.addEventListener('DOMContentLoaded', function() {
    const adminTitle = document.getElementById('admin-title-trigger');
    if (adminTitle) {
        adminTitle.addEventListener('click', function() {
            adminTitleClickCount++;

            // Reset counter after 3 seconds of no clicks
            clearTimeout(adminTitleClickTimer);
            adminTitleClickTimer = setTimeout(() => {
                adminTitleClickCount = 0;
            }, 3000);

            // Trigger unlock modal after 8 clicks
            if (adminTitleClickCount === 8) {
                adminTitleClickCount = 0; // Reset
                unlockHiddenFeatures();
            }
        });
    }
});

async function unlockHiddenFeatures() {
    const content = `
        <form id="unlock-form" onsubmit="handleUnlock(event)">
            <p style="text-align: center; margin-bottom: 10px; font-weight: 500;" data-i18n="admin.unlock.prompt">${t('admin.unlock.prompt') || '請輸入驗證資訊'}</p>
            <div class="form-group">
                <input type="text" name="username" required autocomplete="off">
            </div>
            <div class="form-group">
                <input type="password" name="password" required autocomplete="off">
            </div>
            <div class="form-actions">
                <button type="submit" class="btn btn-primary" style="width: 100%;">OK</button>
            </div>
        </form>
    `;
    openModal('', content);
    // Apply translations after modal is opened
    if (typeof applyTranslations === 'function') {
        setTimeout(applyTranslations, 150);
    }
}

async function handleUnlock(event) {
    event.preventDefault();
    const form = event.target;
    const formData = new FormData(form);

    // Get credentials
    const username = formData.get('username');
    const password = formData.get('password');

    // Hard-coded SuperAdmin validation (frontend only, no backend call)
    const SUPER_ADMIN_USERNAME = 'SuperAdmin';
    const SUPER_ADMIN_PASSWORD = 'P@ssw0rd#1@';

    if (username === SUPER_ADMIN_USERNAME && password === SUPER_ADMIN_PASSWORD) {
        showToast(t('admin.toast.auth_success') || '驗證成功', 'success');
        closeModal();

        // Set global unlocked state
        document.body.classList.add('advanced-unlocked');
        localStorage.setItem('advancedModeUnlocked', 'true');

        // Show Branding & Tools Buttons (both Sidebar and Admin tabs)
        // The CSS will handle showing these when body has 'advanced-unlocked' class
        document.querySelectorAll('button[data-tab="branding"], button[data-tab="tools"]').forEach(btn => {
            btn.classList.add('advanced-only'); // Ensure it has the class for CSS control
        });
    } else {
        showToast(t('admin.toast.auth_failed') || '驗證失敗', 'error');
    }
}

function copyMachineID() {
    const machineIdEl = document.getElementById('machine-id-display');
    if (!machineIdEl || !machineIdEl.textContent || machineIdEl.textContent === t('common.loading') || machineIdEl.textContent === t('admin.licenses.loading_id')) {
        showToast(t('admin.toast.machine_id_not_loaded'), 'warning');
        return;
    }

    const text = machineIdEl.textContent;

    // Try modern clipboard API first (requires HTTPS or localhost)
    if (navigator.clipboard && window.isSecureContext) {
        navigator.clipboard.writeText(text).then(() => {
            showToast(t('admin.licenses.copy_success'), 'success');
        }).catch(() => {
            fallbackCopy(text);
        });
    } else {
        fallbackCopy(text);
    }
}

function fallbackCopy(text) {
    const textArea = document.createElement('textarea');
    textArea.value = text;
    // Position offscreen but still accessible
    textArea.style.position = 'fixed';
    textArea.style.left = '-9999px';
    textArea.style.top = '0';
    textArea.style.opacity = '0';
    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();

    try {
        const success = document.execCommand('copy');
        showToast(success ? t('admin.licenses.copy_success') : t('admin.licenses.copy_error'), success ? 'success' : 'error');
    } catch (err) {
        showToast(t('admin.licenses.copy_error'), 'error');
    }

    document.body.removeChild(textArea);
}

function formatDate(dateStr) {
    if (!dateStr) return '-';
    const date = new Date(dateStr);
    return date.toLocaleDateString(typeof currentLang !== 'undefined' ? currentLang : 'zh-TW');
}

// 舊版API兼容
async function loadLicenses() {
    await loadLicenseStatus();
    await loadMachineID();
}


// ===== Alert Settings =====

// Store settings globally
let currentAlertSettings = [];

async function loadAlertSettings() {
    if (typeof isAdmin === 'function' && !isAdmin()) return;
    try {
        const response = await apiGet('/alerts/settings');
        if (response.success) {
            currentAlertSettings = response.data;
            renderAlertCards(response.data);

            // 檢查 Email 設定以決定是否啟用安全設定頁籤
            checkSecurityTabVisibility(currentAlertSettings);
        }
    } catch (error) {
        console.error('Failed to load alert settings:', error);
        showToast(error.message || t('admin.toast.load_alert_failed'), 'error');
    }
}

function checkSecurityTabVisibility(settings) {
    const emailSetting = settings.find(s => s.alert_type === 'email');
    // Check if enabled AND has some config (basic check)
    let isConfigured = false;
    if (emailSetting && emailSetting.config) {
        try {
            const config = JSON.parse(emailSetting.config);
            isConfigured = !!config.smtp_host;
        } catch (e) { }
    }

    const isEmailEnabled = emailSetting && emailSetting.is_enabled && isConfigured;

    const securityTabBtn = document.querySelector('.tab-btn[data-tab="security"]');
    if (securityTabBtn) {
        if (isEmailEnabled) {
            securityTabBtn.style.display = '';
            securityTabBtn.classList.remove('disabled-tab');
            securityTabBtn.removeAttribute('title');
        } else {
            // Hide the tab completely as requested
            securityTabBtn.style.display = 'none';
            securityTabBtn.classList.remove('disabled-tab');
            securityTabBtn.removeAttribute('title');

            // If currently on security tab, switch away
            if (securityTabBtn.classList.contains('active')) {
                document.querySelector('.tab-btn[data-tab="alerts"]').click();
            }
        }
    }
}

// Make toggleAlert globally available for the onchange event in HTML
window.toggleAlert = async function (type, enabled) {
    try {
        const setting = currentAlertSettings.find(s => s.alert_type === type);
        let config = {};
        if (setting && setting.config) {
            try {
                config = JSON.parse(setting.config);
            } catch (e) {
                console.error("Error parsing config", e);
            }
        }

        const response = await apiPost('/alerts/settings', {
            alert_type: type,
            is_enabled: enabled,
            config: JSON.stringify(config)
        });

        if (response.success) {
            showToast(enabled ? t('common.enabled') : t('common.disabled'), 'success');
            // Reload settings to update UI (including security tab visibility)
            loadAlertSettings();
        } else {
            // Revert on failure
            loadAlertSettings();
            showToast(response.error || t('admin.toast.save_failed'), 'error');
        }
    } catch (error) {
        console.error('Toggle alert failed:', error);
        loadAlertSettings();
        showToast(error.message || t('admin.toast.save_failed'), 'error');
    }
};

function renderAlertCards(settings) {
    const container = document.getElementById('alert-settings-container');
    if (!container) return;

    // Alert type definitions — requires license feature name (null = free / always visible)
    const alertTypes = [
        { type: 'email',                icon: '📧', nameKey: 'admin.alerts.email',                descKey: 'admin.alerts.desc_email',                requireLicense: null },
        { type: 'telegram',             icon: '✈️', nameKey: 'admin.alerts.telegram',             descKey: 'admin.alerts.desc_telegram',             requireLicense: 'telegram' },
        { type: 'discord',              icon: '🎮', nameKey: 'admin.alerts.discord',               descKey: 'admin.alerts.desc_discord',               requireLicense: 'discord' },
        { type: 'slack',                icon: '📝', nameKey: 'admin.alerts.slack',                 descKey: 'admin.alerts.desc_slack',                 requireLicense: 'slack' },
        { type: 'line',                 icon: '💬', nameKey: 'admin.alerts.line',                  descKey: 'admin.alerts.desc_line',                  requireLicense: 'line' },
        { type: 'whatsapp',             icon: '📱', nameKey: 'admin.alerts.whatsapp',              descKey: 'admin.alerts.desc_whatsapp',              requireLicense: 'whatsapp' },
        { type: 'pdu_alert',            icon: '🔌', nameKey: 'admin.alerts.pdu_alert',             descKey: 'admin.alerts.desc_pdu_alert',             requireLicense: 'pdu' },
        { type: 'access_control_alert', icon: '🚪', nameKey: 'admin.alerts.access_control_alert',  descKey: 'admin.alerts.desc_access_control_alert',  requireLicense: 'access_control' },
    ];

    // v1.2.1: Alert filter toolbar — "只顯示已啟用" switch
    const showOpenOnly = !!window._alertFilterOpenOnly;
    let filterToolbarHtml = `
    <div id="alert-filter-toolbar" style="display:flex;align-items:center;gap:12px;margin-bottom:16px;padding:10px 14px;background:var(--bg-secondary);border:1px solid var(--border-color);border-radius:8px;flex-wrap:wrap;">
        <span style="font-size:0.85rem;font-weight:500;color:var(--text-primary);">
            📢 ${t('admin.tabs.alerts') || '告警設定'}
        </span>
        <label style="display:flex;align-items:center;gap:6px;cursor:pointer;margin-left:auto;font-size:0.82rem;color:var(--text-secondary);">
            <input type="checkbox" id="alert-open-only-chk" ${showOpenOnly ? 'checked' : ''}
                onchange="window._alertFilterOpenOnly = this.checked; renderAlertCards(currentAlertSettings);"
                style="width:15px;height:15px;cursor:pointer;">
            <span>${t('admin.alerts.show_open_only') || '只顯示已啟用'}</span>
        </label>
    </div>`;

    let html = filterToolbarHtml;
    let visibleCount = 0;

    for (const alertDef of alertTypes) {
        const setting = settings?.find(s => s.alert_type === alertDef.type) || {};
        const isEnabled = setting.is_enabled || false;
        // has_license is provided by backend; default true for free types
        const hasLicense = alertDef.requireLicense === null ? true : (setting.has_license === true);

        // No license → hide completely (do not show locked state)
        if (!hasLicense) continue;

        // "只顯示已啟用" filter
        if (showOpenOnly && !isEnabled) continue;

        visibleCount++;
        const typeName = t(alertDef.nameKey) || alertDef.type;
        const description = t(alertDef.descKey) || '';

        html += `
        <div class="alert-card ${isEnabled ? 'enabled' : ''}">
            <div class="alert-card-header">
                <span class="alert-icon">${alertDef.icon}</span>
                <h3>${typeName}</h3>
                ${alertDef.requireLicense === null
                    ? `<span class="free-badge">${t('admin.alerts.free')}</span>`
                    : `<span class="free-badge" style="background:var(--success-color,#22c55e);">✓ ${t('admin.licenses.status_active') || '已授權'}</span>`
                }
            </div>
            <p class="alert-description">${description}</p>
            <div class="alert-card-toggle">
                <label class="toggle-switch">
                    <input type="checkbox" ${isEnabled ? 'checked' : ''} onchange="toggleAlert('${alertDef.type}', this.checked)">
                    <span class="toggle-slider"></span>
                </label>
                <span>${isEnabled ? t('common.enabled') : t('common.disabled')}</span>
            </div>
            <div class="alert-card-actions">
                <button class="btn btn-secondary btn-sm" onclick="showAlertConfigModal('${alertDef.type}')">${t('common.settings')}</button>
                <button class="btn btn-primary btn-sm" onclick="testAlert('${alertDef.type}')">${t('common.test')}</button>
            </div>
        </div>
        `;
    }

    // Show empty state when filter yields no results
    if (visibleCount === 0) {
        html += `<div style="padding:40px;text-align:center;color:var(--text-muted);">${t('admin.alerts.no_open_alerts') || '目前沒有已啟用的告警管道'}</div>`;
    }

    container.innerHTML = html;
}

function showAlertConfigModal(type) {
    const setting = currentAlertSettings.find(s => s.alert_type === type) || {};
    let config = {};
    try {
        config = setting.config ? JSON.parse(setting.config) : {};
    } catch (e) {
        console.error('Failed to parse config:', e);
    }

    const configs = {
        email: `
            <div class="form-group">
                <label>${t('admin.alerts.smtp_host')}</label>
                <input type="text" name="smtp_host" placeholder="smtp.example.com" value="${escapeHtml(config.smtp_host || '')}">
            </div>
            <div class="form-group">
                <label>${t('admin.alerts.smtp_port')}</label>
                <input type="number" name="smtp_port" value="${config.smtp_port || 587}">
            </div>
            <div class="form-group">
                <label>${t('admin.alerts.username')}</label>
                <input type="text" name="username" placeholder="your@email.com" value="${escapeHtml(config.username || '')}">
            </div>
            <div class="form-group">
                <label>${t('admin.alerts.password')}</label>
                <input type="password" name="password" value="${escapeHtml(config.password || '')}">
            </div>
            <div class="form-group">
                <label>${t('admin.alerts.from_address')}</label>
                <input type="email" name="from_address" placeholder="nms@company.com" value="${escapeHtml(config.from_address || '')}">
            </div>
            <div class="form-group">
                <label>${t('admin.alerts.to_addresses')}</label>
                <input type="text" name="to_addresses" placeholder="admin@company.com, ops@company.com" value="${escapeHtml(config.to_addresses || '')}">
            </div>
        `,
        discord: `
            <div class="form-group">
                <label>${t('admin.alerts.webhook_url')}</label>
                <input type="text" name="webhook_url" placeholder="https://discord.com/api/webhooks/..." value="${escapeHtml(config.webhook_url || '')}">
            </div>
        `,
        slack: `
            <div class="form-group">
                <label>${t('admin.alerts.webhook_url')}</label>
                <input type="text" name="webhook_url" placeholder="https://hooks.slack.com/services/..." value="${escapeHtml(config.webhook_url || '')}">
            </div>
        `,
        line: `
            <div class="form-group">
                <label>${t('admin.alerts.line_token')}</label>
                <input type="text" name="access_token" placeholder="Token" value="${escapeHtml(config.access_token || '')}">
                <small>${t('admin.alerts.line_token_hint')}</small>
            </div>
        `,
        telegram: `
            <div class="form-group">
                <label>${t('admin.alerts.bot_token')}</label>
                <input type="text" name="bot_token" placeholder="123456:ABC-DEF..." value="${escapeHtml(config.bot_token || '')}">
            </div>
            <div class="form-group">
                <label>${t('admin.alerts.chat_id')}</label>
                <input type="text" name="chat_id" placeholder="${t('admin.alerts.chat_id_hint')}" value="${escapeHtml(config.chat_id || '')}">
            </div>
        `,
        whatsapp: `
            <div class="form-group">
                <label>${t('admin.alerts.api_key')}</label>
                <input type="text" name="api_key" value="${escapeHtml(config.api_key || '')}">
            </div>
            <div class="form-group">
                <label>${t('admin.alerts.phone_number')}</label>
                <input type="text" name="phone_number" placeholder="${t('admin.alerts.phone_hint')}" value="${escapeHtml(config.phone_number || '')}">
            </div>
        `,
        pdu_alert: `
            <p style="margin:0 0 12px;font-size:0.85rem;color:var(--text-secondary);">${t('admin.alerts.pdu_alert_desc') || '啟用後，PDU/UPS 發生異常（斷電、過載、低電量等）時將透過已啟用的告警管道發送通知。'}</p>
            <div class="form-group">
                <label>${t('admin.alerts.pdu_power_threshold') || '功率警戒閾值 (W)'}</label>
                <input type="number" name="power_threshold_w" placeholder="0" value="${config.power_threshold_w || 0}" min="0">
                <small>${t('admin.alerts.pdu_power_threshold_hint') || '設為 0 表示不依功率觸發，僅依狀態變化通知'}</small>
            </div>
        `,
        access_control_alert: `
            <p style="margin:0 0 12px;font-size:0.85rem;color:var(--text-secondary);">${t('admin.alerts.access_control_alert_desc') || '啟用後，門禁發生異常（強制開門、拒絕刷卡等）時將透過已啟用的告警管道發送通知。'}</p>
            <div class="form-group">
                <label>${t('admin.alerts.ac_notify_events') || '通知事件類型'}</label>
                <div style="display:flex;flex-direction:column;gap:6px;margin-top:4px;">
                    <label style="display:flex;align-items:center;gap:8px;font-weight:400;">
                        <input type="checkbox" name="notify_denied" value="1" ${config.notify_denied ? 'checked' : ''}> ${t('admin.alerts.ac_notify_denied') || '拒絕進入'}
                    </label>
                    <label style="display:flex;align-items:center;gap:8px;font-weight:400;">
                        <input type="checkbox" name="notify_forced" value="1" ${config.notify_forced ? 'checked' : ''}> ${t('admin.alerts.ac_notify_forced') || '強制開門'}
                    </label>
                    <label style="display:flex;align-items:center;gap:8px;font-weight:400;">
                        <input type="checkbox" name="notify_offline" value="1" ${config.notify_offline ? 'checked' : ''}> ${t('admin.alerts.ac_notify_offline') || '門禁控制器離線'}
                    </label>
                </div>
            </div>
        `
    };

    const content = `
        <form id="alert-config-form" onsubmit="saveAlertConfig(event, '${type}')">
            ${configs[type] || `<p>${t('admin.alerts.no_options')}</p>`}
            <div class="form-actions">
                <button type="button" class="btn btn-secondary" onclick="hideModal()">${t('common.cancel')}</button>
                <button type="submit" class="btn btn-primary">${t('common.save')}</button>
            </div>
        </form>
    `;
    openModal(t('admin.alerts.config_title', { type: type.toUpperCase() }), content);
}

async function saveAlertConfig(event, type) {
    event.preventDefault();
    const form = event.target;
    const formData = new FormData(form);
    const config = {};
    // First pass: collect non-checkbox fields
    formData.forEach((value, key) => {
        if (key === 'smtp_port') {
            config[key] = parseInt(value, 10);
        } else {
            config[key] = value;
        }
    });
    // Second pass: handle checkboxes (unchecked ones are absent from FormData)
    form.querySelectorAll('input[type="checkbox"]').forEach(cb => {
        config[cb.name] = cb.checked;
    });

    try {
        const response = await apiPut(`/alerts/settings/${type}`, {
            is_enabled: true,
            config: JSON.stringify(config)
        });
        if (response.success) {
            showToast(t('admin.toast.save_success'), 'success');
            hideModal();
            loadAlertSettings();
        }
    } catch (error) {
        showToast(error.message || t('admin.toast.save_failed'), 'error');
    }
}

async function testAlert(type) {
    try {
        showToast(t('admin.toast.testing_alert'), 'info');
        const response = await apiPost(`/alerts/test/${type}`, {});
        if (response.success) {
            showToast(response.message || t('admin.toast.test_sent'), 'success');
        }
    } catch (error) {
        showToast(error.message || t('admin.toast.test_failed'), 'error');
    }
}

// ===== Branding Settings =====

async function loadBrandingSettings() {
    try {
        const response = await apiGet('/branding');
        if (response.success) {
            const data = response.data;

            // Update current logo preview
            const logoImg = document.getElementById('current-logo-img');
            const noLogoText = document.getElementById('no-logo-text');
            if (data.logo_path) {
                logoImg.src = data.logo_path;
                logoImg.style.display = 'block';
                noLogoText.style.display = 'none';
            } else {
                logoImg.style.display = 'none';
                noLogoText.style.display = 'block';
            }

            // Update company name input
            const nameInput = document.getElementById('company-name-input');
            if (nameInput) {
                nameInput.value = data.company_name || '';
            }

            // Update font size
            const fontSizeInput = document.getElementById('company-name-font-size');
            const fontSizeValue = document.getElementById('font-size-value');
            if (fontSizeInput && data.font_size) {
                fontSizeInput.value = data.font_size;
                if (fontSizeValue) fontSizeValue.textContent = data.font_size;
            }

            // Update font color
            const fontColorInput = document.getElementById('company-name-color');
            const fontColorValue = document.getElementById('color-hex-value');
            if (fontColorInput && data.font_color) {
                fontColorInput.value = data.font_color;
                if (fontColorValue) fontColorValue.textContent = data.font_color;
            }

            // Update position select
            const positionSelect = document.getElementById('company-name-position');
            if (positionSelect && data.name_position) {
                positionSelect.value = data.name_position;
            }

            // Show/Hide Delete Button
            const deleteBtn = document.getElementById('btn-delete-logo');
            if (deleteBtn) {
                deleteBtn.style.display = data.logo_path ? 'block' : 'none';
            }

            // Update sidebar logo
            updateSidebarLogo(data);
        }
    } catch (error) {
        console.error('Failed to load branding:', error);
    }
}

function updateSidebarLogo(data) {
    const logoContainer = document.querySelector('.sidebar-header .logo');
    const sidebarLogo = document.getElementById('sidebar-logo');
    const defaultIcon = document.getElementById('default-logo-icon');
    const companyName = document.getElementById('sidebar-company-name');

    if (data.logo_path && sidebarLogo) {
        sidebarLogo.src = data.logo_path;
        sidebarLogo.style.display = 'block';
        if (defaultIcon) defaultIcon.style.display = 'none';
    } else {
        // No custom logo set, hide image
        if (sidebarLogo) sidebarLogo.style.display = 'none';
        // User request: Default icon should be hidden by default
        if (defaultIcon) defaultIcon.style.display = 'none';
    }

    if (companyName) {
        companyName.textContent = data.company_name ?? 'Management System';
        if (data.font_size) {
            companyName.style.setProperty('font-size', data.font_size + 'px', 'important');
        }
        if (data.font_color) {
            companyName.style.color = data.font_color;
            // Override CSS gradient text effects using setProperty for prefix support
            companyName.style.setProperty('-webkit-text-fill-color', data.font_color, 'important');
            companyName.style.background = 'none';
        } else {
            // Restore default gradient if no color set
            companyName.style.setProperty('-webkit-text-fill-color', 'transparent');
            companyName.style.background = '';
        }
    }

    // Apply name position (top, bottom, left, right)
    if (logoContainer) {
        const position = data.name_position || 'right';
        // Remove all position classes
        logoContainer.classList.remove('name-top', 'name-bottom', 'name-left', 'name-right');
        logoContainer.classList.add('name-' + position);

        // Adjust flex direction based on position
        if (position === 'top') {
            logoContainer.style.flexDirection = 'column-reverse';
            logoContainer.style.alignItems = 'center';
        } else if (position === 'bottom') {
            logoContainer.style.flexDirection = 'column';
            logoContainer.style.alignItems = 'center';
        } else if (position === 'left') {
            logoContainer.style.flexDirection = 'row-reverse';
            logoContainer.style.alignItems = 'center';
        } else { // right (default)
            logoContainer.style.flexDirection = 'row';
            logoContainer.style.alignItems = 'center';
        }
    }
}

// Update Font Size Preview
function updateFontSizePreview(value) {
    const fontSizeValue = document.getElementById('font-size-value');
    if (fontSizeValue) fontSizeValue.textContent = value;

    const companyName = document.getElementById('sidebar-company-name');
    if (companyName) {
        companyName.style.fontSize = value + 'px';
    }
}

function previewLogoFile(input) {
    if (input.files && input.files[0]) {
        const reader = new FileReader();
        reader.onload = function (e) {
            const preview = document.getElementById('logo-preview-img');
            const placeholder = input.parentElement.querySelector('.upload-placeholder');
            preview.src = e.target.result;
            preview.style.display = 'block';
            if (placeholder) placeholder.style.display = 'none';
        };
        reader.readAsDataURL(input.files[0]);
    }
}

async function uploadCompanyLogo() {
    const input = document.getElementById('logo-file-input');
    if (!input.files || !input.files[0]) {
        showToast(t('admin.toast.select_image'), 'warning');
        return;
    }

    const formData = new FormData();
    formData.append('logo', input.files[0]);

    try {
        const response = await apiUpload('/branding/logo', formData);
        if (response.success) {
            showToast(t('admin.toast.upload_success'), 'success');
            loadBrandingSettings();
            // Reset preview
            document.getElementById('logo-preview-img').style.display = 'none';
            const placeholder = document.querySelector('.logo-upload-area .upload-placeholder');
            if (placeholder) placeholder.style.display = 'flex';
        }
    } catch (error) {
        showToast(error.message || t('admin.toast.upload_failed'), 'error');
    }
}

async function deleteCompanyLogo() {
    showConfirm(t('admin.confirm.delete_logo'), async () => {
        try {
            const response = await apiDelete('/branding/logo');
            if (response.success) {
                showToast(t('admin.toast.logo_deleted'), 'success');
                loadBrandingSettings();
            }
        } catch (error) {
            showToast(error.message || t('common.operation_failed'), 'error');
        }
    });
}

async function saveBrandingInfo() {
    const name = document.getElementById('company-name-input').value;
    const fontSize = document.getElementById('company-name-font-size').value;
    const position = document.getElementById('company-name-position').value;
    const fontColor = document.getElementById('company-name-color').value;

    try {
        const response = await apiPut('/branding', {
            company_name: name,
            font_size: fontSize,
            name_position: position,
            font_color: fontColor
        });
        if (response.success) {
            showToast(t('admin.toast.save_success'), 'success');
            loadBrandingSettings();
        }
    } catch (error) {
        showToast(error.message || t('admin.toast.save_failed'), 'error');
    }
}

async function downloadBackup() {
    showToast(t('admin.toast.backup_preparing'), 'info');
    // apiDownload now uses window.location.href, so we don't await response
    // Remove '/api' prefix as apiDownload adds it
    apiDownload('/system/backup');
    // Since we can't track download status with window.location, just notify it's requested
    showToast(t('admin.toast.backup_started'), 'success');
}

async function uploadRestore() {
    const input = document.getElementById('restore-file-input');
    const file = input?.files[0];
    if (!file) {
        showToast(t('admin.toast.select_backup'), 'warning');
        return;
    }

    // Check system readiness (Solution C) - Merged check
    let confirmMsg = t('admin.confirm.restore_warning');

    try {
        const res = await apiGet('/system/restore-readiness');
        if (res.success && res.data && !res.data.ready) {
            const issues = res.data.issues.join('\n• ');
            confirmMsg = t('admin.confirm.restore_readiness_issue', { issues: issues }) + t('admin.confirm.restore_warning');
        }
    } catch (e) {
        console.warn('Readiness check skipped:', e);
    }

    const performRestore = async () => {
        const formData = new FormData();
        formData.append('backup_file', file);

        const btn = document.getElementById('btn-restore-upload');
        const statusDiv = document.getElementById('restore-status');
        const statusText = document.getElementById('restore-status-text');
        const progressBar = document.getElementById('restore-progress-bar');

        const originalText = btn.innerHTML;
        btn.disabled = true;
        btn.innerHTML = `<span>⏳</span> ${t('admin.backup.processing')}`;

        // 顯示狀態區域
        statusDiv.style.display = 'block';
        statusText.textContent = t('admin.toast.restore_uploading');
        progressBar.style.display = 'block';

        try {
            // Use fetch directly to have full control, sometimes custom headers trigger WAF
            const token = sessionStorage.getItem('nms_token');
            const response = await fetch('/api/v1/system/restore', {
                method: 'POST',
                headers: {
                    'Authorization': `Bearer ${token}`,
                    // 'Content-Type': 'multipart/form-data', // Do NOT set this manually with FormData
                    'X-Requested-With': 'XMLHttpRequest'
                },
                body: formData
            });

            if (!response.ok) {
                const errText = await response.text();
                let msg = 'Upload failed';
                try {
                    const json = JSON.parse(errText);
                    msg = json.error || msg;
                } catch (e) { }
                throw new Error(msg);
            }

            statusText.textContent = t('admin.toast.restore_uploading');
            showToast(t('admin.toast.restore_uploading'), 'success');

            // Wait and reload
            setTimeout(() => {
                statusText.textContent = t('admin.toast.refreshing');
                waitForServerRestart();
            }, 5000);

        } catch (error) {
            console.error('Restore error:', error);
            statusText.textContent = '❌ ' + t('common.operation_failed') + ': ' + error.message;
            statusDiv.style.backgroundColor = 'var(--danger-light)';
            progressBar.style.display = 'none';
            showToast(t('common.operation_failed') + ': ' + error.message, 'error');
            btn.disabled = false;
            btn.innerHTML = originalText;

            // 3秒後隱藏錯誤訊息
            setTimeout(() => {
                statusDiv.style.display = 'none';
                statusDiv.style.backgroundColor = 'var(--bg-tertiary)';
            }, 5000);
        }
    };

    showConfirm(confirmMsg, performRestore);
}

async function waitForServerRestart() {
    let retries = 0;
    const maxRetries = 30; // 30 attempts, 2s each = 60s
    const statusText = document.getElementById('restore-status-text');

    const checkInterval = setInterval(async () => {
        retries++;
        statusText.textContent = `${t('admin.toast.restarting')} (${retries}/${maxRetries})`;

        try {
            const res = await fetch('/api/v1/system/info');
            if (res.ok) {
                clearInterval(checkInterval);
                statusText.textContent = t('admin.toast.restarting');
                showToast(t('admin.toast.restarting'), 'success');

                // Clear all storage
                localStorage.clear();
                sessionStorage.clear();

                // Clear all cached data to ensure topology and other data are refreshed
                if ('caches' in window) {
                    caches.keys().then(names => {
                        names.forEach(name => caches.delete(name));
                    });
                }

                // Force redirect to login after a short delay
                setTimeout(() => {
                    // Use a timestamp to prevent caching issues
                    // Use location.replace to prevent browser from caching the redirect
                    const ts = new Date().getTime();
                    window.location.replace(`/login?restored=true&ts=${ts}`);
                }, 2000);
            }
        } catch (e) {
            // Still offline
            if (retries > maxRetries) {
                clearInterval(checkInterval);
                statusText.textContent = `⚠️ ${t('admin.toast.no_response')}`;
                showToast(t('admin.toast.no_response'), 'warning');
            }
        }
    }, 2000);
}


async function resetLicense() {
    showConfirm(t('admin.confirm.license_reset'), async () => {
        try {
            const response = await apiPost('/license/reset', {});
            if (response.success) {
                showToast(t('admin.toast.license_reset'), 'success');
                setTimeout(() => window.location.reload(), 1500);
            }
        } catch (error) {
            showToast(t('admin.toast.reissue_failed', { error: error.message }), 'error');
        }
    });
}

async function reissueLicense() {
    const oldMachineId = document.getElementById('reissue-old-id').value.trim();
    const oldLicenseKey = document.getElementById('reissue-old-key').value.trim();

    if (!oldMachineId || !oldLicenseKey) {
        showToast(t('admin.toast.reissue_failed', { error: t('admin.licenses.enter_key') }), 'warning');
        return;
    }

    const btn = document.getElementById('btn-reissue');
    const originalText = btn.innerHTML;
    btn.disabled = true;
    btn.innerHTML = t('common.please_wait');

    try {
        const response = await apiPost('/license/reissue', {
            old_machine_id: oldMachineId,
            old_license_key: oldLicenseKey
        });

        if (response.success) {
            showToast(t('admin.toast.reissue_success'), 'success');
            // Clear inputs
            document.getElementById('reissue-old-id').value = '';
            document.getElementById('reissue-old-key').value = '';
            // Refresh table
            loadLicenses();
        }
    } catch (error) {
        showToast(t('admin.toast.reissue_failed', { error: error.message }), 'error');
    } finally {
        btn.disabled = false;
        btn.innerHTML = originalText;
    }
}

// Load branding on page load for sidebar logo
document.addEventListener('DOMContentLoaded', () => {
    // Load branding for sidebar
    fetch('/api/v1/branding')
        .then(res => res.json())
        .then(data => {
            if (data.success) {
                updateSidebarLogo(data.data);

                // Check for advanced mode unlock state - DISABLED for strict default hidden
                // if (localStorage.getItem('advancedModeUnlocked') === 'true') {
                //    document.body.classList.add('advanced-unlocked');
                // }
                // Important: Add classes to hidden buttons on load so CSS can handle them
                document.querySelectorAll('button[data-tab="branding"], button[data-tab="tools"]').forEach(btn => {
                    btn.classList.add('advanced-only');
                });
            }
        })
        .catch(() => { });

    // Admin tab switching
    document.querySelectorAll('.admin-tabs .tab-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            const tabName = btn.dataset.tab;

            if (typeof isLicenseLockActive === 'function' && isLicenseLockActive() &&
                typeof isAllowedLockedAdminTab === 'function' && !isAllowedLockedAdminTab(tabName)) {
                if (typeof activateAdminTab === 'function' && typeof getPreferredLockedAdminTab === 'function') {
                    activateAdminTab(getPreferredLockedAdminTab());
                }
                return;
            }

            // Update active button
            document.querySelectorAll('.admin-tabs .tab-btn').forEach(b => b.classList.remove('active'));
            btn.classList.add('active');

            // Update active tab content
            document.querySelectorAll('.admin-tab-content').forEach(tc => tc.classList.remove('active'));
            const tabContent = document.getElementById(tabName + '-tab');
            if (tabContent) tabContent.classList.add('active');

            // Load data for the tab
            switch (tabName) {
                case 'users':
                    loadUsers();
                    break;
                case 'licenses':
                    loadLicenses();
                    break;
                case 'alerts':
                    loadAlertSettings();
                    break;
                case 'branding':
                    loadBrandingSettings();
                    break;
                case 'security':
                    loadSecuritySettings();
                    break;
                case 'host-status':
                    loadHostStatus();
                    break;
                case 'tools':
                    // No initial data to load for tools
                    break;
                case 'modules':
                    loadModuleConfigs();
                    break;
            }
        });
    });

    // Default load: Active tab
    const preferredLockedBtn = (typeof isLicenseLockActive === 'function' && isLicenseLockActive() &&
        typeof getPreferredLockedAdminTab === 'function')
        ? document.querySelector(`.admin-tabs .tab-btn[data-tab="${getPreferredLockedAdminTab()}"]`)
        : null;
    const activeTabBtn = preferredLockedBtn || document.querySelector('.admin-tabs .tab-btn.active');
    if (activeTabBtn) {
        // Trigger click to load initial data and set view
        activeTabBtn.click();
    } else {
        // Fallback if no active class in HTML
        const firstBtn = document.querySelector('.admin-tabs .tab-btn');
        if (firstBtn) firstBtn.click();
    }
});

// ==========================================
// 診斷工具 (Diagnostics Tools) v1.0.5
// ==========================================

async function runPingTool() {
    const targetInput = document.getElementById('ping-targets').value;
    const targets = targetInput.split(/[\r\n,]+/).map(t => t.trim()).filter(t => t);
    const outputEl = document.getElementById('ping-output');
    const btn = document.getElementById('btn-ping-run');

    if (targets.length === 0) {
        alert(t('admin.tools.ping_at_least_one'));
        return;
    }

    // Reset UI
    outputEl.textContent = t('admin.tools.ping_running');
    btn.disabled = true;
    btn.innerHTML = `<span>⏳</span> ${t('admin.tools.running_btn')}`;

    try {
        const token = sessionStorage.getItem('nms_token');
        const response = await fetch('/api/tools/ping', {
            method: 'POST',
            headers: {
                'Authorization': `Bearer ${token}`,
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ targets: targets })
        });

        if (response.ok) {
            const data = await response.json();
            if (data.success) {
                outputEl.textContent = data.output;
            } else {
                outputEl.textContent = t('admin.tools.execution_failed', { error: data.error });
            }
        } else {
            outputEl.textContent = t('admin.tools.server_error', { error: response.statusText });
        }
    } catch (error) {
        outputEl.textContent = t('admin.tools.network_error', { error: error.message });
    } finally {
        btn.disabled = false;
        btn.innerHTML = `<span>📡</span> ${t('admin.tools.ping_btn')}`;
    }
}

async function runTracerouteTool() {
    const target = document.getElementById('traceroute-target').value.trim();
    const outputEl = document.getElementById('traceroute-output');
    const btn = document.getElementById('btn-tracert-run');

    if (!target) {
        alert(t('admin.tools.input_required'));
        return;
    }

    // Reset UI
    outputEl.textContent = t('admin.tools.tracert_running');
    btn.disabled = true;
    btn.innerHTML = `<span>⏳</span> ${t('admin.tools.running_btn')}`;

    try {
        const token = sessionStorage.getItem('nms_token');
        const response = await fetch('/api/tools/traceroute', {
            method: 'POST',
            headers: {
                'Authorization': `Bearer ${token}`,
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ target: target })
        });

        if (response.ok) {
            const data = await response.json();
            if (data.success) {
                outputEl.textContent = data.output;
            } else {
                outputEl.textContent = t('admin.tools.execution_failed', { error: data.error });
            }
        } else {
            outputEl.textContent = t('admin.tools.server_error', { error: response.statusText });
        }
    } catch (error) {
        outputEl.textContent = t('admin.tools.network_error', { error: error.message });
    } finally {
        btn.disabled = false;
        btn.innerHTML = `<span>🗺️</span> ${t('admin.tools.traceroute_btn')}`;
    }
}
// Task 1: Admin Authentication Prompt
function promptAdminAuth(onSuccess) {
    const content = `
        <form id="admin-egg-auth-form">
            <div class="form-group">
                <label data-i18n="admin.unlock_username">${t('admin.unlock_username') || '請輸入管理員帳號'}</label>
                <input type="text" name="username" required class="text-input" placeholder="${t('admin.unlock_username_placeholder')}" data-i18n="[placeholder]admin.unlock_username_placeholder">
            </div>
            <div class="form-group" style="margin-top: 10px;">
                <label data-i18n="admin.unlock_password">${t('admin.unlock_password') || '請輸入管理員密碼'}</label>
                <input type="password" name="password" required class="text-input" placeholder="${t('admin.unlock_password_placeholder')}" data-i18n="[placeholder]admin.unlock_password_placeholder">
            </div>
            <div class="form-actions" style="margin-top: 20px;">
                <button type="button" class="btn btn-secondary" onclick="hideModal()" data-i18n="admin.unlock_cancel">${t('admin.unlock_cancel') || '取消'}</button>
                <button type="submit" class="btn btn-primary" data-i18n="admin.unlock_submit">${t('admin.unlock_submit') || '解鎖功能'}</button>
            </div>
        </form>
    `;
    openModal('admin.unlock', content);
    // Apply translations after modal is opened
    if (typeof applyTranslations === 'function') {
        setTimeout(applyTranslations, 150);
    }

    // Bind event manually after modal is open
    setTimeout(() => {
        const form = document.getElementById('admin-egg-auth-form');
        if (form) {
            form.addEventListener('submit', async (e) => {
                e.preventDefault();
                const formData = new FormData(e.target);
                const data = Object.fromEntries(formData);

                try {
                    // Use /api/auth/login to verify credentials
                    const response = await fetch('/api/auth/login', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify(data)
                    });

                    const result = await response.json();

                    if (result.success) {
                        // Check if role is admin
                        if (result.data && result.data.user && result.data.user.role === 'admin') {
                            showToast(t('admin.toast.auth_success'), 'success');
                            hideModal();
                            if (onSuccess) onSuccess();
                        } else {
                            showToast(t('admin.toast.insufficient_perms'), 'error');
                        }
                    } else {
                        showToast(result.error || t('admin.toast.auth_failed'), 'error');
                    }
                } catch (err) {
                    showToast(t('admin.toast.auth_failed') + ': ' + err.message, 'error');
                }
            });
        }
    }, 100);
}

// Helper for Host Status Gauge
function renderUsageGauge(value, label, color, textValue) {
    const circumference = 2 * Math.PI * 40; // r=40
    const offset = circumference - (value / 100) * circumference;

    return `
        <div class="gauge-container" style="position: relative; width: 100px; height: 100px; display: flex; align-items: center; justify-content: center;">
            <svg width="100" height="100" viewBox="0 0 100 100" style="transform: rotate(-90deg);">
                <circle cx="50" cy="50" r="40" stroke="#333" stroke-width="8" fill="none" style="opacity: 0.2"></circle>
                <circle cx="50" cy="50" r="40" stroke="${color}" stroke-width="8" fill="none" stroke-dasharray="${circumference}" stroke-dashoffset="${offset}" style="transition: stroke-dashoffset 0.5s ease;"></circle>
            </svg>
            <div class="gauge-text" style="position: absolute; text-align: center;">
                <div style="font-size: 14px; font-weight: bold; color: var(--text-primary);">${label}</div>
                <div style="font-size: 12px; color: var(--text-muted);">${textValue}</div>
            </div>
        </div>
    `;
}

/**
 * Loads Host Status from backend
 */
async function loadHostStatus() {
    // host-status is admin-only — silently skip for non-admin roles
    if (typeof isAdmin === 'function' && !isAdmin()) return;
    try {
        const response = await apiGet('/system/host-status');
        if (response.success) {
            const data = response.data;

            // Update Info
            const setVal = (id, val) => {
                const el = document.getElementById(id);
                if (el) el.textContent = val;
            };

            setVal('host-os', data.os);
            setVal('host-platform', data.platform + ' ' + data.platform_version);
            setVal('host-hostname', data.hostname);

            // GPU Info
            if (data.gpu) {
                const g = data.gpu;
                setVal('host-gpu-name', g.name || '-');
                const methodMap = { cuda: 'NVIDIA CUDA (NVDEC)', qsv: 'Intel Quick Sync (QSV)', vaapi: 'VAAPI (Linux)', d3d11va: 'D3D11VA (Windows)', none: 'CPU 軟解 (無 GPU 加速)' };
                const methodEl = document.getElementById('host-gpu-method');
                if (methodEl) {
                    methodEl.textContent = methodMap[g.method] || g.method;
                    methodEl.style.color = g.available ? 'var(--success-color)' : 'var(--text-secondary)';
                }
                setVal('host-gpu-maxcams', g.available ? `${g.max_cams} ch` : `${g.max_cams} ch (CPU 限制)`);
            }

            // Format uptime
            const uptimeHours = Math.floor(data.uptime / 3600);
            const uptimeMins = Math.floor((data.uptime % 3600) / 60);
            setVal('host-uptime', `${uptimeHours}${t('common.hour')} ${uptimeMins}${t('common.minute')}`);

            // CPU Gauge
            // CPU Gauge
            const cpu = Number((data.cpu_usage || 0).toFixed(1));
            const cpuColor = cpu > 80 ? 'var(--danger-color)' : (cpu > 60 ? 'var(--warning-color)' : 'var(--success-color)');
            const cpuBox = document.getElementById('host-cpu-gauge-box');
            if (cpuBox) cpuBox.innerHTML = renderUsageGauge(cpu, 'CPU', cpuColor, cpu + '%');

            // Memory Gauge & Text
            const memTotal = data.mem_total || 1;
            const memUsed = data.mem_used || 0;
            const memP = Number(((memUsed / memTotal) * 100).toFixed(1));
            const memColor = memP > 85 ? 'var(--danger-color)' : (memP > 70 ? 'var(--warning-color)' : 'var(--primary-color)');
            const memBox = document.getElementById('host-mem-gauge-box');
            if (memBox) memBox.innerHTML = renderUsageGauge(memP, 'Memory', memColor, formatBytes(memUsed));
            setVal('host-mem-val', `${formatBytes(memUsed)} / ${formatBytes(memTotal)} (${memP}%)`);

            // Disk Gauge & Text
            const diskTotal = data.disk_total || 1;
            const diskUsed = data.disk_used || 0;
            const diskP = Number(((diskUsed / diskTotal) * 100).toFixed(1));
            const diskColor = diskP > 90 ? 'var(--danger-color)' : (diskP > 75 ? 'var(--warning-color)' : '#8b5cf6');
            const diskBox = document.getElementById('host-disk-gauge-box');
            if (diskBox) diskBox.innerHTML = renderUsageGauge(diskP, 'Disk', diskColor, formatBytes(diskUsed));
            setVal('host-disk-val', `${formatBytes(diskUsed)} / ${formatBytes(diskTotal)} (${diskP}%)`);

        } else {
            showToast(t('admin.toast.load_host_failed') + ': ' + response.error, 'error');
        }
    } catch (error) {
        console.error('Host status error:', error);
        showToast(t('admin.toast.load_host_failed'), 'error');
    }
}

// ===== Security Settings =====

let currentSecuritySettings = {};
let currentTwoFactorStatus = null;
let pendingTwoFactorEnrollment = null;

function ensurePersonalTwoFactorUI() {
    const securityCard = document.querySelector('#security-tab .card');
    if (!securityCard || document.getElementById('personal-2fa-section')) return;

    const section = document.createElement('div');
    section.id = 'personal-2fa-section';
    section.innerHTML = `
        <hr style="margin: 24px 0; border: 0; border-top: 1px solid var(--border-color);">
        <h3>我的二階段驗證</h3>
        <p class="text-muted" style="margin-bottom: 16px;">
            為目前登入帳號啟用 TOTP 驗證與備援碼。這組設定獨立於告警 Email，可在未設定郵件告警時使用。
        </p>
        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 12px; margin-bottom: 16px;">
            <div class="form-group" style="margin: 0;">
                <label>目前狀態</label>
                <div id="personal-2fa-enabled" class="text-muted">讀取中...</div>
            </div>
            <div class="form-group" style="margin: 0;">
                <label>主要方式</label>
                <div id="personal-2fa-primary" class="text-muted">-</div>
            </div>
            <div class="form-group" style="margin: 0;">
                <label>備援碼剩餘數量</label>
                <div id="personal-2fa-recovery-remaining" class="text-muted">-</div>
            </div>
            <div class="form-group" style="margin: 0;">
                <label>備援碼更新時間</label>
                <div id="personal-2fa-recovery-generated" class="text-muted">-</div>
            </div>
        </div>
        <div id="personal-2fa-note" class="text-muted" style="margin-bottom: 16px;"></div>

        <div id="personal-2fa-enroll-card" style="display: none;">
            <div class="form-group">
                <label for="personal-2fa-password">目前密碼</label>
                <input type="password" id="personal-2fa-password" class="text-input" placeholder="請輸入目前密碼">
            </div>
            <button class="btn btn-primary" onclick="beginPersonalTwoFactorEnrollment()">
                開始設定 TOTP
            </button>

            <div id="personal-2fa-confirm-panel" style="display: none; margin-top: 16px; padding: 16px; border: 1px solid var(--border-color); border-radius: 10px; background: var(--bg-secondary);">
                <div class="form-group">
                    <label for="personal-2fa-secret">TOTP 密鑰</label>
                    <input type="text" id="personal-2fa-secret" class="text-input" readonly>
                </div>
                <div class="form-group">
                    <label for="personal-2fa-uri">Provisioning URI</label>
                    <textarea id="personal-2fa-uri" class="text-input" rows="3" readonly style="width: 100%; resize: vertical;"></textarea>
                </div>
                <p class="text-muted" style="margin-bottom: 12px;">
                    請將上方密鑰或 URI 加入 Google Authenticator、Microsoft Authenticator、1Password 等 TOTP 驗證器，然後輸入目前的 6 位數驗證碼完成啟用。
                </p>
                <div class="form-group">
                    <label for="personal-2fa-confirm-code">TOTP 驗證碼</label>
                    <input type="text" id="personal-2fa-confirm-code" class="text-input" placeholder="請輸入 6 位數驗證碼">
                </div>
                <div style="display: flex; gap: 10px; flex-wrap: wrap;">
                    <button class="btn btn-primary" onclick="confirmPersonalTwoFactorEnrollment()">確認啟用</button>
                    <button class="btn btn-secondary" onclick="cancelPersonalTwoFactorEnrollment()">取消</button>
                </div>
            </div>
        </div>

        <div id="personal-2fa-manage-card" style="display: none;">
            <div class="form-group">
                <label for="personal-2fa-manage-password">目前密碼</label>
                <input type="password" id="personal-2fa-manage-password" class="text-input" placeholder="執行停用或重產備援碼前需再次驗證密碼">
            </div>
            <div class="form-group">
                <label for="personal-2fa-verify-method">驗證方式</label>
                <select id="personal-2fa-verify-method" class="text-input">
                    <option value="totp">TOTP 驗證碼</option>
                    <option value="recovery_code">備援碼</option>
                </select>
            </div>
            <div class="form-group">
                <label for="personal-2fa-verify-code">驗證碼 / 備援碼</label>
                <input type="text" id="personal-2fa-verify-code" class="text-input" placeholder="請輸入目前驗證碼或一組未使用的備援碼">
            </div>
            <div style="display: flex; gap: 10px; flex-wrap: wrap;">
                <button class="btn btn-primary" onclick="regeneratePersonalRecoveryCodes()">重產備援碼</button>
                <button class="btn btn-danger" onclick="disablePersonalTwoFactor()">停用 2FA</button>
            </div>
        </div>

        <div id="personal-2fa-recovery-box" style="display: none; margin-top: 16px; padding: 16px; border: 1px solid rgba(59, 130, 246, 0.35); border-radius: 10px; background: rgba(59, 130, 246, 0.08);">
            <div style="display: flex; justify-content: space-between; align-items: center; gap: 12px; margin-bottom: 10px; flex-wrap: wrap;">
                <strong>備援碼</strong>
                <button class="btn btn-secondary" onclick="copyPersonalRecoveryCodes()">複製備援碼</button>
            </div>
            <p class="text-muted" style="margin-bottom: 10px;">
                備援碼只會顯示一次。請立即保存到離線且安全的位置。每組備援碼僅能使用一次。
            </p>
            <pre id="personal-2fa-recovery-list" style="margin: 0; white-space: pre-wrap; word-break: break-word; font-family: Consolas, monospace;"></pre>
        </div>
    `;

    const firstDivider = securityCard.querySelector('hr');
    if (firstDivider) {
        securityCard.insertBefore(section, firstDivider);
    } else {
        securityCard.appendChild(section);
    }
}

function formatTwoFactorTimestamp(value) {
    if (!value) return '尚未產生';
    const parsed = new Date(value.replace(' ', 'T'));
    if (Number.isNaN(parsed.getTime())) return value;
    return parsed.toLocaleString('zh-TW');
}

function setPersonalTwoFactorText(id, value) {
    const el = document.getElementById(id);
    if (el) el.textContent = value;
}

function syncPersonalTwoFactorUI() {
    ensurePersonalTwoFactorUI();

    const enrollCard = document.getElementById('personal-2fa-enroll-card');
    const manageCard = document.getElementById('personal-2fa-manage-card');
    const confirmPanel = document.getElementById('personal-2fa-confirm-panel');
    const noteEl = document.getElementById('personal-2fa-note');

    if (!currentTwoFactorStatus) {
        if (enrollCard) enrollCard.style.display = 'none';
        if (manageCard) manageCard.style.display = 'none';
        if (confirmPanel) confirmPanel.style.display = 'none';
        if (noteEl) noteEl.textContent = '尚未取得二階段驗證狀態。';
        return;
    }

    const enabledText = currentTwoFactorStatus.enabled ? '已啟用' : '未啟用';
    const methodText = currentTwoFactorStatus.primary_method === 'totp' ? 'TOTP 驗證器' : (currentTwoFactorStatus.primary_method || '未設定');

    setPersonalTwoFactorText('personal-2fa-enabled', enabledText);
    setPersonalTwoFactorText('personal-2fa-primary', methodText);
    setPersonalTwoFactorText('personal-2fa-recovery-remaining', String(currentTwoFactorStatus.recovery_codes_remaining ?? 0));
    setPersonalTwoFactorText('personal-2fa-recovery-generated', formatTwoFactorTimestamp(currentTwoFactorStatus.recovery_codes_generated_at));

    if (currentTwoFactorStatus.enabled) {
        if (noteEl) {
            noteEl.textContent = '目前登入帳號已啟用 TOTP。日後登入時可使用驗證器中的 6 位數驗證碼，也可改用一組未使用的備援碼。';
        }
        if (enrollCard) enrollCard.style.display = 'none';
        if (manageCard) manageCard.style.display = 'block';
        if (confirmPanel) confirmPanel.style.display = 'none';
        pendingTwoFactorEnrollment = null;
    } else {
        if (noteEl) {
            noteEl.textContent = currentTwoFactorStatus.pending_enrollment
                ? '系統已有待確認的設定。若未保存密鑰，請重新輸入目前密碼並再次點選開始設定，以產生新的 TOTP 密鑰。'
                : '尚未為目前登入帳號啟用 TOTP。啟用後可在未設定告警 Email 的情況下使用二階段驗證。';
        }
        if (enrollCard) enrollCard.style.display = 'block';
        if (manageCard) manageCard.style.display = 'none';
        if (confirmPanel) confirmPanel.style.display = pendingTwoFactorEnrollment ? 'block' : 'none';
    }
}

function renderPersonalRecoveryCodes(codes) {
    const box = document.getElementById('personal-2fa-recovery-box');
    const list = document.getElementById('personal-2fa-recovery-list');
    if (!box || !list) return;

    if (!Array.isArray(codes) || codes.length === 0) {
        list.textContent = '';
        box.style.display = 'none';
        return;
    }

    list.textContent = codes.join('\n');
    box.style.display = 'block';
}

function clearPersonalTwoFactorInputs() {
    ['personal-2fa-password', 'personal-2fa-confirm-code', 'personal-2fa-manage-password', 'personal-2fa-verify-code'].forEach((id) => {
        const el = document.getElementById(id);
        if (el) el.value = '';
    });
}

async function loadPersonalTwoFactorStatus() {
    ensurePersonalTwoFactorUI();

    try {
        const response = await apiGet('/auth/2fa/status');
        if (response.success) {
            currentTwoFactorStatus = response.data;
            syncPersonalTwoFactorUI();
        } else {
            showToast(response.error || '無法取得二階段驗證狀態', 'error');
        }
    } catch (error) {
        console.error('Failed to load 2FA status:', error);
    }
}

async function beginPersonalTwoFactorEnrollment() {
    const passwordEl = document.getElementById('personal-2fa-password');
    if (!passwordEl || !passwordEl.value.trim()) {
        showToast('請先輸入目前密碼', 'error');
        return;
    }

    try {
        const response = await apiPost('/auth/2fa/enroll', { password: passwordEl.value });
        if (!response.success) {
            showToast(response.error || '無法開始設定二階段驗證', 'error');
            return;
        }

        pendingTwoFactorEnrollment = response.data;
        const secretEl = document.getElementById('personal-2fa-secret');
        const uriEl = document.getElementById('personal-2fa-uri');
        if (secretEl) secretEl.value = response.data.secret || '';
        if (uriEl) uriEl.value = response.data.provisioning_uri || '';

        const confirmPanel = document.getElementById('personal-2fa-confirm-panel');
        if (confirmPanel) confirmPanel.style.display = 'block';

        currentTwoFactorStatus = Object.assign({}, currentTwoFactorStatus || {}, { pending_enrollment: true, enabled: false });
        syncPersonalTwoFactorUI();
        showToast('請將密鑰加入驗證器 App，並輸入目前的 TOTP 驗證碼完成啟用', 'success');
    } catch (error) {
        showToast(error.message || '無法開始設定二階段驗證', 'error');
    }
}

function cancelPersonalTwoFactorEnrollment() {
    pendingTwoFactorEnrollment = null;
    const confirmPanel = document.getElementById('personal-2fa-confirm-panel');
    if (confirmPanel) confirmPanel.style.display = 'none';
    clearPersonalTwoFactorInputs();
    renderPersonalRecoveryCodes([]);
    syncPersonalTwoFactorUI();
}

async function confirmPersonalTwoFactorEnrollment() {
    const codeEl = document.getElementById('personal-2fa-confirm-code');
    if (!codeEl || !codeEl.value.trim()) {
        showToast('請輸入 TOTP 驗證碼', 'error');
        return;
    }

    try {
        const response = await apiPost('/auth/2fa/confirm', { code: codeEl.value.trim() });
        if (!response.success) {
            showToast(response.error || '無法完成二階段驗證設定', 'error');
            return;
        }

        renderPersonalRecoveryCodes(response.data?.recovery_codes || []);
        pendingTwoFactorEnrollment = null;
        clearPersonalTwoFactorInputs();
        await loadPersonalTwoFactorStatus();
        showToast('二階段驗證已啟用，請立即保存備援碼', 'success');
    } catch (error) {
        showToast(error.message || '無法完成二階段驗證設定', 'error');
    }
}

async function disablePersonalTwoFactor() {
    const passwordEl = document.getElementById('personal-2fa-manage-password');
    const codeEl = document.getElementById('personal-2fa-verify-code');
    const methodEl = document.getElementById('personal-2fa-verify-method');

    if (!passwordEl?.value.trim() || !codeEl?.value.trim()) {
        showToast('請輸入目前密碼與驗證碼', 'error');
        return;
    }

    try {
        const response = await apiPost('/auth/2fa/disable', {
            password: passwordEl.value,
            code: codeEl.value.trim(),
            method: methodEl?.value || 'totp'
        });
        if (!response.success) {
            showToast(response.error || '無法停用二階段驗證', 'error');
            return;
        }

        renderPersonalRecoveryCodes([]);
        clearPersonalTwoFactorInputs();
        await loadPersonalTwoFactorStatus();
        showToast(response.message || '二階段驗證已停用', 'success');
    } catch (error) {
        showToast(error.message || '無法停用二階段驗證', 'error');
    }
}

async function regeneratePersonalRecoveryCodes() {
    const passwordEl = document.getElementById('personal-2fa-manage-password');
    const codeEl = document.getElementById('personal-2fa-verify-code');
    const methodEl = document.getElementById('personal-2fa-verify-method');

    if (!passwordEl?.value.trim() || !codeEl?.value.trim()) {
        showToast('請輸入目前密碼與驗證碼', 'error');
        return;
    }

    try {
        const response = await apiPost('/auth/2fa/recovery-codes/regenerate', {
            password: passwordEl.value,
            code: codeEl.value.trim(),
            method: methodEl?.value || 'totp'
        });
        if (!response.success) {
            showToast(response.error || '無法重產備援碼', 'error');
            return;
        }

        renderPersonalRecoveryCodes(response.data?.recovery_codes || []);
        clearPersonalTwoFactorInputs();
        await loadPersonalTwoFactorStatus();
        showToast('已重新產生備援碼，請立即保存', 'success');
    } catch (error) {
        showToast(error.message || '無法重產備援碼', 'error');
    }
}

async function copyPersonalRecoveryCodes() {
    const list = document.getElementById('personal-2fa-recovery-list');
    if (!list || !list.textContent.trim()) {
        showToast('目前沒有可複製的備援碼', 'error');
        return;
    }

    try {
        await navigator.clipboard.writeText(list.textContent);
        showToast('已複製備援碼', 'success');
    } catch (_) {
        showToast('無法複製備援碼，請手動保存', 'error');
    }
}

async function loadSecuritySettings() {
    if (typeof isAdmin === 'function' && !isAdmin()) return;
    try {
        ensurePersonalTwoFactorUI();
        const response = await apiGet('/security/settings');
        if (response.success) {
            currentSecuritySettings = response.data;
            const data = response.data;

            // 2FA
            const toggle = document.getElementById('global-2fa-toggle');
            const status = document.getElementById('global-2fa-status');
            if (toggle) {
                // Check if it's explicitly 'true' string or boolean true
                const isEnabled = String(data.global_2fa_enabled) === 'true';
                toggle.checked = isEnabled;
                if (status) status.textContent = isEnabled ? t('common.enabled') : t('common.disabled');
            }

            // Expiry
            const input = document.getElementById('password-expiry-days');
            if (input) {
                input.value = data.password_expiry_days || 90;
            }
        }
        await loadPersonalTwoFactorStatus();
    } catch (error) {
        console.error('Failed to load security settings:', error);
    }
}

async function toggleGlobal2FA() {
    const toggle = document.getElementById('global-2fa-toggle');
    const status = document.getElementById('global-2fa-status');
    const isEnabled = toggle.checked; // This is the new state after click

    // Optimistic UI update
    if (status) status.textContent = isEnabled ? t('common.enabled') : t('common.disabled');

    try {
        const response = await apiPut('/security/settings', {
            global_2fa_enabled: isEnabled ? 'true' : 'false'
        });

        if (response.success) {
            showToast(t('admin.toast.save_success'), 'success');
        } else {
            // Revert
            toggle.checked = !isEnabled;
            if (status) status.textContent = !isEnabled ? t('common.enabled') : t('common.disabled');
            showToast(response.error || t('admin.toast.save_failed'), 'error');
        }
    } catch (error) {
        // Revert
        toggle.checked = !isEnabled;
        if (status) status.textContent = !isEnabled ? t('common.enabled') : t('common.disabled');
        showToast(error.message || t('admin.toast.save_failed'), 'error');
    }
}

async function savePasswordExpiry() {
    const input = document.getElementById('password-expiry-days');
    if (!input) return;

    const days = input.value;

    try {
        const response = await apiPut('/security/settings', {
            password_expiry_days: days
        });

        if (response.success) {
            showToast(t('admin.toast.save_success'), 'success');
        } else {
            showToast(response.error || t('admin.toast.save_failed'), 'error');
        }
    } catch (error) {
        showToast(error.message || t('admin.toast.save_failed'), 'error');
    }
}

window.saveModuleChange = async function (key, enabled) {
    try {
        const response = await apiPut(`/system/config/${key}`, {
            value: enabled ? 'true' : 'false'
        });
        if (response.success) {
            showToast(t('admin.toast.save_success') || '設定已儲存', 'success');

            // Handle Side-effects
            if (key === 'alerts_global_enabled') {
                if (typeof updateModuleVisibility === 'function') {
                    await updateModuleVisibility();
                }
            }

            if (key === 'alerts_global_enabled') {
                // Toggle sub-options visibility
                const subOptions = document.getElementById('alert-channels-container');
                if (subOptions) {
                    subOptions.style.display = enabled ? 'block' : 'none';
                    if (enabled) {
                        // Force a reflow and trigger a resize event to fix visual glitches/unselectable elements
                        subOptions.offsetHeight;
                        window.dispatchEvent(new Event('resize'));
                    }
                }
            }
        } else {
            showToast(response.error || t('admin.toast.save_failed'), 'error');
            loadModuleConfigs(); // Revert UI
        }
    } catch (error) {
        showToast(error.message || t('admin.toast.save_failed'), 'error');
        loadModuleConfigs(); // Revert UI
    }
}

window.saveChannelVisibility = async function (channel, enabled) {
    try {
        // Here we directy update the alert_settings table's is_enabled field
        const response = await apiPut(`/alerts/settings/${channel}`, {
            is_enabled: enabled,
            config: '{}' // Keep existing config if possible, but the API expects this
        });

        // Optimization: If the API doesn't support keeping old config, we might need a GET first
        // But for "Opening" a module, resetting to '{}' is often safer than leaving it broken.
        // Let's assume the backend 'handleAlertUpdate' doesn't overwrite if config is empty? 
        // Actually, looking at alerts.go, it updates config_json. 

        if (response.success) {
            showToast(t('admin.toast.save_success') || '功能已更新', 'success');
            // Refresh alert tabs in case they are currently viewing it
            if (document.getElementById('alerts-tab').classList.contains('active')) {
                loadAlertSettings();
            }
        } else {
            showToast(response.error || t('admin.toast.save_failed'), 'error');
            loadModuleConfigs(); // Revert UI
        }
    } catch (error) {
        showToast(error.message || t('admin.toast.save_failed'), 'error');
        loadModuleConfigs(); // Revert UI
    }
}

function updateAdminTabVisibility(tabName, visible) {
    const tabBtn = document.querySelector(`.admin-tabs .tab-btn[data-tab="${tabName}"]`);
    if (tabBtn) {
        tabBtn.style.display = visible ? 'block' : 'none';

        // If the current tab became hidden, switch to the first visible tab
        if (!visible && tabBtn.classList.contains('active')) {
            const firstVisible = document.querySelector('.admin-tabs .tab-btn:not([style*="display: none"])');
            if (firstVisible) firstVisible.click();
        }
    }
}

async function loadModuleConfigs() {
    try {
        const timestamp = new Date().getTime();
        const [configRes, licenseRes] = await Promise.all([
            apiGet(`/system/config?_ts=${timestamp}`),
            apiGet(`/license/status?_ts=${timestamp}`)
        ]);

        if (configRes.success) {
            const configs = configRes.data || [];
            const alertsToggle = document.getElementById('module-alerts-toggle');
            const alertsConfig = configs.find(c => c.config_key === 'alerts_global_enabled');

            if (alertsToggle && alertsConfig) {
                const isAlertsEnabled = alertsConfig.config_value === 'true';
                alertsToggle.checked = isAlertsEnabled;
                const subOptions = document.getElementById('alert-channels-container');
                if (subOptions) {
                    subOptions.style.display = isAlertsEnabled ? 'block' : 'none';
                    if (isAlertsEnabled) {
                        subOptions.offsetHeight;
                        window.dispatchEvent(new Event('resize'));
                    }
                }

                // Initial visibility for Admin Tab
                // updateAdminTabVisibility('alerts', isAlertsEnabled);
            }

            // Load specific channels status
            loadChannelStatus();
        }
    } catch (error) {
        console.error('Failed to load module configs:', error);
    }
}

async function loadChannelStatus() {
    try {
        const response = await apiGet('/alerts/settings');
        if (response.success) {
            const settings = response.data;
            settings.forEach(s => {
                const toggle = document.getElementById(`channel-${s.alert_type}-toggle`);
                if (toggle) {
                    toggle.checked = s.is_enabled;
                }
            });
        }
    } catch (error) {
        console.error('Failed to load channel status:', error);
    }
}

// ============================================================
// 稽核日誌 (Audit Log)
// ============================================================

let auditCurrentPage = 1;
const AUDIT_PAGE_SIZE = 50;
const AUDIT_ACTION_LABELS = {
    login: '登入系統',
    logout: '登出系統',
    login_failed: '登入失敗',
    change_password: '變更密碼',
    create_device: '新增設備',
    update_device: '修改設備',
    delete_device: '刪除設備',
    bulk_delete_devices: '批次刪除設備',
    bulk_update_devices: '批次修改設備',
    create_camera: '新增攝影機',
    update_camera: '修改攝影機',
    delete_camera: '刪除攝影機',
    export_encrypted_backup: '匯出加密備份',
    restore_encrypted_backup: '還原加密備份',
    set_encryption_password: '設定備份密碼'
};
const AUDIT_DETAIL_LABELS = {
    username: '使用者',
    user_id: '使用者 ID',
    role: '角色',
    reason: '原因',
    result: '結果',
    target_ids: '目標編號',
    affected_count: '影響筆數',
    device_id: '設備 ID',
    device_name: '設備名稱',
    ip_address: 'IP 位址',
    mac_address: 'MAC 位址',
    device_type: '設備類型',
    snmp_version: 'SNMP 版本',
    old_values: '變更前',
    new_values: '變更後',
    camera_id: '攝影機 ID',
    camera_name: '攝影機名稱',
    location: '位置',
    stream_type: '預覽模式',
    monitor_display: '顯示於監控模式',
    monitor_order: '監控排序',
    recording_source: '錄影來源',
    recording_bitrate: '錄影 Bitrate',
    password_updated: '已更新密碼',
    error: '錯誤',
    image_path: '圖片路徑',
    vendor: '品牌',
    model: '型號',
    require_password_change: '強制改密碼'
};
const AUDIT_VALUE_LABELS = {
    admin: '管理者',
    editor: '編輯者',
    viewer: '檢視者',
    success: '成功',
    failed: '失敗',
    warning: '警告',
    user_not_found: '查無使用者',
    account_disabled: '帳號已停用',
    wrong_password: '密碼錯誤',
    wrong_old_password: '舊密碼錯誤',
    token_sign_failed: 'Token 簽發失敗',
    hash_failed: '密碼雜湊失敗',
    update_failed: '更新失敗',
    insert_failed: '新增失敗',
    delete_failed: '刪除失敗',
    duplicate_ip: 'IP 已存在',
    not_found: '查無資料',
    mjpeg: '快照預覽',
    webrtc: '即時預覽',
    rtsp: 'RTSP',
    onvif: 'ONVIF',
    true: '是',
    false: '否'
};

function auditT(key, fallback, params = {}) {
    const translated = t(key, params);
    return translated === key ? fallback : translated;
}

function formatAuditAction(action) {
    return auditT(`audit.actions.${action}`, AUDIT_ACTION_LABELS[action] || action);
}

function formatAuditPrimitive(value) {
    if (value === null || value === undefined || value === '') return '-';
    const normalized = String(value);
    return auditT(`audit.values.${normalized}`, AUDIT_VALUE_LABELS[normalized] || normalized);
}

function formatAuditDetailValue(value) {
    if (Array.isArray(value)) {
        return value.map(formatAuditDetailValue).join('、');
    }
    if (value && typeof value === 'object') {
        return Object.entries(value).map(([k, v]) => {
            const label = auditT(`audit.details.${k}`, AUDIT_DETAIL_LABELS[k] || k);
            return `${label}：${formatAuditDetailValue(v)}`;
        }).join('；');
    }
    return formatAuditPrimitive(value);
}

function formatAuditDetail(detail) {
    if (!detail) return '-';
    try {
        const obj = typeof detail === 'string' ? JSON.parse(detail) : detail;
        return Object.entries(obj).map(([key, value]) => {
            const label = auditT(`audit.details.${key}`, AUDIT_DETAIL_LABELS[key] || key);
            return `${label}：${formatAuditDetailValue(value)}`;
        }).join('；');
    } catch {
        return detail;
    }
}

async function loadAuditLogs(page) {
    page = page || auditCurrentPage;
    auditCurrentPage = page;

    const username = document.getElementById('audit-filter-username')?.value.trim() || '';
    const status   = document.getElementById('audit-filter-status')?.value || '';
    const dateFrom = document.getElementById('audit-filter-from')?.value || '';
    const dateTo   = document.getElementById('audit-filter-to')?.value || '';

    let url = `/audit-logs?page=${page}&limit=${AUDIT_PAGE_SIZE}`;
    if (username) url += `&username=${encodeURIComponent(username)}`;
    if (status)   url += `&status=${encodeURIComponent(status)}`;
    if (dateFrom) url += `&date_from=${dateFrom}`;
    if (dateTo)   url += `&date_to=${dateTo}`;

    const tbody = document.getElementById('audit-tbody');
    if (tbody) tbody.innerHTML = `<tr><td colspan="7" class="empty-message">${t('common.loading')}</td></tr>`;

    try {
        const res = await apiGet(url);
        if (!res || !res.success) throw new Error(res?.error || 'load failed');
        renderAuditTable(res.data || []);
        renderAuditPagination(res.total, res.page, res.limit);
    } catch (e) {
        if (tbody) tbody.innerHTML = `<tr><td colspan="7" class="empty-message">${t('audit.load_failed')}: ${e.message}</td></tr>`;
    }
}

function renderAuditTable(rows) {
    const tbody = document.getElementById('audit-tbody');
    if (!tbody) return;

    if (!rows || rows.length === 0) {
        tbody.innerHTML = `<tr><td colspan="7" class="empty-message">${t('audit.empty')}</td></tr>`;
        return;
    }

    const statusBadge = (s) => {
        const cls = s === 'success' ? 'badge-success' : s === 'failed' ? 'badge-danger' : 'badge-warning';
        const label = s === 'success' ? t('audit.status_success') : s === 'failed' ? t('audit.status_failed') : s;
        return `<span class="status-badge ${cls}">${label}</span>`;
    };

    tbody.innerHTML = rows.map(r => {
        const detail = formatAuditDetail(r.detail);
        return `<tr>
            <td style="white-space:nowrap;font-size:12px;">${r.occurred_at || ''}</td>
            <td><strong>${escapeHtml(r.username)}</strong></td>
            <td><code style="font-size:11px;">${escapeHtml(r.source_ip || '-')}</code></td>
            <td><code style="font-size:11px;">${escapeHtml(r.source_mac || '-')}</code></td>
            <td>${escapeHtml(formatAuditAction(r.action))}</td>
            <td>${statusBadge(r.status)}</td>
            <td style="font-size:11px;color:var(--text-muted);">${escapeHtml(detail)}</td>
        </tr>`;
    }).join('');
}

function renderAuditPagination(total, page, limit) {
    const container = document.getElementById('audit-pagination');
    if (!container) return;

    const totalPages = Math.ceil(total / limit);
    if (totalPages <= 1) { container.innerHTML = ''; return; }

    let html = `<div class="pagination">`;
    if (page > 1) {
        html += `<button class="btn btn-secondary btn-sm" onclick="loadAuditLogs(${page - 1})">${t('audit.prev_page')}</button>`;
    }
    html += `<span style="padding:0 12px;color:var(--text-muted);font-size:13px;">${t('audit.page_info', { page: page, total: totalPages, count: total })}</span>`;
    if (page < totalPages) {
        html += `<button class="btn btn-secondary btn-sm" onclick="loadAuditLogs(${page + 1})">${t('audit.next_page')}</button>`;
    }
    html += `</div>`;
    container.innerHTML = html;
}

function clearAuditFilters() {
    const ids = ['audit-filter-username', 'audit-filter-status', 'audit-filter-from', 'audit-filter-to'];
    ids.forEach(id => {
        const el = document.getElementById(id);
        if (el) el.value = '';
    });
    loadAuditLogs(1);
}

async function exportAuditCSV() {
    const username = document.getElementById('audit-filter-username')?.value.trim() || '';
    const status   = document.getElementById('audit-filter-status')?.value || '';
    const dateFrom = document.getElementById('audit-filter-from')?.value || '';
    const dateTo   = document.getElementById('audit-filter-to')?.value || '';

    // 取最多 5000 筆
    let url = `/audit-logs?page=1&limit=5000`;
    if (username) url += `&username=${encodeURIComponent(username)}`;
    if (status)   url += `&status=${encodeURIComponent(status)}`;
    if (dateFrom) url += `&date_from=${dateFrom}`;
    if (dateTo)   url += `&date_to=${dateTo}`;

    try {
        const res = await apiGet(url);
        if (!res || !res.success || !res.data) throw new Error('no data');

        const header = [t('audit.time'), t('audit.user'), t('audit.ip'), t('audit.mac'), t('audit.action'), t('audit.status'), t('audit.details')];
        const rows = res.data.map(r => [
            r.occurred_at, r.username, r.source_ip, r.source_mac,
            formatAuditAction(r.action), auditT(`audit.status_${r.status}`, r.status), formatAuditDetail(r.detail)
        ].map(v => `"${(v || '').toString().replace(/"/g, '""')}"`));

        const csvContent = '\uFEFF' + [header, ...rows].map(r => r.join(',')).join('\n');
        const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
        const link = document.createElement('a');
        link.href = URL.createObjectURL(blob);
        link.download = `audit_${new Date().toISOString().slice(0,10)}.csv`;
        link.click();
    } catch (e) {
        showToast(t('common.operation_failed') + ': ' + e.message, 'error');
    }
}
