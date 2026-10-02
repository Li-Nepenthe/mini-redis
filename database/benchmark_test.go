package database

import (
	"fmt"
	"sync/atomic"
	"testing"
)

// BenchmarkEngine 使用 b 的并行计时，比较 1/16/64 分片在约 90% 读、全写与 80% 单 key 热点（约半写）下的耗时和分配。
// 预填 10000 key/64 字节值后 ResetTimer；每 worker 用独立伪随机序列，避免共享随机锁污染被测竞争。
// 不启动 TCP、TTL worker 或 AOF，ns/op 是规定负载的平均操作成本，不是网络吞吐或 P99。
func BenchmarkEngine(b *testing.B) {
	const keyCount = 10000
	for _, workload := range []string{"read90", "write100", "hot80"} {
		// 两层子基准分别固定负载与分片数；每格重新建引擎，不共享前一格的状态/计时。
		b.Run(workload, func(b *testing.B) {
			for _, shards := range []uint32{1, 16, 64} {
				b.Run(fmt.Sprintf("shards%d", shards), func(b *testing.B) {
					engine := NewEngine(shards)
					reads, writes := make([][][]byte, keyCount), make([][][]byte, keyCount)
					value := []byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
					for i := 0; i < keyCount; i++ {
						key := []byte(fmt.Sprintf("key:%05d", i))
						reads[i] = [][]byte{[]byte("GET"), key}
						writes[i] = [][]byte{[]byte("SET"), key, value}
						if _, err := engine.Exec(writes[i]); err != nil {
							b.Fatal(err)
						}
					}
					var workerID atomic.Uint64
					b.ReportAllocs()
					b.ResetTimer()
					b.RunParallel(func(pb *testing.PB) {
						// 每个 worker 使用独立固定序列；不用共享随机数锁改变待测竞争。
						state := workerID.Add(1)
						for pb.Next() {
							state ^= state << 13
							state ^= state >> 7
							state ^= state << 17
							keyIndex := int(state % keyCount)
							write := state>>32%10 == 0
							if workload == "write100" {
								write = true
							} else if workload == "hot80" {
								if state>>16%10 < 8 {
									keyIndex = 0
								}
								write = state>>32&1 == 0
							}
							args := reads[keyIndex]
							if write {
								args = writes[keyIndex]
							}
							if _, err := engine.Exec(args); err != nil {
								b.Error(err)
								return
							}
						}
					})
				})
			}
		})
	}
}
