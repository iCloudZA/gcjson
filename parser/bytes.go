package parser

import "strconv"

// IsIntegerLiteralBytes 与 IsIntegerLiteral(string) 逻辑一致，但避免 string 转换
func IsIntegerLiteralBytes(b []byte) bool {
	for i := 0; i < len(b); i++ {
		c := b[i]
		if c == '.' || c == 'e' || c == 'E' || c == '+' || (c == '-' && i > 0) {
			return false
		}
	}
	return true
}

// 快速 int64 解析的 []byte 版本
func ParseIntFastBytes(b []byte) (int64, bool) {
	if len(b) == 0 {
		return 0, false
	}
	i := 0
	neg := false
	if b[0] == '-' {
		neg, i = true, 1
		if len(b) == 1 {
			return 0, false
		}
	}
	var n int64
	for ; i < len(b); i++ {
		c := b[i] - '0'
		if c > 9 {
			return 0, false
		}
		d := int64(c)
		if n > (9223372036854775807-d)/10 {
			return 0, false
		}
		n = n*10 + d
	}
	if neg {
		n = -n
	}
	return n, true
}

// ParseFloat64Bytes 直接用标准库的零拷贝接口	：strconv.ParseFloat 需要 string，但只在确认为非整数时调用
func ParseFloat64Bytes(b []byte) (float64, bool) {
	f, err := strconv.ParseFloat(string(b), 64)
	return f, err == nil
}
