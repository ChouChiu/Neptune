import { webhookCallback } from "grammy";
import { createBot } from "./bot";
import { handleAdminRoutes } from "./features/admin-panel";
import {
	formatReleaseMessage,
	sendToTelegram,
	verifySignature,
} from "./features/github-release";
import type { Env } from "./types";

export default {
	async fetch(request: Request, env: Env): Promise<Response> {
		const url = new URL(request.url);

		if (url.pathname === "/webhook" && request.method === "POST") {
			try {
				const bot = createBot(env);
				const handleUpdate = webhookCallback(bot, "std/http");
				return await handleUpdate(request);
			} catch (error) {
				const errMsg =
					error instanceof Error
						? `${error.message}\n${error.stack}`
						: String(error);
				console.error("Webhook error:", errMsg);
				// 返回 200 避免 Telegram 重试
				return new Response("OK");
			}
		}

		if (url.pathname === "/set-webhook" && request.method === "GET") {
			const token = url.searchParams.get("token");
			if (!env.GITHUB_WEBHOOK_SECRET || token !== env.GITHUB_WEBHOOK_SECRET) {
				return new Response("Unauthorized", { status: 401 });
			}

			const bot = createBot(env);
			const webhookUrl = `${url.origin}/webhook`;
			const result = await bot.api.setWebhook(webhookUrl, {
				allowed_updates: ["message", "chat_member", "callback_query"],
			});
			await bot.api.setMyCommands([
				{ command: "help", description: "显示帮助信息" },
				{ command: "ping", description: "检查机器人是否在线" },
				{ command: "id", description: "获取当前群组 ID" },
				{ command: "connect", description: "绑定私聊与群组" },
				{ command: "switch", description: "切换管理的群组（私聊）" },
				{ command: "setwelcome", description: "设置欢迎消息" },
				{ command: "enablewelcome", description: "启用入群欢迎" },
				{ command: "disablewelcome", description: "禁用入群欢迎" },
				{ command: "setverifybutton", description: "设置认证按钮文案" },
				{ command: "setverifytimeout", description: "设置认证超时时间" },
				{ command: "testverify", description: "测试验证消息" },
				{ command: "rule", description: "设置群规（入群需阅读）" },
				{ command: "addkeyword", description: "添加关键词规则" },
				{ command: "addregex", description: "添加正则规则" },
				{ command: "listkeywords", description: "列出所有规则" },
				{ command: "removekeyword", description: "删除规则" },
				{ command: "enablevotekick", description: "启用投票踢人" },
				{ command: "disablevotekick", description: "禁用投票踢人" },
				{ command: "kick", description: "发起踢人投票（回复目标消息）" },
				{ command: "warn", description: "警告用户（回复目标消息）" },
				{ command: "report", description: "举报用户（回复目标消息，附原因）" },
			]);
			return new Response(JSON.stringify(result), {
				headers: { "Content-Type": "application/json" },
			});
		}

		if (url.pathname === "/test" && request.method === "GET") {
			const authHeader = request.headers.get("Authorization");
			if (!authHeader || authHeader !== `Bearer ${env.GITHUB_WEBHOOK_SECRET}`) {
				return new Response("Unauthorized", { status: 401 });
			}

			try {
				const bot = createBot(env);
				const me = await bot.api.getMe();

				let dbStatus = "ok";
				try {
					await env.db.prepare("SELECT 1").first();
				} catch (e) {
					dbStatus = `error: ${e}`;
				}

				return new Response(
					JSON.stringify({
						bot: me,
						db: dbStatus,
					}),
					{ headers: { "Content-Type": "application/json" } },
				);
			} catch (error) {
				return new Response(`Error: ${error}`, { status: 500 });
			}
		}

		if (url.pathname === "/github-webhook" && request.method === "POST") {
			try {
				const ghEvent = request.headers.get("X-GitHub-Event");

				if (ghEvent === "ping") {
					return new Response("pong", { status: 200 });
				}

				if (ghEvent !== "release") {
					return new Response("ignored", { status: 200 });
				}

				const body = await request.text();

				const sigValid = await verifySignature(
					body,
					request.headers.get("X-Hub-Signature-256"),
					env.GITHUB_WEBHOOK_SECRET,
				);
				if (!sigValid) {
					console.error("[Release] Invalid signature");
					return new Response("Unauthorized", { status: 401 });
				}

				const payload = JSON.parse(body);
				if (payload.action !== "published") {
					return new Response("ignored", { status: 200 });
				}

				const text = formatReleaseMessage(payload);
				await sendToTelegram(env.BOT_TOKEN, env.RELEASE_CHANNEL_ID, text);

				return new Response("ok", { status: 200 });
			} catch (error) {
				const errMsg =
					error instanceof Error
						? `${error.message}\n${error.stack}`
						: String(error);
				console.error("[Release] Handler error:", errMsg);
				return new Response("Internal Error", { status: 500 });
			}
		}

		// ── Admin panel ───────────────────────────────────────
		const adminResponse = await handleAdminRoutes(request, env);
		if (adminResponse) return adminResponse;

		if (url.pathname === "/") {
			return Response.redirect(`${url.origin}/admin`, 302);
		}

		return new Response("Neptune is running!");
	},
};
