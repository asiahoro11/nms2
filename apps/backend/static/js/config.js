(function (global) {
    const config = {
        appVersion: 'v1.2.4-PoC',
        assetVersion: 'v1.2.4',
        productName: 'Management System'
    };

    function applyVersionTargets(root) {
        if (!root || typeof root.querySelectorAll !== 'function') {
            return;
        }

        root.querySelectorAll('[data-app-version]').forEach((node) => {
            const template = node.getAttribute('data-app-version-template');
            node.textContent = template
                ? template.replace('{version}', config.appVersion)
                : config.appVersion;
        });
    }

    function setAppVersion(nextVersion) {
        if (typeof nextVersion !== 'string') {
            return;
        }

        const normalized = nextVersion.trim();
        if (!normalized || normalized === config.appVersion) {
            return;
        }

        config.appVersion = normalized;
        if (typeof document !== 'undefined') {
            applyVersionTargets(document);
        }
    }

    function getVersionedAssetUrl(path) {
        if (!path) {
            return '';
        }

        if (/^(https?:)?\/\//i.test(path)) {
            return path;
        }

        const version = `v=${encodeURIComponent(config.assetVersion)}`;
        if (/[?&]v=/.test(path)) {
            return path.replace(/([?&])v=[^&#]*/, `$1${version}`);
        }

        return `${path}${path.includes('?') ? '&' : '?'}${version}`;
    }

    function escapeAttribute(value) {
        return String(value)
            .replace(/&/g, '&amp;')
            .replace(/"/g, '&quot;');
    }

    function renderStyleTags(paths) {
        return paths
            .map((path) => `<link rel="stylesheet" href="${escapeAttribute(getVersionedAssetUrl(path))}">`)
            .join('\n');
    }

    function renderScriptTags(paths) {
        return paths
            .map((path) => `<script src="${escapeAttribute(getVersionedAssetUrl(path))}"><\/script>`)
            .join('\n');
    }

    global.NMS_CONFIG = config;
    global.getAppVersion = function getAppVersion() {
        return config.appVersion;
    };
    global.getAssetVersion = function getAssetVersion() {
        return config.assetVersion;
    };
    global.getConfiguredAssetUrl = function getConfiguredAssetUrl(path) {
        return getVersionedAssetUrl(path);
    };
    global.getConfiguredServiceWorkerUrl = function getConfiguredServiceWorkerUrl(path = '/sw.js') {
        return getVersionedAssetUrl(path);
    };
    global.writeConfiguredStyles = function writeConfiguredStyles(paths) {
        document.write(renderStyleTags(paths));
    };
    global.writeConfiguredScripts = function writeConfiguredScripts(paths) {
        document.write(renderScriptTags(paths));
    };
    global.applyConfiguredVersion = function applyConfiguredVersion(root = document) {
        applyVersionTargets(root);
    };
    global.refreshConfiguredVersion = async function refreshConfiguredVersion() {
        try {
            const response = await fetch(`/api/v1/system/info?_ts=${Date.now()}`, {
                headers: {
                    'Pragma': 'no-cache',
                    'Cache-Control': 'no-cache'
                }
            });

            if (!response.ok) {
                return;
            }

            const payload = await response.json();
            const runtimeVersion = payload && payload.data ? payload.data.version : '';
            setAppVersion(runtimeVersion);
        } catch (error) {
            console.debug('Runtime version sync skipped:', error);
        }
    };

    if (typeof document !== 'undefined') {
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', () => applyVersionTargets(document), { once: true });
        } else {
            applyVersionTargets(document);
        }
        global.refreshConfiguredVersion();
    }
})(window);
