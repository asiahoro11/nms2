const API_BASE = '/api/v1';

function apiNormalizeEndpoint(endpoint) {
    if (typeof endpoint !== 'string') {
        return '';
    }

    const trimmed = endpoint.trim();
    if (!trimmed) {
        return '';
    }

    if (trimmed === API_BASE) {
        return '';
    }

    if (trimmed.startsWith(`${API_BASE}/`)) {
        return trimmed.slice(API_BASE.length);
    }

    return trimmed.startsWith('/') ? trimmed : `/${trimmed}`;
}

function apiBuildURL(endpoint) {
    return `${API_BASE}${apiNormalizeEndpoint(endpoint)}`;
}

function apiResolveLoginRoute() {
    return window.location.pathname.startsWith('/static/') ? '/static/login.html' : '/login';
}

function apiResolveIndexRoute() {
    return window.location.pathname.startsWith('/static/') ? '/static/index.html' : '/index.html';
}

function apiSetLicenseLockState(reason) {
    sessionStorage.setItem('nms_license_locked', 'true');
    if (reason) {
        sessionStorage.setItem('nms_license_lock_reason', reason);
    } else {
        sessionStorage.removeItem('nms_license_lock_reason');
    }
}

function apiClearLicenseLockState() {
    sessionStorage.removeItem('nms_license_locked');
    sessionStorage.removeItem('nms_license_lock_reason');
}

function getAuthToken() {
    return sessionStorage.getItem('nms_token');
}

async function api(endpoint, options = {}) {
    const url = apiBuildURL(endpoint);
    const token = getAuthToken();

    const defaultOptions = {
        cache: 'no-store',
        headers: {
            'Content-Type': 'application/json',
            'Pragma': 'no-cache',
            'Cache-Control': 'no-cache'
        }
    };

    if (token) {
        defaultOptions.headers['Authorization'] = `Bearer ${token}`;
    }

    const mergedOptions = {
        ...defaultOptions,
        ...options,
        headers: {
            ...defaultOptions.headers,
            ...options.headers
        }
    };

    try {
        const response = await fetch(url, mergedOptions);

        if (response.status === 401) {
            if (options.skipRedirectOn401) {
                const text = await response.text();
                let data;
                try {
                    data = JSON.parse(text);
                } catch (e) {
                    throw new Error('Unauthorized');
                }
                throw new Error(data.error || 'Unauthorized');
            }

            sessionStorage.removeItem('nms_token');
            sessionStorage.removeItem('nms_user');
            sessionStorage.removeItem('nms_expires');
            apiClearLicenseLockState();
            window.location.href = apiResolveLoginRoute();
            throw new Error('Unauthorized');
        }

        if (response.status === 423) {
            const text = await response.text();
            let data = null;
            try {
                data = text ? JSON.parse(text) : null;
            } catch (e) {
                data = null;
            }

            const reason = data?.data?.reason || 'license_required';
            apiSetLicenseLockState(reason);

            if (typeof window.handleLicenseLocked === 'function') {
                window.handleLicenseLocked(reason, endpoint);
            } else {
                window.location.replace(`${apiResolveIndexRoute()}?_cb=${Date.now()}`);
            }

            throw new Error(data?.error || 'license_locked');
        }

        if (response.status === 403) {
            throw new Error('Forbidden');
        }

        const text = await response.text();
        if (!text) {
            return { success: response.ok };
        }

        let data;
        try {
            data = JSON.parse(text);
        } catch (e) {
            console.error('JSON Parse Error:', e);
            console.error('Raw Response:', text);
            throw new Error(`Invalid JSON response: ${e.message}. Raw: ${text.substring(0, 100)}`);
        }

        if (!response.ok) {
            throw new Error(data.error || 'API request failed');
        }

        return data;
    } catch (error) {
        console.error('API Error:', error);
        throw error;
    }
}

async function apiGet(endpoint, options = {}) {
    const separator = endpoint.includes('?') ? '&' : '?';
    const timestamp = new Date().getTime();
    return api(`${endpoint}${separator}_ts=${timestamp}`, { ...options, method: 'GET' });
}

async function apiPost(endpoint, body, options = {}) {
    return api(endpoint, {
        ...options,
        method: 'POST',
        body: JSON.stringify(body)
    });
}

async function apiPut(endpoint, body, options = {}) {
    return api(endpoint, {
        ...options,
        method: 'PUT',
        body: JSON.stringify(body)
    });
}

async function apiDelete(endpoint, body) {
    console.log('[API] DELETE Request:', endpoint);
    const opts = { method: 'DELETE' };
    if (body !== undefined) {
        opts.body = JSON.stringify(body);
    }
    return api(endpoint, opts);
}

async function apiUpload(endpoint, formData) {
    const url = apiBuildURL(endpoint);
    const token = getAuthToken();

    const headers = {};
    if (token) {
        headers['Authorization'] = `Bearer ${token}`;
    }

    try {
        const response = await fetch(url, {
            method: 'POST',
            headers,
            body: formData
        });

        if (response.status === 401) {
            sessionStorage.removeItem('nms_token');
            sessionStorage.removeItem('nms_user');
            sessionStorage.removeItem('nms_expires');
            apiClearLicenseLockState();
            window.location.href = apiResolveLoginRoute();
            throw new Error('Unauthorized');
        }

        const data = await response.json();

        if (!response.ok) {
            throw new Error(data.error || 'Upload failed');
        }

        return data;
    } catch (error) {
        console.error('Upload Error:', error);
        throw error;
    }
}

function apiDownload(endpoint) {
    const token = getAuthToken();
    const url = `${API_BASE}${endpoint}`;

    let finalUrl = url;
    if (token) {
        const separator = url.includes('?') ? '&' : '?';
        finalUrl = `${url}${separator}token=${encodeURIComponent(token)}`;
    }

    window.location.href = finalUrl;
}

async function rebootDevice(id, body = {}) {
    return apiPost(`/devices/${id}/reboot`, body);
}

async function getDevicePoE(id) {
    return apiGet(`/devices/${id}/poe`);
}

async function controlPoEPort(deviceId, portIndex, body = {}) {
    return apiPost(`/devices/${deviceId}/poe/${portIndex}/action`, body);
}

async function getDeviceConfigBackups(deviceId) {
    return apiGet(`/devices/${deviceId}/backups`);
}
