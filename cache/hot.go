package cache

import (
	"sync/atomic"
)

const HotSize = 64

var (
	hotPaths [HotSize]atomic.Value // 存 string
	hotPtr   uint32
)

//go:nosplit
func PutHot(path string) {
	i := atomic.AddUint32(&hotPtr, 1)
	hotPaths[i&(HotSize-1)].Store(path)
}

//go:nosplit
func HitHot(path string) bool {
	h := fastHash(path)
	for i := 0; i < 4; i++ {
		if v := hotPaths[(h+uint32(i))&(HotSize-1)].Load(); v != nil && v.(string) == path {
			return true
		}
	}
	return false
}

//go:nosplit
func fastHash(s string) uint32 {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h = (h ^ uint32(s[i])) * 16777619
	}
	return h
}
