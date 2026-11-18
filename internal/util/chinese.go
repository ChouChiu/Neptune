package util

import "github.com/liuzl/gocc"

// t2sConverter converts Traditional Chinese to Simplified Chinese.
var t2sConverter *gocc.OpenCC

func init() {
	var err error
	t2sConverter, err = gocc.New("t2s")
	if err != nil {
		panic("failed to initialize t2s converter: " + err.Error())
	}
}

// ToSimplified converts Traditional Chinese text to Simplified Chinese.
func ToSimplified(text string) string {
	result, err := t2sConverter.Convert(text)
	if err != nil {
		return text
	}
	return result
}
