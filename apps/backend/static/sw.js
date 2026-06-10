const SW_VERSION = new URL(self.location.href).searchParams.get('v') || 'v1.2.4.8';
const CACHE_NAME = `sync-${SW_VERSION}`;
const STATIC_ASSETS = [
    '/',
    '/index.html',
    '/monitor.html',
    '/embed.html',
    '/static/monitor.html',
    `/static/css/main.css?v=${SW_VERSION}`,
    `/static/css/dashboard.css?v=${SW_VERSION}`,
    `/static/css/topology.css?v=${SW_VERSION}`,
    `/static/css/components.css?v=${SW_VERSION}`,
    `/static/css/license.css?v=${SW_VERSION}`,
    `/static/css/user.css?v=${SW_VERSION}`,
    `/static/js/config.js?v=${SW_VERSION}`,
    `/static/js/app.js?v=${SW_VERSION}`,
    `/static/js/api.js?v=${SW_VERSION}`,
    `/static/js/utils.js?v=${SW_VERSION}`,
    `/static/js/i18n.js?v=${SW_VERSION}`,
    `/static/js/auth.js?v=${SW_VERSION}`,
    `/static/js/dashboard.js?v=${SW_VERSION}`,
    `/static/js/devices.js?v=${SW_VERSION}`,
    `/static/js/topology.js?v=${SW_VERSION}`,
    `/static/js/logs.js?v=${SW_VERSION}`,
    `/static/js/admin.js?v=${SW_VERSION}`,
    `/static/js/reports.js?v=${SW_VERSION}`,
    `/static/js/iot.js?v=${SW_VERSION}`,
    `/static/js/embed.js?v=${SW_VERSION}`,
    `/static/js/camera-webrtc.js?v=${SW_VERSION}`,
    `/static/i18n/en-US.json?v=${SW_VERSION}`,
    `/static/i18n/zh-TW.json?v=${SW_VERSION}`,
    `/static/i18n/zh-CN.json?v=${SW_VERSION}`,
    `/static/i18n/ja-JP.json?v=${SW_VERSION}`,
    `/static/i18n/ko-KR.json?v=${SW_VERSION}`
];

// Install and cache static assets.
self.addEventListener('install', (event) => {
    event.waitUntil(
        caches.open(CACHE_NAME).then((cache) => cache.addAll(STATIC_ASSETS).catch((err) => {
            console.warn('[SW] Some assets failed to cache:', err);
        }))
    );
    self.skipWaiting();
});

// Activate and clean old caches.
self.addEventListener('activate', (event) => {
    event.waitUntil(
        caches.keys().then((keys) => Promise.all(
            keys.filter((key) => key !== CACHE_NAME)
                .map((key) => caches.delete(key))
        ))
    );
    self.clients.claim();
});

// Network-first with cache fallback for static assets.
self.addEventListener('fetch', (event) => {
    const url = new URL(event.request.url);

    if (event.request.method !== 'GET') return;
    if (url.pathname.startsWith('/api/')) return;
    if (url.pathname === '/sw.js') return;

    event.respondWith(
        fetch(event.request)
            .then((response) => {
                if (response.ok) {
                    const clone = response.clone();
                    caches.open(CACHE_NAME).then((cache) => {
                        cache.put(event.request, clone);
                    });
                }
                return response;
            })
            .catch(async () => {
                const cached = await caches.match(event.request);
                if (cached) return cached;

                if (event.request.mode === 'navigate') {
                    const monitorFallback = await caches.match('/static/monitor.html');
                    if (monitorFallback) return monitorFallback;

                    const appFallback = await caches.match('/index.html');
                    if (appFallback) return appFallback;
                }

                return new Response('', {
                    status: 504,
                    statusText: 'Network unavailable'
                });
            })
    );
});
