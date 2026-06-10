'use strict';

async function iotLoad() {
    iotApplyAdminVisibility();
    iotEnsureSecurityNotes();
    await Promise.all([
        iotLoadStatus(),
        iotLoadCapabilities(),
        iotLoadDevices(),
        iotLoadMeasurements(),
        iotLoadForwarderSettings({ quiet: true }),
        integrationLoadSettings({ quiet: true }),
        embedLoadTokens({ quiet: true })
    ]);
}

function iotApplyAdminVisibility() {
    const role = typeof getUserRole === 'function' ? getUserRole() : null;
    document.querySelectorAll('#iot [data-permission]').forEach((item) => {
        const allowed = String(item.dataset.permission || '')
            .split(',')
            .map((value) => value.trim())
            .filter(Boolean);
        item.style.display = allowed.includes(role) || allowed.includes('all') ? '' : 'none';
    });
}

async function iotLoadStatus() {
    try {
        const response = await apiGet('/iot/status');
        const data = response.data || {};
        setText('iot-device-count', data.device_count || 0);
        setText('iot-enabled-count', data.enabled_count || 0);
        setText('iot-measurement-count', data.measurement_count || 0);
        setText('iot-forward-pending-count', data.forward_pending_count || 0);
        setText('iot-forward-failed-count', data.forward_failed_count || 0);
        setText('iot-forward-sent-count', data.forward_sent_hold_count || 0);
    } catch (error) {
        console.error('[IoT] status failed', error);
    }
}

async function iotLoadCapabilities() {
    const container = document.getElementById('iot-capabilities-list');
    if (!container) return;
    try {
        const response = await apiGet('/iot/capabilities');
        const items = response.data || [];
        if (!items.length) {
            container.innerHTML = '<div class="empty-message">No protocol profiles</div>';
            return;
        }
        container.innerHTML = items.map((item) => `
            <div class="iot-capability">
                <strong>${escapeIot(item.name || item.protocol)}</strong>
                <span class="iot-protocol-badge">${escapeIot(item.status || item.mode || '-')}</span>
                <small>${escapeIot(item.description || '')}</small>
            </div>
        `).join('');
    } catch (error) {
        container.innerHTML = `<div class="empty-message">${escapeIot(error.message || 'Load failed')}</div>`;
    }
}

async function iotLoadDevices() {
    const tbody = document.getElementById('iot-devices-tbody');
    if (!tbody) return;
    try {
        const response = await apiGet('/iot/devices');
        const devices = response.data || [];
        if (!devices.length) {
            tbody.innerHTML = '<tr><td colspan="8" class="empty-message">No IoT devices</td></tr>';
            return;
        }
        tbody.innerHTML = devices.map((d) => `
            <tr>
                <td>${escapeIot(d.name)}</td>
                <td>${escapeIot(d.protocol)}</td>
                <td>${escapeIot(d.host || d.topic || '-')} ${d.port ? ':' + d.port : ''}</td>
                <td>FC${escapeIot(String(d.function_code || 3))} ${escapeIot(String(d.address ?? '-'))} / ${escapeIot(d.data_type || '-')}</td>
                <td>${escapeIot(d.metric || 'value')}</td>
                <td>${d.last_value === null || d.last_value === undefined ? '-' : escapeIot(String(d.last_value))}</td>
                <td>${escapeIot(d.last_seen || d.last_error || '-')}</td>
                <td>
                    <button class="btn btn-secondary btn-sm" onclick="iotPollDevice(${Number(d.id)})">Poll</button>
                    <button class="btn btn-danger btn-sm" onclick="iotDeleteDevice(${Number(d.id)})" data-permission="admin">Delete</button>
                </td>
            </tr>
        `).join('');
    } catch (error) {
        tbody.innerHTML = `<tr><td colspan="8" class="empty-message">${escapeIot(error.message || 'Load failed')}</td></tr>`;
    }
}

async function iotLoadMeasurements() {
    const tbody = document.getElementById('iot-measurements-tbody');
    if (!tbody) return;
    try {
        const response = await apiGet('/iot/measurements?limit=50');
        const items = response.data || [];
        if (!items.length) {
            tbody.innerHTML = '<tr><td colspan="5" class="empty-message">No measurements</td></tr>';
            return;
        }
        tbody.innerHTML = items.map((m) => `
            <tr>
                <td>${escapeIot(m.created_at || '-')}</td>
                <td>${escapeIot(m.external_id || (m.device_id ? 'device:' + m.device_id : '-'))}</td>
                <td>${escapeIot(m.metric || 'value')}</td>
                <td>${escapeIot(String(m.value))}</td>
                <td>${escapeIot(m.forward_status || 'pending')}</td>
            </tr>
        `).join('');
    } catch (error) {
        tbody.innerHTML = `<tr><td colspan="5" class="empty-message">${escapeIot(error.message || 'Load failed')}</td></tr>`;
    }
}

