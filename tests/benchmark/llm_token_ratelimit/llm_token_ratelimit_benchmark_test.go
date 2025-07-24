// Copyright 1999-2020 Alibaba Group Holding Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package llmtokenratelimit

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
	llmtokenratelimit "github.com/alibaba/sentinel-golang/core/llm_token_ratelimit"
)

func InitSentinel() {
	// 初始化 Sentinel
	if err := sentinel.InitDefault(); err != nil {
		panic(err)
	}

	// 加载测试规则
	_, err := llmtokenratelimit.LoadRules([]*llmtokenratelimit.Rule{
		{
			Resource: "/api/.*",
			Strategy: llmtokenratelimit.FixedWindow,
			RuleName: "api-rule",
			RuleItems: []*llmtokenratelimit.RuleItem{
				{
					Identifier: llmtokenratelimit.Identifier{
						Type:  llmtokenratelimit.Header,
						Value: ".*",
					},
					KeyItems: []*llmtokenratelimit.KeyItem{
						{
							Key: ".*",
							Token: llmtokenratelimit.Token{
								Number:        10000, // 高限制，避免被阻塞
								CountStrategy: llmtokenratelimit.InputTokens,
							},
							Time: llmtokenratelimit.Time{
								Unit:  llmtokenratelimit.Second,
								Value: 1,
							},
						},
					},
				},
			},
		},
		{
			Resource: "/high-freq/.*",
			Strategy: llmtokenratelimit.FixedWindow,
			RuleName: "high-freq-rule",
			RuleItems: []*llmtokenratelimit.RuleItem{
				{
					Identifier: llmtokenratelimit.Identifier{
						Type:  llmtokenratelimit.AllIdentifier,
						Value: ".*",
					},
					KeyItems: []*llmtokenratelimit.KeyItem{
						{
							Key: ".*",
							Token: llmtokenratelimit.Token{
								Number:        1000000, // 超高限制
								CountStrategy: llmtokenratelimit.TotalTokens,
							},
							Time: llmtokenratelimit.Time{
								Unit:  llmtokenratelimit.Second,
								Value: 1,
							},
						},
					},
				},
			},
		},
	})

	if err != nil {
		panic(err)
	}
}

// BenchmarkEntryBasic 基础 Entry 调用性能
func BenchmarkEntryBasic(b *testing.B) {
	InitSentinel()

	resource := "/api/test"

	// 准备上下文
	ctx := new(llmtokenratelimit.Context)
	ctx.SetContext(llmtokenratelimit.KeyRequestInfos,
		llmtokenratelimit.GenerateRequestInfos(
			llmtokenratelimit.WithHeader(map[string]string{
				"X-User-ID": "user123",
			})))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		e, blocked := sentinel.Entry(resource,
			sentinel.WithTrafficType(base.Inbound),
			sentinel.WithArgs(ctx))

		if blocked != nil {
			continue
		}

		// 模拟处理完成
		ctx.SetContext(llmtokenratelimit.KeyUsedTokenInfos,
			llmtokenratelimit.GenerateUsedTokenInfos(
				llmtokenratelimit.WithInputTokens(100)))
		e.SetPair(llmtokenratelimit.KeyContext, ctx)
		e.Exit()
	}
}

// BenchmarkEntryWithDifferentResources 不同资源的性能对比
func BenchmarkEntryWithDifferentResources(b *testing.B) {
	InitSentinel()

	resources := []string{
		"/api/v1/chat",
		"/api/v1/completion",
		"/api/v2/chat",
		"/api/v2/completion",
	}

	for _, resource := range resources {
		b.Run(fmt.Sprintf("Resource_%s", resource), func(b *testing.B) {
			ctx := new(llmtokenratelimit.Context)
			ctx.SetContext(llmtokenratelimit.KeyRequestInfos,
				llmtokenratelimit.GenerateRequestInfos(
					llmtokenratelimit.WithHeader(map[string]string{
						"X-API-Key": "test-key",
					})))

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				e, blocked := sentinel.Entry(resource,
					sentinel.WithTrafficType(base.Inbound),
					sentinel.WithArgs(ctx))

				if blocked != nil {
					continue
				}

				ctx.SetContext(llmtokenratelimit.KeyUsedTokenInfos,
					llmtokenratelimit.GenerateUsedTokenInfos(
						llmtokenratelimit.WithInputTokens(50)))
				e.SetPair(llmtokenratelimit.KeyContext, ctx)
				e.Exit()
			}
		})
	}
}

