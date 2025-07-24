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
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/alibaba/sentinel-golang/core/base"
)

func TestNewContext(t *testing.T) {
	ctx := NewContext()
	if ctx == nil {
		t.Fatal("NewContext() should not return nil")
	}

	if ctx.userContext == nil {
		t.Error("userContext should be initialized")
	}

	if len(ctx.userContext) != 0 {
		t.Error("userContext should be empty initially")
	}
}

func TestContext_SetContext(t *testing.T) {
	tests := []struct {
		name  string
		ctx   *Context
		key   string
		value interface{}
	}{
		{"normal case", NewContext(), "test-key", "test-value"},
		{"empty key", NewContext(), "", "value"},
		{"nil value", NewContext(), "key", nil},
		{"integer value", NewContext(), "int-key", 42},
		{"complex value", NewContext(), "map-key", map[string]string{"nested": "value"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.ctx.SetContext(tt.key, tt.value)

			actual := tt.ctx.GetContext(tt.key)
			if !reflect.DeepEqual(actual, tt.value) {
				t.Errorf("Expected %v, got %v", tt.value, actual)
			}
		})
	}
}

func TestContext_SetContext_NilContext(t *testing.T) {
	var ctx *Context = nil

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("SetContext on nil context should not panic, got: %v", r)
		}
	}()

	ctx.SetContext("key", "value")
}

func TestContext_GetContext(t *testing.T) {
	ctx := NewContext()

	// Test getting non-existent key
	value := ctx.GetContext("non-existent")
	if value != nil {
		t.Errorf("Expected nil for non-existent key, got %v", value)
	}

	// Test getting existing key
	expectedValue := "test-value"
	ctx.SetContext("test-key", expectedValue)

	actualValue := ctx.GetContext("test-key")
	if actualValue != expectedValue {
		t.Errorf("Expected %v, got %v", expectedValue, actualValue)
	}
}

func TestContext_GetContext_NilContext(t *testing.T) {
	var ctx *Context = nil

	value := ctx.GetContext("any-key")
	if value != nil {
		t.Errorf("Expected nil from nil context, got %v", value)
	}
}

func TestContext_GetContext_NilUserContext(t *testing.T) {
	ctx := &Context{userContext: nil}

	value := ctx.GetContext("any-key")
	if value != nil {
		t.Errorf("Expected nil from context with nil userContext, got %v", value)
	}
}

func TestContext_ConcurrentReadWrite(t *testing.T) {
	ctx := NewContext()
	const numGoroutines = 50
	const numOperations = 1000

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*2)

	// Start concurrent writers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := "writer-" + strconv.Itoa(writerID) + "-key-" + strconv.Itoa(j)
				value := "writer-" + strconv.Itoa(writerID) + "-value-" + strconv.Itoa(j)
				ctx.SetContext(key, value)

				// Verify immediately
				if got := ctx.GetContext(key); got != value {
					errors <- fmt.Errorf("writer %d: expected %v, got %v", writerID, value, got)
					return
				}
			}
		}(i)
	}

	// Start concurrent readers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				// Try to read from different writers
				writerID := j % numGoroutines
				key := "writer-" + strconv.Itoa(writerID) + "-key-" + strconv.Itoa(j)

				// Reading might return nil if writer hasn't written yet, that's ok
				ctx.GetContext(key)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// Wait with timeout
	select {
	case <-done:
		// All good
	case err := <-errors:
		t.Fatal(err)
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out")
	}

	// Check if any errors occurred
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}

func TestContext_DataRace(t *testing.T) {
	ctx := NewContext()
	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup

	// Multiple goroutines writing to the same key
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := "shared-key"
				value := "goroutine-" + strconv.Itoa(id) + "-iteration-" + strconv.Itoa(j)
				ctx.SetContext(key, value)

				// Read it back
				ctx.GetContext(key)
			}
		}(i)
	}

	// Multiple goroutines reading
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				ctx.GetContext("shared-key")
				ctx.GetContext("non-existent-key")
			}
		}()
	}

	wg.Wait()
}

