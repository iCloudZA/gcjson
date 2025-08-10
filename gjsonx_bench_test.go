package gcjson

import (
	"encoding/json"
	"fmt"
	"github.com/tidwall/gjson"
	"testing"
)

func init() {
	SetDefaultDrillKeys("data") // 保证 AnyData 下钻到 data
}

// 测试用复杂 JSON 数据
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

// 简单 JSON 数据
var simpleJSON = []byte(`{
    "name": "Alice",
    "age": 30,
    "active": true,
    "score": 95.5,
    "tags": ["golang", "json", "performance"]
}`)

var (
	sinkInt    int
	sinkString string
	sinkBool   bool
	sinkFloat  float64
	sinkAny    any
)

// ==================== 基础功能对比 ====================

func BenchmarkSimpleKey_GCJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s, _ := AnyAs[string](simpleJSON, "name")
		sinkString = s
	}
}

func BenchmarkSimpleKey_GJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkString = gjson.GetBytes(simpleJSON, "name").String()
	}
}

func BenchmarkSimpleKey_StdLib(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var m map[string]any
		json.Unmarshal(simpleJSON, &m)
		sinkString = m["name"].(string)
	}
}

func BenchmarkNumber_GCJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		n, _ := AnyAs[int64](simpleJSON, "age")
		sinkInt = int(n)
	}
}

func BenchmarkNumber_GJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkInt = int(gjson.GetBytes(simpleJSON, "age").Int())
	}
}

func BenchmarkNumber_StdLib(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var m map[string]any
		json.Unmarshal(simpleJSON, &m)
		sinkInt = int(m["age"].(float64))
	}
}

func BenchmarkDeepPath_GCJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s, _ := AnyAs[string](sampleJSON, "data.nested.deep.layer1.layer2.layer3.value")
		sinkString = s
	}
}

func BenchmarkDeepPath_GJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkString = gjson.GetBytes(sampleJSON, "data.nested.deep.layer1.layer2.layer3.value").String()
	}
}

func BenchmarkDeepPath_StdLib(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var m map[string]any
		json.Unmarshal(sampleJSON, &m)
		layer3 := m["data"].(map[string]any)["nested"].(map[string]any)["deep"].(map[string]any)["layer1"].(map[string]any)["layer2"].(map[string]any)["layer3"].(map[string]any)
		sinkString = layer3["value"].(string)
	}
}

// ==================== 泛型 API ====================

func BenchmarkGeneric_AnyAs_String(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s, _ := AnyAs[string](simpleJSON, "name")
		sinkString = s
	}
}

func BenchmarkGeneric_AnyAsFast_String(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s, _ := AnyAsFast[string](simpleJSON, "name")
		sinkString = s
	}
}

func BenchmarkGeneric_AnyAs_Int64(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		n, _ := AnyAs[int64](simpleJSON, "age")
		sinkInt = int(n)
	}
}

func BenchmarkGeneric_AnyAsFast_Int64(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		n, _ := AnyAsFast[int64](simpleJSON, "age")
		sinkInt = int(n)
	}
}

func BenchmarkGeneric_AnyAsFast_Bool(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		v, _ := AnyAsFast[bool](simpleJSON, "active")
		sinkBool = v
	}
}

func BenchmarkGeneric_AnyAsFast_Float64(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		v, _ := AnyAsFast[float64](simpleJSON, "score")
		sinkFloat = v
	}
}

// ==================== 数据下钻 ====================

func BenchmarkDrill_AnyData(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkAny = AnyData(sampleJSON, "id")
	}
}

func BenchmarkDrill_Manual(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkAny = Any(sampleJSON, "data.id")
	}
}

// ==================== 数组和对象遍历 ====================

func BenchmarkArray_GCJSON_EachArray(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		count := 0
		EachArray(sampleJSON, "data.nested.items", func(_ int, r gjson.Result) bool {
			count += int(r.Int())
			return true
		})
		sinkInt = count
	}
}

func BenchmarkArray_GJSON_ForEach(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		count := 0
		gjson.GetBytes(sampleJSON, "data.nested.items").ForEach(func(_, value gjson.Result) bool {
			count += int(value.Int())
			return true
		})
		sinkInt = count
	}
}

func BenchmarkArray_StdLib(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var m map[string]any
		json.Unmarshal(sampleJSON, &m)
		items := m["data"].(map[string]any)["nested"].(map[string]any)["items"].([]any)
		count := 0
		for _, v := range items {
			count += int(v.(float64))
		}
		sinkInt = count
	}
}

func BenchmarkObject_GCJSON_EachObject(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		count := 0
		EachObject(sampleJSON, "data.nested", func(k string, r gjson.Result) bool {
			count += len(k) + len(r.Raw)
			return true
		})
		sinkInt = count
	}
}

// ==================== 热点路径缓存 ====================

func BenchmarkHotPath_Repeat(b *testing.B) {
	for i := 0; i < 100; i++ {
		Any(sampleJSON, "data.nested.flag")
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		v, _ := AnyAs[bool](sampleJSON, "data.nested.flag")
		sinkBool = v
	}
}

func BenchmarkHotPath_Cold(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		path := fmt.Sprintf("data.nested.flag%d", i%1000)
		Any(sampleJSON, path)
	}
}
