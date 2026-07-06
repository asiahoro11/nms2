// Made by YTSworks
// YTS工作室製作
// Authentication Module

// Get stored auth data
function getToken() {
    return sessionStorage.getItem('nms_token');
}

function getUser() {
    const userStr = sessionStorage.getItem('nms_user');
    if (userStr) {
        try {
            return JSON.parse(userStr);
        } catch {
            return null;
        }
    }
    return null;
}

function getUserRole() {
    const user = getUser();
    return user ? user.role : null;
}

// Check if user is authenticated
function isAuthenticated() {
    const token = getToken();
    const expires = sessionStorage.getItem('nms_expires');

    if (!token) return false;

    // Check if token is expired
    if (expires) {
        const expiresAt = parseInt(expires) * 1000; // Convert to milliseconds
        if (Date.now() > expiresAt) {
            logout();
            return false;
        }
    }

    return true;
}

// Check auth and redirect if not authenticated
function requireAuth() {
    if (!isAuthenticated()) {
        navigateToFrontendRoute('/login');
        return false;
    }
    return true;
}

// Logout
function logout() {
    if (typeof stopNotifications === 'function') stopNotifications();
    sessionStorage.removeItem('nms_token');
    sessionStorage.removeItem('nms_user');
    sessionStorage.removeItem('nms_expires');
    sessionStorage.removeItem('nms_license_locked');
    sessionStorage.removeItem('nms_license_lock_reason');
    navigateToFrontendRoute('/login');
}

// Permission checks
function isAdmin() {
    return getUserRole() === 'admin';
}

function isEditor() {
    const role = getUserRole();
    return role === 'admin' || role === 'editor';
}

function isViewer() {
    return getUserRole() === 'viewer';
}

// Check if user can access a feature
function canAccess(feature) {
    const role = getUserRole();

    const permissions = {
        // All roles can access
        'dashboard': ['admin', 'editor', 'viewer'],
        'devices_view': ['admin', 'editor', 'viewer'],
        'topology_view': ['admin', 'editor', 'viewer'],
        'license_view': ['admin', 'editor', 'viewer'],

        // Admin and Editor can access
        'devices_edit': ['admin', 'editor'],
        'topology_edit': ['admin', 'editor'],
        'logs': ['admin', 'editor'],
        'reports': ['admin', 'editor'],

        // Admin only
        'users': ['admin'],
        'license_manage': ['admin'],
    };

    const allowedRoles = permissions[feature];
    if (!allowedRoles) return false;

    return allowedRoles.includes(role);
}

// Apply role-based UI visibility
function applyRoleBasedUI() {
    const role = getUserRole();
    const user = getUser();

    // Update user info display
    const usernameEls = document.querySelectorAll('.current-username');
    usernameEls.forEach(el => {
        if (user) el.textContent = user.username;
    });

    const roleEls = document.querySelectorAll('.current-role');
    roleEls.forEach(el => {
        if (role) {
            // Wait for translations to load before applying
            const applyRoleText = () => {
                const roleLabels = {
                    admin: t('admin.users.role_admin') || '管理者',
                    editor: t('admin.users.role_editor') || '編輯者',
                    viewer: t('admin.users.role_viewer') || '檢視者'
                };
                const normalizedRole = role.toLowerCase();
                el.textContent = roleLabels[normalizedRole] || role;
            };

            // Apply immediately if translations are ready
            if (typeof t === 'function') {
                applyRoleText();
            }

            // Also listen for language changes
            window.addEventListener('languageChanged', applyRoleText);
        }
    });

    // Hide user management tab for non-admin
    const usersTab = document.querySelector('[data-tab="users"]');
    if (usersTab && !isAdmin()) {
        usersTab.style.display = 'none';
    }

    // Hide reports tab for viewers
    const reportsTab = document.querySelector('[data-tab="reports"]');
    if (reportsTab && isViewer()) {
        reportsTab.style.display = 'none';
    }

    // Show logout button
    const logoutBtn = document.getElementById('logout-btn');
    if (logoutBtn) {
        logoutBtn.style.display = 'block';
    }
}

// Apply navigation permissions based on data-permission attributes
function applyNavigationPermissions() {
    const user = getUser();
    const role = user?.role || 'viewer';

    // Nav items (sidebar + bottom nav)
    document.querySelectorAll('.nav-item[data-permission], .bottom-nav-item[data-permission]').forEach(item => {
        const allowedRoles = item.dataset.permission.split(',').map(r => r.trim());
        if (!allowedRoles.includes('all') && !allowedRoles.includes(role)) {
            item.style.display = 'none';
        } else {
            item.style.display = '';
        }
    });

    // Action buttons (not nav items) with data-permission — hide for unauthorized roles
    // Use visibility:hidden + width:0 so layout doesn't shift for inline elements
    document.querySelectorAll('button[data-permission], a[data-permission]').forEach(item => {
        const allowedRoles = item.dataset.permission.split(',').map(r => r.trim());
        if (!allowedRoles.includes('all') && !allowedRoles.includes(role)) {
            item.style.display = 'none';
        }
        // Note: do NOT restore to '' here — these buttons may be managed by other logic (e.g. bulk select)
    });
}

