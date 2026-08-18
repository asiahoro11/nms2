// Made by YTSworks
// YTS工作室製作
'use strict';

let _iotDevices = [];
let _iotLicensed = false;

// ─── Sensor metadata ────────────────────────────────────────────────────────

const IOT_SENSOR_META = {
    temperature:          { unit: 'C'    },
    humidity:             { unit: '%'     },
    temperature_humidity: { unit: ''      },
    power:                { unit: 'W'     },
    voltage:              { unit: 'V'     },
    current:              { unit: 'A'     },
    pressure:             { unit: 'hPa'   },
    co2:                  { unit: 'ppm'   },
    pm25:                 { unit: 'ug/m3' },
};

function iotSensorTypeLabel(type) {
    if (!type) return t('iot.modal.sensor_general') || 'General';
    return t('iot.sensor_type.' + type) || type;
}

const IOT_METRIC_UNIT = {
    temperature: 'C', temp: 'C',
    humidity: '%', humi: '%',
    power: 'W', voltage: 'V', current: 'A',
    pressure: 'hPa', co2: 'ppm', pm25: 'ug/m3',
};

function iotUnitForMetric(metric) {
    if (!metric) return '';
    const key = metric.toLowerCase();
    for (const [k, u] of Object.entries(IOT_METRIC_UNIT)) {
        if (key.includes(k)) return u;
    }
    return '';
}

// ─── Tab navigation ─────────────────────────────────────────────────────────

function iotSwitchTab(name, btn) {
    document.querySelectorAll('#iot .iot-tab-pane').forEach(p => p.classList.remove('active'));
    document.querySelectorAll('#iot .iot-nav-tab').forEach(b => b.classList.remove('active'));
    document.getElementById('iot-tab-' + name).classList.add('active');
    btn.classList.add('active');
}

// ─── Page lifecycle ─────────────────────────────────────────────────────────

async function iotLoad() {
    iotApplyAdminVisibility();
    iotEnsureSecurityNotes();
    await iotLoadStatus();
    if (!_iotLicensed) {
        iotApplyLicenseUI();
        return;
    }
    await Promise.all([
        iotLoadCapabilities(),
        iotLoadDevices(),
        iotLoadMeasurements(),
        iotLoadForwarderSettings({ quiet: true }),
        integrationLoadSettings({ quiet: true }),
        embedLoadTokens({ quiet: true }),
    ]);
}

function iotApplyLicenseUI() {
    const notice = document.getElementById('iot-license-notice');
    const content = document.getElementById('iot-module-content');
    const navLock = document.getElementById('iot-nav-lock');
    const actionButtons = document.querySelectorAll('#iot-refresh-btn, #iot-add-btn, #iot button:not(#iot-license-notice button)');
    if (typeof setModuleLock === 'function') {
        setModuleLock('iot', !_iotLicensed);
    }
    if (_iotLicensed) {
        if (notice) notice.style.display = 'none';
        if (content) content.style.display = '';
        if (navLock) navLock.style.display = 'none';
        actionButtons.forEach((btn) => { btn.disabled = false; btn.classList.remove('btn-disabled'); });
    } else {
        if (notice) notice.style.display = 'flex';
        if (content) content.style.display = 'none';
        if (navLock) navLock.style.display = '';
        actionButtons.forEach((btn) => { btn.disabled = true; btn.classList.add('btn-disabled'); });
    }
}

function iotApplyAdminVisibility() {
    const role = typeof getUserRole === 'function' ? getUserRole() : null;
    document.querySelectorAll('#iot [data-permission]').forEach((el) => {
        const allowed = String(el.dataset.permission || '')
            .split(',').map((v) => v.trim()).filter(Boolean);
        el.style.display = allowed.includes(role) || allowed.includes('all') ? '' : 'none';
    });
}

// ─── Status & capabilities ──────────────────────────────────────────────────

async function iotLoadStatus() {
    try {
        const response = await apiGet('/iot/status');
        const data = response.data || {};
        _iotLicensed = !!data.enabled;
        iotApplyLicenseUI();
        setText('iot-device-count',         data.device_count || 0);
        setText('iot-enabled-count',         data.enabled_count || 0);
        setText('iot-measurement-count',     data.measurement_count || 0);
        setText('iot-forward-pending-count', data.forward_pending_count || 0);
        setText('iot-forward-failed-count',  data.forward_failed_count || 0);
        setText('iot-forward-sent-count',    data.forward_sent_hold_count || 0);
        setText('iot-forward-page-pending',  data.forward_pending_count || 0);
        setText('iot-forward-page-failed',   data.forward_failed_count || 0);
        setText('iot-forward-page-sent',     data.forward_sent_hold_count || 0);
        setText('iot-forward-last-attempt',  data.forward_last_attempt_at ? iotRelativeTime(data.forward_last_attempt_at) : '-');
        setText('iot-forward-last-success',  data.forward_last_success_at ? iotRelativeTime(data.forward_last_success_at) : '-');
        const errorEl = document.getElementById('iot-forward-last-error');
        if (errorEl) errorEl.textContent = data.forward_last_error ? `${t('common.error') || '錯誤'}：${data.forward_last_error}` : '';
    } catch (error) {
        _iotLicensed = false;
        iotApplyLicenseUI();
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
            container.innerHTML = `<div class="empty-message">${t('common.loading')}</div>`;
            return;
        }
        const modeLabel = {
            direct_poll:       t('iot.status.ready') || 'Ready',
            gateway_ingest:    t('iot.status.bridge_ready') || 'Bridge Ready',
            store_and_forward: t('iot.status.ready') || 'Ready',
        };
        const statusBadge = (status) => {
            if (status === 'ready')        return `<span class="iot-protocol-badge badge-success">${t('iot.status.ready')}</span>`;
            if (status === 'bridge_ready') return `<span class="iot-protocol-badge badge-secondary">${t('iot.status.bridge_ready')}</span>`;
            return `<span class="iot-protocol-badge badge-secondary">${escapeIot(status)}</span>`;
        };
        container.innerHTML = items.map((item) => `
            <div class="iot-capability">
                <strong>${escapeIot(item.name || item.protocol)}</strong>
                ${statusBadge(item.status)}
                <small>${escapeIot(modeLabel[item.mode] || item.mode || '')} — ${escapeIot(item.description || '')}</small>
            </div>`).join('');
    } catch (error) {
        container.innerHTML = `<div class="empty-message">${escapeIot(error.message || t('iot.devices.load_failed'))}</div>`;
    }
}

