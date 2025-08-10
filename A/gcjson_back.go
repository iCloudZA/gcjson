package A

import (
	"errors"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"unicode/utf8"
	"unsafe"

	"github.com/bytedance/sonic"
	"github.com/tidwall/gjson"
)

const maxJSONSize = 10 << 20 // 10MB

// ===== 默认下钻键 =====
var defaultDrillKeys = []string{"data"}

func SetDefaultDrillKeys(keys ...string) { defaultDrillKeys = keys }

// ===== 无锁热点路径缓存（近似 LRU，写入覆盖）=====
const hotSize = 64

var (
	hotPaths [hotSize]atomic.Value // 存 string
	hotPtr   uint32
)

//go:nosplit
func putHot(path string) {
	i := atomic.AddUint32(&hotPtr, 1)
	hotPaths[i&(hotSize-1)].Store(path)
}

//go:nosplit
func hitHot(path string) bool {
	h := fastHash(path)
	for i := 0; i < 4; i++ {
		if v := hotPaths[(h+uint32(i))&(hotSize-1)].Load(); v != nil && v.(string) == path {
			return true
		}
	}
	return false
}

// ===== 快速 hash =====
//
//go:nosplit
func fastHash(s string) uint32 {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h = (h ^ uint32(s[i])) * 16777619
	}
	return h
}

// ===== 零拷贝字符串 -> []byte（只读场景）=====
//
//go:nosplit
func unsafeStringToBytes(s string) []byte {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// ===== UTF-8 校验 =====
//
//go:nosplit
func validUTF8(b []byte) bool { return utf8.Valid(b) }

// ===== From =====
func From(v any) ([]byte, error) {
	switch x := v.(type) {
	case nil:
		return nil, errors.New("nil input")
	case string:
		b := unsafeStringToBytes(x)
		if !validUTF8(b) {
			return nil, errors.New("invalid UTF-8 string")
		}
		return b, nil
	case []byte:
		if !validUTF8(x) {
			return nil, errors.New("invalid UTF-8 bytes")
		}
		return x, nil
	case *string:
		if x == nil {
			return nil, errors.New("nil string pointer")
		}
		b := unsafeStringToBytes(*x)
		if !validUTF8(b) {
			return nil, errors.New("invalid UTF-8 string")
		}
		return b, nil
	default:
		return sonic.ConfigStd.Marshal(v)
	}
}

// ===== 极快路径：纯顶层 key（ASCII、无点/括号/通配/空格）=====
//
//go:nosplit
func isSimpleTopKey(path string) bool {
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

// 在顶层对象中 O(n) 扫描 key，命中后回落到 gjson 取子树
//
//go:nosplit
func getTopKeyFast(b []byte, key string) (gjson.Result, bool) {
	if len(b) < 2 || b[0] != '{' {
		return gjson.Result{}, false
	}
	kb := unsafeStringToBytes(key)
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

// ===== 主入口：Get with 双通道 =====
func get(b []byte, path string) gjson.Result {
	if isSimpleTopKey(path) {
		if r, ok := getTopKeyFast(b, path); ok {
			return r
		}
	}
	if !hitHot(path) {
		putHot(path)
	}
	return gjson.GetBytes(b, path)
}

// ===== Get 封装 =====
func GetAny(v any, path string) (gjson.Result, error) {
	b, err := From(v)
	if err != nil {
		return gjson.Result{}, err
	}
	if len(b) > maxJSONSize {
		return gjson.Result{}, errors.New("json too large")
	}
	return get(b, path), nil
}

func GetData(v any, path string) (gjson.Result, error) {
	core := pickData(v, defaultDrillKeys)
	b, err := From(core)
	if err != nil {
		return gjson.Result{}, err
	}
	if len(b) > maxJSONSize {
		return gjson.Result{}, errors.New("json too large")
	}
	return get(b, path), nil
}

func GetDataWithKeys(v any, keys []string, path string) (gjson.Result, error) {
	core := pickData(v, keys)
	b, err := From(core)
	if err != nil {
		return gjson.Result{}, err
	}
	if len(b) > maxJSONSize {
		return gjson.Result{}, errors.New("json too large")
	}
	return get(b, path), nil
}

// ===== 自动类型推断 =====
func Any(v any, path string) any {
	r, err := GetAny(v, path)
	if err != nil || len(r.Raw) == 0 {
		return nil
	}
	return toNative(r)
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
	return toNative(r)
}
func AnyDataWithKeys(v any, keys []string, path string) any {
	r, err := GetDataWithKeys(v, keys, path)
	if err != nil || len(r.Raw) == 0 {
		return nil
	}
	return toNative(r)
}

// ===== 泛型直达 =====
func AnyAs[T any](v any, path string) (T, bool) {
	var zero T
	r, err := GetAny(v, path)
	if err != nil || len(r.Raw) == 0 {
		return zero, false
	}
	val, ok := toNative(r).(T)
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
	val, ok := toNative(r).(T)
	return val, ok
}

// ===== 泛型快路径（零分配常见类型）=====
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
		if r.Type == gjson.Number && isIntegerLiteral(r.Raw) {
			if i, ok := parseIntFast(r.Raw); ok {
				return any(i).(T), true
			}
		}
	case int:
		if r.Type == gjson.Number && isIntegerLiteral(r.Raw) {
			if i, ok := parseIntFast(r.Raw); ok {
				return any(int(i)).(T), true
			}
		}
	case float64:
		if r.Type == gjson.Number {
			if isIntegerLiteral(r.Raw) {
				if i, ok := parseIntFast(r.Raw); ok {
					return any(float64(i)).(T), true
				}
			} else {
				if f, ok := parseFloat64(r.Raw); ok {
					return any(f).(T), true
				}
			}
		}
	}
	val, ok := toNative(r).(T)
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
		if r.Type == gjson.Number && isIntegerLiteral(r.Raw) {
			if i, ok := parseIntFast(r.Raw); ok {
				return any(i).(T), true
			}
		}
	case int:
		if r.Type == gjson.Number && isIntegerLiteral(r.Raw) {
			if i, ok := parseIntFast(r.Raw); ok {
				return any(int(i)).(T), true
			}
		}
	case float64:
		if r.Type == gjson.Number {
			if isIntegerLiteral(r.Raw) {
				if i, ok := parseIntFast(r.Raw); ok {
					return any(float64(i)).(T), true
				}
			} else {
				if f, ok := parseFloat64(r.Raw); ok {
					return any(f).(T), true
				}
			}
		}
	}
	val, ok := toNative(r).(T)
	return val, ok
}

