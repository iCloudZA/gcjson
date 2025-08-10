package structfast

import (
	"reflect"
	"sync"
	"time"
	"unsafe"
)

type Kind uint8

const (
	KindInvalid Kind = iota
	// 标量
	KindBool
	KindInt
	KindInt8
	KindInt16
	KindInt32
	KindInt64
	KindUint
	KindUint8
	KindUint16
	KindUint32
	KindUint64
	KindUintptr
	KindFloat32
	KindFloat64
	KindComplex64
	KindComplex128
	KindString
	KindBytes // []byte
	KindIface // interface{}
	KindUnsafePtr
	// 复合
	KindPtr
	KindStruct // 用 schema 递归
	KindOther  // 其他复杂类型（slice/map/array/别名等），用构建期生成的 reader
)

type field struct {
	name   string
	kind   Kind
	offset uintptr
	schema *Schema                  // 仅 KindStruct / KindPtr-to-Struct 使用
	elem   *fieldElem               // KindPtr/Bytes 的元素信息或特殊处理
	reader func(unsafe.Pointer) any // 读取器（热路径零反射）
}

type fieldElem struct {
	kind   Kind
	schema *Schema
	// 对于 OTHER 类型，保留反射类型用于构建期生成 reader
	rtype reflect.Type
}

type Schema struct {
	typ    reflect.Type
	fields map[string]field // 导出字段
}

var cache sync.Map // reflect.Type -> *Schema

// ===== 对外 API =====

func Compile[T any]() *Schema {
	var zero *T
	rt := reflect.TypeOf(zero).Elem()
	if s, ok := cache.Load(rt); ok {
		return s.(*Schema)
	}
	s := buildSchema(rt)
	if s == nil {
		s = &Schema{typ: rt, fields: map[string]field{}}
	}
	actual, _ := cache.LoadOrStore(rt, s)
	return actual.(*Schema)
}

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
	if v, ok := val.(R); ok {
		return v, true
	}
	return zero, false
}

func GetByPtrTyped[T any](v any, path string) (T, bool) {
	var zero T
	pv, ok := v.(**T) // 用双指针避免反射取地址
	if !ok {
		ptr, ok2 := v.(*T)
		if !ok2 {
			return zero, false
		}
		pv = &ptr
	}
	val, ok := Compile[T]().getAt(unsafe.Pointer(*pv), path)
	if !ok {
		return zero, false
	}
	if out, ok := val.(T); ok {
		return out, true
	}
	return zero, false
}

func GetAny[T any](p *T, path string) (any, bool) {
	s := Compile[T]()
	if s == nil {
		return nil, false
	}
	return s.getAt(unsafe.Pointer(p), path)
}

// ===== 内部实现 =====

func (s *Schema) getAt(ptr unsafe.Pointer, path string) (any, bool) {
	i := 0
	for i < len(path) {
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

		// 中间段需要能“下钻”
		switch f.kind {
		case KindStruct:
			if f.schema == nil {
				return nil, false
			}
			s = f.schema
			ptr = addr
			continue
		case KindPtr:
			// 自动解引用
			p := *(*unsafe.Pointer)(addr)
			if p == nil {
				return nil, false
			}
			// 指向 struct：可继续下钻
			if f.elem != nil && f.elem.kind == KindStruct && f.elem.schema != nil {
				s = f.elem.schema
				ptr = p
				continue
			}
			// 指向标量：如果后续还有路径，则失败
			if i < len(path) {
				return nil, false
			}
			// 终端：读出指针目标
			return f.reader(unsafe.Pointer(&p)), true
		default:
			// 非可下钻类型，如果还有剩余路径 -> 失败
			if i < len(path) {
				return nil, false
			}
			return f.reader(addr), true
		}
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
		if sf.PkgPath != "" {
			continue // 非导出字段
		}
		f := makeField(sf)
		fields[sf.Name] = f
	}
	return &Schema{typ: rt, fields: fields}
}

