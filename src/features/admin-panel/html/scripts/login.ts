export function renderLoginScript(): string {
	return `
var __loginInProgress = false;

function showLoginError(msg) {
	var el = document.getElementById('login-error');
	if (!el) return;
	el.textContent = msg;
	el.classList.remove('hidden');
	setTimeout(function() { el.classList.add('hidden'); }, 4000);
}

function setLoginLoading(loading) {
	var widget = document.getElementById('tg-widget');
	var spinner = document.getElementById('login-loading');
	var hint = document.getElementById('login-hint');
	if (loading) {
		if (widget) widget.style.display = 'none';
		if (spinner) spinner.classList.remove('hidden');
		if (hint) hint.classList.add('hidden');
	} else {
		if (widget) widget.style.display = '';
		if (spinner) spinner.classList.add('hidden');
		if (hint) hint.classList.remove('hidden');
	}
}

function doTelegramLogin(data) {
	if (__loginInProgress) return;
	__loginInProgress = true;
	setLoginLoading(true);
	fetch('/admin/auth/login', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		credentials: 'same-origin',
		body: JSON.stringify(data)
	}).then(function(r) { return r.json(); }).then(function(res) {
		if (res.ok) {
			var spinner = document.getElementById('login-loading');
			if (spinner) spinner.innerHTML = '<span class="text-green-400">登录成功 ✓</span>';
			setTimeout(function() {
				document.body.setAttribute('data-state', 'app');
				window.__adminUser = res.user;
				window.__renderDashboard();
			}, 500);
		} else {
			__loginInProgress = false;
			setLoginLoading(false);
			showLoginError(res.error || '登录失败，请重试');
		}
	}).catch(function() {
		__loginInProgress = false;
		setLoginLoading(false);
		showLoginError('网络错误，请重试');
	});
}

function onTelegramAuth(data) {
	doTelegramLogin(data);
}

window.addEventListener('message', function(e) {
	if (e.origin !== 'https://oauth.telegram.org') return;
	try {
		var d = typeof e.data === 'string' ? JSON.parse(e.data) : e.data;
		if (d.event === 'auth_user' && d.auth_data) {
			doTelegramLogin(d.auth_data);
		}
	} catch (_) {}
});
`;
}
