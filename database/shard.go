// 本文件：shard 保存数据和期限，getShard 路由 key，lockKeys 为多 key 命令统一加锁次序。

package database

import (
	"sort"
	"sync"
)

// expiring 额外保存带期限的 key，让后台清理能在 O(1) 时间随机取样；只用 map 做不到。
// expires 中的 expiration.index 记录 key 在 expiring 里的位置，供删除时用末项补洞。
// 修改数据或过期索引时，调用者必须持有分片写锁。
type shard struct {
	mu       sync.RWMutex
	data     map[string]any
	expires  map[string]expiration
	expiring []string
}

// 保留 FNV-1 的先乘后异或顺序，改变散列会改变 key 所属的分片。
func fnv32(key string) uint32 {
	hash := uint32(2166136261)
	for i := 0; i < len(key); i++ {
		hash *= 16777619
		hash ^= uint32(key[i])
	}
	return hash
}

func (e *Engine) getShard(key string) *shard {
	return e.shards[fnv32(key)&(e.shardCount-1)]
}

// 统一按分片编号加锁，避免 DEL a b 与 DEL b a 互相等待。
// 同一分片只锁一次；返回的闭包负责逆序释放，调用者应立即 defer。
func (e *Engine) lockKeys(keys [][]byte, write bool) func() {
	seen := make(map[int]bool, len(keys))
	indices := make([]int, 0, len(keys))
	for _, key := range keys {
		index := int(fnv32(string(key)) & (e.shardCount - 1))
		if !seen[index] {
			seen[index] = true
			indices = append(indices, index)
		}
	}
	sort.Ints(indices)
	for _, i := range indices {
		if write {
			e.shards[i].mu.Lock()
		} else {
			e.shards[i].mu.RLock()
		}
	}
	return func() {
		for i := len(indices) - 1; i >= 0; i-- {
			if write {
				e.shards[indices[i]].mu.Unlock()
			} else {
				e.shards[indices[i]].mu.RUnlock()
			}
		}
	}
}