// ===== 类型检测 =====
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
		if isIntegerLiteral(r.Raw) {
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

func EachObject(v any, path string, fn func(k string, r gjson.Result) bool) bool {
	r, err := GetAny(v, path)
	if err != nil || len(r.Raw) == 0 {
		return false
	}
	ok := false
	r.ForEach(func(k, v gjson.Result) bool { ok = true; return fn(k.String(), v) })
	return ok
}

func ArrayAny(v any, path string) []any {
	if a, ok := Any(v, path).([]any); ok {
		return a
	}
	return nil
}

func EachArray(v any, path string, fn func(i int, r gjson.Result) bool) bool {
	r, err := GetAny(v, path)
	if err != nil || len(r.Raw) == 0 {
		return false
	}
	i := 0
	r.ForEach(func(_, v gjson.Result) bool { keep := fn(i, v); i++; return keep })
	return i > 0
}

func EachObjectBytes(v any, path string, fn func(keyBytes []byte, val gjson.Result) bool) bool {
	b, err := From(v)
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
	b, err := From(v)
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

// ===== 数字解析（零分配）=====
//
//go:nosplit
func isIntegerLiteral(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '.' || c == 'e' || c == 'E' || c == '+' || (c == '-' && i > 0) {
			return false
		}
	}
	return true
}

//go:nosplit
func parseIntFast(s string) (int64, bool) {
	if len(s) == 0 {
		return 0, false
	}
	i := 0
	neg := false
	if s[0] == '-' {
		neg, i = true, 1
		if len(s) == 1 {
			return 0, false
		}
	}
	var n int64
	for ; i < len(s); i++ {
		c := s[i] - '0'
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
func parseFloat64(s string) (float64, bool) {
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
}

// ===== gjson.Result -> 原生类型（最小化分配）=====
func toNative(r gjson.Result) any {
	switch r.Type {
	case gjson.Null:
		return nil
	case gjson.True, gjson.False:
		return r.Bool()
	case gjson.String:
		return r.String()
	case gjson.Number:
		if isIntegerLiteral(r.Raw) {
			if i, ok := parseIntFast(r.Raw); ok {
				return i
			}
		} else {
			if f, ok := parseFloat64(r.Raw); ok {
				return f
			}
		}
		return r.Raw
	case gjson.JSON:
		raw := r.Raw
		if len(raw) > 0 && raw[0] == '[' {
			arr := r.Array()
			out := make([]any, 0, len(arr))
			for _, it := range arr {
				out = append(out, toNative(it))
			}
			return out
		}
		m := make(map[string]any, 8)
		r.ForEach(func(k, v gjson.Result) bool {
			m[k.String()] = toNative(v)
			return true
		})
		return m
	default:
		return r.Value()
	}
}

// ===== pickData：优先无反射路径 =====
type structInfo struct {
	fieldMap map[string]int
}

var structCache sync.Map // map[reflect.Type]*structInfo

//go:nosplit
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		b := make([]byte, len(s))
		b[0] = s[0] - 32
		copy(b[1:], s[1:])
		return string(b)
	}
	return s
}

func pickData(v any, keys []string) any {
	if v == nil {
		return nil
	}

	// 快路径：map[string]any
	if m, ok := v.(map[string]any); ok {
		for _, k := range keys {
			if val, ok := m[k]; ok {
				return val
			}
			if val, ok := m[capitalizeFirst(k)]; ok {
				return val
			}
		}
		return v
	}

	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.Map:
		iter := rv.MapRange()
		for iter.Next() {
			k := iter.Key()
			if k.Kind() != reflect.String {
				continue
			}
			ks := k.String()
			for _, key := range keys {
				if ks == key || ks == capitalizeFirst(key) {
					return iter.Value().Interface()
				}
			}
		}
		return v
	case reflect.Struct:
		rt := rv.Type()
		if cached, ok := structCache.Load(rt); ok {
			si := cached.(*structInfo)
			for _, k := range keys {
				if idx, ok := si.fieldMap[k]; ok {
					return rv.Field(idx).Interface()
				}
			}
		} else {
			si := &structInfo{fieldMap: make(map[string]int, len(keys))}
			for i := 0; i < rt.NumField(); i++ {
				sf := rt.Field(i)
				name := sf.Name
				tag := sf.Tag.Get("json")
				for _, k := range keys {
					if name == capitalizeFirst(k) || tag == k || tagHasName(tag, k) {
						si.fieldMap[k] = i
					}
				}
			}
			structCache.Store(rt, si)
			for _, k := range keys {
				if idx, ok := si.fieldMap[k]; ok {
					return rv.Field(idx).Interface()
				}
			}
		}
		return v
	default:
		return v
	}
}

