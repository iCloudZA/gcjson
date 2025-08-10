package gcjson

import (
	"errors"
	"github.com/iCloudZA/gcjson/cache"
	"github.com/iCloudZA/gcjson/convert"
	"github.com/iCloudZA/gcjson/fast"
	"github.com/iCloudZA/gcjson/iterator"
	"github.com/iCloudZA/gcjson/parser"
	"github.com/iCloudZA/gcjson/picker"
	"github.com/iCloudZA/gcjson/raw"

	"github.com/tidwall/gjson"
)

type Result = gjson.Result

func init() {
	gjson.DisableModifiers = true
}

// SetDefaultDrillKeys 默认下钻键配置
func SetDefaultDrillKeys(keys ...string) {
	picker.SetDefaultDrillKeys(keys...)
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

// AnyAs 泛型直达
func AnyAs[T any](v any, path string) (T, bool) {
	var zero T
	r, err := GetAny(v, path)
	if err != nil || len(r.Raw) == 0 {
		return zero, false
	}
	val, ok := parser.ToNative(r).(T)
	return val, ok
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
	return iterator.EachObject(v, path, fn)
}

func EachArray(v any, path string, fn func(i int, r gjson.Result) bool) bool {
	return iterator.EachArray(v, path, fn)
}

func EachObjectBytes(v any, path string, fn func(keyBytes []byte, val gjson.Result) bool) bool {
	return iterator.EachObjectBytes(v, path, fn)
}

func EachArrayZero(v any, path string, fn func(i int, r gjson.Result) bool) bool {
	return iterator.EachArrayZero(v, path, fn)
}

func ForEachArrayResult(v any, path string, fn func(idx int, r gjson.Result) bool) bool {
	return iterator.ForEachArrayResult(v, path, fn)
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
	return iterator.ForEachObjectResult(v, path, fn)
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
