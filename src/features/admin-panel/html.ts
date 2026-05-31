export function renderAdminHtml(botUsername: string): string {
	return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Neptune Admin</title>
<style>
:root {
--bg-deep: #0d0a1a;
--bg-panel: #151028;
--bg-surface: #1d1635;
--bg-hover: #261e45;
--border: #2d2450;
--border-active: #6c5ce7;
--text: #e8e2f4;
--text-dim: #8b7fb5;
--text-muted: #5a5080;
--accent: #6c5ce7;
--accent-glow: rgba(108, 92, 231, 0.3);
--cyan: #00cec9;
--green: #00b894;
--orange: #fdcb6e;
--red: #e17055;
--pink: #fd79a8;
--sidebar-w: 240px;
--radius: 8px;
--radius-sm: 4px;
}
* { margin: 0; padding: 0; box-sizing: border-box; }
html, body { height: 100%; }
body {
font-family: -apple-system, "Segoe UI", Roboto, sans-serif;
background: var(--bg-deep);
color: var(--text);
display: flex;
min-height: 100vh;
overflow: hidden;
}
::-webkit-scrollbar { width: 6px; }
::-webkit-scrollbar-track { background: transparent; }
::-webkit-scrollbar-thumb { background: var(--border); border-radius: 3px; }
::-webkit-scrollbar-thumb:hover { background: var(--text-muted); }

/* ── Sidebar ───────────────────────────── */
.sidebar {
width: var(--sidebar-w);
min-width: var(--sidebar-w);
height: 100vh;
background: var(--bg-panel);
border-right: 1px solid var(--border);
display: flex;
flex-direction: column;
position: fixed;
left: 0; top: 0; bottom: 0;
z-index: 10;
}
.sidebar-header {
padding: 24px 20px 20px;
border-bottom: 1px solid var(--border);
}
.sidebar-header h1 {
font-size: 18px;
font-weight: 700;
letter-spacing: -0.3px;
color: var(--text);
display: flex;
align-items: center;
gap: 8px;
}
.sidebar-header h1 .logo {
font-size: 22px;
filter: drop-shadow(0 0 6px var(--accent-glow));
}
.sidebar-nav {
flex: 1;
padding: 12px 10px;
display: flex;
flex-direction: column;
gap: 2px;
}
.nav-item {
display: flex;
align-items: center;
gap: 10px;
padding: 10px 14px;
border-radius: var(--radius);
color: var(--text-dim);
cursor: pointer;
transition: all 0.15s ease;
font-size: 14px;
font-weight: 500;
border: 1px solid transparent;
user-select: none;
}
.nav-item:hover {
background: var(--bg-hover);
color: var(--text);
}
.nav-item.active {
background: var(--accent-glow);
color: var(--text);
border-color: var(--border-active);
}
.nav-item .icon { font-size: 16px; width: 20px; text-align: center; }
.sidebar-footer {
padding: 16px 20px;
border-top: 1px solid var(--border);
display: flex;
align-items: center;
gap: 10px;
font-size: 13px;
color: var(--text-dim);
}
.sidebar-footer .avatar {
width: 32px; height: 32px;
border-radius: 50%;
background: var(--accent);
display: flex;
align-items: center;
justify-content: center;
font-weight: 700;
font-size: 14px;
color: #fff;
}

/* ── Main content ──────────────────────── */
.main {
flex: 1;
margin-left: var(--sidebar-w);
display: flex;
flex-direction: column;
height: 100vh;
}
.topbar {
padding: 20px 32px;
border-bottom: 1px solid var(--border);
display: flex;
align-items: center;
justify-content: space-between;
min-height: 64px;
}
.topbar h2 {
font-size: 20px;
font-weight: 700;
letter-spacing: -0.3px;
}
.content {
flex: 1;
overflow-y: auto;
padding: 24px 32px;
}
.filters {
display: flex;
gap: 8px;
margin-bottom: 20px;
flex-wrap: wrap;
}
.filter-btn {
padding: 6px 14px;
border-radius: 999px;
border: 1px solid var(--border);
background: transparent;
color: var(--text-dim);
cursor: pointer;
font-size: 13px;
font-weight: 500;
transition: all 0.15s ease;
}
.filter-btn:hover {
border-color: var(--text-muted);
color: var(--text);
}
.filter-btn.active {
background: var(--accent);
border-color: var(--accent);
color: #fff;
}

/* ── Table ─────────────────────────────── */
.table-wrap {
background: var(--bg-panel);
border: 1px solid var(--border);
border-radius: var(--radius);
overflow: hidden;
}
table { width: 100%; border-collapse: collapse; }
th {
text-align: left;
padding: 12px 16px;
font-size: 12px;
font-weight: 600;
text-transform: uppercase;
letter-spacing: 0.5px;
color: var(--text-muted);
background: var(--bg-surface);
border-bottom: 1px solid var(--border);
white-space: nowrap;
}
td {
padding: 12px 16px;
font-size: 13px;
border-bottom: 1px solid var(--border);
vertical-align: top;
max-width: 300px;
word-break: break-word;
}
tr:last-child td { border-bottom: none; }
tr:hover td { background: var(--bg-hover); }
.mono { font-family: "JetBrains Mono", "SF Mono", "Fira Code", monospace; font-size: 12px; }
.badge {
display: inline-block;
padding: 2px 8px;
border-radius: 999px;
font-size: 11px;
font-weight: 600;
text-transform: uppercase;
letter-spacing: 0.3px;
}
.badge-pending { background: rgba(253, 203, 110, 0.15); color: var(--orange); }
.badge-approved { background: rgba(0, 184, 148, 0.15); color: var(--green); }
.badge-dismissed { background: rgba(139, 127, 181, 0.15); color: var(--text-dim); }
.actions { display: flex; gap: 6px; }
.btn-sm {
padding: 4px 10px;
border-radius: var(--radius-sm);
border: 1px solid var(--border);
background: transparent;
color: var(--text-dim);
cursor: pointer;
font-size: 12px;
font-weight: 500;
transition: all 0.15s ease;
}
.btn-sm:hover { color: var(--text); border-color: var(--text-muted); }
.btn-approve:hover { color: var(--green); border-color: var(--green); }
.btn-dismiss:hover { color: var(--red); border-color: var(--red); }

/* ── Empty state ───────────────────────── */
.empty {
text-align: center;
padding: 60px 20px;
color: var(--text-muted);
font-size: 14px;
}
.empty .icon { font-size: 40px; margin-bottom: 12px; opacity: 0.5; }

/* ── Login ─────────────────────────────── */
.login-box {
text-align: center;
padding: 48px 40px;
background: var(--bg-panel);
border: 1px solid var(--border);
border-radius: 12px;
max-width: 380px;
width: 90%;
animation: fadeInUp 0.4s ease;
}
.login-box .logo { font-size: 48px; margin-bottom: 16px; }
.login-box h2 { font-size: 20px; margin-bottom: 6px; font-weight: 700; }
.login-box p { font-size: 14px; color: var(--text-dim); margin-bottom: 28px; }

/* ── Loading ───────────────────────────── */
.loading {
display: flex;
align-items: center;
justify-content: center;
padding: 40px;
gap: 8px;
color: var(--text-muted);
font-size: 14px;
}
.spinner {
width: 18px; height: 18px;
border: 2px solid var(--border);
border-top-color: var(--accent);
border-radius: 50%;
animation: spin 0.6s linear infinite;
}

/* ── Error toast ───────────────────────── */
.toast {
position: fixed;
top: 20px;
right: 20px;
padding: 12px 20px;
background: var(--bg-surface);
border: 1px solid var(--red);
border-radius: var(--radius);
color: var(--text);
font-size: 13px;
z-index: 1000;
animation: slideIn 0.3s ease;
}

@keyframes fadeInUp { from { opacity: 0; transform: translateY(12px); } to { opacity: 1; transform: translateY(0); } }
@keyframes spin { to { transform: rotate(360deg); } }
@keyframes slideIn { from { opacity: 0; transform: translateX(20px); } to { opacity: 1; transform: translateX(0); } }

@media (max-width: 768px) {
.sidebar { display: none; }
.main { margin-left: 0; }
.content { padding: 16px; }
.topbar { padding: 16px; }
}

/* ── State toggling ────────────────────── */
body[data-state="login"] .sidebar { display: none !important; }
body[data-state="login"] .main { display: none !important; }
body[data-state="app"] #login-page { display: none !important; }
#login-page {
position: fixed;
inset: 0;
display: flex;
align-items: center;
justify-content: center;
background: var(--bg-deep);
z-index: 100;
}
</style>
</head>
<body data-state="login">
<div id="login-page">
<div class="login-box">
<div class="logo">♆</div>
<h2>Neptune Admin</h2>
<p>请使用 Telegram 账号登录</p>
<script async src="https://telegram.org/js/telegram-widget.js?22" data-telegram-login="${botUsername.replace(/\\/g, "\\\\").replace(/'/g, "\\'")}" data-size="large" data-request-access="write"></script>
</div>
</div>
<nav class="sidebar">
<div class="sidebar-header">
<h1><span class="logo">♆</span> Neptune Admin</h1>
</div>
<div class="sidebar-nav" id="sidebar-nav"></div>
<div class="sidebar-footer" id="sidebar-footer" style="display:none">
<div class="avatar" id="user-avatar">?</div>
<span id="user-name">-</span>
</div>
</nav>
<main class="main" id="main-area"></main>

