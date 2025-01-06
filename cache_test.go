package ccache

import (
	"math/rand"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/darkit/ccache/assert"
)

func Test_SetIfNotExists(t *testing.T) {
	cache := New(Configure[string]())
	defer cache.Stop()
	assert.Equal(t, cache.ItemCount(), 0)

	cache.Set("spice", "flow", time.Minute)
	assert.Equal(t, cache.ItemCount(), 1)

	// set if exists
	cache.SetIfNotExists("spice", "worm", time.Minute)
	assert.Equal(t, cache.ItemCount(), 1)
	assert.Equal(t, cache.Get("spice").Value(), "flow")

	// set if not exists
	cache.Delete("spice")
	cache.SetIfNotExists("spice", "worm", time.Minute)
	assert.Equal(t, cache.Get("spice").Value(), "worm")

	assert.Equal(t, cache.ItemCount(), 1)
}

func Test_Cache_SetIfNotExistsWithFunc(t *testing.T) {
	cache := New(Configure[string]())

	// 测试1: 键不存在时的情况
	callCount := 0
	item := cache.SetIfNotExistsWithFunc("key1", func() string {
		callCount++
		return "value1"
	}, time.Minute)

	// 验证结果
	assert.NotNil(t, item)
	assert.Equal(t, "value1", item.Value())
	assert.Equal(t, 1, callCount)
	assert.False(t, item.Expired())

	// 测试2: 键已存在时的情况
	item = cache.SetIfNotExistsWithFunc("key1", func() string {
		callCount++
		return "value2"
	}, time.Minute)

	// 验证函数未被调用，返回原有值
	assert.NotNil(t, item)
	assert.Equal(t, "value1", item.Value())
	assert.Equal(t, 1, callCount) // callCount 应该保持不变

	// 测试3: 并发场景
	var wg sync.WaitGroup
	concurrentCount := int32(0)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			item := cache.SetIfNotExistsWithFunc("key2", func() string {
				atomic.AddInt32(&concurrentCount, 1)
				return "concurrent"
			}, time.Minute)
			assert.NotNil(t, item)
		}()
	}
	wg.Wait()

	// 验证并发场景下函数只被调用一次
	assert.Equal(t, int32(1), atomic.LoadInt32(&concurrentCount))
	item = cache.Get("key2")
	assert.Equal(t, "concurrent", item.Value())

	// 测试4: 过期键的情况
	cache.Set("expired", "old", time.Millisecond*-1)
	expiredCallCount := 0
	item = cache.SetIfNotExistsWithFunc("expired", func() string {
		expiredCallCount++
		return "new"
	}, time.Minute)

	// 验证过期键仍然返回原值（因为 SetIfNotExists 只检查键是否存在，不检查过期状态）
	assert.NotNil(t, item)
	assert.Equal(t, "old", item.Value())
	assert.Equal(t, 0, expiredCallCount)
	assert.True(t, item.Expired())
}

func Test_Extend(t *testing.T) {
	cache := New(Configure[string]())
	defer cache.Stop()
	assert.Equal(t, cache.ItemCount(), 0)

	// non exist
	ok := cache.Extend("spice", time.Minute*10)
	assert.Equal(t, false, ok)

	// exist
	cache.Set("spice", "flow", time.Minute)
	assert.Equal(t, cache.ItemCount(), 1)

	ok = cache.Extend("spice", time.Minute*10) // 10 + 10
	assert.Equal(t, true, ok)

	item := cache.Get("spice")
	less := time.Minute*22 < time.Duration(item.expires)
	assert.Equal(t, true, less)
	more := time.Minute*18 < time.Duration(item.expires)
	assert.Equal(t, true, more)

	assert.Equal(t, cache.ItemCount(), 1)
}