// BenchmarkEntryWithVaryingTokens 不同Token数量的性能对比
func BenchmarkEntryWithVaryingTokens(b *testing.B) {
	InitSentinel()

	tokenCounts := []int{10, 100, 1000, 5000, 10000}
	resource := "/api/tokens"

	for _, tokenCount := range tokenCounts {
		b.Run(fmt.Sprintf("Tokens_%d", tokenCount), func(b *testing.B) {
			ctx := new(llmtokenratelimit.Context)
			ctx.SetContext(llmtokenratelimit.KeyRequestInfos,
				llmtokenratelimit.GenerateRequestInfos(
					llmtokenratelimit.WithHeader(map[string]string{
						"X-User-ID": "benchmark-user",
					})))

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				e, blocked := sentinel.Entry(resource,
					sentinel.WithTrafficType(base.Inbound),
					sentinel.WithArgs(ctx))

				if blocked != nil {
					continue
				}

				ctx.SetContext(llmtokenratelimit.KeyUsedTokenInfos,
					llmtokenratelimit.GenerateUsedTokenInfos(
						llmtokenratelimit.WithInputTokens(int64(tokenCount))))
				e.SetPair(llmtokenratelimit.KeyContext, ctx)
				e.Exit()
			}
		})
	}
}

// BenchmarkEntryConcurrent 并发访问性能
func BenchmarkEntryConcurrent(b *testing.B) {
	InitSentinel()

	resource := "/high-freq/concurrent"

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			ctx := new(llmtokenratelimit.Context)
			ctx.SetContext(llmtokenratelimit.KeyRequestInfos,
				llmtokenratelimit.GenerateRequestInfos(
					llmtokenratelimit.WithHeader(map[string]string{
						"X-Worker-ID": fmt.Sprintf("worker-%d", i%100),
					})))

			e, blocked := sentinel.Entry(resource,
				sentinel.WithTrafficType(base.Inbound),
				sentinel.WithArgs(ctx))

			if blocked != nil {
				continue
			}

			ctx.SetContext(llmtokenratelimit.KeyUsedTokenInfos,
				llmtokenratelimit.GenerateUsedTokenInfos(
					llmtokenratelimit.WithInputTokens(100)))
			e.SetPair(llmtokenratelimit.KeyContext, ctx)
			e.Exit()

			i++
		}
	})
}

// BenchmarkEntryMemoryUsage 内存使用性能测试
func BenchmarkEntryMemoryUsage(b *testing.B) {
	InitSentinel()

	resource := "/api/memory-test"

	b.ResetTimer()
	b.ReportAllocs()

	var memStatsBefore, memStatsAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStatsBefore)

	for i := 0; i < b.N; i++ {
		ctx := new(llmtokenratelimit.Context)
		ctx.SetContext(llmtokenratelimit.KeyRequestInfos,
			llmtokenratelimit.GenerateRequestInfos(
				llmtokenratelimit.WithHeader(map[string]string{
					"X-Session-ID": fmt.Sprintf("session-%d", i),
				})))

		e, blocked := sentinel.Entry(resource,
			sentinel.WithTrafficType(base.Inbound),
			sentinel.WithArgs(ctx))

		if blocked != nil {
			fmt.Println("Blocked")
			continue
		}

		ctx.SetContext(llmtokenratelimit.KeyUsedTokenInfos,
			llmtokenratelimit.GenerateUsedTokenInfos(
				llmtokenratelimit.WithInputTokens(2000)))
		e.SetPair(llmtokenratelimit.KeyContext, ctx)
		e.Exit()
	}

	runtime.GC()
	runtime.ReadMemStats(&memStatsAfter)

	b.ReportMetric(float64(memStatsAfter.Alloc-memStatsBefore.Alloc)/float64(b.N), "bytes/op")
}

// BenchmarkEntryWithComplexHeaders 复杂Header场景
func BenchmarkEntryWithComplexHeaders(b *testing.B) {
	InitSentinel()

	resource := "/api/complex-headers"

	// 复杂的Header数据
	complexHeaders := map[string]string{
		"Authorization":    "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
		"X-Request-ID":     "req-12345-67890-abcdef",
		"X-User-Agent":     "MyApp/1.0.0 (iOS 15.0; iPhone13,2)",
		"X-Client-Version": "2.1.3",
		"X-Tenant-ID":      "tenant-enterprise-premium",
		"X-Feature-Flags":  "feature1,feature2,feature3,feature4",
		"X-Locale":         "en-US",
		"X-Timezone":       "America/New_York",
		"X-Device-ID":      "device-uuid-1234567890abcdef",
		"X-Session-Token":  "session-token-9876543210fedcba",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := new(llmtokenratelimit.Context)
		ctx.SetContext(llmtokenratelimit.KeyRequestInfos,
			llmtokenratelimit.GenerateRequestInfos(
				llmtokenratelimit.WithHeader(complexHeaders)))

		e, blocked := sentinel.Entry(resource,
			sentinel.WithTrafficType(base.Inbound),
			sentinel.WithArgs(ctx))

		if blocked != nil {
			continue
		}

		ctx.SetContext(llmtokenratelimit.KeyUsedTokenInfos,
			llmtokenratelimit.GenerateUsedTokenInfos(
				llmtokenratelimit.WithInputTokens(150),
				llmtokenratelimit.WithOutputTokens(75)))
		e.SetPair(llmtokenratelimit.KeyContext, ctx)
		e.Exit()
	}
}