<script>
function onTelegramAuth(data) {
	fetch('/admin/auth/login', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		credentials: 'same-origin',
		body: JSON.stringify(data)
	}).then(function(r) { return r.json(); }).then(function(res) {
		if (res.ok) { document.body.setAttribute('data-state', 'app'); initApp(res.user); }
		else { alert('登录失败: ' + (res.error || '未知错误')); }
	}).catch(function() { alert('登录请求失败'); });
}

window.addEventListener('message', function(e) {
	if (e.origin !== 'https://oauth.telegram.org') return;
	var d = typeof e.data === 'string' ? JSON.parse(e.data) : e.data;
	if (d.event === 'auth_user' && d.auth_data) {
		onTelegramAuth(d.auth_data);
	}
});

(function() {
'use strict';

var SECTIONS = [];
var currentSection = null;
var user = null;
var BOT_USERNAME = '${botUsername.replace(/\\/g, "\\\\").replace(/'/g, "\\'")}';

function registerSection(def) { SECTIONS.push(def); }

var API = {
	fetch: function(url, opts) {
		opts = opts || {};
		opts.credentials = 'same-origin';
		if (opts.body && typeof opts.body === 'object' && !(opts.body instanceof FormData)) {
			opts.headers = Object.assign({ 'Content-Type': 'application/json' }, opts.headers);
			opts.body = JSON.stringify(opts.body);
		}
		return fetch(url, opts).then(function(r) {
			if (r.status === 401) { user = null; render(); throw new Error('Unauthorized'); }
			return r;
		});
	},
	get: function(url) { return this.fetch(url).then(function(r) { return r.json(); }); },
	post: function(url, body) { return this.fetch(url, { method: 'POST', body: body }).then(function(r) { return r.json(); }); }
};

