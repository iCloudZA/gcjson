package zeronode

import (
	"testing"

	"github.com/tidwall/gjson"
)

// 测试 JSON 样本
var sampleJSON = []byte(`{
	"id": 123456789,
	"name": "hello world",
	"flag": true,
	"price": 99.99,
	"nested": {
		"layer1": {
			"layer2": {
				"value": "deep value",
				"number": 9876543210,
				"bool": false,
				"array": [
					{"name": "obj1", "score": 99.9},
					{"name": "obj2", "score": 88.8},
					{"name": "obj3", "score": 77.7}
				]
			}
		}
	}
}`)

func BenchmarkZeronodeGet(b *testing.B) {
	node := FromBytes(sampleJSON)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = node.Get("name").String()
	}
}

func BenchmarkGjsonGet(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = gjson.GetBytes(sampleJSON, "name").String()
	}
}

func BenchmarkZeronodeGetPath(b *testing.B) {
	node := FromBytes(sampleJSON)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = node.GetPathFast("nested.layer1.layer2.value").String()
	}
}

func BenchmarkGjsonGetPath(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = gjson.GetBytes(sampleJSON, "nested.layer1.layer2.value").String()
	}
}

func BenchmarkZeronodeForEachObject(b *testing.B) {
	node := FromBytes(sampleJSON).Get("nested").Get("layer1").Get("layer2")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		node.ForEachObject(func(k []byte, v Node) bool {
			_ = k
			_ = v
			return true
		})
	}
}

func BenchmarkGjsonForEach(b *testing.B) {
	result := gjson.GetBytes(sampleJSON, "nested.layer1.layer2")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		result.ForEach(func(_, _ gjson.Result) bool {
			return true
		})
	}
}
