export function renderSidebarScript(): string {
	return `
var SECTIONS = [];
var currentSection = null;

function registerSection(def) { SECTIONS.push(def); }

function refreshIcons() {
	if (typeof lucide !== 'undefined') lucide.createIcons();
}

function toggleSidebar() {
	var sidebar = document.getElementById('sidebar');
	var backdrop = document.getElementById('sidebar-backdrop');
	if (!sidebar || !backdrop) return;
	var isOpen = !sidebar.classList.contains('-translate-x-full');
	if (isOpen) {
		closeSidebar();
	} else {
		sidebar.classList.remove('-translate-x-full');
		backdrop.classList.remove('hidden');
	}
}

function closeSidebar() {
	var sidebar = document.getElementById('sidebar');
	var backdrop = document.getElementById('sidebar-backdrop');
	if (sidebar) sidebar.classList.add('-translate-x-full');
	if (backdrop) backdrop.classList.add('hidden');
}

window.__renderDashboard = function() {
	var user = window.__adminUser;
	if (!user) return;
	document.body.setAttribute('data-state', 'app');

	var nav = document.getElementById('sidebar-nav');
	nav.innerHTML = '';
	SECTIONS.forEach(function(s) {
		var item = document.createElement('div');
		item.className = 'flex items-center gap-3 px-3 py-2.5 rounded-lg cursor-pointer text-sm font-medium transition-colors ' +
			(currentSection === s.id ? 'bg-white/10 text-white' : 'text-neutral-400 hover:bg-white/5 hover:text-white');
		item.innerHTML = '<i data-lucide="' + s.icon + '" class="w-5 h-5 shrink-0"></i><span>' + esc(s.label) + '</span>';
		item.onclick = function() {
			currentSection = s.id;
			window.__renderDashboard();
			closeSidebar();
		};
		nav.appendChild(item);
	});

	var avatar = document.getElementById('user-avatar');
	avatar.textContent = (user.first_name || '?')[0].toUpperCase();
	var nameEl = document.getElementById('user-name');
	nameEl.textContent = user.username ? '@' + user.username : (user.first_name || '');

	var footer = document.getElementById('sidebar-footer');
	footer.classList.remove('hidden');

	if (!currentSection && SECTIONS.length > 0) currentSection = SECTIONS[0].id;
	var section = SECTIONS.find(function(s) { return s.id === currentSection; });
	if (!section) return;

	var topbarTitle = document.getElementById('topbar-title');
	if (topbarTitle) topbarTitle.innerHTML = '<i data-lucide="' + section.icon + '" class="w-5 h-5 shrink-0"></i><span>' + esc(section.label) + '</span>';

	var mainArea = document.getElementById('main-area');
	var existing = mainArea.querySelector('.content');
	if (existing) existing.remove();
	var contentEl = document.createElement('div');
	contentEl.className = 'content';
	contentEl.innerHTML = '<div class="flex items-center justify-center gap-3 py-20 text-neutral-600 text-sm"><div class="spinner"></div>加载中...</div>';
	mainArea.appendChild(contentEl);

	section.render(contentEl, API);
	refreshIcons();
};

window.__render = function() {
	if (!window.__adminUser) {
		document.body.setAttribute('data-state', 'login');
		return;
	}
	window.__renderDashboard();
};
`;
}
