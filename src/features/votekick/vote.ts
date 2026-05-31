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
	const deadline = new Date(expiresAt * 1000);
	const hh = String(deadline.getHours()).padStart(2, "0");
	const mm = String(deadline.getMinutes()).padStart(2, "0");
	const ss = String(deadline.getSeconds()).padStart(2, "0");
	return (
		`🗳️ 投票踢人\n\n` +
		`目标: ${targetName}\n` +
		`发起人: ${initiatorName}\n\n` +
		`赞成: ${yesBar} ${yesCount}/${VOTE_THRESHOLD}\n` +
		`反对: ${noBar} ${noCount}\n\n` +
		`截止时间: ${hh}:${mm}:${ss}`
	);
}