func Test_CacheDeletesAValue(t *testing.T) {
	cache := New(Configure[string]())
	defer cache.Stop()
	assert.Equal(t, cache.ItemCount(), 0)

	cache.Set("spice", "flow", time.Minute)
	cache.Set("worm", "sand", time.Minute)
	assert.Equal(t, cache.ItemCount(), 2)

	cache.Delete("spice")
	assert.Equal(t, cache.Get("spice"), nil)
	assert.Equal(t, cache.Get("worm").Value(), "sand")
	assert.Equal(t, cache.ItemCount(), 1)
}

func Test_CacheDeletesAPrefix(t *testing.T) {
	cache := New(Configure[string]())
	defer cache.Stop()
	assert.Equal(t, cache.ItemCount(), 0)

	cache.Set("aaa", "1", time.Minute)
	cache.Set("aab", "2", time.Minute)
	cache.Set("aac", "3", time.Minute)
	cache.Set("ac", "4", time.Minute)
	cache.Set("z5", "7", time.Minute)
	assert.Equal(t, cache.ItemCount(), 5)

	assert.Equal(t, cache.DeletePrefix("9a"), 0)
	assert.Equal(t, cache.ItemCount(), 5)

	assert.Equal(t, cache.DeletePrefix("aa"), 3)
	assert.Equal(t, cache.Get("aaa"), nil)
	assert.Equal(t, cache.Get("aab"), nil)
	assert.Equal(t, cache.Get("aac"), nil)
	assert.Equal(t, cache.Get("ac").Value(), "4")
	assert.Equal(t, cache.Get("z5").Value(), "7")
	assert.Equal(t, cache.ItemCount(), 2)
}

func Test_CacheDeletesAFunc(t *testing.T) {
	cache := New(Configure[int]())
	defer cache.Stop()
	assert.Equal(t, cache.ItemCount(), 0)

	cache.Set("a", 1, time.Minute)
	cache.Set("b", 2, time.Minute)
	cache.Set("c", 3, time.Minute)
	cache.Set("d", 4, time.Minute)
	cache.Set("e", 5, time.Minute)
	cache.Set("f", 6, time.Minute)
	assert.Equal(t, cache.ItemCount(), 6)

	assert.Equal(t, cache.DeleteFunc(func(key string, item *Item[int]) bool {
		return false
	}), 0)
	assert.Equal(t, cache.ItemCount(), 6)

	assert.Equal(t, cache.DeleteFunc(func(key string, item *Item[int]) bool {
		return item.Value() < 4
	}), 3)
	assert.Equal(t, cache.ItemCount(), 3)

	assert.Equal(t, cache.DeleteFunc(func(key string, item *Item[int]) bool {
		return key == "d"
	}), 1)
	assert.Equal(t, cache.ItemCount(), 2)
}

func Test_CacheOnDeleteCallbackCalled(t *testing.T) {
	onDeleteFnCalled := int32(0)
	onDeleteFn := func(item *Item[string]) {
		if item.key == "spice" {
			atomic.AddInt32(&onDeleteFnCalled, 1)
		}
	}

	cache := New(Configure[string]().OnDelete(onDeleteFn))
	cache.Set("spice", "flow", time.Minute)
	cache.Set("worm", "sand", time.Minute)

	cache.SyncUpdates() // wait for worker to pick up preceding updates

	cache.Delete("spice")
	cache.SyncUpdates()

	assert.Equal(t, cache.Get("spice"), nil)
	assert.Equal(t, cache.Get("worm").Value(), "sand")
	assert.Equal(t, atomic.LoadInt32(&onDeleteFnCalled), 1)
}

func Test_CacheFetchesExpiredItems(t *testing.T) {
	cache := New(Configure[string]())
	fn := func() (string, error) { return "moo-moo", nil }

	cache.Set("beef", "moo", time.Second*-1)
	assert.Equal(t, cache.Get("beef").Value(), "moo")

	out, _ := cache.Fetch("beef", time.Second, fn)
	assert.Equal(t, out.Value(), "moo-moo")
}

