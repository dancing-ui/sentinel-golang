// Copyright 1999-2020 Alibaba Group Holding Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"math/rand"

	"github.com/alibaba/sentinel-golang/util"
	"github.com/go-redis/redis/v7"
)

type BenchmarkLogger struct {
	logFile *os.File
	logger  *log.Logger
	wg      sync.WaitGroup
}

func NewBenchmarkLogger(filename string) (*BenchmarkLogger, error) {
	if err := os.MkdirAll("benchmark_logs", 0755); err != nil {
		return nil, err
	}

	timestamp := time.Now().Format("20060102_150405")
	fullPath := fmt.Sprintf("benchmark_logs/%s_%s.log", filename, timestamp)

	file, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	logger := log.New(file, "", log.LstdFlags|log.Lmicroseconds)

	return &BenchmarkLogger{
		logFile: file,
		logger:  logger,
	}, nil
}

func (bl *BenchmarkLogger) Printf(format string, v ...interface{}) {
	bl.wg.Add(1)
	go func() {
		defer bl.wg.Done()
		message := fmt.Sprintf(format, v...)
		bl.logger.Print(message)
	}()
}

func (bl *BenchmarkLogger) PrintfSync(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	bl.logger.Print(message)
}

func (bl *BenchmarkLogger) Close() {
	if bl.logFile != nil {
		bl.logFile.Close()
	}
}

type PredictionPattern interface {
	GetEstimatedTokens(timestamp int64) int
	GetActualTokens(estimated int, timestamp int64) int
}

type ExtremeUnderestimatePattern struct {
	BaseTokens        int
	UnderestimateRate float64
	startTime         int64
}

func (p *ExtremeUnderestimatePattern) GetEstimatedTokens(timestamp int64) int {
	if p.startTime == 0 {
		p.startTime = timestamp
	}

	actualWillConsume := p.BaseTokens + rand.Intn(200)
	estimatedTokens := int(float64(actualWillConsume) * p.UnderestimateRate)

	return estimatedTokens
}

func (p *ExtremeUnderestimatePattern) GetActualTokens(estimated int, timestamp int64) int {
	return int(float64(estimated)/p.UnderestimateRate) + rand.Intn(20)
}

type PETASimulator struct {
	redisClient    *redis.ClusterClient
	keyPrefix      string
	TimeWindow     int64 // Second
	TokenSize      int
	withholdScript string
	correctScript  string
}

func loadLuaScript(scriptPath string) string {
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		return ""
	}
	return string(data)
}

func NewPETASimulator(keyPrefix string, timeWindow int64, tokenSize int) *PETASimulator {
	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    []string{"127.0.0.1:6379"},
		Username: "redis",
		Password: "redis",
	})
	if err := client.Ping().Err(); err != nil {
		fmt.Printf("Failed to connect to Redis: %v\n", err)
		return nil
	}
	if err := client.FlushDB().Err(); err != nil {
		fmt.Printf("Failed to flush Redis database: %v\n", err)
		return nil
	}
	return &PETASimulator{
		redisClient:    client,
		keyPrefix:      keyPrefix,
		TimeWindow:     timeWindow,
		TokenSize:      tokenSize,
		withholdScript: loadLuaScript("../../../../core/llm_token_ratelimit/script/peta/peta_withhold.lua"),
		correctScript:  loadLuaScript("../../../../core/llm_token_ratelimit/script/peta/peta_correct.lua"),
	}
}

func (p *PETASimulator) Exit() {
	if p.redisClient != nil {
		p.redisClient.Close()
	}
}

func (p *PETASimulator) Withhold(ctx *context.Context, slidingWindowKey string, tokenBucketKey string, estimatedToken int) (int, error) {
	keys := []string{slidingWindowKey, tokenBucketKey}
	args := []interface{}{estimatedToken, util.CurrentTimeMillis(), p.TokenSize, p.TimeWindow * 1000}

	response, err := p.redisClient.Eval(p.withholdScript, keys, args...).Result()
	if err != nil {
		return 0, fmt.Errorf("error executing withhold script: %w", err)
	}
	if response == nil {
		return 0, fmt.Errorf("withhold script returned nil response")
	}
	results, ok := response.([]interface{})
	if !ok || len(results) != 1 {
		return 0, fmt.Errorf("withhold script returned unexpected response format: %v", response)
	}
	waitingTime, ok := results[0].(int64)
	if !ok {
		return 0, fmt.Errorf("withhold script returned unexpected waiting time format: %v", results[0])
	}
	if waitingTime < 0 {
		return 0, fmt.Errorf("withhold script returned negative waiting time: %d", waitingTime)
	}
	return int(waitingTime), nil
}

func (p *PETASimulator) Correct(ctx *context.Context, slidingWindowKey string, tokenBucketKey string, estimatedToken int, actualToken int) (int, error) {
	keys := []string{slidingWindowKey, tokenBucketKey}
	args := []interface{}{estimatedToken, util.CurrentTimeMillis(), p.TokenSize, p.TimeWindow * 1000, actualToken}

	response, err := p.redisClient.Eval(p.correctScript, keys, args...).Result()
	if err != nil {
		return 0, fmt.Errorf("error executing correct script: %w", err)
	}
	if response == nil {
		return 0, fmt.Errorf("correct script returned nil response")
	}
	results, ok := response.([]interface{})
	if !ok || len(results) != 1 {
		return 0, fmt.Errorf("correct script returned unexpected response format: %v", response)
	}
	correctResult, ok := results[0].(int64)
	if !ok {
		return 0, fmt.Errorf("correct script returned unexpected correct result format: %v", results[0])
	}
	return int(correctResult), nil
}

