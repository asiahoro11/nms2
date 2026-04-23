/* ============================================================
   Access Control (門禁) Module — door-module.js
   ============================================================ */

'use strict';

// ── Modal helpers (operate ac-door-modal / ac-card-modal directly) ──
function acShowOverlay(id) {
    const el = document.getElementById(id);
    if (el) { el.style.display = 'flex'; document.body.style.overflow = 'hidden'; }
}
function acHideOverlay(id) {
    const el = document.getElementById(id);
    if (el) { el.style.display = 'none'; document.body.style.overflow = ''; }
}

// ── State ────────────────────────────────────────────────────
let acModuleEnabled = false;
let acDoors = [];
let acCards = [];
let acEvents = [];
let acActiveTab = 'doors'; // doors | cards | schedules | events
let acSchedules = [];

// ── Init ─────────────────────────────────────────────────────
async function acInit() {
    await acLoadStatus();
    acSwitchTab(acActiveTab);
}

async function acLoadStatus() {
    try {
        const res = await apiGet('/api/v1/access-control/status');
        if (res.success) {
            acModuleEnabled = res.data.enabled;
            document.getElementById('ac-license-notice').style.display = acModuleEnabled ? 'none' : 'flex';
            document.getElementById('ac-module-content').style.display = acModuleEnabled ? 'block' : 'none';
        }
    } catch (e) {
        console.error('[AC] status error', e);
    }
}

// ── Tab switching ─────────────────────────────────────────────
function acSwitchTab(tab) {
    acActiveTab = tab;
    document.querySelectorAll('.ac-tab-btn').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.tab === tab);
    });
    document.querySelectorAll('.ac-panel').forEach(p => {
        p.classList.toggle('active', p.id === 'ac-panel-' + tab);
    });
    if (!acModuleEnabled) return;
    if (tab === 'doors')     acLoadDoors();
    if (tab === 'cards')     acLoadCards();
    if (tab === 'schedules') acLoadSchedules();
    if (tab === 'events')    acLoadEvents();
}

// ── Doors ─────────────────────────────────────────────────────
async function acLoadDoors() {
    try {
        const res = await apiGet('/api/v1/access-control/doors');
        if (res.success) {
            acDoors = res.data;
            acRenderDoors();
        }
    } catch (e) { showToast(t('common.load_failed'), 'error'); }
}

function acRenderDoors() {
    const grid = document.getElementById('ac-door-grid');
    if (!grid) return;
    if (!acDoors.length) {
        grid.innerHTML = `<div class="empty-state">${t('ac.no_doors')}</div>`;
        return;
    }
    grid.innerHTML = acDoors.map(d => `
        <div class="door-card" data-id="${d.id}">
            <div class="door-card-header">
                <span class="door-card-name">🚪 ${escHtml(d.name)}</span>
                <span class="door-status-badge ${d.status}">${acStatusLabel(d.status)}</span>
            </div>
            <div class="door-card-location">📍 ${escHtml(d.location || '—')}</div>
            <div style="font-size:0.8rem;color:var(--text-secondary);">
                ${escHtml(d.ip_address || '—')}${d.port && d.port !== 80 ? ':'+d.port : ''}
                ${d.manufacturer ? ' · ' + escHtml(d.manufacturer) : ''}
                ${d.model ? ' ' + escHtml(d.model) : ''}
            </div>
            <div class="door-card-actions">
                <button class="door-action-btn open-btn" onclick="acDoorAction(${d.id},'open')">${t('ac.open_door')}</button>
                <button class="door-action-btn close-btn" onclick="acDoorAction(${d.id},'close')">${t('ac.close_door')}</button>
                <button class="door-action-btn edit-btn" onclick="acOpenEditDoorModal(${d.id})">✏️</button>
                <button class="door-action-btn delete-btn" onclick="acDeleteDoor(${d.id})">🗑️</button>
            </div>
        </div>`).join('');
}

function acStatusLabel(status) {
    const map = { open: t('ac.status_open'), close: t('ac.status_close'),
                  alarm: t('ac.status_alarm'), unknown: t('ac.status_unknown') };
    return map[status] || status;
}

async function acDoorAction(id, action) {
    try {
        const res = await apiPost(`/api/v1/access-control/doors/${id}/action`, { action });
        if (res.success) {
            showToast(`${t('ac.action_sent')}: ${action}`, 'success');
            setTimeout(acLoadDoors, 800);
        } else {
            showToast(res.error || t('common.error'), 'error');
        }
    } catch (e) { showToast(t('common.error'), 'error'); }
}

