// Homelable-style topology canvas for the NMS vanilla frontend.
// Keeps the existing NMS topology API contract and global function names.

let topologyData = { nodes: [], links: [] };
let topologyCanvas = null;
let topologyViewport = null;
let topologyLinksSvg = null;
let topologyNodesLayer = null;
let topologyState = {
    scale: 1,
    x: 0,
    y: 0,
    draggingNode: null,
    draggingCanvas: false,
    dragStart: null,
    lastPointer: null,
    linkCreateMode: false,
    selectedSourceNode: null,
    selectedSourceAnchor: null,
    view: (function () {
        try { return localStorage.getItem('topology_default_view') || 'force'; } catch (e) { return 'force'; }
    })()
};

const topologyPalette = {
    router: { bg: '#eef2ff', fg: '#4f46e5', border: '#818cf8' },
    switch: { bg: '#ecfdf5', fg: '#047857', border: '#34d399' },
    firewall: { bg: '#fef2f2', fg: '#b91c1c', border: '#f87171' },
    server: { bg: '#eff6ff', fg: '#1d4ed8', border: '#60a5fa' },
    access_point: { bg: '#f5f3ff', fg: '#6d28d9', border: '#a78bfa' },
    ipcam: { bg: '#fffbeb', fg: '#b45309', border: '#fbbf24' },
    pdu: { bg: '#f0fdfa', fg: '#0f766e', border: '#2dd4bf' },
    ups: { bg: '#fff7ed', fg: '#c2410c', border: '#fb923c' },
    other: { bg: '#f8fafc', fg: '#475569', border: '#94a3b8' },
    unknown: { bg: '#f8fafc', fg: '#475569', border: '#94a3b8' }
};

const fallbackTypeLabels = {
    router: 'Router',
    switch: 'Switch',
    firewall: 'Firewall',
    server: 'Server',
    access_point: 'AP',
    ipcam: 'Camera',
    pdu: 'PDU',
    ups: 'UPS',
    other: 'Device',
    unknown: 'Device'
};

const nodeBox = { width: 200, height: 92 };

async function loadTopology() {
    try {
        const response = await apiGet('/topology');
        if (!response || !response.success) {
            throw new Error(response && response.message ? response.message : 'Topology API failed');
        }
        topologyData = normalizeTopology(response.data || {});
        renderTopology();
    } catch (error) {
        console.error('[Topology] load failed:', error);
        showToast(safeT('topology.toast.load_failed', 'Topology load failed'), 'error');
    }
}

function normalizeTopology(data) {
    const nodes = Array.isArray(data.nodes) ? data.nodes.map((node) => ({ ...node })) : [];
    const links = Array.isArray(data.links) ? data.links.map((link) => ({ ...link })) : [];
    const width = getCanvasSize().width || 1000;
    const height = getCanvasSize().height || 650;
    nodes.forEach((node, index) => {
        node.id = normalizeId(node.id);
        node.x = Number(node.x);
        node.y = Number(node.y);
        if (!Number.isFinite(node.x) || !Number.isFinite(node.y)) {
            node.x = 0;
            node.y = 0;
        }
        if (!isPlaced(node)) {
            const angle = (index / Math.max(nodes.length, 1)) * Math.PI * 2;
            const radius = Math.max(180, Math.min(width, height) * 0.3);
            node.previewX = width / 2 + Math.cos(angle) * radius;
            node.previewY = height / 2 + Math.sin(angle) * radius;
        }
    });

    // Keep newly discovered/unpositioned devices in the left sidebar. They
    // enter the canvas only after the user drops them or topology discovery
    // explicitly assigns positions. Do not auto-layout the whole inventory.

    return { nodes, links };
}

function renderTopology() {
    setupTopologyDom();
    renderTopologySidebar();
    renderTopologyCanvas();
    updateTopologyViewButtons();
}

