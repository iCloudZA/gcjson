package fast

import (
	"github.com/icloudza/gcjson/convert"
	"github.com/tidwall/gjson"
	"math"
)

//go:nosplit
func IsSimpleTopKey(path string) bool {
	if len(path) == 0 {
		return false
	}
	for i := 0; i < len(path); i++ {
		c := path[i]
		// 允许 a-zA-Z0-9_-
		if (c|0x20) >= 'a' && (c|0x20) <= 'z' {
			continue
		}
		if c >= '0' && c <= '9' {
			continue
		}
		if c == '_' || c == '-' {
			continue
		}
		return false
	}
	return true
}

//go:nosplit
func GetTopKeyFast(b []byte, key string) (gjson.Result, bool) {
	if len(b) < 2 || b[0] != '{' {
		return gjson.Result{}, false
	}
	kb := convert.UnsafeStringToBytes(key)
	i := 1
	for i < len(b) {
		// 跳过空白
		for i < len(b) && b[i] <= 32 {
			i++
		}
		if i >= len(b) || b[i] == '}' {
			break
		}
		// 读取字符串 key
		if b[i] != '"' {
			return gjson.Result{}, false
		}
		i++
		start := i
		esc := false
		for i < len(b) {
			c := b[i]
			i++
			if esc {
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
				continue
			}
			if c == '"' {
				break
			}
		}
		if i > len(b) {
			return gjson.Result{}, false
		}
		ks := b[start : i-1]

		// 冒号
		for i < len(b) && b[i] <= 32 {
			i++
		}
		if i >= len(b) || b[i] != ':' {
			return gjson.Result{}, false
		}
		i++
		for i < len(b) && b[i] <= 32 {
			i++
		}

		// 比较 key（大小写敏感）
		if len(ks) == len(kb) {
			eq := true
			for j := 0; j < len(kb); j++ {
				if ks[j] != kb[j] {
					eq = false
					break
				}
			}
			if eq {
				// 直接在原 JSON 上用 gjson 取该 key（可靠且仍很快）
				return gjson.GetBytes(b, key), true
			}
		}

		// 跳过值到下一个逗号或对象结束（简化状态机）
		depth := 0
		inStr := false
		esc2 := false
		for i < len(b) {
			c := b[i]
			i++
			if inStr {
				if esc2 {
					esc2 = false
					continue
				}
				if c == '\\' {
					esc2 = true
					continue
				}
				if c == '"' {
					inStr = false
				}
				continue
			}
			switch c {
			case '"':
				inStr = true
			case '{', '[':
				depth++
			case '}', ']':
				if depth == 0 && c == '}' {
					goto NEXT
				}
				if depth > 0 {
					depth--
				}
			case ',':
				if depth == 0 {
					goto NEXT
				}
			}
		}
	NEXT:
		continue
	}
	return gjson.Result{}, false
}

// ParseFloat 快速解析 JSON 数字（零分配）
// 仅支持标准 JSON 格式，不支持 NaN/Inf
func ParseFloat(b []byte) (float64, bool) {
	if len(b) == 0 {
		return 0, false
	}
	i := 0
	neg := false
	if b[i] == '-' {
		neg = true
		i++
	}

	// 整数部分
	var intPart uint64
	for i < len(b) {
		c := b[i]
		if c < '0' || c > '9' {
			break
		}
		intPart = intPart*10 + uint64(c-'0')
		i++
	}

	// 小数部分
	var fracPart uint64
	var fracDiv float64 = 1
	if i < len(b) && b[i] == '.' {
		i++
		for i < len(b) {
			c := b[i]
			if c < '0' || c > '9' {
				break
			}
			fracPart = fracPart*10 + uint64(c-'0')
			fracDiv *= 10
			i++
		}
	}

	// 指数部分
	expSign := 1
	expVal := 0
	if i < len(b) && (b[i] == 'e' || b[i] == 'E') {
		i++
		if i < len(b) && b[i] == '+' {
			i++
		} else if i < len(b) && b[i] == '-' {
			expSign = -1
			i++
		}
		for i < len(b) {
			c := b[i]
			if c < '0' || c > '9' {
				break
			}
			expVal = expVal*10 + int(c-'0')
			i++
		}
	}

	// 组合
	f := float64(intPart) + float64(fracPart)/fracDiv
	if expVal != 0 {
		f *= math.Pow10(expSign * expVal)
	}
	if neg {
		f = -f
	}
	return f, true
}
