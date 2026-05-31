import type { Bot } from "grammy";
import { registerVerifyCommands } from "./commands";
import { registerVerifyHandlers } from "./handlers";

export { handleCaptchaReply } from "./captcha-handler";
export { restrictUser } from "./handlers";

export function registerVerifyFeature(
	bot: Bot,
	db: D1Database,
	bucket: R2Bucket,
	reuseCaptcha: boolean,
): void {
	registerVerifyCommands(bot, db);
	registerVerifyHandlers(bot, db, bucket, reuseCaptcha);
}