function setupTopologyDom() {
    const root = document.getElementById('topology-svg');
    if (!root) return;

    root.innerHTML = `
        <div class="homelable-stage" id="homelable-stage">
            <div class="homelable-grid"></div>
            <div class="homelable-viewport" id="homelable-viewport">
                <svg class="homelable-links" id="homelable-links"></svg>
                <div class="homelable-nodes" id="homelable-nodes"></div>
            </div>
            <div class="homelable-empty" id="homelable-empty"></div>
        </div>
    `;

    topologyCanvas = document.getElementById('homelable-stage');
    topologyViewport = document.getElementById('homelable-viewport');
    topologyLinksSvg = document.getElementById('homelable-links');
    topologyNodesLayer = document.getElementById('homelable-nodes');

    topologyCanvas.addEventListener('pointerdown', onCanvasPointerDown);
    topologyCanvas.addEventListener('pointermove', onCanvasPointerMove);
    topologyCanvas.addEventListener('pointerup', onCanvasPointerUp);
    topologyCanvas.addEventListener('pointercancel', onCanvasPointerUp);
    topologyCanvas.addEventListener('wheel', onCanvasWheel, { passive: false });
    topologyCanvas.addEventListener('dragover', (event) => event.preventDefault());
    topologyCanvas.addEventListener('drop', handleDrop);

    applyViewportTransform();
}

function renderTopologySidebar() {
    const list = document.getElementById('pending-devices-list');
    const count = document.getElementById('pending-count');
    if (!list) return;

    const pendingNodes = topologyData.nodes.filter((node) => !isPlaced(node));
    if (count) count.textContent = pendingNodes.length;

    list.innerHTML = '';
    pendingNodes.forEach((node) => {
        const item = document.createElement('div');
        item.className = 'pending-device homelable-pending-device';
        item.draggable = true;
        item.dataset.deviceId = node.id;
        item.innerHTML = `
            <div class="pending-icon">${deviceIconMarkup(node)}</div>
            <div class="pending-info">
                <div class="pending-name" title="${escapeAttr(deviceName(node))}">${escapeHtml(deviceName(node))}</div>
                <div class="pending-ip">${escapeHtml(node.ip_address || '-')}</div>
            </div>
            <div class="pending-action">Drag</div>
        `;
        item.addEventListener('dragstart', (event) => {
            event.dataTransfer.setData('device-id', node.id);
            event.dataTransfer.effectAllowed = 'move';
        });
        list.appendChild(item);
    });
}

function renderTopologyCanvas() {
    if (!topologyNodesLayer || !topologyLinksSvg) return;

    const allNodes = topologyData.nodes || [];
    const nodes = topologyState.view === 'tree'
        ? layoutTreePreview(allNodes, topologyData.links || [])
        : allNodes.filter((node) => isPlaced(node));
    const nodeMap = new Map(nodes.map((node) => [String(node.id), node]));
    const links = (topologyData.links || []).filter((link) =>
        nodeMap.has(String(link.source)) && nodeMap.has(String(link.target))
    );

    topologyNodesLayer.innerHTML = '';
    topologyLinksSvg.innerHTML = '';

    const empty = document.getElementById('homelable-empty');
    if (empty) {
        empty.textContent = nodes.length === 0 ? safeT('topology.drag_hint', 'Drag devices from the left panel') : '';
        empty.style.display = nodes.length === 0 ? 'flex' : 'none';
    }

    links.forEach((link) => renderLink(link, nodeMap));
    nodes.forEach((node) => renderNode(node));
}

function renderNode(node) {
    const card = document.createElement('div');
    const type = node.device_type || 'unknown';
    const colors = topologyPalette[type] || topologyPalette.unknown;
    card.className = `homelable-node ${type} ${node.is_online ? 'online' : 'offline'}`;
    card.dataset.nodeId = node.id;
    card.style.left = `${node.x}px`;
    card.style.top = `${node.y}px`;
    card.style.setProperty('--node-bg', colors.bg);
    card.style.setProperty('--node-fg', colors.fg);
    card.style.setProperty('--node-border', colors.border);
    card.innerHTML = `
        <button class="homelable-handle top" data-anchor="top" title="Connect from top"></button>
        <button class="homelable-handle right" data-anchor="right" title="Connect from right"></button>
        <button class="homelable-handle bottom" data-anchor="bottom" title="Connect from bottom"></button>
        <button class="homelable-handle left" data-anchor="left" title="Connect from left"></button>
        <div class="homelable-node-icon">${deviceIconMarkup(node)}</div>
        <div class="homelable-node-main">
            <div class="homelable-node-title" title="${escapeAttr(deviceName(node))}">${escapeHtml(deviceName(node))}</div>
            <div class="homelable-node-meta">${escapeHtml(node.ip_address || '-')}</div>
        </div>
        <div class="homelable-node-badges">
            <span class="homelable-type">${escapeHtml(fallbackTypeLabels[type] || type)}</span>
            <span class="homelable-status-dot" title="${node.is_online ? 'Online' : 'Offline'}"></span>
        </div>
    `;

    card.addEventListener('pointerdown', (event) => onNodePointerDown(event, node));
    card.addEventListener('dblclick', () => viewDevice(node.id));
    card.addEventListener('click', (event) => onNodeClick(event, node));
    card.addEventListener('contextmenu', (event) => showNodeMenu(event, node));
    card.querySelectorAll('.homelable-handle').forEach((handle) => {
        handle.addEventListener('pointerdown', (event) => event.stopPropagation());
        handle.addEventListener('click', (event) => onHandleClick(event, node, handle.dataset.anchor));
    });
    topologyNodesLayer.appendChild(card);
}

