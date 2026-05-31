export function renderApiScript(): string {
	return `
var API = {
	fetch: function(url, opts) {
		opts = opts || {};
		opts.credentials = 'same-origin';
		if (opts.body && typeof opts.body === 'object' && !(opts.body instanceof FormData)) {
			opts.headers = Object.assign({ 'Content-Type': 'application/json' }, opts.headers);
			opts.body = JSON.stringify(opts.body);
		}
		return fetch(url, opts).then(function(r) {
			if (r.status === 401) { window.__adminUser = null; window.__render(); throw new Error('Unauthorized'); }
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

function toast(msg, type) {
	type = type || 'error';
	var colors = type === 'success'
		? 'border-green-500/30 text-green-400'
		: type === 'warning'
		? 'border-yellow-500/30 text-yellow-400'
		: 'border-red-500/30 text-red-400';
	var el = document.createElement('div');
	el.className = 'fixed top-4 left-1/2 -translate-x-1/2 z-[200] px-5 py-3 bg-neutral-900 border rounded-lg text-sm font-medium shadow-2xl shadow-black/50 toast-enter ' + colors;
	el.textContent = msg;
	document.body.appendChild(el);
	setTimeout(function() {
		el.className = el.className.replace('toast-enter', 'toast-exit');
		setTimeout(function() { el.remove(); }, 300);
	}, 3000);
}
`;
}