async function iotCreateDevice() {
    const payload = {
        name: valueOf('iot-name') || 'Modbus Device',
        protocol: 'modbus_tcp',
        host: valueOf('iot-host'),
        port: Number(valueOf('iot-port') || 502),
        unit_id: Number(valueOf('iot-unit') || 1),
        address: Number(valueOf('iot-address') || 0),
        function_code: Number(valueOf('iot-function-code') || 3),
        quantity: iotQuantityForType(valueOf('iot-data-type')),
        data_type: valueOf('iot-data-type') || 'uint16',
        byte_order: valueOf('iot-byte-order') || 'big',
        word_order: valueOf('iot-word-order') || 'big',
        scale: Number(valueOf('iot-scale') || 1),
        offset: Number(valueOf('iot-offset') || 0),
        metric: valueOf('iot-metric') || 'value',
        poll_interval_seconds: Number(valueOf('iot-poll-interval') || 60),
        enabled: true
    };
    if (!payload.host) {
        showToast('Host IP is required', 'warning');
        return;
    }
    try {
        await apiPost('/iot/devices', payload);
        showToast('IoT device added', 'success');
        await iotLoad();
    } catch (error) {
        showToast(error.message || 'Create failed', 'error');
    }
}

async function iotLoadForwarderSettings(options = {}) {
    const url = document.getElementById('iot-forward-url');
    if (!url) return;
    try {
        const response = await apiGet('/iot/forwarder/settings');
        const data = response.data || {};
        const enabled = document.getElementById('iot-forward-enabled');
        const batch = document.getElementById('iot-forward-batch');
        if (enabled) enabled.value = data.enabled ? 'true' : 'false';
        url.value = data.url || '';
        if (batch) batch.value = data.batch_size || 50;
    } catch (error) {
        if (!options.quiet) showToast(error.message || 'Load forwarder settings failed', 'error');
    }
}

async function iotSaveForwarderSettings() {
    const payload = {
        enabled: valueOf('iot-forward-enabled') === 'true',
        url: valueOf('iot-forward-url'),
        token: valueOf('iot-forward-token'),
        batch_size: Number(valueOf('iot-forward-batch') || 50)
    };
    try {
        await apiPut('/iot/forwarder/settings', payload);
        const token = document.getElementById('iot-forward-token');
        if (token) token.value = '';
        showToast('IoT forwarder saved', 'success');
        await iotLoadStatus();
    } catch (error) {
        showToast(error.message || 'Save forwarder failed', 'error');
    }
}

async function iotFlushQueue() {
    try {
        const response = await apiPost('/iot/queue/flush', {});
        const sent = response.data ? response.data.sent : 0;
        showToast(`Forward queue flushed  sent ${sent}`, 'success');
        await iotLoad();
    } catch (error) {
        showToast(error.message || 'Flush failed', 'error');
    }
}

async function iotPollDevice(id) {
    try {
        await apiPost(`/iot/devices/${id}/poll`, {});
        showToast('Poll complete', 'success');
        await iotLoad();
    } catch (error) {
        showToast(error.message || 'Poll failed', 'error');
    }
}

async function iotDeleteDevice(id) {
    if (!confirm('Delete IoT device?')) return;
    try {
        await apiDelete(`/iot/devices/${id}`);
        showToast('IoT device deleted', 'success');
        await iotLoad();
    } catch (error) {
        showToast(error.message || 'Delete failed', 'error');
    }
}

async function iotSendIngest() {
    const payload = {
        external_id: valueOf('iot-ingest-id'),
        name: valueOf('iot-ingest-name'),
        protocol: valueOf('iot-ingest-protocol') || 'rest',
        metric: valueOf('iot-ingest-metric') || 'value',
        value: Number(valueOf('iot-ingest-value') || 0),
        raw: { source: 'ui-test' }
    };
    try {
        await apiPost('/iot/ingest', payload);
        showToast('Ingest accepted', 'success');
        await iotLoad();
    } catch (error) {
        showToast(error.message || 'Ingest failed', 'error');
    }
}

