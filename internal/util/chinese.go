package util

import "github.com/liuzl/gocc"

// t2sConverter converts Traditional Chinese to Simplified Chinese.
var t2sConverter *gocc.OpenCC

// s2tConverter converts Simplified Chinese to Traditional Chinese.
var s2tConverter *gocc.OpenCC

func init() {
	var err error
	t2sConverter, err = gocc.New("t2s")
	if err != nil {
		panic("failed to initialize t2s converter: " + err.Error())
	}
	s2tConverter, err = gocc.New("s2t")
	if err != nil {
		panic("failed to initialize s2t converter: " + err.Error())
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

// ToTraditional converts Simplified Chinese text to Traditional Chinese.
func ToTraditional(text string) string {
	result, err := s2tConverter.Convert(text)
	if err != nil {
		return text
	}
	return result
}