function renderLink(link, nodeMap) {
    const source = nodeMap.get(String(link.source));
    const target = nodeMap.get(String(link.target));
    if (!source || !target) return;

    const anchors = inferLinkAnchors(source, target);
    const sourcePoint = anchorPoint(source, anchors.source);
    const targetPoint = anchorPoint(target, anchors.target);
    const pathData = linkPath(sourcePoint, targetPoint, anchors.source, anchors.target);
    const labelPoint = midpoint(sourcePoint, targetPoint);
    const linkType = normalizedLinkType(link);

    const group = document.createElementNS('http://www.w3.org/2000/svg', 'g');
    group.classList.add('homelable-link-group', linkType);
    group.dataset.linkId = link.id || '';

    const hit = document.createElementNS('http://www.w3.org/2000/svg', 'path');
    hit.setAttribute('d', pathData);
    hit.setAttribute('class', 'homelable-link-hit');

    const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
    path.setAttribute('d', pathData);
    path.setAttribute('class', `homelable-link ${linkType}`);

    const sourceDot = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
    sourceDot.setAttribute('cx', String(sourcePoint.x));
    sourceDot.setAttribute('cy', String(sourcePoint.y));
    sourceDot.setAttribute('r', '4');
    sourceDot.setAttribute('class', `homelable-link-endpoint ${linkType}`);

    const targetDot = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
    targetDot.setAttribute('cx', String(targetPoint.x));
    targetDot.setAttribute('cy', String(targetPoint.y));
    targetDot.setAttribute('r', '4');
    targetDot.setAttribute('class', `homelable-link-endpoint ${linkType}`);

    const label = document.createElementNS('http://www.w3.org/2000/svg', 'text');
    label.setAttribute('x', String(labelPoint.x));
    label.setAttribute('y', String(labelPoint.y - 10));
    label.setAttribute('class', 'homelable-link-label');
    label.textContent = linkLabel(link);

    group.appendChild(hit);
    group.appendChild(path);
    group.appendChild(sourceDot);
    group.appendChild(targetDot);
    if (label.textContent) group.appendChild(label);
    group.addEventListener('click', (event) => showLinkDetails(event, link));
    group.addEventListener('contextmenu', (event) => showLinkMenu(event, link));
    topologyLinksSvg.appendChild(group);
}

function onNodePointerDown(event, node) {
    if (event.button !== 0 || topologyState.linkCreateMode || event.target.closest('.homelable-handle')) return;
    event.stopPropagation();
    topologyState.draggingNode = node;
    topologyState.dragStart = {
        pointerX: event.clientX,
        pointerY: event.clientY,
        nodeX: node.x,
        nodeY: node.y
    };
    event.currentTarget.setPointerCapture(event.pointerId);
    window.addEventListener('pointermove', onCanvasPointerMove);
    window.addEventListener('pointerup', onCanvasPointerUp, { once: true });
}

function onHandleClick(event, node, anchor) {
    event.preventDefault();
    event.stopPropagation();
    if (!topologyState.linkCreateMode) {
        startLinkCreateMode();
    }
    if (!topologyState.selectedSourceNode) {
        topologyState.selectedSourceNode = node;
        topologyState.selectedSourceAnchor = anchor || 'auto';
        highlightSelectedNode(node.id, topologyState.selectedSourceAnchor);
        showToast(safeT('topology.toast.select_target', 'Select target connector'), 'info');
        return;
    }
    if (String(topologyState.selectedSourceNode.id) === String(node.id)) {
        showToast(safeT('topology.toast.cannot_self_connect', 'Cannot link a device to itself'), 'warning');
        return;
    }
    showCreateLinkModal(topologyState.selectedSourceNode, node, topologyState.selectedSourceAnchor, anchor || 'auto');
    exitLinkCreateMode();
}