func TestContext_LazylInitialization(t *testing.T) {
	// Create context with nil userContext
	ctx := &Context{}

	// SetContext should initialize userContext
	ctx.SetContext("key", "value")

	if ctx.userContext == nil {
		t.Error("userContext should be initialized after SetContext")
	}

	// Verify value was set
	if got := ctx.GetContext("key"); got != "value" {
		t.Errorf("Expected 'value', got %v", got)
	}
}

func TestExtractContextFromArgs(t *testing.T) {
	llmCtx := NewContext()
	llmCtx.SetContext("test", "value")

	tests := []struct {
		name     string
		ctx      *base.EntryContext
		expected *Context
	}{
		{
			"nil entry context",
			nil,
			nil,
		},
		{
			"no args",
			&base.EntryContext{Input: &base.SentinelInput{Args: []interface{}{}}},
			nil,
		},
		{
			"wrong type args",
			&base.EntryContext{Input: &base.SentinelInput{Args: []interface{}{"string", 42, map[string]string{}}}},
			nil,
		},
		{
			"correct context in args",
			&base.EntryContext{Input: &base.SentinelInput{Args: []interface{}{"other", llmCtx, "more"}}},
			llmCtx,
		},
		{
			"multiple contexts, return first",
			&base.EntryContext{Input: &base.SentinelInput{Args: []interface{}{llmCtx, NewContext()}}},
			llmCtx,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractContextFromArgs(tt.ctx)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestExtractContextFromData(t *testing.T) {
	llmCtx := NewContext()
	llmCtx.SetContext("test", "value")

	tests := []struct {
		name     string
		ctx      *base.EntryContext
		expected *Context
	}{
		{
			"nil entry context",
			nil,
			nil,
		},
		{
			"no data",
			&base.EntryContext{Data: map[interface{}]interface{}{}},
			nil,
		},
		{
			"wrong key",
			&base.EntryContext{Data: map[interface{}]interface{}{"wrong-key": llmCtx}},
			nil,
		},
		{
			"correct key wrong type",
			&base.EntryContext{Data: map[interface{}]interface{}{KeyContext: "not-context"}},
			nil,
		},
		{
			"correct key and type",
			&base.EntryContext{Data: map[interface{}]interface{}{KeyContext: llmCtx}},
			llmCtx,
		},
		{
			"mixed data",
			&base.EntryContext{Data: map[interface{}]interface{}{
				"other-key": "value",
				KeyContext:  llmCtx,
				"more-key":  42,
			}},
			llmCtx,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractContextFromData(tt.ctx)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestContext_ConcurrentSetSameKey(t *testing.T) {
	ctx := NewContext()
	const numGoroutines = 20
	const key = "concurrent-key"

	var wg sync.WaitGroup
	values := make([]string, numGoroutines)

	// Each goroutine sets a unique value
	for i := 0; i < numGoroutines; i++ {
		values[i] = "value-" + strconv.Itoa(i)
		wg.Add(1)
		go func(id int, value string) {
			defer wg.Done()
			ctx.SetContext(key, value)
		}(i, values[i])
	}

	wg.Wait()

	// Verify one of the values is set (race condition, any value is valid)
	finalValue := ctx.GetContext(key)
	if finalValue == nil {
		t.Error("Expected some value to be set")
	}

	// Verify the final value is one of the values we set
	found := false
	for _, expectedValue := range values {
		if finalValue == expectedValue {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Final value %v is not one of the expected values %v", finalValue, values)
	}
}

func TestContext_ConcurrentGetNonExistentKey(t *testing.T) {
	ctx := NewContext()
	const numGoroutines = 50

	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := "non-existent-" + strconv.Itoa(id)
			value := ctx.GetContext(key)
			if value != nil {
				t.Errorf("Expected nil for non-existent key, got %v", value)
			}
		}(i)
	}

	wg.Wait()
}

func TestContext_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	ctx := NewContext()
	const numGoroutines = 100
	const numOperations = 10000

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			localCtx := NewContext()

			for j := 0; j < numOperations; j++ {
				key := "stress-key-" + strconv.Itoa(j%100) // Limited key space
				value := "stress-value-" + strconv.Itoa(id) + "-" + strconv.Itoa(j)

				// Mix of operations on shared and local context
				if j%3 == 0 {
					ctx.SetContext(key, value)
				} else if j%3 == 1 {
					ctx.GetContext(key)
				} else {
					localCtx.SetContext(key, value)
					if got := localCtx.GetContext(key); got != value {
						errors <- fmt.Errorf("goroutine %d: expected %v, got %v", id, value, got)
						return
					}
				}
			}
		}(i)
	}

	// Wait with timeout
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
	case <-time.After(30 * time.Second):
		t.Fatal("Stress test timed out")
	}

	// Check for any accumulated errors
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}

// Benchmark tests
func BenchmarkContext_SetContext(b *testing.B) {
	ctx := NewContext()
	key := "benchmark-key"
	value := "benchmark-value"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			ctx.SetContext(key+strconv.Itoa(i), value+strconv.Itoa(i))
			i++
		}
	})
}