func Test_CacheGCsTheOldestItems(t *testing.T) {
	cache := New(Configure[int]().ItemsToPrune(10))
	for i := 0; i < 500; i++ {
		cache.Set(strconv.Itoa(i), i, time.Minute)
	}
	cache.SyncUpdates()
	cache.GC()
	assert.Equal(t, cache.Get("9"), nil)
	assert.Equal(t, cache.Get("10").Value(), 10)
	assert.Equal(t, cache.ItemCount(), 490)
}

func Test_CachePromotedItemsDontGetPruned(t *testing.T) {
	cache := New(Configure[int]().ItemsToPrune(10).GetsPerPromote(1))
	for i := 0; i < 500; i++ {
		cache.Set(strconv.Itoa(i), i, time.Minute)
	}
	cache.SyncUpdates()
	cache.Get("9")
	cache.SyncUpdates()
	cache.GC()
	assert.Equal(t, cache.Get("9").Value(), 9)
	assert.Equal(t, cache.Get("10"), nil)
	assert.Equal(t, cache.Get("11").Value(), 11)
}

func Test_GetWithoutPromoteDoesNotPromote(t *testing.T) {
	cache := New(Configure[int]().ItemsToPrune(10).GetsPerPromote(1))
	for i := 0; i < 500; i++ {
		cache.Set(strconv.Itoa(i), i, time.Minute)
	}
	cache.SyncUpdates()
	cache.GetWithoutPromote("9")
	cache.SyncUpdates()
	cache.GC()
	assert.Equal(t, cache.Get("9"), nil)
	assert.Equal(t, cache.Get("10").Value(), 10)
	assert.Equal(t, cache.Get("11").Value(), 11)
}

func Test_CacheTrackerDoesNotCleanupHeldInstance(t *testing.T) {
	cache := New(Configure[int]().ItemsToPrune(11).Track())
	item0 := cache.TrackingSet("0", 0, time.Minute)
	for i := 1; i < 11; i++ {
		cache.Set(strconv.Itoa(i), i, time.Minute)
	}
	item1 := cache.TrackingGet("1")
	cache.SyncUpdates()
	cache.GC()
	assert.Equal(t, cache.Get("0").Value(), 0)
	assert.Equal(t, cache.Get("1").Value(), 1)
	item0.Release()
	item1.Release()
	cache.GC()
	assert.Equal(t, cache.Get("0"), nil)
	assert.Equal(t, cache.Get("1"), nil)
}

func Test_CacheRemovesOldestItemWhenFull(t *testing.T) {
	onDeleteFnCalled := false
	onDeleteFn := func(item *Item[int]) {
		if item.key == "0" {
			onDeleteFnCalled = true
		}
	}

	cache := New(Configure[int]().MaxSize(5).ItemsToPrune(1).OnDelete(onDeleteFn))
	for i := 0; i < 7; i++ {
		cache.Set(strconv.Itoa(i), i, time.Minute)
	}
	cache.SyncUpdates()
	assert.Equal(t, cache.Get("0"), nil)
	assert.Equal(t, cache.Get("1"), nil)
	assert.Equal(t, cache.Get("2").Value(), 2)
	assert.Equal(t, onDeleteFnCalled, true)
	assert.Equal(t, cache.ItemCount(), 5)
}

func Test_CacheRemovesOldestItemWhenFullBySizer(t *testing.T) {
	cache := New(Configure[*SizedItem]().MaxSize(9).ItemsToPrune(2))
	for i := 0; i < 7; i++ {
		cache.Set(strconv.Itoa(i), &SizedItem{i, 2}, time.Minute)
	}
	cache.SyncUpdates()
	assert.Equal(t, cache.Get("0"), nil)
	assert.Equal(t, cache.Get("1"), nil)
	assert.Equal(t, cache.Get("2"), nil)
	assert.Equal(t, cache.Get("3"), nil)
	assert.Equal(t, cache.Get("4").Value().id, 4)
	assert.Equal(t, cache.GetDropped(), 4)
	assert.Equal(t, cache.GetDropped(), 0)
}