async function acDeleteDoor(id) {
    const door = acDoors.find(d => d.id === id);
    if (!confirm(t('ac.confirm_delete_door') + ' ' + (door?.name || ''))) return;
    try {
        const res = await apiDelete(`/api/v1/access-control/doors/${id}`);
        if (res.success) { showToast(t('ac.door_deleted'), 'success'); acLoadDoors(); }
        else showToast(res.error || t('common.error'), 'error');
    } catch (e) { showToast(t('common.error'), 'error'); }
}

// ── Door add/edit modal ───────────────────────────────────────
function acOpenAddDoorModal() {
    acShowDoorModal(null);
}
function acOpenEditDoorModal(id) {
    const door = acDoors.find(d => d.id === id);
    acShowDoorModal(door);
}

function acShowDoorModal(door) {
    const isEdit = !!door;
    const title = isEdit ? t('ac.edit_door') : t('ac.add_door');
    const html = `
        <div class="modal-header"><h3>${title}</h3></div>
        <div class="modal-body">
            <div class="ac-form-grid">
                <div class="form-group full-width">
                    <label>${t('ac.door_name')} *</label>
                    <input type="text" id="ac-d-name" class="form-control" value="${escHtml(door?.name||'')}" required>
                </div>
                <div class="form-group">
                    <label>${t('ac.location')}</label>
                    <input type="text" id="ac-d-location" class="form-control" value="${escHtml(door?.location||'')}">
                </div>
                <div class="form-group">
                    <label>${t('ac.ip_address')}</label>
                    <input type="text" id="ac-d-ip" class="form-control" value="${escHtml(door?.ip_address||'')}" placeholder="192.168.1.100">
                </div>
                <div class="form-group">
                    <label>${t('ac.port')}</label>
                    <input type="number" id="ac-d-port" class="form-control" value="${door?.port||80}">
                </div>
                <div class="form-group">
                    <label>${t('ac.protocol')}</label>
                    <select id="ac-d-protocol" class="form-control">
                        <option value="http"  ${(!door||door.protocol==='http')  ?'selected':''}>HTTP</option>
                        <option value="https" ${door?.protocol==='https'         ?'selected':''}>HTTPS</option>
                        <option value="tcp"   ${door?.protocol==='tcp'           ?'selected':''}>TCP</option>
                        <option value="rs485" ${door?.protocol==='rs485'         ?'selected':''}>RS-485</option>
                    </select>
                </div>
                <div class="form-group">
                    <label>${t('ac.username')}</label>
                    <input type="text" id="ac-d-user" class="form-control" value="${escHtml(door?.username||'')}">
                </div>
                <div class="form-group">
                    <label>${t('ac.password')}${isEdit ? ' ('+t('ac.leave_blank_to_keep')+')' : ''}</label>
                    <input type="password" id="ac-d-pass" class="form-control" placeholder="${isEdit?'••••••':''}">
                </div>
                <div class="form-group">
                    <label>${t('ac.manufacturer')}</label>
                    <input type="text" id="ac-d-mfr" class="form-control" value="${escHtml(door?.manufacturer||'')}">
                </div>
                <div class="form-group">
                    <label>${t('ac.model')}</label>
                    <input type="text" id="ac-d-model" class="form-control" value="${escHtml(door?.model||'')}">
                </div>
            </div>
        </div>
        <div class="modal-footer">
            <button class="btn btn-secondary" onclick="acHideOverlay('ac-door-modal')">${t('common.cancel')}</button>
            <button class="btn btn-primary" onclick="acSaveDoor(${door?.id||'null'})">${t('common.save')}</button>
        </div>`;
    document.getElementById('ac-door-modal-content').innerHTML = html;
    acShowOverlay('ac-door-modal');
}

async function acSaveDoor(id) {
    const payload = {
        name:         document.getElementById('ac-d-name').value.trim(),
        location:     document.getElementById('ac-d-location').value.trim(),
        ip_address:   document.getElementById('ac-d-ip').value.trim(),
        port:         parseInt(document.getElementById('ac-d-port').value) || 80,
        protocol:     document.getElementById('ac-d-protocol').value,
        username:     document.getElementById('ac-d-user').value.trim(),
        password:     document.getElementById('ac-d-pass').value,
        manufacturer: document.getElementById('ac-d-mfr').value.trim(),
        model:        document.getElementById('ac-d-model').value.trim(),
    };
    if (!payload.name) { showToast(t('ac.name_required'), 'error'); return; }
    try {
        const res = id
            ? await apiPut(`/api/v1/access-control/doors/${id}`, payload)
            : await apiPost('/api/v1/access-control/doors', payload);
        if (res.success) {
            showToast(id ? t('ac.door_updated') : t('ac.door_created'), 'success');
            acHideOverlay('ac-door-modal');
            acLoadDoors();
        } else {
            showToast(res.error || t('common.error'), 'error');
        }
    } catch (e) { showToast(t('common.error'), 'error'); }
}