func makeField(sf reflect.StructField) field {
	t := sf.Type
	off := sf.Offset

	// 特判常见类型以便零开销读取
	switch t.Kind() {
	case reflect.Bool:
		return field{sf.Name, KindBool, off, nil, nil, func(p unsafe.Pointer) any { return *(*bool)(p) }}
	case reflect.Int:
		return field{sf.Name, KindInt, off, nil, nil, func(p unsafe.Pointer) any { return *(*int)(p) }}
	case reflect.Int8:
		return field{sf.Name, KindInt8, off, nil, nil, func(p unsafe.Pointer) any { return *(*int8)(p) }}
	case reflect.Int16:
		return field{sf.Name, KindInt16, off, nil, nil, func(p unsafe.Pointer) any { return *(*int16)(p) }}
	case reflect.Int32:
		return field{sf.Name, KindInt32, off, nil, nil, func(p unsafe.Pointer) any { return *(*int32)(p) }}
	case reflect.Int64:
		// time.Time 也是 struct，不走这里
		return field{sf.Name, KindInt64, off, nil, nil, func(p unsafe.Pointer) any { return *(*int64)(p) }}
	case reflect.Uint:
		return field{sf.Name, KindUint, off, nil, nil, func(p unsafe.Pointer) any { return *(*uint)(p) }}
	case reflect.Uint8:
		return field{sf.Name, KindUint8, off, nil, nil, func(p unsafe.Pointer) any { return *(*uint8)(p) }}
	case reflect.Uint16:
		return field{sf.Name, KindUint16, off, nil, nil, func(p unsafe.Pointer) any { return *(*uint16)(p) }}
	case reflect.Uint32:
		return field{sf.Name, KindUint32, off, nil, nil, func(p unsafe.Pointer) any { return *(*uint32)(p) }}
	case reflect.Uint64:
		return field{sf.Name, KindUint64, off, nil, nil, func(p unsafe.Pointer) any { return *(*uint64)(p) }}
	case reflect.Uintptr:
		return field{sf.Name, KindUintptr, off, nil, nil, func(p unsafe.Pointer) any { return *(*uintptr)(p) }}
	case reflect.Float32:
		return field{sf.Name, KindFloat32, off, nil, nil, func(p unsafe.Pointer) any { return *(*float32)(p) }}
	case reflect.Float64:
		return field{sf.Name, KindFloat64, off, nil, nil, func(p unsafe.Pointer) any { return *(*float64)(p) }}
	case reflect.Complex64:
		return field{sf.Name, KindComplex64, off, nil, nil, func(p unsafe.Pointer) any { return *(*complex64)(p) }}
	case reflect.Complex128:
		return field{sf.Name, KindComplex128, off, nil, nil, func(p unsafe.Pointer) any { return *(*complex128)(p) }}
	case reflect.String:
		return field{sf.Name, KindString, off, nil, nil, func(p unsafe.Pointer) any { return *(*string)(p) }}
	case reflect.Slice:
		// 专门优化 []byte
		if t.Elem().Kind() == reflect.Uint8 {
			return field{sf.Name, KindBytes, off, nil, &fieldElem{kind: KindUint8}, func(p unsafe.Pointer) any {
				return *(*[]byte)(p)
			}}
		}
		// 其他 slice：构建期用反射生成 reader（读时无反射）
		rt := t
		reader := func(p unsafe.Pointer) any {
			// 用 reflect.NewAt 构造只读接口（读取阶段不再构建）
			return reflect.NewAt(rt, p).Elem().Interface()
		}
		return field{sf.Name, KindOther, off, nil, &fieldElem{kind: KindOther, rtype: rt}, reader}
	case reflect.Array, reflect.Map:
		rt := t
		reader := func(p unsafe.Pointer) any { return reflect.NewAt(rt, p).Elem().Interface() }
		return field{sf.Name, KindOther, off, nil, &fieldElem{kind: KindOther, rtype: rt}, reader}
	case reflect.Interface:
		// 直接取接口槽
		return field{sf.Name, KindIface, off, nil, nil, func(p unsafe.Pointer) any { return *(*interface{})(p) }}
	case reflect.Pointer:
		et := t.Elem()
		// *struct：建立子 schema，可下钻
		if et.Kind() == reflect.Struct && !isTimeType(et) {
			sub := lookupOrBuild(et)
			reader := func(p unsafe.Pointer) any {
				pp := *(*unsafe.Pointer)(p)
				if pp == nil {
					return nil
				}
				// 返回 *struct 作为 any
				return reflect.NewAt(et, pp).Elem().Addr().Interface()
			}
			return field{sf.Name, KindPtr, off, sub, &fieldElem{kind: KindStruct, schema: sub}, reader}
		}
		// 其他指针：构建期生成 reader；读时自动解引用返回目标值
		rt := et
		reader := func(p unsafe.Pointer) any {
			pp := *(*unsafe.Pointer)(p)
			if pp == nil {
				return nil
			}
			return reflect.NewAt(rt, pp).Elem().Interface()
		}
		return field{sf.Name, KindPtr, off, nil, &fieldElem{kind: kindOfReflect(et), rtype: rt}, reader}
	case reflect.Struct:
		// time.Time 专判
		if isTimeType(t) {
			reader := func(p unsafe.Pointer) any { return *(*time.Time)(p) }
			return field{sf.Name, KindOther, off, nil, &fieldElem{rtype: t}, reader}
		}
		sub := lookupOrBuild(t)
		return field{sf.Name, KindStruct, off, sub, nil, func(p unsafe.Pointer) any {
			// 返回 struct 值（拷贝一份结构体值到接口，零反射）
			// 用反射构造接口仍需要 reflect；此处保持简单返回 struct 值：
			return reflect.NewAt(t, p).Elem().Interface()
		}}
	case reflect.UnsafePointer:
		return field{sf.Name, KindUnsafePtr, off, nil, nil, func(p unsafe.Pointer) any { return *(*unsafe.Pointer)(p) }}
	default:
		// 其他别名/复杂类型：用构建期生成的 reader；读取时无反射逻辑分支
		rt := t
		reader := func(p unsafe.Pointer) any { return reflect.NewAt(rt, p).Elem().Interface() }
		return field{sf.Name, KindOther, off, nil, &fieldElem{rtype: rt}, reader}
	}
}

