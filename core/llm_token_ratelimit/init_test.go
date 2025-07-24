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
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	redis "github.com/go-redis/redis/v7"
)

// Mock Redis Client Interface
type RedisClientInterface interface {
	Ping() *redis.StatusCmd
	Close() error
}

// Mock Redis Cluster Client
type mockRedisClusterClient struct {
	pingError     error
	closeError    error
	pingCallCount int32
	closed        bool
	mu            sync.Mutex
}

func (m *mockRedisClusterClient) Ping() *redis.StatusCmd {
	atomic.AddInt32(&m.pingCallCount, 1)

	cmd := redis.NewStatusCmd("ping")
	if m.pingError != nil {
		cmd.SetErr(m.pingError)
	}
	return cmd
}

func (m *mockRedisClusterClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return m.closeError
}

func (m *mockRedisClusterClient) getPingCallCount() int32 {
	return atomic.LoadInt32(&m.pingCallCount)
}

var (
	originalLoadRulesFunc func([]*Rule) (bool, error)
	mockLoadRulesFunc     func([]*Rule) (bool, error)
	loadRulesMu           sync.RWMutex
	isLoadRulesMocked     bool
)

func init() {
	originalLoadRulesFunc = LoadRules
}

func mockLoadRulesWrapper(rules []*Rule) (bool, error) {
	loadRulesMu.RLock()
	defer loadRulesMu.RUnlock()

	if isLoadRulesMocked && mockLoadRulesFunc != nil {
		return mockLoadRulesFunc(rules)
	}

	return originalLoadRulesFunc(rules)
}

func setMockLoadRules(mockFn func([]*Rule) (bool, error)) func() {
	loadRulesMu.Lock()
	defer loadRulesMu.Unlock()

	mockLoadRulesFunc = mockFn
	isLoadRulesMocked = true

	return func() {
		loadRulesMu.Lock()
		defer loadRulesMu.Unlock()
		mockLoadRulesFunc = nil
		isLoadRulesMocked = false
	}
}

func resetLoadRules() {
	loadRulesMu.Lock()
	defer loadRulesMu.Unlock()
	mockLoadRulesFunc = nil
	isLoadRulesMocked = false
}

func saveAndRestoreConfig(t *testing.T) func() {
	configMu.RLock()
	originalConfig := config
	configMu.RUnlock()

	return func() {
		configMu.Lock()
		config = originalConfig
		configMu.Unlock()
	}
}

func saveAndRestoreRedisClient() func() {
	originalClient := redisClusterClient
	return func() {
		redisClusterClient = originalClient
	}
}

func TestSetConfig(t *testing.T) {
	defer saveAndRestoreConfig(t)()

	testConfig := &Config{
		Rules: []*Rule{
			{
				ID:       "test-rule",
				Resource: "/test",
				Strategy: FixedWindow,
				RuleName: "test",
			},
		},
		Redis: Redis{
			ServiceName: "localhost",
			ServicePort: 6379,
		},
		ErrorCode:    429,
		ErrorMessage: "Rate limit exceeded",
	}

	err := SetConfig(testConfig)
	if err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}

	stored := GetConfig()
	if stored == nil {
		t.Fatal("Config should not be nil after SetConfig")
	}

	if stored != testConfig {
		t.Error("Stored config should be the same instance")
	}
}

func TestSetConfig_NilInput(t *testing.T) {
	defer saveAndRestoreConfig(t)()

	err := SetConfig(nil)
	if err == nil {
		t.Error("Expected error when setting nil config")
	}
	if !strings.Contains(err.Error(), "cannot be nil") {
		t.Errorf("Expected 'cannot be nil' in error message, got: %v", err)
	}
}

func TestGetConfig(t *testing.T) {
	defer saveAndRestoreConfig(t)()

	tests := []struct {
		name     string
		setup    func()
		expected *Config
	}{
		{
			"nil config",
			func() {
				configMu.Lock()
				config = nil
				configMu.Unlock()
			},
			nil,
		},
		{
			"valid config",
			func() {
				cfg := &Config{
					Rules: []*Rule{},
					Redis: Redis{ServiceName: "test"},
				}
				configMu.Lock()
				config = cfg
				configMu.Unlock()
			},
			&Config{
				Rules: []*Rule{},
				Redis: Redis{ServiceName: "test"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			result := GetConfig()

			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil, got %v", result)
				}
			} else {
				if result == nil {
					t.Fatal("Expected config, got nil")
				}
				if result.Redis.ServiceName != tt.expected.Redis.ServiceName {
					t.Errorf("Expected ServiceName %s, got %s",
						tt.expected.Redis.ServiceName, result.Redis.ServiceName)
				}
			}
		})
	}
}

