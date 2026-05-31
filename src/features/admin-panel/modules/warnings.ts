import { getAllWarnings } from "../../../shared/db/queries";
import type { Env } from "../../../types";
import { verifySession } from "../auth";
import type { AdminPanelModule } from "../types";

export const warningsModule: AdminPanelModule = {
	id: "warnings",
	label: "警告记录",
	icon: "⚠️",
	apiPrefix: "/admin/api/warnings",
	registerRoutes(routes, getEnv) {
		routes.set("/admin/api/warnings", async (req) => {
			const env = getEnv();
			const userId = await authenticate(req, env);
			if (!userId) return unauthorized();

			const warnings = await getAllWarnings(env.db);
			return Response.json({ warnings });
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