func lookupOrBuild(t reflect.Type) *Schema {
	if s, ok := cache.Load(t); ok {
		return s.(*Schema)
	}
	sub := buildSchema(t)
	if sub == nil {
		sub = &Schema{typ: t, fields: map[string]field{}}
	}
	cache.Store(t, sub)
	return sub
}

func isTimeType(t reflect.Type) bool {
	// 规避反射包 pathplan 比较带来的分配，这里直接比对指针地址也可以，
	// 为可读性使用包名 + 名称判断
	return t.PkgPath() == "time" && t.Name() == "Time"
}

func kindOfReflect(t reflect.Type) Kind {
	switch t.Kind() {
	case reflect.Bool:
		return KindBool
	case reflect.Int:
		return KindInt
	case reflect.Int8:
		return KindInt8
	case reflect.Int16:
		return KindInt16
	case reflect.Int32:
		return KindInt32
	case reflect.Int64:
		return KindInt64
	case reflect.Uint:
		return KindUint
	case reflect.Uint8:
		return KindUint8
	case reflect.Uint16:
		return KindUint16
	case reflect.Uint32:
		return KindUint32
	case reflect.Uint64:
		return KindUint64
	case reflect.Uintptr:
		return KindUintptr
	case reflect.Float32:
		return KindFloat32
	case reflect.Float64:
		return KindFloat64
	case reflect.Complex64:
		return KindComplex64
	case reflect.Complex128:
		return KindComplex128
	case reflect.String:
		return KindString
	case reflect.Interface:
		return KindIface
	case reflect.Struct:
		return KindStruct
	default:
		return KindOther
	}
}
