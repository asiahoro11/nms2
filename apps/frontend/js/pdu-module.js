// Made by YTSworks
// YTS工作室製作
// ============================================================
// PDU/UPS Module  v1.2.1
// License key: pdu_enabled in system_config
// Supports: UPS (RFC 1628 + APC), PDU power strips
// ============================================================

let pduDevices = [];
let pduLicensed = false;

// ── Init ──────────────────────────────────────────────────────

async function pduInit() {
    await pduCheckStatus();
    await pduLoadDevices();
}
window.pduInit = pduInit;

async function pduCheckStatus() {
    try {
        const res = await apiGet('/pdu/status');
        pduLicensed = !!(res && res.success && res.data && res.data.enabled);
    } catch (e) {
        pduLicensed = false;
    }
    pduApplyLicenseUI();
}

function pduApplyLicenseUI() {
    const notice = document.getElementById('pdu-license-notice');
    const addBtn = document.getElementById('pdu-add-btn');
    if (typeof setModuleLock === 'function') {
        setModuleLock('pdu', !pduLicensed);
    }
    if (pduLicensed) {
        if (notice) notice.style.display = 'none';
        if (addBtn) { addBtn.disabled = false; addBtn.classList.remove('btn-disabled'); }
    } else {
        if (notice) notice.style.display = 'flex';
        if (addBtn) { addBtn.disabled = true;  addBtn.classList.add('btn-disabled'); }
    }
}

// ── Load & render ─────────────────────────────────────────────

async function pduLoadDevices() {
    if (!pduLicensed) { pduRender([]); return; }
    try {
        const res = await apiGet('/pdu/devices');
        pduDevices = (res && res.success && res.data) ? res.data : [];
    } catch (e) {
        pduDevices = [];
    }
    pduRender(pduDevices);
}
window.pduLoadDevices = pduLoadDevices;

function pduRender(devices) {
    const grid = document.getElementById('pdu-grid');
    if (!grid) return;

    if (!pduLicensed) {
        grid.innerHTML = '';
        return;
    }
    if (!devices || devices.length === 0) {
        grid.innerHTML = `<div class="pdu-empty"><div class="pdu-empty-icon">🔌</div>
            <div>${t('pdu.empty') || '尚無 PDU/UPS 裝置，請點擊「新增」'}</div></div>`;
        return;
    }

    grid.innerHTML = devices.map(d => pduRenderCard(d)).join('');
}

function pduRenderCard(d) {
    const icon = d.device_type === 'ups' ? '🔋' : '🔌';
    const statusLabel = pduStatusLabel(d.status);
    const statusCls   = d.status || 'unknown';
    const location    = d.location ? escPdu(d.location) : '—';
    const model       = d.model ? escPdu(d.model) : '';

    return `
    <div class="pdu-card" id="pdu-card-${d.id}">
        <div class="pdu-card-header">
            <div class="pdu-type-icon">${icon}</div>
            <div class="pdu-card-title">
                <h4>${escPdu(d.name)}</h4>
                <small>${escPdu(d.ip_address)}${model ? ' · ' + model : ''}</small><br>
                <small>${location}</small>
            </div>
            <span class="pdu-status-badge ${statusCls}">${statusLabel}</span>
        </div>
        <div class="pdu-metrics" id="pdu-metrics-${d.id}">
            <div class="pdu-metric"><div class="pdu-metric-label">${t('pdu.type') || '類型'}</div>
                <div class="pdu-metric-value">${d.device_type === 'ups' ? 'UPS' : 'PDU'}</div></div>
            <div class="pdu-metric"><div class="pdu-metric-label">${t('pdu.snmp_community') || 'Community'}</div>
                <div class="pdu-metric-value">${escPdu(d.snmp_community)}</div></div>
        </div>
        <div class="pdu-card-actions">
            <button class="btn btn-xs btn-secondary" onclick="pduPoll(${d.id})" title="${t('pdu.poll') || '立即查詢'}">
                🔄 ${t('pdu.poll') || '查詢'}
            </button>
            <button class="btn btn-xs btn-secondary" onclick="pduEdit(${d.id})" title="${t('common.edit') || '編輯'}">
                ⚙ ${t('common.edit') || '編輯'}
            </button>
            <button class="btn btn-xs btn-danger" onclick="pduDelete(${d.id})" title="${t('common.delete') || '刪除'}">
                🗑
            </button>
        </div>
    </div>`;
}

