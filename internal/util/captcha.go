package util

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math/rand/v2"

	"github.com/steambap/captcha"
)

// CaptchaResult holds the generated captcha text and PNG image data.
type CaptchaResult struct {
	Text string
	PNG  []byte
}

const captchaChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// GenerateCaptcha generates a 4:3 (480x360) PNG captcha with generous safe margins
// so Telegram chat bubble thumbnails never crop any characters.
func GenerateCaptcha(size int) CaptchaResult {
	if size <= 0 {
		return CaptchaResult{}
	}

	const (
		padX   = 60
		padY   = 110
		innerH = 240
	)
	innerW := max(440, size*88)
	width := innerW + padX*2
	height := innerH + padY*2

	data, err := captcha.New(innerW, innerH, func(o *captcha.Options) {
		o.CharPreset = captchaChars
		o.TextLength = size
		o.CurveNumber = 3
		o.Noise = 1.2
		o.FontScale = 0.60
	})
	if err != nil {
		panic("failed to generate captcha: " + err.Error())
	}

	var innerBuf bytes.Buffer
	if err := data.WriteImage(&innerBuf); err != nil {
		panic("failed to encode inner captcha PNG: " + err.Error())
	}
	innerImg, err := png.Decode(&innerBuf)
	if err != nil {
		panic("failed to decode inner captcha PNG: " + err.Error())
	}

	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)

	innerRect := image.Rect(padX, padY, padX+innerW, padY+innerH)
	draw.Draw(canvas, innerRect, innerImg, innerImg.Bounds().Min, draw.Over)

	// Match the inner captcha noise density across the outer margin so there is no visible border.
	innerArea := innerW * innerH
	outerArea := width*height - innerArea
	outerDots := outerArea * 5 / 100
	for i := 0; i < outerDots; i++ {
		x := rand.IntN(width)
		y := rand.IntN(height)
		if image.Pt(x, y).In(innerRect) {
			continue
		}
		canvas.SetRGBA(x, y, color.RGBA{
			R: uint8(rand.IntN(220)),
			G: uint8(rand.IntN(220)),
			B: uint8(rand.IntN(220)),
			A: 255,
		})
	}

	var outBuf bytes.Buffer
	if err := png.Encode(&outBuf, canvas); err != nil {
		panic("failed to encode captcha PNG: " + err.Error())
	}

	return CaptchaResult{
		Text: data.Text,
		PNG:  outBuf.Bytes(),
	}
}