// ── Cards ─────────────────────────────────────────────────────
async function acLoadCards(q = '') {
    try {
        const res = await apiGet('/api/v1/access-control/cards' + (q ? '?q=' + encodeURIComponent(q) : ''));
        if (res.success) {
            acCards = res.data;
            acRenderCards();
        }
    } catch (e) { showToast(t('common.load_failed'), 'error'); }
}

function acRenderCards() {
    const tbody = document.getElementById('ac-cards-tbody');
    if (!tbody) return;
    if (!acCards.length) {
        tbody.innerHTML = `<tr><td colspan="6" class="empty-state">${t('ac.no_cards')}</td></tr>`;
        return;
    }
    tbody.innerHTML = acCards.map(card => `
        <tr>
            <td>${escHtml(card.card_number)}</td>
            <td>${escHtml(card.holder_name)}</td>
            <td>${escHtml(card.department || '—')}</td>
            <td><span class="card-active-badge ${card.is_active ? 'active' : 'inactive'}">
                ${card.is_active ? t('ac.active') : t('ac.inactive')}
            </span></td>
            <td>${card.valid_until ? escHtml(card.valid_until) : '—'}</td>
            <td>
                <button class="btn btn-sm btn-secondary" onclick="acOpenEditCardModal(${card.id})">✏️</button>
                <button class="btn btn-sm btn-danger" onclick="acDeleteCard(${card.id})" style="margin-left:4px;">🗑️</button>
            </td>
        </tr>`).join('');
}

function acOpenAddCardModal() { acShowCardModal(null); }
function acOpenEditCardModal(id) {
    const card = acCards.find(c => c.id === id);
    acShowCardModal(card);
}

function acShowCardModal(card) {
    const isEdit = !!card;
    const html = `
        <div class="modal-header"><h3>${isEdit ? t('ac.edit_card') : t('ac.add_card')}</h3></div>
        <div class="modal-body">
            <div class="ac-form-grid">
                <div class="form-group">
                    <label>${t('ac.card_number')} *</label>
                    <input type="text" id="ac-c-num" class="form-control" value="${escHtml(card?.card_number||'')}" required>
                </div>
                <div class="form-group">
                    <label>${t('ac.holder_name')} *</label>
                    <input type="text" id="ac-c-holder" class="form-control" value="${escHtml(card?.holder_name||'')}" required>
                </div>
                <div class="form-group">
                    <label>${t('ac.department')}</label>
                    <input type="text" id="ac-c-dept" class="form-control" value="${escHtml(card?.department||'')}">
                </div>
                <div class="form-group">
                    <label>${t('ac.status')}</label>
                    <select id="ac-c-active" class="form-control">
                        <option value="1" ${card?.is_active !== false ? 'selected' : ''}>${t('ac.active')}</option>
                        <option value="0" ${card?.is_active === false ? 'selected' : ''}>${t('ac.inactive')}</option>
                    </select>
                </div>
                <div class="form-group">
                    <label>${t('ac.valid_from')}</label>
                    <input type="date" id="ac-c-from" class="form-control" value="${card?.valid_from||''}">
                </div>
                <div class="form-group">
                    <label>${t('ac.valid_until')}</label>
                    <input type="date" id="ac-c-until" class="form-control" value="${card?.valid_until||''}">
                </div>
            </div>
        </div>
        <div class="modal-footer">
            <button class="btn btn-secondary" onclick="acHideOverlay('ac-card-modal')">${t('common.cancel')}</button>
            <button class="btn btn-primary" onclick="acSaveCard(${card?.id||'null'})">${t('common.save')}</button>
        </div>`;
    document.getElementById('ac-card-modal-content').innerHTML = html;
    acShowOverlay('ac-card-modal');
}