function pduStatusLabel(s) {
    const map = {
        normal:   t('pdu.status_normal')   || '正常',
        warning:  t('pdu.status_warning')  || '警告',
        critical: t('pdu.status_critical') || '嚴重',
        offline:  t('pdu.status_offline')  || '離線',
        unknown:  t('pdu.status_unknown')  || '未知',
    };
    return map[s] || s || '未知';
}

// ── Poll (on-demand SNMP query) ────────────────────────────────

window.pduPoll = async function(id) {
    const card = document.getElementById(`pdu-card-${id}`);
    if (card) card.style.opacity = '0.6';
    try {
        const res = await apiPost(`/pdu/devices/${id}/poll`, {});
        if (res && res.success && res.data) {
            const d = res.data;
            // Update card in place
            const idx = pduDevices.findIndex(x => x.id === id);
            if (idx >= 0) pduDevices[idx] = d;
            pduUpdateCard(d);
            showToast(`${d.name}: ${pduStatusLabel(d.status)}`, d.status === 'critical' ? 'error' : d.status === 'warning' ? 'warning' : 'success');
        } else {
            showToast((res && res.error) || t('pdu.poll_failed') || '查詢失敗', 'error');
        }
    } catch (e) {
        showToast(t('pdu.poll_failed') || '查詢失敗', 'error');
    } finally {
        if (card) card.style.opacity = '1';
    }
};

function pduUpdateCard(d) {
    const container = document.getElementById(`pdu-card-${d.id}`);
    if (!container) return;

    // Update status badge
    const badge = container.querySelector('.pdu-status-badge');
    if (badge) {
        badge.className = `pdu-status-badge ${d.status || 'unknown'}`;
        badge.textContent = pduStatusLabel(d.status);
    }

    // Update metrics
    const metrics = document.getElementById(`pdu-metrics-${d.id}`);
    if (!metrics) return;
    let html = '';

    if (d.battery_capacity_pct !== undefined && d.battery_capacity_pct !== null) {
        const pct = d.battery_capacity_pct;
        const barCls = pct >= 50 ? 'good' : pct >= 20 ? 'medium' : 'critical';
        html += `<div class="pdu-metric">
            <div class="pdu-metric-label">${t('pdu.battery_capacity') || '電池容量'}</div>
            <div class="pdu-metric-value">${pct}%</div>
            <div class="pdu-battery-bar"><div class="pdu-battery-fill ${barCls}" style="width:${pct}%"></div></div>
        </div>`;
    }
    if (d.battery_runtime_min !== undefined && d.battery_runtime_min !== null) {
        html += `<div class="pdu-metric">
            <div class="pdu-metric-label">${t('pdu.runtime') || '剩餘時間'}</div>
            <div class="pdu-metric-value">${d.battery_runtime_min} ${t('pdu.minutes') || '分鐘'}</div>
        </div>`;
    }
    if (d.battery_temp_c !== undefined && d.battery_temp_c !== null) {
        html += `<div class="pdu-metric">
            <div class="pdu-metric-label">${t('pdu.battery_temp') || '電池溫度'}</div>
            <div class="pdu-metric-value">${d.battery_temp_c.toFixed(1)} °C</div>
        </div>`;
    }
    if (d.input_voltage !== undefined && d.input_voltage !== null) {
        html += `<div class="pdu-metric">
            <div class="pdu-metric-label">${t('pdu.input_voltage') || '輸入電壓'}</div>
            <div class="pdu-metric-value">${d.input_voltage} V</div>
        </div>`;
    }
    if (d.output_load_pct !== undefined && d.output_load_pct !== null) {
        html += `<div class="pdu-metric">
            <div class="pdu-metric-label">${t('pdu.output_load') || '負載'}</div>
            <div class="pdu-metric-value">${d.output_load_pct}%</div>
        </div>`;
    }
    if (d.output_source) {
        html += `<div class="pdu-metric">
            <div class="pdu-metric-label">${t('pdu.output_source') || '輸出來源'}</div>
            <div class="pdu-metric-value">${pduOutputSourceLabel(d.output_source)}</div>
        </div>`;
    }
    if (d.alarms_present !== undefined && d.alarms_present !== null && d.alarms_present > 0) {
        html += `<div class="pdu-metric" style="grid-column:1/-1">
            <div class="pdu-metric-label" style="color:#ef4444;">${t('pdu.alarms') || '⚠ 告警'}</div>
            <div class="pdu-metric-value" style="color:#ef4444;">${d.alarms_present} ${t('pdu.alarms_count') || '個'}</div>
        </div>`;
    }

    if (!html) {
        html = `<div class="pdu-metric"><div class="pdu-metric-label">${t('pdu.type') || '類型'}</div>
            <div class="pdu-metric-value">${d.device_type === 'ups' ? 'UPS' : 'PDU'}</div></div>
            <div class="pdu-metric"><div class="pdu-metric-label">${t('pdu.snmp_community') || 'Community'}</div>
            <div class="pdu-metric-value">${escPdu(d.snmp_community)}</div></div>`;
    }
    metrics.innerHTML = html;
}

