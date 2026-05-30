export interface CaptchaResult {
	text: string;
	bmp: Uint8Array;
}

const CHARS = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789";

const CAPTCHA_CACHE_KEY = "captcha/cache/meta.json";
const MAX_REUSE_COUNT = 10;

function randomInt(min: number, max: number): number {
	return Math.floor(Math.random() * (max - min + 1)) + min;
}

function randomChar(): string {
	return CHARS[Math.floor(Math.random() * CHARS.length)] ?? "A";
}

// 5x7 点阵字体（每个字符 5 列 x 7 行）
const FONT: Record<string, number[]> = {
	A: [0x7c, 0x12, 0x11, 0x12, 0x7c],
	B: [0x7f, 0x49, 0x49, 0x49, 0x36],
	C: [0x3e, 0x41, 0x41, 0x41, 0x22],
	D: [0x7f, 0x41, 0x41, 0x41, 0x3e],
	E: [0x7f, 0x49, 0x49, 0x49, 0x41],
	F: [0x7f, 0x09, 0x09, 0x09, 0x01],
	G: [0x3e, 0x41, 0x49, 0x49, 0x7a],
	H: [0x7f, 0x08, 0x08, 0x08, 0x7f],
	J: [0x20, 0x40, 0x41, 0x3f, 0x01],
	K: [0x7f, 0x08, 0x14, 0x22, 0x41],
	L: [0x7f, 0x40, 0x40, 0x40, 0x40],
	M: [0x7f, 0x02, 0x0c, 0x02, 0x7f],
	N: [0x7f, 0x04, 0x08, 0x10, 0x7f],
	P: [0x7f, 0x09, 0x09, 0x09, 0x06],
	Q: [0x3e, 0x41, 0x51, 0x21, 0x5e],
	R: [0x7f, 0x09, 0x19, 0x29, 0x46],
	S: [0x46, 0x49, 0x49, 0x49, 0x31],
	T: [0x01, 0x01, 0x7f, 0x01, 0x01],
	U: [0x3f, 0x40, 0x40, 0x40, 0x3f],
	V: [0x1f, 0x20, 0x40, 0x20, 0x1f],
	W: [0x3f, 0x40, 0x38, 0x40, 0x3f],
	X: [0x63, 0x14, 0x08, 0x14, 0x63],
	Y: [0x07, 0x08, 0x70, 0x08, 0x07],
	Z: [0x61, 0x51, 0x49, 0x45, 0x43],
	"2": [0x62, 0x51, 0x49, 0x45, 0x43],
	"3": [0x22, 0x41, 0x49, 0x49, 0x36],
	"4": [0x0c, 0x0a, 0x09, 0x7f, 0x08],
	"5": [0x4f, 0x49, 0x49, 0x49, 0x31],
	"6": [0x3e, 0x49, 0x49, 0x49, 0x30],
	"7": [0x01, 0x71, 0x09, 0x05, 0x03],
	"8": [0x36, 0x49, 0x49, 0x49, 0x36],
	"9": [0x06, 0x49, 0x49, 0x49, 0x3e],
};

function createBmp(
	width: number,
	height: number,
	pixels: Uint8Array,
): Uint8Array {
	const rowSize = Math.ceil((width * 3) / 4) * 4;
	const imageSize = rowSize * height;
	const fileSize = 54 + imageSize;

	const buffer = new Uint8Array(fileSize);
	const view = new DataView(buffer.buffer);

	// File header
	buffer[0] = 0x42; // B
	buffer[1] = 0x4d; // M
	view.setUint32(2, fileSize, true);
	view.setUint32(10, 54, true);

	// Info header
	view.setUint32(14, 40, true);
	view.setInt32(18, width, true);
	view.setInt32(22, height, true);
	view.setUint16(26, 1, true);
	view.setUint16(28, 24, true);
	view.setUint32(34, imageSize, true);

	// Pixel data (bottom-up)
	for (let y = 0; y < height; y++) {
		for (let x = 0; x < width; x++) {
			const srcIdx = ((height - 1 - y) * width + x) * 3;
			const dstIdx = 54 + y * rowSize + x * 3;
			buffer[dstIdx] = pixels[srcIdx] ?? 200; // B
			buffer[dstIdx + 1] = pixels[srcIdx + 1] ?? 200; // G
			buffer[dstIdx + 2] = pixels[srcIdx + 2] ?? 200; // R
		}
	}

	return buffer;
}

