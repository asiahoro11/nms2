// Version: 2.0.1 -

// Returns true if the device is an EdgeCore switch (backup / reboot / PoE supported)
function isEdgecoreDevice(device) {
    const vendors = ['edgecore', 'edge-core', 'ecs'];
    const v = (device.vendor || device.manufacturer || '').toLowerCase();
    return vendors.some(k => v.includes(k));
}

async function loadDevices(silent = false) {
    // Ensure globally available
    if (typeof selectedDeviceIds === 'undefined') {
        window.selectedDeviceIds = new Set();
    } else if (!silent) {
        // Only clear selection if NOT a silent refresh (auto-refresh)
        // Manual reload (page change/filter) should clear it.
        selectedDeviceIds.clear();
    }

    const { page, limit, search, type, status } = state.devices;

    let url = `/devices?page=${page}&limit=${limit}`;
    if (search) url += `&search=${encodeURIComponent(search)}`;
    if (type) url += `&type=${type}`;
    if (status) url += `&status=${status}`;

    try {
        const response = await apiGet(url);
        if (response.success) {

            // If NOT silent, we already cleared above.
            // If silent, we keep 'selectedDeviceIds' as is.

            if (!silent) {
                updateBulkDeleteButtonState();
                // Update select all checkbox
                const selectAll = document.getElementById('select-all-devices');
                if (selectAll) selectAll.checked = false;
            }

            renderDevices(response.data);
            renderPagination('devices-pagination', response.total, response.page, response.limit, loadDevicesPage);
        }
    } catch (error) {
        console.error('Failed to load devices:', error);
        if (!silent) showToast(t('devices.toast.load_failed'), 'error');
    }
}

function loadDevicesPage(page) {
    state.devices.page = page;
    loadDevices();
}

let _lastDevices = [];  // cache for subnet group re-render

