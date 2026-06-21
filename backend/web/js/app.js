const API_BASE = '/api';
const DEVICE_TYPE = 'web';
let token = localStorage.getItem('token');
let user = JSON.parse(localStorage.getItem('user') || 'null');
let deviceId = localStorage.getItem('device_id') || generateDeviceId();
let ws = null;
let heartbeatInterval = null;
let historyItems = [];
let maxSeqId = 0;

localStorage.setItem('device_id', deviceId);

function generateDeviceId() {
    return 'web-' + Math.random().toString(36).substring(2, 15) + Date.now().toString(36);
}

function getDeviceName() {
    const browser = detectBrowser();
    const os = detectOS();
    return `${os} · ${browser}`;
}

function detectBrowser() {
    const ua = navigator.userAgent;
    if (ua.includes('Edg')) return 'Edge';
    if (ua.includes('Chrome')) return 'Chrome';
    if (ua.includes('Firefox')) return 'Firefox';
    if (ua.includes('Safari')) return 'Safari';
    return 'Browser';
}

function detectOS() {
    const ua = navigator.userAgent;
    if (ua.includes('Windows')) return 'Windows';
    if (ua.includes('Mac')) return 'macOS';
    if (ua.includes('Linux')) return 'Linux';
    if (ua.includes('Android')) return 'Android';
    if (ua.includes('iPhone') || ua.includes('iPad')) return 'iOS';
    return 'Unknown';
}

function showToast(message, type = 'info') {
    const toast = document.getElementById('toast');
    toast.textContent = message;
    toast.className = `toast ${type}`;
    setTimeout(() => toast.classList.add('hidden'), 3000);
}

function switchTab(tab) {
    document.getElementById('tab-login').classList.toggle('active', tab === 'login');
    document.getElementById('tab-register').classList.toggle('active', tab === 'register');
    document.getElementById('login-form').classList.toggle('hidden', tab !== 'login');
    document.getElementById('register-form').classList.toggle('hidden', tab !== 'register');
}

async function apiCall(endpoint, method = 'GET', data = null) {
    const headers = {
        'Content-Type': 'application/json',
    };
    if (token) {
        headers['Authorization'] = `Bearer ${token}`;
    }

    const options = { method, headers };
    if (data) {
        options.body = JSON.stringify(data);
    }

    const res = await fetch(API_BASE + endpoint, options);
    const result = await res.json();

    if (!res.ok) {
        throw new Error(result.error || '请求失败');
    }
    return result;
}

async function handleLogin(e) {
    e.preventDefault();
    const username = document.getElementById('login-username').value;
    const password = document.getElementById('login-password').value;

    try {
        const result = await apiCall('/login', 'POST', { username, password });
        onAuthSuccess(result);
        showToast('登录成功！', 'success');
    } catch (err) {
        showToast(err.message, 'error');
    }
}

async function handleRegister(e) {
    e.preventDefault();
    const username = document.getElementById('register-username').value;
    const password = document.getElementById('register-password').value;
    const email = document.getElementById('register-email').value;

    try {
        const result = await apiCall('/register', 'POST', { username, password, email });
        onAuthSuccess(result);
        showToast('注册成功！', 'success');
    } catch (err) {
        showToast(err.message, 'error');
    }
}

function onAuthSuccess(result) {
    token = result.token;
    user = result.user;
    localStorage.setItem('token', token);
    localStorage.setItem('user', JSON.stringify(user));

    document.getElementById('auth-page').classList.add('hidden');
    document.getElementById('main-page').classList.remove('hidden');

    document.getElementById('user-display').textContent = user.username;

    bindDevice();
    connectWebSocket();
    loadHistory();
    loadDevices();
}

async function bindDevice() {
    try {
        await apiCall('/device/bind', 'POST', {
            device_id: deviceId,
            device_name: getDeviceName(),
            device_type: DEVICE_TYPE
        });
    } catch (err) {
        console.warn('设备绑定警告:', err.message);
    }
}