func Test_CacheSetUpdatesSizeOnDelta(t *testing.T) {
	cache := New(Configure[*SizedItem]())
	cache.Set("a", &SizedItem{0, 2}, time.Minute)
	cache.Set("b", &SizedItem{0, 3}, time.Minute)
	cache.SyncUpdates()
	assert.Equal(t, cache.GetSize(), 5)
	cache.Set("b", &SizedItem{0, 3}, time.Minute)
	cache.SyncUpdates()
	assert.Equal(t, cache.GetSize(), 5)
	cache.Set("b", &SizedItem{0, 4}, time.Minute)
	cache.SyncUpdates()
	assert.Equal(t, cache.GetSize(), 6)
	cache.Set("b", &SizedItem{0, 2}, time.Minute)
	cache.SyncUpdates()
	assert.Equal(t, cache.GetSize(), 4)
	cache.Delete("b")
	cache.SyncUpdates()
	assert.Equal(t, cache.GetSize(), 2)
}

func Test_CacheReplaceDoesNotchangeSizeIfNotSet(t *testing.T) {
	cache := New(Configure[*SizedItem]())
	cache.Set("1", &SizedItem{1, 2}, time.Minute)
	cache.Set("2", &SizedItem{1, 2}, time.Minute)
	cache.Set("3", &SizedItem{1, 2}, time.Minute)
	cache.Replace("4", &SizedItem{1, 2})
	cache.SyncUpdates()
	assert.Equal(t, cache.GetSize(), 6)
}

func Test_CacheReplaceChangesSize(t *testing.T) {
	cache := New(Configure[*SizedItem]())
	cache.Set("1", &SizedItem{1, 2}, time.Minute)
	cache.Set("2", &SizedItem{1, 2}, time.Minute)

	cache.Replace("2", &SizedItem{1, 2})
	cache.SyncUpdates()
	assert.Equal(t, cache.GetSize(), 4)

	cache.Replace("2", &SizedItem{1, 1})
	cache.SyncUpdates()
	assert.Equal(t, cache.GetSize(), 3)

	cache.Replace("2", &SizedItem{1, 3})
	cache.SyncUpdates()
	assert.Equal(t, cache.GetSize(), 5)
}

func Test_CacheResizeOnTheFly(t *testing.T) {
	cache := New(Configure[int]().MaxSize(9).ItemsToPrune(1))
	for i := 0; i < 5; i++ {
		cache.Set(strconv.Itoa(i), i, time.Minute)
	}
	cache.SetMaxSize(3)
	cache.SyncUpdates()
	assert.Equal(t, cache.GetDropped(), 2)
	assert.Equal(t, cache.Get("0"), nil)
	assert.Equal(t, cache.Get("1"), nil)
	assert.Equal(t, cache.Get("2").Value(), 2)
	assert.Equal(t, cache.Get("3").Value(), 3)
	assert.Equal(t, cache.Get("4").Value(), 4)

	cache.Set("5", 5, time.Minute)
	cache.SyncUpdates()
	assert.Equal(t, cache.GetDropped(), 1)
	assert.Equal(t, cache.Get("2"), nil)
	assert.Equal(t, cache.Get("3").Value(), 3)
	assert.Equal(t, cache.Get("4").Value(), 4)
	assert.Equal(t, cache.Get("5").Value(), 5)

	cache.SetMaxSize(10)
	cache.Set("6", 6, time.Minute)
	cache.SyncUpdates()
	assert.Equal(t, cache.GetDropped(), 0)
	assert.Equal(t, cache.Get("3").Value(), 3)
	assert.Equal(t, cache.Get("4").Value(), 4)
	assert.Equal(t, cache.Get("5").Value(), 5)
	assert.Equal(t, cache.Get("6").Value(), 6)
}