function onNodeClick(event, node) {
    if (!topologyState.linkCreateMode) return;
    event.stopPropagation();
    if (!topologyState.selectedSourceNode) {
        topologyState.selectedSourceNode = node;
        topologyState.selectedSourceAnchor = 'auto';
        highlightSelectedNode(node.id);
        showToast(safeT('topology.toast.select_target', 'Select target device'), 'info');
        return;
    }
    if (String(topologyState.selectedSourceNode.id) === String(node.id)) {
        showToast(safeT('topology.toast.cannot_self_connect', 'Cannot link a device to itself'), 'warning');
        return;
    }
    showCreateLinkModal(topologyState.selectedSourceNode, node, topologyState.selectedSourceAnchor, 'auto');
    exitLinkCreateMode();
}

function onCanvasPointerDown(event) {
    if (event.button !== 0 || event.target.closest('.homelable-node')) return;
    topologyState.draggingCanvas = true;
    topologyState.lastPointer = { x: event.clientX, y: event.clientY };
    topologyCanvas.setPointerCapture(event.pointerId);
}

function onCanvasPointerMove(event) {
    if (topologyState.draggingNode && topologyState.dragStart) {
        const dx = (event.clientX - topologyState.dragStart.pointerX) / topologyState.scale;
        const dy = (event.clientY - topologyState.dragStart.pointerY) / topologyState.scale;
        topologyState.draggingNode.x = Math.round(topologyState.dragStart.nodeX + dx);
        topologyState.draggingNode.y = Math.round(topologyState.dragStart.nodeY + dy);
        renderTopologyCanvas();
        return;
    }

    if (topologyState.draggingCanvas && topologyState.lastPointer) {
        topologyState.x += event.clientX - topologyState.lastPointer.x;
        topologyState.y += event.clientY - topologyState.lastPointer.y;
        topologyState.lastPointer = { x: event.clientX, y: event.clientY };
        applyViewportTransform();
    }
}

function onCanvasPointerUp() {
    if (topologyState.draggingNode) {
        saveTopologyPositions(true);
    }
    topologyState.draggingNode = null;
    topologyState.dragStart = null;
    topologyState.draggingCanvas = false;
    topologyState.lastPointer = null;
    window.removeEventListener('pointermove', onCanvasPointerMove);
}

function onCanvasWheel(event) {
    event.preventDefault();
    const delta = event.deltaY > 0 ? -0.08 : 0.08;
    const nextScale = clamp(topologyState.scale + delta, 0.35, 2.3);
    topologyState.scale = nextScale;
    applyViewportTransform();
}

function applyViewportTransform() {
    if (!topologyViewport) return;
    topologyViewport.style.transform = `translate(${topologyState.x}px, ${topologyState.y}px) scale(${topologyState.scale})`;
}

async function handleDrop(event) {
    event.preventDefault();
    const deviceId = event.dataTransfer.getData('device-id');
    const node = topologyData.nodes.find((item) => String(item.id) === String(deviceId));
    if (!node || !topologyCanvas) return;
    const point = screenToCanvasPoint(event.clientX, event.clientY);
    node.x = Math.round(point.x - 100);
    node.y = Math.round(point.y - 34);
    await saveTopologyPositions(true);
    renderTopology();
    showToast(safeT('topology.toast.device_placed', 'Device placed'), 'success');
}

function screenToCanvasPoint(clientX, clientY) {
    const rect = topologyCanvas.getBoundingClientRect();
    return {
        x: (clientX - rect.left - topologyState.x) / topologyState.scale,
        y: (clientY - rect.top - topologyState.y) / topologyState.scale
    };
}

async function saveTopologyPositions(silent = false) {
    const positions = (topologyData.nodes || [])
        .filter((node) => isPlaced(node))
        .map((node) => ({
            device_id: Number(node.id),
            x: Math.round(Number(node.x) || 0),
            y: Math.round(Number(node.y) || 0)
        }));

    if (positions.length === 0) {
        if (!silent) showToast(safeT('topology.toast.no_devices_to_save', 'No devices to save'), 'warning');
        return;
    }

    try {
        const response = await apiPut('/topology/positions', { positions });
        if (!response || !response.success) throw new Error('Save failed');
        if (!silent) showToast(safeT('topology.toast.positions_saved', 'Positions saved'), 'success');
    } catch (error) {
        console.error('[Topology] save positions failed:', error);
        if (!silent) showToast(safeT('topology.toast.save_positions_failed', 'Save positions failed'), 'error');
    }
}