// Show change password modal
function showChangePasswordModal(forced = false) {
    console.log(`[Auth] showChangePasswordModal called, forced: ${forced}`);

    const content = `
        <form id="change-password-form" onsubmit="changePassword(event, ${forced})">
            <div class="form-group">
                <label data-i18n="user.password_old">${t('user.password_old') || '目前密碼'}</label>
                <input type="password" name="old_password" required placeholder="${t('user.password_old_placeholder') || '請輸入目前密碼'}" data-i18n="[placeholder]user.password_old_placeholder">
            </div>
            <div class="form-group">
                <label data-i18n="user.password_new">${t('user.password_new') || '新密碼'}</label>
                <input type="password" name="new_password" required minlength="8" placeholder="${t('user.password_new_placeholder') || '請輸入新密碼'}" data-i18n="[placeholder]user.password_new_placeholder">
                <small style="color: var(--text-secondary, #64748b); display: block; margin-top: 4px; font-size: 0.8rem;">
                    * 至少 8 碼，需含大寫、小寫字母，以及數字或特殊字元
                </small>
            </div>
            <div class="form-group">
                <label data-i18n="user.password_confirm">${t('user.password_confirm') || '確認新密碼'}</label>
                <input type="password" name="confirm_password" required minlength="8" placeholder="${t('user.password_confirm_placeholder') || '請再次輸入新密碼'}" data-i18n="[placeholder]user.password_confirm_placeholder">
            </div>
            <div class="form-actions">
                ${forced ? '' : `<button type="button" class="btn btn-secondary" onclick="hideModal()" data-i18n="common.cancel">${t('common.cancel') || '取消'}</button>`}
                <button type="submit" class="btn btn-primary" data-i18n="user.change_password">${t('user.change_password') || '變更密碼'}</button>
            </div>
        </form>
    `;

    console.log(`[Auth] Opening modal...`);
    openModal(
        forced ? t('user.password_force_change') || '🔒 請設定新密碼' : t('user.change_password_title') || '變更密碼',
        content
    );

    console.log('[Auth] Modal opened, applying translations...');
    if (typeof applyTranslations === 'function') {
        applyTranslations();
    }
}

async function changePassword(event, forced) {
    event.preventDefault();
    const form = event.target;
    const formData = new FormData(form);

    const oldPassword = formData.get('old_password');
    const newPassword = formData.get('new_password');
    const confirmPassword = formData.get('confirm_password');

    if (newPassword !== confirmPassword) {
        showToast(t('user.password_mismatch') || '新密碼與確認密碼不符', 'error');
        return;
    }

    // Client-side password strength check
    // Rule: min 8 chars, uppercase + lowercase + (digit OR special char)
    const hasUpper = /[A-Z]/.test(newPassword);
    const hasLower = /[a-z]/.test(newPassword);
    const hasDigit = /[0-9]/.test(newPassword);
    const hasSpecial = /[^A-Za-z0-9]/.test(newPassword);
    if (newPassword.length < 8) {
        showToast('密碼長度至少需要 8 個字元', 'error');
        return;
    }
    if (!hasUpper) {
        showToast('密碼必須包含至少 1 個大寫字母', 'error');
        return;
    }
    if (!hasLower) {
        showToast('密碼必須包含至少 1 個小寫字母', 'error');
        return;
    }
    if (!hasDigit && !hasSpecial) {
        showToast('密碼必須包含至少 1 個數字或特殊字元', 'error');
        return;
    }

    try {
        const response = await apiPost('/auth/change-password', {
            old_password: oldPassword,
            new_password: newPassword
        });

        if (response.success) {
            showToast(t('user.password_success') || '密碼已更新，系統將重新整理', 'success');
            if (forced) {
                sessionStorage.removeItem('nms_require_pwd_change');
                // Unlock forced modal before reload
                if (typeof unlockForcedModal === 'function') {
                    unlockForcedModal();
                }
            }

            // 強制 hard reload (清除 JS/CSS 快取，等同 Ctrl+F5)
            setTimeout(() => {
                window.location.replace('/index.html?_cb=' + Date.now());
            }, 1000);
        }
    } catch (error) {
        showToast(error.message || t('user.password_error') || '密碼更新失敗', 'error');
    }
}

// Initialize auth on page load
function initAuth() {
    if (!requireAuth()) return false;
    applyRoleBasedUI();
    applyNavigationPermissions();

    // Check for forced password change
    if (sessionStorage.getItem('nms_require_pwd_change') === 'true') {
        // Use requestAnimationFrame to ensure DOM is fully rendered before opening modal
        requestAnimationFrame(() => {
            requestAnimationFrame(() => {
                console.log('[Auth] Opening password change modal...');
                showChangePasswordModal(true);
            });
        });
    }

    return true;
}
