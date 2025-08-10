
```markdown
# GCJSON - High-Performance JSON Processing Library

GCJSON 是一个高性能的 Go JSON 处理库，专为大规模、高频次的 JSON 数据处理场景设计。

## 特性

- 🚀 **极高性能**: 零拷贝、内联优化、热点路径缓存
- 🎯 **智能优化**: 简单路径快速处理，复杂路径回退到 gjson
- 🔧 **灵活易用**: 支持泛型、自动类型推断、数据下钻
- 🛡️ **类型安全**: 完整的类型检查和错误处理
- 📦 **模块化设计**: 清晰的包结构，便于维护和扩展

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
```

## API 分类

### 基础查询 API
- `GetAny(v, path)` - 获取任意路径的 gjson.Result
- `GetData(v, path)` - 自动下钻到 data 字段后查询
- `Any(v, path)` - 自动类型推断，返回原生 Go 类型

### 泛型 API
- `AnyAs[T](v, path)` - 泛型类型断言
- `AnyAsFast[T](v, path)` - 零分配泛型快路径
- `AnyOr(v, path, def)` - 带默认值的查询

### 迭代器 API
- `EachObject(v, path, fn)` - 遍历对象
- `EachArray(v, path, fn)` - 遍历数组
- `ForEachArrayResult(v, path, fn)` - Result 版本数组迭代

### 原始数据 API
- `Raw(v, path)` - 获取原始 JSON 字符串
- `RawBytes(v, path)` - 零拷贝获取原始字节
- `RawMany(v, paths...)` - 批量获取多个路径

## 性能优化

### 热点路径缓存
自动缓存常用的 JSON 路径，提升重复查询性能。

### 简单路径快速处理
对于简单的顶层键（如 `user`, `data`），使用 O(n) 扫描而非完整解析。

### 零拷贝操作
尽可能使用 unsafe 指针操作，避免不必要的内存分配。

### 泛型快路径
针对常见类型（string, int64, float64, bool）提供零分配路径。

## 模块架构

```
gcjson/
├── cache/      # 热点路径缓存
├── convert/    # 类型转换和序列化
├── fast/       # 快速路径优化
├── iterator/   # 迭代器功能
├── parser/     # 数字解析和类型推断
├── picker/     # 数据提取和下钻
└── raw/        # 原始数据处理
```

## 基准测试

```bash
go test -bench=. -benchmem
```

典型性能表现：
- 简单路径查询: ~10ns/op, 0 allocs/op
- 复杂路径查询: ~50ns/op, 1 allocs/op
- 泛型快路径: ~15ns/op, 0 allocs/op