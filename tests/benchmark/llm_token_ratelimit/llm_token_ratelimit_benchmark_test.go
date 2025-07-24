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
	if err := sentinel.InitDefault(); err != nil {
		panic(err)
	}

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
								Number:        10000,
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
								Number:        1000000,
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

func BenchmarkEntryBasic(b *testing.B) {
	InitSentinel()

	resource := "/api/test"

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

		ctx.SetContext(llmtokenratelimit.KeyUsedTokenInfos,
			llmtokenratelimit.GenerateUsedTokenInfos(
				llmtokenratelimit.WithInputTokens(100)))
		e.SetPair(llmtokenratelimit.KeyContext, ctx)
		e.Exit()
	}
}

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

func BenchmarkEntryWithComplexHeaders(b *testing.B) {
	InitSentinel()

	resource := "/api/complex-headers"

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

func BenchmarkEntryErrorCases(b *testing.B) {
	InitSentinel()

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

	for i := 0; i < b.N; i++ {
		jobChan <- i
	}
	close(jobChan)

	wg.Wait()
}

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

	var totalLatency time.Duration
	var maxLatency time.Duration
	var minLatency = time.Hour

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
