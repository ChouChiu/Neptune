const MDV2_SPECIAL = /[_*[\]()~`>#+\-=|{}.!\\]/g;

export function escapeMd(text: string): string {
	return text.replace(MDV2_SPECIAL, "\\$&");
}

export function escapeMarkdown(text: string): string {
	return escapeMd(
		text
			.split("\n")
			.map((line) => line.replace(/^[-+*]\s/, "⦁ "))
			.join("\n"),
	);
}

export function escapeMarkdownCode(text: string): string {
	return text.replace(/[`\\]/g, "\\$&");
}

export function escapeMarkdownLinkUrl(url: string): string {
	return url.replace(/[)\\]/g, "\\$&");
}

export function markdownBold(text: string): string {
	return `*${escapeMd(text)}*`;
}

export function markdownItalic(text: string): string {
	return `_${escapeMd(text)}_`;
}

export function markdownLink(text: string, url: string): string {
	return `[${escapeMd(text)}](${escapeMarkdownLinkUrl(url)})`;
}

export function createMarkdownPlaceholderStore() {
	const placeholders: string[] = [];

	return {
		protect(markdown: string): string {
			const token = `\u200BMDV2PH${placeholders.length}\u200B`;
			placeholders.push(markdown);
			return token;
		},
		restore(text: string): string {
			let restored = text;
			for (let i = 0; i < placeholders.length; i++) {
				const placeholder = placeholders[i];
				if (placeholder !== undefined) {
					restored = restored.replaceAll(`\u200BMDV2PH${i}\u200B`, placeholder);
				}
			}
			return restored;
		},
	};
}

export function trimCodeFencePadding(code: string): string {
	return code.endsWith("\n") ? code.slice(0, -1) : code;
}

function formatCodeBlock(language: string, code: string): string {
	const trimmedLanguage = language.trim();
	const safeLanguage = /^[A-Za-z0-9]+$/.test(trimmedLanguage)
		? trimmedLanguage
		: "";
	const openingFence = safeLanguage ? `\`\`\`${safeLanguage}\n` : "```\n";
	return `${openingFence}${escapeMarkdownCode(trimCodeFencePadding(code))}\n\`\`\``;
}

export function formatGeneratedMarkdownV2(text: string): string {
	const store = createMarkdownPlaceholderStore();
	let formatted = text.replace(/\r\n/g, "\n");

	formatted = formatted.replace(
		/```([^\n`]*)\n?([\s\S]*?)```/g,
		(_match, language: string, code: string) =>
			store.protect(formatCodeBlock(language, code)),
	);

	formatted = formatted.replace(/`([^`\n]+?)`/g, (_match, code: string) =>
		store.protect(`\`${escapeMarkdownCode(code)}\``),
	);

	formatted = formatted.replace(
		/\[([^\]\n]+?)\]\(([^)\s]+?)\)/g,
		(_match, linkText: string, url: string) =>
			store.protect(markdownLink(linkText, url)),
	);

	formatted = formatted.replace(/^#{1,6}\s+(.+)$/gm, (_match, title: string) =>
		store.protect(markdownBold(title.trim())),
	);

	formatted = formatted.replace(
		/\*\*([^*\n]+?)\*\*/g,
		(_match, inner: string) => store.protect(markdownBold(inner)),
	);

	formatted = formatted.replace(
		/(^|[\s([{])_([^_\n]+?)_(?=$|[\s)\]},.!?;:])/g,
		(_match, prefix: string, inner: string) =>
			`${prefix}${store.protect(markdownItalic(inner))}`,
	);

	formatted = formatted.replace(
		/(^|[\s([{])\*([^*\n]+?)\*(?=$|[\s)\]},.!?;:])/g,
		(_match, prefix: string, inner: string) =>
			`${prefix}${store.protect(markdownItalic(inner))}`,
	);

	formatted = formatted
		.split("\n")
		.map((line) => line.replace(/^(\s*)[-+*]\s+/, "$1⦁ "))
		.join("\n");

	return store.restore(escapeMd(formatted));
}
