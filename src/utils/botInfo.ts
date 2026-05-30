import type { Api } from "grammy";

let cachedUsername: string | null = null;

export async function getBotUsername(api: Api): Promise<string> {
	if (cachedUsername) return cachedUsername;
	const me = await api.getMe();
	cachedUsername = me.username ?? "";
	return cachedUsername;
}
