const SEND_INTERVAL_MS = 1000;
const MAX_SEND_QUEUE_SIZE = 10;

type SendResult = "sent" | "failed" | "dropped";

interface QueuedSend {
	run: () => Promise<void>;
	resolve: (result: SendResult) => void;
}

const sendQueue: QueuedSend[] = [];
let isProcessing = false;
let lastSendAt = 0;

function sleep(ms: number): Promise<void> {
	return new Promise((resolve) => setTimeout(resolve, ms));
}

function queueSize(): number {
	return sendQueue.length + (isProcessing ? 1 : 0);
}

async function processSendQueue(): Promise<void> {
	if (isProcessing) return;
	isProcessing = true;

	try {
		while (sendQueue.length > 0) {
			const item = sendQueue.shift();
			if (!item) continue;

			const delay = Math.max(0, lastSendAt + SEND_INTERVAL_MS - Date.now());
			if (delay > 0) await sleep(delay);

			try {
				await item.run();
				item.resolve("sent");
			} catch (error) {
				console.error("Telegram send queue task failed:", error);
				item.resolve("failed");
			} finally {
				lastSendAt = Date.now();
			}
		}
	} finally {
		isProcessing = false;
		if (sendQueue.length > 0) void processSendQueue();
	}
}

export function enqueueTelegramSend(
	run: () => Promise<void>,
): Promise<SendResult> {
	if (queueSize() >= MAX_SEND_QUEUE_SIZE) {
		console.warn("Telegram send queue full, dropping message", {
			maxSize: MAX_SEND_QUEUE_SIZE,
			currentSize: queueSize(),
		});
		return Promise.resolve("dropped");
	}

	const result = new Promise<SendResult>((resolve) => {
		sendQueue.push({ run, resolve });
	});
	void processSendQueue();
	return result;
}