function resetTopologyView() {
    topologyState.scale = 1;
    topologyState.x = 0;
    topologyState.y = 0;
    applyViewportTransform();
}

async function discoverTopology() {
    try {
        const response = await apiPost('/topology/discover', {});
        if (!response || !response.success) throw new Error('Discover failed');
        showToast(safeT('topology.toast.discovery_started', 'Topology discovery started'), 'info');
        setTimeout(loadTopology, 3000);
    } catch (error) {
        console.error('[Topology] discovery failed:', error);
        showToast(safeT('topology.toast.discovery_failed', 'Topology discovery failed'), 'error');
    }
}

function startLinkCreateMode() {
    topologyState.linkCreateMode = true;
    topologyState.selectedSourceNode = null;
    topologyState.selectedSourceAnchor = null;
    document.querySelector('.topology-container')?.classList.add('link-create-mode');
    showToast(safeT('topology.toast.manual_mode_hint', 'Select source and target devices'), 'info');
}

function exitLinkCreateMode() {
    topologyState.linkCreateMode = false;
    topologyState.selectedSourceNode = null;
    topologyState.selectedSourceAnchor = null;
    document.querySelector('.topology-container')?.classList.remove('link-create-mode');
    document.querySelectorAll('.homelable-node.selected').forEach((el) => el.classList.remove('selected'));
    document.querySelectorAll('.homelable-handle.selected').forEach((el) => el.classList.remove('selected'));
}

async function showCreateLinkModal(source, target, sourceAnchor = 'auto', targetAnchor = 'auto') {
    const content = `
        <form id="create-link-form" onsubmit="createManualLink(event, ${Number(source.id)}, ${Number(target.id)})">
            <div class="homelable-link-preview">
                <div>${deviceIconMarkup(source)}<strong>${escapeHtml(deviceName(source))}</strong><small>${escapeHtml(anchorLabel(sourceAnchor))}</small></div>
                <span>to</span>
                <div>${deviceIconMarkup(target)}<strong>${escapeHtml(deviceName(target))}</strong><small>${escapeHtml(anchorLabel(targetAnchor))}</small></div>
            </div>
            <div class="form-group">
                <label>${safeT('topology.form.link_type', 'Link type')}</label>
                <select name="link_type">
                    <option value="manual">${safeT('topology.manual_link', 'Manual')}</option>
                    <option value="lldp">LLDP</option>
                </select>
            </div>
            <div class="form-group">
                <label>${safeT('topology.form.label', 'Link label')}</label>
                <input type="text" name="link_label" placeholder="uplink / trunk / manual" />
            </div>
            <div class="form-group">
                <label>${safeT('topology.form.speed', 'Link speed')}</label>
                <input type="number" name="link_speed" placeholder="1000000000" />
            </div>
            <div class="form-actions">
                <button type="button" class="btn btn-secondary" onclick="closeModal()">${safeT('common.cancel', 'Cancel')}</button>
                <button type="submit" class="btn btn-primary">${safeT('topology.manual_connect', 'Connect')}</button>
            </div>
        </form>
    `;
    openModal(safeT('topology.modal.create_link_title', 'Create topology link'), content);
}

async function createManualLink(event, sourceId, targetId) {
    event.preventDefault();
    const form = event.target;
    const formData = new FormData(form);
    const payload = {
        source_device_id: sourceId,
        target_device_id: targetId,
        link_type: String(formData.get('link_type') || 'manual'),
        link_label: String(formData.get('link_label') || ''),
        link_speed: Number(formData.get('link_speed') || 0)
    };

    try {
        const response = await apiPost('/topology/links', payload);
        if (!response || !response.success) throw new Error('Create link failed');
        closeModal();
        showToast(safeT('topology.toast.link_created', 'Link created'), 'success');
        await loadTopology();
    } catch (error) {
        console.error('[Topology] create link failed:', error);
        showToast(safeT('topology.toast.create_link_failed', 'Create link failed'), 'error');
    }
}

function showLinkDetails(event, link) {
    event.stopPropagation();
    const source = findNode(link.source);
    const target = findNode(link.target);
    const content = `
        <div class="device-details">
            <div class="device-details-row"><label>Source</label><span>${escapeHtml(deviceName(source))}</span></div>
            <div class="device-details-row"><label>Target</label><span>${escapeHtml(deviceName(target))}</span></div>
            <div class="device-details-row"><label>Type</label><span>${escapeHtml(normalizedLinkType(link).toUpperCase())}</span></div>
            <div class="device-details-row"><label>Speed</label><span>${escapeHtml(linkSpeedLabel(link) || '-')}</span></div>
            ${link.link_label ? `<div class="device-details-row"><label>Label</label><span>${escapeHtml(link.link_label)}</span></div>` : ''}
            ${link.id ? `<button class="btn btn-danger" onclick="deleteTopologyLink(${Number(link.id)})">Delete link</button>` : ''}
        </div>
    `;
    openModal(safeT('topology.modal.link_detail_title', 'Link details'), content);
}