// ─── Sensor cards ───────────────────────────────────────────────────────────

async function iotLoadDevices() {
    const tbody = document.getElementById('iot-devices-tbody');
    const cardContainer = document.getElementById('iot-sensor-cards');
    try {
        const response = await apiGet('/iot/devices');
        const devices = response.data || [];
        _iotDevices = devices;

        iotRenderSensorCards(devices, cardContainer);

        if (!tbody) return;
        if (!devices.length) {
            tbody.innerHTML = `<tr><td colspan="9" class="empty-message">${t('iot.devices.no_devices')}</td></tr>`;
            return;
        }
        tbody.innerHTML = devices.map((d) => {
            const signals = Array.isArray(d.signals) ? d.signals : [];
            const normalizedProtocol = iotNormalizeFormProtocol(d.protocol);
            const target = (normalizedProtocol === 'modbus_tcp' || normalizedProtocol === 'modbus_rtu_tcp')
                ? `${escapeIot(d.host || '-')}:${d.port || 502}`
                : escapeIot(d.serial_port || '-');
            const hasError = !!d.last_error;
            const isOffline = !d.last_seen;
            const statusClass = hasError ? 'badge-error' : isOffline ? 'badge-offline' : 'badge-online';
            const statusText = hasError ? t('iot.status.error') : isOffline ? t('iot.status.offline') : t('iot.status.online');
            const errStyle = hasError ? 'color:var(--warning-color)' : '';
            return `
            <tr>
                <td><strong>${escapeIot(d.name)}</strong></td>
                <td style="font-size:12px">${escapeIot(d.external_id || d.topic || '-')}</td>
                <td><span class="iot-protocol-badge">${escapeIot(iotProtocolLabel(normalizedProtocol))}</span></td>
                <td style="font-size:12px;color:var(--text-muted)">${target}</td>
                <td style="font-size:12px">${signals.length > 1 ? `${signals.length} ${t('iot.modal.signals') || 'signals'}` : `FC${d.function_code || 3} @${d.address ?? 0} / ${escapeIot(iotRegisterDataTypeLabel(d))}`}</td>
                <td>${escapeIot(signals.length ? signals.map((signal) => signal.metric).join(', ') : (d.metric || 'value'))}</td>
                <td style="${errStyle}"><strong>${iotFormatDeviceLastValue(d)}</strong></td>
                <td style="font-size:11px;color:var(--text-muted)">
                    <div class="iot-device-status-cell">
                        <span>${escapeIot(iotRelativeTime(d.last_seen))}</span>
                        <span class="iot-sensor-badge ${statusClass}">${escapeIot(statusText)}</span>
                    </div>
                </td>
                <td>
                    <button class="btn btn-secondary btn-sm" onclick="iotPollDevice(${Number(d.id)})">${t('iot.devices.poll_btn')}</button>
                    <button class="btn btn-secondary btn-sm" onclick="iotOpenEditModal(${Number(d.id)})" data-permission="admin">${t('iot.modal.edit_btn') || 'Edit'}</button>
                    <button class="btn btn-danger btn-sm" onclick="iotDeleteDevice(${Number(d.id)})" data-permission="admin">${t('iot.devices.delete')}</button>
                </td>
            </tr>`;
        }).join('');
    } catch (error) {
        if (tbody) tbody.innerHTML = `<tr><td colspan="9" class="empty-message">${escapeIot(error.message || t('iot.devices.load_failed'))}</td></tr>`;
    }
}

function iotRenderSensorCards(devices, container) {
    if (!container) return;
    const enabled = (devices || []).filter((d) => d.enabled);
    if (!enabled.length) {
        container.innerHTML = `<div class="empty-message" style="grid-column:1/-1">${t('iot.devices.no_devices')}</div>`;
        return;
    }
    container.innerHTML = enabled.map((d) => {
        const hasError  = !!d.last_error;
        const isOffline = !d.last_seen;
        const stateClass = hasError ? 'sensor-error' : isOffline ? 'sensor-offline' : 'sensor-ok';
        const badge = hasError
            ? `<span class="iot-sensor-badge badge-error">${t('iot.status.error')}</span>`
            : isOffline
                ? `<span class="iot-sensor-badge badge-offline">${t('iot.status.offline')}</span>`
                : `<span class="iot-sensor-badge badge-online">${t('iot.status.online')}</span>`;
        const readings  = iotBuildReadings(d);
        return `
        <div class="iot-sensor-card ${stateClass}">
            <div>
                <div class="iot-sensor-name" title="${escapeIotAttribute(d.name)}">${escapeIot(d.name)}</div>
                <div class="iot-sensor-protocol">${escapeIot(iotProtocolLabel(d.protocol))} · Unit ${d.unit_id || 1}</div>
            </div>
            <div class="iot-sensor-readings">${readings}</div>
            <div class="iot-sensor-footer">
                <span>${escapeIot(iotRelativeTime(d.last_seen))}</span>
                ${badge}
            </div>
        </div>`;
    }).join('');
}

