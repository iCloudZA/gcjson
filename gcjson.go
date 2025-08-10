package gcjson

import (
	"errors"
	"github.com/icloudza/gcjson/cache"
	"github.com/icloudza/gcjson/convert"
	"github.com/icloudza/gcjson/fast"
	"github.com/icloudza/gcjson/parser"
	"github.com/icloudza/gcjson/picker"
	"github.com/icloudza/gcjson/raw"
	"github.com/icloudza/gcjson/structfast"
	"github.com/tidwall/gjson"
)

type Result = gjson.Result

func init() {
	gjson.DisableModifiers = true
}

func get(b []byte, path string) Result {
	if fast.IsSimpleTopKey(path) {
		if r, ok := fast.GetTopKeyFast(b, path); ok {
			return r
		}
	}
	if !cache.HitHot(path) {
		cache.PutHot(path)
	}
	return gjson.GetBytes(b, path)
}

func GetAny(v any, path string) (gjson.Result, error) {
	b, err := convert.From(v)
	if err != nil {
		return gjson.Result{}, err
	}
	if len(b) > convert.MaxJSONSize {
		return gjson.Result{}, errors.New("json too large")
	}
	return get(b, path), nil
}

func GetData(v any, path string) (gjson.Result, error) {
	core := picker.PickData(v, picker.GetDefaultDrillKeys())
	b, err := convert.From(core)
	if err != nil {
		return gjson.Result{}, err
	}
	if len(b) > convert.MaxJSONSize {
		return gjson.Result{}, errors.New("json too large")
	}
	return get(b, path), nil
}

func GetDataWithKeys(v any, keys []string, path string) (gjson.Result, error) {
	core := picker.PickData(v, keys)
	b, err := convert.From(core)
	if err != nil {
		return gjson.Result{}, err
	}
	if len(b) > convert.MaxJSONSize {
		return gjson.Result{}, errors.New("json too large")
	}
	return get(b, path), nil
}

// Any 自动类型推断
func Any(v any, path string) any {
	r, err := GetAny(v, path)
	if err != nil || len(r.Raw) == 0 {
		return nil
	}
	return parser.ToNative(r)
}

func AnyOr(v any, path string, def any) any {
	if x := Any(v, path); x != nil {
		return x
	}
	return def
}

func AnyData(v any, path string) any {
	r, err := GetData(v, path)
	if err != nil || len(r.Raw) == 0 {
		return nil
	}
	return parser.ToNative(r)
}

func AnyDataWithKeys(v any, keys []string, path string) any {
	r, err := GetDataWithKeys(v, keys, path)
	if err != nil || len(r.Raw) == 0 {
		return nil
	}
	return parser.ToNative(r)
}

func AnyAs[T any](v any, path string) (T, bool) {
	var zero T

	// 结构体快路径：v 是 *struct 且 pathplan 是字段链
	if isLikelyStructPath(path) {
		if out, ok := structfast.GetByPtrTyped[T](v, path); ok { // 泛型快路径
			return out, true
		}
	}

	// JSON 回退
	r, err := GetAny(v, path)
	if err != nil || len(r.Raw) == 0 {
		return zero, false
	}
	return parser.ToNativeTyped[T](r)
}

// 仅允许 A.B.C 这种导出字段链：A-Za-z0-9_，每段首字母必须大写（导出）
func isLikelyStructPath(p string) bool {
	if p == "" {
		return false
	}
	start := 0
	for i := 0; i <= len(p); i++ {
		if i == len(p) || p[i] == '.' {
			seg := p[start:i]
			if len(seg) == 0 {
				return false
			}
			// 首字母大写（导出）
			c := seg[0]
			if !(c >= 'A' && c <= 'Z') {
				return false
			}
			// 余下只允许字母/数字/下划线
			for j := 1; j < len(seg); j++ {
				ch := seg[j]
				isAlpha := (ch|0x20) >= 'a' && (ch|0x20) <= 'z'
				isDigit := ch >= '0' && ch <= '9'
				if !(isAlpha || isDigit || ch == '_') {
					return false
				}
			}
			start = i + 1
		}
	}
	return true
}

type jtype byte

const (
	jtInvalid jtype = 0
	jtString  jtype = 's'
	jtNumber  jtype = 'n'
	jtTrue    jtype = 't'
	jtFalse   jtype = 'f'
	jtNull    jtype = '0'
	jtJSON    jtype = 'j' // object/array
)

type view struct {
	raw  []byte
	kind jtype
}

