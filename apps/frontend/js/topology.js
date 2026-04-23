// 拓樸圖功能
let topologyData = { nodes: [], links: [] };
let simulation = null;
let svg = null;
let g = null;
let zoom = null;
let linkCreateMode = false;
let selectedSourceNode = null;
// v1.2.1: 拓樸視圖模式 ('force' | 'tree') — 從 localStorage 讀取用戶偏好
let currentTopologyView = (function() {
    try { return localStorage.getItem('topology_default_view') || 'force'; } catch(e) { return 'force'; }
})();

async function loadTopology() {
    try {
        const response = await apiGet('/topology');
        if (response.success) {
            topologyData = response.data;
            console.log('[Topology] Loaded', topologyData.nodes?.length || 0, 'nodes from API');
            console.log('[Topology] Raw API response:', JSON.stringify(response.data, null, 2));
            if (topologyData.nodes && topologyData.nodes.length > 0) {
                console.log('[Topology] First node:', topologyData.nodes[0]);
            }
            processTopologyData();
        }
    } catch (error) {
        console.error('[Topology] Load failed:', error);
        showToast(t('topology.toast.load_failed'), 'error');
    }
}

// processTopologyData 已由 v1.2.2 群組縮圖功能覆寫（見檔案尾部）

function toggleTopologySidebar() {
    const sidebar = document.getElementById('topology-sidebar');
    const container = document.querySelector('.topology-wrapper');
    const btn = sidebar.querySelector('.sidebar-header button');

    sidebar.classList.toggle('collapsed');

    if (sidebar.classList.contains('collapsed')) {
        btn.innerHTML = '▶';
        btn.title = t('topology.expand');
    } else {
        btn.innerHTML = '◀';
        btn.title = t('topology.collapse');
    }

    // Resize map when sidebar state changes (wait for transition)
    setTimeout(() => {
        if (svg) {
            const newWidth = document.getElementById('topology-svg').clientWidth;
            const newHeight = document.getElementById('topology-svg').clientHeight;
            // Update force center if needed or ensure zoom behaves
            if (simulation) {
                simulation.force('center', d3.forceCenter(newWidth / 2, newHeight / 2));
                simulation.alpha(0.3).restart();
            }
        }
    }, 350);
}

function renderTopologySidebar(nodes) {
    const list = document.getElementById('pending-devices-list');
    const count = document.getElementById('pending-count');

    console.log('[Topology] renderTopologySidebar called with', nodes.length, 'devices');
    if (count) count.textContent = nodes.length;
    if (!list) {
        console.error('[Topology] pending-devices-list element not found!');
        return;
    }

    list.innerHTML = '';
    nodes.forEach(node => {
        const item = document.createElement('div');
        item.className = 'pending-device';
        item.draggable = true;
        const displayName = getDeviceDisplayName(node);

        // Use unified icon logic
        const iconHtml = getDeviceIcon(node);

        item.innerHTML = `
            <div class="pending-icon" style="flex-shrink: 0; width: 32px; height: 32px; display: flex; align-items: center; justify-content: center;">
                ${iconHtml}
            </div>
            <div class="pending-info" style="margin-left: 12px;">
                <div class="pending-name" title="${displayName}">${displayName}</div>
                <div class="pending-ip">${node.ip_address}</div>
            </div>
            <div class="pending-action">⋮</div>
        `;

        item.addEventListener('dragstart', (e) => {
            e.dataTransfer.setData('device-id', node.id);
            e.dataTransfer.effectAllowed = 'move';
        });

        list.appendChild(item);
    });
}

function renderTopology(nodes, links) {
    const container = document.getElementById('topology-svg');
    container.innerHTML = '';

    const width = container.clientWidth;
    const height = container.clientHeight;

    // 建立 SVG
    svg = d3.select('#topology-svg')
        .append('svg')
        .attr('width', '100%')
        .attr('height', '100%')
        .on('dragover', (event) => event.preventDefault())
        .on('drop', handleDrop);

    // 定義箭頭
    svg.append('defs').append('marker')
        .attr('id', 'marker-arrowhead')
        .attr('viewBox', '-0 -5 10 10')
        .attr('refX', 40)
        .attr('refY', 0)
        .attr('orient', 'auto')
        .attr('markerWidth', 6)
        .attr('markerHeight', 6)
        .append('path')
        .attr('d', 'M 0,-5 L 10,0 L 0,5')
        .attr('fill', 'var(--text-secondary)');

    // 建立縮放群組
    g = svg.append('g');

    // 縮放功能
    zoom = d3.zoom()
        .scaleExtent([0.1, 4])
        .on('zoom', (event) => {
            g.attr('transform', event.transform);
        });

    svg.call(zoom);

    if (nodes.length === 0) {
        g.append('text')
            .attr('x', width / 2)
            .attr('y', height / 2)
            .attr('text-anchor', 'middle')
            .attr('fill', 'var(--text-muted)')
            .text(t('topology.drag_hint'));
    }

    // 準備節點資料
    const d3Nodes = nodes;
    d3Nodes.forEach(n => {
        n.x = n.x || (width / 2 + (Math.random() - 0.5) * 50);
        n.y = n.y || (height / 2 + (Math.random() - 0.5) * 50);
        n.fx = (n.x && n.x !== 0) ? n.x : null;
        n.fy = (n.y && n.y !== 0) ? n.y : null;
    });

    // 準備連線資料
    const nodeMap = new Map(d3Nodes.map(n => [n.id, n]));
    const d3Links = (links || []).filter(l =>
        nodeMap.has(l.source) && nodeMap.has(l.target)
    ).map(l => ({
        ...l,
        source: nodeMap.get(l.source),
        target: nodeMap.get(l.target)
    }));

    // 建立力導向模擬
    simulation = d3.forceSimulation(d3Nodes)
        .force('link', d3.forceLink(d3Links).id(d => d.id).distance(200))
        .force('charge', d3.forceManyBody().strength(-800))
        .force('center', d3.forceCenter(width / 2, height / 2))
        .force('collision', d3.forceCollide().radius(80));

    // 繪製連線群組
    const linkGroup = g.append('g').attr('class', 'links');

    // 繪製連線
    const link = linkGroup.selectAll('g')
        .data(d3Links)
        .enter()
        .append('g')
        .attr('class', 'link-group')
        .style('cursor', 'pointer')
        .on('click', (event, d) => showLinkDetails(event, d))
        .on('contextmenu', (event, d) => showLinkContextMenu(event, d));

    // Hit area (透明但較寬，方便點擊)
    link.append('line')
        .attr('class', 'link-hit-area')
        .attr('stroke', 'transparent')
        .attr('stroke-width', 20);

    // 可見線條
    link.append('line')
        .attr('class', d => {
            const isSnmpLink = (d.source.snmp_version > 0 && d.target.snmp_version > 0);
            const flowClass = isSnmpLink ? 'snmp-flow' : 'ping-flow';
            return `topology-link ${d.link_type || 'auto'} ${d.is_manual ? 'manual' : ''} ${flowClass}`;
        })
        .attr('stroke-width', d => d.is_manual ? 3 : 2);

    link.append('text')
        .attr('class', 'link-label')
        .attr('text-anchor', 'middle')
        .attr('dy', -8)
        .attr('fill', 'var(--text-secondary)')
        .attr('font-size', '10px')
        .text(d => {
            const totalBandwidth = (d.bandwidth_in || 0) + (d.bandwidth_out || 0);
            if (d.source_if_id || d.target_if_id || totalBandwidth > 0) {
                return `↓${formatTraffic((d.bandwidth_in || 0) * 8)} ↑${formatTraffic((d.bandwidth_out || 0) * 8)}`;
            } else if (d.link_speed > 0) {
                return formatLinkSpeed(d.link_speed);
            }
            return '';
        })
        .attr('fill', d => {
            const hasTraffic = (d.source_if_id || d.target_if_id || ((d.bandwidth_in || 0) + (d.bandwidth_out || 0)) > 0);
            return hasTraffic ? '#10b981' : 'var(--text-secondary)';
        });

    // 繪製節點群組
    const node = g.append('g')
        .attr('class', 'nodes')
        .selectAll('g')
        .data(d3Nodes)
        .enter()
        .append('g')
        .attr('class', d => `topology-node ${d.device_type} ${d.is_online ? '' : 'offline'}`)
        .call(d3.drag()
            .on('start', dragstarted)
            .on('drag', dragged)
            .on('end', dragended))
        .on('click', (event, d) => handleNodeClick(event, d))
        .on('contextmenu', (event, d) => showNodeContextMenu(event, d))
        .on('mouseover', (event, d) => showTooltip(event, d))
        .on('mouseout', hideTooltip);

    // 繪製節點圓形
    node.append('circle')
        .attr('r', 30)
        .attr('fill', d => getNodeColor(d.device_type))
        .attr('stroke', d => getNodeStroke(d.device_type))
        .attr('stroke-width', 3);

    // 繪製節點圖示或圖片
    node.each(function (d) {
        // Use unified device icon logic
        const iconHtml = getDeviceIcon(d);

        // Extract src if it's an image tag
        if (iconHtml.includes('<img')) {
            const srcMatch = iconHtml.match(/src="([^"]+)"/);
            const imagePath = srcMatch ? srcMatch[1] : null;

            if (imagePath) {
                d3.select(this)
                    .append('image')
                    .attr('href', imagePath)
                    .attr('x', -20)
                    .attr('y', -20)
                    .attr('width', 40)
                    .attr('height', 40)
                    .attr('clip-path', 'circle(20px)');
            }
        } else {
            // It's likely an emoji or raw text inside generic html
            // Strip tags just in case, but usually getDeviceIcon returns raw emoji for fallback
            // Or if it returns <span...>{emoji}</span> we need to extract text
            const tempDiv = document.createElement('div');
            tempDiv.innerHTML = iconHtml;
            const textContent = tempDiv.textContent || tempDiv.innerText;

            d3.select(this)
                .append('text')
                .attr('text-anchor', 'middle')
                .attr('dy', 6)
                .attr('font-size', '20px')
                .text(textContent.trim());
        }
    });

    // 繪製節點標籤
    node.append('text')
        .attr('dy', 50)
        .attr('text-anchor', 'middle')
        .attr('font-size', '12px')
        .attr('fill', 'var(--text-primary)')
        .text(d => getDeviceDisplayName(d));

    // 更新位置
    simulation.on('tick', () => {
        // 更新所有 line (包括 hit-area)
        link.selectAll('line')
            .attr('x1', d => d.source.x)
            .attr('y1', d => d.source.y)
            .attr('x2', d => d.target.x)
            .attr('y2', d => d.target.y);

        link.select('text')
            .attr('transform', d => {
                const x = (d.source.x + d.target.x) / 2;
                const y = (d.source.y + d.target.y) / 2;
                const dx = d.target.x - d.source.x;
                const dy = d.target.y - d.source.y;
                let angle = Math.atan2(dy, dx) * 180 / Math.PI;
                if (angle > 90 || angle < -90) {
                    angle += 180;
                }
                return `translate(${x}, ${y}) rotate(${angle})`;
            })
            .attr('x', null)
            .attr('y', null);

        node.attr('transform', d => `translate(${d.x}, ${d.y})`);
    });

    createTooltip();
}

