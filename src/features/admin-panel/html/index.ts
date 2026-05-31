import { renderLayout } from "./layout";
import { renderScripts } from "./scripts";
import { renderStyles } from "./styles";

export function renderAdminHtml(botUsername: string): string {
	return renderLayout(botUsername, renderStyles(), renderScripts(botUsername));
}