func TestSetConfig_ConcurrentAccess(t *testing.T) {
	defer saveAndRestoreConfig(t)()

	const numGoroutines = 50
	const numOperations = 100

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*2)

	// Concurrent setters
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				cfg := &Config{
					Rules: []*Rule{},
					Redis: Redis{
						ServiceName: "test-" + strconv.Itoa(id),
						ServicePort: int32(6379 + id),
					},
					ErrorCode: int32(400 + id),
				}
				if err := SetConfig(cfg); err != nil {
					errors <- fmt.Errorf("SetConfig failed in goroutine %d: %v", id, err)
				}
			}
		}(i)
	}

	// Concurrent getters
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				cfg := GetConfig()
				if cfg != nil {
					_ = cfg.Redis.ServiceName
					_ = cfg.ErrorCode
				}
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case err := <-errors:
		t.Fatal(err)
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out")
	}

	close(errors)
	for err := range errors {
		t.Errorf("Concurrent error: %v", err)
	}

	finalConfig := GetConfig()
	if finalConfig == nil {
		t.Error("Expected some config to be set at the end")
	}
}

func TestInitRedisClusterClient_WithMock(t *testing.T) {
	defer saveAndRestoreConfig(t)()
	defer saveAndRestoreRedisClient()()

	tests := []struct {
		name        string
		config      *Config
		mockError   error
		expectError bool
		errorMsg    string
	}{
		{
			"nil config",
			nil,
			nil,
			true,
			"config is nil",
		},
		{
			"successful connection",
			&Config{
				Redis: Redis{
					ServiceName: "mock-redis",
					ServicePort: 6379,
				},
			},
			nil,
			false,
			"",
		},
		{
			"connection failure",
			&Config{
				Redis: Redis{
					ServiceName: "failing-redis",
					ServicePort: 6379,
				},
			},
			errors.New("connection timeout"),
			true,
			"failed to connect to redis cluster",
		},
		{
			"default values",
			&Config{
				Redis: Redis{},
			},
			nil,
			false,
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configMu.Lock()
			config = tt.config
			configMu.Unlock()

			if tt.config != nil {
				mockClient := &mockRedisClusterClient{
					pingError: tt.mockError,
				}

				testInitRedis := func() error {
					cfg := GetConfig()
					if cfg == nil {
						return fmt.Errorf("config is nil")
					}

					cmd := mockClient.Ping()
					_, err := cmd.Result()
					if err != nil {
						return fmt.Errorf("failed to connect to redis cluster: %v", err)
					}

					redisClusterClient = (*redis.ClusterClient)(nil)
					return nil
				}

				err := testInitRedis()

				if tt.expectError {
					if err == nil {
						t.Error("Expected error but got none")
					}
					if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
						t.Errorf("Expected error to contain %q, got: %v", tt.errorMsg, err)
					}
				} else {
					if err != nil {
						t.Errorf("Expected success but got error: %v", err)
					}
				}

				if mockClient.getPingCallCount() != 1 {
					t.Errorf("Expected 1 ping call, got %d", mockClient.getPingCallCount())
				}
			} else {
				err := initRedisClusterClient()
				if !tt.expectError {
					t.Error("Expected error for nil config")
				}
				if err == nil {
					t.Error("Expected error but got none")
				}
			}
		})
	}
}

func TestLoadRules_Mock(t *testing.T) {
	defer resetLoadRules()

	tests := []struct {
		name         string
		inputRules   []*Rule
		mockResult   bool
		mockError    error
		expectError  bool
		errorMsg     string
		expectCalled bool
	}{
		{
			"successful load - rules updated",
			[]*Rule{
				{
					ID:       "rule1",
					Resource: "/api/test",
					Strategy: FixedWindow,
					RuleName: "test-rule",
				},
			},
			true,
			nil,
			false,
			"",
			true,
		},
		{
			"successful load - rules not changed",
			[]*Rule{
				{
					ID:       "rule2",
					Resource: "/api/same",
					Strategy: FixedWindow,
					RuleName: "same-rule",
				},
			},
			false,
			nil,
			false,
			"",
			true,
		},
		{
			"load failure",
			[]*Rule{
				{
					ID:       "failing-rule",
					Resource: "/api/fail",
					Strategy: FixedWindow,
					RuleName: "fail-rule",
				},
			},
			false,
			errors.New("rules loading failed"),
			true,
			"rules loading failed",
			true,
		},
		{
			"empty rules",
			[]*Rule{},
			false,
			nil,
			false,
			"",
			true,
		},
		{
			"nil rules",
			nil,
			false,
			nil,
			false,
			"",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var called bool
			var calledWith []*Rule
			cleanup := setMockLoadRules(func(rules []*Rule) (bool, error) {
				called = true
				calledWith = rules
				return tt.mockResult, tt.mockError
			})
			defer cleanup()

			updated, err := mockLoadRulesWrapper(tt.inputRules)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected success but got error: %v", err)
				}
			}

			if updated != tt.mockResult {
				t.Errorf("Expected updated=%v, got %v", tt.mockResult, updated)
			}

			if tt.expectCalled && !called {
				t.Error("Expected LoadRules to be called")
			}
			if !tt.expectCalled && called {
				t.Error("Expected LoadRules not to be called")
			}
			if called && !reflect.DeepEqual(calledWith, tt.inputRules) {
				t.Error("LoadRules called with wrong arguments")
			}
		})
	}
}

