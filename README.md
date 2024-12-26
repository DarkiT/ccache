# CCache  一个LRU缓存，专注于支持高并发

[![GoDoc](https://godoc.org/github.com/darkit/ccache?status.svg)](https://godoc.org/github.com/darkit/ccache)
[![Go Report Card](https://goreportcard.com/badge/github.com/darkit/ccache)](https://goreportcard.com/report/github.com/darkit/ccache)
[![Coverage Status](https://coveralls.io/repos/github/DarkiT/ccache/badge.svg?branch=master)](https://coveralls.io/github/DarkiT/ccache?branch=master)
[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

CCache 是一个用 Go 语言编写的 LRU 缓存，专注于支持高并发。

通过以下方式减少对列表的锁竞争：
* 引入一个窗口来限制项目被提升的频率
* 使用缓冲通道来为单个工作者排队处理提升操作
* 在同一个工作者线程中进行垃圾回收

除非另有说明，所有方法都是线程安全的。

## 配置
### 基础配置
导入并创建一个 `Cache` 实例：
```go
import (
  "github.com/darkit/ccache"
)

// 创建一个存储字符串值的缓存
var cache = ccache.New(ccache.Configure[string]())
```

`Configure` 提供了链式 API：
```go
// 创建一个存储整数值的缓存
var cache = ccache.New(ccache.Configure[int]().MaxSize(1000).ItemsToPrune(100))
```

### 配置选项
最可能需要调整的配置选项是：
* `MaxSize(int)` - 缓存中存储的最大项目数（默认：10000）
* `GetsPerPromote(int)` - 在提升一个项目之前被获取的次数（默认：3）
* `ItemsToPrune(int)` - 当达到 `MaxSize` 时要清理的项目数量（默认：1000）

改变缓存内部结构的配置，通常不需要调整：
* `Buckets` - ccache 对其内部映射进行分片以提供更高的并发性（默认：16）
* `PromoteBuffer(int)` - 用于队列提升操作的缓冲区大小（默认：1024）
* `DeleteBuffer(int)` - 用于队列删除操作的缓冲区大小（默认：1024）

## 基本操作
### Get
获取缓存中的值：
```go
item := cache.Get("user:4")
if item == nil {
  //处理空值情况
} else {
  user := item.Value()
}
```

返回的 `*Item` 提供了几个方法：
* `Value() T` - 缓存的值
* `Expired() bool` - 项目是否已过期
* `TTL() time.Duration` - 项目过期前的持续时间
* `Expires() time.Time` - 项目将要过期的时间

### Set
设置缓存值：
```go
cache.Set("user:4", user, time.Minute * 10)
```

### Delete
删除缓存值：
```go
cache.Delete("user:4")
```

### Has
检查键是否存在：
```go
exists, value := cache.Has("user:4")
if exists {
    // 使用 value
} else {
    // 处理不存在的情况
}
```

## 高级操作
### Fetch
智能获取或设置缓存值：
```go
item, err := cache.Fetch("user:4", time.Minute * 10, func() (*User, error) {
    // 缓存未命中时执行
    user, err := db.GetUser(4)
    if err != nil {
        return nil, err
    }
    return user, nil
})
```

### GetMulti/SetMulti
批量操作：
```go
// 批量获取
items := cache.GetMulti([]string{"user:1", "user:2", "user:3"})

// 批量设置
items := map[string]string{
    "user:1": "value1",
    "user:2": "value2",
}
cache.SetMulti(items, time.Minute*10)
```

### PushSlice/PushMap
向切片、Map类型的缓存追加值：
```go
// 设置初始切片
cache.Set("numbers", []int{1, 2, 3}, time.Minute)

// 追加新的值
err := cache.PushSlice("numbers", []int{4}, time.Minute)
if err != nil {
    // 处理错误
}
// 设置初始切片
cache.Set("map_numbers", map[string]int{"a": 1, "b": 2}, time.Minute)

// 追加新的值
err := cache.PushMap("map_numbers", map[string]int{"c": 3, "d": 4}, time.Minute)
if err != nil {
// 处理错误
}
```

### Extend/Touch
更新缓存项的过期时间。两个方法的主要区别在于是否影响缓存项在 LRU 链表中的位置：

```go
// 使用 Extend - 仅更新过期时间，不改变缓存项在 LRU 中的位置
success := cache.Extend("user:4", time.Minute * 10)

// 使用 Touch - 更新过期时间，同时将缓存项移动到 LRU 链表前端
success := cache.Touch("user:4", time.Minute * 10)
```

选择建议：
- 使用 `Extend`：当只需要延长过期时间，不需要改变访问频率时
- 使用 `Touch`：当需要同时更新过期时间和访问状态时

注意：两个方法都返回 bool 值表示操作是否成功（键是否存在）。

### Replace
更新值但保持TTL：
```go
cache.Replace("user:4", user)
```

### SetIfNotExists/SetIfNotExistsWithFunc
两种方式设置不存在的键值：

```go
// 方式1：直接设置值
cache.SetIfNotExists("key", "value", time.Minute)

// 方式2：通过函数生成值（延迟计算）
item := cache.SetIfNotExistsWithFunc("key", func() string {
    // 只在键不存在时才会执行
    return expensiveOperation()
}, time.Minute)
```

两者的区别：
- `SetIfNotExists`：直接传入值，简单直观
- `SetIfNotExistsWithFunc`：
    - 传入函数来生成值
    - 只在键不存在时才会执行函数
    - 适合值的计算成本较高的场景
    - 返回设置的缓存项

注意：`SetIfNotExistsWithFunc` 在并发场景下更高效，因为它避免了不必要的值计算。
### Inc/Dec
数值增减：
```go
newVal, err := cache.Inc("counter", 1)
if err != nil {
    // 处理错误
}

newVal, err = cache.Dec("counter", 1)
if err != nil {
    // 处理错误
}
```

## 监控和管理
### GetDropped
获取被驱逐的键数量：
```go
dropped := cache.GetDropped()
```

### Stop
停止缓存的后台工作进程：
```go
cache.Stop()
```

## 特殊功能

### 追踪模式
CCache 支持特殊的追踪模式，主要用于与代码中需要维护数据长期引用的其他部分配合使用。

使用 `Track()` 配置追踪模式：
```go
cache = ccache.New(ccache.Configure[int]().Track())
```

通过 `TrackingGet` 获取的项目在调用 `Release` 之前不会被清除：
```go
item := cache.TrackingGet("user:4")
user := item.Value()   // 如果缓存中没有 "user:4"，将返回 nil
item.Release()         // 即使 item.Value() 返回 nil 也可以调用
```

追踪模式的主要优势：
- `Release` 通常在代码的其他地方延迟调用
- 可以使用 `TrackingSet` 设置需要追踪的值
- 有助于确保系统返回一致的数据
- 适合其他代码部分也持有对象引用的场景

### 分层缓存（LayeredCache）
`LayeredCache` 提供了通过主键和次键来存储和检索值的能力。它特别适合需要管理数据多个变体的场景，如 HTTP 缓存。

主要特点：
- 支持主键和次键的组合
- 可以针对特定键或主键下所有值进行删除
- 使用与主缓存相同的配置
- 支持可选的追踪功能

使用示例：
```go
cache := ccache.Layered(ccache.Configure[string]())

// 设置不同格式的数据
cache.Set("/users/goku", "type:json", "{value_to_cache}", time.Minute * 5)
cache.Set("/users/goku", "type:xml", "<value_to_cache>", time.Minute * 5)

// 获取特定格式
json := cache.Get("/users/goku", "type:json")
xml := cache.Get("/users/goku", "type:xml")

// 删除操作
cache.Delete("/users/goku", "type:json")     // 删除特定格式
cache.DeleteAll("/users/goku")               // 删除所有格式
```

### 二级缓存（SecondaryCache）
当使用 `LayeredCache` 时，有时需要频繁操作某个主键下的缓存条目。`SecondaryCache` 提供了这种便利性：

```go
cache := ccache.Layered(ccache.Configure[string]())
sCache := cache.GetOrCreateSecondaryCache("/users/goku")
sCache.Set("type:json", "{value_to_cache}", time.Minute * 5)
```

特点：
- 与普通 `Cache` 的操作方式相同
- `Get` 操作不返回 nil，而是返回空缓存
- 简化了对特定主键数据的操作

## 大小控制
缓存项目的大小管理：
- 默认每个项目大小为 1
- 如果值实现了 `Size() int64` 方法，将使用该方法返回的大小
- 每个缓存条目有约 350 字节的额外开销（不计入大小限制）

示例：
- 配置 `MaxSize(10000)` 可存储 10000 个默认大小的项目
- 对于实现了 `Size() int64` 的项目，实际存储数量 = MaxSize/项目大小

## 许可证

MIT License - 查看 [LICENSE](LICENSE) 文件了解详情。
