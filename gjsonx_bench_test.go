package gcjson

import (
	"fmt"
	"github.com/iCloudZA/gcjson/picker"
	"github.com/tidwall/gjson"
	"testing"
)

var sampleJSON = []byte(`{
	"data": {
		"id": 1234567890123456789,
		"name": "hello world",
		"nested": {
			"flag": true,
			"items": [1, 2, 3, 4, 5],
			"deep": {
				"layer1": {
					"layer2": {
						"layer3": {
							"value": "deep value",
							"number": 9876543210.12345,
							"bool": false,
							"nullval": null,
							"array": [
								{"name": "obj1", "score": 99.9},
								{"name": "obj2", "score": 88.8},
								{"name": "obj3", "score": 77.7}
							]
						}
					}
				}
			}
		}
	},
	"meta": {
		"version": "1.0.0",
		"authors": ["Alice", "Bob", "Charlie"],
		"config": {
			"features": {
				"enableX": true,
				"enableY": false,
				"threshold": 0.85
			},
			"limits": {
				"max_users": 1000000,
				"timeout": "30s",
				"regions": ["us-east", "eu-west", "ap-southeast"]
			}
		}
	},
	"logs": [
		{"timestamp": "2025-08-10T14:00:00Z", "level": "INFO", "message": "System started"},
		{"timestamp": "2025-08-10T14:05:00Z", "level": "WARN", "message": "High memory usage"},
		{"timestamp": "2025-08-10T14:10:00Z", "level": "ERROR", "message": "Out of memory"}
	],
	"extra": {
		"escaped": "Line1\\nLine2\\tTabbed",
		"unicode": "测试😊漢字"
	}
}`)

var sinkInt int

// -------------------- Benchmark 基础功能 --------------------
func BenchmarkGetAny(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = GetAny(sampleJSON, "data.name")
	}
}

func BenchmarkAny(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Any(sampleJSON, "data.id")
	}
}

func BenchmarkAnyData(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = AnyData(sampleJSON, "nested.items.3")
	}
}

func BenchmarkTypeOfAny(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = TypeOfAny(sampleJSON, "data.id")
	}
}

func BenchmarkMapAny(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		n := 0
		EachObjectBytes(sampleJSON, "data.nested", func(kb []byte, r gjson.Result) bool {
			// 轻量聚合，防止 DCE；不转 string
			n += len(kb) + len(r.Raw)
			return true
		})
		sinkInt = n
	}
}

func BenchmarkArrayAny(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		n := 0
		EachArrayZero(sampleJSON, "data.nested.items", func(idx int, r gjson.Result) bool {
			n += idx + len(r.Raw)
			return true
		})
		sinkInt = n
	}
}

// -------------------- Benchmark 深路径测试 --------------------
func BenchmarkDeepValue(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = AnyData(sampleJSON, "nested.deep.layer1.layer2.layer3.value")
	}
}

func BenchmarkDeepNumber(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = AnyData(sampleJSON, "nested.deep.layer1.layer2.layer3.number")
	}
}

func BenchmarkDeepArrayScore(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = AnyData(sampleJSON, "nested.deep.layer1.layer2.layer3.array.2.score")
	}
}

func BenchmarkDeepNull(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = AnyData(sampleJSON, "nested.deep.layer1.layer2.layer3.nullval")
	}
}

// -------------------- Benchmark pickData 对比 --------------------
func BenchmarkPickData_Default(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = picker.PickData(map[string]any{"data": 123}, []string{"data"})
	}
}

type testStruct struct {
	Data int `json:"data"`
}

func BenchmarkPickData_Struct(b *testing.B) {
	ts := testStruct{Data: 456}
	for i := 0; i < b.N; i++ {
		_ = picker.PickData(ts, []string{"data"})
	}
}

// 综合测试
func BenchmarkAll(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = AnyData(sampleJSON, "nested.items.2")
		_ = TypeOfAny(sampleJSON, "data.name")
		_ = MapAny(sampleJSON, "data.nested")
		_ = ArrayAny(sampleJSON, "data.nested.items")
		_ = AnyData(sampleJSON, "nested.deep.layer1.layer2.layer3.array.1.name")
	}
}

// -------------------- 示例 --------------------
func Example_usage() {
	fmt.Println(Any(sampleJSON, "data.nested.flag"))
	// Output: true
}
