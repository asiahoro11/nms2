// Made by YTSworks
// YTS工作室製作
(function (global) {
    const iconPaths = {
        brand: [
            '<path d="M12 2l7 3.5v6.2c0 4.5-2.8 7.8-7 9.8-4.2-2-7-5.3-7-9.8V5.5L12 2z"/>',
            '<path d="M9 12l2 2 4-5"/>'
        ],
        dashboard: [
            '<rect x="3" y="3" width="7" height="8" rx="1.5"/>',
            '<rect x="14" y="3" width="7" height="5" rx="1.5"/>',
            '<rect x="14" y="12" width="7" height="9" rx="1.5"/>',
            '<rect x="3" y="15" width="7" height="6" rx="1.5"/>'
        ],
        devices: [
            '<rect x="4" y="4" width="16" height="6" rx="2"/>',
            '<rect x="4" y="14" width="16" height="6" rx="2"/>',
            '<path d="M8 7h.01M8 17h.01M12 7h4M12 17h4"/>'
        ],
        topology: [
            '<circle cx="6" cy="7" r="3"/>',
            '<circle cx="18" cy="7" r="3"/>',
            '<circle cx="12" cy="18" r="3"/>',
            '<path d="M8.5 9.2l2 5.2M15.5 9.2l-2 5.2M9 7h6"/>'
        ],
        cameras: [
            '<rect x="3" y="6" width="13" height="12" rx="2"/>',
            '<path d="M16 10l5-3v10l-5-3zM7 10h5"/>'
        ],
        access: [
            '<path d="M7 3h9a2 2 0 0 1 2 2v16H7z"/>',
            '<path d="M10 12h.01M5 21h16"/>'
        ],
        pdu: [
            '<path d="M9 7V3M15 7V3M7 7h10v5a5 5 0 0 1-10 0z"/>',
            '<path d="M12 17v4M8 21h8"/>'
        ],
        logs: [
            '<path d="M8 4h8M8 8h8M8 12h5"/>',
            '<rect x="5" y="3" width="14" height="18" rx="2"/>',
            '<path d="M9 17l2 2 4-5"/>'
        ],
        admin: [
            '<circle cx="12" cy="12" r="3"/>',
            '<path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1-2 2-.1-.1a1.7 1.7 0 0 0-1.9-.3 1.7 1.7 0 0 0-1 1.6V20h-3v-.1a1.7 1.7 0 0 0-1-1.6 1.7 1.7 0 0 0-1.9.3l-.1.1-2-2 .1-.1a1.7 1.7 0 0 0 .3-1.9 1.7 1.7 0 0 0-1.6-1H4v-3h.1a1.7 1.7 0 0 0 1.6-1 1.7 1.7 0 0 0-.3-1.9l-.1-.1 2-2 .1.1a1.7 1.7 0 0 0 1.9.3 1.7 1.7 0 0 0 1-1.6V4h3v.1a1.7 1.7 0 0 0 1 1.6 1.7 1.7 0 0 0 1.9-.3l.1-.1 2 2-.1.1a1.7 1.7 0 0 0-.3 1.9 1.7 1.7 0 0 0 1.6 1h.1v3h-.1a1.7 1.7 0 0 0-1.6 1z"/>'
        ],
        bell: [
            '<path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9"/>',
            '<path d="M10 21h4"/>'
        ],
        maximize: [
            '<path d="M8 3H3v5M16 3h5v5M8 21H3v-5M21 16v5h-5"/>',
            '<path d="M3 3l6 6M21 3l-6 6M3 21l6-6M21 21l-6-6"/>'
        ],
        minimize: [
            '<path d="M9 3v6H3M15 3v6h6M9 21v-6H3M15 21v-6h6"/>'
        ],
        key: [
            '<circle cx="7.5" cy="14.5" r="3.5"/>',
            '<path d="M10 12l9-9M14 6l4 4M16 4l4 4"/>'
        ],
        logout: [
            '<path d="M10 17l5-5-5-5M15 12H3"/>',
            '<path d="M21 3v18h-8"/>'
        ],
        menu: [
            '<path d="M4 6h16M4 12h16M4 18h16"/>'
        ],
        collapse: [
            '<path d="M15 18l-6-6 6-6"/>'
        ],
        expand: [
            '<path d="M9 18l6-6-6-6"/>'
        ],
        total: [
            '<path d="M4 6h16M4 12h16M4 18h16"/>',
            '<path d="M7 4v4M7 10v4M7 16v4"/>'
        ],
        online: [
            '<circle cx="12" cy="12" r="9"/>',
            '<path d="M8 12l2.5 2.5L16 9"/>'
        ],
        offline: [
            '<circle cx="12" cy="12" r="9"/>',
            '<path d="M8 8l8 8M16 8l-8 8"/>'
        ],
        memory: [
            '<rect x="6" y="5" width="12" height="14" rx="2"/>',
            '<path d="M9 9h6M9 13h6M8 2v3M12 2v3M16 2v3M8 19v3M12 19v3M16 19v3"/>'
        ],
        cpu: [
            '<rect x="6" y="6" width="12" height="12" rx="2"/>',
            '<path d="M9 9h6v6H9zM9 2v4M12 2v4M15 2v4M9 18v4M12 18v4M15 18v4M2 9h4M2 12h4M2 15h4M18 9h4M18 12h4M18 15h4"/>'
        ],
        disk: [
            '<path d="M6 4h12l2 5v9a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V9z"/>',
            '<path d="M4 9h16M8 16h.01M12 16h4"/>'
        ],
        network: [
            '<rect x="3" y="4" width="7" height="5" rx="1.5"/>',
            '<rect x="14" y="4" width="7" height="5" rx="1.5"/>',
            '<rect x="8.5" y="15" width="7" height="5" rx="1.5"/>',
            '<path d="M6.5 9v2.5H12M17.5 9v2.5H12M12 11.5V15"/>'
        ],
        link: [
            '<path d="M10 13a5 5 0 0 0 7.1 0l2-2a5 5 0 0 0-7.1-7.1l-1.1 1.1"/>',
            '<path d="M14 11a5 5 0 0 0-7.1 0l-2 2a5 5 0 0 0 7.1 7.1l1.1-1.1"/>'
        ],
        save: [
            '<path d="M5 3h12l2 2v16H5z"/>',
            '<path d="M8 3v6h8V3M8 21v-7h8v7"/>'
        ],
        sync: [
            '<path d="M21 12a9 9 0 0 1-15.4 6.4L3 16"/>',
            '<path d="M3 16v5h5"/>',
            '<path d="M3 12A9 9 0 0 1 18.4 5.6L21 8"/>',
            '<path d="M16 8h5V3"/>'
        ],
        info: [
            '<circle cx="12" cy="12" r="9"/>',
            '<path d="M12 10v6M12 7h.01"/>'
        ],
        alert: [
            '<path d="M12 3l10 18H2L12 3z"/>',
            '<path d="M12 9v5M12 17h.01"/>'
        ],
        lock: [
            '<rect x="5" y="10" width="14" height="10" rx="2"/>',
            '<path d="M8 10V7a4 4 0 0 1 8 0v3"/>'
        ]
    };

    const pageIcons = {
        dashboard: 'dashboard',
        devices: 'devices',
        topology: 'topology',
        cameras: 'cameras',
        'access-control': 'access',
        pdu: 'pdu',
        logs: 'logs',
        admin: 'admin'
    };

    const classIcons = {
        'fa-link': 'link',
        'fa-save': 'save',
        'fa-sync': 'sync',
        'fa-info-circle': 'info',
        'icon-cpu': 'cpu',
        'icon-memory': 'memory',
        'icon-disk': 'disk',
        'icon-network': 'network'
    };

    const iconSelector = [
        '.nav-icon',
        '.bottom-nav-icon',
        '.stat-icon',
        '.license-notice-icon',
        '#default-logo-icon',
        '.mobile-menu-btn',
        '#notif-bell-btn',
        '#fullscreen-btn',
        '#logout-btn',
        'button[onclick*="showChangePasswordModal"]',
        '.sidebar-collapse-btn .collapse-icon',
        '[data-icon]',
        'i.fas',
        'i.fa',
        'i[class^="icon-"]',
        'i[class*=" icon-"]'
    ].join(', ');

    function createIconSvg(name) {
        const iconName = iconPaths[name] ? name : 'dashboard';
        const paths = iconPaths[iconName];
        return '<svg class="svg-icon svg-icon-' + iconName + '" viewBox="0 0 24 24" aria-hidden="true" focusable="false" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">' + paths.join('') + '</svg>';
    }

    function setIcon(element, name) {
        if (!element || !name) return;
        element.innerHTML = createIconSvg(name);
        element.dataset.icon = name;
        element.dataset.iconReady = 'true';
        element.classList.add('nms-icon-ready');
    }

    function inferIconName(element, statIndex) {
        if (element.dataset.icon && iconPaths[element.dataset.icon]) return element.dataset.icon;

        for (const className in classIcons) {
            if (element.classList.contains(className)) {
                return classIcons[className];
            }
        }

        const pageNode = element.closest('[data-page]');
        if (pageNode && pageIcons[pageNode.dataset.page]) return pageIcons[pageNode.dataset.page];

        if (element.id === 'default-logo-icon') return 'brand';
        if (element.id === 'notif-bell-btn') return 'bell';
        if (element.id === 'fullscreen-btn') return document.fullscreenElement ? 'minimize' : 'maximize';
        if (element.id === 'logout-btn') return 'logout';
        if (element.classList.contains('mobile-menu-btn')) return 'menu';
        if (element.classList.contains('collapse-icon')) return 'collapse';
        if (element.classList.contains('license-notice-icon')) return 'alert';
        if (element.matches('button[onclick*="showChangePasswordModal"]')) return 'key';
        if (element.classList.contains('stat-icon')) return ['total', 'online', 'offline', 'memory'][statIndex] || 'dashboard';

        return null;
    }

    function collectIconElements(root) {
        const elements = [];
        if (root.matches && root.matches(iconSelector)) {
            elements.push(root);
        }
        if (root.querySelectorAll) {
            root.querySelectorAll(iconSelector).forEach((element) => elements.push(element));
        }
        return elements;
    }

    function applyIcons(root = document) {
        let statIndex = 0;
        collectIconElements(root).forEach((element) => {
            if (element.dataset.iconReady === 'true' && element.querySelector('svg.svg-icon')) return;
            const index = element.classList.contains('stat-icon') ? statIndex++ : undefined;
            const iconName = inferIconName(element, index);
            if (iconName) setIcon(element, iconName);
        });
    }

    function observeDynamicIcons() {
        if (!document.body || typeof MutationObserver === 'undefined') return;
        const observer = new MutationObserver((mutations) => {
            mutations.forEach((mutation) => {
                mutation.addedNodes.forEach((node) => {
                    if (node.nodeType === Node.ELEMENT_NODE) {
                        applyIcons(node);
                    }
                });
            });
        });
        observer.observe(document.body, { childList: true, subtree: true });
    }

    global.NMSIcons = {
        apply: applyIcons,
        set: setIcon,
        create: createIconSvg
    };

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => {
            applyIcons();
            observeDynamicIcons();
        }, { once: true });
    } else {
        applyIcons();
        observeDynamicIcons();
    }

    document.addEventListener('fullscreenchange', () => {
        const btn = document.getElementById('fullscreen-btn');
        if (btn) setIcon(btn, document.fullscreenElement ? 'minimize' : 'maximize');
    });

    global.addEventListener('languageChanged', () => applyIcons());
})(window);
