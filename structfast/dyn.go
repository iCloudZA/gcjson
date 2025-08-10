package structfast

import (
	"reflect"
	"unsafe"
)

// GetByPtr 动态类型版：ptr 必须是 *struct，path 如 "A.B.C"。
// 成功返回字段值（any）与 true；否则 (nil, false)。
// 仅首次编译该结构体的 Schema 时用一次反射，之后全走 unsafe。
func GetByPtr(ptr any, path string) (any, bool) {
	rv := reflect.ValueOf(ptr)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return nil, false
	}
	el := rv.Elem()
	if el.Kind() != reflect.Struct {
		return nil, false
	}
	rt := el.Type()

	// Schema 缓存里拿（Compile 会缓存）
	var s *Schema
	if cached, ok := cache.Load(rt); ok {
		s = cached.(*Schema)
	} else {
		s = buildSchema(rt)
		if s == nil {
			s = &Schema{typ: rt, fields: map[string]field{}}
		}
		cache.Store(rt, s)
	}
	return s.getAt(unsafe.Pointer(el.UnsafeAddr()), path)
}
