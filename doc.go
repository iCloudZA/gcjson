// Package gcjson 提供基于 gjson 的高性能 JSON 查询能力。
//
// 特点：
//   - 零反序列化：无需 json.Unmarshal，直接路径访问 JSON 数据。
//   - 零或极少分配：多数查询仅需 0~2 次内存分配。
//   - 泛型直达：支持 Go 泛型 API，直接返回指定类型。
//   - 支持数组、对象遍历，以及路径下钻（drill）。
//
// 输入类型：
//   - 仅接受 []byte 或 string 类型作为输入。
//
// 内部依赖：
//   - 使用 gjson 进行路径解析与取值。
//
// # 示例
//
// 基础用法：
//
//	jsonData := []byte(`{"name":"Alice","age":30}`)
//	name := gcjson.Any(jsonData, "name")         // 动态类型
//	age, _ := gcjson.AnyAs[int64](jsonData, "age") // 泛型类型安全
//
// 泛型快速访问（零分配版本）：
//
//	name, _ := gcjson.AnyAsFast[string](jsonData, "name")
//
// 自动下钻：
//
//	sample := []byte(`{"data":{"id":123}}`)
//	gcjson.SetDefaultDrillKeys("data")
//	id := gcjson.AnyData(sample, "id") // 等价于 gcjson.Any(sample, "data.id")
//
// 数组遍历：
//
//	gcjson.EachArray(jsonData, "tags", func(i int, r gjson.Result) bool {
//	    fmt.Println(i, r.String())
//	    return true
//	})
package gcjson
