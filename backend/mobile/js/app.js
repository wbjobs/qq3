const API_BASE = '/api';
const DEVICE_TYPE = 'mobile';
let token = localStorage.getItem('token');
let user = JSON.parse(localStorage.getItem('user') || 'null');
let deviceId = localStorage.getItem('device_id') || generateDeviceId();
let ws = null;
let heartbeatInterval = null;
let historyItems = [];
let maxSeqId = 0;

localStorage.setItem('device_id', deviceId);

function generateDeviceId() {
    return 'mobile-' + Math.random().toString(36).substring(2, 15) + Date.now().toString(36);
}

function getDeviceName() {
    const ua = navigator.userAgent;
    let device = '手机';
    let brand = '';

    if (ua.includes('iPhone')) { brand = 'iPhone'; }
    else if (ua.includes('iPad')) { brand = 'iPad'; }
    else if (ua.includes('Android')) { brand = 'Android'; }
    else if (ua.includes('HarmonyOS')) { brand = '鸿蒙'; }

    if (ua.includes('MicroMessenger')) { device = '微信'; }
    else if (ua.includes('QQ/')) { device = 'QQ'; }

    return brand ? `${brand} · ${device}` : device;
}

function showToast(message, type = 'info') {
    const toast = document.getElementById('toast');
    toast.textContent = message;
    toast.className = `m-toast ${type}`;
    toast.classList.remove('hidden');
    setTimeout(() => toast.classList.add('hidden'), 2600);
}

function switchTab(tab) {
    const loginBtn = document.getElementById('tab-login');
    const regBtn = document.getElementById('tab-register');
    loginBtn.classList.toggle('active', tab === 'login');
    regBtn.classList.toggle('active', tab === 'register');
    document.getElementById('login-form').classList.toggle('hidden', tab !== 'login');
    document.getElementById('register-form').classList.toggle('hidden', tab !== 'register');
}

function switchNav(tab) {
    document.querySelectorAll('.m-nav-tab').forEach(el => {
        el.classList.toggle('active', el.dataset.tab === tab);
    });
    document.querySelectorAll('.m-tab-content').forEach(el => {
        el.classList.toggle('active', el.id === 'tab-' + tab);
    });
}

async function apiCall(endpoint, method = 'GET', data = null) {
    const headers = { 'Content-Type': 'application/json' };
    if (token) headers['Authorization'] = `Bearer ${token}`;

    const options = { method, headers };
    if (data) options.body = JSON.stringify(data);

    const res = await fetch(API_BASE + endpoint, options);
    const result = await res.json();
    if (!res.ok) throw new Error(result.error || '请求失败');
    return result;
}

async function handleLogin(e) {
    e.preventDefault();
    const username = document.getElementById('login-username').value.trim();
    const password = document.getElementById('login-password').value;
    if (!username || !password) {
        showToast('请填写完整信息', 'warning');
        return;
    }
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
    const username = document.getElementById('register-username').value.trim();
    const password = document.getElementById('register-password').value;
    const email = document.getElementById('register-email').value.trim();
    if (!username || !password) {
        showToast('请填写必填项', 'warning');
        return;
    }
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
    document.getElementById('user-display').textContent = '@' + user.username;

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
        console.warn('Device bind warn:', err.message);
    }
}