// BenchmarkEntryErrorCases 错误场景性能
func BenchmarkEntryErrorCases(b *testing.B) {
	InitSentinel()

	// 测试没有规则匹配的资源
	resource := "/no-match/resource"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := new(llmtokenratelimit.Context)
		ctx.SetContext(llmtokenratelimit.KeyRequestInfos,
			llmtokenratelimit.GenerateRequestInfos())

		e, blocked := sentinel.Entry(resource,
			sentinel.WithTrafficType(base.Inbound),
			sentinel.WithArgs(ctx))

		if blocked != nil {
			continue
		}

		e.Exit()
	}
}

// BenchmarkEntryScaleTest 规模化测试
func BenchmarkEntryScaleTest(b *testing.B) {
	InitSentinel()

	const numWorkers = 100
	const numResources = 10

	resources := make([]string, numResources)
	for i := 0; i < numResources; i++ {
		resources[i] = fmt.Sprintf("/api/scale/resource-%d", i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	var wg sync.WaitGroup
	jobChan := make(chan int, b.N)

	// 启动工作协程
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for range jobChan {
				resource := resources[workerID%numResources]

				ctx := new(llmtokenratelimit.Context)
				ctx.SetContext(llmtokenratelimit.KeyRequestInfos,
					llmtokenratelimit.GenerateRequestInfos(
						llmtokenratelimit.WithHeader(map[string]string{
							"X-Worker-ID": fmt.Sprintf("worker-%d", workerID),
						})))

				e, blocked := sentinel.Entry(resource,
					sentinel.WithTrafficType(base.Inbound),
					sentinel.WithArgs(ctx))

				if blocked != nil {
					continue
				}

				ctx.SetContext(llmtokenratelimit.KeyUsedTokenInfos,
					llmtokenratelimit.GenerateUsedTokenInfos(
						llmtokenratelimit.WithInputTokens(100)))
				e.SetPair(llmtokenratelimit.KeyContext, ctx)
				e.Exit()
			}
		}(w)
	}

	// 分发任务
	for i := 0; i < b.N; i++ {
		jobChan <- i
	}
	close(jobChan)

	wg.Wait()
}

// BenchmarkEntryLatencyProfile 延迟分析
func BenchmarkEntryLatencyProfile(b *testing.B) {
	InitSentinel()

	resource := "/api/latency-test"

	latencies := make([]time.Duration, b.N)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		start := time.Now()

		ctx := new(llmtokenratelimit.Context)
		ctx.SetContext(llmtokenratelimit.KeyRequestInfos,
			llmtokenratelimit.GenerateRequestInfos())

		e, blocked := sentinel.Entry(resource,
			sentinel.WithTrafficType(base.Inbound),
			sentinel.WithArgs(ctx))

		if blocked == nil {
			ctx.SetContext(llmtokenratelimit.KeyUsedTokenInfos,
				llmtokenratelimit.GenerateUsedTokenInfos(
					llmtokenratelimit.WithInputTokens(100)))
			e.SetPair(llmtokenratelimit.KeyContext, ctx)
			e.Exit()
		}

		latencies[i] = time.Since(start)
	}

	// 计算延迟统计
	var totalLatency time.Duration
	var maxLatency time.Duration
	var minLatency = time.Hour // 初始化为一个很大的值

	for _, lat := range latencies {
		totalLatency += lat
		if lat > maxLatency {
			maxLatency = lat
		}
		if lat < minLatency {
			minLatency = lat
		}
	}

	avgLatency := totalLatency / time.Duration(b.N)

	b.ReportMetric(float64(avgLatency.Nanoseconds()), "avg-ns/op")
	b.ReportMetric(float64(maxLatency.Nanoseconds()), "max-ns/op")
	b.ReportMetric(float64(minLatency.Nanoseconds()), "min-ns/op")
}
