package structfast

import (
	"reflect"
	"time"
	"unsafe"

	"github.com/icloudza/gcjson/zeronode"
)

var timeType = reflect.TypeOf(time.Time{})

func Decode[T any](root zeronode.Node, out *T) bool {
	if out == nil || root.Type() != 'o' {
		return false
	}
	rv := reflect.ValueOf(out).Elem()
	return decodeStruct(root, rv, getTypePlan(rv.Type()))
}

// ===== 核心递归 =====

func decodeStruct(obj zeronode.Node, dst reflect.Value, plan *typePlan) bool {
	okAny := false
	for _, f := range plan.fields {
		n := getByPath(obj, f.path)
		if n.Type() == 0 {
			continue
		}
		if decodeValue(n, dst.Field(f.index)) {
			okAny = true
		}
	}
	return okAny
}

func decodeValue(n zeronode.Node, fv reflect.Value) bool {
	if !fv.CanSet() {
		return false
	}

	// 指针：按元素递归
	if fv.Kind() == reflect.Pointer {
		if fv.IsNil() {
			fv.Set(reflect.New(fv.Type().Elem()))
		}
		return decodeValue(n, fv.Elem())
	}

	// time.Time
	if fv.Type() == timeType {
		if t, ok := parseTimeNode(n); ok {
			*(*time.Time)(unsafe.Pointer(fv.UnsafeAddr())) = t
			return true
		}
		return false
	}

	switch fv.Kind() {
	case reflect.String:
		if n.Type() == 's' {
			// 若不需转义解码可用 n.String()（零拷贝）
			fv.SetString(n.UnescapedString())
			return true
		}
	case reflect.Bool:
		if n.Type() == 'b' {
			if v, ok := n.Bool(); ok {
				fv.SetBool(v)
				return true
			}
		}
	case reflect.Int, reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8:
		if n.Type() == 'n' {
			if v, ok := n.Int(); ok {
				fv.SetInt(v)
				return true
			}
		}
	case reflect.Uint, reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8:
		if n.Type() == 'n' {
			if v, ok := n.Int(); ok && v >= 0 {
				fv.SetUint(uint64(v))
				return true
			}
		}
	case reflect.Float64, reflect.Float32:
		if n.Type() == 'n' {
			if v, ok := n.Float(); ok {
				fv.SetFloat(v)
				return true
			}
		}
	case reflect.Struct:
		if n.Type() == 'o' {
			return decodeStruct(n, fv, getTypePlan(fv.Type()))
		}
	case reflect.Slice:
		// []byte: 从字符串
		if fv.Type().Elem().Kind() == reflect.Uint8 {
			if n.Type() == 's' {
				// 复制一份，避免引用底层 JSON 缓冲
				sb := []byte(n.UnescapedString())
				fv.SetBytes(sb)
				return true
			}
			return false
		}
		if n.Type() != 'a' {
			return false
		}
		et := fv.Type().Elem()
		tmp := reflect.MakeSlice(fv.Type(), 0, 8)
		n.ForEachArray(func(_ int, elem zeronode.Node) bool {
			ev := reflect.New(et).Elem()
			if decodeValue(elem, ev) {
				tmp = reflect.Append(tmp, ev)
			}
			return true
		})
		fv.Set(tmp)
		return true
	case reflect.Map:
		// 仅支持 map[string]T
		if fv.Type().Key().Kind() != reflect.String || n.Type() != 'o' {
			return false
		}
		vt := fv.Type().Elem()
		if fv.IsNil() {
			fv.Set(reflect.MakeMapWithSize(fv.Type(), 8))
		}
		n.ForEachObject(func(k []byte, vv zeronode.Node) bool {
			ev := reflect.New(vt).Elem()
			if decodeValue(vv, ev) {
				fv.SetMapIndex(reflect.ValueOf(string(k)), ev)
			}
			return true
		})
		return true
	default:
		return false
	}

	return false
}

// ===== 路径、时间辅助 =====

func getByPath(n zeronode.Node, path []string) zeronode.Node {
	cur := n
	for i := 0; i < len(path); i++ {
		if cur.Type() != 'o' {
			return zeronode.Node{}
		}
		cur = cur.Get(path[i]) // 单段零分配
		if cur.Type() == 0 {
			return zeronode.Node{}
		}
	}
	return cur
}

func parseTimeNode(n zeronode.Node) (time.Time, bool) {
	switch n.Type() {
	case 's':
		s := n.UnescapedString()
		// 快路径：RFC3339 / RFC3339Nano
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t, true
		}
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return t, true
		}
		// 常见 ISO8601 变体
		layouts := []string{
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05",
			"2006-01-02",
		}
		for _, ly := range layouts {
			if t, err := time.ParseInLocation(ly, s, time.Local); err == nil {
				return t, true
			}
		}
	case 'n':
		if iv, ok := n.Int(); ok {
			// 10 位秒，13 位毫秒
			switch {
			case iv > 1e14: // 微秒/纳秒，按纳秒处理
				return time.Unix(0, iv), true
			case iv >= 1e12: // 毫秒
				return time.Unix(0, iv*int64(time.Millisecond)), true
			default: // 秒
				return time.Unix(iv, 0), true
			}
		}
	}
	return time.Time{}, false
}