type BenchmarkStats struct {
	mu                sync.Mutex
	totalCorrectTime  time.Duration
	maxCorrectTime    time.Duration
	correctCount      int64
	totalWithholdTime time.Duration
	maxWithholdTime   time.Duration
	withholdCount     int64
	passCount         int64
	correctErrorCount int64
	maxWaitingTime    time.Duration
	totalWaitingTime  time.Duration
	waitingCount      int64
	actualCount       int64
	estimatedCount    int64
}

func (s *BenchmarkStats) AddCorrectTime(duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalCorrectTime += duration
	s.correctCount++
	if duration > s.maxCorrectTime {
		s.maxCorrectTime = duration
	}
}

func (s *BenchmarkStats) AddWithholdTime(duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalWithholdTime += duration
	s.withholdCount++
	if duration > s.maxWithholdTime {
		s.maxWithholdTime = duration
	}
}

func (s *BenchmarkStats) AddWaitingTime(duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalWaitingTime += duration
	s.waitingCount++
	if duration > s.maxWaitingTime {
		s.maxWaitingTime = duration
	}
}

func (s *BenchmarkStats) AddPass() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.passCount++
}

func (s *BenchmarkStats) AddCorrectError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.correctErrorCount++
}

func (s *BenchmarkStats) AddActualCount(actual int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.actualCount += int64(actual)
}

func (s *BenchmarkStats) AddEstimatedCount(estimated int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.estimatedCount += int64(estimated)
}

func runPETABenchmark(b *testing.B, pattern PredictionPattern, concurrency int, duraion time.Duration) {
	simulator := NewPETASimulator("peta-benchmark", 5, 10000)
	defer simulator.Exit()

	logger, _ := NewBenchmarkLogger("peta_benchmark")
	defer logger.Close()

	stats := &BenchmarkStats{}

	ctx := context.Background()

	var wg sync.WaitGroup

	startTime := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)

		slidingWindowKey := fmt.Sprintf("{%s}:sliding-window", simulator.keyPrefix)
		tokenBucketKey := fmt.Sprintf("{%s}:token-bucket", simulator.keyPrefix)

		go func(workerID int) {
			defer wg.Done()

			for time.Since(startTime) < duraion {
				estimated := pattern.GetEstimatedTokens(int64(util.CurrentTimeMillis()))

				withholdStart := time.Now()
				waitingTime, err := simulator.Withhold(&ctx, slidingWindowKey, tokenBucketKey, estimated)
				withholdDuration := time.Since(withholdStart)
				go stats.AddWithholdTime(withholdDuration)

				if err != nil {
					logger.Printf("Worker %d withhold error: %v", workerID, err)
					continue
				}

				logger.Printf("Worker %d costed %d estimated tokens", workerID, estimated)
				go stats.AddEstimatedCount(estimated)

				if waitingTime != 0 {
					go stats.AddWaitingTime(time.Duration(waitingTime) * time.Millisecond)
					logger.Printf("Worker %d waiting for %d milliseconds", workerID, waitingTime)
					time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond) // Simulate waiting
					continue
				}

				time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond) // Simulate some processing time

				actual := pattern.GetActualTokens(estimated, int64(util.CurrentTimeMillis()))

				logger.Printf("Worker %d costed %d actual tokens", workerID, actual)
				go stats.AddActualCount(actual)

				correctStart := time.Now()
				correctResult, err := simulator.Correct(&ctx, slidingWindowKey, tokenBucketKey, estimated, actual)
				correctDuration := time.Since(correctStart)
				go stats.AddCorrectTime(correctDuration)

				if err != nil || correctResult == 0 {
					go stats.AddCorrectError()
					logger.Printf("Worker %d correct error: %v", workerID, err)
					continue
				}

				go stats.AddPass()
			}
		}(i)
	}

	wg.Wait()

	logger.wg.Wait()

	logger.PrintfSync("=== Performance Statistics ===")
	logger.PrintfSync("Withhold - count: %v, max_used_duration: %v, total_used_time: %v", stats.withholdCount, stats.maxWithholdTime, stats.totalWithholdTime.String())
	logger.PrintfSync("Correct - count: %v, max_used_duration: %v, total_used_time: %v, error_count: %v", stats.correctCount, stats.maxCorrectTime, stats.totalCorrectTime.String(), stats.correctErrorCount)
	logger.PrintfSync("Waiting - count: %v, max_used_duration: %v, total_used_time: %v", stats.waitingCount, stats.maxWaitingTime, stats.totalWaitingTime.String())
	logger.PrintfSync("Block - count: %v", stats.waitingCount)
	logger.PrintfSync("Pass - count: %v", stats.passCount)
	logger.PrintfSync("Actual Tokens - count: %v", stats.actualCount)
}

func BenchmarkPETA_ExtremeUnderestimate(b *testing.B) {
	pattern := &ExtremeUnderestimatePattern{
		BaseTokens:        1000,
		UnderestimateRate: 0.02,
	}

	concurrency := 1
	duration := 30 * time.Second

	b.ResetTimer()
	runPETABenchmark(b, pattern, concurrency, duration)
	b.StopTimer()
}
