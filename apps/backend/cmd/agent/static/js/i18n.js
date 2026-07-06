// Made by YTSworks
// YTS工作室製作
const I18N_STORAGE_KEY = 'nms_lang';
const DEFAULT_LANG = 'zh-TW';
const FALLBACK_LANG = 'zh-TW';
const SUPPORTED_LANGS = {
    'zh-TW': '繁體中文',
    'en-US': 'English',
    'zh-CN': '简体中文',
    'ja-JP': '日本語',
    'ko-KR': '한국어'
};

const builtInTranslations = {
    app: {
        title: '管理系統',
        title_short: '管理系統'
    },
    nav: {
        dashboard: '儀表板',
        devices: '設備清單',
        topology: '網路拓樸',
        cameras: '攝影機監控',
        access_control: '門禁管理',
        pdu: 'PDU/UPS 監控',
        logs: '日誌與稽核',
        admin: '系統管理',
        logout: '登出'
    },
    common: {
        search: '搜尋',
        status: '狀態',
        action: '操作',
        loading: '載入中...',
        success: '成功',
        error: '錯誤',
        cancel: '取消',
        save: '儲存',
        theme: '主題'
    },
    login: {
        title: '系統登入',
        subtitle: '請登入以繼續',
        username: '登入帳號',
        password: '登入密碼',
        language: '語系切換',
        login_btn: '登入',
        processing: '登入中...',
        forgot_password: '忘記密碼？',
        request_reset_title: '重設您的密碼',
        request_reset_desc: '請輸入您註冊的 Email，將發送重設連結到您的信箱。',
        email: '電子郵件 (Email)',
        email_placeholder: '例如: user@example.com',
        send_reset_link: '發送重設信件',
        back_to_login: '返回登入',
        reset_success_msg: '重設指令已發出',
        reset_success_desc: '如果此 Email 已註冊，您將收到重設密碼的信件。',
        error_required: '請輸入電子郵件',
        error_failed: '登入失敗',
        error_connection: '連線失敗，請稍後再試'
    },
    password_reset: {
        title: '重設密碼',
        desc: '請輸入新的密碼以完成重設。',
        new_password: '新密碼',
        confirm_password: '確認新密碼',
        reset_btn: '更新密碼',
        success: '密碼已重設成功，正在返回登入頁...',
        error_invalid: '重設連結無效或已過期。',
        error_mismatch: '兩次輸入的密碼不一致',
        error_requirements: '密碼至少 8 個字，且需包含英文大小寫與數字或符號'
    },
    logs: {
        title: '日誌與稽核',
        center_title: '日誌與稽核',
        tabs: {
            system: '系統日誌',
            device: '設備日誌',
            audit: '稽核日誌',
            config: '設定變更'
        },
        types: {
            system: '系統日誌',
            device: '設備日誌',
            audit: '稽核日誌',
            config_change: '設定變更'
        }
    },
    audit: {
        actions: {
            login: '登入系統',
            logout: '登出系統',
            login_failed: '登入失敗'
        }
    }
};

let currentLang = localStorage.getItem(I18N_STORAGE_KEY) || DEFAULT_LANG;
let translations = builtInTranslations;
let fallbackTranslations = builtInTranslations;
const translationCache = {};

function isPlainObject(value) {
    return value && typeof value === 'object' && !Array.isArray(value);
}

function deepMergeTranslations(base, override) {
    const result = { ...base };
    for (const [key, value] of Object.entries(override || {})) {
        if (isPlainObject(value) && isPlainObject(result[key])) {
            result[key] = deepMergeTranslations(result[key], value);
        } else {
            result[key] = value;
        }
    }
    return result;
}

function getPathValue(source, pathParts) {
    let cursor = source;
    for (const part of pathParts) {
        const nextKey = cursor ? Object.keys(cursor).find((key) => key.toLowerCase() === part.toLowerCase()) : null;
        if (!nextKey) {
            return undefined;
        }
        cursor = cursor[nextKey];
    }
    return cursor;
}

function isUnresolvedTranslationValue(value, keyPath) {
    if (typeof value !== 'string') return false;

    const normalized = value.trim();
    if (!normalized) return true;
    if (normalized === keyPath) return true;
    if (/^[a-z0-9_]+(\.[a-z0-9_]+)+$/i.test(normalized)) return true;
    if (/[\uE000-\uF8FF]/.test(normalized)) return true;
    if (normalized.includes('??')) return true;

    const suspiciousCount = (normalized.match(/[蝜蝞撖銝隤瑼霈閮蝺]/g) || []).length;
    if (suspiciousCount >= 2) return true;

    return false;
}