function iotBuildReadings(d) {
    const value = (d.last_value === null || d.last_value === undefined) ? null : d.last_value;
    const unit  = iotUnitForMetric(d.metric);
    const fmt   = (v, u) => {
        if (v === null || v === undefined || Number.isNaN(Number(v))) return '-';
        return `${Number(v).toFixed(u === 'C' || u === '%' ? 1 : 2)}`;
    };

    const bitReadings = Array.isArray(d.last_readings) ? d.last_readings : [];
    if ((d.function_code === 1 || d.function_code === 2) && bitReadings.length > 1) {
        return bitReadings.map((r) => `
        <div class="iot-sensor-reading">
            <div class="iot-sensor-reading-label">${escapeIot(r.metric)}</div>
            <div class="iot-sensor-reading-value">${Number(r.value) ? '1' : '0'}</div>
        </div>`).join('');
    }

    if (d.sensor_type === 'temperature_humidity') {
        const pair = iotTemperatureHumidityPair(d);
        const temperature = pair ? pair.temperature : value;
        const humidity = pair ? pair.humidity : null;
        return `
        <div class="iot-sensor-reading">
            <div class="iot-sensor-reading-label">${t('iot.sensor_type.temperature')}</div>
            <div class="iot-sensor-reading-value">${fmt(temperature,'C')}<span class="iot-sensor-reading-unit">C</span></div>
        </div>
        <div class="iot-sensor-reading">
            <div class="iot-sensor-reading-label">${t('iot.sensor_type.humidity')}</div>
            <div class="iot-sensor-reading-value">${fmt(humidity,'%')}<span class="iot-sensor-reading-unit">%</span></div>
        </div>`;
    }
    if (bitReadings.length > 1) {
        return bitReadings.map((reading) => {
            const readingUnit = reading.unit || iotUnitForMetric(reading.metric);
            return `
            <div class="iot-sensor-reading">
                <div class="iot-sensor-reading-label">${escapeIot(reading.metric)}</div>
                <div class="iot-sensor-reading-value">${fmt(reading.value, readingUnit)}${readingUnit ? `<span class="iot-sensor-reading-unit">${escapeIot(readingUnit)}</span>` : ''}</div>
            </div>`;
        }).join('');
    }
    const meta  = IOT_SENSOR_META[d.sensor_type] || null;
    const label = d.sensor_type ? iotSensorTypeLabel(d.sensor_type) : (d.metric || t('iot.ingest.value'));
    const displayUnit = unit || (meta ? meta.unit : '');
    return `
    <div class="iot-sensor-reading">
        <div class="iot-sensor-reading-label">${escapeIot(label)}</div>
        <div class="iot-sensor-reading-value">${fmt(value, displayUnit)}${displayUnit ? `<span class="iot-sensor-reading-unit">${escapeIot(displayUnit)}</span>` : ''}</div>
    </div>`;
}

function iotFormatValue(value, metric) {
    if (value === null || value === undefined || Number.isNaN(Number(value))) return '-';
    const unit = iotUnitForMetric(metric);
    return `${Number(value).toFixed(unit === 'C' || unit === '%' ? 1 : 2)}${unit ? ' ' + unit : ''}`;
}

function iotRegisterDataTypeLabel(d) {
    if (d && d.sensor_type === 'temperature_humidity') return 'T:int16 / H:uint16';
    return d && d.data_type ? d.data_type : '-';
}
function iotFormatDeviceLastValue(d) {
    if (d && d.sensor_type === 'temperature_humidity') {
        const pair = iotTemperatureHumidityPair(d);
        if (pair) return `${Number(pair.temperature).toFixed(1)} C / ${Number(pair.humidity).toFixed(1)} %`;
    }
    return iotFormatValue(d ? d.last_value : null, d ? d.metric : null);
}

function iotTemperatureHumidityPair(d) {
    const readings = Array.isArray(d.last_readings) ? d.last_readings : [];
    const temperature = readings.find((r) => r.metric === 'temperature');
    const humidity = readings.find((r) => r.metric === 'humidity');
    if (temperature && humidity) {
        return { temperature: Number(temperature.value), humidity: Number(humidity.value) };
    }
    return iotDecodeTemperatureHumidityRaw(d);
}

function iotMetricLabel(metric) {
    const key = 'iot.sensor_type.' + (metric || 'value');
    const label = t(key);
    return label !== key ? label : (metric || 'value');
}

function iotProtocolLabel(protocol) {
    const map = {
        modbus_tcp: 'Modbus TCP', modbus_rtu: 'Modbus RTU', modbus_rs485: 'RS485', modbus_rtu_tcp: 'RTU over TCP',
        rest: 'REST', mqtt: 'MQTT', opcua: 'OPC-UA', bacnet: 'BACnet',
    };
    return map[protocol] || protocol || '-';
}

function iotRelativeTime(isoStr) {
    if (!isoStr) return t('common.never') || '-';
    const diff = Date.now() - new Date(isoStr).getTime();
    if (isNaN(diff) || diff < 0) return isoStr;
    if (diff < 60000)    return `${Math.floor(diff / 1000)}s`;
    if (diff < 3600000)  return `${Math.floor(diff / 60000)}${t('common.minutes_ago') || 'm ago'}`;
    if (diff < 86400000) return `${Math.floor(diff / 3600000)}${t('common.hours_ago') || 'h ago'}`;
    return `${Math.floor(diff / 86400000)}${t('common.days_ago') || 'd ago'}`;
}

// ─── Measurements table ─────────────────────────────────────────────────────

