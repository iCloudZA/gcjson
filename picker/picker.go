package picker

import (
	"reflect"
	"sync"
)

var defaultDrillKeys = []string{"data"}

func SetDefaultDrillKeys(keys ...string) {
	defaultDrillKeys = keys
}

func GetDefaultDrillKeys() []string {
	return defaultDrillKeys
}

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

func PickData(v any, keys []string) any {
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