//go:nosplit
func tagHasName(tag, name string) bool {
	if tag == "" {
		return false
	}
	for i := 0; i < len(tag); i++ {
		if tag[i] == ',' {
			return tag[:i] == name
		}
	}
	return tag == name
}

func AnyMany(v any, paths ...string) ([]any, error) {
	b, err := From(v)
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
		out[i] = toNative(r)
	}
	return out, nil
}

func RawMany(v any, paths ...string) ([]string, error) {
	b, err := From(v)
	if err != nil {
		return nil, err
	}
	results := gjson.GetManyBytes(b, paths...)
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.Raw
	}
	return out, nil
}

// ===== 原始片段 =====
func Raw(v any, path string) (string, bool) {
	r, err := GetAny(v, path)
	if err != nil || len(r.Raw) == 0 {
		return "", false
	}
	return r.Raw, true
}

// 零拷贝原始字节（尽量）——优先用 Index 切原始 []byte
func RawBytes(v any, path string) ([]byte, bool) {
	b, err := From(v)
	if err != nil {
		return nil, false
	}
	r := gjson.GetBytes(b, path)
	if len(r.Raw) == 0 {
		return nil, false
	}
	if r.Index >= 0 && r.Index+len(r.Raw) <= len(b) {
		return b[r.Index : r.Index+len(r.Raw)], true
	}
	return []byte(r.Raw), true
}

func RawManyBytes(v any, paths ...string) ([][]byte, error) {
	b, err := From(v)
	if err != nil {
		return nil, err
	}
	rs := gjson.GetManyBytes(b, paths...)
	out := make([][]byte, len(rs))
	for i, r := range rs {
		if len(r.Raw) == 0 {
			continue
		}
		if r.Index >= 0 && r.Index+len(r.Raw) <= len(b) {
			out[i] = b[r.Index : r.Index+len(r.Raw)]
		} else {
			out[i] = []byte(r.Raw)
		}
	}
	return out, nil
}

// ===== 迭代器 =====
func ForEachArrayResult(v any, path string, fn func(idx int, r gjson.Result) bool) bool {
	r, err := GetAny(v, path)
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
	r, err := GetAny(v, path)
	if err != nil || len(r.Raw) == 0 {
		return false
	}
	count := 0
	r.ForEach(func(k, val gjson.Result) bool {
		count++
		return fn(k.String(), val)
	})
	return count > 0
}

// ===== 可选：在 init 中关闭 modifiers，进一步省解析成本 =====
func init() {
	gjson.DisableModifiers = true
}
