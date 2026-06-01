import { getAllWarnings } from "../../../shared/db/queries";
import { authenticate, unauthorized } from "../auth-helpers";
import type { AdminPanelModule } from "../types";

export const warningsModule: AdminPanelModule = {
	id: "warnings",
	label: "警告记录",
	icon: "⚠️",
	apiPrefix: "/admin/api/warnings",
	registerRoutes(routes) {
		routes.set("/admin/api/warnings", async (req, env) => {
			const userId = await authenticate(req, env);
			if (!userId) return unauthorized();

			const warnings = await getAllWarnings(env.db, userId);
			return Response.json({ warnings });
		});
	},
};