async function iotLoadMeasurements() {
    const tbody = document.getElementById('iot-measurements-tbody');
    if (!tbody) return;
    try {
        const response = await apiGet('/iot/measurements?limit=50');
        const items = response.data || [];
        if (!items.length) {
            tbody.innerHTML = `<tr><td colspan="5" class="empty-message">${t('iot.measurements.no_data')}</td></tr>`;
            return;
        }
        tbody.innerHTML = items.map((m) => {
            const id = m.external_id || (m.device_id ? `device:${m.device_id}` : '-');
            const statusHtml = m.forward_status === 'sent'
                ? '<span style="color:var(--success-color)">sent</span>'
                : m.forward_status === 'failed'
                    ? '<span style="color:var(--danger-color)">failed</span>'
                    : escapeIot(m.forward_status || 'pending');
            return `
            <tr>
                <td style="font-size:11px;white-space:nowrap">${escapeIot(m.created_at || '-')}</td>
                <td>${escapeIot(id)}</td>
                <td>${escapeIot(iotMetricLabel(m.metric))}</td>
                <td><strong>${iotFormatValue(m.value, m.metric)}</strong></td>
                <td>${statusHtml}</td>
            </tr>`;
        }).join('');
    } catch (error) {
        if (tbody) tbody.innerHTML = `<tr><td colspan="5" class="empty-message">${escapeIot(error.message || 'Load failed')}</td></tr>`;
    }
}

// ─── Add / Edit device modal ────────────────────────────────────────────────

let _iotEditId = null;
let _iotSignalRows = [];

function iotNormalizeSignalRow(signal = {}, index = 0) {
    const metric = String(signal.metric || signal.name || `signal_${index + 1}`).trim();
    return {
        name: String(signal.name || metric).trim(),
        metric,
        function_code: Number(signal.function_code || 3),
        address: Number(signal.address ?? 0),
        data_type: String(signal.data_type || 'uint16'),
        byte_order: String(signal.byte_order || 'big'),
        word_order: String(signal.word_order || 'big'),
        scale: Number(signal.scale ?? 1),
        offset: Number(signal.offset ?? 0),
        unit: String(signal.unit || ''),
    };
}

function iotSignalInput(value, field, index, type = 'text', extra = '') {
    return `<input class="text-input" type="${type}" value="${escapeIotAttribute(value)}" ${extra}
        oninput="iotUpdateSignalRow(${index},'${field}',this.value)">`;
}

function iotRenderSignalRows() {
    const container = document.getElementById('iot-signal-rows');
    if (!container) return;
    if (!_iotSignalRows.length) _iotSignalRows = [iotNormalizeSignalRow()];
    container.innerHTML = _iotSignalRows.map((signal, index) => {
        const isLast = index === _iotSignalRows.length - 1;
        return `
        <div class="iot-signal-row">
            ${iotSignalInput(signal.name, 'name', index)}
            ${iotSignalInput(signal.metric, 'metric', index)}
            <select class="select-input" onchange="iotUpdateSignalRow(${index},'function_code',this.value)">
                <option value="3" ${signal.function_code === 3 ? 'selected' : ''}>FC03 Holding</option>
                <option value="4" ${signal.function_code === 4 ? 'selected' : ''}>FC04 Input</option>
            </select>
            ${iotSignalInput(signal.address, 'address', index, 'number', 'min="0"')}
            <select class="select-input" onchange="iotUpdateSignalRow(${index},'data_type',this.value)">
                ${['uint16','int16','uint32','int32','float32','float64'].map((type) =>
                    `<option value="${type}" ${signal.data_type === type ? 'selected' : ''}>${type}</option>`).join('')}
            </select>
            <select class="select-input" onchange="iotUpdateSignalRow(${index},'byte_order',this.value)">
                <option value="big" ${signal.byte_order === 'big' ? 'selected' : ''}>Big</option>
                <option value="little" ${signal.byte_order === 'little' ? 'selected' : ''}>Little</option>
            </select>
            <select class="select-input" onchange="iotUpdateSignalRow(${index},'word_order',this.value)">
                <option value="big" ${signal.word_order === 'big' ? 'selected' : ''}>Big</option>
                <option value="little" ${signal.word_order === 'little' ? 'selected' : ''}>Little</option>
            </select>
            ${iotSignalInput(signal.scale, 'scale', index, 'number', 'step="0.001"')}
            ${iotSignalInput(signal.offset, 'offset', index, 'number', 'step="0.001"')}
            ${iotSignalInput(signal.unit, 'unit', index)}
            <button type="button" class="iot-signal-action ${isLast ? 'add' : 'remove'}"
                onclick="${isLast ? 'iotAddSignalRow()' : `iotRemoveSignalRow(${index})`}"
                title="${isLast ? '新增訊號' : '刪除訊號'}">${isLast ? '+' : '−'}</button>
        </div>`;
    }).join('');
}

function iotUpdateSignalRow(index, field, value) {
    if (!_iotSignalRows[index]) return;
    if (['function_code', 'address', 'scale', 'offset'].includes(field)) {
        _iotSignalRows[index][field] = Number(value);
    } else {
        _iotSignalRows[index][field] = value;
    }
}

function iotAddSignalRow() {
    _iotSignalRows.push(iotNormalizeSignalRow({}, _iotSignalRows.length));
    iotRenderSignalRows();
}

function iotRemoveSignalRow(index) {
    if (_iotSignalRows.length <= 1) return;
    _iotSignalRows.splice(index, 1);
    iotRenderSignalRows();
}

function iotCollectSignals() {
    const seen = new Set();
    const signals = _iotSignalRows.map((signal, index) => iotNormalizeSignalRow(signal, index));
    for (const signal of signals) {
        if (!signal.metric) throw new Error('每筆訊號都必須填寫 JSON Tag');
        if (!/^[A-Za-z_][A-Za-z0-9_.-]*$/.test(signal.metric)) {
            throw new Error(`JSON Tag「${signal.metric}」格式不正確`);
        }
        if (seen.has(signal.metric)) throw new Error(`JSON Tag「${signal.metric}」不可重複`);
        seen.add(signal.metric);
    }
    return signals;
}

