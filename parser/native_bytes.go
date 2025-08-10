package parser

import (
	"github.com/icloudza/gcjson/zeronode"
)

// ToNativeBytes 将一段 JSON（完整值，非片段）转为 Go 原生类型：
// null -> nil
// true/false -> bool
// "str" -> string
// 123 / 3.14 -> int64 / float64（尽量整数快路径）
// [ ... ] -> []any（递归）
// { ... } -> map[string]any（递归）
//
// 完全不依赖 gjson.Result。
func ToNativeBytes(b []byte) any {
	n := zeronode.FromBytes(b)
	switch n.Type() {
	case 'l': // null
		return nil
	case 'b': // bool
		v, _ := n.Bool()
		return v
	case 's': // string
		// 这里返回已反转义的 string（有分配，便于通用）
		return n.UnescapedString()
	case 'n': // number
		// 尽量走整数快路径
		if i, ok := ParseIntFastBytes(b); ok && IsIntegerLiteralBytes(b) {
			return i
		}
		if f, ok := ParseFloat64Bytes(b); ok {
			return f
		}
		// 解析失败就原样返回字符串（理论上不该走到）
		return string(b)
	case 'a': // array
		out := make([]any, 0, 8)
		n.ForEachArray(func(_ int, v zeronode.Node) bool {
			out = append(out, ToNativeBytes(v.Raw()))
			return true
		})
		return out
	case 'o': // object
		m := make(map[string]any, 8)
		n.ForEachObject(func(k []byte, v zeronode.Node) bool {
			m[string(k)] = ToNativeBytes(v.Raw())
			return true
		})
		return m
	default:
		return nil
	}
}
