export function renderReportsScript(): string {
	return `
registerSection({
	id: 'reports', label: '举报管理', icon: 'message-square',
	render: function(container, api) {
		var currentFilter = 'pending';
		function load(filter) {
			currentFilter = filter;
			var filters = [
				{ key: 'pending', label: '待处理' },
				{ key: 'approved', label: '已通过' },
				{ key: 'dismissed', label: '已驳回' },
				{ key: '', label: '全部' }
			];
			var filtersHtml = filters.map(function(f) {
				return '<button class="px-3 py-1.5 rounded-full text-xs font-medium border transition-colors ' +
					(filter === f.key
						? 'bg-white text-black border-white'
						: 'border-white/15 text-neutral-400 hover:text-white hover:border-white/30') +
					'" data-f="' + f.key + '">' + f.label + '</button>';
			}).join('');
			container.innerHTML =
				'<div class="island bg-neutral-950 border border-white/10 rounded-2xl p-5 mb-4">' +
					'<div class="flex flex-wrap gap-2">' + filtersHtml + '</div>' +
				'</div>' +
				'<div class="flex items-center justify-center gap-3 py-16 text-neutral-600 text-sm"><div class="spinner"></div>加载中...</div>';
			container.querySelectorAll('[data-f]').forEach(function(btn) {
				btn.onclick = function() { load(btn.getAttribute('data-f')); };
			});
			var url = '/admin/api/reports' + (filter ? '?status=' + filter : '');
			api.get(url).then(function(data) { renderCards(container, data.reports, api); });
		}
		function renderCards(container, reports, api) {
			var loading = container.querySelector('.flex.items-center.justify-center');
			if (loading) loading.remove();
			var existing = container.querySelector('.island-list');
			if (existing) existing.remove();
			if (!reports || reports.length === 0) {
				var empty = document.createElement('div');
				empty.className = 'island bg-neutral-950 border border-white/10 rounded-2xl p-8 text-center';
				empty.innerHTML = '<i data-lucide="inbox" class="w-10 h-10 mx-auto mb-3 text-neutral-600"></i><p class="text-neutral-600 text-sm">暂无举报记录</p>';
				container.appendChild(empty);
				refreshIcons();
				return;
			}
			var list = document.createElement('div');
			list.className = 'island-list flex flex-col gap-3';
			reports.forEach(function(r, i) {
				var badge = r.status === 'pending'
					? '<span class="inline-block px-2.5 py-0.5 rounded-full text-xs font-medium bg-yellow-500/10 text-yellow-400">待处理</span>'
					: r.status === 'approved'
					? '<span class="inline-block px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-500/10 text-green-400">已通过</span>'
					: '<span class="inline-block px-2.5 py-0.5 rounded-full text-xs font-medium bg-neutral-500/10 text-neutral-400">已驳回</span>';
				var actions = r.status === 'pending'
					? '<button class="px-3 py-1.5 text-xs rounded-lg border border-white/15 text-neutral-400 hover:text-green-400 hover:border-green-400/50 transition-colors" data-id="' + r.id + '" data-act="approved">通过</button>' +
					  '<button class="px-3 py-1.5 text-xs rounded-lg border border-white/15 text-neutral-400 hover:text-red-400 hover:border-red-400/50 transition-colors" data-id="' + r.id + '" data-act="dismissed">驳回</button>'
					: '';
				var msgPreview = r.reported_message_text
					? '<div class="mt-3 p-3 bg-white/[0.03] rounded-xl border border-white/5 text-xs text-neutral-400 break-words">' +
					  '<span class="text-neutral-600 text-[10px] uppercase tracking-wider">被举报消息</span><br>' +
					  esc(r.reported_message_text) + '</div>'
					: '';
				var card = document.createElement('div');
				card.className = 'island bg-neutral-950 border border-white/10 rounded-2xl p-5 hover:border-white/20 transition-colors';
				card.style.animationDelay = (i * 0.05) + 's';
				card.innerHTML =
					'<div class="flex items-start justify-between gap-3">' +
						'<div class="flex-1 min-w-0">' +
							'<div class="flex items-center gap-2 mb-1.5">' +
								'<span class="text-xs font-mono text-neutral-500">#' + r.id + '</span>' +
								badge +
								'<span class="text-xs text-neutral-600">' + timeAgo(r.created_at) + '</span>' +
							'</div>' +
							'<div class="text-sm text-white mb-1.5">' + esc(r.content) + '</div>' +
							'<div class="flex flex-wrap gap-x-4 gap-y-1 text-xs text-neutral-500">' +
								'<span>群组: <span class="font-mono">' + r.group_id + '</span></span>' +
								'<span>举报人: <span class="font-mono">' + r.reporter_id + '</span></span>' +
								'<span>被举报人: <span class="font-mono">' + r.reported_user_id + '</span></span>' +
							'</div>' +
							msgPreview +
						'</div>' +
						'<div class="flex gap-1.5 shrink-0">' + actions + '</div>' +
					'</div>';
				list.appendChild(card);
			});
			list.querySelectorAll('[data-id]').forEach(function(btn) {
				btn.onclick = function() {
					var id = btn.getAttribute('data-id');
					var act = btn.getAttribute('data-act');
					api.post('/admin/api/reports/' + id + '/resolve', { status: act }).then(function() {
						toast('操作成功', 'success');
						load(currentFilter);
					}).catch(function() { toast('操作失败'); });
				};
			});
			container.appendChild(list);
			refreshIcons();
		}
		load('pending');
	}
});
`;
}
