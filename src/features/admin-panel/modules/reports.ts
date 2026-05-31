import { getReports, updateReportStatus } from "../../../shared/db/queries";
import type { Env } from "../../../types";
import { verifySession } from "../auth";
import type { AdminPanelModule } from "../types";

export const reportsModule: AdminPanelModule = {
	id: "reports",
	label: "举报管理",
	icon: "✉️",
	apiPrefix: "/admin/api/reports",
	registerRoutes(routes, getEnv) {
		routes.set("/admin/api/reports", async (req) => {
			const env = getEnv();
			const userId = await authenticate(req, env);
			if (!userId) return unauthorized();

			const url = new URL(req.url);
			const status = url.searchParams.get("status") ?? undefined;
			const reports = await getReports(env.db, status);
			return Response.json({ reports });
		});

		routes.set("/admin/api/reports/:id/resolve", async (req) => {
			const env = getEnv();
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

			await updateReportStatus(env.db, reportId, body.status, userId);
			return Response.json({ ok: true });
		});
	},
};

async function authenticate(req: Request, env: Env): Promise<number | null> {
	const cookie = parseCookie(req.headers.get("Cookie") ?? "", "nep_session");
	if (!cookie) return null;
	return verifySession(env.BOT_TOKEN, cookie);
}

function unauthorized(): Response {
	return Response.json({ error: "Unauthorized" }, { status: 401 });
}

function parseCookie(header: string, name: string): string | null {
	for (const part of header.split(";")) {
		const [key, ...vals] = part.trim().split("=");
		if (key === name) return vals.join("=");
	}
	return null;
}