async function handleDrop(event) {
    event.preventDefault();
    const deviceId = parseInt(event.dataTransfer.getData('device-id'));
    if (!deviceId) return;

    // 計算滑鼠在 SVG 中的位置
    const container = document.getElementById('topology-svg');
    const rect = container.getBoundingClientRect();
    const x = event.clientX - rect.left;
    const y = event.clientY - rect.top;

    // 轉換為 Zoom 後的座標
    // transform = {k, x, y}
    // realX = (screenX - tx) / k
    const transform = d3.zoomTransform(svg.node());
    const realX = (x - transform.x) / transform.k;
    const realY = (y - transform.y) / transform.k;

    // 更新後端位置
    try {
        const payload = {
            positions: [{
                device_id: deviceId,
                x: realX,
                y: realY
            }]
        };
        const response = await apiPut('/topology/positions', payload);
        if (response.success) {
            // 更新本地資料並重新渲染
            const nodeIndex = topologyData.nodes.findIndex(n => n.id === deviceId);
            if (nodeIndex !== -1) {
                topologyData.nodes[nodeIndex].x = realX;
                topologyData.nodes[nodeIndex].y = realY;
                processTopologyData(); // 重新整理 sidebar 和 canvas
                showToast(t('topology.toast.device_placed'), 'success');
            }
        }
    } catch (error) {
        showToast(t('topology.toast.update_position_failed'), 'error');
    }
}

function formatLinkSpeed(bps) {
    if (!bps || bps === 0) return '';
    if (bps >= 1000000000000) return `${(bps / 1000000000000).toFixed(0)}T`;
    if (bps >= 1000000000) return `${(bps / 1000000000).toFixed(0)}G`;
    if (bps >= 1000000) return `${(bps / 1000000).toFixed(0)}M`;
    if (bps >= 1000) return `${(bps / 1000).toFixed(0)}K`;
    return `${bps}`;
}

function formatTraffic(bps) {
    if (bps === 0 || bps === '0') return '0';
    return formatLinkSpeed(bps);
}

function handleNodeClick(event, d) {
    if (linkCreateMode) {
        if (!selectedSourceNode) {
            selectedSourceNode = d;
            showToast(t('topology.toast.select_source').replace('{name}', d.name), 'info');
            // 高亮選中的節點
            d3.select(event.target.closest('.topology-node'))
                .select('circle')
                .attr('stroke', '#fbbf24')
                .attr('stroke-width', 5);
        } else if (selectedSourceNode.id !== d.id) {
            showCreateLinkModal(selectedSourceNode, d);
            exitLinkCreateMode();
        } else {
            showToast(t('topology.toast.cannot_self_connect'), 'warning');
        }
    } else {
        viewDevice(d.id);
    }
}

function getNodeColor(type) {
    const colors = {
        router: '#6366f1',
        switch: '#10b981',
        firewall: '#ef4444',
        server: '#3b82f6',
        ipcam: '#f59e0b',
        video_wall: '#8b5cf6',
        other: '#6b7280'
    };
    return colors[type] || colors.other;
}

function getNodeStroke(type) {
    const colors = {
        router: '#818cf8',
        switch: '#34d399',
        firewall: '#f87171',
        server: '#60a5fa',
        ipcam: '#fbbf24',
        video_wall: '#a78bfa',
        other: '#9ca3af'
    };
    return colors[type] || colors.other;
}

function dragstarted(event, d) {
    if (!event.active) simulation.alphaTarget(0.3).restart();
    d.fx = d.x;
    d.fy = d.y;
}

function dragged(event, d) {
    d.fx = event.x;
    d.fy = event.y;
}

function dragended(event, d) {
    if (!event.active) simulation.alphaTarget(0);
    // Pin the node (do NOT nullify fx/fy)
    d.fx = d.x;
    d.fy = d.y;
    d.savedX = d.x;
    d.savedY = d.y;
    // Auto-save immediately
    saveTopologyPositions(true);
}

function createTooltip() {
    if (!document.getElementById('topology-tooltip')) {
        const tooltip = document.createElement('div');
        tooltip.id = 'topology-tooltip';
        tooltip.className = 'topology-tooltip';
        document.querySelector('.topology-container').appendChild(tooltip);
    }
}

function showTooltip(event, d) {
    const tooltip = document.getElementById('topology-tooltip');
    const sysInfo = d.sys_name ? `<p>SysName: ${d.sys_name}</p>` : '';
    const uptime = d.sys_uptime ? `<p>Uptime: ${d.sys_uptime}</p>` : '';
    const location = d.sys_location ? `<p>${t('devices.location') || '位置'}: ${d.sys_location}</p>` : '';

    tooltip.innerHTML = `
        <h4>${escapeHtml(getDeviceDisplayName(d))}</h4>
        <p>IP: ${d.ip_address}</p>
        <p>${t('topology.detail.link_type') || '類型'}: ${d.device_type}</p>
        ${sysInfo}
        ${uptime}
        ${location}
        <div class="status ${d.is_online ? 'online' : 'offline'}">
            ${d.is_online ? '● ' + (t('devices.online') || '線上') : '○ ' + (t('devices.offline') || '離線')}
        </div>
    `;
    tooltip.classList.add('visible');

    const rect = event.target.getBoundingClientRect();
    const containerRect = document.querySelector('.topology-container').getBoundingClientRect();
    tooltip.style.left = (rect.left - containerRect.left + 40) + 'px';
    tooltip.style.top = (rect.top - containerRect.top - 20) + 'px';
}

function hideTooltip() {
    const tooltip = document.getElementById('topology-tooltip');
    if (tooltip) tooltip.classList.remove('visible');
}

// ===== 手動連線功能 =====
function startLinkCreateMode() {
    linkCreateMode = true;
    selectedSourceNode = null;
    showToast(t('topology.toast.manual_mode_hint'), 'info');
    document.querySelector('.topology-container').classList.add('link-create-mode');
}

function exitLinkCreateMode() {
    linkCreateMode = false;
    selectedSourceNode = null;
    document.querySelector('.topology-container').classList.remove('link-create-mode');
    // 重置節點高亮
    d3.selectAll('.topology-node circle')
        .attr('stroke', d => getNodeStroke(d.device_type))
        .attr('stroke-width', 3);
}