function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/ws?device_id=${encodeURIComponent(deviceId)}`;

    ws = new WebSocket(wsUrl);

    ws.onopen = () => {
        updateWSStatus(true);
        const authMsg = JSON.stringify({ type: 'auth', token });
        ws.send(authMsg);

        heartbeatInterval = setInterval(() => {
            if (ws.readyState === WebSocket.OPEN) {
                ws.send(JSON.stringify({ type: 'ping' }));
            }
        }, 25000);
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
            console.error('WS parse error:', e);
        }
    };

    ws.onerror = () => updateWSStatus(false);

    ws.onclose = () => {
        updateWSStatus(false);
        if (heartbeatInterval) clearInterval(heartbeatInterval);
        setTimeout(connectWebSocket, 3000);
    };
}

function updateWSStatus(connected) {
    const dot = document.getElementById('ws-dot');
    dot.classList.toggle('online', connected);
    dot.classList.toggle('offline', !connected);
    dot.title = connected ? '在线' : '离线';
}

function onClipboardReceived(data) {
    if (navigator.vibrate) navigator.vibrate(50);
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

async function syncClipboard() {
    const content = document.getElementById('clipboard-input').value.trim();
    if (!content) {
        showToast('请输入内容', 'warning');
        return;
    }
    try {
        const result = await apiCall('/clipboard/sync', 'POST', {
            content,
            device_id: deviceId,
            device_name: getDeviceName(),
            content_type: 'text'
        });
        showToast('已同步到所有设备！', 'success');
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
        showToast('请长按输入框手动粘贴', 'warning');
    }
}

async function copyToClipboard(text) {
    try {
        await navigator.clipboard.writeText(text);
        if (navigator.vibrate) navigator.vibrate(30);
        showToast('已复制', 'success');
    } catch (err) {
        const ta = document.createElement('textarea');
        ta.value = text;
        ta.style.position = 'fixed';
        ta.style.opacity = '0';
        document.body.appendChild(ta);
        ta.select();
        document.execCommand('copy');
        document.body.removeChild(ta);
        if (navigator.vibrate) navigator.vibrate(30);
        showToast('已复制', 'success');
    }
}

async function loadHistory() {
    try {
        const result = await apiCall('/clipboard/history?limit=50&offset=0');
        const items = result.items || [];
        document.getElementById('history-count').textContent = `共 ${result.total || items.length} 条`;

        items.forEach(it => normalizeItem(it));
        historyItems = items.sort((a, b) => (b.seq_id || 0) - (a.seq_id || 0));
        if (historyItems.length > 0) {
            maxSeqId = Math.max(...historyItems.map(it => it.seq_id || 0));
        }

        renderHistory(historyItems);
    } catch (err) {
        showToast('加载失败: ' + err.message, 'error');
    }
}

function renderHistory(items) {
    const container = document.getElementById('history-list');
    if (!items.length) {
        container.innerHTML = `
            <div class="m-empty">
                <svg viewBox="0 0 24 24" width="56" height="56" fill="none" stroke="currentColor" stroke-width="1.5">
                    <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
                    <polyline points="14 2 14 8 20 8"/>
                    <line x1="9" y1="15" x2="15" y2="15"/>
                </svg>
                <p>暂无同步记录</p>
                <p>输入内容点击发送即可开始同步</p>
            </div>
        `;
        return;
    }
    container.innerHTML = items.map(it => historyItemHTML(it)).join('');
}

function historyItemHTML(it) {
    const time = formatTime(it.created_at);
    const device = escapeHTML(it.device_name || '未知设备');
    const content = escapeHTML(it.content || '');
    const translation = it.is_translated && it.translation ? escapeHTML(it.translation) : '';
    const hasTrans = !!translation;
    const seqBadge = it.seq_id ? `<span style="font-size:10px;color:var(--text-3);margin-left:4px;">#${it.seq_id}</span>` : '';

    let transHTML = '';
    if (hasTrans) {
        transHTML = `
            <div class="m-hi-translation">
                <div class="m-hi-tl-label">
                    <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M5 8l6 6"/>
                        <path d="M4 14l6-6 2-3"/>
                        <path d="M2 5h12"/>
                        <path d="M7 2h1"/>
                        <path d="M22 22l-5-10-5 10"/>
                        <path d="M14 18h6"/>
                    </svg>
                    译文
                </div>
                <div class="m-hi-tl-text">${translation}</div>
            </div>
        `;
    }

    return `
        <div class="m-history-item">
            <div class="m-hi-header">
                <span class="m-hi-device">
                    <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2">
                        <rect x="5" y="2" width="14" height="20" rx="2"/>
                        <rect x="2" y="3" width="20" height="14" rx="2"/>
                    </svg>
                    ${device}${seqBadge}
                </span>
                <span class="m-hi-time">${time}</span>
            </div>
            <div class="m-hi-content">${content}</div>
            ${transHTML}
            <div class="m-hi-actions">
                <button class="m-hi-btn primary" onclick="copyToClipboard(\`${escapeForJS(it.content)}\`)">
                    <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2">
                        <rect x="9" y="9" width="13" height="13" rx="2"/>
                        <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
                    </svg>
                    复制原文
                </button>
                ${hasTrans ? `<button class="m-hi-btn" onclick="copyToClipboard(\`${escapeForJS(it.translation)}\`)">
                    <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M5 8l6 6"/>
                        <path d="M4 14l6-6 2-3"/>
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
        console.warn('Load devices warn:', err.message);
    }
}