function timeAgo(ts) {
	var diff = Math.floor(Date.now() / 1000) - ts;
	if (diff < 60) return diff + '秒前';
	if (diff < 3600) return Math.floor(diff / 60) + '分钟前';
	if (diff < 86400) return Math.floor(diff / 3600) + '小时前';
	return Math.floor(diff / 86400) + '天前';
}

function esc(s) {
	if (!s) return '';
	var d = document.createElement('div');
	d.textContent = s;
	return d.innerHTML;
}

function toast(msg) {
	var el = document.createElement('div');
	el.className = 'toast';
	el.textContent = msg;
	document.body.appendChild(el);
	setTimeout(function() { el.remove(); }, 3000);
}

function render() {
	if (!user) {
		document.body.setAttribute('data-state', 'login');
		return;
	}
	document.body.setAttribute('data-state', 'app');
	renderDashboard();
}

function initApp(u) {
	user = u;
	renderDashboard();
}

function renderDashboard() {
	var mainArea = document.getElementById('main-area');
	var nav = document.getElementById('sidebar-nav');
	nav.innerHTML = '';
	SECTIONS.forEach(function(s) {
		var item = document.createElement('div');
		item.className = 'nav-item' + (currentSection === s.id ? ' active' : '');
		item.innerHTML = '<span class="icon">' + s.icon + '</span>' + esc(s.label);
		item.onclick = function() { currentSection = s.id; renderDashboard(); };
		nav.appendChild(item);
	});

	var footer = document.getElementById('sidebar-footer');
	footer.style.display = 'flex';
	var avatar = document.getElementById('user-avatar');
	avatar.textContent = (user.first_name || '?')[0].toUpperCase();
	var nameEl = document.getElementById('user-name');
	nameEl.textContent = user.username ? '@' + user.username : (user.first_name || '');

	if (!currentSection && SECTIONS.length > 0) currentSection = SECTIONS[0].id;
	var section = SECTIONS.find(function(s) { return s.id === currentSection; });
	if (!section) return;

	var topbar = document.querySelector('.topbar');
	if (topbar) topbar.remove();

	var topbarEl = document.createElement('div');
	topbarEl.className = 'topbar';
	topbarEl.innerHTML = '<h2>' + section.icon + ' ' + esc(section.label) + '</h2>';
	mainArea.insertBefore(topbarEl, mainArea.firstChild);

	var content = mainArea.querySelector('.content');
	if (content) content.remove();
	var contentEl = document.createElement('div');
	contentEl.className = 'content';
	contentEl.innerHTML = '<div class="loading"><div class="spinner"></div>加载中...</div>';
	mainArea.appendChild(contentEl);

	section.render(contentEl, API);
}

API.get('/admin/auth/me').then(function(res) {
	if (res.user) user = res.user;
	render();
}).catch(function() { render(); });

