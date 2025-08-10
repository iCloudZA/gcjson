package structfast

import (
	"reflect"
	"sync"
	"unsafe"
)

// 支持的基础类型（可按需扩展）
type Kind uint8

const (
	KindInvalid Kind = iota
	KindBool
	KindInt
	KindInt64
	KindUint
	KindUint64
	KindFloat64
	KindString
	KindStruct // 仅用于嵌套
)

type field struct {
	name   string
	kind   Kind
	offset uintptr
	// 嵌套结构体使用
	schema *Schema
}

type Schema struct {
	typ    reflect.Type
	fields map[string]field // 仅导出字段
}

var cache sync.Map // reflect.Type -> *Schema

// Compile[T]：为 T 生成字段偏移表（一次性反射，后续全 unsafe）
func Compile[T any]() *Schema {
	var zero *T
	rt := reflect.TypeOf(zero).Elem()
	if s, ok := cache.Load(rt); ok {
		return s.(*Schema)
	}
	s := buildSchema(rt)
	if s == nil {
		// 放入一个空 Schema，避免重复构建
		s = &Schema{typ: rt, fields: map[string]field{}}
	}
	actual, _ := cache.LoadOrStore(rt, s)
	return actual.(*Schema)
}

// Get：从 *T 上读取 path（"A.B.C"）对应的值，类型为 R。
// 仅第一次 Compile 使用反射；Get 本身无反射。
func Get[T any, R any](p *T, path string) (R, bool) {
	var zero R
	s := Compile[T]()
	if s == nil {
		return zero, false
	}
	val, ok := s.getAt(unsafe.Pointer(p), path)
	if !ok {
		return zero, false
	}
	// 尝试直接转换（避免接口装箱带来的性能损失）
	if v, ok := val.(R); ok {
		return v, true
	}
	return zero, false
}

// GetAny：从 *T 上读取任意值（返回 any）
func GetAny[T any](p *T, path string) (any, bool) {
	s := Compile[T]()
	if s == nil {
		return nil, false
	}
	return s.getAt(unsafe.Pointer(p), path)
}

//// ---- 内部实现 ----

// 解析路由并逐层下钻
func (s *Schema) getAt(ptr unsafe.Pointer, path string) (any, bool) {
	i := 0
	for i < len(path) {
		// 取下一段 key
		start := i
		for i < len(path) && path[i] != '.' {
			i++
		}
		seg := path[start:i]
		if i < len(path) && path[i] == '.' {
			i++
		}
		f, ok := s.fields[seg]
		if !ok {
			return nil, false
		}
		addr := unsafe.Add(ptr, f.offset)

		// 中间段必须是 struct
		if f.kind == KindStruct {
			if f.schema == nil {
				return nil, false
			}
			s = f.schema
			ptr = addr
			continue
		}

		// 如果还有剩余路径，但命中非 struct，失败
		if i < len(path) {
			return nil, false
		}
		// 终端字段读取
		return readValue(addr, f.kind), true
	}
	return nil, false
}

func buildSchema(rt reflect.Type) *Schema {
	if rt.Kind() != reflect.Struct {
		return nil
	}
	fields := make(map[string]field, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)
		// 跳过未导出字段
		if sf.PkgPath != "" {
			continue
		}
		k, sub := toKind(sf.Type)
		fd := field{
			name:   sf.Name,
			kind:   k,
			offset: sf.Offset,
			schema: sub,
		}
		fields[sf.Name] = fd
	}
	return &Schema{typ: rt, fields: fields}
}

func toKind(t reflect.Type) (Kind, *Schema) {
	switch t.Kind() {
	case reflect.Bool:
		return KindBool, nil
	case reflect.Int:
		return KindInt, nil
	case reflect.Int64:
		return KindInt64, nil
	case reflect.Uint:
		return KindUint, nil
	case reflect.Uint64:
		return KindUint64, nil
	case reflect.Float64:
		return KindFloat64, nil
	case reflect.String:
		return KindString, nil
	case reflect.Struct:
		// 嵌套结构体：递归构建一次 schema（缓存内共享）
		if s, ok := cache.Load(t); ok {
			return KindStruct, s.(*Schema)
		}
		sub := buildSchema(t)
		if sub == nil {
			sub = &Schema{typ: t, fields: map[string]field{}}
		}
		cache.Store(t, sub)
		return KindStruct, sub
	default:
		// 其他类型暂不支持（可按需扩展：time.Time、[]byte、*T、数组等）
		return KindInvalid, nil
	}
}

func readValue(addr unsafe.Pointer, k Kind) any {
	switch k {
	case KindBool:
		return *(*bool)(addr)
	case KindInt:
		return *(*int)(addr)
	case KindInt64:
		return *(*int64)(addr)
	case KindUint:
		return *(*uint)(addr)
	case KindUint64:
		return *(*uint64)(addr)
	case KindFloat64:
		return *(*float64)(addr)
	case KindString:
		return *(*string)(addr)
	default:
		return nil
	}
}
