# GCJSON - High-Performance JSON Processing Library

GCJSON 是一个高性能的 Go JSON 处理库，专为大规模、高频次的 JSON 数据处理场景设计。

## 特性

- 🚀 **极高性能**: 零拷贝、内联优化、热点路径缓存
- 🎯 **智能优化**: 简单路径快速处理，复杂路径回退到 gjson
- 🔧 **灵活易用**: 支持泛型、自动类型推断、数据下钻
- 🛡️ **类型安全**: 完整的类型检查和错误处理

## 快速开始

```go
package main

import (
    "fmt"
    "gcjson"
)

func main() {
    data := `{
        "data": {
            "user": {
                "name": "Alice",
                "age": 30,
                "scores": [95, 87, 92]
            }
        }
    }`

    // 基础用法
    name := gcjson.Any(data, "data.user.name")
    fmt.Println("Name:", name) // Name: Alice

    // 泛型用法
    age, ok := gcjson.AnyAs[int64](data, "data.user.age")
    if ok {
        fmt.Println("Age:", age) // Age: 30
    }

    // 数组处理
    gcjson.EachArray(data, "data.user.scores", func(i int, r gjson.Result) bool {
        fmt.Printf("Score %d: %v\n", i, r.Int())
        return true
    })
}