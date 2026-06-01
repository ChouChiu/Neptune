import { currentTimestamp } from "../../shared/utils/time";

export const VOTE_THRESHOLD = 5;

export function buildVoteText(
	targetName: string,
	initiatorName: string,
	yesCount: number,
	noCount: number,
	expiresAt: number,
): string {
	const yesBar = "🟢".repeat(Math.min(yesCount, 10));
	const noBar = "🔴".repeat(Math.min(noCount, 10));
	const remaining = Math.max(0, expiresAt - currentTimestamp());
	const minutes = Math.floor(remaining / 60);
	const seconds = remaining % 60;
	const timeStr = minutes > 0 ? `${minutes}分${seconds}秒` : `${seconds}秒`;
	return (
		`🗳️ 投票踢人\n\n` +
		`目标: ${targetName}\n` +
		`发起人: ${initiatorName}\n\n` +
		`赞成: ${yesBar} ${yesCount}/${VOTE_THRESHOLD}\n` +
		`反对: ${noBar} ${noCount}\n\n` +
		`剩余时间: ${timeStr}`
	);
}