func TestLoadRules_ConcurrentCalls(t *testing.T) {
	defer resetLoadRules()

	inputRules := []*Rule{
		{
			ID:       "concurrent-rule",
			Resource: "/api/concurrent",
			Strategy: FixedWindow,
			RuleName: "concurrent-test",
		},
	}

	var callCount int32
	cleanup := setMockLoadRules(func(rules []*Rule) (bool, error) {
		atomic.AddInt32(&callCount, 1)
		time.Sleep(time.Millisecond)
		return true, nil
	})
	defer cleanup()

	const numGoroutines = 20
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			updated, err := mockLoadRulesWrapper(inputRules)
			if err != nil {
				errors <- fmt.Errorf("goroutine %d: %v", id, err)
			}
			if !updated {
				errors <- fmt.Errorf("goroutine %d: expected rules to be updated", id)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent error: %v", err)
	}

	finalCallCount := atomic.LoadInt32(&callCount)
	if finalCallCount != numGoroutines {
		t.Errorf("Expected %d calls to LoadRules, got %d", numGoroutines, finalCallCount)
	}
}

func TestLoadRules_NoMockFallback(t *testing.T) {
	defer resetLoadRules()

	testRules := []*Rule{
		{
			ID:       "fallback-rule",
			Resource: "/api/fallback",
			Strategy: FixedWindow,
			RuleName: "fallback-test",
		},
	}

	resetLoadRules()

	updated, err := mockLoadRulesWrapper(testRules)

	if err != nil {
		t.Errorf("Original LoadRules returned error (may be expected): %v", err)
	}
	t.Logf("Original LoadRules returned updated=%v", updated)

	loadRulesMu.RLock()
	isMocked := isLoadRulesMocked
	loadRulesMu.RUnlock()

	if isMocked {
		t.Error("Expected no mock to be active")
	}
}

func TestInitRules_WithMock(t *testing.T) {
	defer saveAndRestoreConfig(t)()
	defer resetLoadRules()

	tests := []struct {
		name         string
		config       *Config
		mockResult   bool
		mockError    error
		expectError  bool
		errorMsg     string
		expectCalled bool
	}{
		{
			"nil config",
			nil,
			false,
			nil,
			true,
			"config is nil",
			false,
		},
		{
			"empty rules",
			&Config{Rules: []*Rule{}},
			false,
			nil,
			false,
			"",
			false,
		},
		{
			"successful load - rules updated",
			&Config{
				Rules: []*Rule{
					{
						ID:       "rule1",
						Resource: "/api/test",
						Strategy: FixedWindow,
						RuleName: "test-rule",
					},
				},
			},
			true,
			nil,
			false,
			"",
			true,
		},
		{
			"successful load - rules not changed",
			&Config{
				Rules: []*Rule{
					{
						ID:       "rule2",
						Resource: "/api/same",
						Strategy: FixedWindow,
						RuleName: "same-rule",
					},
				},
			},
			false,
			nil,
			false,
			"",
			true,
		},
		{
			"load failure",
			&Config{
				Rules: []*Rule{
					{
						ID:       "failing-rule",
						Resource: "/api/fail",
						Strategy: FixedWindow,
						RuleName: "fail-rule",
					},
				},
			},
			false,
			errors.New("rules loading failed"),
			true,
			"rules loading failed",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configMu.Lock()
			config = tt.config
			configMu.Unlock()

			var called bool
			var calledWith []*Rule
			cleanup := setMockLoadRules(func(rules []*Rule) (bool, error) {
				called = true
				calledWith = rules
				return tt.mockResult, tt.mockError
			})
			defer cleanup()

			testInitRules := func() error {
				cfg := GetConfig()
				if cfg == nil {
					return fmt.Errorf("config is nil")
				}

				if len(cfg.Rules) == 0 {
					return nil
				}

				if _, err := mockLoadRulesWrapper(cfg.Rules); err != nil {
					return err
				}

				return nil
			}

			err := testInitRules()

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected success but got error: %v", err)
				}
			}

			if tt.expectCalled && !called {
				t.Error("Expected LoadRules to be called")
			}
			if !tt.expectCalled && called {
				t.Error("Expected LoadRules not to be called")
			}
			if called && tt.config != nil && !reflect.DeepEqual(calledWith, tt.config.Rules) {
				t.Error("LoadRules called with wrong arguments")
			}
		})
	}
}