async function showCreateLinkModal(source, target) {
    // 取得兩個設備的介面
    let sourceInterfaces = [];
    let targetInterfaces = [];

    try {
        const [srcResp, tgtResp] = await Promise.all([
            apiGet(`/topology/device/${source.id}/interfaces`),
            apiGet(`/topology/device/${target.id}/interfaces`)
        ]);
        if (srcResp.success) sourceInterfaces = srcResp.data || [];
        if (tgtResp.success) targetInterfaces = tgtResp.data || [];
    } catch (e) {
        console.error('Failed to load interfaces', e);
    }

    const sourceOptions = sourceInterfaces.map(i =>
        `<option value="${i.id}" data-speed="${i.if_speed}">${i.if_name || i.if_index} (${i.if_speed_text}, ${i.if_status})</option>`
    ).join('');

    const targetOptions = targetInterfaces.map(i =>
        `<option value="${i.id}" data-speed="${i.if_speed}">${i.if_name || i.if_index} (${i.if_speed_text}, ${i.if_status})</option>`
    ).join('');

    const content = `
        <form id="create-link-form" onsubmit="createManualLink(event, ${source.id}, ${target.id})">
            <div class="link-endpoints">
                <div class="endpoint">
                    <div class="endpoint-icon" style="display: flex; justify-content: center;">${getDeviceIcon(source)}</div>
                    <div class="endpoint-name">${getDeviceDisplayName(source)}</div>
                </div>
                <div class="link-arrow">↔</div>
                <div class="endpoint">
                    <div class="endpoint-icon" style="display: flex; justify-content: center;">${getDeviceIcon(target)}</div>
                    <div class="endpoint-name">${getDeviceDisplayName(target)}</div>
                </div>
            </div>
            
            <div class="form-row">
                <div class="form-group">
                    <label>${t('topology.form.source_if') || '來源介面'}</label>
                    <select name="source_if_id">
                        <option value="">${t('topology.form.select_if_placeholder') || '-- 選擇介面 --'}</option>
                        ${sourceOptions}
                    </select>
                </div>
                <div class="form-group">
                    <label>${t('topology.form.target_if') || '目標介面'}</label>
                    <select name="target_if_id">
                        <option value="">${t('topology.form.select_if_placeholder') || '-- 選擇介面 --'}</option>
                        ${targetOptions}
                    </select>
                </div>
            </div>
            
            <div class="form-group">
                <label>${t('topology.form.link_speed') || '線路速度'}</label>
                <select name="link_speed">
                    <option value="0">${t('topology.form.auto_detect') || '自動偵測'}</option>
                    <option value="10000000">10 Mbps</option>
                    <option value="100000000">100 Mbps</option>
                    <option value="1000000000">1 Gbps</option>
                    <option value="10000000000">10 Gbps</option>
                    <option value="25000000000">25 Gbps</option>
                    <option value="40000000000">40 Gbps</option>
                    <option value="100000000000">100 Gbps</option>
                </select>
            </div>

            <div class="form-group">
                <label>${t('topology.form.peer_type') || '連線關係'}</label>
                <select name="link_type">
                    <option value="manual">${t('topology.form.peer_type_manual') || '一般連線'}</option>
                    <option value="ha">${t('topology.form.peer_type_ha') || 'HA / 備援（同層）'}</option>
                    <option value="vrf">${t('topology.form.peer_type_vrf') || 'VRF（同層虛擬路由）'}</option>
                    <option value="spine_leaf">${t('topology.form.peer_type_spine_leaf') || 'Spine-Leaf Stack（同層）'}</option>
                </select>
                <small style="color:var(--text-muted);margin-top:4px;display:block">${t('topology.form.peer_type_hint') || 'HA / VRF / Spine-Leaf 在樹狀圖中會顯示為同層節點'}</small>
            </div>

            <div class="form-group">
                <label>${t('topology.form.link_label') || '連線標籤（選填）'}</label>
                <input type="text" name="link_label" placeholder="${t('topology.form.link_label_placeholder') || '例如: WAN Link, 主幹線路'}">
            </div>
            
            <div class="form-actions">
                <button type="button" class="btn btn-secondary" onclick="hideModal()">${t('topology.form.cancel') || '取消'}</button>
                <button type="submit" class="btn btn-primary">${t('topology.form.create_link') || '建立連線'}</button>
            </div>
        </form>
    `;
    openModal('topology.modal.manual_link_title', content);
}

async function createManualLink(event, sourceId, targetId) {
    event.preventDefault();
    const form = event.target;
    const formData = new FormData(form);

    const sourceIfId = formData.get('source_if_id');
    const targetIfId = formData.get('target_if_id');

    const link = {
        source_device_id: sourceId,
        target_device_id: targetId,
        source_if_id: sourceIfId ? parseInt(sourceIfId) : null,
        target_if_id: targetIfId ? parseInt(targetIfId) : null,
        link_speed: parseInt(formData.get('link_speed')) || 0,
        link_label: formData.get('link_label') || '',
        link_type: formData.get('link_type') || 'manual'
    };

    try {
        const response = await apiPost('/topology/links', link);
        if (response.success) {
            showToast(t('topology.toast.link_created'), 'success');
            hideModal();
            loadTopology();
        }
    } catch (error) {
        showToast(error.message || t('topology.toast.create_link_failed'), 'error');
    }
}

function showLinkDetails(event, link) {
    event.stopPropagation();

    const sourceName = link.source.sys_name || link.source.name || link.source.ip_address;
    const targetName = link.target.sys_name || link.target.name || link.target.ip_address;
    const speedText = link.link_speed ? formatLinkSpeedFull(link.link_speed) : t('topology.detail.unknown') || '未知';
    const peerTypeLabels = {
        ha:         t('topology.form.peer_type_ha')         || 'HA / 備援',
        vrf:        t('topology.form.peer_type_vrf')        || 'VRF（虛擬路由）',
        spine_leaf: t('topology.form.peer_type_spine_leaf') || 'Spine-Leaf Stack',
        manual:     t('topology.detail.manual_link')        || '手動建立',
    };
    const typeText = link.is_manual
        ? (peerTypeLabels[link.link_type] || peerTypeLabels.manual)
        : (t('topology.detail.auto_link') || '自動發現');

    // Traffic info (matches logic in the toast version)
    const inBw = link.bandwidth_in ? formatLinkSpeed(link.bandwidth_in * 8) : '0';
    const outBw = link.bandwidth_out ? formatLinkSpeed(link.bandwidth_out * 8) : '0';

    const content = `
        <div class="link-details">
            <div class="device-details-row">
                <label>${t('topology.detail.source_device') || '來源設備'}</label>
                <span>${sourceName}</span>
            </div>
            <div class="device-details-row">
                <label>${t('topology.detail.target_device') || '目標設備'}</label>
                <span>${targetName}</span>
            </div>
            <div class="device-details-row">
                <label>${t('topology.detail.link_type') || '連線類型'}</label>
                <span>${typeText}</span>
            </div>
            <div class="device-details-row">
                <label>${t('topology.detail.speed_limit') || '線路速限'}</label>
                <span>${speedText}</span>
            </div>
            <div class="device-details-row">
                <label>${t('topology.detail.current_traffic') || '當前流量 (IN/OUT)'}</label>
                <span class="status-online">↓ ${inBw} / ↑ ${outBw}</span>
            </div>
            ${link.source_if_name ? `<div class="device-details-row"><label>${t('topology.form.source_if') || '來源介面'}</label><span>${link.source_if_name}</span></div>` : ''}
            ${link.target_if_name ? `<div class="device-details-row"><label>${t('topology.form.target_if') || '目標介面'}</label><span>${link.target_if_name}</span></div>` : ''}
            ${link.link_label ? `<div class="device-details-row"><label>${t('topology.detail.label') || '標籤'}</label><span>${link.link_label}</span></div>` : ''}
        </div>
        <div class="form-actions mt-4">
            <button class="btn btn-danger" onclick="confirmDeleteLink(${link.id})">
                ${t('topology.detail.delete_link') || '🗑️ 刪除此連線'}
            </button>
            <button class="btn btn-secondary" onclick="closeModal()">
                ${t('topology.detail.close') || '關閉'}
            </button>
        </div>
    `;
    openModal('topology.modal.link_detail_title', content);
}

function formatLinkSpeedFull(bps) {
    if (!bps || bps === 0) return t('topology.detail.not_set') || '未設定';
    if (bps >= 1000000000000) return `${(bps / 1000000000000).toFixed(0)} Tbps`;
    if (bps >= 1000000000) return `${(bps / 1000000000).toFixed(0)} Gbps`;
    if (bps >= 1000000) return `${(bps / 1000000).toFixed(0)} Mbps`;
    if (bps >= 1000) return `${(bps / 1000).toFixed(0)} Kbps`;
    return `${bps} bps`;
}

function confirmDeleteLink(linkId) {
    showConfirm(t('topology.confirm.delete_link_simple') || '確定要刪除此連線嗎？', () => deleteLink(linkId));
}

async function deleteLink(linkId) {
    try {
        const response = await apiDelete(`/topology/links/${linkId}`);
        if (response.success) {
            showToast(t('topology.toast.link_deleted'), 'success');
            hideModal();
            loadTopology();
        }
    } catch (error) {
        showToast(t('topology.toast.delete_failed'), 'error');
    }
}

// Ensure global access for HTML onclick handlers
window.confirmDeleteLink = confirmDeleteLink;
window.confirmDeleteContextLink = confirmDeleteContextLink;
window.deleteLink = deleteLink;
window.deleteTopologyLink = deleteTopologyLink;

// ===== 其他功能 =====
async function saveTopologyPositions(silent = false) {
    if (!topologyData.nodes || topologyData.nodes.length === 0) {
        if (!silent) showToast(t('topology.toast.no_devices_to_save'), 'warning');
        return;
    }

    const positions = topologyData.nodes.map(n => ({
        device_id: n.id,
        x: n.savedX || n.x || 0,
        y: n.savedY || n.y || 0
    }));

    try {
        const response = await apiPut('/topology/positions', { positions });
        if (response.success) {
            if (!silent) showToast(t('topology.toast.positions_saved'), 'success');
        }
    } catch (error) {
        if (!silent) showToast(t('topology.toast.save_positions_failed'), 'error');
    }
}

function resetTopologyView() {
    if (svg && zoom) {
        svg.transition()
            .duration(750)
            .call(zoom.transform, d3.zoomIdentity);
    }
}