function showNodeMenu(event, node) {
    event.preventDefault();
    viewDevice(node.id);
}

function showLinkMenu(event, link) {
    event.preventDefault();
    showLinkDetails(event, link);
}

async function deleteTopologyLink(linkId) {
    try {
        const response = await apiDelete(`/topology/links/${linkId}`);
        if (!response || !response.success) throw new Error('Delete link failed');
        closeModal();
        showToast(safeT('topology.toast.link_deleted', 'Link deleted'), 'success');
        await loadTopology();
    } catch (error) {
        console.error('[Topology] delete link failed:', error);
        showToast(safeT('topology.toast.delete_link_failed', 'Delete link failed'), 'error');
    }
}

function switchTopologyView(view, saveAsDefault) {
    topologyState.view = view === 'tree' ? 'tree' : 'force';
    if (saveAsDefault) {
        try { localStorage.setItem('topology_default_view', topologyState.view); } catch (e) {}
    }
    renderTopologyCanvas();
    updateTopologyViewButtons();
}

function updateTopologyViewButtons() {
    document.querySelectorAll('.topo-view-btn').forEach((button) => button.classList.remove('active'));
    document.getElementById(`topo-btn-${topologyState.view}`)?.classList.add('active');
}

function toggleTopologyLegend() {
    const legend = document.getElementById('topology-legend');
    if (!legend) return;
    legend.classList.toggle('collapsed');
    try { localStorage.setItem('topology_legend_collapsed', String(legend.classList.contains('collapsed'))); } catch (e) {}
}

function toggleTopologySidebar() {
    const sidebar = document.getElementById('topology-sidebar');
    if (!sidebar) return;
    sidebar.classList.toggle('collapsed');
}

function layoutTreePreview(nodes, links) {
    const visible = nodes.map((node) => ({ ...node }));
    const nodeById = new Map(visible.map((node) => [String(node.id), node]));
    const adjacency = new Map(visible.map((node) => [String(node.id), []]));

    (links || []).forEach((link) => {
        const sourceId = String(link.source);
        const targetId = String(link.target);
        if (!adjacency.has(sourceId) || !adjacency.has(targetId)) return;
        adjacency.get(sourceId).push(targetId);
        adjacency.get(targetId).push(sourceId);
    });

    const visited = new Set();
    const components = [];

    visible.forEach((node) => {
        const nodeId = String(node.id);
        if (visited.has(nodeId)) return;

        const componentIds = [];
        const queue = [nodeId];
        visited.add(nodeId);

        while (queue.length > 0) {
            const current = queue.shift();
            componentIds.push(current);
            (adjacency.get(current) || []).forEach((next) => {
                if (visited.has(next)) return;
                visited.add(next);
                queue.push(next);
            });
        }

        components.push(componentIds);
    });

    let componentOffsetY = 80;
    const levelGapX = 300;
    const rowGapY = 128;

    components.forEach((componentIds) => {
        const rootId = chooseTreeRoot(componentIds, nodeById, adjacency);
        const levels = buildTreeLevels(rootId, componentIds, adjacency);
        let componentHeight = 0;

        levels.forEach((ids, level) => {
            ids.sort((a, b) => deviceName(nodeById.get(a)).localeCompare(deviceName(nodeById.get(b))));
            const levelHeight = Math.max(0, (ids.length - 1) * rowGapY);
            componentHeight = Math.max(componentHeight, levelHeight + nodeBox.height);

            ids.forEach((id, index) => {
                const node = nodeById.get(id);
                if (!node) return;
                node.x = 120 + level * levelGapX;
                node.y = componentOffsetY + index * rowGapY;
            });
        });

        componentOffsetY += Math.max(componentHeight + 100, 220);
    });

    return visible;
}