async function acSaveCard(id) {
    const isActive = document.getElementById('ac-c-active').value === '1';
    const payload = {
        card_number: document.getElementById('ac-c-num').value.trim(),
        holder_name: document.getElementById('ac-c-holder').value.trim(),
        department:  document.getElementById('ac-c-dept').value.trim(),
        is_active:   isActive,
        valid_from:  document.getElementById('ac-c-from').value,
        valid_until: document.getElementById('ac-c-until').value,
    };
    if (!payload.card_number || !payload.holder_name) {
        showToast(t('ac.card_fields_required'), 'error'); return;
    }
    try {
        const res = id
            ? await apiPut(`/api/v1/access-control/cards/${id}`, payload)
            : await apiPost('/api/v1/access-control/cards', payload);
        if (res.success) {
            showToast(id ? t('ac.card_updated') : t('ac.card_created'), 'success');
            acHideOverlay('ac-card-modal');
            acLoadCards();
        } else {
            showToast(res.error || t('common.error'), 'error');
        }
    } catch (e) { showToast(t('common.error'), 'error'); }
}

async function acDeleteCard(id) {
    const card = acCards.find(c => c.id === id);
    if (!confirm(t('ac.confirm_delete_card') + ' ' + (card?.holder_name || ''))) return;
    try {
        const res = await apiDelete(`/api/v1/access-control/cards/${id}`);
        if (res.success) { showToast(t('ac.card_deleted'), 'success'); acLoadCards(); }
        else showToast(res.error || t('common.error'), 'error');
    } catch (e) { showToast(t('common.error'), 'error'); }
}

// ── Events ────────────────────────────────────────────────────
async function acLoadEvents() {
    const evType = document.getElementById('ac-ev-type-filter')?.value || '';
    const limit  = document.getElementById('ac-ev-limit')?.value || '100';
    try {
        let url = `/api/v1/access-control/events?limit=${limit}`;
        if (evType) url += `&event_type=${evType}`;
        const res = await apiGet(url);
        if (res.success) {
            acEvents = res.data;
            acRenderEvents();
        }
    } catch (e) { showToast(t('common.load_failed'), 'error'); }
}

function acRenderEvents() {
    const tbody = document.getElementById('ac-events-tbody');
    if (!tbody) return;
    if (!acEvents.length) {
        tbody.innerHTML = `<tr><td colspan="5" class="empty-state">${t('ac.no_events')}</td></tr>`;
        return;
    }
    tbody.innerHTML = acEvents.map(ev => `
        <tr>
            <td>${fmtDatetime(ev.occurred_at)}</td>
            <td>${escHtml(ev.door_name || '—')}</td>
            <td>${escHtml(ev.card_number || '—')}</td>
            <td>${escHtml(ev.holder_name || '—')}</td>
            <td><span class="event-type-badge ${ev.event_type}">${acEventLabel(ev.event_type)}</span></td>
        </tr>`).join('');
}

function acEventLabel(type) {
    const map = {
        access: t('ac.ev_access'), denied: t('ac.ev_denied'), alarm: t('ac.ev_alarm'),
        open: t('ac.ev_open'), close: t('ac.ev_close'), tamper: t('ac.ev_tamper'),
    };
    return map[type] || type;
}

// ── Card Schedules ────────────────────────────────────────────
async function acLoadSchedules(statusFilter) {
    const sf = statusFilter || document.getElementById('ac-sched-status-filter')?.value || '';
    try {
        const res = await apiGet('/api/v1/access-control/schedules' + (sf ? '?status=' + sf : ''));
        if (res.success) {
            acSchedules = res.data;
            acRenderSchedules();
        }
    } catch(e) { showToast(t('common.load_failed'), 'error'); }
}