/* ── Reports Module ───────────────────── */
registerSection({
	id: 'reports', label: '举报管理', icon: '✉️',
	render: function(container, api) {
		var currentFilter = 'pending';
		function load(filter) {
			currentFilter = filter;
			container.innerHTML =
				'<div class="filters">' +
				'<button class="filter-btn' + (filter === 'pending' ? ' active' : '') + '" data-f="pending">待处理</button>' +
				'<button class="filter-btn' + (filter === 'approved' ? ' active' : '') + '" data-f="approved">已通过</button>' +
				'<button class="filter-btn' + (filter === 'dismissed' ? ' active' : '') + '" data-f="dismissed">已驳回</button>' +
				'<button class="filter-btn' + (filter === '' ? ' active' : '') + '" data-f="">全部</button>' +
				'</div>' +
				'<div class="loading"><div class="spinner"></div>加载中...</div>';
			container.querySelectorAll('.filter-btn').forEach(function(btn) {
				btn.onclick = function() { load(btn.getAttribute('data-f')); };
			});
			var url = '/admin/api/reports' + (filter ? '?status=' + filter : '');
			api.get(url).then(function(data) { renderTable(container, data.reports, api); });
		}
		function renderTable(container, reports, api) {
			var loading = container.querySelector('.loading');
			if (loading) loading.remove();
			var existing = container.querySelector('.table-wrap');
			if (existing) existing.remove();
			if (!reports || reports.length === 0) {
				var empty = document.createElement('div');
				empty.className = 'empty';
				empty.innerHTML = '<div class="icon">📭</div><p>暂无举报记录</p>';
				container.appendChild(empty);
				return;
			}
			var wrap = document.createElement('div');
			wrap.className = 'table-wrap';
			var html = '<table><thead><tr><th>ID</th><th>群组</th><th>举报人</th><th>被举报人</th><th>内容</th><th>状态</th><th>时间</th><th>操作</th></tr></thead><tbody>';
			reports.forEach(function(r) {
				var badge = 'badge-' + r.status;
				var label = r.status === 'pending' ? '待处理' : r.status === 'approved' ? '已通过' : '已驳回';
				var actions = '';
				if (r.status === 'pending') {
					actions =
						'<button class="btn-sm btn-approve" data-id="' + r.id + '" data-act="approved">通过</button>' +
						'<button class="btn-sm btn-dismiss" data-id="' + r.id + '" data-act="dismissed">驳回</button>';
				}
				html += '<tr>' +
					'<td class="mono">#' + r.id + '</td>' +
					'<td class="mono">' + r.group_id + '</td>' +
					'<td class="mono">' + r.reporter_id + '</td>' +
					'<td class="mono">' + r.reported_user_id + '</td>' +
					'<td>' + esc(r.content) + '</td>' +
					'<td><span class="badge ' + badge + '">' + label + '</span></td>' +
					'<td>' + timeAgo(r.created_at) + '</td>' +
					'<td class="actions">' + actions + '</td>' +
					'</tr>';
			});
			html += '</tbody></table>';
			wrap.innerHTML = html;
			wrap.querySelectorAll('.btn-sm').forEach(function(btn) {
				btn.onclick = function() {
					var id = btn.getAttribute('data-id');
					var act = btn.getAttribute('data-act');
					api.post('/admin/api/reports/' + id + '/resolve', { status: act }).then(function() {
						load(currentFilter);
					}).catch(function() { toast('操作失败'); });
				};
			});
			container.appendChild(wrap);
		}
		load('pending');
	}
});

/* ── Warnings Module ──────────────────── */
registerSection({
	id: 'warnings', label: '警告记录', icon: '⚠️',
	render: function(container, api) {
		container.innerHTML = '<div class="loading"><div class="spinner"></div>加载中...</div>';
		api.get('/admin/api/warnings').then(function(data) {
			container.innerHTML = '';
			if (!data.warnings || data.warnings.length === 0) {
				container.innerHTML = '<div class="empty"><div class="icon">✅</div><p>暂无警告记录</p></div>';
				return;
			}
			var wrap = document.createElement('div');
			wrap.className = 'table-wrap';
			var html = '<table><thead><tr><th>ID</th><th>群组</th><th>用户</th><th>操作人</th><th>原因</th><th>时间</th></tr></thead><tbody>';
			data.warnings.forEach(function(w) {
				html += '<tr>' +
					'<td class="mono">#' + w.id + '</td>' +
					'<td class="mono">' + w.group_id + '</td>' +
					'<td class="mono">' + w.user_id + '</td>' +
					'<td class="mono">' + w.admin_id + '</td>' +
					'<td>' + (w.reason ? esc(w.reason) : '<span style="color:var(--text-muted)">-</span>') + '</td>' +
					'<td>' + timeAgo(w.created_at) + '</td>' +
					'</tr>';
			});
			html += '</tbody></table>';
			wrap.innerHTML = html;
			container.appendChild(wrap);
		}).catch(function() { container.innerHTML = '<div class="empty"><div class="icon">⚠️</div><p>加载失败</p></div>'; });
	}
});
})();
</script>
</body>
</html>`;
}