function chooseTreeRoot(componentIds, nodeById, adjacency) {
    const preferredTypes = new Set(['router', 'firewall', 'switch']);
    const sorted = [...componentIds].sort((a, b) => {
        const nodeA = nodeById.get(a) || {};
        const nodeB = nodeById.get(b) || {};
        const preferredA = preferredTypes.has(String(nodeA.device_type || '').toLowerCase()) ? 0 : 1;
        const preferredB = preferredTypes.has(String(nodeB.device_type || '').toLowerCase()) ? 0 : 1;
        if (preferredA !== preferredB) return preferredA - preferredB;
        const degreeB = (adjacency.get(b) || []).length;
        const degreeA = (adjacency.get(a) || []).length;
        if (degreeB !== degreeA) return degreeB - degreeA;
        return deviceName(nodeA).localeCompare(deviceName(nodeB));
    });
    return sorted[0];
}

function buildTreeLevels(rootId, componentIds, adjacency) {
    const componentSet = new Set(componentIds);
    const seen = new Set([rootId]);
    const levels = [];
    const queue = [{ id: rootId, level: 0 }];

    while (queue.length > 0) {
        const current = queue.shift();
        if (!levels[current.level]) levels[current.level] = [];
        levels[current.level].push(current.id);

        (adjacency.get(current.id) || []).forEach((next) => {
            if (!componentSet.has(next) || seen.has(next)) return;
            seen.add(next);
            queue.push({ id: next, level: current.level + 1 });
        });
    }

    componentIds.forEach((id) => {
        if (seen.has(id)) return;
        if (!levels[0]) levels[0] = [];
        levels[0].push(id);
    });

    return levels;
}

function autoLayoutNodes(nodes, links, width, height) {
    const centerX = width / 2;
    const centerY = height / 2;
    const radius = Math.max(180, Math.min(width, height) * 0.34);
    nodes.forEach((node, index) => {
        const angle = (index / Math.max(nodes.length, 1)) * Math.PI * 2 - Math.PI / 2;
        node.x = Math.round(centerX + Math.cos(angle) * radius - 100);
        node.y = Math.round(centerY + Math.sin(angle) * radius - 34);
    });
}

function highlightSelectedNode(nodeId, anchor) {
    document.querySelectorAll('.homelable-node.selected').forEach((el) => el.classList.remove('selected'));
    document.querySelectorAll('.homelable-handle.selected').forEach((el) => el.classList.remove('selected'));
    const nodeEl = document.querySelector(`.homelable-node[data-node-id="${CSS.escape(String(nodeId))}"]`);
    nodeEl?.classList.add('selected');
    if (anchor && anchor !== 'auto') {
        nodeEl?.querySelector(`.homelable-handle.${anchor}`)?.classList.add('selected');
    }
}

function isPlaced(node) {
    const x = Number(node.x);
    const y = Number(node.y);
    return Number.isFinite(x) && Number.isFinite(y) && !(x === 0 && y === 0) && !(x === -1 && y === -1);
}

function normalizeId(id) {
    return typeof id === 'number' ? id : String(id || '');
}

function findNode(id) {
    return (topologyData.nodes || []).find((node) => String(node.id) === String(id)) || null;
}

function linkLabel(link) {
    const type = normalizedLinkType(link);
    const speed = linkSpeedLabel(link);
    const customLabel = link.link_label ? String(link.link_label) : '';

    if (type === 'lldp') return speed ? `LLDP · ${speed}` : (customLabel ? `LLDP · ${customLabel}` : 'LLDP');
    if (type === 'fdb') return speed ? `FDB · ${speed}` : (customLabel ? `FDB · ${customLabel}` : 'FDB');
    if (type === 'manual') return speed ? `Manual · ${speed}` : (customLabel || 'Manual');
    return speed || customLabel || type.toUpperCase();
}

function linkSpeedLabel(link) {
    const speed = Number(link.link_speed || 0);
    if (speed > 0) return formatLinkSpeed(speed);
    const bandwidth = Number(link.bandwidth_usage || 0);
    if (bandwidth > 0) return formatLinkSpeed(bandwidth);
    return '';
}

function normalizedLinkType(link) {
    const type = String(link.link_type || '').toLowerCase();
    if (type === 'lldp') return 'lldp';
    if (type === 'fdb') return 'fdb';
    if (link.is_manual) return 'manual';
    return type || 'auto';
}