function sanitizeTranslationTree(node, pathParts = []) {
    if (Array.isArray(node)) {
        return node;
    }

    if (isPlainObject(node)) {
        const cleaned = {};
        for (const [key, value] of Object.entries(node)) {
            const next = sanitizeTranslationTree(value, [...pathParts, key]);
            if (next !== undefined) {
                cleaned[key] = next;
            }
        }
        return cleaned;
    }

    if (typeof node === 'string') {
        const keyPath = pathParts.join('.');
        return isUnresolvedTranslationValue(node, keyPath) ? undefined : node;
    }

    return node;
}

async function fetchTranslationsFile(lang) {
    if (translationCache[lang]) {
        return translationCache[lang];
    }

    const assetVersion = window.NMS_CONFIG?.assetVersion || 'v1.2.4';
    const response = await fetch(`/static/i18n/${lang}.json?v=${encodeURIComponent(assetVersion)}`, {
        cache: 'no-store'
    });

    if (!response.ok) {
        throw new Error(`Failed to load locale ${lang}: ${response.status}`);
    }

    const data = await response.json();
    translationCache[lang] = sanitizeTranslationTree(data) || {};
    return translationCache[lang];
}

async function loadTranslations(lang) {
    try {
        const fallbackData = await fetchTranslationsFile(FALLBACK_LANG);
        fallbackTranslations = deepMergeTranslations(builtInTranslations, fallbackData);
    } catch (error) {
        console.error(`[i18n] Fallback locale load failed: ${FALLBACK_LANG}`, error);
        fallbackTranslations = builtInTranslations;
    }

    if (lang === FALLBACK_LANG) {
        translations = fallbackTranslations;
        return;
    }

    try {
        const localeData = await fetchTranslationsFile(lang);
        translations = deepMergeTranslations(fallbackTranslations, localeData);
    } catch (error) {
        console.error(`[i18n] Locale load failed: ${lang}, using fallback locale.`, error);
        translations = fallbackTranslations;
    }
}

function t(key, params = {}) {
    if (!key) return '';

    const keyParts = key.split('.');
    const translated = getPathValue(translations, keyParts);
    if (typeof translated === 'string' && !isUnresolvedTranslationValue(translated, key)) {
        return translated.replace(/{(\w+)}/g, (match, token) => (params[token] !== undefined ? params[token] : match));
    }

    const fallback = getPathValue(builtInTranslations, keyParts);
    if (typeof fallback === 'string' && !isUnresolvedTranslationValue(fallback, key)) {
        return fallback.replace(/{(\w+)}/g, (match, token) => (params[token] !== undefined ? params[token] : match));
    }

    return key;
}

function applyTranslations() {
    document.querySelectorAll('[data-i18n]').forEach((element) => {
        let key = element.dataset.i18n;
        let attr = null;

        if (key.startsWith('[')) {
            const match = key.match(/^\[(.+?)\](.*)$/);
            if (match) {
                attr = match[1];
                key = match[2];
            }
        }

        const translated = t(key);
        if (attr) {
            element.setAttribute(attr, translated);
        } else if (element.tagName === 'INPUT' || element.tagName === 'TEXTAREA') {
            element.placeholder = translated;
        } else {
            element.textContent = translated;
        }
    });
}

function renderLanguageSwitcher() {
    const container = document.getElementById('lang-switcher-container') || document.getElementById('login-lang-container');
    if (!container) return;

    const select = document.createElement('select');
    select.className = 'lang-select';
    select.innerHTML = Object.entries(SUPPORTED_LANGS)
        .map(([lang, label]) => `<option value="${lang}" ${lang === currentLang ? 'selected' : ''}>${label}</option>`)
        .join('');

    container.innerHTML = '';
    container.appendChild(select);
    select.onchange = (event) => changeLanguage(event.target.value);
}

async function changeLanguage(lang) {
    currentLang = lang;
    localStorage.setItem(I18N_STORAGE_KEY, lang);
    await loadTranslations(lang);
    applyTranslations();
    renderLanguageSwitcher();
    window.dispatchEvent(new CustomEvent('languageChanged', { detail: { lang } }));

    if (typeof window.currentPageReload === 'function' && !window.isModalOpen) {
        window.currentPageReload();
    }
}

async function initI18n() {
    await loadTranslations(currentLang);
    applyTranslations();
    renderLanguageSwitcher();
}

(async function bootstrapI18n() {
    await initI18n();
})();