function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/ws?device_id=${encodeURIComponent(deviceId)}`;

    ws = new WebSocket(wsUrl);
    ws.onopen = () => {
        updateWSStatus(true);
        console.log('WebSocket连接成功');

        const authMsg = JSON.stringify({ type: 'auth', token });
        ws.send(authMsg);

        heartbeatInterval = setInterval(() => {
            if (ws.readyState === WebSocket.OPEN) {
                ws.send(JSON.stringify({ type: 'ping' }));
            }
        }, 30000);
    };

    ws.onmessage = (event) => {
        try {
            const msg = JSON.parse(event.data);
            if (msg.type === 'clipboard_sync') {
                onClipboardReceived(msg.data);
            } else if (msg.type === 'translation_update') {
                onTranslationUpdate(msg.data);
            }
        } catch (e) {
            console.error('解析WebSocket消息失败:', e);
        }
    };

    ws.onerror = (err) => {
        console.error('WebSocket错误:', err);
        updateWSStatus(false);
    };

    ws.onclose = () => {
        updateWSStatus(false);
        if (heartbeatInterval) {
            clearInterval(heartbeatInterval);
        }
        console.log('WebSocket连接关闭，3秒后重连...');
        setTimeout(connectWebSocket, 3000);
    };
}

function updateWSStatus(connected) {
    const el = document.getElementById('ws-status');
    el.classList.toggle('connected', connected);
    el.classList.toggle('disconnected', !connected);
    el.querySelector('.status-text').textContent = connected ? '在线' : '离线';
}

function normalizeItem(it) {
    if (it.seq_id === undefined || it.seq_id === null) {
        it.seq_id = Date.now();
    }
    return it;
}

function insertHistoryItem(item) {
    normalizeItem(item);

    const exists = historyItems.some(it => it.id === item.id);
    if (!exists) {
        historyItems.push(item);
    }

    historyItems.sort((a, b) => (b.seq_id || 0) - (a.seq_id || 0));

    if (historyItems.length > 100) {
        historyItems = historyItems.slice(0, 100);
    }

    maxSeqId = Math.max(maxSeqId, item.seq_id || 0);
    renderHistory(historyItems);
}

function onClipboardReceived(data) {
    showToast(`收到来自 ${data.device_name || '其他设备'} 的内容`, 'success');
    insertHistoryItem(data);
}

function onTranslationUpdate(data) {
    const idx = historyItems.findIndex(it => it.id === data.id);
    if (idx !== -1) {
        historyItems[idx].translation = data.translation;
        historyItems[idx].is_translated = true;
        historyItems[idx].seq_id = data.seq_id;
        renderHistory(historyItems);
        showToast('翻译已更新', 'success');
    }
}

async function syncClipboard() {
    const content = document.getElementById('clipboard-input').value.trim();
    if (!content) {
        showToast('请输入要同步的内容', 'warning');
        return;
    }

    try {
        const result = await apiCall('/clipboard/sync', 'POST', {
            content,
            device_id: deviceId,
            device_name: getDeviceName(),
            content_type: 'text'
        });

        showToast('同步成功！', 'success');
        document.getElementById('clipboard-input').value = '';

        insertHistoryItem(result.item);
    } catch (err) {
        showToast(err.message, 'error');
    }
}

async function pasteFromClipboard() {
    try {
        const text = await navigator.clipboard.readText();
        if (text) {
            document.getElementById('clipboard-input').value = text;
            showToast('已从剪贴板粘贴', 'success');
        } else {
            showToast('剪贴板为空', 'warning');
        }
    } catch (err) {
        showToast('无法访问剪贴板，请手动粘贴', 'warning');
    }
}

async function copyToClipboard(text) {
    try {
        await navigator.clipboard.writeText(text);
        showToast('已复制到剪贴板', 'success');
    } catch (err) {
        const textarea = document.createElement('textarea');
        textarea.value = text;
        document.body.appendChild(textarea);
        textarea.select();
        document.execCommand('copy');
        document.body.removeChild(textarea);
        showToast('已复制到剪贴板', 'success');
    }
}

async function loadHistory() {
    try {
        const result = await apiCall('/clipboard/history?limit=50&offset=0');
        const items = result.items || [];

        items.forEach(it => normalizeItem(it));
        historyItems = items.sort((a, b) => (b.seq_id || 0) - (a.seq_id || 0));
        if (historyItems.length > 0) {
            maxSeqId = Math.max(...historyItems.map(it => it.seq_id || 0));
        }

        renderHistory(historyItems);
    } catch (err) {
        showToast('加载历史失败: ' + err.message, 'error');
    }
}

function renderHistory(items) {
    const container = document.getElementById('history-list');
    if (!items.length) {
        container.innerHTML = `
            <div class="history-empty">
                <svg viewBox="0 0 24 24" width="48" height="48" fill="none" stroke="currentColor" stroke-width="1.5">
                    <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
                    <polyline points="14 2 14 8 20 8"/>
                    <line x1="9" y1="15" x2="15" y2="15"/>
                </svg>
                <p>暂无同步记录</p>
                <p style="font-size:12px;margin-top:4px">在上方输入内容开始同步吧</p>
            </div>
        `;
        return;
    }
    container.innerHTML = items.map(item => historyItemHTML(item)).join('');
}

function historyItemHTML(item) {
    const time = formatTime(item.created_at);
    const device = escapeHTML(item.device_name || '未知设备');
    const content = escapeHTML(item.content || '');
    const translation = item.is_translated && item.translation ? escapeHTML(item.translation) : '';
    const seqBadge = item.seq_id ? `<span style="font-size:11px;color:var(--text-muted);margin-left:6px;font-weight:400;">#${item.seq_id}</span>` : '';

    let translationHTML = '';
    if (translation) {
        translationHTML = `
            <div class="history-translation">
                <div class="translation-label">
                    <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M5 8l6 6"/>
                        <path d="M4 14l6-6 2-3"/>
                        <path d="M2 5h12"/>
                        <path d="M7 2h1"/>
                        <path d="M22 22l-5-10-5 10"/>
                        <path d="M14 18h6"/>
                    </svg>
                    中文翻译
                </div>
                <div class="translation-text">${translation}</div>
            </div>
        `;
    }

    return `
        <div class="history-item">
            <div class="history-item-header">
                <span class="history-device">
                    <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2">
                        <rect x="2" y="3" width="20" height="14" rx="2"/>
                        <line x1="8" y1="21" x2="16" y2="21"/>
                        <line x1="12" y1="17" x2="12" y2="21"/>
                    </svg>
                    ${device}${seqBadge}
                </span>
                <span class="history-time">${time}</span>
            </div>
            <div class="history-content">${content}</div>
            ${translationHTML}
            <div class="history-actions">
                <button class="btn-secondary" onclick="copyToClipboard(\`${escapeForJS(item.content)}\`)">
                    <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
                        <rect x="9" y="9" width="13" height="13" rx="2"/>
                        <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
                    </svg>
                    复制原文
                </button>
                ${translation ? `<button class="btn-secondary" onclick="copyToClipboard(\`${escapeForJS(item.translation)}\`)">
                    <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M5 8l6 6"/>
                        <path d="M4 14l6-6 2-3"/>
                        <path d="M2 5h12"/>
                        <path d="M7 2h1"/>
                        <path d="M22 22l-5-10-5 10"/>
                        <path d="M14 18h6"/>
                    </svg>
                    复制译文
                </button>` : ''}
            </div>
        </div>
    `;
}

