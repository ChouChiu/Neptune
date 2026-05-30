export function replacePlaceholders(
	text: string,
	vars: {
		nickname?: string;
		userid?: number | string;
		groupname?: string;
	},
): string {
	let result = text;
	if (vars.nickname) {
		result = result.replace(/\{nickname\}/g, vars.nickname);
	}
	if (vars.userid !== undefined) {
		result = result.replace(/\{userid\}/g, String(vars.userid));
	}
	if (vars.groupname) {
		result = result.replace(/\{groupname\}/g, vars.groupname);
	}
	return result;
}