function inferLinkAnchors(source, target) {
    const sx = Number(source.x) + nodeBox.width / 2;
    const sy = Number(source.y) + nodeBox.height / 2;
    const tx = Number(target.x) + nodeBox.width / 2;
    const ty = Number(target.y) + nodeBox.height / 2;
    const dx = tx - sx;
    const dy = ty - sy;

    if (Math.abs(dx) >= Math.abs(dy)) {
        return {
            source: dx >= 0 ? 'right' : 'left',
            target: dx >= 0 ? 'left' : 'right'
        };
    }
    return {
        source: dy >= 0 ? 'bottom' : 'top',
        target: dy >= 0 ? 'top' : 'bottom'
    };
}

function anchorPoint(node, anchor) {
    const x = Number(node.x) || 0;
    const y = Number(node.y) || 0;
    switch (anchor) {
        case 'top':
            return { x: x + nodeBox.width / 2, y };
        case 'right':
            return { x: x + nodeBox.width, y: y + nodeBox.height / 2 };
        case 'bottom':
            return { x: x + nodeBox.width / 2, y: y + nodeBox.height };
        case 'left':
        default:
            return { x, y: y + nodeBox.height / 2 };
    }
}

function linkPath(sourcePoint, targetPoint, sourceAnchor, targetAnchor) {
    const sourceVector = anchorVector(sourceAnchor);
    const targetVector = anchorVector(targetAnchor);
    const distance = Math.max(70, Math.min(220, Math.hypot(targetPoint.x - sourcePoint.x, targetPoint.y - sourcePoint.y) * 0.45));
    const c1 = {
        x: sourcePoint.x + sourceVector.x * distance,
        y: sourcePoint.y + sourceVector.y * distance
    };
    const c2 = {
        x: targetPoint.x + targetVector.x * distance,
        y: targetPoint.y + targetVector.y * distance
    };
    return `M ${sourcePoint.x} ${sourcePoint.y} C ${c1.x} ${c1.y} ${c2.x} ${c2.y} ${targetPoint.x} ${targetPoint.y}`;
}

function anchorVector(anchor) {
    switch (anchor) {
        case 'top':
            return { x: 0, y: -1 };
        case 'right':
            return { x: 1, y: 0 };
        case 'bottom':
            return { x: 0, y: 1 };
        case 'left':
        default:
            return { x: -1, y: 0 };
    }
}

function midpoint(a, b) {
    return { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 };
}

function anchorLabel(anchor) {
    const labels = { top: 'Top', right: 'Right', bottom: 'Bottom', left: 'Left', auto: 'Auto side' };
    return labels[anchor] || labels.auto;
}

function formatLinkSpeed(bps) {
    if (!bps) return '';
    if (bps >= 1000000000000) return `${Math.round(bps / 1000000000000)}T`;
    if (bps >= 1000000000) return `${Math.round(bps / 1000000000)}G`;
    if (bps >= 1000000) return `${Math.round(bps / 1000000)}M`;
    if (bps >= 1000) return `${Math.round(bps / 1000)}K`;
    return String(bps);
}

function deviceName(node) {
    if (!node) return '-';
    if (typeof getDeviceDisplayName === 'function') return getDeviceDisplayName(node);
    return node.name || node.hostname || node.ip_address || `Device ${node.id}`;
}

function deviceIconMarkup(node) {
    if (typeof getDeviceIcon === 'function') return getDeviceIcon(node);
    return `<span>${escapeHtml((node.device_type || '?').slice(0, 2).toUpperCase())}</span>`;
}

function getCanvasSize() {
    const root = document.getElementById('topology-svg');
    return {
        width: root ? root.clientWidth : 1000,
        height: root ? root.clientHeight : 650
    };
}

function safeT(key, fallback) {
    try {
        const value = typeof t === 'function' ? t(key) : '';
        return value && value !== key ? value : fallback;
    } catch (e) {
        return fallback;
    }
}

function clamp(value, min, max) {
    return Math.max(min, Math.min(max, value));
}

function escapeHtml(value) {
    return String(value ?? '')
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
}

function escapeAttr(value) {
    return escapeHtml(value).replace(/`/g, '&#096;');
}

window.loadTopology = loadTopology;
window.saveTopologyPositions = saveTopologyPositions;
window.resetTopologyView = resetTopologyView;
window.discoverTopology = discoverTopology;
window.startLinkCreateMode = startLinkCreateMode;
window.exitLinkCreateMode = exitLinkCreateMode;
window.createManualLink = createManualLink;
window.deleteTopologyLink = deleteTopologyLink;
window.switchTopologyView = switchTopologyView;
window.toggleTopologyLegend = toggleTopologyLegend;
window.toggleTopologySidebar = toggleTopologySidebar;
