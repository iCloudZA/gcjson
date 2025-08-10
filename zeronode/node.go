package zeronode

import (
	"bytes"
	"github.com/icloudza/gcjson/fast"
	"unicode/utf8"
	"unsafe"
)

// Node 表示一个零分配（zero-allocation）的 JSON 节点。
// 节点不会复制 JSON 内容，内部保存原始字节切片和在其中的范围。
// 这样可以高性能地进行字段访问和遍历。
//
// typ 字段说明：
//
//	'o' = object（对象）
//	'a' = array（数组）
//	's' = string（字符串）
//	'n' = number（数字）
//	'b' = bool（布尔）
//	'l' = null（空值）
type Node struct {
	raw   []byte // 原始 JSON 数据
	start int    // 节点值起始索引
	end   int    // 节点值结束索引（不包含）
	typ   byte   // 节点类型
}

//
// ========================= 构造与类型判断 =========================
//

// FromBytes 从 JSON 字节构造一个 Node。
// 不会解析成 Go 结构体，仅找到根节点的起止位置和类型。
func FromBytes(b []byte) Node {
	start, typ := skipWS(b, 0)
	if start >= len(b) {
		return Node{}
	}
	end := findValueEnd(b, start)
	return Node{raw: b, start: start, end: end, typ: typ}
}

// Type 返回节点类型。
// 返回值参考 Node.typ 说明。
func (n Node) Type() byte { return n.typ }

//
// ========================= 基本类型访问 =========================
//

// String 返回 JSON 字符串值（去掉引号），零拷贝。
// 如果节点不是字符串类型，返回空字符串。
func (n Node) String() string {
	if n.typ != 's' {
		return ""
	}
	return unsafe.String(&n.raw[n.start+1], n.end-n.start-2)
}

// StringBytes 返回 JSON 字符串值的原始字节（去掉引号），零拷贝。
// 如果节点不是字符串类型，返回 nil。
func (n Node) StringBytes() []byte {
	if n.typ != 's' {
		return nil
	}
	return n.raw[n.start+1 : n.end-1]
}

// UnescapedString 返回解码后的字符串（处理转义符，例如 \n、\uXXXX）。
// 有内存分配（非零拷贝），仅在需要解码时调用。
func (n Node) UnescapedString() string {
	if n.typ != 's' {
		return ""
	}
	b := unescapeJSONString(n.raw[n.start+1 : n.end-1])
	return string(b)
}

// Int 尝试解析节点为整数。
// 返回值：整数值、是否成功。
func (n Node) Int() (int64, bool) {
	if n.typ != 'n' {
		return 0, false
	}
	i := n.start
	neg := false
	if n.raw[i] == '-' {
		neg = true
		i++
	}
	var v int64
	for ; i < n.end; i++ {
		c := n.raw[i]
		if c < '0' || c > '9' {
			break
		}
		v = v*10 + int64(c-'0')
	}
	if neg {
		v = -v
	}
	return v, true
}

// Float 尝试解析节点为浮点数。
// 返回值：浮点值、是否成功。
func (n Node) Float() (float64, bool) {
	if n.typ != 'n' {
		return 0, false
	}
	f, ok := fast.ParseFloat(n.raw[n.start:n.end])
	return f, ok
}

// Bool 尝试解析节点为布尔值。
// 返回值：布尔值、是否成功。
func (n Node) Bool() (bool, bool) {
	if n.typ != 'b' {
		return false, false
	}
	return n.raw[n.start] == 't', true
}

// IsNull 判断节点是否为 null。
func (n Node) IsNull() bool { return n.typ == 'l' }

//
// ========================= 对象 / 数组访问 =========================
//

// Get 在对象节点中获取指定 key 的值（string 形式）。
// 如果节点不是对象，返回空节点。
func (n Node) Get(key string) Node { return n.getBytes([]byte(key)) }

// GetBytes 在对象节点中获取指定 key 的值（[]byte 形式）。
// 如果节点不是对象，返回空节点。
func (n Node) GetBytes(key []byte) Node { return n.getBytes(key) }