func BenchmarkContext_GetContext(b *testing.B) {
	ctx := NewContext()
	// Pre-populate with data
	for i := 0; i < 1000; i++ {
		ctx.SetContext("key-"+strconv.Itoa(i), "value-"+strconv.Itoa(i))
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			ctx.GetContext("key-" + strconv.Itoa(i%1000))
			i++
		}
	})
}

func BenchmarkContext_SetGet(b *testing.B) {
	ctx := NewContext()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := "key-" + strconv.Itoa(i%100)
			value := "value-" + strconv.Itoa(i)
			ctx.SetContext(key, value)
			ctx.GetContext(key)
			i++
		}
	})
}

func BenchmarkExtractContextFromArgs(b *testing.B) {
	llmCtx := NewContext()
	entryCtx := &base.EntryContext{
		Input: &base.SentinelInput{
			Args: []interface{}{"string", 42, llmCtx, "more"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractContextFromArgs(entryCtx)
	}
}

func BenchmarkExtractContextFromData(b *testing.B) {
	llmCtx := NewContext()
	entryCtx := &base.EntryContext{
		Data: map[interface{}]interface{}{
			"key1":     "value1",
			KeyContext: llmCtx,
			"key2":     42,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractContextFromData(entryCtx)
	}
}

// Test helper functions
func TestContext_EdgeCases(t *testing.T) {
	t.Run("empty string key", func(t *testing.T) {
		ctx := NewContext()
		ctx.SetContext("", "empty-key-value")
		if got := ctx.GetContext(""); got != "empty-key-value" {
			t.Errorf("Expected 'empty-key-value', got %v", got)
		}
	})

	t.Run("nil value", func(t *testing.T) {
		ctx := NewContext()
		ctx.SetContext("nil-value", nil)
		if got := ctx.GetContext("nil-value"); got != nil {
			t.Errorf("Expected nil, got %v", got)
		}
	})

	t.Run("overwrite value", func(t *testing.T) {
		ctx := NewContext()
		ctx.SetContext("key", "value1")
		ctx.SetContext("key", "value2")
		if got := ctx.GetContext("key"); got != "value2" {
			t.Errorf("Expected 'value2', got %v", got)
		}
	})

	t.Run("complex data types", func(t *testing.T) {
		ctx := NewContext()
		complexValue := struct {
			Field1 string
			Field2 int
			Field3 []string
		}{
			Field1: "test",
			Field2: 42,
			Field3: []string{"a", "b", "c"},
		}

		ctx.SetContext("complex", complexValue)
		if got := ctx.GetContext("complex"); !reflect.DeepEqual(got, complexValue) {
			t.Errorf("Expected %v, got %v", complexValue, got)
		}
	})
}