func Test_CacheForEachFunc(t *testing.T) {
	cache := New(Configure[int]().MaxSize(3).ItemsToPrune(1))
	assert.List(t, forEachKeys[int](cache), []string{})

	cache.Set("1", 1, time.Minute)
	assert.List(t, forEachKeys(cache), []string{"1"})

	cache.Set("2", 2, time.Minute)
	cache.SyncUpdates()
	assert.List(t, forEachKeys(cache), []string{"1", "2"})

	cache.Set("3", 3, time.Minute)
	cache.SyncUpdates()
	assert.List(t, forEachKeys(cache), []string{"1", "2", "3"})

	cache.Set("4", 4, time.Minute)
	cache.SyncUpdates()
	assert.List(t, forEachKeys(cache), []string{"2", "3", "4"})

	cache.Set("stop", 5, time.Minute)
	cache.SyncUpdates()
	assert.DoesNotContain(t, forEachKeys(cache), "stop")

	cache.Set("6", 6, time.Minute)
	cache.SyncUpdates()
	assert.DoesNotContain(t, forEachKeys(cache), "stop")
}

func Test_CachePrune(t *testing.T) {
	maxSize := int64(500)
	cache := New(Configure[string]().MaxSize(maxSize).ItemsToPrune(50))
	epoch := 0
	for i := 0; i < 10000; i++ {
		epoch += 1
		expired := make([]string, 0)
		for i := 0; i < 50; i += 1 {
			key := strconv.FormatInt(rand.Int63n(maxSize*20), 10)
			item := cache.Get(key)
			if item == nil || item.TTL() > 1*time.Minute {
				expired = append(expired, key)
			}
		}
		for _, key := range expired {
			cache.Set(key, key, 5*time.Minute)
		}
		if epoch%500 == 0 {
			assert.True(t, cache.GetSize() <= 500)
		}
	}
}

func Test_ConcurrentStop(t *testing.T) {
	for i := 0; i < 100; i++ {
		cache := New(Configure[string]())
		r := func() {
			for {
				key := strconv.Itoa(int(rand.Int31n(100)))
				switch rand.Int31n(3) {
				case 0:
					cache.Get(key)
				case 1:
					cache.Set(key, key, time.Minute)
				case 2:
					cache.Delete(key)
				}
			}
		}
		go r()
		go r()
		go r()
		time.Sleep(time.Millisecond * 10)
		cache.Stop()
	}
}

func Test_ConcurrentClearAndSet(t *testing.T) {
	for i := 0; i < 1000000; i++ {
		var stop atomic.Bool
		var wg sync.WaitGroup

		cache := New(Configure[string]())
		r := func() {
			for !stop.Load() {
				cache.Set("a", "a", time.Minute)
			}
			wg.Done()
		}
		go r()
		wg.Add(1)
		cache.Clear()
		stop.Store(true)
		wg.Wait()
		cache.SyncUpdates()

		// The point of this test is to make sure that the cache's lookup and its
		// recency list are in sync. But the two aren't written to atomically:
		// the lookup is written to directly from the call to Set, whereas the
		// list is maintained by the background worker. This can create a period
		// where the two are out of sync. Even SyncUpdate is helpless here, since
		// it can only sync what's been written to the buffers.
		for i := 0; i < 10; i++ {
			expectedCount := 0
			if cache.list.Head != nil {
				expectedCount = 1
			}
			actualCount := cache.ItemCount()
			if expectedCount == actualCount {
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Errorf("cache list and lookup are not consistent")
		t.FailNow()
	}
}

func BenchmarkFrequentSets(b *testing.B) {
	cache := New(Configure[int]())
	defer cache.Stop()

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		key := strconv.Itoa(n)
		cache.Set(key, n, time.Minute)
	}
}

func BenchmarkFrequentGets(b *testing.B) {
	cache := New(Configure[int]())
	defer cache.Stop()
	numKeys := 500
	for i := 0; i < numKeys; i++ {
		key := strconv.Itoa(i)
		cache.Set(key, i, time.Minute)
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		key := strconv.FormatInt(rand.Int63n(int64(numKeys)), 10)
		cache.Get(key)
	}
}

func BenchmarkGetWithPromoteSmall(b *testing.B) {
	getsPerPromotes := 5
	cache := New(Configure[int]().GetsPerPromote(int32(getsPerPromotes)))
	defer cache.Stop()

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		key := strconv.Itoa(n)
		cache.Set(key, n, time.Minute)
		for i := 0; i < getsPerPromotes; i++ {
			cache.Get(key)
		}
	}
}

