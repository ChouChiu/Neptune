import { renderApiScript } from "./api";
import { renderLoginScript } from "./login";
import { renderReportsScript } from "./reports";
import { renderSidebarScript } from "./sidebar";
import { renderWarningsScript } from "./warnings";

export function renderScripts(_botUsername: string): string {
	return `
'use strict';

${renderApiScript()}

${renderLoginScript()}

${renderSidebarScript()}

${renderReportsScript()}

${renderWarningsScript()}

API.get('/admin/auth/me').then(function(res) {
	if (res.user) window.__adminUser = res.user;
	window.__render();
}).catch(function() { window.__render(); });
`;
}
