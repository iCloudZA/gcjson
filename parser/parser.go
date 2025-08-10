package parser

import (
	"strconv"

	"github.com/tidwall/gjson"
)

//go:nosplit
func IsIntegerLiteral(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '.' || c == 'e' || c == 'E' || c == '+' || (c == '-' && i > 0) {
			return false
		}
	}
	return true
}

//go:nosplit
func ParseIntFast(s string) (int64, bool) {
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

func ParseFloat64(s string) (float64, bool) {
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
}

func ToNative(r gjson.Result) any {
	switch r.Type {
	case gjson.Null:
		return nil
	case gjson.True, gjson.False:
		return r.Bool()
	case gjson.String:
		return r.String()
	case gjson.Number:
		if IsIntegerLiteral(r.Raw) {
			if i, ok := ParseIntFast(r.Raw); ok {
				return i
			}
		} else {
			if f, ok := ParseFloat64(r.Raw); ok {
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
				out = append(out, ToNative(it))
			}
			return out
		}
		m := make(map[string]any, 8)
		r.ForEach(func(k, v gjson.Result) bool {
			m[k.String()] = ToNative(v)
			return true
		})
		return m
	default:
		return r.Value()
	}
}

func ToNativeTyped[T any](v any) (T, bool) {
	var zero T
	if out, ok := v.(T); ok {
		return out, true
	}
	return zero, false
}

func ToNativeTypedBytes[T any](b []byte) (T, bool) {
	var zero T
	val := ToNativeBytes(b)
	if out, ok := val.(T); ok {
		return out, true
	}
	return zero, false
}