func viewFromG(r gjson.Result) view {
	if len(r.Raw) == 0 {
		return view{nil, jtInvalid}
	}
	switch r.Type {
	case gjson.String:
		return view{[]byte(r.Raw), jtString}
	case gjson.Number:
		return view{[]byte(r.Raw), jtNumber}
	case gjson.True:
		return view{[]byte(r.Raw), jtTrue}
	case gjson.False:
		return view{[]byte(r.Raw), jtFalse}
	case gjson.Null:
		return view{[]byte(r.Raw), jtNull}
	default:
		// 包括 JSON（对象/数组）
		return view{[]byte(r.Raw), jtJSON}
	}
}

func AnyOrAs[T any](v any, path string, def T) T {
	if val, ok := AnyAs[T](v, path); ok {
		return val
	}
	return def
}

func AnyDataAs[T any](v any, path string) (T, bool) {
	var zero T
	r, err := GetData(v, path)
	if err != nil || len(r.Raw) == 0 {
		return zero, false
	}
	val, ok := parser.ToNative(r).(T)
	return val, ok
}

// AnyAsFast 泛型快路径（零分配常见类型）
func AnyAsFast[T any](v any, path string) (T, bool) {
	var zero T

	r, err := GetAny(v, path)
	if err != nil || len(r.Raw) == 0 {
		return zero, false
	}
	vw := viewFromG(r)

	switch any(zero).(type) {
	case string:
		if vw.kind == jtString && len(vw.raw) >= 2 {
			// 去引号（注意：不做反转义；需要反转义的场景用 AnyAs）
			s := string(vw.raw[1 : len(vw.raw)-1])
			return any(s).(T), true
		}
	case bool:
		if vw.kind == jtTrue {
			return any(true).(T), true
		}
		if vw.kind == jtFalse {
			return any(false).(T), true
		}
	case int, int64:
		if vw.kind == jtNumber && parser.IsIntegerLiteralBytes(vw.raw) {
			if i64, ok := parser.ParseIntFastBytes(vw.raw); ok {
				if _, ok2 := any(zero).(int); ok2 {
					return any(int(i64)).(T), true
				}
				return any(i64).(T), true
			}
		}
	case float64:
		if vw.kind == jtNumber {
			if parser.IsIntegerLiteralBytes(vw.raw) {
				if i64, ok := parser.ParseIntFastBytes(vw.raw); ok {
					return any(float64(i64)).(T), true
				}
			} else if f, ok := parser.ParseFloat64Bytes(vw.raw); ok {
				return any(f).(T), true
			}
		}
	}

	// 复杂类型 or 类型不匹配 → 回退
	if out, ok := parser.ToNative(r).(T); ok {
		return out, true
	}
	return zero, false
}

func AnyOrAsFast[T any](v any, path string, def T) T {
	if val, ok := AnyAsFast[T](v, path); ok {
		return val
	}
	return def
}

func AnyDataAsFast[T any](v any, path string) (T, bool) {
	var zero T
	r, err := GetData(v, path)
	if err != nil || len(r.Raw) == 0 {
		return zero, false
	}
	switch any(zero).(type) {
	case string:
		if r.Type == gjson.String {
			return any(r.String()).(T), true
		}
	case bool:
		if r.Type == gjson.True || r.Type == gjson.False {
			return any(r.Bool()).(T), true
		}
	case int64:
		if r.Type == gjson.Number && parser.IsIntegerLiteral(r.Raw) {
			if i, ok := parser.ParseIntFast(r.Raw); ok {
				return any(i).(T), true
			}
		}
	case int:
		if r.Type == gjson.Number && parser.IsIntegerLiteral(r.Raw) {
			if i, ok := parser.ParseIntFast(r.Raw); ok {
				return any(int(i)).(T), true
			}
		}
	case float64:
		if r.Type == gjson.Number {
			if parser.IsIntegerLiteral(r.Raw) {
				if i, ok := parser.ParseIntFast(r.Raw); ok {
					return any(float64(i)).(T), true
				}
			} else {
				if f, ok := parser.ParseFloat64(r.Raw); ok {
					return any(f).(T), true
				}
			}
		}
	}
	val, ok := parser.ToNative(r).(T)
	return val, ok
}

// TypeOfAny 类型检测
func TypeOfAny(v any, path string) string {
	r, err := GetAny(v, path)
	if err != nil || len(r.Raw) == 0 {
		return "null"
	}
	switch r.Type {
	case gjson.True, gjson.False:
		return "bool"
	case gjson.String:
		return "string"
	case gjson.Number:
		if parser.IsIntegerLiteral(r.Raw) {
			return "int"
		}
		return "float"
	case gjson.JSON:
		if len(r.Raw) > 0 && r.Raw[0] == '[' {
			return "array"
		}
		return "object"
	default:
		return "unknown"
	}
}

