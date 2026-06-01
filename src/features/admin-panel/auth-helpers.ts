import type { Env } from "../../types";
import { verifySession } from "./auth";

export async function authenticate(
	req: Request,
	env: Env,
): Promise<number | null> {
	const cookie = parseCookie(req.headers.get("Cookie") ?? "", "nep_session");
	if (!cookie) return null;
	return verifySession(env.BOT_TOKEN, cookie);
}

export function unauthorized(): Response {
	return Response.json({ error: "Unauthorized" }, { status: 401 });
}

export function parseCookie(header: string, name: string): string | null {
	for (const part of header.split(";")) {
		const [key, ...vals] = part.trim().split("=");
		if (key === name) return vals.join("=");
	}
	return null;
}