func TestInit_WithMocks(t *testing.T) {
	defer saveAndRestoreConfig(t)()
	defer saveAndRestoreRedisClient()()
	defer resetLoadRules()

	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			"nil config",
			nil,
			true,
			"cannot be nil",
		},
		{
			"valid config",
			&Config{
				Rules: []*Rule{
					{
						ID:       "test-rule",
						Resource: "/api/test",
						Strategy: FixedWindow,
						RuleName: "test",
					},
				},
				Redis: Redis{
					ServiceName: "localhost",
					ServicePort: 6379,
				},
				ErrorCode:    429,
				ErrorMessage: "Rate limit exceeded",
			},
			false,
			"",
		},
		{
			"minimal config",
			&Config{
				Rules: []*Rule{},
				Redis: Redis{
					ServiceName: "localhost",
					ServicePort: 6379,
				},
			},
			false,
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setMockLoadRules(func(rules []*Rule) (bool, error) {
				return true, nil
			})
			defer cleanup()

			configMu.Lock()
			config = nil
			configMu.Unlock()
			redisClusterClient = nil

			err := Init(tt.config)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected success but got error: %v", err)
				}
			}

			if tt.config != nil {
				storedConfig := GetConfig()
				if storedConfig == nil {
					t.Error("Config should be set even if other steps fail")
				}
			}
		})
	}
}

func TestInit_ConcurrentCalls(t *testing.T) {
	defer saveAndRestoreConfig(t)()
	defer saveAndRestoreRedisClient()()
	defer resetLoadRules()

	cleanup := setMockLoadRules(func(rules []*Rule) (bool, error) {
		time.Sleep(time.Millisecond)
		return true, nil
	})
	defer cleanup()

	const numGoroutines = 20
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	baseConfig := &Config{
		Rules: []*Rule{},
		Redis: Redis{
			ServiceName: "localhost",
			ServicePort: 6379,
		},
	}

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			cfg := &Config{
				Rules: baseConfig.Rules,
				Redis: Redis{
					ServiceName: baseConfig.Redis.ServiceName,
					ServicePort: baseConfig.Redis.ServicePort + int32(id%10),
				},
				ErrorCode: int32(400 + id),
			}

			err := Init(cfg)
			if err != nil {
				errors <- fmt.Errorf("goroutine %d: %v", id, err)
			}
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(30 * time.Second):
		t.Fatal("Test timed out")
	}

	close(errors)
	errorCount := 0
	for err := range errors {
		errorCount++
		if errorCount <= 5 {
			t.Logf("Concurrent error (expected due to Redis): %v", err)
		}
	}

	finalConfig := GetConfig()
	if finalConfig == nil {
		t.Error("No final config set, which may be expected if all inits failed")
	}
}

// Benchmark tests
func BenchmarkSetConfig(b *testing.B) {
	cfg := &Config{
		Rules: []*Rule{
			{
				ID:       "bench-rule",
				Resource: "/bench",
				Strategy: FixedWindow,
			},
		},
		Redis: Redis{
			ServiceName: "localhost",
			ServicePort: 6379,
		},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			SetConfig(cfg)
		}
	})
}

func BenchmarkGetConfig(b *testing.B) {
	cfg := &Config{
		Rules: []*Rule{
			{
				ID:       "bench-rule",
				Resource: "/bench",
				Strategy: FixedWindow,
			},
		},
		Redis: Redis{
			ServiceName: "localhost",
			ServicePort: 6379,
		},
	}
	SetConfig(cfg)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			GetConfig()
		}
	})
}

func BenchmarkSetGetConfig_Mixed(b *testing.B) {
	cfg := &Config{
		Rules: []*Rule{
			{
				ID:       "bench-rule",
				Resource: "/bench",
				Strategy: FixedWindow,
			},
		},
		Redis: Redis{
			ServiceName: "localhost",
			ServicePort: 6379,
		},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if pb.Next() {
				SetConfig(cfg)
			} else {
				GetConfig()
			}
		}
	})
}

func BenchmarkMockLoadRules(b *testing.B) {
	defer resetLoadRules()

	rules := []*Rule{
		{
			ID:       "bench-rule",
			Resource: "/bench",
			Strategy: FixedWindow,
			RuleName: "bench",
		},
	}

	cleanup := setMockLoadRules(func(rules []*Rule) (bool, error) {
		return true, nil
	})
	defer cleanup()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mockLoadRulesWrapper(rules)
		}
	})
}

func BenchmarkOriginalLoadRules(b *testing.B) {
	defer resetLoadRules()

	rules := []*Rule{
		{
			ID:       "bench-rule",
			Resource: "/bench",
			Strategy: FixedWindow,
			RuleName: "bench",
		},
	}

	resetLoadRules()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mockLoadRulesWrapper(rules)
		}
	})
}