async function discoverTopology() {
    try {
        await apiPost('/topology/discover');
        showToast(t('topology.toast.discovery_started'), 'info');
        setTimeout(loadTopology, 3000);
    } catch (error) {
        showToast(t('topology.toast.discovery_failed'), 'error');
    }
}

// ===== 自動更新拓樸流量 =====
let topologyRefreshInterval = null;

function startTopologyRefresh() {
    // 清除舊的interval
    if (topologyRefreshInterval) {
        clearInterval(topologyRefreshInterval);
    }

    // 每5秒更新一次流量
    topologyRefreshInterval = setInterval(async () => {
        if (state.currentPage === 'topology') {
            await updateTopologyTraffic();
        }
    }, 5000);
}

function stopTopologyRefresh() {
    if (topologyRefreshInterval) {
        clearInterval(topologyRefreshInterval);
        topologyRefreshInterval = null;
    }
}

async function updateTopologyTraffic() {
    try {
        const response = await apiGet('/topology');
        if (response.success && response.data.links) {
            const links = response.data.links;
            topologyData.links = links;

            if (!g) return;

            if (currentTopologyView === 'tree') {
                // 樹狀視圖：更新 .tree-link-label 文字
                g.selectAll('.tree-link-grp').each(function(d) {
                    const s = d.source.data.id;
                    const t2 = d.target.data.id;
                    const ld = links.find(l =>
                        (l.source === s && l.target === t2) ||
                        (l.source === t2 && l.target === s)
                    );
                    if (!ld) return;
                    const bwIn  = (ld.bandwidth_in  || 0) * 8;
                    const bwOut = (ld.bandwidth_out || 0) * 8;
                    const hasBw = bwIn > 0 || bwOut > 0 || ld.source_if_id || ld.target_if_id;
                    const label = hasBw
                        ? `↓${formatTraffic(bwIn)} ↑${formatTraffic(bwOut)}`
                        : (ld.link_speed > 0 ? formatLinkSpeed(ld.link_speed) : '');
                    d3.select(this).select('.tree-link-label')
                        .text(label)
                        .attr('fill', hasBw ? '#10b981' : 'var(--text-secondary)');
                });
            } else {
                // 力導向視圖
                g.selectAll('.link-group text').each(function(d) {
                    const linkData = links.find(l =>
                        (l.source === d.source.id && l.target === d.target.id) ||
                        (l.source === d.target.id && l.target === d.source.id)
                    );
                    if (!linkData) return;
                    const totalBandwidth = (linkData.bandwidth_in || 0) + (linkData.bandwidth_out || 0);
                    if (linkData.source_if_id || linkData.target_if_id || totalBandwidth > 0) {
                        d3.select(this).text(
                            `↓${formatTraffic((linkData.bandwidth_in || 0) * 8)} ↑${formatTraffic((linkData.bandwidth_out || 0) * 8)}`
                        ).attr('fill', '#10b981');
                    } else if (linkData.link_speed > 0) {
                        d3.select(this).text(formatLinkSpeed(linkData.link_speed))
                            .attr('fill', 'var(--text-secondary)');
                    }
                });
            }
        }
    } catch (error) {
        console.error('更新拓樸流量失敗:', error);
    }
}

// 當頁面載入拓樸時啟動自動更新
const originalLoadTopology = loadTopology;
loadTopology = async function () {
    await originalLoadTopology();
    startTopologyRefresh();
};

// ===== Context Menu for Topology =====
let contextMenuElement = null;

function createContextMenu() {
    if (!contextMenuElement) {
        contextMenuElement = document.createElement('div');
        contextMenuElement.id = 'topology-context-menu';
        contextMenuElement.className = 'topology-context-menu';
        contextMenuElement.style.display = 'none';
        document.body.appendChild(contextMenuElement);

        // Close context menu when clicking elsewhere
        document.addEventListener('click', hideContextMenu);
    }
    return contextMenuElement;
}

function showNodeContextMenu(event, d) {
    event.preventDefault();
    event.stopPropagation();

    const menu = createContextMenu();
    const displayName = d.sys_name || d.name || d.ip_address;

    menu.innerHTML = `
        <div class="context-menu-header">${displayName}</div>
        <div class="context-menu-item" onclick="viewDevice(${d.id}); hideContextMenu();">
            📋 查看詳情
        </div>
        ${!isViewer() ? `<div class="context-menu-item danger" onclick="confirmRemoveFromTopology(${d.id}, '${displayName.replace(/'/g, "\\'")}')">
            🗑️ 從拓樸移除
        </div>` : ''}
    `;

    menu.style.left = event.clientX + 'px';
    menu.style.top = event.clientY + 'px';
    menu.style.display = 'block';
}

function showLinkContextMenu(event, d) {
    event.preventDefault();
    event.stopPropagation();

    const menu = createContextMenu();
    const sourceName = d.source.sys_name || d.source.name || d.source.ip_address;
    const targetName = d.target.sys_name || d.target.name || d.target.ip_address;

    menu.innerHTML = `
        <div class="context-menu-header">${sourceName} ↔ ${targetName}</div>
        <div class="context-menu-item" onclick="showLinkDetailsModal(${JSON.stringify(d).replace(/"/g, '&quot;')}); hideContextMenu();">
            📋 連線詳情
        </div>
        ${!isViewer() ? `<div class="context-menu-item danger" onclick="confirmDeleteContextLink(${d.id}, '${sourceName}', '${targetName}')">
            🗑️ 刪除連線
        </div>` : ''}
    `;

    menu.style.left = event.clientX + 'px';
    menu.style.top = event.clientY + 'px';
    menu.style.display = 'block';
}

// Redundant showLinkDetails removed (consolidated with modal version)

function showLinkDetailsModal(linkData) {
    // Could be expanded to show a full modal with link details
    const sourceName = linkData.source?.name || linkData.source;
    const targetName = linkData.target?.name || linkData.target;
    showToast(t('topology.toast.link_info').replace('{source}', sourceName).replace('{target}', targetName), 'info');
}

function hideContextMenu() {
    if (contextMenuElement) {
        contextMenuElement.style.display = 'none';
    }
}

function confirmRemoveFromTopology(deviceId, deviceName) {
    hideContextMenu();

    if (confirm(t('topology.confirm.remove_device').replace('{name}', deviceName))) {
        removeDeviceFromTopology(deviceId);
    }
}

async function removeDeviceFromTopology(deviceId) {
    try {
        // Reset device position to (-1, -1) to move it back to pending list
        const payload = {
            positions: [{
                device_id: deviceId,
                x: -1,
                y: -1
            }]
        };

        const response = await apiPut('/topology/positions', payload);
        if (response.success) {
            // Update local data
            const nodeIndex = topologyData.nodes.findIndex(n => n.id === deviceId);
            if (nodeIndex !== -1) {
                topologyData.nodes[nodeIndex].x = -1;
                topologyData.nodes[nodeIndex].y = -1;
            }

            processTopologyData(); // Re-render sidebar and canvas
            showToast(t('topology.toast.device_removed'), 'success');
        }
    } catch (error) {
        console.error('Remove device from topology error:', error);
        showToast(t('topology.toast.remove_device_failed'), 'error');
    }
}

function confirmDeleteContextLink(linkId, sourceName, targetName) {
    hideContextMenu();

    if (confirm(t('topology.confirm.delete_link').replace('{source}', sourceName).replace('{target}', targetName))) {
        deleteTopologyLink(linkId);
    }
}

async function deleteTopologyLink(linkId) {
    try {
        const response = await apiDelete(`/topology/links/${linkId}`);
        if (response.success) {
            // Remove link from local data
            const linkIndex = topologyData.links.findIndex(l => l.id === linkId);
            if (linkIndex !== -1) {
                topologyData.links.splice(linkIndex, 1);
            }

            processTopologyData(); // Re-render
            showToast(t('topology.toast.link_deleted'), 'success');
        }
    } catch (error) {
        console.error('Delete topology link error:', error);
        showToast(t('topology.toast.delete_link_failed'), 'error');
    }
}


// ===== Legend Toggle =====
function toggleTopologyLegend() {
    const legend = document.getElementById('topology-legend');
    if (legend) {
        legend.classList.toggle('collapsed');
        // Persist state
        const collapsed = legend.classList.contains('collapsed');
        try { localStorage.setItem('topology_legend_collapsed', collapsed); } catch(e) {}
    }
}

// Restore legend state and topology view preference on page load
document.addEventListener('DOMContentLoaded', function() {
    try {
        const collapsed = localStorage.getItem('topology_legend_collapsed') === 'true';
        if (collapsed) {
            const legend = document.getElementById('topology-legend');
            if (legend) legend.classList.add('collapsed');
        }
    } catch(e) {}

    // 恢復視圖按鈕狀態（依 currentTopologyView，已在頂部從 localStorage 讀取）
    setTimeout(() => {
        document.querySelectorAll('.topo-view-btn').forEach(btn => btn.classList.remove('active'));
        const activeBtn = document.getElementById(`topo-btn-${currentTopologyView}`);
        if (activeBtn) activeBtn.classList.add('active');
        _updateTopoSetDefaultBtns(currentTopologyView);
    }, 0);
});

