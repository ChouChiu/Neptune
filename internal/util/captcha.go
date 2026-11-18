package util

import (
	"encoding/binary"
	"math/rand/v2"
	"strings"
)

// CaptchaResult holds the generated captcha text and BMP image data.
type CaptchaResult struct {
	Text string
	BMP  []byte
}

const captchaChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// FONT defines the 5x7 dot-matrix font for captcha characters.
// Each character is represented by 5 bytes (columns), each byte's bits represent rows.
var FONT = map[byte][]byte{
	'A': {0x7c, 0x12, 0x11, 0x12, 0x7c},
	'B': {0x7f, 0x49, 0x49, 0x49, 0x36},
	'C': {0x3e, 0x41, 0x41, 0x41, 0x22},
	'D': {0x7f, 0x41, 0x41, 0x41, 0x3e},
	'E': {0x7f, 0x49, 0x49, 0x49, 0x41},
	'F': {0x7f, 0x09, 0x09, 0x09, 0x01},
	'G': {0x3e, 0x41, 0x49, 0x49, 0x7a},
	'H': {0x7f, 0x08, 0x08, 0x08, 0x7f},
	'J': {0x20, 0x40, 0x41, 0x3f, 0x01},
	'K': {0x7f, 0x08, 0x14, 0x22, 0x41},
	'L': {0x7f, 0x40, 0x40, 0x40, 0x40},
	'M': {0x7f, 0x02, 0x0c, 0x02, 0x7f},
	'N': {0x7f, 0x04, 0x08, 0x10, 0x7f},
	'P': {0x7f, 0x09, 0x09, 0x09, 0x06},
	'Q': {0x3e, 0x41, 0x51, 0x21, 0x5e},
	'R': {0x7f, 0x09, 0x19, 0x29, 0x46},
	'S': {0x46, 0x49, 0x49, 0x49, 0x31},
	'T': {0x01, 0x01, 0x7f, 0x01, 0x01},
	'U': {0x3f, 0x40, 0x40, 0x40, 0x3f},
	'V': {0x1f, 0x20, 0x40, 0x20, 0x1f},
	'W': {0x3f, 0x40, 0x38, 0x40, 0x3f},
	'X': {0x63, 0x14, 0x08, 0x14, 0x63},
	'Y': {0x07, 0x08, 0x70, 0x08, 0x07},
	'Z': {0x61, 0x51, 0x49, 0x45, 0x43},
	'2': {0x62, 0x51, 0x49, 0x45, 0x43},
	'3': {0x22, 0x41, 0x49, 0x49, 0x36},
	'4': {0x0c, 0x0a, 0x09, 0x7f, 0x08},
	'5': {0x4f, 0x49, 0x49, 0x49, 0x31},
	'6': {0x3e, 0x49, 0x49, 0x49, 0x30},
	'7': {0x01, 0x71, 0x09, 0x05, 0x03},
	'8': {0x36, 0x49, 0x49, 0x49, 0x36},
	'9': {0x06, 0x49, 0x49, 0x49, 0x3e},
}

func randomInt(minVal, maxVal int) int {
	return rand.IntN(maxVal-minVal+1) + minVal
}

func randomChar() byte {
	return captchaChars[rand.IntN(len(captchaChars))]
}

// createBmp creates a 24-bit BMP image from BGR pixel data.
func createBmp(width, height int, pixels []byte) []byte {
	rowSize := ((width*3 + 3) / 4) * 4
	imageSize := rowSize * height
	fileSize := 54 + imageSize

	buffer := make([]byte, fileSize)

	// File header
	buffer[0] = 0x42 // B
	buffer[1] = 0x4d // M
	binary.LittleEndian.PutUint32(buffer[2:6], uint32(fileSize))
	binary.LittleEndian.PutUint32(buffer[10:14], 54)

	// Info header
	binary.LittleEndian.PutUint32(buffer[14:18], 40)
	binary.LittleEndian.PutUint32(buffer[18:22], uint32(width))
	binary.LittleEndian.PutUint32(buffer[22:26], uint32(height))
	binary.LittleEndian.PutUint16(buffer[26:28], 1)
	binary.LittleEndian.PutUint16(buffer[28:30], 24)
	binary.LittleEndian.PutUint32(buffer[34:38], uint32(imageSize))

	// Pixel data (bottom-up)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcIdx := ((height-1-y)*width + x) * 3
			dstIdx := 54 + y*rowSize + x*3
			if srcIdx+2 < len(pixels) {
				buffer[dstIdx] = pixels[srcIdx]     // B
				buffer[dstIdx+1] = pixels[srcIdx+1] // G
				buffer[dstIdx+2] = pixels[srcIdx+2] // R
			}
		}
	}

	return buffer
}

// GenerateCaptcha generates a captcha image with the specified number of characters.
func GenerateCaptcha(size int) CaptchaResult {
	var text strings.Builder
	text.Grow(size)
	for i := 0; i < size; i++ {
		text.WriteByte(randomChar())
	}

	const (
		scale   = 10
		padding = 15
	)
	charW := 5 * scale
	charH := 7 * scale
	width := size*charW + (size-1)*padding + padding*2
	height := charH + padding*2

	pixels := make([]byte, width*height*3)

	// Background color (light gray)
	for i := 0; i < len(pixels); i += 3 {
		pixels[i] = 220   // B
		pixels[i+1] = 220 // G
		pixels[i+2] = 220 // R
	}

	// Interference lines
	for l := 0; l < 6; l++ {
		x1 := randomInt(0, width-1)
		y1 := randomInt(0, height-1)
		x2 := randomInt(0, width-1)
		y2 := randomInt(0, height-1)
		steps := max(abs(x2-x1), abs(y2-y1))
		if steps == 0 {
			steps = 1
		}
		for s := 0; s <= steps; s++ {
			x := x1 + (x2-x1)*s/steps
			y := y1 + (y2-y1)*s/steps
			if x >= 0 && x < width && y >= 0 && y < height {
				idx := (y*width + x) * 3
				pixels[idx] = byte(randomInt(50, 150))
				pixels[idx+1] = byte(randomInt(50, 150))
				pixels[idx+2] = byte(randomInt(50, 150))
			}
		}
	}

	// Draw text
	captchaText := text.String()
	for i := 0; i < size; i++ {
		glyph := FONT[captchaText[i]]
		offsetX := padding + i*(charW+padding)
		offsetY := padding

		cr := byte(randomInt(20, 100))
		cg := byte(randomInt(20, 100))
		cb := byte(randomInt(20, 100))

		for gy := 0; gy < 7; gy++ {
			for gx := 0; gx < 5; gx++ {
				if glyph[gx]&(1<<gy) != 0 {
					for sy := 0; sy < scale; sy++ {
						for sx := 0; sx < scale; sx++ {
							px := offsetX + gx*scale + sx
							py := offsetY + gy*scale + sy
							if px < width && py < height {
								idx := (py*width + px) * 3
								pixels[idx] = cb
								pixels[idx+1] = cg
								pixels[idx+2] = cr
							}
						}
					}
				}
			}
		}
	}

	// Interference dots
	for i := 0; i < 40; i++ {
		x := randomInt(0, width-1)
		y := randomInt(0, height-1)
		idx := (y*width + x) * 3
		pixels[idx] = byte(randomInt(0, 200))
		pixels[idx+1] = byte(randomInt(0, 200))
		pixels[idx+2] = byte(randomInt(0, 200))
	}

	bmp := createBmp(width, height, pixels)

	return CaptchaResult{
		Text: captchaText,
		BMP:  bmp,
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