func MapAny(v any, path string) map[string]any {
	if m, ok := Any(v, path).(map[string]any); ok {
		return m
	}
	return nil
}

func ArrayAny(v any, path string) []any {
	if a, ok := Any(v, path).([]any); ok {
		return a
	}
	return nil
}

func AnyMany(v any, paths ...string) ([]any, error) {
	b, err := convert.From(v)
	if err != nil {
		return nil, err
	}
	results := gjson.GetManyBytes(b, paths...)
	out := make([]any, len(results))
	for i, r := range results {
		if len(r.Raw) == 0 {
			out[i] = nil
			continue
		}
		out[i] = parser.ToNative(r)
	}
	return out, nil
}

// EachObject 迭代器 API
func EachObject(v any, path string, fn func(k string, r gjson.Result) bool) bool {
	b, err := convert.From(v)
	if err != nil {
		return false
	}
	r := gjson.GetBytes(b, path)
	if len(r.Raw) == 0 {
		return false
	}
	ok := false
	r.ForEach(func(k, v gjson.Result) bool { ok = true; return fn(k.String(), v) })
	return ok
}

func EachArray(v any, path string, fn func(i int, r gjson.Result) bool) bool {
	b, err := convert.From(v)
	if err != nil {
		return false
	}
	r := gjson.GetBytes(b, path)
	if len(r.Raw) == 0 {
		return false
	}
	i := 0
	r.ForEach(func(_, v gjson.Result) bool { keep := fn(i, v); i++; return keep })
	return i > 0
}

func EachObjectBytes(v any, path string, fn func(keyBytes []byte, val gjson.Result) bool) bool {
	b, err := convert.From(v)
	if err != nil {
		return false
	}
	r := gjson.GetBytes(b, path)
	if len(r.Raw) == 0 {
		return false
	}
	hit := false
	r.ForEach(func(k, v gjson.Result) bool {
		if k.Index < 0 || k.Index+len(k.Raw) > len(b) {
			return true
		}
		kb := b[k.Index : k.Index+len(k.Raw)]
		if len(kb) >= 2 && kb[0] == '"' && kb[len(kb)-1] == '"' {
			kb = kb[1 : len(kb)-1]
		}
		hit = true
		return fn(kb, v)
	})
	return hit
}

func EachArrayZero(v any, path string, fn func(i int, r gjson.Result) bool) bool {
	b, err := convert.From(v)
	if err != nil {
		return false
	}
	r := gjson.GetBytes(b, path)
	if len(r.Raw) == 0 {
		return false
	}
	i := 0
	r.ForEach(func(_, v gjson.Result) bool { keep := fn(i, v); i++; return keep })
	return i > 0
}

func ForEachArrayResult(v any, path string, fn func(idx int, r gjson.Result) bool) bool {
	b, err := convert.From(v)
	if err != nil {
		return false
	}
	r := gjson.GetBytes(b, path)
	if len(r.Raw) == 0 {
		return false
	}
	i := 0
	r.ForEach(func(_, val gjson.Result) bool {
		ok := fn(i, val)
		i++
		return ok
	})
	return i > 0
}

func ForEachDataArrayResult(v any, path string, fn func(idx int, r gjson.Result) bool) bool {
	r, err := GetData(v, path)
	if err != nil || len(r.Raw) == 0 {
		return false
	}
	i := 0
	r.ForEach(func(_, val gjson.Result) bool {
		ok := fn(i, val)
		i++
		return ok
	})
	return i > 0
}

func ForEachObjectResult(v any, path string, fn func(key string, r gjson.Result) bool) bool {
	b, err := convert.From(v)
	if err != nil {
		return false
	}
	r := gjson.GetBytes(b, path)
	if len(r.Raw) == 0 {
		return false
	}
	count := 0
	r.ForEach(func(k, val gjson.Result) bool {
		count++
		return fn(k.String(), val)
	})
	return count > 0
}

// Raw 原始数据 API
func Raw(v any, path string) (string, bool) {
	return raw.Get(v, path)
}

func RawBytes(v any, path string) ([]byte, bool) {
	return raw.GetBytes(v, path)
}

func RawMany(v any, paths ...string) ([]string, error) {
	return raw.GetMany(v, paths...)
}

func RawManyBytes(v any, paths ...string) ([][]byte, error) {
	return raw.GetManyBytes(v, paths...)
}