/** 根據目前視圖更新「設為預設」按鈕的顯示：只顯示非目前預設的那個 */
function _updateTopoSetDefaultBtns(currentView) {
    try {
        const savedDefault = localStorage.getItem('topology_default_view') || 'force';
        const btnForce = document.getElementById('topo-set-default-force');
        const btnTree  = document.getElementById('topo-set-default-tree');
        // 目前視圖下，只顯示「設為預設」按鈕（且當前非預設時才明顯）
        if (btnForce) {
            btnForce.style.display = currentView === 'force' ? 'inline-flex' : 'none';
            btnForce.style.opacity = savedDefault === 'force' ? '0.4' : '0.85';
            btnForce.title = savedDefault === 'force' ? '已是預設：網狀圖' : '設網狀圖為預設';
        }
        if (btnTree) {
            btnTree.style.display = currentView === 'tree' ? 'inline-flex' : 'none';
            btnTree.style.opacity = savedDefault === 'tree' ? '0.4' : '0.85';
            btnTree.title = savedDefault === 'tree' ? '已是預設：樹狀圖' : '設樹狀圖為預設';
        }
    } catch(e) {}
}
window._updateTopoSetDefaultBtns = _updateTopoSetDefaultBtns;

// ===== v1.2.1: 拓樸視圖切換 =====

/**
 * 切換拓樸視圖 ('force' | 'tree')
 * @param {string} view - 'force' | 'tree'
 * @param {boolean} saveAsDefault - true = 同時設為預設視圖（持久化）
 */
function switchTopologyView(view, saveAsDefault) {
    currentTopologyView = view;

    // 更新按鈕狀態
    document.querySelectorAll('.topo-view-btn').forEach(btn => btn.classList.remove('active'));
    const activeBtn = document.getElementById(`topo-btn-${view}`);
    if (activeBtn) activeBtn.classList.add('active');

    // 若要設為預設，存入 localStorage
    if (saveAsDefault) {
        try { localStorage.setItem('topology_default_view', view); } catch(e) {}
        showToast(view === 'tree' ? '已設為預設：樹狀圖' : '已設為預設：網狀圖', 'success');
    }

    // 更新「設為預設」按鈕的顯示狀態
    _updateTopoSetDefaultBtns(view);

    // 重新渲染
    processTopologyData();
}
window.switchTopologyView = switchTopologyView;

/**
 * 樹狀拓樸渲染 - 使用 D3 tree layout
 */
