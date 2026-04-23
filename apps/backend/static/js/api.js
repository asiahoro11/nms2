// API 基礎設定
const API_BASE = '/api/v1';

function apiResolveLoginRoute() {
    return window.location.pathname.startsWith('/static/') ? '/static/login.html' : '/login';
}

// Get auth token
function getAuthToken() {
    return sessionStorage.getItem('nms_token');
}

// API 呼叫函數
async function api(endpoint, options = {}) {
    const url = `${API_BASE}${endpoint}`;

    const token = getAuthToken();

    const defaultOptions = {
        cache: 'no-store', // Aggressively disable caching
        headers: {
            'Content-Type': 'application/json',
            'Pragma': 'no-cache',
            'Cache-Control': 'no-cache'
        },
    };

    // Add Authorization header if token exists
    if (token) {
        defaultOptions.headers['Authorization'] = `Bearer ${token}`;
    }

    const mergedOptions = {
        ...defaultOptions,
        ...options,
        headers: {
            ...defaultOptions.headers,
            ...options.headers,
        },
    };

    try {
        const response = await fetch(url, mergedOptions);

        // Handle 401 Unauthorized - redirect to login
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
            window.location.href = apiResolveLoginRoute();
            throw new Error('未授權，請重新登入');
        }

        // Handle 403 Forbidden
        if (response.status === 403) {
            throw new Error('權限不足');
        }

        const text = await response.text();

        // Handle empty response (e.g. 204 No Content)
        if (!text) {
            return { success: response.ok };
        }

        let data;
        try {
            data = JSON.parse(text);
        } catch (e) {
            console.error('JSON Parse Error:', e);
            console.error('Raw Response:', text);
            throw new Error(`伺服器回應格式錯誤: ${e.message}. Raw: ${text.substring(0, 100)}`);
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

// GET 請求
async function apiGet(endpoint, options = {}) {
    const separator = endpoint.includes('?') ? '&' : '?';
    const timestamp = new Date().getTime();
    return api(`${endpoint}${separator}_ts=${timestamp}`, { ...options, method: 'GET' });
}

// POST 請求
async function apiPost(endpoint, body, options = {}) {
    return api(endpoint, {
        ...options,
        method: 'POST',
        body: JSON.stringify(body),
    });
}

// PUT 請求
async function apiPut(endpoint, body, options = {}) {
    return api(endpoint, {
        ...options,
        method: 'PUT',
        body: JSON.stringify(body),
    });
}

// DELETE 請求（body 可選，用於批次刪除等）
async function apiDelete(endpoint, body) {
    console.log('[API] DELETE Request:', endpoint);
    const opts = { method: 'DELETE' };
    if (body !== undefined) {
        opts.body = JSON.stringify(body);
    }
    return api(endpoint, opts);
}

// 上傳檔案
async function apiUpload(endpoint, formData) {
    const url = `${API_BASE}${endpoint}`;
    const token = getAuthToken();

    const headers = {};
    if (token) {
        headers['Authorization'] = `Bearer ${token}`;
    }

    try {
        const response = await fetch(url, {
            method: 'POST',
            headers,
            body: formData,
        });

        // Handle 401 Unauthorized
        if (response.status === 401) {
            sessionStorage.removeItem('nms_token');
            sessionStorage.removeItem('nms_user');
            window.location.href = apiResolveLoginRoute();
            throw new Error('未授權，請重新登入');
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

// 下載檔案
function apiDownload(endpoint) {
    const token = getAuthToken();
    const url = `${API_BASE}${endpoint}`;

    // Construct final URL
    let finalUrl = url;
    if (token) {
        const separator = url.includes('?') ? '&' : '?';
        finalUrl = `${url}${separator}token=${encodeURIComponent(token)}`;
    }

    // Use location.href for downloads to avoid popup blockers
    // The browser will handle the Content-Disposition: attachment without navigating away
    window.location.href = finalUrl;
}

// 設備控制
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
