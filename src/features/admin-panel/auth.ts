function timingSafeEqual(a: string, b: string): boolean {
	if (a.length !== b.length) return false;
	let result = 0;
	for (let i = 0; i < a.length; i++) {
		result |= a.charCodeAt(i) ^ b.charCodeAt(i);
	}
	return result === 0;
}

export async function verifyTelegramAuth(
	botToken: string,
	data: Record<string, string>,
): Promise<boolean> {
	const hash = data.hash;
	if (!hash) return false;

	const checkData: Record<string, string> = {};
	for (const [key, value] of Object.entries(data)) {
		if (key !== "hash") checkData[key] = value;
	}

	const sortedEntries = Object.entries(checkData).sort(([a], [b]) =>
		a.localeCompare(b),
	);
	const dataCheckString = sortedEntries
		.map(([key, value]) => `${key}=${value}`)
		.join("\n");

	const encoder = new TextEncoder();

	// Telegram Login Widget: secret_key = SHA256(bot_token)
	const secretHash = await crypto.subtle.digest(
		"SHA-256",
		encoder.encode(botToken),
	);

	const key = await crypto.subtle.importKey(
		"raw",
		secretHash,
		{ name: "HMAC", hash: "SHA-256" },
		false,
		["sign"],
	);
	const signature = await crypto.subtle.sign(
		"HMAC",
		key,
		encoder.encode(dataCheckString),
	);
	const computedHash = Array.from(new Uint8Array(signature))
		.map((b) => b.toString(16).padStart(2, "0"))
		.join("");

	return timingSafeEqual(computedHash, hash);
}

export async function signSession(
	botToken: string,
	userId: number,
	expiresAt: number,
): Promise<string> {
	const payload = `${userId}:${expiresAt}`;
	const encoder = new TextEncoder();
	const key = await crypto.subtle.importKey(
		"raw",
		encoder.encode(botToken),
		{ name: "HMAC", hash: "SHA-256" },
		false,
		["sign"],
	);
	const signature = await crypto.subtle.sign(
		"HMAC",
		key,
		encoder.encode(payload),
	);
	const sig = Array.from(new Uint8Array(signature))
		.map((b) => b.toString(16).padStart(2, "0"))
		.join("");
	return `${payload}:${sig}`;
}

export async function verifySession(
	botToken: string,
	cookie: string,
): Promise<number | null> {
	const parts = cookie.split(":");
	if (parts.length !== 3) return null;

	const userId = Number(parts[0]);
	const expiresAt = Number(parts[1]);
	if (Number.isNaN(userId) || Number.isNaN(expiresAt)) return null;

	if (Math.floor(Date.now() / 1000) > expiresAt) return null;

	const payload = `${parts[0]}:${parts[1]}`;
	const encoder = new TextEncoder();
	const key = await crypto.subtle.importKey(
		"raw",
		encoder.encode(botToken),
		{ name: "HMAC", hash: "SHA-256" },
		false,
		["sign"],
	);
	const signature = await crypto.subtle.sign(
		"HMAC",
		key,
		encoder.encode(payload),
	);
	const sig = Array.from(new Uint8Array(signature))
		.map((b) => b.toString(16).padStart(2, "0"))
		.join("");

	if (!timingSafeEqual(sig, parts[2] ?? "")) return null;
	return userId;
}
