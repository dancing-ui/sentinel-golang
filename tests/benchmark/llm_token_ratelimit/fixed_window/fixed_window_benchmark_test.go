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

	"github.com/go-redis/redis/v8"
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

type TokenPattern interface {
	GetActualTokens() int
}

type FixedTokenPattern struct {
	FixedTokens int
}

func (p *FixedTokenPattern) GetActualTokens() int {
	return p.FixedTokens
}

type FixedWindowSimulator struct {
	redisClient  *redis.ClusterClient
	keyPrefix    string
	TimeWindow   int64 // Second
	TokenSize    int
	queryScript  string
	updateScript string
}

func loadLuaScript(scriptPath string) string {
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		return ""
	}
	return string(data)
}

func NewFixedWindowSimulator(timeWindow int64, tokenSize int) *FixedWindowSimulator {
	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    []string{"127.0.0.1:6379"},
		Username: "redis",
		Password: "redis",
	})
	if err := client.Ping(context.TODO()).Err(); err != nil {
		fmt.Printf("Failed to connect to Redis: %v\n", err)
		return nil
	}
	if err := client.FlushDB(context.TODO()).Err(); err != nil {
		fmt.Printf("Failed to flush Redis database: %v\n", err)
		return nil
	}
	return &FixedWindowSimulator{
		redisClient:  client,
		TimeWindow:   timeWindow,
		TokenSize:    tokenSize,
		queryScript:  loadLuaScript("../../../../core/llm_token_ratelimit/script/fixed_window/query.lua"),
		updateScript: loadLuaScript("../../../../core/llm_token_ratelimit/script/fixed_window/update.lua"),
	}
}

func (p *FixedWindowSimulator) Exit() {
	if p.redisClient != nil {
		p.redisClient.Close()
	}
}

func (p *FixedWindowSimulator) Query(ctx *context.Context, key string) (int, error) {
	keys := []string{key}
	args := []interface{}{p.TokenSize, p.TimeWindow * 1000}

	response, err := p.redisClient.Eval(context.TODO(), p.queryScript, keys, args...).Result()
	if err != nil {
		return 0, fmt.Errorf("error executing query script: %w", err)
	}
	if response == nil {
		return 0, fmt.Errorf("query script returned nil response")
	}
	results, ok := response.([]interface{})
	if !ok || len(results) != 2 {
		return 0, fmt.Errorf("query script returned unexpected response format: %v", response)
	}
	remaining, ok := results[0].(int64)
	if !ok {
		return 0, fmt.Errorf("query script returned unexpected remaining token format: %v", results[0])
	}
	return int(remaining), nil
}

func (p *FixedWindowSimulator) Update(ctx *context.Context, key string, actualToken int) error {
	keys := []string{key}
	args := []interface{}{p.TokenSize, p.TimeWindow * 1000, actualToken}

	response, err := p.redisClient.Eval(context.TODO(), p.updateScript, keys, args...).Result()
	if err != nil {
		return fmt.Errorf("error executing update script: %w", err)
	}
	if response == nil {
		return fmt.Errorf("update script returned nil response")
	}
	results, ok := response.([]interface{})
	if !ok || len(results) != 2 {
		return fmt.Errorf("update script returned unexpected response format: %v", response)
	}
	return nil
}

func runFixedWindowBenchmark(b *testing.B, pattern TokenPattern, concurrency int, interval time.Duration) {
	simulator := NewFixedWindowSimulator(10, 65)
	defer simulator.Exit()

	logger, _ := NewBenchmarkLogger("fixed_window_benchmark")
	defer logger.Close()

	ctx := context.Background()

	var wg sync.WaitGroup

	var testOnce sync.Once

	for i := 0; i < concurrency; i++ {
		wg.Add(1)

		key := "test-fixed-window-key"

		go func(workerID int) {
			defer wg.Done()

			remaining, err := simulator.Query(&ctx, key)
			if err != nil {
				logger.PrintfSync("Worker %d query error: %v", workerID, err)
				return
			}
			logger.PrintfSync("Worker %d queried successfully", workerID)
			if remaining < 0 {
				return
			}
			testOnce.Do(func() {
				time.Sleep(time.Second * 2)
			})
			if err := simulator.Update(&ctx, key, pattern.GetActualTokens()); err != nil {
				logger.PrintfSync("Worker %d update error: %v", workerID, err)
				return
			}
			logger.PrintfSync("Worker %d updated successfully", workerID)
		}(i)
		time.Sleep(interval)
	}

	wg.Wait()

	logger.wg.Wait()
}

func BenchmarkFixedWindow_ExtremeUnderestimate(b *testing.B) {
	pattern := &FixedTokenPattern{
		FixedTokens: 60,
	}

	concurrency := 10

	b.ResetTimer()
	runFixedWindowBenchmark(b, pattern, concurrency, time.Second*1)
	b.StopTimer()
}