function renderTreeTopology(nodes, links) {
    const container = document.getElementById('topology-svg');
    container.innerHTML = '';

    if (!nodes || nodes.length === 0) {
        container.innerHTML = `<div style="display:flex;align-items:center;justify-content:center;height:100%;color:var(--text-muted);font-size:1rem;">${t('topology.drag_hint') || '請從左側拖曳設備到此處'}</div>`;
        return;
    }

    const width = container.clientWidth || 800;
    const height = container.clientHeight || 600;

    // 建立 SVG
    svg = d3.select('#topology-svg')
        .append('svg')
        .attr('width', '100%')
        .attr('height', '100%');

    // 縮放群組
    g = svg.append('g');
    zoom = d3.zoom()
        .scaleExtent([0.1, 4])
        .on('zoom', (event) => { g.attr('transform', event.transform); });
    svg.call(zoom);

    // --- 找樹根 ---
    // 優先選 router / firewall；否則選入度最小的節點
    let rootNode = nodes.find(n => n.device_type === 'router') ||
                   nodes.find(n => n.device_type === 'firewall');

    if (!rootNode) {
        // 計算每個節點的入度
        const inDegree = {};
        nodes.forEach(n => { inDegree[n.id] = 0; });
        links.forEach(l => {
            const tgtId = typeof l.target === 'object' ? l.target.id : l.target;
            if (inDegree[tgtId] !== undefined) inDegree[tgtId]++;
        });
        rootNode = nodes.reduce((a, b) => (inDegree[a.id] <= inDegree[b.id] ? a : b));
    }

    // --- 全局識別同層群組（HA / VRF / Spine-Leaf）---
    // 判斷依據（優先順序）：
    //   1. 手動連線的 link_type 為 'ha' / 'vrf' / 'spine_leaf' → 明確同層
    //   2. 若無明確 link_type，退而以拓樸推斷：同 device_type 直連且同屬
    //      HA_CAPABLE_TYPES → 視為同層（向下相容）
    const HA_CAPABLE_TYPES  = new Set(['router', 'firewall', 'switch']);
    const PEER_LINK_TYPES   = new Set(['ha', 'vrf', 'spine_leaf']);

    const nodeTypeMap = new Map(nodes.map(n => [n.id, n.device_type]));

    // Union-Find
    const ufParent = new Map(nodes.map(n => [n.id, n.id]));
    function ufFind(id) {
        if (ufParent.get(id) !== id) ufParent.set(id, ufFind(ufParent.get(id)));
        return ufParent.get(id);
    }
    function ufUnion(a, b) { ufParent.set(ufFind(a), ufFind(b)); }

    links.forEach(l => {
        const s   = typeof l.source === 'object' ? l.source.id : l.source;
        const t2  = typeof l.target === 'object' ? l.target.id : l.target;
        const lt  = l.link_type || '';

        if (PEER_LINK_TYPES.has(lt)) {
            // 只有明確標記為同層連線（ha/vrf/spine_leaf）才合併
            ufUnion(s, t2);
        }
    });

    // 收集所有節點的群組（size > 1 才是真正的同層群組）
    const haGroups = new Map(); // groupRoot → Set<nodeId>
    nodes.forEach(n => {
        const gr = ufFind(n.id);
        if (!haGroups.has(gr)) haGroups.set(gr, new Set());
        haGroups.get(gr).add(n.id);
    });

    // nodeId → 所屬群組 root（只有群組 size > 1 才記錄）
    const nodeToHaGroup = new Map();
    haGroups.forEach((members, gr) => {
        if (members.size > 1) members.forEach(id => nodeToHaGroup.set(id, gr));
    });

    // 群組內部連線 → 建樹時排除（不建 parent-child）
    const haInternalLinkSet = new Set();
    links.forEach(l => {
        const s  = typeof l.source === 'object' ? l.source.id : l.source;
        const t2 = typeof l.target === 'object' ? l.target.id : l.target;
        const sg = nodeToHaGroup.get(s);
        const tg = nodeToHaGroup.get(t2);
        if (sg && sg === tg) {
            haInternalLinkSet.add(`${s}-${t2}`);
            haInternalLinkSet.add(`${t2}-${s}`);
        }
    });

    // 每個群組選一個「代表節點」(primary)，其餘為 peer
    // 優先：名稱含 hq/primary/master/active/main/spine；其次取第一個
    const haPrimaryOf = new Map(); // groupRoot → primaryId
    const haPeersOf   = new Map(); // groupRoot → Set<peerId>
    haGroups.forEach((members, gr) => {
        if (members.size <= 1) return;
        const arr = [...members];
        const primary = arr.find(id => {
            const name = (nodes.find(n => n.id === id)?.name || '').toLowerCase();
            return /hq|primary|master|active|main|spine/.test(name);
        }) || arr[0];
        haPrimaryOf.set(gr, primary);
        haPeersOf.set(gr, new Set(arr.filter(id => id !== primary)));
    });

    // rootNode 若是群組的 peer（非 primary），換成 primary
    const rootGroup = nodeToHaGroup.get(rootNode.id);
    if (rootGroup) {
        const primary = haPrimaryOf.get(rootGroup);
        if (primary) rootNode = nodes.find(n => n.id === primary) || rootNode;
    }

    // --- 建立樹結構（BFS，感知 HA 群組）---
    const nodeMap = new Map(nodes.map(n => [n.id, { ...n, children: [] }]));

    // 所有 peer 一開始就 visited（不被別的節點搶走）
    const allPeerIds = new Set();
    haPeersOf.forEach(peers => peers.forEach(id => allPeerIds.add(id)));

    const visited = new Set([rootNode.id, ...allPeerIds]);
    const queue = [rootNode.id];

    // 根層：把 rootNode 所在群組的 peer 掛為 rootNode 的兄弟（放進 rootNode.children 最前面）
    if (rootGroup) {
        haPeersOf.get(rootGroup)?.forEach(pid => {
            nodeMap.get(rootNode.id).children.push(nodeMap.get(pid));
            queue.push(pid);
        });
    }

    // 建立鄰接表（排除群組內部連線）
    const adj = new Map();
    nodes.forEach(n => adj.set(n.id, []));
    links.forEach(l => {
        const s = typeof l.source === 'object' ? l.source.id : l.source;
        const t2 = typeof l.target === 'object' ? l.target.id : l.target;
        if (haInternalLinkSet.has(`${s}-${t2}`)) return; // 排除 HA 群組內連線
        if (adj.has(s)) adj.get(s).push(t2);
        if (adj.has(t2)) adj.get(t2).push(s);
    });

    while (queue.length > 0) {
        const cur = queue.shift();
        const neighbors = adj.get(cur) || [];
        neighbors.forEach(nid => {
            if (!visited.has(nid) && nodeMap.has(nid)) {
                visited.add(nid);
                nodeMap.get(cur).children.push(nodeMap.get(nid));
                queue.push(nid);

                // 若 nid 是某個 HA/Stack 群組的 primary，把同群 peer 掛為其兄弟
                const nidGroup = nodeToHaGroup.get(nid);
                if (nidGroup && haPrimaryOf.get(nidGroup) === nid) {
                    haPeersOf.get(nidGroup)?.forEach(pid => {
                        // peer 已在 allPeerIds → visited，不會被重複處理
                        nodeMap.get(nid).children.push(nodeMap.get(pid));
                        queue.push(pid); // peer 也要往下展開自己的子節點
                    });
                }
            }
        });
    }

    // 未被訪問的節點（孤立節點）直接掛到 root 下
    nodes.forEach(n => {
        if (!visited.has(n.id) && n.id !== rootNode.id) {
            nodeMap.get(rootNode.id).children.push(nodeMap.get(n.id));
        }
    });

    // 建立 D3 hierarchy
    const root = d3.hierarchy(nodeMap.get(rootNode.id), d => {
        // 如果節點被折疊，則不展開其子節點
        if (treeCollapsedNodes.has(d.id) || treeCollapsedNodes.has(String(d.id))) return null;
        if (treeCollapsedNodes.has(d.id) || treeCollapsedNodes.has(String(d.id))) return null;
        return d.children;
    });
    const treeLayout = d3.tree().size([width - 120, height - 120]);
    treeLayout(root);

    // 建立 link 查詢 map: "srcId-tgtId" → linkData
    const linkDataMap = new Map();
    links.forEach(l => {
        const s = typeof l.source === 'object' ? l.source.id : l.source;
        const t2 = typeof l.target === 'object' ? l.target.id : l.target;
        linkDataMap.set(`${s}-${t2}`, l);
        linkDataMap.set(`${t2}-${s}`, l);
    });

    function getLinkData(d) {
        const s = d.source.data.id;
        const t2 = d.target.data.id;
        return linkDataMap.get(`${s}-${t2}`) || linkDataMap.get(`${t2}-${s}`) || null;
    }

    function countHiddenOffline(node) {
        if (!(treeCollapsedNodes.has(node.id) || treeCollapsedNodes.has(String(node.id)))) return 0;
        let count = 0;
        const walk = (children) => {
            (children || []).forEach(child => {
                if (!child.is_online) count++;
                walk(child.children);
            });
        };
        walk(node.children);
        return count;
    }

    // 繪製連線
    const linkGroup = g.append('g').attr('class', 'tree-links');
    const treeLinks = root.links();

    const linkPaths = linkGroup.selectAll('g.tree-link-grp')
        .data(treeLinks)
        .enter()
        .append('g')
        .attr('class', 'tree-link-grp');

    // 曲線路徑
    linkPaths.append('path')
        .attr('class', d => {
            const ld = getLinkData(d);
            const isSnmp = ld && (d.source.data.snmp_version > 0 && d.target.data.snmp_version > 0);
            const flowCls = isSnmp ? 'snmp-flow' : 'ping-flow';
            return `tree-link topology-link auto ${flowCls}`;
        })
        .attr('d', d3.linkVertical()
            .x(d => d.x + 60)
            .y(d => d.y + 60))
        .attr('fill', 'none')
        .attr('stroke-width', 1.5);

    // 流量標籤（顯示在曲線中點）
    linkPaths.append('text')
        .attr('class', 'tree-link-label link-label')
        .attr('text-anchor', 'middle')
        .attr('font-size', '10px')
        .attr('dy', -4)
        .attr('fill', d => {
            const ld = getLinkData(d);
            if (!ld) return 'var(--text-secondary)';
            const bw = (ld.bandwidth_in || 0) + (ld.bandwidth_out || 0);
            return (bw > 0 || ld.source_if_id || ld.target_if_id) ? '#10b981' : 'var(--text-secondary)';
        })
        .attr('transform', d => {
            const mx = ((d.source.x + 60) + (d.target.x + 60)) / 2;
            const my = ((d.source.y + 60) + (d.target.y + 60)) / 2;
            return `translate(${mx},${my})`;
        })
        .text(d => {
            const ld = getLinkData(d);
            if (!ld) return '';
            const bwIn  = (ld.bandwidth_in  || 0) * 8;
            const bwOut = (ld.bandwidth_out || 0) * 8;
            if (bwIn > 0 || bwOut > 0 || ld.source_if_id || ld.target_if_id) {
                return `↓${formatTraffic(bwIn)} ↑${formatTraffic(bwOut)}`;
            }
            if (ld.link_speed > 0) return formatLinkSpeed(ld.link_speed);
            return '';
        });

    // 繪製節點群組
    const nodeGroup = g.append('g').attr('class', 'tree-nodes');
    const treeDescendants = root.descendants().map(d => {
        d.hiddenOfflineCount = countHiddenOffline(d.data);
        return d;
    });
    const nodeEl = nodeGroup.selectAll('g')
        .data(treeDescendants)
        .enter()
        .append('g')
        .attr('class', d => {
            const cls = ['topology-node', d.data.device_type];
            if (!d.data.is_online) cls.push('offline');
            if (treeCollapsedNodes.has(d.data.id) || treeCollapsedNodes.has(String(d.data.id))) cls.push('collapsed');
            if ((d.hiddenOfflineCount || 0) > 0) cls.push('has-hidden-alert');
            return cls.join(' ');
        })
        .attr('transform', d => `translate(${d.x + 60}, ${d.y + 60})`)
        .on('click', (event, d) => {
            // v1.2.2: 左鍵改為收縮/展開子樹（如果有點可以縮的話）
            // 右鍵原本已提供 viewDevice
            if (d.data.children && d.data.children.length > 0) {
                toggleTreeNodeCollapse(d.data.id);
            } else if (treeCollapsedNodes.has(d.data.id) || treeCollapsedNodes.has(String(d.data.id))) {
                // 如果是已折疊狀態，即使當前沒 children 物件（被 hierarchy 過濾了），也要能點擊展開
                toggleTreeNodeCollapse(d.data.id);
            } else {
                // 葉子節點或無子設備：保持檢視功能或不動作
                // 由於 user 提到右鍵已有詳情，這裡我們改為「如果沒東西縮放，就還是開詳情」或者「乾脆只做縮放」
                // 為了用戶體驗，我們在沒有子節點時保留 viewDevice
                if (typeof viewDevice === 'function') viewDevice(d.data.id);
            }
        })
        .on('mouseover', (event, d) => showTooltip(event, d.data))
        .on('mouseout', hideTooltip)
        .style('cursor', 'pointer');

    // 節點圓形
    nodeEl.append('circle')
        .attr('r', 28)
        .attr('fill', d => getNodeColor(d.data.device_type))
        .attr('stroke', d => getNodeStroke(d.data.device_type))
        .attr('stroke-width', 3);

    // 節點圖示
    nodeEl.each(function(d) {
        const iconHtml = getDeviceIcon(d.data);
        if (iconHtml && iconHtml.includes('<img')) {
            const srcMatch = iconHtml.match(/src="([^"]+)"/);
            if (srcMatch) {
                d3.select(this).append('image')
                    .attr('href', srcMatch[1])
                    .attr('x', -18).attr('y', -18)
                    .attr('width', 36).attr('height', 36)
                    .attr('clip-path', 'circle(18px)');
            }
        } else {
            const tmp = document.createElement('div');
            tmp.innerHTML = iconHtml || '';
            const txt = (tmp.textContent || tmp.innerText || '?').trim();
            d3.select(this).append('text')
                .attr('text-anchor', 'middle').attr('dy', 6)
                .attr('font-size', '18px').text(txt);
        }
    });

    // 節點標籤
    nodeEl.append('text')
        .attr('dy', 46)
        .attr('text-anchor', 'middle')
        .attr('font-size', '11px')
        .attr('fill', 'var(--text-primary)')
        .text(d => getDeviceDisplayName(d.data));

    // 離線標記
    nodeEl.filter(d => !d.data.is_online)
        .append('circle')
        .attr('r', 8).attr('cx', 20).attr('cy', -20)
        .attr('fill', '#ef4444')
        .append('title').text('離線');

    // 增加「折疊」視覺標記 (+ 號)
    const collapsedTreeNodeEl = nodeEl.filter(d => treeCollapsedNodes.has(d.data.id) || treeCollapsedNodes.has(String(d.data.id)));
    collapsedTreeNodeEl
        .append('circle')
        .attr('r', 10).attr('cx', 20).attr('cy', 20)
        .attr('class', 'tree-collapse-badge')
        .attr('fill', 'var(--primary-color)')
        .attr('stroke', '#fff').attr('stroke-width', 1);
    
    collapsedTreeNodeEl
        .append('text')
        .attr('x', 20).attr('y', 23.5)
        .attr('text-anchor', 'middle')
        .attr('fill', '#fff').attr('font-size', '12px').attr('font-weight', 'bold')
        .text('+');

    collapsedTreeNodeEl.filter(d => (d.hiddenOfflineCount || 0) > 0)
        .append('circle')
        .attr('class', 'tree-collapse-ring')
        .attr('r', 34);

    collapsedTreeNodeEl.filter(d => (d.hiddenOfflineCount || 0) > 0)
        .append('circle')
        .attr('class', 'tree-collapse-dot')
        .attr('r', 5).attr('cx', 24).attr('cy', -24);

    createTooltip();
}
window.renderTreeTopology = renderTreeTopology;

// ===== v1.2.2: 群組縮圖功能（子網設備折疊） =====