function acRenderSchedules() {
    const tbody = document.getElementById('ac-schedules-tbody');
    if (!tbody) return;
    if (!acSchedules.length) {
        tbody.innerHTML = `<tr><td colspan="8" class="empty-state">${t('ac.no_schedules') || '尚無排程記錄'}</td></tr>`;
        return;
    }
    const dayNames = ['', '一', '二', '三', '四', '五', '六', '日'];
    tbody.innerHTML = acSchedules.map(s => {
        const days = (s.allow_days || '1,2,3,4,5,6,7').split(',').map(d => '週' + (dayNames[parseInt(d)] || d)).join(' ');
        const statusMap = { pending: '⏳ 待審核', approved: '✅ 已核准', rejected: '❌ 已拒絕', expired: '🔒 已過期' };
        const statusCls = { pending: 'warning', approved: 'success', rejected: 'danger', expired: 'muted' };
        return `<tr>
            <td>${escHtml(s.card_number)}</td>
            <td>${escHtml(s.holder_name || '—')}</td>
            <td>${escHtml(s.door_name || t('ac.all_doors') || '所有門')}</td>
            <td style="font-size:0.78rem;">${escHtml(s.valid_from)} ~ ${escHtml(s.valid_until)}</td>
            <td style="font-size:0.78rem;">${escHtml(s.time_from)}~${escHtml(s.time_until)}<br><span style="color:var(--text-muted);font-size:0.7rem;">${days}</span></td>
            <td><span class="card-active-badge ${statusCls[s.status] || ''}">${statusMap[s.status] || s.status}</span></td>
            <td style="font-size:0.75rem;color:var(--text-muted);">${s.approved_by ? escHtml(s.approved_by) : '—'}</td>
            <td>
                ${s.status === 'pending' ? `
                <button class="btn btn-sm btn-primary" onclick="acApproveSchedule(${s.id},'approve')" title="${t('ac.approve') || '核准'}">✅</button>
                <button class="btn btn-sm btn-danger" onclick="acApproveSchedule(${s.id},'reject')" title="${t('ac.reject') || '拒絕'}" style="margin-left:2px;">❌</button>
                ` : ''}
                <button class="btn btn-sm btn-secondary" onclick="acOpenEditScheduleModal(${s.id})" title="${t('common.edit') || '編輯'}" style="margin-left:2px;">✏️</button>
                <button class="btn btn-sm btn-danger" onclick="acDeleteSchedule(${s.id})" title="${t('common.delete') || '刪除'}" style="margin-left:2px;">🗑️</button>
            </td>
        </tr>`;
    }).join('');
}

async function acApproveSchedule(id, action) {
    try {
        const res = await apiPost(`/api/v1/access-control/schedules/${id}/approve`, { action });
        if (res.success) { showToast(res.message || t('common.saved'), 'success'); acLoadSchedules(); }
        else showToast(res.error || t('common.error'), 'error');
    } catch(e) { showToast(t('common.error'), 'error'); }
}

async function acDeleteSchedule(id) {
    if (!confirm(t('ac.confirm_delete_schedule') || '確定要刪除此排程？')) return;
    try {
        const res = await apiDelete(`/api/v1/access-control/schedules/${id}`);
        if (res.success) { showToast(t('common.deleted') || '已刪除', 'success'); acLoadSchedules(); }
    } catch(e) { showToast(t('common.error'), 'error'); }
}

function acOpenAddScheduleModal() { acShowScheduleModal(null); }
function acOpenEditScheduleModal(id) {
    const s = acSchedules.find(x => x.id === id);
    acShowScheduleModal(s);
}