function iotOpenAddModal() {
    _iotEditId = null;
    const titleEl = document.getElementById('iot-modal-title');
    const btnEl   = document.getElementById('iot-modal-save-btn');
    if (titleEl) titleEl.setAttribute('data-i18n', 'iot.modal.add_title'), titleEl.textContent = t('iot.modal.add_title') || 'Add IoT Device';
    if (btnEl)   btnEl.setAttribute('data-i18n', 'iot.modal.add'), btnEl.textContent = t('iot.modal.add') || 'Add Device';
    // Reset form to defaults
    setVal('iot-name', '');
    setVal('iot-external-id', '');
    setVal('iot-host', '');
    setVal('iot-port', '502');
    setVal('iot-serial-port', '');
    setVal('iot-baud-rate', '9600');
    setVal('iot-data-bits', '8');
    setVal('iot-parity', 'N');
    setVal('iot-stop-bits', '1');
    setVal('iot-unit', '1');
    setVal('iot-poll-interval', '60');
    _iotSignalRows = [iotNormalizeSignalRow({ name: '訊號 1', metric: 'signal_1' })];
    iotRenderSignalRows();
    iotSwitchProtocol('modbus_tcp');
    const modal = document.getElementById('iot-add-modal');
    if (modal) modal.style.display = 'flex';
}

function iotOpenEditModal(idOrObj) {
    const d = (typeof idOrObj === 'number' || typeof idOrObj === 'string')
        ? _iotDevices.find((x) => x.id === Number(idOrObj))
        : idOrObj;
    if (!d) return;
    _iotEditId = d.id;
    const titleEl = document.getElementById('iot-modal-title');
    const btnEl   = document.getElementById('iot-modal-save-btn');
    if (titleEl) titleEl.setAttribute('data-i18n', 'iot.modal.edit_title'), titleEl.textContent = t('iot.modal.edit_title') || 'Edit IoT Device';
    if (btnEl)   btnEl.setAttribute('data-i18n', 'iot.modal.save'), btnEl.textContent = t('iot.modal.save') || 'Save';

    // Fill protocol tab first
    const protocol = iotNormalizeFormProtocol(d.protocol);
    iotSwitchProtocol(protocol);

    setVal('iot-name',          d.name || '');
    setVal('iot-external-id',   d.external_id || d.topic || '');
    setVal('iot-host',          d.host || '');
    setVal('iot-port',          String(d.port || 502));
    setVal('iot-serial-port',   d.serial_port || '');
    setVal('iot-baud-rate',     String(d.baud_rate || 9600));
    setVal('iot-data-bits',     String(d.data_bits || 8));
    setVal('iot-parity',        d.parity || 'N');
    setVal('iot-stop-bits',     String(d.stop_bits || 1));
    setVal('iot-unit',          String(d.unit_id || 1));
    setVal('iot-poll-interval', String(d.poll_interval_seconds || 60));
    if (Array.isArray(d.signals) && d.signals.length) {
        _iotSignalRows = d.signals.map(iotNormalizeSignalRow);
    } else if (d.sensor_type === 'temperature_humidity') {
        _iotSignalRows = [
            iotNormalizeSignalRow({ name: '冰水供水溫度', metric: 'chwSupplyTempC', address: d.address ?? 0, function_code: d.function_code || 4, data_type: 'int16', scale: d.scale ?? 0.1, offset: d.offset ?? 0 }),
            iotNormalizeSignalRow({ name: '冰水回水溫度', metric: 'chwReturnTempC', address: (d.address ?? 0) + 1, function_code: d.function_code || 4, data_type: 'uint16', scale: d.scale ?? 0.1, offset: 0 }),
        ];
    } else {
        _iotSignalRows = [iotNormalizeSignalRow(d)];
    }
    iotRenderSignalRows();

    const modal = document.getElementById('iot-add-modal');
    if (modal) modal.style.display = 'flex';
}

function iotCloseAddModal() {
    _iotEditId = null;
    const modal = document.getElementById('iot-add-modal');
    if (modal) modal.style.display = 'none';
}

function iotNormalizeFormProtocol(protocol) {
    switch (String(protocol || '').trim().toLowerCase()) {
        case 'modbus_rtu_tcp':
        case 'modbus-rtu-tcp':
        case 'rtu_tcp':
        case 'rtu-over-tcp':
        case 'rtu_over_tcp':
            return 'modbus_rtu_tcp';
        case 'modbus_rtu':
        case 'modbus-rtu':
        case 'modbus_rs485':
        case 'modbus-rs485':
        case 'rtu':
        case 'rs485':
            return 'modbus_rs485';
        default:
            return 'modbus_tcp';
    }
}

function iotSetProtocolFieldGroup(groupId, enabled) {
    const group = document.getElementById(groupId);
    if (!group) return;
    group.hidden = !enabled;
    group.setAttribute('aria-hidden', enabled ? 'false' : 'true');
    group.querySelectorAll('input, select, textarea').forEach((field) => {
        field.disabled = !enabled;
    });
}

function iotSwitchProtocol(protocol, btn) {
    const normalizedProtocol = iotNormalizeFormProtocol(protocol);
    const modal = document.getElementById('iot-add-modal') || document;
    const tabs = modal.querySelectorAll('.iot-protocol-tab');
    tabs.forEach((tab) => tab.classList.remove('active'));
    const activeTab = btn && btn.dataset.iotProtocol === normalizedProtocol
        ? btn
        : modal.querySelector(`.iot-protocol-tab[data-iot-protocol="${normalizedProtocol}"]`);
    if (activeTab) activeTab.classList.add('active');
    const input = document.getElementById('iot-protocol');
    if (input) input.value = normalizedProtocol;
    const isTCP = normalizedProtocol === 'modbus_tcp' || normalizedProtocol === 'modbus_rtu_tcp';
    iotSetProtocolFieldGroup('iot-tcp-fields', isTCP);
    iotSetProtocolFieldGroup('iot-rs485-fields', !isTCP);
}