export async function generateCaptcha(
	bucket: R2Bucket,
	size = 5,
	reuse = false,
): Promise<CaptchaResult> {
	if (reuse) {
		const cached = await bucket.get(CAPTCHA_CACHE_KEY);
		if (cached) {
			const meta = JSON.parse(await cached.text()) as {
				text: string;
				useCount: number;
			};
			if (meta.useCount < MAX_REUSE_COUNT) {
				const bmpObj = await bucket.get("captcha/cache/captcha.bmp");
				if (bmpObj) {
					meta.useCount++;
					await bucket.put(
						CAPTCHA_CACHE_KEY,
						JSON.stringify({
							...meta,
							useCount: meta.useCount,
						}),
					);
					return {
						text: meta.text,
						bmp: new Uint8Array(await bmpObj.arrayBuffer()),
					};
				}
			}
		}
	}

	let text = "";
	for (let i = 0; i < size; i++) {
		text += randomChar();
	}

	const scale = 4;
	const charW = 5 * scale;
	const charH = 7 * scale;
	const padding = 10;
	const width = size * charW + (size - 1) * padding + padding * 2;
	const height = charH + padding * 2;

	const pixels = new Uint8Array(width * height * 3);

	// 背景色
	for (let i = 0; i < pixels.length; i += 3) {
		pixels[i] = 220; // B
		pixels[i + 1] = 220; // G
		pixels[i + 2] = 220; // R
	}

	// 干扰线
	for (let l = 0; l < 6; l++) {
		const x1 = randomInt(0, width - 1);
		const y1 = randomInt(0, height - 1);
		const x2 = randomInt(0, width - 1);
		const y2 = randomInt(0, height - 1);
		const steps = Math.max(Math.abs(x2 - x1), Math.abs(y2 - y1));
		for (let s = 0; s <= steps; s++) {
			const x = Math.round(x1 + ((x2 - x1) * s) / steps);
			const y = Math.round(y1 + ((y2 - y1) * s) / steps);
			if (x >= 0 && x < width && y >= 0 && y < height) {
				const idx = (y * width + x) * 3;
				pixels[idx] = randomInt(50, 150);
				pixels[idx + 1] = randomInt(50, 150);
				pixels[idx + 2] = randomInt(50, 150);
			}
		}
	}

	// 绘制文字
	for (let i = 0; i < size; i++) {
		const ch = text[i] ?? "A";
		const glyph = FONT[ch] ?? FONT.A ?? [0x7f, 0x7f, 0x7f, 0x7f, 0x7f];
		const offsetX = padding + i * (charW + padding);
		const offsetY = padding;

		const cr = randomInt(20, 100);
		const cg = randomInt(20, 100);
		const cb = randomInt(20, 100);

		for (let gy = 0; gy < 7; gy++) {
			for (let gx = 0; gx < 5; gx++) {
				if ((glyph[gx] ?? 0) & (1 << gy)) {
					for (let sy = 0; sy < scale; sy++) {
						for (let sx = 0; sx < scale; sx++) {
							const px = offsetX + gx * scale + sx;
							const py = offsetY + gy * scale + sy;
							if (px < width && py < height) {
								const idx = (py * width + px) * 3;
								pixels[idx] = cb;
								pixels[idx + 1] = cg;
								pixels[idx + 2] = cr;
							}
						}
					}
				}
			}
		}
	}

	// 干扰点
	for (let i = 0; i < 40; i++) {
		const x = randomInt(0, width - 1);
		const y = randomInt(0, height - 1);
		const idx = (y * width + x) * 3;
		pixels[idx] = randomInt(0, 200);
		pixels[idx + 1] = randomInt(0, 200);
		pixels[idx + 2] = randomInt(0, 200);
	}

	const bmp = createBmp(width, height, pixels);

	const result = { text, bmp };
	if (reuse) {
		await bucket.put(
			CAPTCHA_CACHE_KEY,
			JSON.stringify({
				text,
				useCount: 1,
			}),
		);
		await bucket.put("captcha/cache/captcha.bmp", bmp, {
			httpMetadata: { contentType: "image/bmp" },
		});
	}
	return result;
}

export async function uploadCaptchaToR2(
	bucket: R2Bucket,
	key: string,
	data: Uint8Array,
): Promise<string> {
	await bucket.put(key, data, {
		httpMetadata: { contentType: "image/bmp" },
	});
	return key;
}

export async function getCaptchaFromR2(
	bucket: R2Bucket,
	key: string,
): Promise<Uint8Array | null> {
	const obj = await bucket.get(key);
	if (!obj) return null;
	return await obj.arrayBuffer().then((buf) => new Uint8Array(buf));
}
