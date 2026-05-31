const MDV2_SPECIAL = /[_[\]()~`>#+=|{}.!\\]/g;

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