function acShowScheduleModal(sched) {
    const isEdit = !!sched;
    // Build door options
    const doorOpts = acDoors.map(d =>
        `<option value="${d.id}" ${sched?.door_id === d.id ? 'selected' : ''}>${escHtml(d.name)}</option>`
    ).join('');
    const dayNums = [1,2,3,4,5,6,7];
    const dayLabels = ['一','二','三','四','五','六','日'];
    const savedDays = (sched?.allow_days || '1,2,3,4,5,6,7').split(',').map(Number);
    const dayChecks = dayNums.map((n,i) =>
        `<label style="display:inline-flex;align-items:center;gap:3px;margin-right:6px;">
            <input type="checkbox" class="sched-day-cb" value="${n}" ${savedDays.includes(n)?'checked':''}> 週${dayLabels[i]}
        </label>`
    ).join('');

    const html = `
        <div class="modal-header"><h3>${isEdit ? (t('ac.edit_schedule')||'編輯排程') : (t('ac.add_schedule')||'新增排程進入申請')}</h3></div>
        <div class="modal-body">
            <div class="ac-form-grid" style="grid-template-columns:1fr 1fr;">
                <div class="form-group">
                    <label>${t('ac.card_number')||'卡號'} *</label>
                    <input type="text" id="sched-card-num" class="form-control" value="${escHtml(sched?.card_number||'')}" placeholder="輸入卡號或新卡號" required>
                </div>
                <div class="form-group">
                    <label>${t('ac.holder_name')||'持卡人'}</label>
                    <input type="text" id="sched-holder" class="form-control" value="${escHtml(sched?.holder_name||'')}">
                </div>
                <div class="form-group">
                    <label>${t('ac.department')||'部門'}</label>
                    <input type="text" id="sched-dept" class="form-control" value="${escHtml(sched?.department||'')}">
                </div>
                <div class="form-group">
                    <label>${t('ac.door')||'適用門禁'}</label>
                    <select id="sched-door" class="form-control">
                        <option value="">${t('ac.all_doors')||'所有門（不限）'}</option>
                        ${doorOpts}
                    </select>
                </div>
                <div class="form-group">
                    <label>${t('ac.valid_from')||'開始日期'} *</label>
                    <input type="date" id="sched-from" class="form-control" value="${sched?.valid_from||''}">
                </div>
                <div class="form-group">
                    <label>${t('ac.valid_until')||'結束日期'} *</label>
                    <input type="date" id="sched-until" class="form-control" value="${sched?.valid_until||''}">
                </div>
                <div class="form-group">
                    <label>${t('ac.time_from')||'允許進入時段（起）'}</label>
                    <input type="time" id="sched-time-from" class="form-control" value="${sched?.time_from||'00:00'}">
                </div>
                <div class="form-group">
                    <label>${t('ac.time_until')||'允許進入時段（迄）'}</label>
                    <input type="time" id="sched-time-until" class="form-control" value="${sched?.time_until||'23:59'}">
                </div>
                <div class="form-group" style="grid-column:1/-1;">
                    <label>${t('ac.allow_days')||'允許進入星期'}</label>
                    <div style="display:flex;flex-wrap:wrap;gap:4px;margin-top:4px;">${dayChecks}</div>
                </div>
                <div class="form-group" style="grid-column:1/-1;">
                    <label>${t('ac.note')||'申請說明'}</label>
                    <textarea id="sched-note" class="form-control" rows="2">${escHtml(sched?.note||'')}</textarea>
                </div>
            </div>
        </div>
        <div class="modal-footer">
            <button class="btn btn-secondary" onclick="acHideOverlay('ac-sched-modal')">${t('common.cancel')||'取消'}</button>
            <button class="btn btn-primary" onclick="acSaveSchedule(${sched?.id||'null'})">${t('common.save')||'儲存'}</button>
        </div>`;
    document.getElementById('ac-sched-modal-content').innerHTML = html;
    acShowOverlay('ac-sched-modal');
}

async function acSaveSchedule(id) {
    const days = [...document.querySelectorAll('.sched-day-cb:checked')].map(cb => cb.value).join(',');
    const doorVal = document.getElementById('sched-door').value;
    const payload = {
        card_number: document.getElementById('sched-card-num').value.trim(),
        holder_name: document.getElementById('sched-holder').value.trim(),
        department:  document.getElementById('sched-dept').value.trim(),
        door_id:     doorVal ? parseInt(doorVal) : null,
        allow_days:  days || '1,2,3,4,5,6,7',
        time_from:   document.getElementById('sched-time-from').value || '00:00',
        time_until:  document.getElementById('sched-time-until').value || '23:59',
        valid_from:  document.getElementById('sched-from').value,
        valid_until: document.getElementById('sched-until').value,
        note:        document.getElementById('sched-note').value.trim(),
    };
    if (!payload.card_number || !payload.valid_from || !payload.valid_until) {
        showToast(t('ac.sched_fields_required') || '請填寫卡號及有效期限', 'error'); return;
    }
    try {
        const res = id
            ? await apiPut(`/api/v1/access-control/schedules/${id}`, payload)
            : await apiPost('/api/v1/access-control/schedules', payload);
        if (res.success) {
            showToast(id ? (t('ac.sched_updated')||'排程已更新') : (t('ac.sched_created')||'排程申請已送出'), 'success');
            acHideOverlay('ac-sched-modal');
            acLoadSchedules();
        } else { showToast(res.error || t('common.error'), 'error'); }
    } catch(e) { showToast(t('common.error'), 'error'); }
}

window.acOpenAddScheduleModal   = acOpenAddScheduleModal;
window.acOpenEditScheduleModal  = acOpenEditScheduleModal;
window.acApproveSchedule        = acApproveSchedule;
window.acDeleteSchedule         = acDeleteSchedule;
window.acSaveSchedule           = acSaveSchedule;
window.acLoadSchedules          = acLoadSchedules;

// ── Helpers ───────────────────────────────────────────────────
function escHtml(s) {
    if (s == null) return '';
    return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

function fmtDatetime(s) {
    if (!s) return '—';
    try {
        const d = new Date(s);
        return d.toLocaleString();
    } catch { return s; }
}
