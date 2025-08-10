package pathplan

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

type seg struct {
	Key string // 对象键；与 idx 互斥
	Idx int    // 数组索引；>=0 有效
}

type Plan struct {
	Segs []seg
}

// ===== 小对象池，减少切片分配 =====
var segPool = sync.Pool{New: func() any { b := make([]seg, 0, 8); return &b }}

func putSegs(p *Plan) {
	if p == nil || cap(p.Segs) > 64 {
		return
	}
	b := p.Segs[:0]
	p.Segs = nil
	segPool.Put(&b)
}

// ===== 热缓存：无锁读，多写覆盖 =====
const hotCap = 512

type hotEntry struct {
	key  string
	plan *Plan
}

var (
	hot     [hotCap]atomic.Pointer[hotEntry]
	hotNext uint64
)

func loadHot(k string) *Plan {
	// 4 路探测，降低碰撞
	h := fnv1a(k)
	for i := 0; i < 4; i++ {
		slot := int((h + uint32(i)*0x9e3779b9) & (hotCap - 1))
		if e := hot[slot].Load(); e != nil && e.key == k {
			return e.plan
		}
	}
	return nil
}

func storeHot(k string, p *Plan) {
	i := atomic.AddUint64(&hotNext, 1)
	slot := int(i & (hotCap - 1))
	hot[slot].Store(&hotEntry{key: k, plan: p})
}

func fnv1a(s string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

// ===== 编译器 =====

func Compile(path string) *Plan {
	if path == "" {
		return &Plan{}
	}
	if p := loadHot(path); p != nil {
		return p
	}

	// 解析到 seg 池
	sb := segPool.Get().(*[]seg)
	segs := (*sb)[:0]

	start := 0
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == '.' {
			if i == start { // 空段
				segs = append(segs, seg{Key: ""})
			} else {
				token := path[start:i]
				// 纯数字 → 数组索引
				if n, ok := atoiDigits(token); ok {
					segs = append(segs, seg{Idx: n})
				} else {
					segs = append(segs, seg{Key: token})
				}
			}
			start = i + 1
		}
	}

	plan := &Plan{Segs: make([]seg, len(segs))}
	copy(plan.Segs, segs)

	// 归还解析临时切片
	*sb = segs[:0]
	segPool.Put(sb)

	storeHot(path, plan)
	return plan
}

func atoiDigits(s string) (int, bool) {
	if len(s) == 0 {
		return 0, false
	}
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i] - '0'
		if c > 9 {
			return 0, false
		}
		n = n*10 + int(c)
	}
	return n, true
}

// ===== 批量编译 =====
func CompileMany(paths []string) []*Plan {
	out := make([]*Plan, len(paths))
	for i, p := range paths {
		out[i] = Compile(p)
	}
	return out
}

// ===== 调试辅助（可选）=====
func (p *Plan) String() string {
	var b strings.Builder
	for i, s := range p.Segs {
		if i > 0 {
			b.WriteByte('.')
		}
		if s.Key != "" {
			b.WriteString(s.Key)
		} else {
			b.WriteString(strconv.Itoa(s.Idx))
		}
	}
	return b.String()
}
