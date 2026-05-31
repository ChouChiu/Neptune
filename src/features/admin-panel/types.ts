import type { Env } from "../../types";

export interface AdminPanelModule {
	id: string;
	label: string;
	icon: string;
	apiPrefix: string;
	registerRoutes(
		routes: Map<string, (req: Request, env: Env) => Promise<Response>>,
		getEnv: () => Env,
	): void;
}