async function loadDevices() {
    try {
        const result = await apiCall('/devices');
        renderDevices(result.devices || []);
    } catch (err) {
        console.warn('加载设备失败:', err.message);
    }
}

function renderDevices(devices) {
    const container = document.getElementById('devices-list');
    if (!devices.length) {
        container.innerHTML = '<p style="color:var(--text-muted);font-size:14px;text-align:center;padding:20px">暂无绑定设备</p>';
        return;
    }

    container.innerHTML = devices.map(device => {
        const icon = getDeviceIcon(device.device_type);
        const typeText = getDeviceTypeText(device.device_type);
        return `
            <div class="device-card">
                <div class="device-icon">${icon}</div>
                <div class="device-info">
                    <div class="device-name">${escapeHTML(device.device_name)}</div>
                    <div class="device-type">${typeText}</div>
                </div>
            </div>
        `;
    }).join('');
}

function getDeviceIcon(type) {
    if (type === 'mobile') {
        return `<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="5" y="2" width="14" height="20" rx="2"/>
            <line x1="12" y1="18" x2="12" y2="18"/>
        </svg>`;
    } else if (type === 'desktop') {
        return `<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="2" y="3" width="20" height="14" rx="2"/>
            <line x1="8" y1="21" x2="16" y2="21"/>
            <line x1="12" y1="17" x2="12" y2="21"/>
        </svg>`;
    }
    return `<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
        <circle cx="12" cy="12" r="10"/>
        <line x1="2" y1="12" x2="22" y2="12"/>
        <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/>
    </svg>`;
}

function getDeviceTypeText(type) {
    const map = { web: '网页端', mobile: '移动端', desktop: '桌面端' };
    return map[type] || '其他';
}

function logout() {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    token = null;
    user = null;

    if (ws) {
        ws.close();
        ws = null;
    }

    document.getElementById('main-page').classList.add('hidden');
    document.getElementById('auth-page').classList.remove('hidden');
    showToast('已退出登录');
}

function formatTime(t) {
    if (!t) return '';
    let date;
    if (typeof t === 'string') {
        date = new Date(t);
    } else {
        date = new Date(t);
    }
    const now = new Date();
    const diff = (now - date) / 1000;

    if (diff < 60) return '刚刚';
    if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`;
    if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`;

    const y = date.getFullYear();
    const m = String(date.getMonth() + 1).padStart(2, '0');
    const d = String(date.getDate()).padStart(2, '0');
    const hh = String(date.getHours()).padStart(2, '0');
    const mm = String(date.getMinutes()).padStart(2, '0');
    return `${y}-${m}-${d} ${hh}:${mm}`;
}

function escapeHTML(str) {
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

function escapeForJS(str) {
    if (!str) return '';
    return str.replace(/\\/g, '\\\\').replace(/`/g, '\\`').replace(/\$/g, '\\$').replace(/\n/g, '\\n');
}

document.addEventListener('DOMContentLoaded', () => {
    if (token && user) {
        document.getElementById('auth-page').classList.add('hidden');
        document.getElementById('main-page').classList.remove('hidden');
        document.getElementById('user-display').textContent = user.username;
        bindDevice();
        connectWebSocket();
        loadHistory();
        loadDevices();
    }

    document.getElementById('clipboard-input').addEventListener('keydown', (e) => {
        if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
            syncClipboard();
        }
    });
});