/** 群組模式是否啟用 */
let groupModeEnabled = (function() {
    try { return localStorage.getItem('topology_group_mode') !== 'false'; } catch(e) { return true; }
})();

/** v1.2.2: 樹狀圖節點折疊狀態 (nodeId Set) */
let treeCollapsedNodes = (function() {
    try {
        const saved = localStorage.getItem('topology_tree_collapsed_nodes');
        const parsed = saved ? JSON.parse(saved) : [];
        const normalized = new Set();
        (parsed || []).forEach(v => {
            normalized.add(v);
            normalized.add(String(v));
            const n = Number(v);
            if (!Number.isNaN(n)) normalized.add(n);
        });
        return normalized;
    } catch(e) { return new Set(); }
})();

/** 目前折疊的子網群組 (/24 prefix) */
let collapsedGroups = (function() {
    try {
        const saved = localStorage.getItem('topology_collapsed_groups');
        return new Set(saved ? JSON.parse(saved) : []);
    } catch(e) { return new Set(); }
})();

function saveTreeCollapsedNodes() {
    try {
        const keys = [...treeCollapsedNodes].map(v => String(v));
        localStorage.setItem('topology_tree_collapsed_nodes', JSON.stringify([...new Set(keys)]));
    } catch(e) {}
}

function toggleTreeNodeCollapse(nodeId) {
    const numericId = Number(nodeId);
    const hasNode = treeCollapsedNodes.has(nodeId)
        || treeCollapsedNodes.has(String(nodeId))
        || (!Number.isNaN(numericId) && treeCollapsedNodes.has(numericId));
    if (hasNode) {
        treeCollapsedNodes.delete(nodeId);
        treeCollapsedNodes.delete(String(nodeId));
        if (!Number.isNaN(numericId)) treeCollapsedNodes.delete(numericId);
    } else {
        treeCollapsedNodes.add(nodeId);
        treeCollapsedNodes.add(String(nodeId));
        if (!Number.isNaN(numericId)) treeCollapsedNodes.add(numericId);
    }
    saveTreeCollapsedNodes();
    processTopologyData(); // 重新渲染
}

function saveCollapsedGroups() {
    try { localStorage.setItem('topology_collapsed_groups', JSON.stringify([...collapsedGroups])); } catch(e) {}
}

function saveGroupMode() {
    try { localStorage.setItem('topology_group_mode', String(groupModeEnabled)); } catch(e) {}
}

/** 取得 IP 的 /24 前綴，例如 "192.168.1.5" → "192.168.1" */
function getSubnetPrefix(ip) {
    if (!ip) return null;
    const parts = ip.split('.');
    if (parts.length < 3) return null;
    return parts.slice(0, 3).join('.');
}

/** 將 nodes 按子網分群，回傳 Map<prefix, nodes[]>（僅含 ≥2 台設備的子網） */
function buildSubnetGroups(nodes) {
    const map = new Map();
    nodes.forEach(n => {
        const prefix = getSubnetPrefix(n.ip_address);
        if (!prefix) return;
        if (!map.has(prefix)) map.set(prefix, []);
        map.get(prefix).push(n);
    });
    // 只保留 ≥2 台的群組
    const result = new Map();
    map.forEach((members, prefix) => {
        if (members.length >= 2) result.set(prefix, members);
    });
    return result;
}

/**
 * 切換群組模式（工具列按鈕觸發）
 */
function toggleGroupMode() {
    groupModeEnabled = !groupModeEnabled;
    saveGroupMode();
    // 更新按鈕樣式
    const btn = document.getElementById('topo-btn-group');
    if (btn) {
        btn.classList.toggle('active', groupModeEnabled);
        btn.title = groupModeEnabled ? '關閉群組縮圖' : '開啟群組縮圖';
    }
    processTopologyData();
}
window.toggleGroupMode = toggleGroupMode;

/**
 * 切換某群組的折疊狀態
 * @param {string} prefix - 子網前綴，例如 "192.168.1"
 */
function toggleGroupCollapse(prefix) {
    if (collapsedGroups.has(prefix)) {
        collapsedGroups.delete(prefix);
    } else {
        collapsedGroups.add(prefix);
    }
    saveCollapsedGroups();
    processTopologyData();
}
window.toggleGroupCollapse = toggleGroupCollapse;

/**
 * 為 force 視圖加入群組縮圖渲染
 * 在原始節點渲染前呼叫，回傳需要隱藏（被折疊）的 node id set
 */
function renderGroupBubbles(gSelection, subnetGroups, nodePositions) {
    const hiddenNodeIds = new Set();

    subnetGroups.forEach((members, prefix) => {
        if (!collapsedGroups.has(prefix)) return; // 未折疊 → 不渲染 bubble

        // 計算群組中心（平均各節點位置）
        let cx = 0, cy = 0, validCount = 0;
        members.forEach(n => {
            const pos = nodePositions.get(n.id);
            if (pos) { cx += pos.x; cy += pos.y; validCount++; }
        });
        if (validCount === 0) return;
        cx /= validCount;
        cy /= validCount;

        const hasOffline = members.some(n => !n.is_online);
        const offlineCount = members.filter(n => !n.is_online).length;
        const radius = Math.max(50, 20 + members.length * 12);

        members.forEach(n => hiddenNodeIds.add(n.id));

        const grp = gSelection.append('g')
            .attr('class', `topo-group-bubble ${hasOffline ? 'has-alert' : ''}`)
            .attr('transform', `translate(${cx},${cy})`)
            .style('cursor', 'pointer')
            .on('click', () => toggleGroupCollapse(prefix));

        // 背景圓（虛線外框）
        grp.append('circle')
            .attr('r', radius)
            .attr('class', 'group-bg-circle');

        // 若有異常設備：呼吸燈外圈
        if (hasOffline) {
            grp.append('circle')
                .attr('r', radius + 8)
                .attr('class', 'group-alert-ring');
        }

        // 中央圖示文字
        grp.append('text')
            .attr('text-anchor', 'middle')
            .attr('dy', -4)
            .attr('class', 'group-bubble-icon')
            .text('⬡');

        // 設備數量標籤
        grp.append('text')
            .attr('text-anchor', 'middle')
            .attr('dy', 14)
            .attr('class', 'group-bubble-count')
            .text(`${members.length} 台設備`);

        // 子網標籤
        grp.append('text')
            .attr('text-anchor', 'middle')
            .attr('dy', radius + 18)
            .attr('class', 'group-bubble-label')
            .text(`${prefix}.x`);

        // 告警標籤
        if (hasOffline) {
            grp.append('text')
                .attr('text-anchor', 'middle')
                .attr('dy', radius + 32)
                .attr('class', 'group-alert-label')
                .text(`⚠ ${offlineCount} 台離線`);
        }

        // 提示：點擊展開
        grp.append('title').text(`子網 ${prefix}.x（${members.length} 台）- 點擊展開`);
    });

    return hiddenNodeIds;
}

/**
 * 在拓樸控制區域注入「群組」按鈕（僅注入一次）
 */
function ensureGroupModeButton() {
    if (document.getElementById('topo-btn-group')) return;
    // 找 topo-view-btn 群組所在容器
    const viewBtns = document.querySelector('.topo-view-btns') || document.querySelector('.topology-controls');
    if (!viewBtns) return;
    const btn = document.createElement('button');
    btn.id = 'topo-btn-group';
    btn.className = 'topo-view-btn' + (groupModeEnabled ? ' active' : '');
    btn.title = groupModeEnabled ? '關閉群組縮圖' : '開啟群組縮圖';
    btn.textContent = '⬡';
    btn.onclick = toggleGroupMode;
    viewBtns.appendChild(btn);
}

// ===== 修補 renderTopology：支援群組折疊 =====
// 覆寫 processTopologyData，注入群組模式邏輯
function processTopologyData() {
    const deployedNodes = [];
    const pendingNodes = [];

    if (topologyData.nodes) {
        topologyData.nodes.forEach((node) => {
            const x = Number(node.x) || 0;
            const y = Number(node.y) || 0;
            if ((x === -1 && y === -1) || (x === 0 && y === 0)) {
                pendingNodes.push(node);
            } else {
                deployedNodes.push(node);
            }
        });
    }

    renderTopologySidebar(pendingNodes);

    // 注入群組按鈕（若尚未注入）
    setTimeout(ensureGroupModeButton, 100);

    if (currentTopologyView === 'tree') {
        // 樹狀圖使用全部節點（不依賴座標），與 monitor.html 行為一致
        const allNodes = topologyData.nodes || [];
        renderTreeTopology(allNodes, topologyData.links || []);
    } else {
        renderTopologyWithGroups(deployedNodes, topologyData.links || []);
    }
}

/**
 * 加強版 renderTopology：支援子網群組折疊
 */