async function iotCreateEmbedToken() {
    const view = valueOf('embed-view') || 'dashboard';
    const minutes = Number(valueOf('embed-expiry') || 1440);
    try {
        const response = await apiPost('/integrations/embed-tokens', {
            name: valueOf('embed-name') || view,
            views: [view],
            expires_in_minutes: minutes
        });
        const url = `${location.origin}/embed.html?view=${encodeURIComponent(view)}&token=${encodeURIComponent(response.data.token)}`;
        const output = document.getElementById('embed-output');
        if (output) {
            output.value = `<iframe src="${escapeIotAttribute(url)}" style="width:100%;height:640px;border:0;" loading="lazy" referrerpolicy="no-referrer" sandbox="allow-scripts allow-same-origin"></iframe>`;
        }
        showToast('Embed token generated  treat the iframe URL as a secret', 'warning');
        await embedLoadTokens({ quiet: true });
    } catch (error) {
        showToast(error.message || 'Token failed', 'error');
    }
}

async function integrationLoadSettings(options = {}) {
    const textarea = document.getElementById('integration-frame-ancestors');
    if (!textarea) return;
    try {
        const response = await apiGet('/integrations/settings');
        const data = response.data || {};
        textarea.value = (data.frame_ancestors || []).join('\n');
    } catch (error) {
        if (!options.quiet) showToast(error.message || 'Load integration settings failed', 'error');
    }
}

async function integrationSaveSettings() {
    const textarea = document.getElementById('integration-frame-ancestors');
    if (!textarea) return;
    const frameAncestors = textarea.value
        .split(/\r?\n/)
        .map((item) => item.trim())
        .filter(Boolean);
    try {
        await apiPut('/integrations/settings', { frame_ancestors: frameAncestors });
        showToast('Integration settings saved', 'success');
        await integrationLoadSettings({ quiet: true });
    } catch (error) {
        showToast(error.message || 'Save integration settings failed', 'error');
    }
}

function iotEnsureSecurityNotes() {
    const embedOutput = document.getElementById('embed-output');
    if (embedOutput && !document.getElementById('embed-token-security-note')) {
        embedOutput.insertAdjacentHTML('beforebegin',
            '<div class="iot-security-note" id="embed-token-security-note"><strong>Security</strong> Embed token URLs grant read-only access to the selected view  use short expiry and revoke tokens after sharing tests</div>');
    }

    const allowlist = document.getElementById('integration-frame-ancestors');
    if (allowlist && !document.getElementById('frame-ancestors-security-note')) {
        allowlist.insertAdjacentHTML('afterend',
            '<div class="iot-security-note" id="frame-ancestors-security-note"><strong>Allowlist</strong> Add only trusted HTTPS origins  avoid wildcard domains and review this list before enabling external portals</div>');
    }
}

async function embedLoadTokens(options = {}) {
    const tbody = document.getElementById('embed-tokens-tbody');
    if (!tbody) return;
    try {
        const response = await apiGet('/integrations/embed-tokens');
        const tokens = response.data || [];
        if (!tokens.length) {
            tbody.innerHTML = '<tr><td colspan="6" class="empty-message">No embed tokens</td></tr>';
            return;
        }
        tbody.innerHTML = tokens.map((token) => {
            const revoked = !!token.revoked_at;
            const status = revoked ? 'revoked' : 'active';
            const action = revoked ? '-' : `<button class="btn btn-danger btn-sm" onclick="embedRevokeToken('${escapeIotAttribute(token.token_id)}')">Revoke</button>`;
            return `
                <tr>
                    <td>${escapeIot(token.name || token.token_id)}</td>
                    <td>${escapeIot((token.views || []).join(', '))}</td>
                    <td>${escapeIot(token.expires_at || '-')}</td>
                    <td>${escapeIot(token.last_used_at || '-')}</td>
                    <td>${escapeIot(status)}</td>
                    <td>${action}</td>
                </tr>
            `;
        }).join('');
    } catch (error) {
        if (!options.quiet) {
            tbody.innerHTML = `<tr><td colspan="6" class="empty-message">${escapeIot(error.message || 'Load failed')}</td></tr>`;
        }
    }
}

async function embedRevokeToken(tokenId) {
    if (!confirm('Revoke this embed token?')) return;
    try {
        await apiDelete(`/integrations/embed-tokens/${encodeURIComponent(tokenId)}`);
        showToast('Embed token revoked', 'success');
        await embedLoadTokens({ quiet: true });
    } catch (error) {
        showToast(error.message || 'Revoke failed', 'error');
    }
}

function iotQuantityForType(type) {
    return ['uint32', 'int32', 'float32'].includes(type) ? 2 : 1;
}

function valueOf(id) {
    const el = document.getElementById(id);
    return el ? String(el.value || '').trim() : '';
}

function setText(id, value) {
    const el = document.getElementById(id);
    if (el) el.textContent = String(value);
}

function escapeIot(value) {
    return String(value ?? '')
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
}

function escapeIotAttribute(value) {
    return escapeIot(value).replace(/`/g, '&#96;');
}
