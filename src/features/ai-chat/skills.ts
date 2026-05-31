import skillsData from "./skills.json";

interface SkillContent {
	[key: string]: unknown;
}

export interface Skill {
	name: string;
	triggers: string[];
	content: SkillContent;
}

interface SkillsJson {
	default: Skill;
	dynamic: Skill[];
}

const data = skillsData as SkillsJson;

export const DEFAULT_SKILL: Skill = data.default;
export const SKILLS: Skill[] = data.dynamic;

function formatValue(value: unknown, indent = 0): string {
	const prefix = "  ".repeat(indent);

	if (typeof value === "string") return value;
	if (typeof value === "number" || typeof value === "boolean")
		return String(value);

	if (Array.isArray(value)) {
		if (value.length === 0) return "";
		if (typeof value[0] === "string") return value.join("、");
		return value
			.map((v) => `${prefix}- ${formatValue(v, indent + 1)}`)
			.join("\n");
	}

	if (typeof value === "object" && value !== null) {
		const lines: string[] = [];
		for (const [k, v] of Object.entries(value)) {
			const formatted = formatValue(v, indent + 1);
			if (!formatted) continue;
			if (Array.isArray(v) && typeof v[0] === "object") {
				lines.push(`${prefix}${k}:\n${formatted}`);
			} else if (typeof v === "object" && !Array.isArray(v)) {
				lines.push(`${prefix}${k}:\n${formatted}`);
			} else {
				lines.push(`${prefix}${k}: ${formatted}`);
			}
		}
		return lines.join("\n");
	}

	return "";
}

export function skillToText(skill: Skill): string {
	return formatValue(skill.content);
}

export function matchSkills(message: string): Skill[] {
	const lowerMessage = message.toLowerCase();
	const matched: Skill[] = [];

	for (const skill of SKILLS) {
		for (const trigger of skill.triggers) {
			if (lowerMessage.includes(trigger.toLowerCase())) {
				matched.push(skill);
				break;
			}
		}
	}

	return matched;
}
