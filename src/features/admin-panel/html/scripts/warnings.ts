export function renderWarningsScript(): string {
	return `
registerSection({
	id: 'warnings', label: '警告记录', icon: 'alert-triangle',
	render: function(container, api) {
		container.innerHTML = '<div class="flex items-center justify-center gap-3 py-16 text-neutral-600 text-sm"><div class="spinner"></div>加载中...</div>';
		api.get('/admin/api/warnings').then(function(data) {
			container.innerHTML = '';
			if (!data.warnings || data.warnings.length === 0) {
				container.innerHTML = '<div class="island bg-neutral-950 border border-white/10 rounded-2xl p-8 text-center"><i data-lucide="check-circle" class="w-10 h-10 mx-auto mb-3 text-neutral-600"></i><p class="text-neutral-600 text-sm">暂无警告记录</p></div>';
				refreshIcons();
				return;
			}
			var card = document.createElement('div');
			card.className = 'island bg-neutral-950 border border-white/10 rounded-2xl overflow-hidden';
			var wrap = document.createElement('div');
			wrap.className = 'overflow-x-auto';
			var rows = data.warnings.map(function(w) {
				return '<tr class="border-b border-white/5 hover:bg-white/5 transition-colors">' +
					'<td class="px-4 py-3 text-xs font-mono text-neutral-500">#' + w.id + '</td>' +
					'<td class="px-4 py-3 text-xs font-mono text-neutral-500">' + w.group_id + '</td>' +
					'<td class="px-4 py-3 text-xs font-mono text-neutral-500">' + w.user_id + '</td>' +
					'<td class="px-4 py-3 text-xs font-mono text-neutral-500">' + w.admin_id + '</td>' +
					'<td class="px-4 py-3 text-sm">' + (w.reason ? esc(w.reason) : '<span class="text-neutral-600">-</span>') + '</td>' +
					'<td class="px-4 py-3 text-xs text-neutral-500">' + timeAgo(w.created_at) + '</td>' +
					'</tr>';
			}).join('');
			wrap.innerHTML = '<table class="w-full text-left"><thead><tr class="bg-white/[0.02]">' +
				['ID','群组','用户','操作人','原因','时间'].map(function(h) {
					return '<th class="px-4 py-3 text-xs font-semibold uppercase tracking-wider text-neutral-500">' + h + '</th>';
				}).join('') +
				'</tr></thead><tbody>' + rows + '</tbody></table>';
			card.appendChild(wrap);
			container.appendChild(card);
			refreshIcons();
		}).catch(function() {
			container.innerHTML = '<div class="island bg-neutral-950 border border-white/10 rounded-2xl p-8 text-center"><i data-lucide="alert-triangle" class="w-10 h-10 mx-auto mb-3 text-neutral-600"></i><p class="text-neutral-600 text-sm">加载失败</p></div>';
			refreshIcons();
		});
	}
});
`;
}