function pduOutputSourceLabel(s) {
    const map = {
        normal:  t('pdu.source_normal')  || '市電',
        bypass:  t('pdu.source_bypass')  || '旁路',
        battery: t('pdu.source_battery') || '電池',
        booster: t('pdu.source_booster') || '升壓',
        reducer: t('pdu.source_reducer') || '降壓',
    };
    return map[s] || s;
}

// ── Add/Edit modal ────────────────────────────────────────────

window.pduAdd = function() {
    pduShowModal(null);
};

window.pduEdit = async function(id) {
    const d = pduDevices.find(x => x.id === id);
    if (d) pduShowModal(d);
};

function pduShowModal(d) {
    const isEdit = !!d;
    const title  = isEdit ? (t('pdu.edit') || '編輯 PDU/UPS') : (t('pdu.add') || '新增 PDU/UPS');

    const content = `
    <form id="pdu-form" onsubmit="pduSave(event)">
        <div class="pdu-form-grid">
            <div class="form-group"><label>${t('pdu.name') || '名稱'} *</label>
                <input type="text" name="name" required class="input-field"
                    value="${isEdit ? escPdu(d.name) : ''}" placeholder="UPS-Server-Room"></div>
            <div class="form-group"><label>${t('pdu.location') || '位置'}</label>
                <input type="text" name="location" class="input-field"
                    value="${isEdit ? escPdu(d.location) : ''}" placeholder="機房 A"></div>
            <div class="form-group"><label>${t('pdu.ip_address') || 'IP 位址'} *</label>
                <input type="text" name="ip_address" required class="input-field"
                    value="${isEdit ? escPdu(d.ip_address) : ''}" placeholder="192.168.1.200"></div>
            <div class="form-group"><label>SNMP Port</label>
                <input type="number" name="port" class="input-field"
                    value="${isEdit ? d.port : 161}" placeholder="161"></div>
            <div class="form-group"><label>${t('pdu.snmp_community') || 'Community'}</label>
                <input type="text" name="snmp_community" class="input-field"
                    value="${isEdit ? escPdu(d.snmp_community) : 'public'}" placeholder="public"></div>
            <div class="form-group"><label>SNMP Version</label>
                <select name="snmp_version" class="input-field">
                    <option value="1" ${isEdit && d.snmp_version===1?'selected':''}>v1</option>
                    <option value="2" ${!isEdit||d.snmp_version===2?'selected':''}>v2c</option>
                </select></div>
            <div class="form-group"><label>${t('pdu.device_type') || '裝置類型'}</label>
                <select name="device_type" class="input-field">
                    <option value="ups"  ${!isEdit||d.device_type==='ups'?'selected':''}>UPS 不斷電系統</option>
                    <option value="pdu"  ${isEdit&&d.device_type==='pdu'?'selected':''}>PDU 電源分配器</option>
                </select></div>
            <div class="form-group"><label>${t('pdu.manufacturer') || '製造商'}</label>
                <input type="text" name="manufacturer" class="input-field"
                    value="${isEdit ? escPdu(d.manufacturer) : ''}" placeholder="APC / Eaton / CyberPower…"></div>
            <div class="form-group" style="grid-column:1/-1"><label>${t('pdu.model') || '型號'}</label>
                <input type="text" name="model" class="input-field"
                    value="${isEdit ? escPdu(d.model) : ''}" placeholder="SMX1500RMI2U…"></div>
        </div>
        <label style="display:flex;align-items:center;gap:8px;margin:12px 0;cursor:pointer;">
            <input type="checkbox" name="is_enabled" ${!isEdit||d.is_enabled?'checked':''}>
            <span>${t('pdu.is_enabled') || '啟用監控'}</span>
        </label>
        <input type="hidden" name="id" value="${isEdit ? d.id : ''}">
        <div style="display:flex;justify-content:flex-end;gap:10px;padding-top:12px;border-top:1px solid var(--border-color);">
            ${isEdit ? `<button type="button" class="btn btn-danger" onclick="pduDelete(${d.id})" style="margin-right:auto;">🗑 ${t('common.delete')||'刪除'}</button>` : ''}
            <button type="button" class="btn btn-secondary" onclick="hideModal()">${t('common.cancel')||'取消'}</button>
            <button type="submit" class="btn btn-primary">${isEdit ? t('common.save')||'儲存' : t('common.add')||'新增'}</button>
        </div>
    </form>`;

    if (typeof openModal === 'function') {
        openModal(title, content, { wide: true });
    } else {
        // fallback: inject into generic modal if openModal not available
        const overlay = document.getElementById('modal-overlay') || document.getElementById('modal-backdrop');
        const body = document.getElementById('modal-body') || document.getElementById('modal-content');
        const titleEl = document.getElementById('modal-title');
        if (body) body.innerHTML = content;
        if (titleEl) titleEl.textContent = title;
        if (overlay) overlay.style.display = 'flex';
    }
}

