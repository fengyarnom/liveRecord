package slug

import (
	pinyin "github.com/mozillazg/go-pinyin"
	"strings"
)

func Normalize(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	a := pinyin.NewArgs()
	a.Style = pinyin.Normal
	var b strings.Builder
	for _, r := range s {
		if r > 127 {
			pys := pinyin.SinglePinyin(r, a)
			if len(pys) > 0 {
				b.WriteString(pys[0])
				b.WriteByte(' ')
				continue
			}
			b.WriteByte(' ')
			continue
		}
		ch := byte(r)
		if ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == ' ' || ch == '-' {
			b.WriteByte(ch)
		} else {
			b.WriteByte(' ')
		}
	}
	out := b.String()
	out = strings.TrimSpace(out)
	out = strings.ReplaceAll(out, " ", "-")
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	out = strings.Trim(out, "-")
	return out
}
