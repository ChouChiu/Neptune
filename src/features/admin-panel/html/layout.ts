export function renderLayout(
	botUsername: string,
	styles: string,
	scripts: string,
): string {
	const safe = botUsername
		.replace(/&/g, "&amp;")
		.replace(/"/g, "&quot;")
		.replace(/</g, "&lt;")
		.replace(/>/g, "&gt;")
		.replace(/'/g, "&#39;");
	return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Neptune Admin</title>
<link rel="icon" type="image/jpeg" href="data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAASABIAAD/4QCARXhpZgAATU0AKgAAAAgABAEaAAUAAAABAAAAPgEbAAUAAAABAAAARgEoAAMAAAABAAIAAIdpAAQAAAABAAAATgAAAAAAAABIAAAAAQAAAEgAAAABAAOgAQADAAAAAf//AACgAgAEAAAAAQAAAECgAwAEAAAAAQAAAEAAAAAA/+0AOFBob3Rvc2hvcCAzLjAAOEJJTQQEAAAAAAAAOEJJTQQlAAAAAAAQ1B2M2Y8AsgTpgAmY7PhCfv/iAdhJQ0NfUFJPRklMRQABAQAAAcgAAAAABDAAAG1udHJSR0IgWFlaIAfgAAEAAQAAAAAAAGFjc3AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAD21gABAAAAANMtAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACWRlc2MAAADwAAAAJHJYWVoAAAEUAAAAFGdYWVoAAAEoAAAAFGJYWVoAAAE8AAAAFHd0cHQAAAFQAAAAFHJUUkMAAAFkAAAAKGdUUkMAAAFkAAAAKGJUUkMAAAFkAAAAKGNwcnQAAAGMAAAAPG1sdWMAAAAAAAAAAQAAAAxlblVTAAAACAAAABwAcwBSAEcAQlhZWiAAAAAAAABvogAAOPUAAAOQWFlaIAAAAAAAAGKZAAC3hQAAGNpYWVogAAAAAAAAJKAAAA+EAAC2z1hZWiAAAAAAAAD21gABAAAAANMtcGFyYQAAAAAABAAAAAJmZgAA8qcAAA1ZAAAT0AAAClsAAAAAAAAAAG1sdWMAAAAAAAAAAQAAAAxlblVTAAAAIAAAABwARwBvAG8AZwBsAGUAIABJAG4AYwAuACAAMgAwADEANv/AABEIAEAAQAMBIgACEQEDEQH/xAAfAAABBQEBAQEBAQAAAAAAAAAAAQIDBAUGBwgJCgv/xAC1EAACAQMDAgQDBQUEBAAAAX0BAgMABBEFEiExQQYTUWEHInEUMoGRoQgjQrHBFVLR8CQzYnKCCQoWFxgZGiUmJygpKjQ1Njc4OTpDREVGR0hJSlNUVVZXWFlaY2RlZmdoaWpzdHV2d3h5eoOEhYaHiImKkpOUlZaXmJmaoqOkpaanqKmqsrO0tba3uLm6wsPExcbHyMnK0tPU1dbX2Nna4eLj5OXm5+jp6vHy8/T19vf4+fr/xAAfAQADAQEBAQEBAQEBAAAAAAAAAQIDBAUGBwgJCgv/xAC1EQACAQIEBAMEBwUEBAABAncAAQIDEQQFITEGEkFRB2FxEyIygQgUQpGhscEJIzNS8BVictEKFiQ04SXxFxgZGiYnKCkqNTY3ODk6Q0RFRkdISUpTVFVWV1hZWmNkZWZnaGlqc3R1dnd4eXqCg4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY2dri4+Tl5ufo6ery8/T19vf4+fr/2wBDAA8PDw8PDxoPDxolGhoaJTIlJSUlMj8yMjIyMj9MPz8/Pz8/TExMTExMTExbW1tbW1tqampqand3d3d3d3d3d3f/2wBDARITEx4cHjQcHDR8VEVUfHx8fHx8fHx8fHx8fHx8fHx8fHx8fHx8fHx8fHx8fHx8fHx8fHx8fHx8fHx8fHx8fHz/3QAEAAT/2gAMAwEAAhEDEQA/APQ6azKg3McCnEgDJrInlMrZ7DpVRjcaVyd77nEY49TVd9QkVsEceopbbyyTvAOcY/n+tJLB5R3DkEn8M81dlew9NiSO+kZvmUY7Vb+0pv8ALOc4z+FZyBMBjzg9KY0kizFlGQBwvtScNdAcexuAgjI5pazbO5MrYZdjHO5fQ1pVm0Sf/9DuLxiIto4yefpWY2PudzUt3cbn6Hav6/1qozZD...">
<script src="https://cdn.jsdelivr.net/npm/@tailwindcss/browser@4"></script>
<script src="https://cdn.jsdelivr.net/npm/lucide"></script>
<style>${styles}</style>
</head>
<body data-state="login" class="bg-black text-white min-h-screen overflow-hidden">

<!-- Login Page -->
<div id="login-page" class="fixed inset-0 z-50 flex items-center justify-center bg-black">
	<div class="login-enter text-center p-10 bg-white/[0.03] border border-white/10 rounded-2xl max-w-sm w-[90%] shadow-2xl shadow-black/50">
		<div class="text-5xl mb-4">♆</div>
		<h2 class="text-xl font-bold mb-1">Neptune Admin</h2>
		<p class="text-sm text-neutral-500 mb-7">请使用 Telegram 账号登录</p>
		<div id="tg-widget">
			<script async src="https://telegram.org/js/telegram-widget.js?22" data-telegram-login="${safe}" data-size="large" data-request-access="write" data-onauth="onTelegramAuth"></script>
		</div>
		<p id="login-hint" class="text-xs text-neutral-600 mt-4">需要管理员权限才能登录</p>
		<div id="login-loading" class="hidden mt-4 flex items-center justify-center gap-2 text-sm text-neutral-400">
			<div class="spinner"></div>正在登录...
		</div>
		<div id="login-error" class="hidden mt-4 text-sm text-red-400"></div>
	</div>
</div>

<!-- Sidebar Backdrop (mobile) -->
<div id="sidebar-backdrop" class="hidden fixed inset-0 z-30 bg-black/60 backdrop-blur-sm lg:hidden" onclick="closeSidebar()"></div>

<!-- Sidebar -->
<nav id="sidebar" class="fixed top-3 left-3 bottom-3 z-40 w-60 bg-neutral-950 border border-white/10 rounded-2xl flex flex-col -translate-x-full lg:translate-x-0 transition-transform duration-200 shadow-xl shadow-black/40">
	<div class="px-5 py-5 border-b border-white/10">
		<h1 class="text-base font-bold tracking-tight flex items-center gap-2">
			<span class="text-lg">♆</span> Neptune Admin
		</h1>
	</div>
	<div id="sidebar-nav" class="flex-1 p-3 flex flex-col gap-0.5 overflow-y-auto"></div>
	<div id="sidebar-footer" class="hidden px-5 py-4 border-t border-white/10 items-center gap-3">
		<div id="user-avatar" class="w-8 h-8 rounded-full bg-white text-black flex items-center justify-center text-sm font-bold">?</div>
		<span id="user-name" class="text-sm text-neutral-400">-</span>
	</div>
</nav>

<!-- Main Area -->
<div id="main-wrapper" class="lg:ml-[252px] flex flex-col h-screen">
	<!-- Topbar -->
	<div class="px-3 lg:px-5 pt-3 pb-2">
		<div class="island bg-neutral-950 border border-white/10 rounded-2xl flex items-center justify-between px-4 lg:px-5 py-3">
			<div class="flex items-center gap-3">
				<button onclick="toggleSidebar()" class="lg:hidden p-1.5 -ml-1.5 rounded-lg hover:bg-white/5 transition-colors">
					<i data-lucide="menu" class="w-5 h-5"></i>
				</button>
				<h2 id="topbar-title" class="text-base font-bold tracking-tight flex items-center gap-2"></h2>
			</div>
		</div>
	</div>
	<!-- Content -->
	<main id="main-area" class="flex-1 overflow-y-auto px-3 lg:px-5 pb-5"></main>
</div>

<script>${scripts}</script>
</body>
</html>`;
}