window.pduSave = async function(e) {
    e.preventDefault();
    const fd = new FormData(e.target);
    const payload = {
        name:          fd.get('name'),
        location:      fd.get('location') || '',
        ip_address:    fd.get('ip_address'),
        port:          parseInt(fd.get('port')) || 161,
        snmp_community: fd.get('snmp_community') || 'public',
        snmp_version:  parseInt(fd.get('snmp_version')) || 2,
        device_type:   fd.get('device_type') || 'ups',
        manufacturer:  fd.get('manufacturer') || '',
        model:         fd.get('model') || '',
        is_enabled:    fd.get('is_enabled') === 'on',
    };
    const id = fd.get('id');
    try {
        let res;
        if (id) {
            res = await apiPut(`/pdu/devices/${id}`, payload);
        } else {
            res = await apiPost('/pdu/devices', payload);
        }
        if (res && res.success) {
            hideModal();
            showToast(id ? (t('pdu.saved')||'已儲存') : (t('pdu.added')||'已新增'), 'success');
            await pduLoadDevices();
        } else {
            showToast((res && res.error) || t('common.error') || '操作失敗', 'error');
        }
    } catch (err) {
        showToast(err.message, 'error');
    }
};

window.pduDelete = async function(id) {
    const d = pduDevices.find(x => x.id === id);
    return showHighRiskConfirm('Delete PDU / UPS device', d ? d.name : String(id), async () => {
    hideModal();
    try {
        const res = await apiDelete(`/pdu/devices/${id}`);
        if (res && res.success) {
            showToast(t('pdu.deleted') || '已刪除', 'success');
            await pduLoadDevices();
        } else {
            showToast((res && res.error) || '刪除失敗', 'error');
        }
    } catch (e) {
        showToast(e.message, 'error');
    }
    });
};

// Escape HTML helper
function escPdu(s) {
    const d = document.createElement('div');
    d.textContent = s || '';
    return d.innerHTML;
}