// getBytes 是 Get/GetBytes 的核心实现。
func (n Node) getBytes(key []byte) Node {
	if n.typ != 'o' {
		return Node{}
	}
	i := n.start + 1 // 跳过 '{'
	for i < n.end {
		i = skipSpaces(n.raw, i)
		if i >= n.end || n.raw[i] == '}' {
			break
		}
		if n.raw[i] != '"' {
			return Node{}
		}
		ks := i + 1
		i++
		for i < n.end && n.raw[i] != '"' {
			if n.raw[i] == '\\' {
				i++
			}
			i++
		}
		ke := i
		i++
		i = skipSpaces(n.raw, i)
		if i >= n.end || n.raw[i] != ':' {
			return Node{}
		}
		i++
		i = skipSpaces(n.raw, i)
		valStart, typ := skipWS(n.raw, i)
		valEnd := findValueEnd(n.raw, valStart)
		if bytes.Equal(n.raw[ks:ke], key) {
			return Node{raw: n.raw, start: valStart, end: valEnd, typ: typ}
		}
		i = valEnd
		i = skipSpaces(n.raw, i)
		if i < n.end && n.raw[i] == ',' {
			i++
		}
	}
	return Node{}
}

// GetPath 按路径链式访问节点。
// 例如：n.GetPath("a", "b", "c") 等价于 n.Get("a").Get("b").Get("c")。
func (n Node) GetPath(keys ...string) Node {
	cur := n
	for _, k := range keys {
		cur = cur.Get(k)
		if cur.typ == 0 {
			break
		}
	}
	return cur
}

// GetPathFast 是一次扫描完成多层路径查找的版本，比多次 Get 快很多。
// 零分配，适合性能敏感场景。
// GetPathFast 一次扫描完成多级路径解析（比递归 Get 快很多）
func (n Node) GetPathFast(keys ...string) Node {
	cur := n
	for depth, key := range keys {
		if cur.typ != 'o' {
			return Node{}
		}
		i := cur.start + 1
		found := false
		for i < cur.end {
			// 跳过空白
			i = skipSpaces(cur.raw, i)
			if i >= cur.end || cur.raw[i] == '}' {
				break
			}
			// 必须是字符串 key
			if cur.raw[i] != '"' {
				return Node{}
			}
			ks := i + 1
			i++
			for i < cur.end && cur.raw[i] != '"' {
				if cur.raw[i] == '\\' {
					i++
				}
				i++
			}
			ke := i
			i++
			i = skipSpaces(cur.raw, i)
			if i >= cur.end || cur.raw[i] != ':' {
				return Node{}
			}
			i++
			i = skipSpaces(cur.raw, i)
			valStart, typ := skipWS(cur.raw, i)
			valEnd := findValueEnd(cur.raw, valStart)

			// key 匹配
			if ke-ks == len(key) {
				match := true
				for j := 0; j < len(key); j++ {
					if cur.raw[ks+j] != key[j] {
						match = false
						break
					}
				}
				if match {
					cur = Node{raw: cur.raw, start: valStart, end: valEnd, typ: typ}
					found = true
					break
				}
			}

			i = valEnd
			i = skipSpaces(cur.raw, i)
			if i < cur.end && cur.raw[i] == ',' {
				i++
			}
		}
		if !found {
			return Node{}
		}
		// 最后一层直接返回
		if depth == len(keys)-1 {
			return cur
		}
	}
	return cur
}

// ForEachObject 遍历对象的所有 key-value 对。
// 回调函数返回 false 时中止遍历。
func (n Node) ForEachObject(fn func(k []byte, v Node) bool) {
	if n.typ != 'o' {
		return
	}
	i := n.start + 1
	for i < n.end {
		i = skipSpaces(n.raw, i)
		if i >= n.end || n.raw[i] == '}' {
			return
		}
		if n.raw[i] != '"' {
			return
		}
		ks := i + 1
		i++
		for i < n.end && n.raw[i] != '"' {
			if n.raw[i] == '\\' {
				i++
			}
			i++
		}
		ke := i
		i++
		i = skipSpaces(n.raw, i)
		if i >= n.end || n.raw[i] != ':' {
			return
		}
		i++
		i = skipSpaces(n.raw, i)
		valStart, typ := skipWS(n.raw, i)
		valEnd := findValueEnd(n.raw, valStart)
		if !fn(n.raw[ks:ke], Node{raw: n.raw, start: valStart, end: valEnd, typ: typ}) {
			return
		}
		i = valEnd
		i = skipSpaces(n.raw, i)
		if i < n.end && n.raw[i] == ',' {
			i++
		}
	}
}

