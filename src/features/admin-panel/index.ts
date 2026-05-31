import type { Env } from "../../types";
import { signSession, verifySession, verifyTelegramAuth } from "./auth";
import { renderAdminHtml } from "./html/index";
import { reportsModule } from "./modules/reports";
import { warningsModule } from "./modules/warnings";
import type { AdminPanelModule } from "./types";

const SESSION_TTL = 86400; // 24 hours

const modules: AdminPanelModule[] = [reportsModule, warningsModule];

const apiRoutes = new Map<
	string,
	(req: Request, env: Env) => Promise<Response>
>();

let routesInitialized = false;
let currentEnv: Env;

function initRoutes(): void {
	if (routesInitialized) return;
	const getEnv = () => currentEnv;
	for (const mod of modules) {
		mod.registerRoutes(apiRoutes, getEnv);
	}
	routesInitialized = true;
}

function parseCookies(header: string): Record<string, string> {
	const cookies: Record<string, string> = {};
	for (const part of header.split(";")) {
		const [key, ...vals] = part.trim().split("=");
		if (key) cookies[key] = vals.join("=");
	}
	return cookies;
}

export async function handleAdminRoutes(
	request: Request,
	env: Env,
): Promise<Response | null> {
	const url = new URL(request.url);
	if (!url.pathname.startsWith("/admin")) return null;

	currentEnv = env;
	initRoutes();

	// ── Serve admin HTML ──────────────────────────────────────
	if (url.pathname === "/admin" && request.method === "GET") {
		return new Response(renderAdminHtml(env.BOT_USERNAME), {
			headers: { "Content-Type": "text/html; charset=utf-8" },
		});
	}

	// ── Telegram Login Widget callback ────────────────────────
	if (url.pathname === "/admin/auth/login" && request.method === "POST") {
		try {
			const data = (await request.json()) as Record<string, string>;
			const valid = await verifyTelegramAuth(env.BOT_TOKEN, data);
			if (!valid) {
				return Response.json({ ok: false, error: "Invalid auth data" });
			}

			const userId = Number(data.id);
			if (Number.isNaN(userId)) {
				return Response.json({ ok: false, error: "Invalid user ID" });
			}

			// Check if user is admin of any group
			const adminRow = await env.db
				.prepare("SELECT 1 FROM admin_connections WHERE user_id = ? LIMIT 1")
				.bind(userId)
				.first();
			if (!adminRow) {
				return Response.json({
					ok: false,
					error: "You are not an admin",
				});
			}

			const expiresAt = Math.floor(Date.now() / 1000) + SESSION_TTL;
			const session = await signSession(env.BOT_TOKEN, userId, expiresAt);

			const user = {
				id: userId,
				first_name: data.first_name ?? "",
				last_name: data.last_name ?? "",
				username: data.username ?? "",
			};

			return new Response(JSON.stringify({ ok: true, user }), {
				headers: {
					"Content-Type": "application/json",
					"Set-Cookie": `nep_session=${session}; Path=/; Max-Age=${SESSION_TTL}; SameSite=Lax`,
				},
			});
		} catch {
			return Response.json({ ok: false, error: "Server error" });
		}
	}

	// ── Check current session ─────────────────────────────────
	if (url.pathname === "/admin/auth/me" && request.method === "GET") {
		const cookies = parseCookies(request.headers.get("Cookie") ?? "");
		const session = cookies.nep_session;
		if (!session) return Response.json({ user: null });

		const userId = await verifySession(env.BOT_TOKEN, session);
		if (!userId) {
			return new Response(JSON.stringify({ user: null }), {
				headers: {
					"Set-Cookie": "nep_session=; Path=/; Max-Age=0; SameSite=Lax",
				},
			});
		}

		return Response.json({
			user: { id: userId, first_name: `Admin #${userId}` },
		});
	}

	// ── API routes ────────────────────────────────────────────
	for (const [pattern, handler] of apiRoutes) {
		if (matchRoute(url.pathname, pattern)) {
			return handler(request, env);
		}
	}

	return new Response(
		JSON.stringify({
			error: "Not Found",
			path: url.pathname,
			method: request.method,
			routes: Array.from(apiRoutes.keys()),
		}),
		{ status: 404, headers: { "Content-Type": "application/json" } },
	);
}

function matchRoute(pathname: string, pattern: string): boolean {
	if (pathname === pattern) return true;
	const patternParts = pattern.split("/");
	const pathParts = pathname.split("/");
	if (patternParts.length !== pathParts.length) return false;
	for (let i = 0; i < patternParts.length; i++) {
		const pp = patternParts[i];
		const rp = pathParts[i];
		if (pp === undefined || rp === undefined) return false;
		if (pp.startsWith(":")) continue;
		if (pp !== rp) return false;
	}
	return true;
}
