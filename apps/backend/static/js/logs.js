(function () {
    const RETENTION_CONFIG_KEYS = [
        { key: 'audit_log_retention_days', labelKey: 'logs.retention.audit', fallback: '稽核日誌' },
        { key: 'system_log_retention_days', labelKey: 'logs.retention.system', fallback: '系統日誌' },
        { key: 'device_log_retention_days', labelKey: 'logs.retention.device', fallback: '設備日誌' },
        { key: 'config_change_log_retention_days', labelKey: 'logs.retention.config_change', fallback: '設定變更' },
    ];

    let optionsCache = null;
    let retentionCache = null;
    let filtersReady = false;

    function translate(key, fallback) {
        try {
            if (typeof t === 'function') {
                const value = t(key);
                if (value && value !== key && !String(value).includes('??') && !String(value).includes('\uFFFD')) {
                    return String(value);
                }
            }
        } catch (_) {
        }
        return fallback;
    }

    const LOG_VALUE_MAP = {
        camera: '攝影機',
        device: '設備',
        topology: '拓樸',
        auth: '認證',
        license: '授權',
        backup: '備份',
        reboot: '重啟',
        poe: 'PoE',
        system: '系統',
        config: '設定',
        security: '安全性',
        scheduler: '排程',
        discovery: '探索',
        polling: '輪詢',
        monitor: '監控',
        streaming: '串流',
        snapshot: '快照',
        live_preview: '即時預覽',
        info: '資訊',
        notice: '通知',
        warning: '警告',
        error: '錯誤',
        critical: '嚴重',
        debug: '除錯',
        success: '成功',
        failed: '失敗',
        reviewed: '已審查',
        pending: '待審查',
        flagged: '需追蹤',
        acked: '已確認',
        unacked: '未確認',
        admin: '管理者',
        editor: '編輯者',
        viewer: '檢視者',
        true: '是',
        false: '否',
        bytes: '位元組',
        nms: 'NMS',
    };

    const LOG_EVENT_MAP = {
        camera_stream_started: '攝影機即時預覽已啟動',
        camera_stream_unavailable: '攝影機即時預覽不可用',
        camera_stream_timeout: '攝影機即時預覽逾時',
        camera_stream_recycled: '攝影機串流已重建',
        camera_snapshot_success: '攝影機快照成功',
        camera_snapshot_failed: '攝影機快照失敗',
        camera_snapshot_timeout: '攝影機快照逾時',
        camera_snapshot_empty: '攝影機快照空畫面',
        camera_health_offline: '攝影機健康檢查判定離線',
        camera_health_recovered: '攝影機已恢復連線',
        camera_recording_started: '攝影機錄影已開始',
        camera_recording_stopped: '攝影機錄影已停止',
        user_login_success: '使用者登入成功',
        user_login_failed: '使用者登入失敗',
        user_logout: '使用者登出',
        license_activated: '授權已啟用',
        license_expired: '授權已到期',
        license_locked: '授權已鎖定',
        backup_restore_ready: '備份還原準備完成',
        backup_restore_scheduled: '備份還原已排程',
        topology_discovery_queued: '拓樸探索已排入佇列',
        topology_discovery_skipped: '拓樸探索已略過',
        device_poll_queued: '設備輪詢已排入佇列',
        device_poll_skipped: '設備輪詢已略過',
    };

    const LOG_MESSAGE_MAP = {
        'camera live preview started': '攝影機即時預覽已啟動',
        'camera live preview unavailable': '攝影機即時預覽不可用',
        'camera live preview timeout': '攝影機即時預覽逾時',
        'camera snapshot succeeded': '攝影機快照成功',
        'camera snapshot failed': '攝影機快照失敗',
        'camera snapshot timeout': '攝影機快照逾時',
        'camera snapshot empty frame': '攝影機快照空畫面',
        'camera health check detected offline': '攝影機健康檢查判定離線',
        'camera recovery detected': '攝影機已恢復連線',
        'camera recording session started': '攝影機錄影工作階段已開始',
        'camera recording session stopped': '攝影機錄影工作階段已停止',
        'user login success': '使用者登入成功',
        'user login failed': '使用者登入失敗',
        'user logout success': '使用者登出成功',
        'license activated': '授權已啟用',
        'license expired': '授權已到期',
        'license locked': '授權已鎖定',
        'backup restore prepared': '備份還原準備完成',
        'backup restore scheduled': '備份還原已排程',
    };

    function normalizeLookupKey(value) {
        return String(value || '')
            .trim()
            .replace(/\s+/g, ' ')
            .replace(/([a-z])([A-Z])/g, '$1_$2')
            .replace(/[\s-]+/g, '_')
            .toLowerCase();
    }

    function titleCaseToWords(value) {
        return String(value || '')
            .replace(/([a-z])([A-Z])/g, '$1 $2')
            .replace(/_/g, ' ')
            .trim();
    }

    function translateLogValue(value) {
        if (value === undefined || value === null || value === '') {
            return '';
        }

        const normalized = normalizeLookupKey(value);
        if (LOG_VALUE_MAP[normalized]) {
            return LOG_VALUE_MAP[normalized];
        }

        return String(value);
    }

    function translateLogMessage(message, eventCode = '') {
        const normalizedEvent = normalizeLookupKey(eventCode);
        if (normalizedEvent && LOG_EVENT_MAP[normalizedEvent]) {
            return LOG_EVENT_MAP[normalizedEvent];
        }

        const normalizedMessage = normalizeLookupKey(message);
        if (normalizedMessage && LOG_MESSAGE_MAP[normalizedMessage]) {
            return LOG_MESSAGE_MAP[normalizedMessage];
        }

        return String(message || '-');
    }

    function translateServiceOrScope(value) {
        const translated = translateLogValue(value);
        return translated || String(value || '-');
    }

    function translateActionLabel(action) {
        if (!action) {
            return '-';
        }
        try {
            const actionLabel = translate(`audit.actions.${String(action).toLowerCase()}`, '');
            if (actionLabel && actionLabel !== `audit.actions.${String(action).toLowerCase()}`) {
                return actionLabel;
            }
        } catch (_) {
        }
        return translateLogMessage(action);
    }

    function escapeHtml(value) {
        return String(value ?? '')
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#39;');
    }

    function ensureLogState() {
        if (!window.state) {
            window.state = {};
        }
        if (!state.logs) {
            state.logs = {};
        }
        state.logs.page = state.logs.page || 1;
        state.logs.limit = state.logs.limit || 50;
        state.logs.type = state.logs.type || 'system_logs';
        state.logs.severity = state.logs.severity || '';
        state.logs.search = state.logs.search || '';
        state.logs.scope = state.logs.scope || '';
        state.logs.actor = state.logs.actor || '';
        state.logs.device = state.logs.device || '';
        state.logs.status = state.logs.status || '';
        state.logs.dateFrom = state.logs.dateFrom || '';
        state.logs.dateTo = state.logs.dateTo || '';
        state.logs.format = state.logs.format || 'csv';
    }

    function page() {
        return document.getElementById('logs');
    }

    function currentType() {
        ensureLogState();
        return state.logs.type;
    }

    function typeCatalog() {
        return {
            system_logs: {
                label: translate('logs.types.system', '系統日誌'),
                endpoint: '/system-logs',
                scopeParam: 'service',
                scopeOptionsKey: 'system_services',
                actorParam: '',
                deviceParam: '',
                statusParam: 'review_status',
                severityParam: 'level',
                scopePlaceholder: translate('logs.placeholders.scope.system', '服務來源'),
                headers: [
                    translate('logs.columns.time', '時間'),
                    translate('logs.columns.system_source', '服務 / 事件'),
                    translate('logs.columns.level_review', '等級 / 審查'),
                    translate('logs.columns.details', '詳細資訊'),
                ],
            },
            device_logs: {
                label: translate('logs.types.device', '設備日誌'),
                endpoint: '/device-logs',
                scopeParam: 'facility',
                scopeOptionsKey: 'device_facilities',
                actorParam: '',
                deviceParam: 'device',
                statusParam: 'ack_status',
                severityParam: 'severity',
                scopePlaceholder: translate('logs.placeholders.scope.device', '設備設施類型'),
                headers: [
                    translate('logs.columns.time', '時間'),
                    translate('logs.columns.device_source', '設備 / 設施'),
                    translate('logs.columns.level_ack', '等級 / 確認'),
                    translate('logs.columns.details', '詳細資訊'),
                ],
            },
            audit_logs: {
                label: translate('logs.types.audit', '稽核日誌'),
                endpoint: '/audit-logs',
                scopeParam: 'module',
                scopeOptionsKey: 'audit_modules',
                actorParam: 'username',
                deviceParam: 'device',
                statusParam: 'status',
                severityParam: '',
                scopePlaceholder: translate('logs.placeholders.scope.audit', '操作模組'),
                headers: [
                    translate('logs.columns.time', '時間'),
                    translate('logs.columns.audit_source', '使用者 / 模組'),
                    translate('logs.columns.status_review', '狀態 / 審查'),
                    translate('logs.columns.details', '詳細資訊'),
                ],
            },
            config_change_logs: {
                label: translate('logs.types.config_change', '設定變更'),
                endpoint: '/config-change-logs',
                scopeParam: 'module',
                scopeOptionsKey: 'config_modules',
                actorParam: 'username',
                deviceParam: 'device',
                statusParam: 'status',
                severityParam: '',
                scopePlaceholder: translate('logs.placeholders.scope.config_change', '變更模組'),
                headers: [
                    translate('logs.columns.time', '時間'),
                    translate('logs.columns.config_source', '模組 / 目標'),
                    translate('logs.columns.status_review', '狀態 / 審查'),
                    translate('logs.columns.details', '詳細資訊'),
                ],
            },
        };
    }

    function typeMeta() {
        return typeCatalog()[currentType()] || typeCatalog().system_logs;
    }

    function reviewStatusMeta(status) {
        const map = {
            pending: { label: translate('logs.status.pending', '待審查'), className: 'warning' },
            reviewed: { label: translate('logs.status.reviewed', '已審查'), className: 'active' },
            flagged: { label: translate('logs.status.flagged', '需追蹤'), className: 'inactive' },
        };
        return map[String(status || 'pending').toLowerCase()] || map.pending;
    }

    function ackStatusMeta(status) {
        const map = {
            acked: { label: translate('logs.status.acked', '已確認'), className: 'active' },
            unacked: { label: translate('logs.status.unacked', '未確認'), className: 'warning' },
        };
        return map[String(status || 'unacked').toLowerCase()] || map.unacked;
    }

    function auditStatusMeta(status) {
        const normalized = String(status || '').toLowerCase();
        const map = {
            success: { label: translate('logs.status.success', '成功'), className: 'active' },
            failed: { label: translate('logs.status.failed', '失敗'), className: 'inactive' },
            warning: { label: translate('logs.status.warning', '警告'), className: 'warning' },
            emergency: { label: translate('logs.severity.emergency', '緊急'), className: 'inactive' },
            alert: { label: translate('logs.severity.alert', '警報'), className: 'inactive' },
            critical: { label: translate('logs.severity.critical', '嚴重'), className: 'inactive' },
            error: { label: translate('logs.severity.error', '錯誤'), className: 'inactive' },
            notice: { label: translate('logs.severity.notice', '通知'), className: 'active' },
            info: { label: translate('logs.severity.info', '資訊'), className: 'active' },
            debug: { label: translate('logs.severity.debug', '除錯'), className: 'warning' },
        };
        return map[normalized] || { label: status || '-', className: 'inactive' };
    }

    function parseJSON(value) {
        if (!value || typeof value !== 'string') {
            return null;
        }
        try {
            return JSON.parse(value);
        } catch (_) {
            return null;
        }
    }

    function formatTime(value) {
        if (!value) {
            return '-';
        }
        try {
            if (typeof formatDateTime === 'function') {
                return formatDateTime(value);
            }
        } catch (_) {
        }
        try {
            return new Date(value).toLocaleString();
        } catch (_) {
            return String(value);
        }
    }

    function renderBadge(meta, fallbackLabel) {
        const model = meta || { label: fallbackLabel || '-', className: 'inactive' };
        return `<span class="status-badge ${escapeHtml(model.className)}">${escapeHtml(model.label)}</span>`;
    }

    function renderKeyValue(label, value) {
        if (value === undefined || value === null || value === '') {
            return '';
        }
        return `<div style="display:flex;gap:8px;align-items:flex-start;"><span style="color:var(--text-muted);min-width:112px;">${escapeHtml(label)}</span><span>${escapeHtml(value)}</span></div>`;
    }

    function humanizeKey(key) {
        const normalized = normalizeLookupKey(key);
        const map = {
            device_name: translate('logs.labels.device_name', '設備名稱'),
            ip_address: translate('logs.labels.ip_address', 'IP 位址'),
            mac_address: translate('logs.labels.mac_address', 'MAC 位址'),
            source_ip: translate('logs.labels.source_ip', '來源 IP'),
            source_mac: translate('logs.labels.source_mac', '來源 MAC'),
            recordset_id: translate('logs.labels.recordset_id', 'Recordset ID'),
            correlation_id: translate('logs.labels.correlation_id', 'Correlation ID'),
            review_status: translate('logs.labels.review_status', '審查狀態'),
            review_note: translate('logs.labels.review_note', '審查備註'),
            ack_status: translate('logs.labels.ack_status', '確認狀態'),
            change_scope: translate('logs.labels.change_scope', '變更範圍'),
            module: translate('logs.labels.module', '模組'),
            username: translate('logs.labels.actor', '操作者'),
            service: translate('logs.labels.service', '服務'),
            facility: translate('logs.labels.facility', '設施類型'),
            message: translate('logs.labels.message', '訊息'),
            bytes: translate('logs.labels.bytes', '位元組'),
            camera_id: translate('logs.labels.camera_id', '攝影機 ID'),
            require_password_change: translate('logs.labels.require_password_change', '強制變更密碼'),
            role: translate('logs.labels.role', '角色'),
            level: translate('logs.labels.level', '等級'),
            event_code: translate('logs.labels.event_code', '事件代碼'),
            node_name: translate('logs.labels.node_name', '節點名稱'),
        };
        if (map[normalized]) {
            return map[normalized];
        }
        return titleCaseToWords(String(key))
            .replace(/_/g, ' ')
            .replace(/\b\w/g, (ch) => ch.toUpperCase());
    }

    function renderDetailPairs(detail) {
        if (!detail || typeof detail !== 'object' || Array.isArray(detail)) {
            return '';
        }
        const rows = Object.entries(detail)
            .filter(([, value]) => value !== undefined && value !== null && value !== '')
            .map(([key, value]) => {
                let output = value;
                if (typeof value === 'object') {
                    output = JSON.stringify(value);
                } else if (typeof value === 'string') {
                    output = translateLogMessage(value);
                    if (output === value) {
                        output = translateLogValue(value);
                    }
                } else if (typeof value === 'boolean') {
                    output = value ? translate('audit.values.true', '是') : translate('audit.values.false', '否');
                }
                return renderKeyValue(humanizeKey(key), output);
            })
            .join('');

        if (!rows) {
            return '';
        }
        return `<div style="display:flex;flex-direction:column;gap:4px;margin-top:8px;">${rows}</div>`;
    }

    function ensureStaticLabels() {
        const logsPage = page();
        if (!logsPage) {
            return;
        }

        const title = translate('logs.center_title', '日誌與稽核');
        const heading = logsPage.querySelector('h1');
        if (heading) {
            heading.textContent = title;
        }

        document.querySelectorAll('.nav-item[data-page="logs"] .nav-text, .bottom-nav-item[data-page="logs"] .bottom-nav-label')
            .forEach((node) => {
                node.textContent = title;
            });

        const exportLabel = logsPage.querySelector('.page-header .btn.btn-secondary span:last-child');
        if (exportLabel) {
            exportLabel.textContent = translate('logs.export', '匯出日誌');
        }
    }

    function ensureTypeSelect() {
        const select = document.getElementById('log-type-select');
        if (!select) {
            return;
        }
        select.innerHTML = Object.entries(typeCatalog())
            .map(([value, meta]) => `<option value="${value}">${escapeHtml(meta.label)}</option>`)
            .join('');
        select.value = currentType();
    }

    function severityOptions() {
        return [
            ['', translate('logs.filters.all_levels', '所有等級')],
            ['emergency', translate('logs.severity.emergency', '緊急')],
            ['alert', translate('logs.severity.alert', '警報')],
            ['critical', translate('logs.severity.critical', '嚴重')],
            ['error', translate('logs.severity.error', '錯誤')],
            ['warning', translate('logs.severity.warning', '警告')],
            ['notice', translate('logs.severity.notice', '通知')],
            ['info', translate('logs.severity.info', '資訊')],
            ['debug', translate('logs.severity.debug', '除錯')],
        ];
    }

    function ensureSeveritySelect() {
        const select = document.getElementById('log-severity-filter');
        if (!select) {
            return;
        }
        select.innerHTML = severityOptions()
            .map(([value, label]) => `<option value="${value}">${escapeHtml(label)}</option>`)
            .join('');
        select.value = state.logs.severity || '';
        select.style.display = typeMeta().severityParam ? '' : 'none';
    }

    function statusOptions() {
        switch (currentType()) {
            case 'system_logs':
                return [
                    ['', translate('logs.filters.all_review_status', '所有審查狀態')],
                    ['pending', translate('logs.status.pending', '待審查')],
                    ['reviewed', translate('logs.status.reviewed', '已審查')],
                    ['flagged', translate('logs.status.flagged', '需追蹤')],
                ];
            case 'device_logs':
                return [
                    ['', translate('logs.filters.all_ack_status', '所有確認狀態')],
                    ['unacked', translate('logs.status.unacked', '未確認')],
                    ['acked', translate('logs.status.acked', '已確認')],
                ];
            case 'audit_logs':
            case 'config_change_logs':
                return [
                    ['', translate('logs.filters.all_action_status', '所有操作狀態')],
                    ['success', translate('logs.status.success', '成功')],
                    ['failed', translate('logs.status.failed', '失敗')],
                    ['warning', translate('logs.status.warning', '警告')],
                ];
            default:
                return [['', '-']];
        }
    }

    async function loadOptions() {
        if (optionsCache) {
            return optionsCache;
        }
        try {
            const response = await apiGet('/log-center/options');
            optionsCache = response && response.success && response.data ? response.data : {};
        } catch (_) {
            optionsCache = {};
        }
        return optionsCache;
    }

    async function loadRetentionConfig() {
        if (retentionCache) {
            return retentionCache;
        }
        try {
            const response = await apiGet('/system/config');
            const rows = response && response.success && Array.isArray(response.data) ? response.data : [];
            const map = {};
            rows.forEach((row) => {
                if (row && row.config_key) {
                    map[row.config_key] = row.config_value || '';
                }
            });
            retentionCache = map;
        } catch (_) {
            retentionCache = {};
        }
        return retentionCache;
    }

    function insertRetentionContainer() {
        const logsPage = page();
        if (!logsPage || logsPage.querySelector('#logs-retention-summary')) {
            return;
        }
        const container = logsPage.querySelector('.logs-container');
        if (!container) {
            return;
        }
        const retention = document.createElement('div');
        retention.id = 'logs-retention-summary';
        logsPage.insertBefore(retention, container);
    }

    async function renderRetentionSummary() {
        const logsPage = page();
        if (!logsPage) {
            return;
        }
        const summary = logsPage.querySelector('#logs-retention-summary');
        if (!summary) {
            return;
        }

        const config = await loadRetentionConfig();
        const enabled = String(config.log_retention_enabled || '').toLowerCase() === 'true';
        const inputHtml = RETENTION_CONFIG_KEYS.map(({ key, labelKey, fallback }) => `
            <label style="display:flex;flex-direction:column;gap:4px;min-width:140px;">
                <span style="font-size:12px;color:var(--text-muted);">${escapeHtml(translate(labelKey, fallback))}</span>
                <input type="number" min="1" step="1" class="select-input logs-retention-input" data-retention-key="${key}" value="${escapeHtml(config[key] || '')}">
            </label>
        `).join('');

        summary.innerHTML = `
            <div class="glass-card" style="padding:12px 14px;margin-bottom:12px;display:flex;flex-direction:column;gap:12px;">
                <div style="display:flex;justify-content:space-between;align-items:center;gap:12px;flex-wrap:wrap;">
                    <div>
                        <strong>${escapeHtml(translate('logs.retention.title', '保留策略'))}</strong>
                        <div style="font-size:12px;color:var(--text-muted);margin-top:4px;">${escapeHtml(translate('logs.retention.description', '管理各類日誌保存天數與啟用狀態。'))}</div>
                    </div>
                    <label style="display:flex;align-items:center;gap:8px;">
                        <input type="checkbox" id="logs-retention-enabled" ${enabled ? 'checked' : ''}>
                        <span style="color:var(--text-muted);">${escapeHtml(translate('logs.retention.enable', '啟用保留策略'))}</span>
                    </label>
                </div>
                <div style="display:flex;flex-wrap:wrap;gap:12px;">${inputHtml}</div>
                <div style="display:flex;gap:12px;align-items:center;flex-wrap:wrap;">
                    <button type="button" class="btn btn-secondary btn-sm" id="logs-retention-save-btn">${escapeHtml(translate('logs.retention.save', '儲存保留策略'))}</button>
                    <span style="font-size:12px;color:var(--text-muted);">${escapeHtml(translate('logs.retention.hint', '目前先寫入 system_config，後續再接自動清理排程。'))}</span>
                </div>
            </div>
        `;

        const saveButton = summary.querySelector('#logs-retention-save-btn');
        if (!saveButton) {
            return;
        }

        saveButton.addEventListener('click', async () => {
            const enabledInput = summary.querySelector('#logs-retention-enabled');
            const values = {
                log_retention_enabled: enabledInput && enabledInput.checked ? 'true' : 'false',
            };

            summary.querySelectorAll('.logs-retention-input').forEach((input) => {
                const key = input.dataset.retentionKey;
                if (key) {
                    values[key] = String(input.value || '').trim();
                }
            });

            saveButton.disabled = true;
            try {
                for (const [key, value] of Object.entries(values)) {
                    await apiPut(`/system/config/${key}`, { value });
                }
                retentionCache = null;
                await renderRetentionSummary();
                showToast(translate('logs.toast.retention_saved', '保留策略已儲存'), 'success');
            } catch (error) {
                showToast(error.message || translate('logs.toast.retention_save_failed', '保留策略儲存失敗'), 'error');
            } finally {
                saveButton.disabled = false;
            }
        });
    }

    function insertFilterBar() {
        const logsPage = page();
        if (!logsPage || filtersReady) {
            return;
        }
        const container = logsPage.querySelector('.logs-container');
        if (!container) {
            return;
        }

        const bar = document.createElement('div');
        bar.id = 'logs-filter-bar';
        bar.className = 'audit-filters';
        bar.innerHTML = `
            <input type="text" id="logs-search-input" class="search-input" placeholder="${escapeHtml(translate('logs.filters.search', '搜尋日誌內容'))}" style="min-width:220px;">
            <input type="text" id="logs-actor-input" class="search-input" placeholder="${escapeHtml(translate('logs.filters.actor', '操作者'))}" style="min-width:160px;">
            <input type="text" id="logs-device-input" class="search-input" placeholder="${escapeHtml(translate('logs.filters.device', '設備名稱 / IP / MAC'))}" style="min-width:200px;">
            <input type="text" id="logs-scope-input" class="search-input" list="logs-scope-datalist" placeholder="${escapeHtml(translate('logs.filters.scope', '模組 / 來源'))}" style="min-width:180px;">
            <datalist id="logs-scope-datalist"></datalist>
            <select id="logs-status-select" class="select-input" style="min-width:160px;"></select>
            <input type="date" id="logs-date-from" class="select-input" title="${escapeHtml(translate('logs.filters.date_from', '起始日期'))}">
            <input type="date" id="logs-date-to" class="select-input" title="${escapeHtml(translate('logs.filters.date_to', '結束日期'))}">
            <select id="logs-format-select" class="select-input">
                <option value="csv">CSV</option>
                <option value="json">JSON</option>
            </select>
            <button type="button" class="btn btn-secondary btn-sm" id="logs-apply-btn">${escapeHtml(translate('logs.filters.apply', '套用篩選'))}</button>
            <button type="button" class="btn btn-secondary btn-sm" id="logs-clear-btn">${escapeHtml(translate('logs.filters.clear', '清除條件'))}</button>
            <button type="button" class="btn btn-primary btn-sm" id="logs-bundle-btn">${escapeHtml(translate('logs.filters.bundle', '匯出審查包'))}</button>
        `;
        logsPage.insertBefore(bar, container);

        const searchInput = bar.querySelector('#logs-search-input');
        const actorInput = bar.querySelector('#logs-actor-input');
        const deviceInput = bar.querySelector('#logs-device-input');
        const scopeInput = bar.querySelector('#logs-scope-input');
        const statusSelect = bar.querySelector('#logs-status-select');
        const dateFromInput = bar.querySelector('#logs-date-from');
        const dateToInput = bar.querySelector('#logs-date-to');
        const formatSelect = bar.querySelector('#logs-format-select');
        const applyButton = bar.querySelector('#logs-apply-btn');
        const clearButton = bar.querySelector('#logs-clear-btn');
        const bundleButton = bar.querySelector('#logs-bundle-btn');

        let debounce = null;
        const scheduleReload = () => {
            clearTimeout(debounce);
            debounce = setTimeout(() => {
                state.logs.page = 1;
                loadLogs();
            }, 250);
        };

        searchInput.addEventListener('input', (event) => {
            state.logs.search = event.target.value.trim();
            scheduleReload();
        });
        actorInput.addEventListener('input', (event) => {
            state.logs.actor = event.target.value.trim();
            scheduleReload();
        });
        deviceInput.addEventListener('input', (event) => {
            state.logs.device = event.target.value.trim();
            scheduleReload();
        });
        scopeInput.addEventListener('input', (event) => {
            state.logs.scope = event.target.value.trim();
        });
        scopeInput.addEventListener('change', (event) => {
            state.logs.scope = event.target.value.trim();
            state.logs.page = 1;
            loadLogs();
        });
        statusSelect.addEventListener('change', (event) => {
            state.logs.status = event.target.value.trim();
            state.logs.page = 1;
            loadLogs();
        });
        dateFromInput.addEventListener('change', (event) => {
            state.logs.dateFrom = event.target.value;
        });
        dateToInput.addEventListener('change', (event) => {
            state.logs.dateTo = event.target.value;
        });
        formatSelect.addEventListener('change', (event) => {
            state.logs.format = event.target.value || 'csv';
        });
        applyButton.addEventListener('click', () => {
            state.logs.page = 1;
            loadLogs();
        });
        clearButton.addEventListener('click', () => {
            state.logs.search = '';
            state.logs.actor = '';
            state.logs.device = '';
            state.logs.scope = '';
            state.logs.status = '';
            state.logs.dateFrom = '';
            state.logs.dateTo = '';
            state.logs.page = 1;

            searchInput.value = '';
            actorInput.value = '';
            deviceInput.value = '';
            scopeInput.value = '';
            statusSelect.value = '';
            dateFromInput.value = '';
            dateToInput.value = '';
            loadLogs();
        });
        bundleButton.addEventListener('click', exportEvidenceBundle);

        const tbody = document.getElementById('logs-tbody');
        if (tbody) {
            tbody.addEventListener('click', handleLogActionClick);
        }

        filtersReady = true;
    }

    async function syncFilterUI() {
        const logsPage = page();
        if (!logsPage) {
            return;
        }

        const actorInput = logsPage.querySelector('#logs-actor-input');
        const deviceInput = logsPage.querySelector('#logs-device-input');
        const scopeInput = logsPage.querySelector('#logs-scope-input');
        const statusSelect = logsPage.querySelector('#logs-status-select');
        const searchInput = logsPage.querySelector('#logs-search-input');
        const dateFromInput = logsPage.querySelector('#logs-date-from');
        const dateToInput = logsPage.querySelector('#logs-date-to');
        const formatSelect = logsPage.querySelector('#logs-format-select');
        const datalist = logsPage.querySelector('#logs-scope-datalist');
        const meta = typeMeta();

        if (searchInput) searchInput.value = state.logs.search || '';
        if (actorInput) {
            actorInput.value = state.logs.actor || '';
            actorInput.style.display = meta.actorParam ? '' : 'none';
        }
        if (deviceInput) {
            deviceInput.value = state.logs.device || '';
            deviceInput.style.display = meta.deviceParam ? '' : 'none';
        }
        if (scopeInput) {
            scopeInput.value = state.logs.scope || '';
            scopeInput.placeholder = meta.scopePlaceholder;
        }
        if (dateFromInput) dateFromInput.value = state.logs.dateFrom || '';
        if (dateToInput) dateToInput.value = state.logs.dateTo || '';
        if (formatSelect) formatSelect.value = state.logs.format || 'csv';

        if (statusSelect) {
            statusSelect.innerHTML = statusOptions()
                .map(([value, label]) => `<option value="${value}">${escapeHtml(label)}</option>`)
                .join('');
            statusSelect.value = state.logs.status || '';
        }

        const options = await loadOptions();
        if (datalist) {
            const values = Array.isArray(options[meta.scopeOptionsKey]) ? options[meta.scopeOptionsKey] : [];
            datalist.innerHTML = values.map((value) => `<option value="${escapeHtml(value)}"></option>`).join('');
        }
    }

    function updateHeaders() {
        const headers = typeMeta().headers;
        const ths = document.querySelectorAll('#logs thead th');
        if (ths.length < 4) {
            return;
        }
        headers.forEach((text, index) => {
            ths[index].textContent = text;
        });
    }

    function renderEmpty(message) {
        const tbody = document.getElementById('logs-tbody');
        if (tbody) {
            tbody.innerHTML = `<tr><td colspan="4" style="text-align:center;padding:32px;color:var(--text-muted);">${escapeHtml(message)}</td></tr>`;
        }
    }

    function renderLoading() {
        renderEmpty(translate('logs.loading', '載入中...'));
    }

    function renderReviewActions(type, entry) {
        if (type === 'device_logs') {
            const nextAckStatus = String(entry.ack_status || '').toLowerCase() === 'acked' ? 'unacked' : 'acked';
            const label = nextAckStatus === 'acked'
                ? translate('logs.actions.ack', '確認事件')
                : translate('logs.actions.unack', '取消確認');
            return `<div style="display:flex;gap:8px;flex-wrap:wrap;margin-top:10px;">
                <button type="button" class="btn btn-secondary btn-sm" data-action="ack" data-id="${entry.id}" data-status="${nextAckStatus}">${escapeHtml(label)}</button>
            </div>`;
        }

        return `<div style="display:flex;gap:8px;flex-wrap:wrap;margin-top:10px;">
            <button type="button" class="btn btn-secondary btn-sm" data-action="review" data-id="${entry.id}" data-status="pending">${escapeHtml(translate('logs.actions.review_pending', '設為待審查'))}</button>
            <button type="button" class="btn btn-secondary btn-sm" data-action="review" data-id="${entry.id}" data-status="reviewed">${escapeHtml(translate('logs.actions.review_reviewed', '設為已審查'))}</button>
            <button type="button" class="btn btn-secondary btn-sm" data-action="review" data-id="${entry.id}" data-status="flagged">${escapeHtml(translate('logs.actions.review_flagged', '設為需追蹤'))}</button>
        </div>`;
    }

    function renderSystemRow(entry) {
        const context = parseJSON(entry.context_json);
        return `<tr>
            <td>${escapeHtml(formatTime(entry.occurred_at))}</td>
            <td>
                <div style="font-weight:600;">${escapeHtml(translateServiceOrScope(entry.service || '-'))}</div>
                <div style="font-size:12px;color:var(--text-muted);">${escapeHtml(translateLogMessage(entry.event_code || entry.node_name || '-', entry.event_code || ''))}</div>
            </td>
            <td>
                <div style="display:flex;flex-direction:column;gap:8px;">
                    ${renderBadge(auditStatusMeta(entry.level), entry.level || '-')}
                    ${renderBadge(reviewStatusMeta(entry.review_status), translate('logs.status.pending', '待審查'))}
                </div>
            </td>
            <td>
                <div style="font-weight:600;">${escapeHtml(translateLogMessage(entry.message || '-', entry.event_code || ''))}</div>
                ${renderDetailPairs(context)}
                ${renderReviewActions('system_logs', entry)}
            </td>
        </tr>`;
    }

    function renderDeviceRow(entry) {
        const context = parseJSON(entry.context_json);
        return `<tr>
            <td>${escapeHtml(formatTime(entry.occurred_at))}</td>
            <td>
                <div style="font-weight:600;">${escapeHtml(entry.device_name || '-')}</div>
                <div style="font-size:12px;color:var(--text-muted);">${escapeHtml(entry.ip_address || translateServiceOrScope(entry.facility) || '-')}</div>
            </td>
            <td>
                <div style="display:flex;flex-direction:column;gap:8px;">
                    ${renderBadge(auditStatusMeta(entry.severity), entry.severity || '-')}
                    ${renderBadge(ackStatusMeta(entry.ack_status), translate('logs.status.unacked', '未確認'))}
                </div>
            </td>
            <td>
                <div style="font-weight:600;">${escapeHtml(translateLogMessage(entry.normalized_message || entry.raw_message || '-'))}</div>
                ${renderDetailPairs(context)}
                ${renderReviewActions('device_logs', entry)}
            </td>
        </tr>`;
    }

    function renderAuditRow(entry) {
        const detail = parseJSON(entry.detail_json || entry.detail);
        const resource = entry.resource_name || entry.resource || '-';
        return `<tr>
            <td>${escapeHtml(formatTime(entry.occurred_at))}</td>
            <td>
                <div style="font-weight:600;">${escapeHtml(entry.username || '-')}</div>
                <div style="font-size:12px;color:var(--text-muted);">${escapeHtml(translateServiceOrScope(entry.module || '-'))} / ${escapeHtml(resource)}</div>
            </td>
            <td>
                <div style="display:flex;flex-direction:column;gap:8px;">
                    ${renderBadge(auditStatusMeta(entry.status), entry.status || '-')}
                    ${renderBadge(reviewStatusMeta(entry.review_status), translate('logs.status.pending', '待審查'))}
                </div>
            </td>
            <td>
                <div style="font-weight:600;">${escapeHtml(translateActionLabel(entry.action || '-'))}</div>
                ${renderKeyValue(translate('logs.labels.source_ip', '來源 IP'), entry.source_ip)}
                ${renderKeyValue(translate('logs.labels.source_mac', '來源 MAC'), entry.source_mac)}
                ${renderKeyValue(translate('logs.labels.recordset_id', 'Recordset ID'), entry.recordset_id)}
                ${renderKeyValue(translate('logs.labels.correlation_id', 'Correlation ID'), entry.correlation)}
                ${renderDetailPairs(detail)}
                ${renderReviewActions('audit_logs', entry)}
            </td>
        </tr>`;
    }

    function renderConfigChangeRow(entry) {
        const detail = parseJSON(entry.detail_json);
        const oldValues = parseJSON(entry.old_values_json);
        const newValues = parseJSON(entry.new_values_json);
        const target = entry.target_name || `${entry.target_type || '-'} ${entry.target_id || ''}`.trim();
        return `<tr>
            <td>${escapeHtml(formatTime(entry.occurred_at))}</td>
            <td>
                <div style="font-weight:600;">${escapeHtml(translateServiceOrScope(entry.module || '-'))}</div>
                <div style="font-size:12px;color:var(--text-muted);">${escapeHtml(target || '-')}</div>
            </td>
            <td>
                <div style="display:flex;flex-direction:column;gap:8px;">
                    ${renderBadge(auditStatusMeta(entry.status), entry.status || '-')}
                    ${renderBadge(reviewStatusMeta(entry.review_status), translate('logs.status.pending', '待審查'))}
                </div>
            </td>
            <td>
                <div style="font-weight:600;">${escapeHtml(translateActionLabel(entry.action || '-'))}</div>
                ${renderKeyValue(translate('logs.labels.actor', '操作者'), entry.username)}
                ${renderKeyValue(translate('logs.labels.change_scope', '變更範圍'), translateServiceOrScope(entry.change_scope))}
                ${renderKeyValue(translate('logs.labels.recordset_id', 'Recordset ID'), entry.recordset_id)}
                ${renderKeyValue(translate('logs.labels.correlation_id', 'Correlation ID'), entry.correlation_id)}
                ${oldValues ? `<div style="margin-top:8px;"><div style="font-size:12px;color:var(--text-muted);margin-bottom:4px;">${escapeHtml(translate('logs.labels.before', '變更前'))}</div>${renderDetailPairs(oldValues)}</div>` : ''}
                ${newValues ? `<div style="margin-top:8px;"><div style="font-size:12px;color:var(--text-muted);margin-bottom:4px;">${escapeHtml(translate('logs.labels.after', '變更後'))}</div>${renderDetailPairs(newValues)}</div>` : ''}
                ${!oldValues && !newValues ? renderDetailPairs(detail) : ''}
                ${renderReviewActions('config_change_logs', entry)}
            </td>
        </tr>`;
    }

    function renderRows(type, rows) {
        if (!Array.isArray(rows) || rows.length === 0) {
            renderEmpty(translate('logs.empty', '查無符合條件的日誌資料'));
            return;
        }
        const tbody = document.getElementById('logs-tbody');
        if (!tbody) {
            return;
        }
        tbody.innerHTML = rows.map((entry) => {
            switch (type) {
                case 'system_logs': return renderSystemRow(entry);
                case 'device_logs': return renderDeviceRow(entry);
                case 'audit_logs': return renderAuditRow(entry);
                case 'config_change_logs': return renderConfigChangeRow(entry);
                default: return '';
            }
        }).join('');
    }

    function renderPagination(total, pageNumber, limit) {
        const container = document.getElementById('logs-pagination');
        if (!container) {
            return;
        }
        const totalPages = Math.max(1, Math.ceil((Number(total) || 0) / Math.max(Number(limit) || 1, 1)));
        if (totalPages <= 1) {
            container.innerHTML = '';
            return;
        }

        const current = Number(pageNumber) || 1;
        const start = Math.max(1, current - 2);
        const end = Math.min(totalPages, current + 2);
        const html = [];
        html.push(`<button ${current <= 1 ? 'disabled' : ''} data-page-nav="${current - 1}">${escapeHtml(translate('logs.pagination.prev', '上一頁'))}</button>`);
        for (let index = start; index <= end; index += 1) {
            html.push(`<button class="${index === current ? 'active' : ''}" data-page-nav="${index}">${index}</button>`);
        }
        html.push(`<button ${current >= totalPages ? 'disabled' : ''} data-page-nav="${current + 1}">${escapeHtml(translate('logs.pagination.next', '下一頁'))}</button>`);
        container.innerHTML = html.join('');

        container.querySelectorAll('button[data-page-nav]').forEach((button) => {
            button.addEventListener('click', () => {
                const nextPage = Number(button.dataset.pageNav || current);
                if (!Number.isFinite(nextPage) || nextPage < 1 || nextPage === current) {
                    return;
                }
                state.logs.page = nextPage;
                loadLogs();
            });
        });
    }

    function buildParams() {
        const meta = typeMeta();
        const params = new URLSearchParams({
            page: String(state.logs.page || 1),
            limit: String(state.logs.limit || 50),
        });
        if (state.logs.search) params.set('search', state.logs.search);
        if (state.logs.scope && meta.scopeParam) params.set(meta.scopeParam, state.logs.scope);
        if (state.logs.actor && meta.actorParam) params.set(meta.actorParam, state.logs.actor);
        if (state.logs.device && meta.deviceParam) params.set(meta.deviceParam, state.logs.device);
        if (state.logs.status && meta.statusParam) params.set(meta.statusParam, state.logs.status);
        if (state.logs.severity && meta.severityParam) params.set(meta.severityParam, state.logs.severity);
        if (state.logs.dateFrom) params.set('date_from', state.logs.dateFrom);
        if (state.logs.dateTo) params.set('date_to', state.logs.dateTo);
        return params;
    }

    function buildExportParams() {
        const params = new URLSearchParams({ type: currentType() });
        if (state.logs.search) params.set('search', state.logs.search);
        if (state.logs.scope) params.set('scope', state.logs.scope);
        if (state.logs.actor) params.set('actor', state.logs.actor);
        if (state.logs.device) params.set('device', state.logs.device);
        if (state.logs.status) params.set('status', state.logs.status);
        if (state.logs.severity) params.set('severity', state.logs.severity);
        if (state.logs.dateFrom) params.set('start', state.logs.dateFrom);
        if (state.logs.dateTo) params.set('end', state.logs.dateTo);
        return params;
    }

    function exportEvidenceBundle() {
        apiDownload(`/log-center/evidence-bundle?${buildExportParams().toString()}`);
        showToast(translate('logs.toast.bundle_exporting', '正在匯出審查包'), 'info');
    }

    async function reviewLog(type, id, reviewStatus) {
        const note = window.prompt(translate('logs.prompts.review_note', '審查備註（可留空）'), '') ?? '';
        const endpointMap = {
            audit_logs: `/audit-logs/${id}/review`,
            system_logs: `/system-logs/${id}/review`,
            config_change_logs: `/config-change-logs/${id}/review`,
        };
        const endpoint = endpointMap[type];
        if (!endpoint) {
            return;
        }
        await apiPut(endpoint, {
            review_status: reviewStatus,
            review_note: note.trim(),
        });
        showToast(`${translate('logs.toast.review_updated_prefix', '已更新為')}${reviewStatusMeta(reviewStatus).label}`, 'success');
    }

    async function ackDeviceLog(id, ackStatus) {
        const note = window.prompt(translate('logs.prompts.ack_note', '確認備註（可留空）'), '') ?? '';
        await apiPut(`/device-logs/${id}/ack`, {
            ack_status: ackStatus,
            review_note: note.trim(),
        });
        showToast(
            ackStatus === 'acked'
                ? translate('logs.toast.ack_updated', '事件已確認')
                : translate('logs.toast.unack_updated', '事件已改回未確認'),
            'success'
        );
    }

    async function handleLogActionClick(event) {
        const button = event.target.closest('button[data-action]');
        if (!button) {
            return;
        }

        const action = button.dataset.action;
        const id = button.dataset.id;
        const status = button.dataset.status;
        button.disabled = true;
        try {
            if (action === 'review') {
                await reviewLog(currentType(), id, status);
            } else if (action === 'ack') {
                await ackDeviceLog(id, status);
            }
            await loadLogs();
        } catch (error) {
            showToast(error.message || translate('logs.toast.action_failed', '日誌操作失敗'), 'error');
        } finally {
            button.disabled = false;
        }
    }

    async function loadLogs() {
        const logsPage = page();
        if (!logsPage) {
            return;
        }

        ensureLogState();
        ensureStaticLabels();
        ensureTypeSelect();
        ensureSeveritySelect();
        insertFilterBar();
        insertRetentionContainer();
        await syncFilterUI();
        await renderRetentionSummary();
        updateHeaders();
        renderLoading();

        try {
            const response = await apiGet(`${typeMeta().endpoint}?${buildParams().toString()}`);
            const list = response && response.success && Array.isArray(response.data) ? response.data : [];
            renderRows(currentType(), list);
            renderPagination(response.total || 0, response.page || state.logs.page || 1, response.limit || state.logs.limit || 50);
        } catch (error) {
            console.error('[logs] load failed:', error);
            renderEmpty(translate('logs.toast.load_failed', '載入日誌失敗'));
            showToast(error.message || translate('logs.toast.load_failed', '載入日誌失敗'), 'error');
        }
    }

    window.loadLogs = loadLogs;
    window.exportLogEvidenceBundle = exportEvidenceBundle;
})();
