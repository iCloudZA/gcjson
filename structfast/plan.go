package structfast

import (
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
)

type fieldPlan struct {
	index int      // struct 字段索引
	path  []string // JSON 路径（支持 a.b.c）
	kind  reflect.Kind
}

type typePlan struct {
	fields []fieldPlan
}

var (
	cachePtr atomic.Pointer[map[reflect.Type]*typePlan]
	mu       sync.Mutex
)

func Register[T any]() {
	var zero T
	t := reflect.TypeOf(&zero)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	_ = getTypePlan(t)
}

func getTypePlan(t reflect.Type) *typePlan {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	m := cachePtr.Load()
	if m != nil {
		if p, ok := (*m)[t]; ok {
			return p
		}
	}
	mu.Lock()
	defer mu.Unlock()

	m = cachePtr.Load()
	if m != nil {
		if p, ok := (*m)[t]; ok {
			return p
		}
	}

	p := buildTypePlan(t)
	newMap := make(map[reflect.Type]*typePlan, 8)
	if m != nil {
		for k, v := range *m {
			newMap[k] = v
		}
	}
	newMap[t] = p
	cachePtr.Store(&newMap)
	return p
}

func buildTypePlan(t reflect.Type) *typePlan {
	n := t.NumField()
	fp := make([]fieldPlan, 0, n)
	for i := 0; i < n; i++ {
		f := t.Field(i)
		if f.PkgPath != "" { // 非导出
			continue
		}
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name := tag
		if name == "" {
			name = f.Name
		} else if c := strings.IndexByte(name, ','); c >= 0 {
			name = name[:c]
		}
		if name == "" {
			name = f.Name
		}
		path := strings.Split(name, ".") // 支持“a.b.c”
		fp = append(fp, fieldPlan{
			index: i,
			path:  path,
			kind:  f.Type.Kind(),
		})
	}
	return &typePlan{fields: fp}
}
