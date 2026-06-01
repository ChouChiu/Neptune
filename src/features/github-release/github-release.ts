import {
	createMarkdownPlaceholderStore,
	escapeMarkdownCode,
	escapeMd,
	markdownBold,
	markdownLink,
	trimCodeFencePadding,
} from "../../shared/utils/markdown";

const TELEGRAM_API = "https://api.telegram.org";
const MAX_MESSAGE_LENGTH = 4096;
const MAX_RETRIES = 3;
const RETRY_DELAY_MS = 1000;

interface GitHubRelease {
	action: string;
	release: {
		tag_name: string;
		name: string | null;
		body: string | null;
		html_url: string;
		prerelease: boolean;
		draft: boolean;
	};
}

export async function verifySignature(
	body: string,
	signatureHeader: string | null,
	secret: string,
): Promise<boolean> {
	if (!signatureHeader) return false;

	const key = await crypto.subtle.importKey(
		"raw",
		new TextEncoder().encode(secret),
		{ name: "HMAC", hash: "SHA-256" },
		false,
		["sign"],
	);

	const sig = await crypto.subtle.sign(
		"HMAC",
		key,
		new TextEncoder().encode(body),
	);

	const computed = `sha256=${Array.from(new Uint8Array(sig))
		.map((b) => b.toString(16).padStart(2, "0"))
		.join("")}`;

	if (computed.length !== signatureHeader.length) return false;

	let result = 0;
	for (let i = 0; i < computed.length; i++) {
		result |= computed.charCodeAt(i) ^ signatureHeader.charCodeAt(i);
	}
	return result === 0;
}

export function convertGfmToMarkdownV2(gfm: string): string {
	const store = createMarkdownPlaceholderStore();

	// strip GitHub callout markers: [!NOTE], [!WARNING], etc.
	// normalize \r\n → \n first (GitHub payloads use CRLF)
	// process line by line to avoid \s eating newlines across lines
	let text = gfm.replace(/\r\n/g, "\n");
	text = text
		.split("\n")
		.map((line) =>
			line.replace(/^>?\s*\[!(?:NOTE|WARNING|TIP|IMPORTANT|CAUTION)\]\s*/i, ""),
		)
		.join("\n");

	// fenced code blocks → protect (not escaped)
	text = text.replace(/```(\w*)\n([\s\S]*?)```/g, (_m, _lang, code) => {
		return store.protect(
			`\`\`\n${escapeMarkdownCode(trimCodeFencePadding(code))}\n\`\`\``,
		);
	});

	// inline code → protect (not escaped)
	text = text.replace(/`([^`\n]+?)`/g, (_m, code) => {
		return store.protect(`\`${escapeMarkdownCode(code)}\``);
	});

	// images → remove
	text = text.replace(/!\[[^\]]*\]\([^)]*\)/g, "");

	// links → protect as-is (don't escape inside link syntax)
	text = text.replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_m, linkText, url) => {
		return store.protect(markdownLink(linkText, url));
	});

	// headings → bold
	text = text.replace(/^#{1,6}\s+(.+)$/gm, (_m, h) =>
		store.protect(markdownBold(h)),
	);

	// list markers: -, *, + at line start → ⦁ (avoid * being parsed as bold)
	text = text.replace(/^(\s*)[-*+]\s/gm, "$1⦁ ");

	// bold **text** → *text* (MarkdownV2 uses single *)
	text = text.replace(/\*\*(.+?)\*\*/g, (_m, inner) =>
		store.protect(markdownBold(inner)),
	);

	// blockquotes: "> text" → protect as Telegram ">text" (not escaped)
	text = text.replace(/^>\s?(.*)$/gm, (_m, content: string) => {
		const trimmed = content.trim();
		if (!trimmed) return "";
		return store.protect(`>${escapeMd(trimmed)}`);
	});

	// strip HTML tags
	text = text.replace(/<[^>]+>/g, "");

	return store.restore(escapeMd(text));
}

export function formatReleaseMessage(release: GitHubRelease): string {
	const tagName = escapeMd(release.release.tag_name);
	const isOhos = release.release.name?.toLowerCase().includes("ohos") ?? false;
	const header = isOhos
		? `🎉 Kazumi ${tagName} for OHOS 已发布`
		: `🎉 Kazumi ${tagName} 已发布`;

	const releaseUrl = release.release.html_url;
	const linkLine = markdownLink("🔗 Release 页面", releaseUrl);

	const suffix = `\n\n${linkLine}`;
	const maxBodyLength = MAX_MESSAGE_LENGTH - header.length - suffix.length - 4;

	let body = release.release.body?.trim();
	if (body && body.length > maxBodyLength) {
		body = `${body.slice(0, maxBodyLength)}…`;
	}
	const bodyMd = body ? convertGfmToMarkdownV2(body) : "";

	let message: string;
	if (bodyMd) {
		message = `${header}\n\n${bodyMd}\n\n${linkLine}`;
	} else {
		message = `${header}\n\n${linkLine}`;
	}

	return message;
}

export async function sendToTelegram(
	botToken: string,
	channelId: string,
	text: string,
): Promise<void> {
	const url = `${TELEGRAM_API}/bot${botToken}/sendMessage`;

	for (let attempt = 1; attempt <= MAX_RETRIES; attempt++) {
		console.log(
			`[Release] Sending to channel ${channelId}, attempt ${attempt}/${MAX_RETRIES}`,
		);

		try {
			const resp = await fetch(url, {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({
					chat_id: channelId,
					text,
					parse_mode: "MarkdownV2",
				}),
			});

			if (resp.ok) {
				console.log(`[Release] Sent successfully on attempt ${attempt}`);
				return;
			}

			const errBody = await resp.text();
			console.error(
				`[Release] Attempt ${attempt} failed: HTTP ${resp.status} — ${errBody}`,
			);
		} catch (err) {
			console.error(`[Release] Attempt ${attempt} error: ${err}`);
		}

		if (attempt < MAX_RETRIES) {
			await new Promise((r) => setTimeout(r, RETRY_DELAY_MS));
		}
	}

	console.error(
		`[Release] All ${MAX_RETRIES} attempts failed for channel ${channelId}`,
	);
}
