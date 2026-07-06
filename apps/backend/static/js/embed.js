// Made by YTSworks
// YTS工作室製作
'use strict';

async function embedLoad() {
    const params = new URLSearchParams(location.search);
    const token = params.get('token') || '';
    const view = params.get('view') || 'dashboard';
    const content = document.getElementById('embed-content');
    if (!token) {
        renderEmbedError('Missing embed token', 'Create a scoped token in IoT / Modbus integrations and keep the embed URL private.');
        return;
    }
    try {
        const response = await fetch(`/api/v1/integrations/embed-snapshot?view=${encodeURIComponent(view)}&token=${encodeURIComponent(token)}`, {
            cache: 'no-store',
            headers: { 'Cache-Control': 'no-cache' }
        });
        const payload = await response.json();
        if (!response.ok || !payload.success) {
            throw new Error(payload.error || 'Embed request failed');
        }
        renderEmbed(payload.data);
    } catch (error) {
        renderEmbedError(error.message || 'Load failed', 'Check token expiry, revoked status, and iframe allowlist before sharing this view again.');
    }
}

function renderEmbed(data) {
    document.getElementById('embed-title').textContent = `${data.system?.name || 'NMS'} - ${data.view}`;
    document.getElementById('embed-subtitle').textContent = `${data.system?.version || ''} ${data.generated_at || ''}`;
    if (data.view === 'dashboard') return renderDashboard(data.dashboard || {});
    if (data.view === 'devices') return renderDevices(data.devices?.items || []);
    if (data.view === 'topology') return renderTopology(data.topology || {});
    if (data.view === 'alerts') return renderEvents(data.events || []);
    if (data.view === 'iot') return renderIoT(data.iot || {});
}

function renderEmbedError(message, hint) {
    document.getElementById('embed-content').innerHTML = `
        <div class="embed-card">
            <div class="embed-muted">Embed unavailable</div>
            <div style="margin-top:6px;font-weight:700;">${escapeEmbed(message)}</div>
            <div class="embed-security-note">
                <strong>Security</strong> ${escapeEmbed(hint)}
            </div>
        </div>
    `;
}

function renderDashboard(d) {
    document.getElementById('embed-content').innerHTML = `
        <div class="embed-grid">
            ${card('Total devices', d.total_devices ?? 0)}
            ${card('Online', d.online_devices ?? 0)}
            ${card('Offline', d.offline_devices ?? 0)}
            ${card('Memory', d.memory_usage || '-')}
        </div>
    `;
}

function renderDevices(items) {
    document.getElementById('embed-content').innerHTML = table(['Name', 'IP', 'Type', 'Status'], items.map((d) => [
        d.name, d.ip_address, d.device_type, d.is_online ? 'online' : 'offline'
    ]));
}

function renderTopology(t) {
    const nodes = t.nodes || [];
    const links = t.links || [];
    document.getElementById('embed-content').innerHTML = `
        <div class="embed-grid">
            ${card('Nodes', nodes.length)}
            ${card('Links', links.length)}
        </div>
        ${table(['Name', 'IP', 'Type'], nodes.slice(0, 50).map((n) => [n.name || n.id, n.ip_address || '-', n.device_type || '-']))}
    `;
}

function renderEvents(items) {
    document.getElementById('embed-content').innerHTML = table(['Time', 'Severity', 'Type', 'Message'], items.map((e) => [
        e.created_at, e.severity, e.event_type, e.message
    ]));
}

function renderIoT(iot) {
    const status = iot.status || {};
    const devices = iot.devices || [];
    document.getElementById('embed-content').innerHTML = `
        <div class="embed-grid">
            ${card('IoT devices', status.device_count ?? 0)}
            ${card('Enabled', status.enabled_count ?? 0)}
            ${card('Measurements', status.measurement_count ?? 0)}
        </div>
        ${table(['Name', 'Protocol', 'Target', 'Last value'], devices.map((d) => [
            d.name, d.protocol, d.host || d.topic || '-', d.last_value ?? '-'
        ]))}
    `;
}

function card(label, value) {
    return `<div class="embed-card"><div class="embed-muted">${escapeEmbed(label)}</div><div class="embed-value">${escapeEmbed(value)}</div></div>`;
}

function table(headers, rows) {
    return `
        <div class="embed-table-wrap" style="margin-top:12px;">
            <table class="data-table">
                <thead><tr>${headers.map((h) => `<th>${escapeEmbed(h)}</th>`).join('')}</tr></thead>
                <tbody>${rows.length ? rows.map((r) => `<tr>${r.map((c) => `<td>${escapeEmbed(c ?? '')}</td>`).join('')}</tr>`).join('') : `<tr><td colspan="${headers.length}" class="empty-message">No data</td></tr>`}</tbody>
            </table>
        </div>
    `;
}

function escapeEmbed(value) {
    return String(value ?? '')
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
}

document.addEventListener('DOMContentLoaded', embedLoad);