// ForEachArray 遍历数组的所有元素。
// 回调函数返回 false 时中止遍历。
func (n Node) ForEachArray(fn func(idx int, v Node) bool) {
	if n.typ != 'a' {
		return
	}
	i := n.start + 1
	idx := 0
	for i < n.end {
		i = skipSpaces(n.raw, i)
		if i >= n.end || n.raw[i] == ']' {
			return
		}
		valStart, typ := skipWS(n.raw, i)
		valEnd := findValueEnd(n.raw, valStart)
		if !fn(idx, Node{raw: n.raw, start: valStart, end: valEnd, typ: typ}) {
			return
		}
		idx++
		i = valEnd
		i = skipSpaces(n.raw, i)
		if i < n.end && n.raw[i] == ',' {
			i++
		}
	}
}

//
// ========================= 内部工具 =========================
//

// skipWS 跳过空白字符，返回下一个非空白字符位置及类型。
func skipWS(b []byte, i int) (int, byte) {
	for i < len(b) && wsTable[b[i]] {
		i++
	}
	if i >= len(b) {
		return i, 0
	}
	switch b[i] {
	case '{':
		return i, 'o'
	case '[':
		return i, 'a'
	case '"':
		return i, 's'
	case 't', 'f':
		return i, 'b'
	case 'n':
		return i, 'l'
	default:
		return i, 'n'
	}
}

// skipSpaces 跳过空格、换行、制表符等。
func skipSpaces(b []byte, i int) int {
	for i < len(b) && wsTable[b[i]] {
		i++
	}
	return i
}

// findValueEnd 查找 JSON 值的结束位置。
// 从值的起始位置开始，扫描直到值的末尾。
func findValueEnd(b []byte, i int) int {
	if i >= len(b) {
		return i
	}
	switch b[i] {
	case '{':
		depth := 0
		for ; i < len(b); i++ {
			if b[i] == '{' {
				depth++
			} else if b[i] == '}' {
				depth--
				if depth == 0 {
					return i + 1
				}
			}
		}
	case '[':
		depth := 0
		for ; i < len(b); i++ {
			if b[i] == '[' {
				depth++
			} else if b[i] == ']' {
				depth--
				if depth == 0 {
					return i + 1
				}
			}
		}
	case '"':
		i++
		for ; i < len(b); i++ {
			if b[i] == '"' {
				return i + 1
			}
			if b[i] == '\\' {
				i++
			}
		}
	default:
		for ; i < len(b); i++ {
			switch b[i] {
			case ',', '}', ']':
				return i
			}
		}
	}
	return i
}

// unescapeJSONString 反转义 JSON 字符串（\n、\uXXXX 等）。
func unescapeJSONString(b []byte) []byte {
	var out []byte
	for i := 0; i < len(b); i++ {
		if b[i] != '\\' {
			out = append(out, b[i])
			continue
		}
		i++
		if i >= len(b) {
			break
		}
		switch b[i] {
		case '"', '\\', '/':
			out = append(out, b[i])
		case 'b':
			out = append(out, '\b')
		case 'f':
			out = append(out, '\f')
		case 'n':
			out = append(out, '\n')
		case 'r':
			out = append(out, '\r')
		case 't':
			out = append(out, '\t')
		case 'u':
			if i+4 < len(b) {
				r, _ := decodeHexRune(b[i+1 : i+5])
				i += 4
				buf := make([]byte, utf8.RuneLen(r))
				utf8.EncodeRune(buf, r)
				out = append(out, buf...)
			}
		}
	}
	return out
}

// decodeHexRune 将 4 个十六进制字符解析为 Unicode 码点。
func decodeHexRune(b []byte) (rune, bool) {
	var r rune
	for _, c := range b {
		r <<= 4
		switch {
		case c >= '0' && c <= '9':
			r += rune(c - '0')
		case c >= 'a' && c <= 'f':
			r += rune(c-'a') + 10
		case c >= 'A' && c <= 'F':
			r += rune(c-'A') + 10
		default:
			return utf8.RuneError, false
		}
	}
	return r, true
}

var wsTable [256]bool

func init() {
	for _, c := range []byte{' ', '\n', '\r', '\t'} {
		wsTable[c] = true
	}
}