function iotBuildDevicePayload(currentDevice = null) {
    const protocol = iotNormalizeFormProtocol(valueOf('iot-protocol'));
    const isRTU    = protocol === 'modbus_rs485';
    const signals = iotCollectSignals();
    const externalID = valueOf('iot-external-id');
    if (!valueOf('iot-name')) throw new Error(t('iot.modal.name_required') || '設備名稱為必填');
    if (!externalID) throw new Error('設備 ID／UUID 為必填');
    const payload  = {
        name:                  valueOf('iot-name'),
        external_id:           externalID,
        protocol,
        sensor_type:           '',
        unit_id:               Number(valueOf('iot-unit') || 1),
        poll_interval_seconds: Number(valueOf('iot-poll-interval') || 60),
        enabled:               currentDevice ? currentDevice.enabled : true,
        signals,
    };
    if (isRTU) {
        payload.serial_port = valueOf('iot-serial-port');
        payload.baud_rate   = Number(valueOf('iot-baud-rate') || 9600);
        payload.data_bits   = Number(valueOf('iot-data-bits') || 8);
        payload.parity      = valueOf('iot-parity') || 'N';
        payload.stop_bits   = Number(valueOf('iot-stop-bits') || 1);
        if (!payload.serial_port) throw new Error(t('iot.modal.serial_required'));
    } else {
        payload.host = valueOf('iot-host');
        payload.port = Number(valueOf('iot-port') || 502);
        if (!payload.host) throw new Error(t('iot.modal.host_required'));
    }
    return payload;
}

async function iotCreateDevice() {
    try {
        const payload = iotBuildDevicePayload();
        await apiPost('/iot/devices', payload);
        showToast(t('iot.modal.add_success'), 'success');
        iotCloseAddModal();
        await iotLoad();
    } catch (error) {
        showToast(error.message || t('iot.modal.add_failed'), 'error');
    }
}

async function iotSaveDevice() {
    if (_iotEditId) {
        await iotUpdateDevice(_iotEditId);
    } else {
        await iotCreateDevice();
    }
}

async function iotUpdateDevice(id) {
    const currentDevice = _iotDevices.find((d) => d.id === Number(id));
    try {
        const payload = iotBuildDevicePayload(currentDevice);
        await apiPut(`/iot/devices/${id}`, payload);
        showToast(t('iot.modal.edit_success') || 'Device updated', 'success');
        iotCloseAddModal();
        await iotLoad();
    } catch (error) {
        showToast(error.message || t('iot.modal.edit_failed') || 'Update failed', 'error');
    }
}

// ─── Device actions ─────────────────────────────────────────────────────────

async function iotPollDevice(id) {
    try {
        await apiPost(`/iot/devices/${id}/poll`, {});
        showToast(t('iot.devices.poll_success'), 'success');
        await iotLoad();
    } catch (error) {
        showToast(error.message || t('iot.devices.poll_failed'), 'error');
    }
}

async function iotDeleteDevice(id) {
    if (!confirm(t('iot.modal.delete_confirm'))) return;
    try {
        await apiDelete(`/iot/devices/${id}`);
        showToast(t('iot.modal.delete_success'), 'success');
        await iotLoad();
    } catch (error) {
        showToast(error.message || t('iot.modal.delete_failed'), 'error');
    }
}

// ─── Forwarder ──────────────────────────────────────────────────────────────

async function iotLoadForwarderSettings(options = {}) {
    const host = document.getElementById('iot-forward-host');
    if (!host) return;
    try {
        const response = await apiGet('/iot/forwarder/settings');
        const data = response.data || {};
        const enabled = document.getElementById('iot-forward-enabled');
        const batch   = document.getElementById('iot-forward-batch');
        const interval = document.getElementById('iot-forward-interval');
        if (enabled) enabled.value = data.enabled ? 'true' : 'false';
        if (batch) batch.value = data.batch_size || 50;
        if (interval) interval.value = data.interval_ms || ((data.interval_seconds || 30) * 1000);
        if (data.url) {
            try {
                const parsed = new URL(data.url);
                setVal('iot-forward-protocol', parsed.protocol.replace(':', '') || 'http');
                setVal('iot-forward-host', parsed.hostname || '');
                setVal('iot-forward-port', parsed.port || (parsed.protocol === 'https:' ? '443' : '80'));
                setVal('iot-forward-path', `${parsed.pathname || '/'}${parsed.search || ''}`);
            } catch (_) {
                setVal('iot-forward-host', data.url);
            }
        }
        setText('iot-forward-last-attempt', data.last_attempt_at ? iotRelativeTime(data.last_attempt_at) : '-');
        setText('iot-forward-last-success', data.last_success_at ? iotRelativeTime(data.last_success_at) : '-');
        const errorEl = document.getElementById('iot-forward-last-error');
        if (errorEl) errorEl.textContent = data.last_error ? `${t('common.error') || '錯誤'}：${data.last_error}` : '';
        iotUpdateForwardURLPreview();
    } catch (error) {
        if (!options.quiet) showToast(error.message || 'Load forwarder settings failed', 'error');
    }
}

function iotBuildForwardURL() {
    const protocol = valueOf('iot-forward-protocol') || 'http';
    const host = valueOf('iot-forward-host');
    const port = Number(valueOf('iot-forward-port') || (protocol === 'https' ? 443 : 80));
    let path = valueOf('iot-forward-path') || '/';
    if (!path.startsWith('/')) path = '/' + path;
    return host ? `${protocol}://${host}:${port}${path}` : '';
}

function iotUpdateForwardURLPreview() {
    setText('iot-forward-url-preview', iotBuildForwardURL() || '-');
}

