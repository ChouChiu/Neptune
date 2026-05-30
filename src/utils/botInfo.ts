import type { Api } from "grammy";

let cachedUsername: string | null = null;
let inflightPromise: Promise<string> | null = null;

export async function getBotUsername(api: Api): Promise<string> {
	if (cachedUsername) return cachedUsername;
	if (inflightPromise) return inflightPromise;

	inflightPromise = api
		.getMe()
		.then((me) => {
			cachedUsername = me.username ?? "";
			inflightPromise = null;
			return cachedUsername;
		})
		.catch((err) => {
			inflightPromise = null;
			throw err;
		});

	return inflightPromise;
}