function renderDevices(devices) {
    _lastDevices = devices || [];
    if (_subnetGroupActive) {
        renderDevicesBySubnet(_lastDevices);
        return;
    }
    const tbody = document.getElementById('devices-tbody');

    if (!devices || devices.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="9" class="empty-message">${t('devices.no_data') || '尚無設備資料'}</td>
            </tr>
        `;
        return;
    }

    let html = '';
    for (const device of devices) {
        const statusClass = device.is_online ? 'online' : 'offline';
        const statusText = device.is_online ? t('devices.online') : t('devices.offline');
        const isSnmp = (device.snmp_community || device.snmp_version === 3);
        const monitorType = device.snmp_version === 3 ? 'SNMP v3' : (device.snmp_community ? 'SNMP' : 'Ping');
        const monitorClass = isSnmp ? 'snmp' : 'ping';
        const imageHtml = getDeviceIcon(device);

        const isChecked = selectedDeviceIds.has(device.id) ? 'checked' : '';
        const vendor = device.vendor || device.manufacturer || '-';

        html += `
            <tr>
                <td>
                    <input type="checkbox" class="device-checkbox" value="${device.id}" onchange="toggleDeviceSelection(${device.id})" ${isChecked}>
                </td>
                <td>
                    <div class="status-indicator">
                        <span class="dot ${statusClass}"></span>
                        <span>${statusText}</span>
                    </div>
                </td>
                <td>${imageHtml}</td>
                <td>
                    <div class="device-name-container">
                        <strong>${escapeHtml(getDeviceDisplayName(device))}</strong>
                        ${(device.is_name_custom && device.sys_name && device.sys_name !== device.name) ? ` <span class="sysname-hint" title="系統原始名稱: ${escapeHtml(device.sys_name)}">🏷️</span>` : ''}
                        <br>
                        <small style="color: var(--text-muted);">${device.mac_address ? `MAC: ${escapeHtml(device.mac_address)}` : ''}</small>
                    </div>
                </td>
                <td>${escapeHtml(vendor)}</td>
                <td>${device.snmp_community ? `<code style="cursor: pointer; color: var(--primary-color); text-decoration: underline;" onclick="window.open('//${device.ip_address}', '_blank')" title="${t('dashboard.top5.connect_hint')}">${device.ip_address}</code>` : `<code>${device.ip_address}</code>`}</td>
                <td>
                    <span class="device-type-badge ${device.device_type}">${getDeviceTypeIcon(device.device_type)} ${device.device_type}</span>
                    <span class="monitor-badge ${monitorClass}">${monitorType}</span>
                </td>
                <td>${formatRelativeTime(device.last_seen)}</td>
                <td>
                    <div class="action-buttons">
                        <button class="action-btn" onclick="viewDevice(${device.id})" title="${t('common.view')}">👁️</button>
                        ${!isViewer() ? `<button class="action-btn" onclick="editDevice(${device.id})" title="${t('common.edit')}">✏️</button>` : ''}
                        ${!isViewer() ? `<button class="action-btn ${isSnmp ? 'advanced-only' : ''}" onclick="uploadDeviceImage(${device.id})" title="${t('common.upload')}">📷</button>` : ''}
                        ${!isViewer() ? `<button class="action-btn danger" onclick="confirmDeleteDevice(${device.id}, '${escapeHtml(getDeviceDisplayName(device))}')" title="${t('common.delete')}">🗑️</button>` : ''}
                    </div>
                </td>
            </tr>
        `;
    }

    tbody.innerHTML = html;
    updateBulkDeleteButtonState();
}

function renderPagination(containerId, total, currentPage, limit, callback) {
    const container = document.getElementById(containerId);
    const totalPages = Math.ceil(total / limit);

    if (totalPages <= 1) {
        container.innerHTML = '';
        return;
    }

    let html = '';
    html += `<button ${currentPage === 1 ? 'disabled' : ''} onclick="${callback.name}(${currentPage - 1})">${t('common.prev_page') || '上一頁'}</button>`;

    const maxVisiblePages = 5;
    let startPage = Math.max(1, currentPage - Math.floor(maxVisiblePages / 2));
    let endPage = Math.min(totalPages, startPage + maxVisiblePages - 1);

    if (endPage - startPage < maxVisiblePages - 1) {
        startPage = Math.max(1, endPage - maxVisiblePages + 1);
    }

    if (startPage > 1) {
        html += `<button onclick="${callback.name}(1)">1</button>`;
        if (startPage > 2) html += `<span>...</span>`;
    }

    for (let i = startPage; i <= endPage; i++) {
        html += `<button class="${i === currentPage ? 'active' : ''}" onclick="${callback.name}(${i})">${i}</button>`;
    }

    if (endPage < totalPages) {
        if (endPage < totalPages - 1) html += `<span>...</span>`;
        html += `<button onclick="${callback.name}(${totalPages})">${totalPages}</button>`;
    }

    html += `<button ${currentPage === totalPages ? 'disabled' : ''} onclick="${callback.name}(${currentPage + 1})">${t('common.next_page') || '下一頁'}</button>`;
    container.innerHTML = html;
}

// ===== 單一設備新增 =====
function showAddDeviceModal() {
    const content = `
        <form id="add-device-form" onsubmit="createDevice(event)">
            <div class="form-group">
                <label data-i18n="devices.modal.name_label">設備名稱 *</label>
                <input type="text" name="name" required placeholder="例如: Core-Router-01" data-i18n="[placeholder]devices.modal.name_placeholder">
            </div>
            <div class="form-group">
                <label data-i18n="devices.modal.ip_label">IP 位址 *</label>
                <input type="text" name="ip_address" required placeholder="例如: 192.168.1.1" data-i18n="[placeholder]devices.modal.ip_placeholder">
            </div>
            <div class="form-group">
                <label data-i18n="devices.modal.device_type">設備類型</label>
                <select name="device_type">
                    <option value="other" data-i18n="devices.type_other" data-i18n="devices.type_other">其他</option>
                    <option value="router" data-i18n="devices.type_router" data-i18n="devices.type_router">路由器</option>
                    <option value="switch" data-i18n="devices.type_switch" data-i18n="devices.type_switch">交換器</option>
                    <option value="access_point" data-i18n="devices.type_ap" data-i18n="devices.type_ap">無線存取點 (AP)</option>
                    <option value="firewall" data-i18n="devices.type_firewall" data-i18n="devices.type_firewall">防火牆</option>
                    <option value="server" data-i18n="devices.type_server" data-i18n="devices.type_server">伺服器</option>
                    <option value="codec" data-i18n="devices.type_codec" data-i18n="devices.type_codec">編解碼器 (Codec)</option>
                    <option value="ipcam" data-i18n="devices.type_ipcam" data-i18n="devices.type_ipcam">攝影機 (IPCAM)</option>
                    <option value="video_wall" data-i18n="devices.type_videowall" data-i18n="devices.type_video_wall">電視牆控制器</option>
                    <option value="access_control" data-i18n="devices.type_access_control">門禁系統</option>
                    <option value="ups" data-i18n="devices.type_ups">UPS 不斷電系統</option>
                    <option value="pdu" data-i18n="devices.type_pdu">PDU 電源分配器</option>
                </select>
            </div>
            <div class="form-group">
                <label data-i18n="devices.modal.monitor_method">監控方式</label>
                <select name="monitor_type" onchange="toggleSnmpFields(this.value)">
                    <option value="ping" selected>Ping Only</option>
                    <option value="snmp">SNMP</option>
                </select>
            </div>
            <div id="snmp-fields" style="display:none;">
                <div class="form-group">
                    <label data-i18n="devices.modal.snmp_version">SNMP 版本</label>
                    <select name="snmp_version" onchange="toggleSnmpVersionFields(this.value)">
                        <option value="2">v2c</option>
                        <option value="1">v1</option>
                        <option value="3">v3 (更安全)</option>
                    </select>
                </div>
                
                <div id="snmpv1v2-fields">
                    <div class="form-group">
                        <label data-i18n="devices.modal.snmp_read">SNMP Read Community (讀取)</label>
                        <input type="text" name="snmp_community" placeholder="預設: public" data-i18n="[placeholder]devices.modal.snmp_read_placeholder">
                    </div>
                    <div class="form-group">
                        <label data-i18n="devices.modal.snmp_write">SNMP Write Community (寫入 - 用於控制)</label>
                        <input type="text" name="snmp_rw_community" placeholder="預設與讀取相同" data-i18n="[placeholder]devices.modal.snmp_write_placeholder">
                    </div>
                </div>

                <div id="snmpv3-fields" style="display:none; border:1px solid var(--border-color); padding:10px; border-radius:8px; margin-top:10px; background: rgba(255,255,255,0.02);">
                    <div class="form-group">
                        <label data-i18n="devices.modal.snmpv3_user">Security Name (安全名稱)</label>
                        <input type="text" name="snmpv3_security_name" placeholder="例如: snmpv3user">
                    </div>
                    <div class="form-group">
                        <label data-i18n="devices.modal.snmpv3_level">Security Level (安全層級)</label>
                        <select name="snmpv3_security_level" onchange="toggleSnmpV3SecurityFields(this.value)">
                            <option value="noAuthNoPriv">noAuthNoPriv (無認證無加密)</option>
                            <option value="authNoPriv">authNoPriv (有認證無加密)</option>
                            <option value="authPriv">authPriv (有認證有加密)</option>
                        </select>
                    </div>
                    <div id="snmpv3-auth-fields" style="display:none;">
                        <div style="display:grid; grid-template-columns: 1fr 2fr; gap:10px;">
                            <div class="form-group">
                                <label data-i18n="devices.modal.snmpv3_auth_p">Auth Protocol</label>
                                <select name="snmpv3_auth_protocol">
                                    <option value="MD5">MD5</option>
                                    <option value="SHA">SHA</option>
                                </select>
                            </div>
                            <div class="form-group">
                                <label data-i18n="devices.modal.snmpv3_auth_pw">Auth Password</label>
                                <input type="password" name="snmpv3_auth_password">
                            </div>
                        </div>
                    </div>
                    <div id="snmpv3-priv-fields" style="display:none;">
                        <div style="display:grid; grid-template-columns: 1fr 2fr; gap:10px;">
                            <div class="form-group">
                                <label data-i18n="devices.modal.snmpv3_priv_p">Priv Protocol</label>
                                <select name="snmpv3_priv_protocol">
                                    <option value="DES">DES</option>
                                    <option value="AES">AES</option>
                                </select>
                            </div>
                            <div class="form-group">
                                <label data-i18n="devices.modal.snmpv3_priv_pw">Priv Password</label>
                                <input type="password" name="snmpv3_priv_password">
                            </div>
                        </div>
                    </div>
                    <div class="form-group">
                        <label data-i18n="devices.modal.context_name">Context Name (可選)</label>
                        <input type="text" name="snmpv3_context_name" data-i18n="[placeholder]devices.modal.snmpv3_context_placeholder">
                    </div>
                </div>

                <div class="form-group" style="margin-top:20px; border-top:1px solid #333; padding-top:10px;">
                    <label data-i18n="devices.modal.admin_account">管理帳號 (CLI/Web)</label>
                    <input type="text" name="cli_username" placeholder="例如: admin" data-i18n="[placeholder]admin.users.username_placeholder">
                </div>
                <div class="form-group">
                    <label data-i18n="devices.modal.admin_password">管理密碼</label>
                    <input type="password" name="cli_password" placeholder="" data-i18n="[placeholder]admin.users.new_password">
                </div>
            </div>
            <div class="form-actions">
                <button type="button" class="btn btn-secondary" onclick="hideModal()" data-i18n="devices.modal.cancel_btn">取消</button>
                <button type="submit" class="btn btn-primary" data-i18n="devices.modal.add_btn">新增</button>
            </div>
        </form>
    `;
    openModal('modals.add_device', content);
    applyTranslations();
    // Force ping selected after applyTranslations (i18n may reset select state)
    setTimeout(() => {
        const monSel = document.querySelector('#add-device-form select[name="monitor_type"]');
        if (monSel) { monSel.value = 'ping'; toggleSnmpFields('ping'); }
    }, 0);
}

function toggleSnmpFields(value) {
    const snmpFields = document.getElementById('snmp-fields');
    snmpFields.style.display = value === 'snmp' ? 'block' : 'none';
    if (value === 'snmp') {
        toggleSnmpVersionFields(document.querySelector('select[name="snmp_version"]').value);
    }
}

function toggleSnmpVersionFields(version) {
    const v1v2 = document.getElementById('snmpv1v2-fields');
    const v3 = document.getElementById('snmpv3-fields');
    if (v1v2) v1v2.style.display = version === '3' ? 'none' : 'block';
    if (v3) v3.style.display = version === '3' ? 'block' : 'none';

    if (version === '3') {
        toggleSnmpV3SecurityFields(document.querySelector('select[name="snmpv3_security_level"]').value);
    }
}

function toggleSnmpV3SecurityFields(level) {
    const authFields = document.getElementById('snmpv3-auth-fields');
    const privFields = document.getElementById('snmpv3-priv-fields');
    if (authFields) authFields.style.display = (level === 'authNoPriv' || level === 'authPriv') ? 'block' : 'none';
    if (privFields) privFields.style.display = level === 'authPriv' ? 'block' : 'none';
}

async function createDevice(event) {
    event.preventDefault();
    const form = event.target;
    const formData = new FormData(form);

    const monitorType = formData.get('monitor_type');
    const device = {
        name: formData.get('name'),
        ip_address: formData.get('ip_address'),
        device_type: formData.get('device_type'),
        monitor_type: monitorType,
        snmp_community: monitorType === 'snmp' ? (formData.get('snmp_community') || '') : '',
        snmp_rw_community: monitorType === 'snmp' ? formData.get('snmp_rw_community') : '',
        snmp_version: monitorType === 'snmp' ? parseInt(formData.get('snmp_version')) : 0,
        cli_username: formData.get('cli_username'),
        cli_password: formData.get('cli_password'),
        snmpv3_security_name: formData.get('snmpv3_security_name'),
        snmpv3_security_level: formData.get('snmpv3_security_level'),
        snmpv3_auth_protocol: formData.get('snmpv3_auth_protocol'),
        snmpv3_auth_password: formData.get('snmpv3_auth_password'),
        snmpv3_priv_protocol: formData.get('snmpv3_priv_protocol'),
        snmpv3_priv_password: formData.get('snmpv3_priv_password'),
        snmpv3_context_name: formData.get('snmpv3_context_name')
    };

    try {
        const response = await apiPost('/devices', device);
        if (response.success) {
            showToast(t('devices.modal.success_add'), 'success');
            hideModal();
            loadDevices();
        }
    } catch (error) {
        showToast(error.message || t('devices.modal.error_add'), 'error');
    }
}

// ===== 批量新增設備 =====
function showBulkAddModal() {
    const content = `
        <div class="bulk-add-tabs">
            <button class="bulk-tab-btn active" onclick="switchBulkTab('range')" data-i18n="devices.modal.tab_range">IP 範圍</button>
            <button class="bulk-tab-btn" onclick="switchBulkTab('subnet')" data-i18n="devices.modal.tab_subnet">子網段掃描</button>
        </div>
        
        <form id="bulk-add-form" onsubmit="executeBulkAdd(event)">
            <!-- IP Range Tab -->
            <div id="bulk-tab-range" class="bulk-tab-content active">
                <div class="form-group">
                    <label data-i18n="devices.modal.range_start">起始 IP *</label>
                    <input type="text" name="start_ip" data-i18n="[placeholder]devices.modal.ip_placeholder">
                </div>
                <div class="form-group">
                    <label data-i18n="devices.modal.range_end">結束 IP *</label>
                    <input type="text" name="end_ip" data-i18n="[placeholder]devices.modal.range_end" placeholder="例如: 192.168.1.254">
                </div>
            </div>
            
            <!-- Subnet Tab -->
            <div id="bulk-tab-subnet" class="bulk-tab-content" style="display:none;">
                <div class="form-group">
                    <label data-i18n="devices.modal.subnet_label">子網段 (CIDR) *</label>
                    <input type="text" name="subnet" data-i18n="[placeholder]devices.modal.subnet_placeholder" placeholder="例如: 192.168.1.0/24">
                    <small class="form-text text-muted" style="display:block; margin-top:4px; color:#aaa;" data-i18n="devices.modal.subnet_hint">支援多網段，請以逗號分隔</small>
                </div>
            </div>
            
            <!-- Common Fields -->
            <div class="form-group">
                <label data-i18n="devices.modal.name_prefix">名稱前綴</label>
                <input type="text" name="name_prefix" placeholder="Device-" value="Device-" data-i18n="[placeholder]devices.modal.name_prefix_placeholder">
            </div>
            <div class="form-group">
                <label data-i18n="devices.modal.device_type">預設設備類型</label>
                <select name="device_type">
                    <option value="other" data-i18n="devices.type_other" data-i18n="devices.type_other">其他</option>
                    <option value="router" data-i18n="devices.type_router" data-i18n="devices.type_router">路由器</option>
                    <option value="switch" data-i18n="devices.type_switch" data-i18n="devices.type_switch">交換器</option>
                    <option value="access_point" data-i18n="devices.type_ap" data-i18n="devices.type_ap">無線存取點 (AP)</option>
                    <option value="firewall" data-i18n="devices.type_firewall" data-i18n="devices.type_firewall">防火牆</option>
                    <option value="server" data-i18n="devices.type_server" data-i18n="devices.type_server">伺服器</option>
                    <option value="codec" data-i18n="devices.type_codec" data-i18n="devices.type_codec">編解碼器 (Codec)</option>
                    <option value="ipcam" data-i18n="devices.type_ipcam" data-i18n="devices.type_ipcam">攝影機 (IPCAM)</option>
                    <option value="video_wall" data-i18n="devices.type_videowall" data-i18n="devices.type_video_wall">電視牆控制器</option>
                    <option value="access_control" data-i18n="devices.type_access_control">門禁系統</option>
                    <option value="ups" data-i18n="devices.type_ups">UPS 不斷電系統</option>
                    <option value="pdu" data-i18n="devices.type_pdu">PDU 電源分配器</option>
                </select>
            </div>
            <div class="form-group">
                <label data-i18n="devices.modal.monitor_method">監控方式</label>
                <select name="monitor_type" onchange="toggleBulkSnmpFields(this.value)">
                    <option value="ping">Ping Only</option>
                    <option value="snmp">SNMP</option>
                </select>
                <label class="checkbox-container" id="include-ping-checkbox" style="margin-top: 8px; display: none;">
                    <input type="checkbox" name="include_ping_only">
                    <span class="checkbox-label" data-i18n="devices.modal.include_ping">勾選: 同時偵測 SNMP+PING 設備 / 不勾選: 只偵測 SNMP 設備</span>
                </label>
            </div>
            
            <div id="bulk-snmp-fields" style="display:none;">
                <div class="form-group">
                    <label data-i18n="devices.modal.snmp_version">SNMP 版本</label>
                    <select name="snmp_version" onchange="toggleBulkSnmpVersionFields(this.value)">
                        <option value="2">v2c</option>
                        <option value="1">v1</option>
                        <option value="3" data-i18n="devices.modal.snmp_v3_premium">v3 (更安全)</option>
                    </select>
                </div>

                <div id="bulk-snmpv1v2-fields">
                    <div class="form-group">
                        <label data-i18n="devices.modal.snmp_read">SNMP Read Community (讀取)</label>
                        <input type="text" name="snmp_community" placeholder="public" value="public">
                    </div>
                </div>

                <div id="bulk-snmpv3-fields" style="display:none; border:1px solid var(--border-color); padding:10px; border-radius:8px; margin-top:10px; background: rgba(255,255,255,0.02);">
                    <div class="form-group">
                        <label data-i18n="devices.modal.snmpv3_user">Security Name (安全名稱)</label>
                        <input type="text" name="snmpv3_security_name" placeholder="例如: snmpv3user">
                    </div>
                    <div class="form-group">
                        <label data-i18n="devices.modal.snmpv3_level">Security Level (安全層級)</label>
                        <select name="snmpv3_security_level" onchange="toggleBulkSnmpV3SecurityFields(this.value)">
                            <option value="noAuthNoPriv">noAuthNoPriv</option>
                            <option value="authNoPriv">authNoPriv</option>
                            <option value="authPriv">authPriv</option>
                        </select>
                    </div>
                    <div id="bulk-snmpv3-auth-fields" style="display:none;">
                        <div style="display:grid; grid-template-columns: 1fr 2fr; gap:10px;">
                            <div class="form-group">
                                <label data-i18n="devices.modal.snmpv3_auth_p">Auth Protocol</label>
                                <select name="snmpv3_auth_protocol">
                                    <option value="MD5">MD5</option>
                                    <option value="SHA">SHA</option>
                                </select>
                            </div>
                            <div class="form-group">
                                <label data-i18n="devices.modal.snmpv3_auth_pw">Auth Password</label>
                                <input type="password" name="snmpv3_auth_password">
                            </div>
                        </div>
                    </div>
                    <div id="bulk-snmpv3-priv-fields" style="display:none;">
                        <div style="display:grid; grid-template-columns: 1fr 2fr; gap:10px;">
                            <div class="form-group">
                                <label data-i18n="devices.modal.snmpv3_priv_p">Priv Protocol</label>
                                <select name="snmpv3_priv_protocol">
                                    <option value="DES">DES</option>
                                    <option value="AES">AES</option>
                                </select>
                            </div>
                            <div class="form-group">
                                <label data-i18n="devices.modal.snmpv3_priv_pw">Priv Password</label>
                                <input type="password" name="snmpv3_priv_password">
                            </div>
                        </div>
                    </div>
                    <div class="form-group">
                        <label data-i18n="devices.modal.snmpv3_context">Context Name (可選)</label>
                        <input type="text" name="snmpv3_context_name" data-i18n="[placeholder]devices.modal.snmpv3_context_placeholder">
                    </div>
                </div>
            </div>
            
            <input type="hidden" name="scan_type" value="range">
            
            <div class="form-actions">
                <button type="button" class="btn btn-secondary" onclick="hideModal()" data-i18n="devices.modal.cancel_btn">取消</button>
                <button type="submit" class="btn btn-primary" data-i18n="devices.modal.scan_btn">🔍 開始掃描</button>
            </div>
        </form>
        
        <div id="bulk-scan-progress" style="display:none;">
            <div class="scan-progress-bar">
                <div class="scan-progress-fill" id="scan-progress-fill"></div>
            </div>
            <p id="scan-status-text" data-i18n="devices.modal.scanning">正在掃描...</p>
        </div>
    `;
    openModal('modals.bulk_add', content);
    applyTranslations();

    // 檢查授權資訊，決定是否顯示「同時偵測 Ping」選項
    checkLicenseAndTogglePingOption();
}

async function checkLicenseAndTogglePingOption() {
    try {
        const response = await apiGet('/license/status');

        if (response.success && response.data) {
            const maxDevices = response.data.max_devices || 10;

            // 只有授權超過 10 台才顯示「同時偵測 Ping」選項
            const includePingCheckbox = document.getElementById('include-ping-checkbox');
            if (includePingCheckbox) {
                // 授權 > 10 台時，在 SNMP 模式下顯示選項
                includePingCheckbox.dataset.maxDevices = maxDevices;
                console.log('[License Check] Max devices:', maxDevices, 'Show ping option:', maxDevices > 10);
            }
        }
    } catch (error) {
        console.error('Failed to check license:', error);
    }
}

function switchBulkTab(tab) {
    document.querySelectorAll('.bulk-tab-btn').forEach(b => b.classList.remove('active'));
    document.querySelectorAll('.bulk-tab-content').forEach(c => c.style.display = 'none');

    event.target.classList.add('active');
    document.getElementById(`bulk-tab-${tab}`).style.display = 'block';
    document.querySelector('input[name="scan_type"]').value = tab;
}

function toggleBulkSnmpFields(value) {
    const snmpFields = document.getElementById('bulk-snmp-fields');
    const pingCheckbox = document.getElementById('include-ping-checkbox');

    snmpFields.style.display = value === 'snmp' ? 'block' : 'none';

    // 只在 SNMP 模式且授權 > 10 台時顯示「同時偵測 Ping」選項
    if (value === 'snmp' && pingCheckbox) {
        const maxDevices = parseInt(pingCheckbox.dataset.maxDevices) || 10;
        pingCheckbox.style.display = maxDevices > 10 ? 'block' : 'none';
    } else if (pingCheckbox) {
        pingCheckbox.style.display = 'none';
    }

    if (value === 'snmp') {
        toggleBulkSnmpVersionFields(document.querySelector('#bulk-add-form select[name="snmp_version"]').value);
    }
}

function toggleBulkSnmpVersionFields(version) {
    const v1v2 = document.getElementById('bulk-snmpv1v2-fields');
    const v3 = document.getElementById('bulk-snmpv3-fields');
    if (v1v2) v1v2.style.display = version === '3' ? 'none' : 'block';
    if (v3) v3.style.display = version === '3' ? 'block' : 'none';

    if (version === '3') {
        toggleBulkSnmpV3SecurityFields(document.querySelector('#bulk-add-form select[name="snmpv3_security_level"]').value);
    }
}

function toggleBulkSnmpV3SecurityFields(level) {
    const authFields = document.getElementById('bulk-snmpv3-auth-fields');
    const privFields = document.getElementById('bulk-snmpv3-priv-fields');
    if (authFields) authFields.style.display = (level === 'authNoPriv' || level === 'authPriv') ? 'block' : 'none';
    if (privFields) privFields.style.display = level === 'authPriv' ? 'block' : 'none';
}

async function executeBulkAdd(event) {
    event.preventDefault();
    const form = event.target;
    const formData = new FormData(form);

    const scanType = formData.get('scan_type');
    const monitorType = formData.get('monitor_type');

    const payload = {
        scan_type: scanType,
        name_prefix: formData.get('name_prefix') || 'Device-',
        device_type: formData.get('device_type'),
        monitor_type: monitorType,
        include_ping_only: formData.get('include_ping_only') === 'on',
        snmp_community: monitorType === 'snmp' ? (formData.get('snmp_community') || '') : '',
        snmp_version: monitorType === 'snmp' ? parseInt(formData.get('snmp_version')) : 0,
        include_ping_only: formData.get('include_ping_only') === 'on',
        snmpv3_security_name: formData.get('snmpv3_security_name'),
        snmpv3_security_level: formData.get('snmpv3_security_level'),
        snmpv3_auth_protocol: formData.get('snmpv3_auth_protocol'),
        snmpv3_auth_password: formData.get('snmpv3_auth_password'),
        snmpv3_priv_protocol: formData.get('snmpv3_priv_protocol'),
        snmpv3_priv_password: formData.get('snmpv3_priv_password'),
        snmpv3_context_name: formData.get('snmpv3_context_name')
    };

    if (scanType === 'range') {
        payload.start_ip = formData.get('start_ip');
        payload.end_ip = formData.get('end_ip');

        if (!payload.start_ip || !payload.end_ip) {
            showToast(t('devices.toast.input_ip_range'), 'warning');
            return;
        }
    } else {
        payload.subnet = formData.get('subnet');

        if (!payload.subnet) {
            showToast(t('devices.toast.input_subnet'), 'warning');
            return;
        }
    }

    // Show progress
    document.getElementById('bulk-add-form').style.display = 'none';
    document.getElementById('bulk-scan-progress').style.display = 'block';

    try {
        const response = await apiPost('/devices/bulk-scan', payload);
        if (response.success) {
            showToast(response.message, 'success');
            hideModal();
            loadDevices();
        }
    } catch (error) {
        showToast(error.message || t('devices.toast.scan_failed'), 'error');
        document.getElementById('bulk-add-form').style.display = 'block';
        document.getElementById('bulk-scan-progress').style.display = 'none';
    }
}

// ===== 設備詳情 =====
async function viewDevice(id) {
    console.log('[DEBUG] viewDevice called with id:', id);
    try {
        console.log('[DEBUG] Fetching basic device info for ID:', id);

        // 1. 先獲取基本設備資訊，判斷監控類型
        const deviceResp = await apiGet(`/devices/${id}`);
        if (!deviceResp.success) throw new Error(deviceResp.error);
        const device = deviceResp.data;

        // 2. 根據監控方式決定是否獲取即時埠位資訊 (Ping Only 設備不執行即時 SNMP 探測，避免 Timeout)
        const isPingOnly = device.snmp_version === 0 || device.device_type === 'ipcam' || device.device_type === 'video_wall';

        console.log(`[DEBUG] Device ${id} monitor type: ${isPingOnly ? 'PingOnly' : 'SNMP'}`);

        // 3. 並行獲取其他數據
        const metricsPromise = apiGet(`/devices/${id}/metrics?limit=1`);
        const interfacesPromise = apiGet(`/devices/${id}/interfaces`);

        // 只有非 PingOnly 設備才去抓取即時 PoE/端口細節
        const livePortsPromise = isPingOnly ?
            Promise.resolve({ success: true, data: [] }) :
            apiGet(`/devices/${id}/poe`).catch(e => ({ success: false, error: e }));

        const [metricsResp, interfacesResp, livePortsResp] = await Promise.all([
            metricsPromise,
            interfacesPromise,
            livePortsPromise
        ]);

        console.log('[DEBUG] Auxiliary API responses:', { metricsResp, interfacesResp, livePortsResp });

        let cpu = 0, mem = 0, disk = 0;
        let memUsed = 0, diskUsed = 0;
        if (metricsResp.success && metricsResp.data && metricsResp.data.length > 0) {
            const m = metricsResp.data[0];
            cpu = m.cpu_usage || 0;
            mem = m.memory_usage || 0;
            disk = m.disk_usage || 0;
            memUsed = m.mem_used || 0;
            diskUsed = m.disk_used || 0;
        }

        let interfaces = interfacesResp.success && interfacesResp.data ? interfacesResp.data : [];
        const activePorts = interfaces.filter(i => i.if_status === 'up').length;

        // Merge Live Data if available
        if (livePortsResp.success && livePortsResp.data) {
            const liveData = livePortsResp.data;
            interfaces = interfaces.map(iface => {
                const live = liveData.find(l => l.index === iface.if_index);
                if (live) {
                    return { ...iface, ...live }; // Merge live PoE/VLAN info
                }
                return iface;
            });
        }
        window.currentDeviceInterfaces = interfaces;

        console.log('[DEBUG] Rendering modal');
        renderDeviceDetailModal(device, cpu, mem, disk, memUsed, diskUsed, activePorts, interfaces);
    } catch (error) {
        console.error('[ERROR] viewDevice failed:', error);
        showToast('無法載入設備詳情: ' + error.message, 'error');
    }
}

function renderDeviceDetailModal(device, cpu, mem, disk, memUsed, diskUsed, activePorts, interfaces) {
    const cpuDeg = (cpu / 100) * 180;
    const memDeg = (mem / 100) * 180;
    const diskDeg = (disk / 100) * 180;

    const content = `
        <div class="device-detail-header">
            <div class="header-left">
                <button type="button" class="btn btn-secondary" onclick="hideModal()" style="padding: 4px 8px;">←</button>
                <div class="header-info">
                    <h1>
                        ${escapeHtml(getDeviceDisplayName(device))} 
                        <span class="status-badge ${device.is_online ? 'online' : 'offline'}">
                            ${device.is_online ? '● Online' : '○ Offline'}
                        </span>
                    </h1>
                    <p>
                        IP: ${device.ip_address} | MAC: ${device.mac_address || '-'} | ${t('devices.detail.type')}: ${device.device_type === 'unknown' ? t('devices.type_other') : device.device_type}
                        <br>
                        <small class="text-muted">${t('devices.detail.brand')}: ${device.vendor || device.manufacturer || '-'} ${device.sys_name ? `| SysName: ${device.sys_name}` : ''}</small>
                    </p>
                </div>
            </div>
            <div class="header-meta" style="display: flex; gap: 15px; align-items: center;">
                ${(device.snmp_community && isEdgecoreDevice(device) && !isViewer()) ? `
                    <button class="btn btn-primary btn-sm" onclick="handleBackupConfig(${device.id})" title="備份設定 (show running-config)">
                        💾 備份
                    </button>
                    <button class="btn btn-danger btn-sm" onclick="handleReboot(${device.id})" title="重新啟動設備">
                        🔄 重啟
                    </button>
                ` : ''}
                ${!isViewer() ? `
                    <button class="btn btn-secondary btn-sm" onclick="openWebSSH(${device.id}, '${escapeHtml(device.ip_address)}')" title="SSH 終端機">
                        >_ SSH
                    </button>
                ` : ''}
                <div class="uptime">${t('devices.detail.uptime')}<br><span class="uptime-val">${device.sys_uptime || '-'}</span></div>
                <div class="snmp-ver">${(device.snmp_version === 0 || !device.snmp_community) ? t('devices.detail.ping_only') : `${t('devices.detail.snmp_v')} v${device.snmp_version}`}</div>
            </div>
        </div>

        <div class="device-stats-grid" style="grid-template-columns: repeat(4, 1fr);">
            <!-- CPU Card -->
            <div class="stat-card">
                <h3><i class="icon-cpu"></i> ${t('devices.detail.cpu_usage')}</h3>
                ${renderUsageGauge(cpu, 'CPU', cpu > 80 ? 'var(--danger-color)' : (cpu > 60 ? 'var(--warning-color)' : 'var(--success-color)'), cpu.toFixed(1) + '%')}
            </div>
            
            <!-- Memory Card -->
            <div class="stat-card memory">
                <h3><i class="icon-memory"></i> ${t('devices.detail.mem_usage')}</h3>
                ${renderUsageGauge(mem, 'MEM', mem > 85 ? 'var(--danger-color)' : (mem > 70 ? 'var(--warning-color)' : 'var(--primary-color)'), memUsed > 0 ? formatBytes(memUsed) : mem.toFixed(1) + '%')}
            </div>

            <!-- Disk Card -->
            <div class="stat-card disk">
                <h3><i class="icon-disk"></i> ${t('devices.detail.disk_usage')}</h3>
                ${renderUsageGauge(disk, 'DISK', disk > 90 ? 'var(--danger-color)' : (disk > 75 ? 'var(--warning-color)' : '#8b5cf6'), diskUsed > 0 ? formatBytes(diskUsed) : disk.toFixed(1) + '%')}
            </div>
            
            <!-- Ports Card -->
            <div class="stat-card ports">
                <h3><i class="icon-network"></i> ${t('devices.detail.interfaces')}</h3>
                <div class="stat-big-number">${activePorts}</div>
                <div class="stat-label">${t('devices.detail.active_ports')}</div>
            </div>
        </div>

        <!-- Physical Port Matrix (For All SNMP Switches) -->
        ${(device.snmp_community && interfaces.length > 0 && device.device_type === 'switch') ? `
        <div class="port-matrix-section">
            <div class="section-header">
                <h3>${t('devices.detail.port_matrix_title')} (Physical Ports Status)</h3>
                <span class="legend">
                    <span class="dot up"></span> ${t('devices.detail.legend_link_up')} 
                    <span class="dot down"></span> ${t('devices.detail.legend_link_down')}
                    <span style="display:inline-block; width:10px; height:10px; background:gold; margin-left:10px; border-radius:50%;"></span> ${t('devices.detail.legend_poe')}
                </span>
            </div>
            <div class="switch-rack">
                <div class="switch-faceplate">
                    <div class="port-grid">
                        ${renderPortMatrix(interfaces, device.id)}
                    </div>
                </div>
            </div>
        </div>
        ` : ''}

        <div class="device-detail-tabs">
            <button class="detail-tab-btn active" onclick="switchDeviceDetailTab('traffic')" data-i18n="devices.detail.tab_traffic">介面速率</button>
            <button class="detail-tab-btn" onclick="switchDeviceDetailTab('logs', ${device.id})" data-i18n="devices.detail.tab_logs">設備日誌</button>
            ${(device.device_type && device.device_type.toLowerCase() === 'switch' && isEdgecoreDevice(device)) ? `
            <button class="detail-tab-btn" onclick="switchDeviceDetailTab('poe', ${device.id})" data-i18n="devices.detail.tab_poe">PoE 狀態</button>
            ` : ''}
            ${isEdgecoreDevice(device) ? `
            <button class="detail-tab-btn" onclick="switchDeviceDetailTab('backups', ${device.id})" data-i18n="devices.detail.tab_backups">備份記錄</button>
            ` : ''}
            ${(device.device_type === 'access_point' || device.device_type === 'ap') ? `
            <button class="detail-tab-btn" onclick="switchDeviceDetailTab('wireless', ${device.id})" data-i18n="devices.detail.tab_wireless">無線狀態</button>
            ` : ''}
        </div>
        
        <div class="device-detail-tab-content active" id="device-traffic-tab">
            <div class="interface-section">
                <h3>${t('devices.detail.traffic_title')} (Interface Real-time Traffic)</h3>
                ${!device.is_online ? `
                <div style="padding:16px;background:var(--bg-secondary);border:1px solid var(--border-color);border-radius:8px;text-align:center;color:var(--text-secondary);">
                    <span style="font-size:1.2rem;">⚠️</span>
                    <div style="margin-top:6px;font-weight:600;">設備離線</div>
                    <div style="font-size:0.85rem;margin-top:4px;">介面資料為最後一次成功輪詢的快取，流量數值不代表即時狀態。</div>
                </div>
                ` : ''}
                <div style="overflow-x: auto; ${!device.is_online ? 'opacity:0.5;pointer-events:none;margin-top:10px;' : ''}">
                    <table class="interface-table">
                        <thead>
                            <tr>
                                <th data-i18n="devices.detail.if_name">${t('devices.detail.if_name')}</th>
                                <th data-i18n="devices.detail.if_status">${t('devices.detail.if_status')}</th>
                                <th data-i18n="devices.detail.if_speed">${t('devices.detail.if_speed')}</th>
                                <th data-i18n="devices.detail.if_in">${t('devices.detail.if_in')}</th>
                                <th data-i18n="devices.detail.if_out">${t('devices.detail.if_out')}</th>
                            </tr>
                        </thead>
                        <tbody>
                            ${interfaces.map(iface => `
                                <tr>
                                    <td>${iface.if_name || iface.if_desc || iface.if_index}</td>
                                    <td>
                                        <span class="status-badge ${iface.if_status === 'up' ? 'active' : 'inactive'}" style="padding: 2px 6px; font-size: 10px;">
                                            ${iface.if_status ? iface.if_status.toUpperCase() : 'UNKNOWN'}
                                        </span>
                                    </td>
                                    <td>${formatSpeed(iface.if_speed)}</td>
                                    <td class="speed-in">${device.is_online ? formatSpeed(iface.bandwidth_in * 8) : '-'}</td>
                                    <td class="speed-out">${device.is_online ? formatSpeed(iface.bandwidth_out * 8) : '-'}</td>
                                </tr>
                            `).join('')}
                        </tbody>
                    </table>
                </div>
            </div>
        </div>
            
        <div class="device-detail-tab-content" id="device-logs-tab">
            <div class="interface-section">
                <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px;">
                    <h3>${t('devices.detail.logs_title')} (Device Logs)</h3>
                    <div class="header-actions">
                        <button class="btn btn-secondary btn-sm" onclick="exportDeviceLogs(${device.id}, 'csv')">${t('devices.detail.export_csv')}</button>
                    </div>
                </div>
                <div id="device-events-container">
                    <p class="empty-message">${t('devices.detail.no_logs')}</p>
                </div>
            </div>
        </div>

        <div class="device-detail-tab-content" id="device-poe-tab">
            <div class="interface-section">
                <h3>${t('devices.detail.poe_title')} (PoE Status & Control)</h3>
                <div id="poe-loading" style="text-align:center; padding: 20px;">${t('devices.detail.loading')}</div>
                <div id="poe-container" style="display:none;">
                    <!-- Content will be loaded dynamically via loadPoEInfo -->
                </div>
            </div>
        </div>

        <div class="device-detail-tab-content" id="device-backups-tab">
            <div class="interface-section">
                <div style="display:flex; align-items:center; justify-content:space-between; margin-bottom:12px;">
                    <h3>💾 設定備份記錄</h3>
                    ${!isViewer() ? `<button class="btn btn-primary btn-sm" onclick="handleBackupConfig(${device.id})">立即備份</button>` : ''}
                </div>
                <div id="backups-loading" style="text-align:center; padding: 20px;">${t('devices.detail.loading')}</div>
                <div id="backups-container" style="display:none;"></div>
            </div>
        </div>

        <div class="device-detail-tab-content" id="device-wireless-tab">
            <div class="interface-section">
                <h3>${t('devices.detail.wireless_title')} (Wireless Status)</h3>
                <div id="wireless-loading" style="text-align:center; padding: 20px;">${t('devices.detail.loading')}</div>
                <div id="wireless-container" style="display:none;">
                    <!-- Content will be loaded dynamically via loadWirelessInfo -->
                </div>
            </div>
        </div>
    `;

    // 使用較寬的模態框
    const modal = document.getElementById('modal');
    const modalContent = document.getElementById('modal-body');
    const overlay = document.getElementById('modal-overlay');

    if (modal && modalContent) {
        document.getElementById('modal-title').textContent = t('devices.detail.title') || '設備詳情';
        modalContent.innerHTML = content;
        modal.classList.add('wide-modal');
        overlay.classList.add('active');
        applyTranslations();
    }
}

function renderPortMatrix(interfaces, deviceId) {
    const livePorts = window.currentDeviceLivePorts || [];

    // Identify physical ports.
    // Strategy: Use DB interfaces (discovered fully) as the base list of ports.
    // Filter DB interfaces for physical ports (Type 6, or Name contains Eth/Ge/etc, and ignore Vlan/Null/Loopback)
    // Then merge status from livePorts where indexes match.

    // 1. Filter Physical Ports from DB Interfaces
    let physicalPorts = interfaces.filter(i => {
        const name = (i.if_name || '').toLowerCase();
        const desc = (i.if_desc || '').toLowerCase();
        const combined = (name + ' ' + desc).toLowerCase();

        // 1. 排除明確的虛擬/邏輯介面 (VLAN, Loopback, Null, Bridge 等)
        if (combined.includes('vlan') ||
            combined.includes('loopback') ||
            combined.includes('null') ||
            combined.includes('bridge') ||
            combined.includes('software') ||
            combined.includes('virtual') ||
            combined.includes('internal')) {
            return false;
        }

        // 2. 包含明確的實體介面名稱格式 (例如 GigabitEthernet, FastEthernet, Eth1/0/1, Port 1 等)
        // 使用更寬鬆的正則表達式，包含常見的交換器端口命名方式
        if (combined.match(/(gigabit|giga|fast|ethernet|eth|port|gi|te|fe|xe|ge)\s*([0-9]|\/\d+|[a-z]\d+)/i)) {
            return true;
        }

        // 3. 次要判斷：如果名稱很短且包含數字與斜線（常見於 1/0/1 這種格式）
        if (name.match(/^[0-9\/]+$/) && name.length > 0) {
            return true;
        }

        // 4. 最後退路：索引通常較小且非上述虛擬關鍵字 (排除明確虛擬後，前 64 個介面通常是實體介面)
        if (i.if_index > 0 && i.if_index <= 64) {
            return true;
        }

        return false;
    });

    // If DB is empty (unlikely if SNMP worked), use livePorts as fallback source
    if (physicalPorts.length === 0 && livePorts.length > 0) {
        physicalPorts = livePorts.filter(p => p.type === 6 && p.index < 1000).map(p => ({
            if_index: p.index,
            if_name: p.descr,
            if_status: p.oper_status === 1 ? 'up' : 'down'
        }));
    }

    // 2. Map Key Data
    let portsToShow = physicalPorts.map(i => {
        // Find corresponding live data
        const live = livePorts.find(l => l.index === i.if_index);

        // Determine status: Live > DB > Down
        let isUp = false;
        if (live) {
            isUp = live.oper_status === 1;
        } else {
            isUp = i.if_status === 'up'; // Fallback to cached status
        }

        return {
            index: i.if_index,
            descr: i.if_name || i.if_desc || `Port ${i.if_index}`,
            oper_status: isUp ? 1 : 2,
            vlan: live ? live.vlan : 0,
            poe: live ? live.poe : false,
            poe_status: live ? live.poe_status_text : '',
            speed: i.if_speed,
            bandwidth_in: i.bandwidth_in || 0,
            bandwidth_out: i.bandwidth_out || 0
        };
    });

    // Sort by index (Numeric)
    portsToShow.sort((a, b) => a.index - b.index);

    // Limit to reasonable number if something goes wrong (e.g. 1000 ports)
    if (portsToShow.length > 60) portsToShow = portsToShow.slice(0, 60);

    // Generate HTML
    let html = '';
    const isSingleRow = portsToShow.length <= 12;

    if (isSingleRow) {
        // Single row layout
        html = `
            <div class="port-grid single-row">
                <div class="port-row">
                    ${portsToShow.map((port, idx) => renderPortIcon(port, idx + 1, deviceId)).join('')}
                </div>
            </div>
        `;
    } else {
        // Double row layout (staggered)
        const topRow = portsToShow.filter((_, idx) => idx % 2 === 0);
        const bottomRow = portsToShow.filter((_, idx) => idx % 2 !== 0);

        html = `
            <div class="port-grid double-row">
                <div class="port-row top">
                    ${topRow.map((port, idx) => renderPortIcon(port, (idx * 2) + 1, deviceId)).join('')}
                </div>
                <div class="port-row bottom">
                    ${bottomRow.map((port, idx) => renderPortIcon(port, (idx * 2) + 2, deviceId)).join('')}
                </div>
            </div>
        `;
    }

    return `
        <div class="switch-rack">
            <div class="switch-faceplate">
                <div class="faceplate-header">
                    <span class="brand-label">SWITCH</span>
                    <div class="status-leds">
                        <span class="led pwr active"></span>
                        <span class="led sys active"></span>
                    </div>
                </div>
                ${html}
            </div>
        </div>
    `;
}

// Helper to render individual port icon
function renderPortIcon(port, displayNum, deviceId) {
    const isUp = port.oper_status === 1;
    const hasPoE = port.poe;
    const vlanColor = port.vlan ? getVlanColor(port.vlan) : 'transparent';
    const vlanStyle = (isUp && port.vlan) ? `box-shadow: 0 0 5px ${vlanColor}; border-color: ${vlanColor};` : '';

    const avgRate = (port.bandwidth_in + port.bandwidth_out) * 8;
    const title = `${port.descr} (顯示編號: ${displayNum})\n` +
        `系統索引 (ifIndex): ${port.index}\n` +
        `狀態: ${isUp ? 'UP' : 'DOWN'}\n` +
        `實體速率: ${formatSpeed(port.speed)}\n` +
        `即時負載: ${formatSpeed(avgRate)} (兩次加總平均)\n` +
        `VLAN: ${port.vlan || '--'}`;

    return `
        <div class="port-icon ${isUp ? 'up' : 'down'} ${hasPoE ? 'poe' : ''}" 
             onmouseover="showPortTooltip(${deviceId}, ${port.index}, ${displayNum}, event)"
             onclick="showPortActions(${deviceId}, ${port.index}, ${displayNum}, event)"
             style="${vlanStyle}"
             title="${title}">
            <div class="port-inner" style="${(isUp && port.vlan) ? 'background-color:' + vlanColor : ''}">
                ${hasPoE ? '<span class="poe-indicator">⚡</span>' : ''}
            </div>
            <div class="port-num">${displayNum}</div>
        </div>
    `;
}

// Show Port Actions Menu
function showPortActions(deviceId, portIndex, displayNum, e) {
    if (!window.currentDeviceInterfaces) return;

    let port = window.currentDeviceInterfaces.find(i => i.if_index === portIndex || i.index === portIndex);
    if (!port) return;

    // Remove existing tooltip/menu
    const oldMenu = document.querySelector('.port-actions-menu');
    if (oldMenu) oldMenu.remove();

    const menu = document.createElement('div');
    menu.className = 'port-actions-menu';

    const isUp = port.oper_status === 1 || port.if_status === 'up';

    let html = `
        <div class="menu-header">Port ${displayNum} 操作</div>
        <div class="menu-body">
    `;

    // PoE Actions (editor+ only)
    if (port.poe && !isViewer()) {
        const admin = port.poe_admin; // 1=on, 2=off
        html += `
            <div class="menu-section">
                <label>PoE 管理</label>
                <div class="action-grid">
                    <button class="btn btn-sm ${admin === 1 ? 'btn-secondary' : 'btn-success'}" onclick="confirmPortAction(${deviceId}, ${portIndex}, 'poe', '${admin === 1 ? 'off' : 'on'}')">${admin === 1 ? 'PoE 關閉' : 'PoE 開啟'}</button>
                    <button class="btn btn-sm btn-primary" onclick="confirmPortAction(${deviceId}, ${portIndex}, 'poe', 'recycle')">PoE 重啟</button>
                </div>
            </div>
        `;
    }

    // Admin Status Actions (editor+ only)
    if (!isViewer()) {
    html += `
        <div class="menu-section">
            <label>埠位狀態管理</label>
            <div class="action-grid">
                ${isUp ?
            `<button class="btn btn-sm btn-danger" onclick="confirmPortAction(${deviceId}, ${portIndex}, 'status', 'down')">Shutdown (關閉)</button>` :
            `<button class="btn btn-sm btn-success" onclick="confirmPortAction(${deviceId}, ${portIndex}, 'status', 'up')">No Shutdown (開啟)</button>`
        }
            </div>
        </div>
    `;
    } // end !isViewer() for admin status actions

    html += `</div>`;
    menu.innerHTML = html;
    document.body.appendChild(menu);

    // Position menu
    const rect = e.target.closest('.port-icon').getBoundingClientRect();
    menu.style.position = 'fixed';
    menu.style.left = (rect.left) + 'px';
    menu.style.top = (rect.bottom + 10) + 'px';
    menu.style.zIndex = '10001';

    // Click outside to close
    const closeMenu = (event) => {
        if (!menu.contains(event.target)) {
            menu.remove();
            document.removeEventListener('mousedown', closeMenu);
        }
    };
    setTimeout(() => document.addEventListener('mousedown', closeMenu), 100);
}

function confirmPortAction(deviceId, portIndex, type, action) {
    const actionMap = { 'up': '開啟 (No Shutdown)', 'down': '關閉 (Shutdown)', 'on': '開啟供電', 'off': '關閉供電', 'recycle': '重啟供電' };
    const detailMsg = type === 'poe'
        ? `確定要對 Port ${portIndex} 執行 ${actionMap[action] || action} 嗎？這可能會導致連接設備重啟。`
        : `確定要對 Port ${portIndex} 執行 ${actionMap[action]} 嗎？這將會中斷該埠位的網路連線！`;

    showConfirm(detailMsg, async () => {
        try {
            let response;
            if (type === 'poe') {
                response = await controlPoEPort(deviceId, portIndex, { action });
            } else {
                response = await apiPost(`/devices/${deviceId}/interfaces/${portIndex}/status`, { status: action });
            }

            if (response.success) {
                showToast(t('devices.toast.command_sent') || '指令已發送', 'success');
                const menu = document.querySelector('.port-actions-menu');
                if (menu) menu.remove();
                setTimeout(() => viewDevice(deviceId), 2000);
            }
        } catch (error) {
            showToast((t('devices.toast.operation_failed') || '操作失敗') + ': ' + error.message, 'error');
        }
    });
}

async function exportDeviceLogs(deviceId, format) {
    const url = `/api/reports/logs?type=events&device_id=${deviceId}&format=${format}`;
    window.open(url, '_blank');
}

// Generate consistent color for VLAN ID
function getVlanColor(vlanId) {
    if (!vlanId) return '#333';
    // HSL based on VLAN ID
    const hue = (vlanId * 137.508) % 360; // Golden angle approximation
    return `hsl(${hue}, 70 %, 40 %)`; // Vivid but not too bright
}

// Handle Reboot — uses stored cli_username/cli_password from DB (no credential prompt)
async function handleReboot(id) {
    showConfirm('確定要重啟設備嗎？重啟期間網路將中斷數分鐘。', async () => {
        try {
            const response = await rebootDevice(id, {});
            if (response.success) {
                showToast(t('devices.toast.reboot_sent') || '重啟指令已發送', 'success');
            }
        } catch (error) {
            showToast((t('devices.toast.reboot_failed') || '重啟失敗') + ': ' + error.message, 'error');
        }
    });
}

// Handle Backup — EdgeCore: SSH show running-config → DB; others: SNMP write-memory
async function handleBackupConfig(id) {
    showConfirm('確定要備份設備設定嗎？系統將透過 SSH 擷取 running-config 並儲存。', async () => {
        try {
            const response = await apiPost(`/devices/${id}/backup`, {});
            if (response.success) {
                showToast(t('devices.toast.backup_sent') || '備份完成', 'success');
                // Refresh backup history if tab is open
                const tab = document.getElementById('device-backups-tab');
                if (tab && tab.classList.contains('active')) {
                    loadBackupHistory(id);
                }
            }
        } catch (error) {
            showToast((t('devices.toast.backup_failed') || '備份失敗') + ': ' + error.message, 'error');
        }
    });
}

// Load PoE Info when tab is switched
async function loadPoEInfo(deviceId) {
    const container = document.getElementById('poe-container');
    const loading = document.getElementById('poe-loading');

    if (!container) return;

    try {
        loading.style.display = 'block';
        container.style.display = 'none';

        const response = await getDevicePoE(deviceId);
        if (response.success && response.data) {
            const poeData = response.data;
            // If using new data format, map keys
            // New: index, descr, poe_admin (1=on, 2=off), poe_status_text
            // Old: port_index, admin_status, status

            // We should normalize or handle dynamic check.
            // The backend now returns full port list if using GetDevicePortDetails.
            // So filter only poe=true ports.

            const validPoE = poeData.filter(p => p.poe === true || p.admin_status !== undefined);

            if (validPoE.length === 0) {
                container.innerHTML = '<p class="empty-message">此設備回報無 PoE 端口</p>';
                loading.style.display = 'none';
                container.style.display = 'block';
                return;
            }

            // Total PoE budget/consumption (from first port entry, same for all)
            const poeBudgetMw = validPoE[0]?.poe_budget_mw || 0;
            const poeUsedMw   = validPoE[0]?.poe_used_mw   || 0;
            const poeBudgetW  = poeBudgetMw ? (poeBudgetMw / 1000).toFixed(0) : null;
            const poeUsedW    = (poeUsedMw / 1000).toFixed(1);
            const poeUsedPct  = poeBudgetMw ? Math.min(100, (poeUsedMw / poeBudgetMw * 100)).toFixed(1) : 0;
            const barColor    = poeUsedPct > 85 ? '#ef4444' : poeUsedPct > 60 ? '#f59e0b' : 'var(--primary-color)';
            const poeBudgetBar = poeBudgetW
                ? `<div style="margin-bottom:14px;padding:12px 16px;background:var(--bg-secondary);border-radius:8px;border:1px solid var(--border-color);">
                    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:10px;">
                        <span style="font-size:0.9rem;font-weight:600;">⚡ PoE 功率摘要</span>
                        <span style="font-size:0.8rem;color:var(--text-secondary);">使用率 ${poeUsedPct}%</span>
                    </div>
                    <div style="display:grid;grid-template-columns:1fr 1fr;gap:10px;margin-bottom:10px;">
                        <div style="padding:8px 12px;background:var(--bg-primary);border-radius:6px;border:1px solid var(--border-color);">
                            <div style="font-size:0.75rem;color:var(--text-secondary);margin-bottom:2px;">總功率 (Budget)</div>
                            <div style="font-size:1.1rem;font-weight:700;">${poeBudgetW} W</div>
                        </div>
                        <div style="padding:8px 12px;background:var(--bg-primary);border-radius:6px;border:1px solid var(--border-color);">
                            <div style="font-size:0.75rem;color:var(--text-secondary);margin-bottom:2px;">已使用功率</div>
                            <div style="font-size:1.1rem;font-weight:700;">${poeUsedW} W</div>
                        </div>
                    </div>
                    <div style="height:8px;background:var(--bg-tertiary);border-radius:4px;overflow:hidden;">
                        <div style="height:100%;width:${poeUsedPct}%;background:${barColor};border-radius:4px;transition:width 0.3s;"></div>
                    </div>
                   </div>`
                : '';

            let html = poeBudgetBar + `
                    <table class="interface-table">
                        <thead>
                            <tr>
                                <th>Port</th>
                                <th>描述</th>
                                <th>PoE 狀態</th>
                                <th>功率 (W)</th>
                                <th>管理</th>
                                <th>操作</th>
                            </tr>
                        </thead>
                        <tbody>
                            ${validPoE.sort((a, b) => (a.index || a.port_index) - (b.index || b.port_index)).map(p => {
                const idx = p.index || p.port_index;
                const status = p.poe_status_text || p.status || 'unknown';
                const admin = p.poe_admin || p.admin_status;

                // Power display priority: EdgeCore private OID (real mW) > class-estimated > N/A
                // poe_power_mw comes from EdgeCore private OID — 0 means no PD connected
                let powerDisplay = '-';
                if (p.poe_power_mw != null && p.poe_power_mw > 0) {
                    // Real measured power — device is drawing power
                    powerDisplay = (p.poe_power_mw / 1000).toFixed(1) + ' W';
                } else if (p.poe_power_mw === 0) {
                    // OID returned 0 — port is PoE capable but no PD connected or non-PoE device
                    powerDisplay = '<small style="color:var(--text-secondary)">非 PoE 設備 / 0 W</small>';
                } else if (status === 'delivering') {
                    // No private OID data, but status says delivering — use class estimate
                    if (p.poe_class_max_mw) {
                        powerDisplay = `≤${(p.poe_class_max_mw / 1000).toFixed(1)} W <small style="color:var(--text-secondary)">(Class ${p.poe_class})</small>`;
                    } else {
                        powerDisplay = '<small style="color:var(--text-secondary)">供電中</small>';
                    }
                }

                return `
                                <tr>
                                    <td>Port ${idx}</td>
                                    <td>${escapeHtml(p.descr || '')}</td>
                                    <td>
                                        <span class="status-badge ${status.includes('delivering') ? 'active' : 'inactive'}">
                                            ${status.toUpperCase()}
                                        </span>
                                    </td>
                                    <td>${powerDisplay}</td>
                                    <td>${admin === 1 ? 'Enabled' : 'Disabled'}</td>
                                    <td>
                                        ${!isViewer() ? `<div class="action-buttons">
                                            <button class="btn btn-xs ${admin === 1 ? 'btn-warning' : 'btn-success'}"
                                                    onclick="handlePoEControl(${deviceId}, ${idx}, '${admin === 1 ? 'off' : 'on'}')"
                                                    title="${admin === 1 ? '關閉供電' : '開啟供電'}">
                                                ${admin === 1 ? '關閉' : '開啟'}
                                            </button>
                                            <button class="btn btn-xs btn-primary"
                                                    onclick="handlePoEControl(${deviceId}, ${idx}, 'recycle')"
                                                    title="重啟端口">
                                                重啟 (Recycle)
                                            </button>
                                        </div>` : '—'}
                                    </td>
                                </tr>
                            `}).join('')}
                        </tbody>
                    </table>
                `;
            container.innerHTML = html;
            loading.style.display = 'none';
            container.style.display = 'block';
        } else {
            container.innerHTML = `<p class="empty-message">${t('devices.detail.no_poe_data') || '此設備不支援 PoE 或暫無數據'}</p>`;
            loading.style.display = 'none';
            container.style.display = 'block';
        }
    } catch (error) {
        container.innerHTML = `<p class="error-message">${t('common.error')}: ${error.message}</p>`;
        loading.style.display = 'none';
        container.style.display = 'block';
    }
}

// Load backup history for a device
async function loadBackupHistory(deviceId) {
    const container = document.getElementById('backups-container');
    const loading = document.getElementById('backups-loading');
    if (!container) return;

    try {
        loading.style.display = 'block';
        container.style.display = 'none';

        const response = await apiGet(`/devices/${deviceId}/backups`);
        if (!response.success) throw new Error(response.error || '載入失敗');

        const backups = response.data || [];

        if (backups.length === 0) {
            container.innerHTML = '<p class="empty-message" style="padding:20px; text-align:center;">尚無備份記錄。點擊「立即備份」開始第一次備份。</p>';
        } else {
            container.innerHTML = `
                <table class="interface-table">
                    <thead>
                        <tr>
                            <th>#</th>
                            <th>備份時間</th>
                            <th>備註</th>
                            <th>大小</th>
                            <th>操作</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${backups.map(b => `
                        <tr>
                            <td>${b.id}</td>
                            <td>${b.created_at}</td>
                            <td>${escapeHtml(b.note || '-')}</td>
                            <td>${b.size > 1024 ? (b.size / 1024).toFixed(1) + ' KB' : b.size + ' B'}</td>
                            <td>
                                <a href="/api/v1/devices/${deviceId}/backups/${b.id}/download"
                                   class="btn btn-xs btn-primary"
                                   download
                                   onclick="this.href='/api/v1/devices/${deviceId}/backups/${b.id}/download?token='+getAuthToken()">
                                    ⬇ 下載
                                </a>
                            </td>
                        </tr>
                        `).join('')}
                    </tbody>
                </table>
            `;
        }

        loading.style.display = 'none';
        container.style.display = 'block';
    } catch (error) {
        container.innerHTML = `<p class="error-message" style="padding:20px;">${error.message}</p>`;
        loading.style.display = 'none';
        container.style.display = 'block';
    }
}

async function loadWirelessInfo(deviceId) {
    const container = document.getElementById('wireless-container');
    const loading = document.getElementById('wireless-loading');

    if (!container) return;

    try {
        loading.style.display = 'block';
        container.style.display = 'none';

        const response = await apiGet(`/devices/${deviceId}/ap`);
        if (response.success && response.data) {
            const data = response.data;
            let html = `
                <div class="wireless-info-grid" style="display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin-bottom: 20px;">
                    <div class="stat-card" style="background: var(--bg-secondary); padding: 20px; border-radius: 12px; border: 1px solid var(--border-color); text-align: center;">
                        <h4 style="margin-top: 0; color: var(--text-muted); font-size: 14px;">連線終端數 (Clients)</h4>
                        <div style="font-size: 36px; font-weight: bold; color: var(--primary-color); margin: 10px 0;">${data.client_count}</div>
                        <div style="font-size: 12px; color: var(--text-muted);">Associated Clients</div>
                    </div>
                </div>

                <div class="ssid-section">
                    <h4 style="border-left: 4px solid var(--primary-color); padding-left: 10px; margin-bottom: 15px;">射頻訊號 (SSIDs)</h4>
                    <div class="ssid-list" style="display: flex; flex-wrap: wrap; gap: 10px;">
                        ${data.ssids && data.ssids.length > 0 ? data.ssids.map(ssid => `
                            <div class="ssid-badge" style="background: rgba(79, 70, 229, 0.1); border: 1px solid var(--primary-color); color: var(--primary-color); padding: 8px 15px; border-radius: 20px; display: flex; align-items: center; gap: 8px;">
                                <span>📶</span>
                                <strong>${escapeHtml(ssid)}</strong>
                            </div>
                        `).join('') : '<p class="empty-message">未偵測到發射中的 SSID</p>'}
                    </div>
                </div>

                <div class="wireless-actions" style="margin-top: 30px; padding-top: 20px; border-top: 1px solid var(--border-color);">
                    <button class="btn btn-secondary btn-sm" onclick="loadWirelessInfo(${deviceId})">🔄 重新整理射頻資訊</button>
                </div>
            `;
            container.innerHTML = html;
            loading.style.display = 'none';
            container.style.display = 'block';
        } else {
            container.innerHTML = '<p class="empty-message">此設備未提供無線存取資訊</p>';
            loading.style.display = 'none';
            container.style.display = 'block';
        }
    } catch (error) {
        container.innerHTML = `<p class="error-message">載入失敗: ${error.message}</p>`;
        loading.style.display = 'none';
        container.style.display = 'block';
    }
}

async function handlePoEControl(deviceId, portIndex, action) {
    const actionText = action === 'recycle' ? '重撥 (Recycle)' : (action === 'on' ? '開啟' : '關閉');
    showConfirm(`確定要對 Port ${portIndex} 執行 ${actionText} 嗎？`, async () => {
        try {
            const response = await controlPoEPort(deviceId, portIndex, { action });
            if (response.success) {
                showToast(t('devices.toast.poe_action_sent').replace('{action}', actionText), 'success');
                // Reload PoE info after a short delay
                setTimeout(() => loadPoEInfo(deviceId), 2000);
            }
        } catch (error) {
            showToast(t('devices.toast.operation_failed') + ': ' + error.message, 'error');
        }
    });
}

function formatSpeed(bps) {
    if (!bps || bps === 0) return '0 bps';
    if (bps >= 1000000000000) return `${(bps / 1000000000000).toFixed(2)} Tbps`;
    if (bps >= 1000000000) return `${(bps / 1000000000).toFixed(1)} Gbps`;
    if (bps >= 1000000) return `${(bps / 1000000).toFixed(1)} Mbps`;
    if (bps >= 1000) return `${(bps / 1000).toFixed(1)} Kbps`;
    return `${bps} bps`;
}

// ===== 編輯設備 =====
async function editDevice(id) {
    try {
        const response = await apiGet(`/devices/${id}`);
        if (response.success) {
            const device = response.data;
            const hasSNMP = (device.snmp_community || device.snmp_version === 3) ? true : false;
            const content = `
                <form id="edit-device-form" onsubmit="updateDevice(event, ${id})">
                    <div class="form-group">
                        <label data-i18n="devices.modal.name_label">${t('devices.modal.name_label')}</label>
                        <input type="text" name="name" required value="${escapeHtml(device.name)}" placeholder="${t('devices.modal.name_placeholder')}">
                    </div>
                    <div class="form-group">
                        <label data-i18n="devices.modal.ip_label">${t('devices.modal.ip_label')}</label>
                        <input type="text" name="ip_address" required value="${device.ip_address}" placeholder="${t('devices.modal.ip_placeholder')}">
                    </div>
                    <div class="form-group">
                        <label data-i18n="devices.modal.device_type">設備類型</label>
                        <select name="device_type">
                            <option value="other" ${device.device_type === 'other' || device.device_type === 'unknown' ? 'selected' : ''} data-i18n="devices.type_other">其他</option>
                            <option value="router" ${device.device_type === 'router' ? 'selected' : ''} data-i18n="devices.type_router">路由器</option>
                            <option value="switch" ${device.device_type === 'switch' ? 'selected' : ''} data-i18n="devices.type_switch">交換器</option>
                            <option value="access_point" ${device.device_type === 'access_point' ? 'selected' : ''} data-i18n="devices.type_ap">無線存取點 (AP)</option>
                            <option value="firewall" ${device.device_type === 'firewall' ? 'selected' : ''} data-i18n="devices.type_firewall">防火牆</option>
                            <option value="server" ${device.device_type === 'server' ? 'selected' : ''} data-i18n="devices.type_server">伺服器</option>
                            <option value="codec" ${device.device_type === 'codec' ? 'selected' : ''} data-i18n="devices.type_codec">編解碼器 (Codec)</option>
                            <option value="ipcam" ${device.device_type === 'ipcam' ? 'selected' : ''} data-i18n="devices.type_ipcam">攝影機 (IPCAM)</option>
                            <option value="video_wall" ${device.device_type === 'video_wall' ? 'selected' : ''} data-i18n="devices.type_video_wall">電視牆控制器</option>
                            <option value="access_control" ${device.device_type === 'access_control' ? 'selected' : ''} data-i18n="devices.type_access_control">門禁系統</option>
                            <option value="ups" ${device.device_type === 'ups' ? 'selected' : ''} data-i18n="devices.type_ups">UPS 不斷電系統</option>
                            <option value="pdu" ${device.device_type === 'pdu' ? 'selected' : ''} data-i18n="devices.type_pdu">PDU 電源分配器</option>
                        </select>
                    </div>
                    
                    <div class="form-group">
                        <label data-i18n="devices.modal.vendor_label">${t('devices.modal.vendor_label')}</label>
                        <input type="text" name="vendor" value="${escapeHtml(device.vendor || '')}" placeholder="${t('devices.modal.vendor_placeholder')}">
                    </div>
                    <div class="form-group">
                        <label data-i18n="devices.modal.model_label">${t('devices.modal.model_label')}</label>
                        <input type="text" name="model" value="${escapeHtml(device.model || '')}" placeholder="${t('devices.modal.model_placeholder')}">
                    </div>

                    <div class="form-group">
                        <label data-i18n="devices.modal.monitor_method">監控方式</label>
                        <select name="monitor_type" onchange="toggleEditSnmpFields(this.value)">
                            <option value="ping" ${!hasSNMP ? 'selected' : ''}>Ping Only</option>
                            <option value="snmp" ${hasSNMP ? 'selected' : ''}>SNMP</option>
                        </select>
                    </div>
                    <div id="edit-snmp-fields" style="display:${hasSNMP || device.snmp_version === 3 ? 'block' : 'none'};">
                        <div class="form-group">
                            <label data-i18n="devices.modal.snmp_version">${t('devices.modal.snmp_version')}</label>
                            <select name="snmp_version" onchange="toggleEditSnmpVersionFields(this.value)">
                                <option value="2" ${device.snmp_version === 2 ? 'selected' : ''}>v2c</option>
                                <option value="1" ${device.snmp_version === 1 ? 'selected' : ''}>v1</option>
                                <option value="3" ${device.snmp_version === 3 ? 'selected' : ''}>v3 (更安全)</option>
                            </select>
                        </div>

                        <div id="edit-snmpv1v2-fields" style="display:${device.snmp_version !== 3 ? 'block' : 'none'};">
                            <div class="form-group">
                                <label data-i18n="devices.modal.snmp_read">${t('devices.modal.snmp_read')}</label>
                                <input type="text" name="snmp_community" value="${device.snmp_community || 'public'}">
                            </div>
                            <div class="form-group">
                                <label data-i18n="devices.modal.snmp_write">${t('devices.modal.snmp_write')}</label>
                                <input type="text" name="snmp_rw_community" value="${device.snmp_rw_community || ''}" placeholder="${t('devices.modal.snmp_write_placeholder')}">
                            </div>
                        </div>

                        <div id="edit-snmpv3-fields" style="display:${device.snmp_version === 3 ? 'block' : 'none'}; border:1px solid var(--border-color); padding:10px; border-radius:8px; margin-top:10px; background: rgba(255,255,255,0.02);">
                            <div class="form-group">
                                <label data-i18n="devices.modal.snmpv3_user">${t('devices.modal.snmpv3_user')}</label>
                                <input type="text" name="snmpv3_security_name" value="${escapeHtml(device.snmpv3_security_name || '')}" placeholder="${t('devices.modal.snmpv3_user_placeholder')}">
                            </div>
                            <div class="form-group">
                                <label data-i18n="devices.modal.snmpv3_level">${t('devices.modal.snmpv3_level')}</label>
                                <select name="snmpv3_security_level" onchange="toggleEditSnmpV3SecurityFields(this.value)">
                                    <option value="noAuthNoPriv" ${device.snmpv3_security_level === 'noAuthNoPriv' ? 'selected' : ''}>noAuthNoPriv (無認證無加密)</option>
                                    <option value="authNoPriv" ${device.snmpv3_security_level === 'authNoPriv' ? 'selected' : ''}>authNoPriv (有認證無加密)</option>
                                    <option value="authPriv" ${device.snmpv3_security_level === 'authPriv' ? 'selected' : ''}>authPriv (有認證有加密)</option>
                                </select>
                            </div>
                            <div id="edit-snmpv3-auth-fields" style="display:${device.snmpv3_security_level === 'authNoPriv' || device.snmpv3_security_level === 'authPriv' ? 'block' : 'none'};">
                                <div style="display:grid; grid-template-columns: 1fr 2fr; gap:10px;">
                                    <div class="form-group">
                                        <label data-i18n="devices.modal.snmpv3_auth_p">${t('devices.modal.snmpv3_auth_p')}</label>
                                        <select name="snmpv3_auth_protocol">
                                            <option value="MD5" ${device.snmpv3_auth_protocol === 'MD5' ? 'selected' : ''}>MD5</option>
                                            <option value="SHA" ${device.snmpv3_auth_protocol === 'SHA' ? 'selected' : ''}>SHA</option>
                                        </select>
                                    </div>
                                    <div class="form-group">
                                        <label data-i18n="devices.modal.snmpv3_auth_pw">${t('devices.modal.snmpv3_auth_pw')}</label>
                                        <input type="password" name="snmpv3_auth_password" value="${escapeHtml(device.snmpv3_auth_password || '')}">
                                    </div>
                                </div>
                            </div>
                            <div id="edit-snmpv3-priv-fields" style="display:${device.snmpv3_security_level === 'authPriv' ? 'block' : 'none'};">
                                <div style="display:grid; grid-template-columns: 1fr 2fr; gap:10px;">
                                    <div class="form-group">
                                        <label data-i18n="devices.modal.snmpv3_priv_p">${t('devices.modal.snmpv3_priv_p')}</label>
                                        <select name="snmpv3_priv_protocol">
                                            <option value="DES" ${device.snmpv3_priv_protocol === 'DES' ? 'selected' : ''}>DES</option>
                                            <option value="AES" ${device.snmpv3_priv_protocol === 'AES' ? 'selected' : ''}>AES</option>
                                        </select>
                                    </div>
                                    <div class="form-group">
                                        <label data-i18n="devices.modal.snmpv3_priv_pw">${t('devices.modal.snmpv3_priv_pw')}</label>
                                        <input type="password" name="snmpv3_priv_password" value="${escapeHtml(device.snmpv3_priv_password || '')}">
                                    </div>
                                </div>
                            </div>
                            <div class="form-group">
                                <label data-i18n="devices.modal.snmpv3_context">${t('devices.modal.snmpv3_context')}</label>
                                <input type="text" name="snmpv3_context_name" value="${escapeHtml(device.snmpv3_context_name || '')}">
                            </div>
                        </div>

                        <div class="form-group" style="margin-top:20px; border-top:1px solid #333; padding-top:10px;">
                            <label data-i18n="devices.modal.cli_username">${t('devices.modal.cli_username')}</label>
                            <input type="text" name="cli_username" value="${escapeHtml(device.cli_username || '')}">
                        </div>
                        <div class="form-group">
                            <label data-i18n="devices.modal.cli_password">${t('devices.modal.cli_password')}</label>
                            <input type="password" name="cli_password" value="${escapeHtml(device.cli_password || '')}" placeholder="${t('devices.modal.cli_password_placeholder')}">
                        </div>
                    </div>
                    <div class="form-actions">
                        <button type="button" class="btn btn-secondary" onclick="hideModal()" data-i18n="devices.modal.cancel_btn">${t('devices.modal.cancel_btn')}</button>
                        <button type="submit" class="btn btn-primary" data-i18n="common.save">${t('common.save')}</button>
                    </div>
                </form>
            `;
            openModal('modals.edit_device', content);
        }
    } catch (error) {
        showToast(t('devices.toast.load_failed'), 'error');
    }
}

function toggleEditSnmpFields(value) {
    const snmpFields = document.getElementById('edit-snmp-fields');
    snmpFields.style.display = value === 'snmp' ? 'block' : 'none';
    if (value === 'snmp') {
        toggleEditSnmpVersionFields(document.querySelector('#edit-device-form select[name="snmp_version"]').value);
    }
}

function toggleEditSnmpVersionFields(version) {
    const v1v2 = document.getElementById('edit-snmpv1v2-fields');
    const v3 = document.getElementById('edit-snmpv3-fields');
    if (v1v2) v1v2.style.display = version === '3' ? 'none' : 'block';
    if (v3) v3.style.display = version === '3' ? 'block' : 'none';

    if (version === '3') {
        toggleEditSnmpV3SecurityFields(document.querySelector('#edit-device-form select[name="snmpv3_security_level"]').value);
    }
}

function toggleEditSnmpV3SecurityFields(level) {
    const authFields = document.getElementById('edit-snmpv3-auth-fields');
    const privFields = document.getElementById('edit-snmpv3-priv-fields');
    if (authFields) authFields.style.display = (level === 'authNoPriv' || level === 'authPriv') ? 'block' : 'none';
    if (privFields) privFields.style.display = level === 'authPriv' ? 'block' : 'none';
}

async function updateDevice(event, id) {
    event.preventDefault();
    const form = event.target;
    const formData = new FormData(form);

    const monitorType = formData.get('monitor_type');
    const device = {
        name: formData.get('name'),
        ip_address: formData.get('ip_address'),
        device_type: formData.get('device_type'),
        vendor: formData.get('vendor'),
        model: formData.get('model'),
        snmp_community: monitorType === 'snmp' ? (formData.get('snmp_community') || '') : '',
        snmp_rw_community: monitorType === 'snmp' ? formData.get('snmp_rw_community') : '',
        snmp_version: monitorType === 'snmp' ? parseInt(formData.get('snmp_version')) : 0,
        cli_username: formData.get('cli_username'),
        cli_password: formData.get('cli_password'),
        snmpv3_security_name: formData.get('snmpv3_security_name'),
        snmpv3_security_level: formData.get('snmpv3_security_level'),
        snmpv3_auth_protocol: formData.get('snmpv3_auth_protocol'),
        snmpv3_auth_password: formData.get('snmpv3_auth_password'),
        snmpv3_priv_protocol: formData.get('snmpv3_priv_protocol'),
        snmpv3_priv_password: formData.get('snmpv3_priv_password'),
        snmpv3_context_name: formData.get('snmpv3_context_name')
    };

    try {
        const response = await apiPut(`/devices/${id}`, device);
        if (response.success) {
            showToast(t('devices.toast.update_success'), 'success');
            hideModal();
            loadDevices();
        }
    } catch (error) {
        showToast(error.message || t('devices.toast.update_failed'), 'error');
    }
}

// ===== 圖片上傳 =====
function uploadDeviceImage(id) {
    const content = `
                <form id="upload-image-form">
                    <div class="image-upload" id="upload-drop-zone" onclick="document.getElementById('image-input').click()" style="position: relative;">
                        <input type="file" id="image-input" accept="image/*" onchange="previewImage(this)">
                            <div class="icon">📷</div>
                            <p data-i18n="devices.modal.upload_hint">${t('devices.modal.upload_hint')}</p>
                            <img id="image-preview" class="image-preview" style="display:none;">
                            <small class="form-text text-muted" style="display:block; margin-top:12px;">
                                <i class="fas fa-info-circle"></i> ${t('devices.modal.upload_hint_detail') || '建議解析度: 800x600 px | 最大: 3840x2160 px (4K)'}
                            </small>
                            </div>
                            <div class="form-actions">
                                <button type="button" class="btn btn-secondary" onclick="hideModal()" data-i18n="devices.modal.cancel_btn">${t('devices.modal.cancel_btn')}</button>
                                <button type="button" class="btn btn-primary" onclick="submitDeviceImage(${id})" data-i18n="devices.modal.upload_btn">${t('devices.modal.upload_btn')}</button>
                            </div>
                        </form>
                        `;
    openModal('devices.modal_upload_image_title', content);

    // Setup drag and drop events after modal is open
    setTimeout(() => {
        const dropZone = document.getElementById('upload-drop-zone');
        if (dropZone) {
            ['dragenter', 'dragover', 'dragleave', 'drop'].forEach(eventName => {
                dropZone.addEventListener(eventName, preventDefaults, false);
            });

            ['dragenter', 'dragover'].forEach(eventName => {
                dropZone.addEventListener(eventName, highlight, false);
            });

            ['dragleave', 'drop'].forEach(eventName => {
                dropZone.addEventListener(eventName, unhighlight, false);
            });

            dropZone.addEventListener('drop', handleDrop, false);
        }
    }, 100);

    function preventDefaults(e) {
        e.preventDefault();
        e.stopPropagation();
    }

    function highlight(e) {
        dropZone.classList.add('highlight');
        dropZone.style.borderColor = 'var(--primary-color)';
        dropZone.style.backgroundColor = 'var(--bg-tertiary)';
    }

    function unhighlight(e) {
        dropZone.classList.remove('highlight');
        dropZone.style.borderColor = 'var(--border-color)';
        dropZone.style.backgroundColor = '';
    }

    function handleDrop(e) {
        const dt = e.dataTransfer;
        const files = dt.files;
        const input = document.getElementById('image-input');

        if (files && files.length > 0) {
            input.files = files; // Assign dropped files to input
            previewImage(input);
        }
    }
}

function previewImage(input) {
    if (input.files && input.files[0]) {
        const file = input.files[0];

        // 1. Check File Size (Max 5MB)
        if (file.size > 5 * 1024 * 1024) {
            showToast(t('devices.toast.image_too_large') || '圖片過大 (最大 5MB)', 'warning');
            input.value = '';
            return;
        }

        const reader = new FileReader();
        reader.onload = function (e) {
            // 2. Check Resolution (Max 4K: 3840x2160)
            const img = new Image();
            img.onload = function () {
                if (this.width > 3840 || this.height > 2160) {
                    showToast(t('devices.toast.resolution_too_high') || `解析度過高: ${this.width}x${this.height} (最大 3840x2160)`, 'warning');
                    input.value = '';
                    document.getElementById('image-preview').style.display = 'none';
                    document.querySelector('.image-upload .icon').style.display = 'block';
                    document.querySelector('.image-upload p').style.display = 'block';
                } else {
                    // Valid
                    const preview = document.getElementById('image-preview');
                    preview.src = e.target.result;
                    preview.style.display = 'block';
                    document.querySelector('.image-upload .icon').style.display = 'none';
                    document.querySelector('.image-upload p').style.display = 'none';
                }
            };
            img.src = e.target.result;
        }
        reader.readAsDataURL(file);
    }
}

async function submitDeviceImage(id) {
    const input = document.getElementById('image-input');
    if (!input.files || !input.files[0]) {
        showToast(t('devices.toast.select_image'), 'warning');
        return;
    }

    const formData = new FormData();
    formData.append('image', input.files[0]);

    try {
        const response = await apiUpload(`/devices/${id}/image`, formData);
        if (response.success) {
            showToast(t('devices.toast.image_upload_success'), 'success');
            hideModal();
            loadDevices();
        }
    } catch (error) {
        showToast(error.message || t('devices.toast.upload_failed'), 'error');
    }
}

// ===== 刪除設備 =====
function confirmDeleteDevice(id, name) {
    console.log('[Devices] confirmDeleteDevice called. ID:', id, 'Name:', name);
    if (!id) {
        console.error('[Devices] Error: ID is missing or invalid:', id);
        showToast(t('devices.toast.invalid_device_id'), 'error');
        return;
    }

    showConfirm(`確定要刪除設備 "${name}" (ID: ${id}) 嗎？`, async () => {
        console.log('[Devices] Executing delete callback for ID:', id);
        try {
            const url = `/devices/${id}`;
            console.log('[Devices] Calling apiDelete with URL:', url);
            const response = await apiDelete(url);
            if (response.success) {
                showToast(t('devices.toast.delete_success'), 'success');
                loadDevices();
            }
        } catch (error) {
            console.error('[Devices] Delete failed:', error);
            showToast('刪除失敗: ' + error.message, 'error');
        }
    });
}


// ===== 批量刪除功能 =====
// selectedDeviceIds is managed globally or at top

// Better to put it at top of file, but let's just ensure it is defined before usage.
// Actually, `loadDevices` uses it. So it should be defined at global scope.
// I'll put it at the very top of `devices.js` in a future edit or assume it's okay if `loadDevices` is called after this file is parsed.
// But `loadDevices` is async and called by app init.
// Let's rely on hoisting of functions, but `selectedDeviceIds` needs to be defined.
// The previous file had it inside `loadDevices` (bug) and global?
// Let's define it outside.

function toggleSelectAll() {
    const selectAllCheckbox = document.getElementById('select-all-devices');
    const checkboxes = document.querySelectorAll('.device-checkbox');
    const isChecked = selectAllCheckbox.checked;

    checkboxes.forEach(cb => {
        cb.checked = isChecked;
        const id = parseInt(cb.value);
        if (isChecked) {
            selectedDeviceIds.add(id);
        } else {
            selectedDeviceIds.delete(id);
        }
    });
    updateBulkDeleteButtonState();
}

function toggleDeviceSelection(id) {
    const checkbox = document.querySelector(`.device-checkbox[value="${id}"]`);
    if (checkbox.checked) {
        selectedDeviceIds.add(id);
    } else {
        selectedDeviceIds.delete(id);
    }
    updateBulkDeleteButtonState();

    // 更新全選 checkbox 狀態
    const selectAllCheckbox = document.getElementById('select-all-devices');
    const checkboxes = document.querySelectorAll('.device-checkbox');
    selectAllCheckbox.checked = checkboxes.length > 0 && selectedDeviceIds.size === checkboxes.length;
}

function updateBulkDeleteButtonState() {
    const deleteBtn = document.getElementById('btn-bulk-delete');
    const editBtn = document.getElementById('btn-bulk-edit');
    const count = selectedDeviceIds.size;

    if (deleteBtn) {
        deleteBtn.innerHTML = `<span>🗑️</span> 批量刪除 (${count})`;
        deleteBtn.style.display = count > 0 ? 'inline-block' : 'none';
    }

    if (editBtn) {
        editBtn.innerHTML = `<span>✏️</span> 批量編輯 (${count})`;
        editBtn.style.display = count > 0 ? 'inline-block' : 'none';
    }
}

function bulkDeleteDevices() {
    if (selectedDeviceIds.size === 0) return;

    showConfirm(`確定要刪除選取的 ${selectedDeviceIds.size} 個設備嗎？此操作無法復原。`, async () => {
        try {
            const response = await apiPost('/devices/bulk-delete', { ids: Array.from(selectedDeviceIds) });
            if (response.success) {
                showToast(`成功刪除 ${selectedDeviceIds.size} 個設備`, 'success');
                selectedDeviceIds.clear();
                updateBulkDeleteButtonState();
                const selectAll = document.getElementById('select-all-devices');
                if (selectAll) selectAll.checked = false;
                loadDevices();
            }
        } catch (error) {
            showToast('刪除失敗: ' + error.message, 'error');
        }
    });
}

function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}


// Switch device detail tabs
function switchDeviceDetailTab(tabName, deviceId) {
    // Update active tab button
    document.querySelectorAll('.detail-tab-btn').forEach(btn => btn.classList.remove('active'));
    if (event && event.target) event.target.classList.add('active');

    // Update active tab content
    document.querySelectorAll('.device-detail-tab-content').forEach(tc => tc.classList.remove('active'));
    const tabContent = document.getElementById('device-' + tabName + '-tab');
    if (tabContent) tabContent.classList.add('active');

    // Load logs if switching to logs tab
    if (tabName === 'logs' && deviceId) {
        loadDeviceEvents(deviceId);
    }

    // Load PoE if switching to poe tab
    if (tabName === 'poe' && deviceId) {
        loadPoEInfo(deviceId);
    }

    // Load Backup history if switching to backups tab
    if (tabName === 'backups' && deviceId) {
        loadBackupHistory(deviceId);
    }

    // Load Wireless if switching to wireless tab
    if (tabName === 'wireless' && deviceId) {
        loadWirelessInfo(deviceId);
    }
}

// Show Port Tooltip
function showPortTooltip(deviceId, portIndex, displayNum, e) {
    if (!window.currentDeviceInterfaces) return;

    // Use merged interfaces as single source of truth
    let port = window.currentDeviceInterfaces.find(i => i.if_index === portIndex || i.index === portIndex);
    if (!port) return;

    // Remove existing tooltip
    const oldTooltip = document.querySelector('.port-tooltip');
    if (oldTooltip) oldTooltip.remove();

    const tooltip = document.createElement('div');
    tooltip.className = 'port-tooltip';

    const statusText = port.oper_status === 1 ? 'UP' : (port.if_status === 'up' ? 'UP' : 'DOWN');
    const statusClass = statusText === 'UP' ? 'active' : 'inactive';

    // VLAN logic
    const vlanHtml = port.vlan ? `<div class="detail-row"><span>VLAN:</span> <span style="color:${getVlanColor(port.vlan)}">ID ${port.vlan}</span></div>` : '';

    // PoE Controls inside tooltip (editor+ only)
    let poeControls = '';
    if (port.poe) {
        const admin = port.poe_admin; // 1=on, 2=off
        const powerW = port.poe_power_mw ? (port.poe_power_mw / 1000).toFixed(1) : '0.0';
        poeControls = `
                        <div class="poe-tooltip-section">
                            <div class="detail-row"><span>PoE:</span> <span>${port.poe_status_text || 'Active'} (${powerW} W)</span></div>
                            ${!isViewer() ? `<div class="tooltip-actions">
                                <button onclick="handlePoEControl(${deviceId}, ${port.index}, '${admin === 1 ? 'off' : 'on'}')" class="btn-xs ${admin === 1 ? 'warn' : 'success'}">${admin === 1 ? 'OFF' : 'ON'}</button>
                                <button onclick="handlePoEControl(${deviceId}, ${port.index}, 'recycle')" class="btn-xs primary">RST</button>
                            </div>` : ''}
                        </div>
                        `;
    }

    const inRate = Number(port.bandwidth_in || 0) * 8;
    const outRate = Number(port.bandwidth_out || 0) * 8;
    const avgRate = inRate + outRate;

    tooltip.innerHTML = `
                        <div class="tooltip-header">Port ${displayNum} (Index: ${portIndex})</div>
                        <div class="detail-row"><span>描述:</span> <span>${port.descr || port.if_name || port.if_desc || '-'}</span></div>
                        <div class="detail-row"><span>狀態:</span> <span class="status-badge ${statusClass}">${statusText}</span></div>
                        <div class="detail-row"><span>實體速率:</span> <span>${formatSpeed(port.speed || port.if_speed)}</span></div>
                        <div class="detail-row"><span>即時負載:</span> <span style="color: var(--primary-color); font-weight: bold;">${formatSpeed(avgRate)}</span></div>
                        <div class="detail-row" style="margin-top: 4px; border-top: 1px dashed #444; padding-top: 4px;">
                            <div style="display: flex; justify-content: space-between; width: 100%; font-size: 11px;">
                                <span>IN: ${formatSpeed(inRate)}</span>
                                <span>OUT: ${formatSpeed(outRate)}</span>
                            </div>
                        </div>
                        ${vlanHtml}
                        ${poeControls}
                        `;

    document.body.appendChild(tooltip);

    // Position tooltip
    const rect = e.target.closest('.port-icon').getBoundingClientRect();
    const tooltipX = rect.left + rect.width / 2 - 100; // rough width estimate
    const tooltipY = rect.top - 140; // rough height estimate

    tooltip.style.left = Math.max(10, tooltipX) + 'px';
    tooltip.style.top = Math.max(10, tooltipY) + 'px';
    tooltip.style.width = '200px';

    // Auto remove
    const removeHandler = () => {
        if (tooltip.parentNode) tooltip.remove();
        document.removeEventListener('mousedown', removeHandler);
    };

    setTimeout(() => {
        document.addEventListener('mousedown', removeHandler);
    }, 500);

    setTimeout(() => {
        if (tooltip.parentNode) tooltip.remove();
    }, 5000);
}

// Load device events for logs tab
async function loadDeviceEvents(deviceId) {
    const container = document.getElementById('device-events-container');
    if (!container) return;

    container.innerHTML = '<p class="loading-text">載入中...</p>';

    try {
        const response = await apiGet('/devices/' + deviceId + '/events');
        if (response.success && response.data && response.data.length > 0) {
            let html = '<table class="interface-table"><thead><tr><th>時間</th><th>事件類型</th><th>嚴重性</th><th>訊息</th></tr></thead><tbody>';
            for (const event of response.data) {
                html += '<tr>';
                html += '<td>' + formatDateTime(event.created_at) + '</td>';
                html += '<td>' + escapeHtml(event.event_type) + '</td>';
                html += '<td><span class="severity-badge ' + event.severity + '">' + event.severity + '</span></td>';
                html += '<td>' + escapeHtml(event.message) + '</td>';
                html += '</tr>';
            }
            html += '</tbody></table>';
            container.innerHTML = html;
        } else {
            container.innerHTML = '<p class="empty-message">尚無設備日誌</p>';
        }
    } catch (error) {
        container.innerHTML = '<p class="error-message">載入失敗: ' + (error.message || '未知錯誤') + '</p>';
    }
}

// ===== 批量編輯功能 =====
function showBulkEditModal() {
    if (selectedDeviceIds.size === 0) {
        showToast('請先選擇設備', 'warning');
        return;
    }

    const content = `
        <form id="bulk-edit-form" onsubmit="executeBulkEdit(event)">
            <p class="text-muted">正在編輯 ${selectedDeviceIds.size} 台設備。未填寫的欄位將保持原樣。</p>
            
            <div class="form-group">
                <label>品牌名稱 (Vendor)</label>
                <input type="text" name="vendor" placeholder="不變更請留空">
            </div>

            <div class="form-group">
                <label>設備型號 (Model)</label>
                <input type="text" name="model" placeholder="不變更請留空">
            </div>
            
            <div class="form-group">
                <label>設備類型 (Device Type)</label>
                <select name="device_type">
                    <option value="">不變更</option>
                    <option value="other" data-i18n="devices.type_other">其他</option>
                    <option value="router" data-i18n="devices.type_router">路由器</option>
                    <option value="switch" data-i18n="devices.type_switch">交換器</option>
                    <option value="access_point" data-i18n="devices.type_ap">無線存取點 (AP)</option>
                    <option value="firewall" data-i18n="devices.type_firewall">防火牆</option>
                    <option value="server" data-i18n="devices.type_server">伺服器</option>
                    <option value="codec" data-i18n="devices.type_codec">編解碼器 (Codec)</option>
                    <option value="ipcam" data-i18n="devices.type_ipcam">攝影機 (IPCAM)</option>
                    <option value="video_wall" data-i18n="devices.type_video_wall">電視牆控制器</option>
                    <option value="access_control" data-i18n="devices.type_access_control">門禁系統</option>
                    <option value="ups" data-i18n="devices.type_ups">UPS 不斷電系統</option>
                    <option value="pdu" data-i18n="devices.type_pdu">PDU 電源分配器</option>
                </select>
            </div>

            <div class="form-group advanced-only">
                <label>統一設備圖片 (可選)</label>
                <div class="image-upload" id="bulk-upload-drop-zone" onclick="document.getElementById('bulk-image-input').click()" style="padding: 20px; min-height: 100px; position: relative;">
                    <input type="file" id="bulk-image-input" name="image" accept="image/*" onchange="previewBulkImage(this)">
                    <div class="icon">📷</div>
                    <p>點擊或拖曳圖片到此處上傳</p>
                    <img id="bulk-image-preview" class="image-preview" style="display:none; max-height: 100px;">
                    <small style="display:block; color: var(--danger-color); font-weight: bold; font-size: 14px; margin-top: 12px;">建議解析度: 512x512px (正方形 PNG/JPG)</small>
                </div>
            </div>

            <div class="form-actions">
                <button type="button" class="btn btn-secondary" onclick="hideModal()">取消</button>
                <button type="submit" class="btn btn-primary">確認更新</button>
            </div>
        </form>
    `;

    openModal('modals.bulk_edit', content);

    // Setup drag and drop events after modal is open
    setTimeout(() => {
        const dropZone = document.getElementById('bulk-upload-drop-zone');
        if (dropZone) {
            ['dragenter', 'dragover', 'dragleave', 'drop'].forEach(eventName => {
                dropZone.addEventListener(eventName, preventDefaults, false);
            });

            ['dragenter', 'dragover'].forEach(eventName => {
                dropZone.addEventListener(eventName, highlight, false);
            });

            ['dragleave', 'drop'].forEach(eventName => {
                dropZone.addEventListener(eventName, unhighlight, false);
            });

            dropZone.addEventListener('drop', handleDrop, false);
        }
    }, 100);

    function preventDefaults(e) {
        e.preventDefault();
        e.stopPropagation();
    }

    function highlight(e) {
        const dropZone = document.getElementById('bulk-upload-drop-zone');
        if (dropZone) {
            dropZone.classList.add('highlight');
            dropZone.style.borderColor = 'var(--primary-color)';
            dropZone.style.backgroundColor = 'var(--bg-tertiary)';
        }
    }

    function unhighlight(e) {
        const dropZone = document.getElementById('bulk-upload-drop-zone');
        if (dropZone) {
            dropZone.classList.remove('highlight');
            dropZone.style.borderColor = 'var(--border-color)';
            dropZone.style.backgroundColor = '';
        }
    }

    function handleDrop(e) {
        const dt = e.dataTransfer;
        const files = dt.files;
        const input = document.getElementById('bulk-image-input');

        if (files && files.length > 0) {
            input.files = files; // Assign dropped files to input
            previewBulkImage(input);
        }
    }
}

function previewBulkImage(input) {
    if (input.files && input.files[0]) {
        const file = input.files[0];

        // 1. Check File Size (Max 5MB)
        if (file.size > 5 * 1024 * 1024) {
            showToast(t('devices.toast.image_too_large') || '圖片過大 (最大 5MB)', 'warning');
            input.value = '';
            return;
        }

        const reader = new FileReader();
        reader.onload = function (e) {
            // 2. Check Resolution (Max 4K, Recommended: 512x512)
            const img = new Image();
            img.onload = function () {
                if (this.width > 3840 || this.height > 2160) {
                    showToast(t('devices.toast.resolution_too_high') || `解析度過高: ${this.width}x${this.height} (最大 3840x2160)`, 'warning');
                    input.value = '';
                    document.getElementById('bulk-image-preview').style.display = 'none';
                    document.querySelector('#bulk-upload-drop-zone .icon').style.display = 'block';
                    document.querySelector('#bulk-upload-drop-zone p').style.display = 'block';
                } else {
                    // Valid
                    const preview = document.getElementById('bulk-image-preview');
                    preview.src = e.target.result;
                    preview.style.display = 'block';
                    document.querySelector('#bulk-upload-drop-zone .icon').style.display = 'none';
                    document.querySelector('#bulk-upload-drop-zone p').style.display = 'none';
                }
            };
            img.src = e.target.result;
        }
        reader.readAsDataURL(file);
    }
}

async function executeBulkEdit(event) {
    event.preventDefault();
    const btn = event.target.querySelector('button[type="submit"]');
    const originalText = btn.innerHTML;
    btn.disabled = true;
    btn.innerHTML = '更新中...';

    const formData = new FormData(event.target);
    const ids = Array.from(selectedDeviceIds).join(',');
    formData.append('ids', ids);

    try {
        const response = await apiUpload('/devices/bulk-update', formData);
        if (response.success) {
            showToast(response.message, 'success');
            hideModal();
            selectedDeviceIds.clear(); // Clear selection after update
            loadDevices();
        }
    } catch (error) {
        showToast(error.message || '批量更新失敗', 'error');
        btn.disabled = false;
        btn.innerHTML = originalText;
    }
}

/**
 * Renders a semi-circle gauge for usage metrics
 * @param {number} value - The percentage value (0-100)
 * @param {string} label - The label for the center (e.g. 'CPU')
 * @param {string} color - The stroke color
 * @returns {string} - HTML string for the SVG gauge
 */

// ============================================================
// 網段分群 UI  v1.0  (subnet /24 grouping for device list)
// ============================================================

let _subnetGroupActive = (localStorage.getItem('deviceSubnetGroup') === '1');
const _subnetCollapsed = new Set(JSON.parse(localStorage.getItem('deviceSubnetCollapsed') || '[]'));

function _subnetPrefix(ip) {
    if (!ip) return '未知網段';
    const parts = ip.split('.');
    if (parts.length < 3) return ip;
    return parts[0] + '.' + parts[1] + '.' + parts[2] + '.0/24';
}

function toggleSubnetGroupMode() {
    _subnetGroupActive = !_subnetGroupActive;
    localStorage.setItem('deviceSubnetGroup', _subnetGroupActive ? '1' : '0');
    const btn = document.getElementById('btn-subnet-group');
    if (btn) btn.classList.toggle('active', _subnetGroupActive);

    const tableWrap = document.querySelector('.devices-table-container');
    const subnetView = document.getElementById('devices-subnet-view');
    const pagination = document.getElementById('devices-pagination');
    if (_subnetGroupActive) {
        if (tableWrap) tableWrap.style.display = 'none';
        if (pagination) pagination.style.display = 'none';
        if (subnetView) subnetView.style.display = '';
        renderDevicesBySubnet(_lastDevices);
    } else {
        if (tableWrap) tableWrap.style.display = '';
        if (pagination) pagination.style.display = '';
        if (subnetView) { subnetView.style.display = 'none'; subnetView.innerHTML = ''; }
        // Re-render table with cached data
        const tbody = document.getElementById('devices-tbody');
        if (tbody && _lastDevices.length) {
            _subnetGroupActive = false;
            renderDevices(_lastDevices);
        }
    }
}

function toggleSubnetCollapse(prefix) {
    if (_subnetCollapsed.has(prefix)) {
        _subnetCollapsed.delete(prefix);
    } else {
        _subnetCollapsed.add(prefix);
    }
    localStorage.setItem('deviceSubnetCollapsed', JSON.stringify([..._subnetCollapsed]));
    renderDevicesBySubnet(_lastDevices);
}

function renderDevicesBySubnet(devices) {
    const view = document.getElementById('devices-subnet-view');
    if (!view) return;

    // Build groups
    const groups = new Map(); // prefix → devices[]
    for (const dev of (devices || [])) {
        const prefix = _subnetPrefix(dev.ip_address);
        if (!groups.has(prefix)) groups.set(prefix, []);
        groups.get(prefix).push(dev);
    }
    // Sort groups by prefix
    const sortedPrefixes = [...groups.keys()].sort();

    if (sortedPrefixes.length === 0) {
        view.innerHTML = '<p class="empty-message" style="padding:32px;text-align:center;">尚無設備資料</p>';
        return;
    }

    let html = '<div class="subnet-group-list">';
    for (const prefix of sortedPrefixes) {
        const devs = groups.get(prefix);
        const total = devs.length;
        const offline = devs.filter(d => !d.is_online).length;
        const online = total - offline;
        const collapsed = _subnetCollapsed.has(prefix);
        const hasAlert = offline > 0;

        html += `
        <div class="subnet-group-section${hasAlert ? ' has-alert' : ''}">
            <div class="subnet-group-header" onclick="toggleSubnetCollapse(${JSON.stringify(prefix)})">
                <span class="subnet-collapse-icon">${collapsed ? '▶' : '▼'}</span>
                <span class="subnet-prefix">${escapeHtml(prefix)}</span>
                <span class="subnet-stats">
                    <span class="subnet-stat online">${online} 線上</span>
                    ${offline > 0 ? `<span class="subnet-stat offline">${offline} 離線</span>` : ''}
                    <span class="subnet-stat total">${total} 台</span>
                </span>
                ${hasAlert ? '<span class="subnet-alert-dot" title="有設備離線">●</span>' : ''}
            </div>
            ${collapsed ? '' : `<div class="subnet-group-body">
                <table class="data-table subnet-inner-table">
                    <thead><tr>
                        <th>狀態</th><th>圖示</th><th>名稱</th><th>品牌</th>
                        <th>IP 位址</th><th>類型 / 監控</th><th>最後連線</th><th>操作</th>
                    </tr></thead>
                    <tbody>
                        ${devs.map(device => {
                            const statusClass = device.is_online ? 'online' : 'offline';
                            const statusText = device.is_online ? (t('devices.online')||'線上') : (t('devices.offline')||'離線');
                            const isSnmp = (device.snmp_community || device.snmp_version === 3);
                            const monitorType = device.snmp_version === 3 ? 'SNMP v3' : (device.snmp_community ? 'SNMP' : 'Ping');
                            const monitorClass = isSnmp ? 'snmp' : 'ping';
                            const imageHtml = getDeviceIcon(device);
                            const vendor = device.vendor || device.manufacturer || '-';
                            return `<tr>
                                <td><div class="status-indicator"><span class="dot ${statusClass}"></span><span>${statusText}</span></div></td>
                                <td>${imageHtml}</td>
                                <td><strong>${escapeHtml(getDeviceDisplayName(device))}</strong></td>
                                <td>${escapeHtml(vendor)}</td>
                                <td><code>${escapeHtml(device.ip_address)}</code></td>
                                <td>
                                    <span class="device-type-badge ${device.device_type}">${getDeviceTypeIcon(device.device_type)} ${device.device_type}</span>
                                    <span class="monitor-badge ${monitorClass}">${monitorType}</span>
                                </td>
                                <td>${formatRelativeTime(device.last_seen)}</td>
                                <td><div class="action-buttons">
                                    <button class="action-btn" onclick="viewDevice(${device.id})" title="檢視">👁️</button>
                                    ${!isViewer() ? `<button class="action-btn" onclick="openWebSSH(${device.id}, '${escapeHtml(device.ip_address)}')" title="SSH 終端機">>_</button>` : ''}
                                    ${!isViewer() ? `<button class="action-btn" onclick="editDevice(${device.id})" title="編輯">✏️</button>` : ''}
                                    ${!isViewer() ? `<button class="action-btn danger" onclick="confirmDeleteDevice(${device.id}, '${escapeHtml(getDeviceDisplayName(device))}')" title="刪除">🗑️</button>` : ''}
                                </div></td>
                            </tr>`;
                        }).join('')}
                    </tbody>
                </table>
            </div>`}
        </div>`;
    }
    html += '</div>';
    view.innerHTML = html;
}

// Apply subnet group mode on page load if it was previously active
document.addEventListener('DOMContentLoaded', () => {
    if (_subnetGroupActive) {
        const btn = document.getElementById('btn-subnet-group');
        if (btn) btn.classList.add('active');
        const tableWrap = document.querySelector('.devices-table-container');
        const pagination = document.getElementById('devices-pagination');
        const subnetView = document.getElementById('devices-subnet-view');
        if (tableWrap) tableWrap.style.display = 'none';
        if (pagination) pagination.style.display = 'none';
        if (subnetView) subnetView.style.display = '';
    }
});

// ============================================================
// WebSSH Terminal  v1.0  (xterm.js + WebSocket relay)
// ============================================================

let _wsshSocket = null;
let _wsshTerm = null;
let _wsshResizeObs = null;

function openWebSSH(deviceId, deviceIp) {
    openModal(
        `SSH 終端機 \u2014 ${escapeHtml(String(deviceIp))}`,
        `<div class="form-group" style="margin-bottom:10px;">
            <label style="font-size:0.85rem;">使用者名稱</label>
            <input type="text" id="wssh-user" class="input-field" value="admin" autocomplete="off">
        </div>
        <div class="form-group" style="margin-bottom:10px;">
            <label style="font-size:0.85rem;">密碼</label>
            <input type="password" id="wssh-pass" class="input-field" autocomplete="off">
        </div>
        <div class="form-group" style="margin-bottom:16px;">
            <label style="font-size:0.85rem;">Port</label>
            <input type="number" id="wssh-port" class="input-field" value="22" style="max-width:100px;">
        </div>
        <div style="display:flex;justify-content:flex-end;gap:10px;">
            <button type="button" class="btn btn-secondary" onclick="hideModal()">取消</button>
            <button type="button" class="btn btn-primary" id="wssh-connect-btn">連線</button>
        </div>`,
        { wide: false }
    );
    setTimeout(() => {
        const u = document.getElementById('wssh-user');
        if (u) u.focus();
        const btn = document.getElementById('wssh-connect-btn');
        if (btn) btn.onclick = () => _startWebSSH(deviceId, deviceIp);
    }, 50);
}

function _startWebSSH(deviceId, deviceIp) {
    const username = (document.getElementById('wssh-user') || {}).value || 'admin';
    const password = (document.getElementById('wssh-pass') || {}).value || '';
    const port     = (document.getElementById('wssh-port') || {}).value || '22';
    hideModal();
    _ensureXterm(() => _openTerminalModal(deviceId, deviceIp, username, password, port));
}

function _ensureXterm(cb) {
    if (window.Terminal) { cb(); return; }
    if (!document.getElementById('xterm-css')) {
        const link = document.createElement('link');
        link.id = 'xterm-css';
        link.rel = 'stylesheet';
        link.href = 'https://cdn.jsdelivr.net/npm/xterm@5.3.0/css/xterm.min.css';
        document.head.appendChild(link);
    }
    const script = document.createElement('script');
    script.src = 'https://cdn.jsdelivr.net/npm/xterm@5.3.0/lib/xterm.min.js';
    script.onload = () => {
        const fit = document.createElement('script');
        fit.src = 'https://cdn.jsdelivr.net/npm/xterm-addon-fit@0.8.0/lib/xterm-addon-fit.min.js';
        fit.onload = cb;
        document.head.appendChild(fit);
    };
    document.head.appendChild(script);
}

function _openTerminalModal(deviceId, deviceIp, username, password, port) {
    let overlay = document.getElementById('wssh-overlay');
    if (!overlay) {
        overlay = document.createElement('div');
        overlay.id = 'wssh-overlay';
        overlay.style.cssText = 'position:fixed;inset:0;z-index:99998;background:#1a1a2e;display:flex;flex-direction:column;';
        overlay.innerHTML =
            '<div style="display:flex;align-items:center;padding:8px 16px;background:#111;border-bottom:1px solid #333;gap:12px;">' +
                '<span style="font-family:monospace;color:#6366f1;font-weight:600;">&gt;_ SSH</span>' +
                '<span id="wssh-title" style="color:#aaa;font-size:0.85rem;flex:1;"></span>' +
                '<span id="wssh-status" style="font-size:0.78rem;padding:2px 10px;border-radius:10px;background:#333;color:#aaa;">連線中\u2026</span>' +
                '<button onclick="closeWebSSH()" style="background:#e74c3c;border:none;color:#fff;padding:4px 12px;border-radius:4px;cursor:pointer;">\u2715 關閉</button>' +
            '</div>' +
            '<div id="wssh-term-container" style="flex:1;padding:8px;overflow:hidden;"></div>';
        document.body.appendChild(overlay);
    }
    overlay.style.display = 'flex';
    document.getElementById('wssh-title').textContent = username + '@' + deviceIp + ':' + port;
    const statusEl = document.getElementById('wssh-status');
    statusEl.textContent = '連線中\u2026';
    statusEl.style.background = '#333';
    statusEl.style.color = '#aaa';

    const container = document.getElementById('wssh-term-container');
    container.innerHTML = '';

    if (_wsshTerm) { try { _wsshTerm.dispose(); } catch(e){} _wsshTerm = null; }
    if (_wsshSocket) { try { _wsshSocket.close(); } catch(e){} _wsshSocket = null; }
    if (_wsshResizeObs) { _wsshResizeObs.disconnect(); _wsshResizeObs = null; }

    const term = new window.Terminal({
        fontFamily: '"Cascadia Code","Fira Code",monospace',
        fontSize: 14,
        theme: { background: '#1a1a2e', foreground: '#e0e0e0', cursor: '#6366f1' },
        cursorBlink: true,
        scrollback: 2000,
    });
    _wsshTerm = term;

    let fitAddon = null;
    if (window.FitAddon) {
        fitAddon = new window.FitAddon.FitAddon();
        term.loadAddon(fitAddon);
    }
    term.open(container);
    if (fitAddon) fitAddon.fit();

    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    const token = sessionStorage.getItem('nms_token') || localStorage.getItem('nms_token') || '';
    const params = new URLSearchParams({ username: username, password: password, port: port });
    if (token) params.set('token', token);
    const wsUrl = proto + '://' + location.host + '/api/v1/devices/' + deviceId + '/terminal?' + params.toString();

    const ws = new WebSocket(wsUrl);
    _wsshSocket = ws;
    ws.binaryType = 'arraybuffer';

    ws.onopen = function() {
        statusEl.textContent = '已連線';
        statusEl.style.background = '#10b981';
        statusEl.style.color = '#fff';
        if (fitAddon) fitAddon.fit();
        _sendResize(ws, term);
    };
    ws.onmessage = function(evt) {
        if (evt.data instanceof ArrayBuffer) {
            term.write(new Uint8Array(evt.data));
        } else {
            term.write(evt.data);
        }
    };
    ws.onerror = function() {
        term.write('\r\n\x1b[31m[WebSSH] 連線錯誤\x1b[0m\r\n');
        statusEl.textContent = '錯誤'; statusEl.style.background = '#e74c3c'; statusEl.style.color = '#fff';
    };
    ws.onclose = function() {
        term.write('\r\n\x1b[33m[WebSSH] 連線已關閉\x1b[0m\r\n');
        statusEl.textContent = '已斷線'; statusEl.style.background = '#e74c3c'; statusEl.style.color = '#fff';
    };

    term.onData(function(data) {
        if (ws.readyState === WebSocket.OPEN) ws.send(data);
    });
    term.onResize(function(size) {
        _sendResize(ws, term, size.cols, size.rows);
    });

    const ro = new ResizeObserver(function() {
        if (fitAddon) { fitAddon.fit(); _sendResize(ws, term); }
    });
    ro.observe(container);
    _wsshResizeObs = ro;
}

function _sendResize(ws, term, cols, rows) {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(JSON.stringify({ type: 'resize', cols: cols || term.cols || 80, rows: rows || term.rows || 24 }));
}

window.closeWebSSH = function() {
    if (_wsshResizeObs) { _wsshResizeObs.disconnect(); _wsshResizeObs = null; }
    if (_wsshSocket) { try { _wsshSocket.close(); } catch(e){} _wsshSocket = null; }
    if (_wsshTerm) { try { _wsshTerm.dispose(); } catch(e){} _wsshTerm = null; }
    const ov = document.getElementById('wssh-overlay');
    if (ov) ov.style.display = 'none';
};