async function iotSaveForwarderSettings() {
    const url = iotBuildForwardURL();
    if (valueOf('iot-forward-enabled') === 'true' && !url) {
        showToast('請輸入拋轉主機 IP／網域', 'warning');
        return;
    }
    const payload = {
        enabled:    valueOf('iot-forward-enabled') === 'true',
        url,
        token:      valueOf('iot-forward-token'),
        batch_size: Number(valueOf('iot-forward-batch') || 50),
        interval_ms: Number(valueOf('iot-forward-interval') || 30000),
    };
    try {
        await apiPut('/iot/forwarder/settings', payload);
        const token = document.getElementById('iot-forward-token');
        if (token) token.value = '';
        showToast(t('iot.forward.save_success'), 'success');
        await Promise.all([iotLoadStatus(), iotLoadForwarderSettings({ quiet: true })]);
    } catch (error) {
        showToast(error.message || t('iot.forward.save_failed'), 'error');
    }
}

async function iotFlushQueue() {
    try {
        const response = await apiPost('/iot/queue/flush', {});
        const sent = response.data ? response.data.sent : 0;
        showToast(`${t('iot.forward.flush_success')} — ${sent}`, 'success');
        await Promise.all([iotLoadStatus(), iotLoadMeasurements(), iotLoadForwarderSettings({ quiet: true })]);
    } catch (error) {
        showToast(error.message || t('iot.forward.flush_failed'), 'error');
    }
}

// ─── Ingest test ────────────────────────────────────────────────────────────

async function iotSendIngest() {
    const payload = {
        external_id: valueOf('iot-ingest-id'),
        name:        valueOf('iot-ingest-name'),
        protocol:    valueOf('iot-ingest-protocol') || 'rest',
        metric:      valueOf('iot-ingest-metric') || 'value',
        value:       Number(valueOf('iot-ingest-value') || 0),
        raw:         { source: 'ui-test' },
    };
    try {
        await apiPost('/iot/ingest', payload);
        showToast(t('iot.ingest.success'), 'success');
        await iotLoad();
    } catch (error) {
        showToast(error.message || t('iot.ingest.failed'), 'error');
    }
}

// ─── Embed tokens ───────────────────────────────────────────────────────────

async function iotCreateEmbedToken() {
    const view    = valueOf('embed-view') || 'dashboard';
    const minutes = Number(valueOf('embed-expiry') || 1440);
    try {
        const response = await apiPost('/integrations/embed-tokens', {
            name:               valueOf('embed-name') || view,
            views:              [view],
            expires_in_minutes: minutes,
        });
        const url = `${location.origin}/embed.html?view=${encodeURIComponent(view)}&token=${encodeURIComponent(response.data.token)}`;
        const output = document.getElementById('embed-output');
        if (output) {
            output.value = `<iframe src="${escapeIotAttribute(url)}" style="width:100%;height:640px;border:0;" loading="lazy" referrerpolicy="no-referrer" sandbox="allow-scripts allow-same-origin"></iframe>`;
        }
        showToast(t('iot.embed.generated'), 'warning');
        await embedLoadTokens({ quiet: true });
    } catch (error) {
        showToast(error.message || t('iot.embed.generate_failed'), 'error');
    }
}

async function embedLoadTokens(options = {}) {
    const tbody = document.getElementById('embed-tokens-tbody');
    if (!tbody) return;
    try {
        const response = await apiGet('/integrations/embed-tokens');
        const tokens = response.data || [];
        if (!tokens.length) {
            tbody.innerHTML = `<tr><td colspan="6" class="empty-message">${t('iot.embed.no_tokens')}</td></tr>`;
            return;
        }
        tbody.innerHTML = tokens.map((token) => {
            const revoked = !!token.revoked_at;
            const action  = revoked ? '-' : `<button class="btn btn-danger btn-sm" onclick="embedRevokeToken('${escapeIotAttribute(token.token_id)}')">${t('iot.embed.revoke')}</button>`;
            return `
            <tr>
                <td>${escapeIot(token.name || token.token_id)}</td>
                <td>${escapeIot((token.views || []).join(', '))}</td>
                <td style="font-size:11px">${escapeIot(token.expires_at || '-')}</td>
                <td style="font-size:11px">${escapeIot(token.last_used_at || '-')}</td>
                <td>${escapeIot(revoked ? 'revoked' : 'active')}</td>
                <td>${action}</td>
            </tr>`;
        }).join('');
    } catch (error) {
        if (!options.quiet) {
            tbody.innerHTML = `<tr><td colspan="6" class="empty-message">${escapeIot(error.message || 'Load failed')}</td></tr>`;
        }
    }
}

async function embedRevokeToken(tokenId) {
    if (!confirm(t('iot.embed.revoke_confirm'))) return;
    try {
        await apiDelete(`/integrations/embed-tokens/${encodeURIComponent(tokenId)}`);
        showToast(t('iot.embed.revoked'), 'success');
        await embedLoadTokens({ quiet: true });
    } catch (error) {
        showToast(error.message || t('iot.embed.revoke_failed'), 'error');
    }
}

// ─── Integration settings ───────────────────────────────────────────────────

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
    const frameAncestors = textarea.value.split(/\r?\n/).map((v) => v.trim()).filter(Boolean);
    try {
        await apiPut('/integrations/settings', { frame_ancestors: frameAncestors });
        showToast(t('iot.allowlist.save_success'), 'success');
        await integrationLoadSettings({ quiet: true });
    } catch (error) {
        showToast(error.message || t('iot.allowlist.save_failed'), 'error');
    }
}

// ─── Security notes ─────────────────────────────────────────────────────────

function iotEnsureSecurityNotes() {
    const embedOutput = document.getElementById('embed-output');
    if (embedOutput && !document.getElementById('embed-token-security-note')) {
        embedOutput.insertAdjacentHTML('beforebegin',
            '<div class="iot-security-note" id="embed-token-security-note"><strong>Security</strong> — Embed token URLs grant read-only access. Use short expiry and revoke tokens after sharing.</div>');
    }
    const allowlist = document.getElementById('integration-frame-ancestors');
    if (allowlist && !document.getElementById('frame-ancestors-security-note')) {
        allowlist.insertAdjacentHTML('afterend',
            '<div class="iot-security-note" id="frame-ancestors-security-note"><strong>Allowlist</strong> — Add only trusted HTTPS origins. Avoid wildcards.</div>');
    }
}

