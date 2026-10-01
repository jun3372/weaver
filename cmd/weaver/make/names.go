package make

import (
	"strings"
	"unicode"
)

// toSnake 将驼峰命名转为 snake_case:HTTPApi → http_api,API2Echo → api2_echo。
func toSnake(name string) string {
	var sb strings.Builder
	runes := []rune(name)
	for i, r := range runes {
		if unicode.IsUpper(r) && i > 0 {
			prev := runes[i-1]
			boundary := unicode.IsLower(prev) || unicode.IsDigit(prev)
			if !boundary && unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
				boundary = true
			}
			if boundary {
				sb.WriteByte('_')
			}
		}
		if unicode.IsUpper(r) {
			r = unicode.ToLower(r)
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// unexport 将首字母转为小写:Chat → chat,HTTPApi → hTTPApi(仅用于实现结构体命名)。
func unexport(name string) string {
	if name == "" {
		return name
	}
	runes := []rune(name)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}
