package raw_test

import (
	"fmt"
	"testing"

	"github.com/icloudza/gcjson/raw"
)

var sampleJSON = []byte(`{
	"logs": [
		{"level": "info", "msg": "start"},
		{"level": "error", "msg": "fail"}
	],
	"data": {
		"user": {
			"name": "Alice",
			"age": 30
		}
	},
	"meta": {
		"version": "1.2.3"
	}
}`)

func TestGet(t *testing.T) {
	s, ok := raw.Get(sampleJSON, "logs.0.level")
	fmt.Println("logs.0.level =", s, ok)
	if !ok || s != `"info"` {
		t.Fatalf("Get logs.0.level failed, got=%q, ok=%v", s, ok)
	}

	bs, ok := raw.GetBytes(sampleJSON, "data.user.name")
	fmt.Println("data.user.name =", string(bs), ok)
	if !ok || string(bs) != `"Alice"` {
		t.Fatalf("GetBytes data.user.name failed, got=%q, ok=%v", string(bs), ok)
	}

	// Not found
	_, ok = raw.Get(sampleJSON, "data.user.nope")
	fmt.Println("data.user.nope found?", ok)
	if ok {
		t.Fatalf("expected not found")
	}
}

func TestGetMany(t *testing.T) {
	ss, err := raw.GetMany(sampleJSON, "meta.version", "data.user.age")
	fmt.Println("GetMany =", ss, err)
	if err != nil {
		t.Fatal(err)
	}
	if ss[0] != `"1.2.3"` || ss[1] != "30" {
		t.Fatalf("unexpected GetMany results: %#v", ss)
	}

	bss, err := raw.GetManyBytes(sampleJSON, "logs.1.msg", "logs.0.msg")
	fmt.Println("GetManyBytes =", string(bss[0]), string(bss[1]), err)
	if err != nil {
		t.Fatal(err)
	}
	if string(bss[0]) != `"fail"` || string(bss[1]) != `"start"` {
		t.Fatalf("unexpected GetManyBytes results: %#v", bss)
	}
}

// ===== 性能测试 =====

func BenchmarkGetMany(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		raw.GetMany(sampleJSON, "meta.version", "data.user.age")
	}
}

func BenchmarkGetManyBytes(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		raw.GetManyBytes(sampleJSON, "meta.version", "data.user.age")
	}
}

func Benchmark_GetBytesByPlan(b *testing.B) {
	doc := sampleJSON
	pl := raw.CompilePath("data.nested.deep.layer1.layer2.layer3.value")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = raw.GetBytesByPlan(doc, pl)
	}
}