// ─── Helpers ────────────────────────────────────────────────────────────────

function iotQuantityForType(type) {
    if (type === 'float64') return 4;
    return ['uint32', 'int32', 'float32'].includes(type) ? 2 : 1;
}

function iotQuantityForSelection(sensorType, dataType) {
    if (sensorType === 'temperature_humidity' && ['uint16', 'int16'].includes(dataType)) return 2;
    return iotQuantityForType(dataType);
}

function iotDataTypeForSelection(sensorType) {
    return sensorType === 'temperature_humidity' ? 'int16' : (valueOf('iot-data-type') || 'uint16');
}

function iotEnsureTemperatureHumidityDataTypeFields() {
    const dataType = document.getElementById('iot-data-type');
    if (!dataType || document.getElementById('iot-temperature-data-type')) return;
    const dataTypeField = dataType.closest('.iot-form-field');
    if (!dataTypeField) return;
    dataTypeField.classList.add('iot-field-data-type-generic');
    dataTypeField.insertAdjacentHTML('afterend', `
        <div class="iot-form-field iot-field-temperature-humidity-types" style="display:none;">
            <label class="iot-form-label" data-i18n="iot.modal.temperature_data_type">Temperature data type</label>
            <input class="text-input" id="iot-temperature-data-type" type="text" value="int16" readonly>
        </div>
        <div class="iot-form-field iot-field-temperature-humidity-types" style="display:none;">
            <label class="iot-form-label" data-i18n="iot.modal.humidity_data_type">Humidity data type</label>
            <input class="text-input" id="iot-humidity-data-type" type="text" value="uint16" readonly>
        </div>`);
    if (typeof applyI18n === 'function') applyI18n();
}
function iotSyncDataTypeFields() {
    iotEnsureTemperatureHumidityDataTypeFields();
    const isTemperatureHumidity = valueOf('iot-sensor-type') === 'temperature_humidity';
    document.querySelectorAll('.iot-field-data-type-generic').forEach((el) => {
        el.style.display = isTemperatureHumidity ? 'none' : '';
    });
    document.querySelectorAll('.iot-field-temperature-humidity-types').forEach((el) => {
        el.style.display = isTemperatureHumidity ? '' : 'none';
    });
    setVal('iot-temperature-data-type', 'int16');
    setVal('iot-humidity-data-type', 'uint16');
    if (isTemperatureHumidity) setVal('iot-data-type', 'int16');
}

function iotApplySensorDefaults() {
    const sensorType = valueOf('iot-sensor-type');
    if (sensorType !== 'temperature' && sensorType !== 'humidity' && sensorType !== 'temperature_humidity') {
        iotSyncDataTypeFields();
        return;
    }
    setVal('iot-function-code', '4');
    setVal('iot-data-type', 'int16');
    setVal('iot-scale', '0.1');
    setVal('iot-byte-order', 'big');
    setVal('iot-word-order', 'big');
    if (sensorType === 'temperature_humidity') {
        setVal('iot-address', '0');
    } else if (sensorType === 'humidity') {
        setVal('iot-address', '1');
    }
    iotSyncDataTypeFields();
}

function iotDecodeTemperatureHumidityRaw(d) {
    const raw = String(d.last_raw || '').replace(/[^0-9a-f]/gi, '');
    if (raw.length < 8 || !['uint16', 'int16'].includes(String(d.data_type || '').toLowerCase())) return null;
    const bytes = raw.match(/../g).map((v) => parseInt(v, 16));
    if (bytes.length < 4 || bytes.some((v) => Number.isNaN(v))) return null;
    const readRegister = (offset, signed) => {
        let value = (bytes[offset] << 8) | bytes[offset + 1];
        if (String(d.byte_order || 'big').toLowerCase() === 'little') {
            value = (bytes[offset + 1] << 8) | bytes[offset];
        }
        if (signed && value >= 0x8000) {
            value -= 0x10000;
        }
        return value;
    };
    const configuredScale = Number(d.scale ?? 1);
    const scale = configuredScale === 1 ? 0.1 : configuredScale;
    const temperatureOffset = Number(d.offset ?? 0);
    const swapped = String(d.word_order || 'big').toLowerCase() === 'little';
    const temperatureOffsetIndex = swapped ? 2 : 0;
    const humidityOffsetIndex = swapped ? 0 : 2;
    return {
        temperature: (readRegister(temperatureOffsetIndex, true) * scale) + temperatureOffset,
        humidity: readRegister(humidityOffsetIndex, false) * scale,
    };
}

document.addEventListener('change', (event) => {
    if (event.target && event.target.id === 'iot-sensor-type') {
        iotApplySensorDefaults();
    }
    if (event.target && ['iot-forward-protocol', 'iot-forward-host', 'iot-forward-port', 'iot-forward-path'].includes(event.target.id)) {
        iotUpdateForwardURLPreview();
    }
});

document.addEventListener('input', (event) => {
    if (event.target && ['iot-forward-host', 'iot-forward-port', 'iot-forward-path'].includes(event.target.id)) {
        iotUpdateForwardURLPreview();
    }
});

function valueOf(id) {
    const el = document.getElementById(id);
    return el ? String(el.value || '').trim() : '';
}

function setVal(id, value) {
    const el = document.getElementById(id);
    if (el) el.value = value;
}

function setText(id, value) {
    const el = document.getElementById(id);
    if (el) el.textContent = String(value);
}

function escapeIot(value) {
    return String(value ?? '')
        .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

function escapeIotAttribute(value) {
    return escapeIot(value).replace(/`/g, '&#96;');
}