function renderTopologyWithGroups(nodes, links) {
    if (!groupModeEnabled) {
        renderTopology(nodes, links);
        return;
    }

    const subnetGroups = buildSubnetGroups(nodes);
    if (subnetGroups.size === 0) {
        // 沒有任何可分群的子網，退回原始渲染
        renderTopology(nodes, links);
        return;
    }

    const container = document.getElementById('topology-svg');
    container.innerHTML = '';

    const width = container.clientWidth;
    const height = container.clientHeight;

    svg = d3.select('#topology-svg')
        .append('svg')
        .attr('width', '100%')
        .attr('height', '100%')
        .on('dragover', (event) => event.preventDefault())
        .on('drop', handleDrop);

    svg.append('defs').append('marker')
        .attr('id', 'marker-arrowhead')
        .attr('viewBox', '-0 -5 10 10')
        .attr('refX', 40)
        .attr('refY', 0)
        .attr('orient', 'auto')
        .attr('markerWidth', 6)
        .attr('markerHeight', 6)
        .append('path')
        .attr('d', 'M 0,-5 L 10,0 L 0,5')
        .attr('fill', 'var(--text-secondary)');

    g = svg.append('g');

    zoom = d3.zoom()
        .scaleExtent([0.1, 4])
        .on('zoom', (event) => { g.attr('transform', event.transform); });
    svg.call(zoom);

    if (nodes.length === 0) {
        g.append('text')
            .attr('x', width / 2).attr('y', height / 2)
            .attr('text-anchor', 'middle')
            .attr('fill', 'var(--text-muted)')
            .text(t('topology.drag_hint'));
    }

    // 確定哪些節點被折疊（隱藏）
    const collapsedNodeIds = new Set();
    subnetGroups.forEach((members, prefix) => {
        if (collapsedGroups.has(prefix)) {
            members.forEach(n => collapsedNodeIds.add(n.id));
        }
    });

    // 只顯示未折疊的節點
    const visibleNodes = nodes.filter(n => !collapsedNodeIds.has(n.id));

    visibleNodes.forEach(n => {
        n.x = n.x || (width / 2 + (Math.random() - 0.5) * 50);
        n.y = n.y || (height / 2 + (Math.random() - 0.5) * 50);
        n.fx = (n.x && n.x !== 0) ? n.x : null;
        n.fy = (n.y && n.y !== 0) ? n.y : null;
    });

    // 只保留兩端都可見的連線
    const nodeMap = new Map(visibleNodes.map(n => [n.id, n]));
    const d3Links = (links || []).filter(l =>
        nodeMap.has(l.source) && nodeMap.has(l.target)
    ).map(l => ({
        ...l,
        source: nodeMap.get(l.source),
        target: nodeMap.get(l.target)
    }));

    simulation = d3.forceSimulation(visibleNodes)
        .force('link', d3.forceLink(d3Links).id(d => d.id).distance(200))
        .force('charge', d3.forceManyBody().strength(-800))
        .force('center', d3.forceCenter(width / 2, height / 2))
        .force('collision', d3.forceCollide().radius(80));

    // 建立連線
    const linkGroup = g.append('g').attr('class', 'links');
    const link = linkGroup.selectAll('g')
        .data(d3Links)
        .enter().append('g')
        .attr('class', 'link-group')
        .style('cursor', 'pointer')
        .on('click', (event, d) => showLinkDetails(event, d))
        .on('contextmenu', (event, d) => showLinkContextMenu(event, d));

    link.append('line').attr('class', 'link-hit-area').attr('stroke', 'transparent').attr('stroke-width', 20);
    link.append('line')
        .attr('class', d => {
            const isSnmpLink = (d.source.snmp_version > 0 && d.target.snmp_version > 0);
            return `topology-link ${d.link_type || 'auto'} ${d.is_manual ? 'manual' : ''} ${isSnmpLink ? 'snmp-flow' : 'ping-flow'}`;
        })
        .attr('stroke-width', d => d.is_manual ? 3 : 2);

    link.append('text').attr('class', 'link-label').attr('text-anchor', 'middle').attr('dy', -8)
        .attr('fill', 'var(--text-secondary)').attr('font-size', '10px')
        .text(d => {
            const total = (d.bandwidth_in || 0) + (d.bandwidth_out || 0);
            if (d.source_if_id || d.target_if_id || total > 0)
                return `↓${formatTraffic((d.bandwidth_in || 0) * 8)} ↑${formatTraffic((d.bandwidth_out || 0) * 8)}`;
            if (d.link_speed > 0) return formatLinkSpeed(d.link_speed);
            return '';
        })
        .attr('fill', d => {
            const has = (d.source_if_id || d.target_if_id || ((d.bandwidth_in || 0) + (d.bandwidth_out || 0)) > 0);
            return has ? '#10b981' : 'var(--text-secondary)';
        });

    // 建立節點
    const node = g.append('g').attr('class', 'nodes')
        .selectAll('g').data(visibleNodes).enter().append('g')
        .attr('class', d => `topology-node ${d.device_type} ${d.is_online ? '' : 'offline'}`)
        .call(d3.drag().on('start', dragstarted).on('drag', dragged).on('end', dragended))
        .on('click', (event, d) => handleNodeClick(event, d))
        .on('contextmenu', (event, d) => showNodeContextMenu(event, d))
        .on('mouseover', (event, d) => showTooltip(event, d))
        .on('mouseout', hideTooltip);

    node.append('circle').attr('r', 30)
        .attr('fill', d => getNodeColor(d.device_type))
        .attr('stroke', d => getNodeStroke(d.device_type))
        .attr('stroke-width', 3);

    node.each(function(d) {
        const iconHtml = getDeviceIcon(d);
        if (iconHtml.includes('<img')) {
            const srcMatch = iconHtml.match(/src="([^"]+)"/);
            if (srcMatch) {
                d3.select(this).append('image')
                    .attr('href', srcMatch[1])
                    .attr('x', -20).attr('y', -20)
                    .attr('width', 40).attr('height', 40)
                    .attr('clip-path', 'circle(20px)');
            }
        } else {
            const tmp = document.createElement('div');
            tmp.innerHTML = iconHtml;
            d3.select(this).append('text')
                .attr('text-anchor', 'middle').attr('dy', 6)
                .attr('font-size', '20px')
                .text((tmp.textContent || tmp.innerText || '').trim());
        }
    });

    node.append('text').attr('dy', 50).attr('text-anchor', 'middle')
        .attr('font-size', '12px').attr('fill', 'var(--text-primary)')
        .text(d => getDeviceDisplayName(d));

    // 渲染群組 bubble（折疊的子網）
    // 先計算所有節點位置（包含折疊的，以上次 x/y 為準）
    const allNodePositions = new Map(nodes.map(n => [n.id, { x: n.x || width/2, y: n.y || height/2 }]));
    const bubbleLayer = g.append('g').attr('class', 'group-bubbles');
    renderGroupBubbles(bubbleLayer, subnetGroups, allNodePositions);

    // 為群組增加展開/折疊按鈕 in sidebar（subnet groups panel）
    renderSubnetGroupPanel(subnetGroups);

    simulation.on('tick', () => {
        link.selectAll('line')
            .attr('x1', d => d.source.x).attr('y1', d => d.source.y)
            .attr('x2', d => d.target.x).attr('y2', d => d.target.y);

        link.select('text')
            .attr('transform', d => {
                const x = (d.source.x + d.target.x) / 2;
                const y = (d.source.y + d.target.y) / 2;
                const dx = d.target.x - d.source.x;
                const dy = d.target.y - d.source.y;
                let angle = Math.atan2(dy, dx) * 180 / Math.PI;
                if (angle > 90 || angle < -90) angle += 180;
                return `translate(${x}, ${y}) rotate(${angle})`;
            }).attr('x', null).attr('y', null);

        node.attr('transform', d => `translate(${d.x}, ${d.y})`);

        // 動態更新 bubble 位置（依節點模擬位置重算中心）
        // (僅在模擬穩定後更新，避免閃爍)
    });

    createTooltip();
}

/**
 * 在 sidebar 下方增加「子網群組」面板，顯示可折疊的子網列表
 */
function renderSubnetGroupPanel(subnetGroups) {
    const sidebar = document.getElementById('topology-sidebar');
    if (!sidebar) return;

    // 移除舊 panel
    const oldPanel = sidebar.querySelector('.subnet-group-panel');
    if (oldPanel) oldPanel.remove();

    if (subnetGroups.size === 0) return;

    const panel = document.createElement('div');
    panel.className = 'subnet-group-panel';

    const header = document.createElement('div');
    header.className = 'subnet-group-header';
    header.innerHTML = `<span>子網群組</span><span class="badge">${subnetGroups.size}</span>`;
    panel.appendChild(header);

    subnetGroups.forEach((members, prefix) => {
        const isCollapsed = collapsedGroups.has(prefix);
        const hasOffline = members.some(n => !n.is_online);
        const offlineCount = members.filter(n => !n.is_online).length;

        const item = document.createElement('div');
        item.className = `subnet-group-item${hasOffline ? ' has-alert' : ''}`;
        item.innerHTML = `
            <div class="subnet-group-info">
                ${hasOffline ? '<span class="subnet-alert-dot"></span>' : '<span class="subnet-ok-dot"></span>'}
                <span class="subnet-label">${prefix}.x</span>
                <span class="subnet-count">${members.length} 台</span>
                ${hasOffline ? `<span class="subnet-offline-badge">${offlineCount} 離線</span>` : ''}
            </div>
            <button class="subnet-toggle-btn" onclick="toggleGroupCollapse('${prefix}')" title="${isCollapsed ? '展開群組' : '折疊群組'}">
                ${isCollapsed ? '▶' : '▼'}
            </button>
        `;
        panel.appendChild(item);
    });

    sidebar.appendChild(panel);
}
