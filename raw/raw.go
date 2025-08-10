package raw

import (
	"github.com/icloudza/gcjson/convert"
	pathplan "github.com/icloudza/gcjson/internal"
	"github.com/icloudza/gcjson/zeronode"
)

// ===== 对外 API =====

// Get 返回 pathplan 对应节点的原始 JSON 字符串（会拷贝一次变成 string）
func Get(v any, path string) (string, bool) {
	if bs, ok := GetBytes(v, path); ok {
		return string(bs), true
	}
	return "", false
}

// GetBytes 返回 pathplan 对应节点在原始 JSON 中的字节切片（零拷贝）
func GetBytes(v any, path string) ([]byte, bool) {
	b, err := convert.From(v)
	if err != nil {
		return nil, false
	}
	if n, ok := findByPath(b, path); ok {
		return trimSpaceBytes(n.Raw()), true
	}
	return nil, false
}

// GetBytesByPlan 新 API：plan 快路径（推荐）
func GetBytesByPlan(doc []byte, pl *pathplan.Plan) ([]byte, bool) {
	node := zeronode.New(doc)
	ok := true
	for i := 0; i < len(pl.Segs); i++ {
		sg := pl.Segs[i]
		if sg.Key != "" {
			node, ok = node.ObjectKey(sg.Key)
		} else {
			if sg.Idx < 0 {
				return nil, false
			}
			node, ok = node.ArrayIndex(sg.Idx)
		}
		if !ok {
			return nil, false
		}
	}
	return node.RawBytes(), true
}

// GetManyBytesByPlan 批量
func GetManyBytesByPlan(doc []byte, plans []*pathplan.Plan) ([][]byte, []bool) {
	out := make([][]byte, len(plans))
	okv := make([]bool, len(plans))
	for i := range plans {
		out[i], okv[i] = GetBytesByPlan(doc, plans[i])
	}
	return out, okv
}

// GetMany 批量获取多个 pathplan 的原始 JSON（string 版本）
func GetMany(v any, paths ...string) ([]string, error) {
	bss, err := GetManyBytes(v, paths...)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(bss))
	for i, bs := range bss {
		if len(bs) > 0 {
			out[i] = string(bs)
		}
	}
	return out, nil
}

// GetManyBytes 批量获取多个 pathplan 的原始 JSON（零拷贝 []byte）
func GetManyBytes(v any, paths ...string) ([][]byte, error) {
	b, err := convert.From(v)
	if err != nil {
		return nil, err
	}
	out := make([][]byte, len(paths))
	for i, p := range paths {
		if n, ok := findByPath(b, p); ok {
			out[i] = trimSpaceBytes(n.Raw())
		}
	}
	return out, nil
}

// ===== 路径解析 =====

func findByPath(b []byte, path string) (zeronode.Node, bool) {
	cur := zeronode.FromBytes(b)
	if cur.Type() == 0 {
		return zeronode.Node{}, false
	}

	start := 0
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == '.' {
			if i > start {
				seg := path[start:i]
				if segIsDigits(seg) {
					if cur.Type() != 'a' {
						return zeronode.Node{}, false
					}
					idx := atoiUnsafe(seg)
					if next, ok := arrayIndex(cur, idx); ok {
						cur = next
					} else {
						return zeronode.Node{}, false
					}
				} else {
					if cur.Type() != 'o' {
						return zeronode.Node{}, false
					}
					child := cur.Get(seg)
					if child.Type() == 0 {
						return zeronode.Node{}, false
					}
					cur = child
				}
			}
			start = i + 1
		}
	}
	return cur, true
}

func arrayIndex(n zeronode.Node, want int) (zeronode.Node, bool) {
	var got zeronode.Node
	n.ForEachArray(func(idx int, v zeronode.Node) bool {
		if idx == want {
			got = v
			return false
		}
		return true
	})
	return got, got.Type() != 0
}

// ===== 辅助函数 =====

func segIsDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func atoiUnsafe(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*10 + int(s[i]-'0')
	}
	return n
}

func trimSpaceBytes(b []byte) []byte {
	start := 0
	for start < len(b) {
		switch b[start] {
		case ' ', '\n', '\t', '\r':
			start++
		default:
			goto trimEnd
		}
	}
trimEnd:
	end := len(b)
	for end > start {
		switch b[end-1] {
		case ' ', '\n', '\t', '\r':
			end--
		default:
			goto done
		}
	}
done:
	if start == 0 && end == len(b) {
		return b // 无空白，直接返回原 slice
	}
	return b[start:end]
}