function renderDevices(devices) {
    const container = document.getElementById('devices-list');
    if (!devices.length) {
        container.innerHTML = `
            <div class="m-empty">
                <svg viewBox="0 0 24 24" width="56" height="56" fill="none" stroke="currentColor" stroke-width="1.5">
                    <rect x="5" y="2" width="14" height="20" rx="2"/>
                    <rect x="2" y="3" width="20" height="14" rx="2"/>
                    <line x1="12" y1="18" x2="12" y2="18"/>
                </svg>
                <p>暂无绑定设备</p>
                <p>登录其他设备后会自动绑定</p>
            </div>
        `;
        return;
    }
    container.innerHTML = devices.map(d => {
        const icon = deviceIconSVG(d.device_type);
        const typeText = { web: '网页端', mobile: '移动端', desktop: '桌面端' }[d.device_type] || '未知';
        const isCurrent = d.device_id === deviceId;
        return `
            <div class="m-device-card" style="${isCurrent ? 'border: 2px solid var(--primary);' : ''}">
                <div class="m-device-icon">${icon}</div>
                <div class="m-device-info">
                    <div class="m-device-name">${escapeHTML(d.device_name)}${isCurrent ? ' <span style="color:var(--primary);font-size:11px;">(当前)</span>' : ''}</div>
                    <div class="m-device-type">${typeText} · ${formatTime(d.last_seen)}</div>
                </div>
                <div class="m-device-status" title="在线"></div>
            </div>
        `;
    }).join('');
}

function deviceIconSVG(type) {
    if (type === 'mobile') {
        return `<svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="5" y="2" width="14" height="20" rx="2"/>
            <line x1="12" y1="18" x2="12" y2="18"/>
        </svg>`;
    } else if (type === 'desktop') {
        return `<svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="2" y="3" width="20" height="14" rx="2"/>
            <line x1="8" y1="21" x2="16" y2="21"/>
            <line x1="12" y1="17" x2="12" y2="21"/>
        </svg>`;
    }
    return `<svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="2">
        <circle cx="12" cy="12" r="10"/>
        <line x1="2" y1="12" x2="22" y2="12"/>
        <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/>
    </svg>`;
}

function logout() {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    token = null;
    user = null;
    if (ws) { ws.close(); ws = null; }
    document.getElementById('main-page').classList.add('hidden');
    document.getElementById('auth-page').classList.remove('hidden');
    showToast('已退出登录');
}

function formatTime(t) {
    if (!t) return '';
    let date = typeof t === 'string' ? new Date(t) : new Date(t);
    const now = new Date();
    const diff = (now - date) / 1000;

    if (diff < 60) return '刚刚';
    if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`;
    if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`;
    if (diff < 86400 * 2) return '昨天';
    if (diff < 86400 * 7) return `${Math.floor(diff / 86400)}天前`;

    const m = String(date.getMonth() + 1).padStart(2, '0');
    const d = String(date.getDate()).padStart(2, '0');
    const hh = String(date.getHours()).padStart(2, '0');
    const mm = String(date.getMinutes()).padStart(2, '0');
    return `${m}-${d} ${hh}:${mm}`;
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
        document.getElementById('user-display').textContent = '@' + user.username;
        bindDevice();
        connectWebSocket();
        loadHistory();
        loadDevices();
    }
});