func BenchmarkGetWithPromoteLarge(b *testing.B) {
	getsPerPromotes := 100
	cache := New(Configure[int]().GetsPerPromote(int32(getsPerPromotes)))
	defer cache.Stop()

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		key := strconv.Itoa(n)
		cache.Set(key, n, time.Minute)
		for i := 0; i < getsPerPromotes; i++ {
			cache.Get(key)
		}
	}
}

func Test_Cache_Has(t *testing.T) {
	cache := New(Configure[string]())

	// Test non-existent key
	value, exists := cache.Has("missing")
	assert.Equal(t, exists, false)
	assert.Equal(t, value, "")

	// Test existing key
	cache.Set("test", "value", time.Minute)
	value, exists = cache.Has("test")
	assert.Equal(t, exists, true)
	assert.Equal(t, value, "value")

	// Test expired key
	cache.Set("expired", "old", time.Second*-1)
	value, exists = cache.Has("expired")
	assert.Equal(t, exists, false)
	assert.Equal(t, value, "")
}

func Test_Cache_GetMulti(t *testing.T) {
	cache := New(Configure[string]())

	// 设置测试数据
	cache.Set("key1", "value1", time.Minute)
	cache.Set("key2", "value2", time.Minute)
	cache.Set("key3", "value3", time.Second*-1) // 已过期

	// 测试获取多个键
	items := cache.GetMulti([]string{"key1", "key2", "key3", "key4"})

	// 验证结果
	assert.Equal(t, 2, len(items))
	assert.NotNil(t, items["key1"])
	assert.NotNil(t, items["key2"])
	assert.Nil(t, items["key3"]) // 过期的键
	assert.Nil(t, items["key4"]) // 不存在的键

	assert.Equal(t, "value1", items["key1"].Value())
	assert.Equal(t, "value2", items["key2"].Value())
}

func Test_Cache_SetMulti(t *testing.T) {
	cache := New(Configure[string]())

	// 准备测试数据
	items := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	// 批量设置
	cache.SetMulti(items, time.Minute)

	// 验证结果
	for key, expectedValue := range items {
		item := cache.Get(key)
		assert.NotNil(t, item)
		assert.Equal(t, expectedValue, item.Value())
		assert.False(t, item.Expired())
	}
}

func Test_Cache_Inc(t *testing.T) {
	cache := New(Configure[int64]())

	// 测试对不存在的键进行增加
	val, err := cache.Inc("counter", 5)
	assert.Nil(t, err)
	assert.Equal(t, int64(5), val)

	// 测试对已存在的键进行增加
	val, err = cache.Inc("counter", 3)
	assert.Nil(t, err)
	assert.Equal(t, int64(8), val)

	// 测试对已存在的键进行减少
	val, err = cache.Dec("counter", 4)
	assert.Nil(t, err)
	assert.Equal(t, int64(4), val)

	// 验证最终值
	item := cache.Get("counter")
	assert.NotNil(t, item)
	assert.Equal(t, int64(4), item.Value())
}

func Test_Cache_Touch(t *testing.T) {
	cache := New(Configure[string]())

	// 设置一个即将过期的项目
	cache.Set("key", "value", time.Millisecond*100)

	// 等待一段时间但不要让它过期
	time.Sleep(time.Millisecond * 50)

	// 更新过期时间
	success := cache.Touch("key", time.Minute)
	assert.True(t, success)

	// 验证项目没有过期
	item := cache.Get("key")
	assert.NotNil(t, item)
	assert.False(t, item.Expired())
	assert.Equal(t, "value", item.Value())

	// 测试对不存在的键进行 Touch
	success = cache.Touch("nonexistent", time.Minute)
	assert.False(t, success)
}

