import {
	addWarning,
	getReport,
	getReports,
	updateReportStatus,
} from "../../../shared/db/queries";
import type { Env } from "../../../types";
import { authenticate, unauthorized } from "../auth-helpers";
import type { AdminPanelModule } from "../types";

export const reportsModule: AdminPanelModule = {
	id: "reports",
	label: "举报管理",
	icon: "✉️",
	apiPrefix: "/admin/api/reports",
	registerRoutes(routes) {
		routes.set("/admin/api/reports", async (req, env) => {
			const userId = await authenticate(req, env);
			if (!userId) return unauthorized();

			const url = new URL(req.url);
			const status = url.searchParams.get("status") ?? undefined;
			const reports = await getReports(env.db, status);
			return Response.json({ reports });
		});

		routes.set("/admin/api/reports/:id/resolve", async (req, env) => {
			const userId = await authenticate(req, env);
			if (!userId) return unauthorized();

			const url = new URL(req.url);
			const pathParts = url.pathname.split("/");
			const reportId = Number(pathParts[4]);
			if (Number.isNaN(reportId)) {
				return Response.json({ error: "Invalid report ID" }, { status: 400 });
			}

			const body = (await req.json()) as { status?: string };
			if (body.status !== "approved" && body.status !== "dismissed") {
				return Response.json(
					{ error: "Status must be 'approved' or 'dismissed'" },
					{ status: 400 },
				);
			}

			const report = await getReport(env.db, reportId);
			if (!report) {
				return Response.json({ error: "Report not found" }, { status: 404 });
			}

			await updateReportStatus(env.db, reportId, body.status, userId);

			if (body.status === "approved") {
				await handleApproved(env, report, userId);
			} else {
				await handleDismissed(env, report);
			}

			return Response.json({ ok: true });
		});
	},
};

async function handleApproved(
	env: Env,
	report: {
		group_id: number;
		reporter_id: number;
		reported_user_id: number;
		reported_message_id: number | null;
		reported_message_text: string;
		content: string;
	},
	adminId: number,
): Promise<void> {
	if (report.reported_message_id) {
		await tgApi(env, "deleteMessage", {
			chat_id: report.group_id,
			message_id: report.reported_message_id,
		}).catch(() => {});
	}

	await addWarning(
		env.db,
		report.group_id,
		report.reported_user_id,
		adminId,
		`举报通过: ${report.content}`,
	);

	const preview = report.reported_message_text
		? `\n📄 被举报内容：「${truncate(report.reported_message_text, 100)}」`
		: "";

	await tgApi(env, "sendMessage", {
		chat_id: report.group_id,
		text:
			`⚠️ 用户 [${report.reported_user_id}](tg://user?id=${report.reported_user_id}) 被举报处理\n\n` +
			`📝 举报原因：${report.content}` +
			preview +
			`\n\n🚫 违规消息已删除，用户已被警告。请遵守群规。`,
		parse_mode: "Markdown",
	}).catch(() => {});

	await tgApi(env, "sendMessage", {
		chat_id: report.reporter_id,
		text:
			`✅ 你的举报已通过处理\n\n` +
			`📋 举报编号：#${report.reported_user_id}\n` +
			`📝 举报原因：${report.content}\n` +
			`🏠 群组：${report.group_id}\n\n` +
			`违规消息已删除，用户已被警告。感谢你维护群组秩序！`,
	}).catch(() => {});
}

async function handleDismissed(
	env: Env,
	report: {
		reporter_id: number;
		group_id: number;
		reported_user_id: number;
		content: string;
	},
): Promise<void> {
	await tgApi(env, "sendMessage", {
		chat_id: report.reporter_id,
		text:
			`❌ 你的举报未通过审核\n\n` +
			`📋 举报编号：#${report.reported_user_id}\n` +
			`📝 举报原因：${report.content}\n` +
			`🏠 群组：${report.group_id}\n\n` +
			`经管理员审核，该举报不符合处理条件。如有疑问请联系群管理员。`,
	}).catch(() => {});
}

async function tgApi(
	env: Env,
	method: string,
	body: Record<string, unknown>,
): Promise<unknown> {
	const res = await fetch(
		`https://api.telegram.org/bot${env.BOT_TOKEN}/${method}`,
		{
			method: "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify(body),
		},
	);
	if (!res.ok) {
		throw new Error(`Telegram API ${method} failed: ${res.status}`);
	}
	return res.json();
}

function truncate(s: string, max: number): string {
	if (s.length <= max) return s;
	return `${s.slice(0, max)}…`;
}