func Test_Cache_Inc_InvalidType(t *testing.T) {
	cache := New(Configure[string]())

	// 设置一个字符串值
	cache.Set("key", "not-a-number", time.Minute)

	// 尝试对字符串值进行增加操作
	_, err := cache.Inc("key", 1)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "value is not an integer")
}

func Test_Cache_GetMulti_Empty(t *testing.T) {
	cache := New(Configure[string]())

	// 测试空键列表
	items := cache.GetMulti([]string{})
	assert.Equal(t, 0, len(items))

	// 测试所有键都不存在的情况
	items = cache.GetMulti([]string{"key1", "key2"})
	assert.Equal(t, 0, len(items))
}

func Test_Cache_SetMulti_Empty(t *testing.T) {
	cache := New(Configure[string]())

	// 测试空map
	cache.SetMulti(map[string]string{}, time.Minute)

	// 验证缓存是空的
	assert.Equal(t, 0, cache.GetDropped())
}

func Test_Cache_Pull(t *testing.T) {
	cache := New[string](Configure[string]())

	// 测试Pull不存在的key
	item := cache.Pull("missing")
	assert.Equal(t, item, nil)

	// 测试Pull存在的key
	cache.Set("test", "value", time.Minute)
	item = cache.Pull("test")
	assert.Equal(t, item.Value(), "value")

	// 验证值已被删除
	assert.Equal(t, cache.Get("test"), nil)
}

func Test_Cache_PushSlice(t *testing.T) {
	cache := New[[]int](Configure[[]int]())

	// 先设置一个初始切片
	cache.Set("numbers", []int{1, 2, 3}, time.Minute)

	// 验证初始状态
	item := cache.Get("numbers")

	// 测试PushSlice
	err := cache.PushSlice("numbers", []int{4}, time.Minute)
	assert.Nil(t, err)

	// 验证最终状态
	item = cache.Get("numbers")
	result := item.Value()
	assert.Equal(t, 4, len(result))
	assert.Equal(t, 1, result[0])
	assert.Equal(t, 2, result[1])
	assert.Equal(t, 3, result[2])
	assert.Equal(t, 4, result[3])
}

func Test_PushMap(t *testing.T) {
	cache := New(Configure[map[string]int]())
	defer cache.Stop()

	// 测试1: 初始设置
	err := cache.PushMap("mymap", map[string]int{"a": 1, "b": 2}, time.Minute)
	assert.Nil(t, err)

	item := cache.Get("mymap")
	assert.NotNil(t, item)
	expectedMap := map[string]int{"a": 1, "b": 2}
	actualMap := item.Value()
	assert.Equal(t, len(actualMap), len(expectedMap))
	for k, v := range expectedMap {
		assert.Equal(t, actualMap[k], v)
	}

	// 测试2: 合并新值
	err = cache.PushMap("mymap", map[string]int{"c": 3, "d": 4}, time.Minute)
	assert.Nil(t, err)

	item = cache.Get("mymap")
	assert.NotNil(t, item)
	expectedMap = map[string]int{"a": 1, "b": 2, "c": 3, "d": 4}
	actualMap = item.Value()
	assert.Equal(t, len(actualMap), len(expectedMap))
	for k, v := range expectedMap {
		assert.Equal(t, actualMap[k], v)
	}
}

type SizedItem struct {
	id int
	s  int64
}

func (s *SizedItem) Size() int64 {
	return s.s
}

func forEachKeys[T any](cache *Cache[T]) []string {
	keys := make([]string, 0, 10)
	cache.ForEachFunc(func(key string, i *Item[T]) bool {
		if key == "stop" {
			return false
		}
		keys = append(keys, key)
		return true
	})
	sort.Strings(keys)
	return keys
}
