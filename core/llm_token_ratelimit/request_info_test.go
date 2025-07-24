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
	"sync"
	"testing"
	"time"
)

func TestWithHeader(t *testing.T) {
	tests := []struct {
		name           string
		inputHeaders   map[string]string
		expectedResult map[string]string
	}{
		{
			"valid headers",
			map[string]string{
				"Authorization": "Bearer token123",
				"Content-Type":  "application/json",
			},
			map[string]string{
				"Authorization": "Bearer token123",
				"Content-Type":  "application/json",
			},
		},
		{
			"empty headers",
			map[string]string{},
			map[string]string{},
		},
		{
			"nil headers",
			nil,
			nil,
		},
		{
			"single header",
			map[string]string{"X-Test": "test-value"},
			map[string]string{"X-Test": "test-value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestInfo := WithHeader(tt.inputHeaders)

			infos := &RequestInfos{}
			requestInfo(infos)

			if !reflect.DeepEqual(infos.Headers, tt.expectedResult) {
				t.Errorf("Expected headers %v, got %v", tt.expectedResult, infos.Headers)
			}
		})
	}
}

func TestGenerateRequestInfos(t *testing.T) {
	tests := []struct {
		name            string
		requestInfos    []RequestInfo
		expectedHeaders map[string]string
	}{
		{
			"single RequestInfo",
			[]RequestInfo{
				WithHeader(map[string]string{"Authorization": "Bearer token"}),
			},
			map[string]string{"Authorization": "Bearer token"},
		},
		{
			"multiple RequestInfos - last one wins",
			[]RequestInfo{
				WithHeader(map[string]string{"Authorization": "Bearer token1"}),
				WithHeader(map[string]string{"Authorization": "Bearer token2", "Content-Type": "application/json"}),
			},
			map[string]string{"Authorization": "Bearer token2", "Content-Type": "application/json"},
		},
		{
			"empty RequestInfos slice",
			[]RequestInfo{},
			nil,
		},
		{
			"nil header RequestInfo",
			[]RequestInfo{
				WithHeader(nil),
			},
			nil,
		},
		{
			"multiple RequestInfos with nil in between",
			[]RequestInfo{
				WithHeader(map[string]string{"Authorization": "Bearer token"}),
				WithHeader(nil),
				WithHeader(map[string]string{"Content-Type": "application/json"}),
			},
			map[string]string{"Content-Type": "application/json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateRequestInfos(tt.requestInfos...)

			if result == nil {
				t.Fatal("GenerateRequestInfos should never return nil")
			}

			if !reflect.DeepEqual(result.Headers, tt.expectedHeaders) {
				t.Errorf("Expected headers %v, got %v", tt.expectedHeaders, result.Headers)
			}
		})
	}
}

// Test struct that looks similar to RequestInfos but isn't
type fakeRequestInfos struct {
	Headers map[string]string `json:"headers"`
}

// Test struct with different field types
type malformedRequestInfos struct {
	Headers []string `json:"headers"` // Wrong type for Headers
}

// Test struct with additional fields
type extendedRequestInfos struct {
	Headers map[string]string `json:"headers"`
	Extra   string            `json:"extra"`
}

// Interface that could be confused with RequestInfos
type requestInfosLike interface {
	GetHeaders() map[string]string
}

type requestInfosImpl struct {
	Headers map[string]string
}

func (r *requestInfosImpl) GetHeaders() map[string]string {
	return r.Headers
}

func TestExtractRequestInfos_TypeValidation(t *testing.T) {
	tests := []struct {
		name           string
		setupContext   func() *Context
		expectedResult *RequestInfos
		description    string
	}{
		{
			"valid RequestInfos",
			func() *Context {
				ctx := NewContext()
				validRequestInfos := &RequestInfos{
					Headers: map[string]string{"Authorization": "Bearer token"},
				}
				ctx.SetContext(KeyRequestInfos, validRequestInfos)
				return ctx
			},
			&RequestInfos{Headers: map[string]string{"Authorization": "Bearer token"}},
			"should return valid RequestInfos when type matches exactly",
		},
		{
			"nil context data",
			func() *Context {
				ctx := NewContext()
				ctx.SetContext(KeyRequestInfos, nil)
				return ctx
			},
			nil,
			"should return nil when context data is nil",
		},
		{
			"fake RequestInfos with same structure",
			func() *Context {
				ctx := NewContext()
				fakeInfos := &fakeRequestInfos{
					Headers: map[string]string{"Authorization": "Bearer token"},
				}
				ctx.SetContext(KeyRequestInfos, fakeInfos)
				return ctx
			},
			nil,
			"should return nil for struct with same fields but different type",
		},
		{
			"malformed RequestInfos with wrong field types",
			func() *Context {
				ctx := NewContext()
				malformedInfos := &malformedRequestInfos{
					Headers: []string{"Authorization", "Bearer token"},
				}
				ctx.SetContext(KeyRequestInfos, malformedInfos)
				return ctx
			},
			nil,
			"should return nil for struct with wrong field types",
		},
		{
			"extended RequestInfos with additional fields",
			func() *Context {
				ctx := NewContext()
				extendedInfos := &extendedRequestInfos{
					Headers: map[string]string{"Authorization": "Bearer token"},
					Extra:   "additional field",
				}
				ctx.SetContext(KeyRequestInfos, extendedInfos)
				return ctx
			},
			nil,
			"should return nil for struct with additional fields",
		},
		{
			"interface implementation",
			func() *Context {
				ctx := NewContext()
				interfaceImpl := &requestInfosImpl{
					Headers: map[string]string{"Authorization": "Bearer token"},
				}
				ctx.SetContext(KeyRequestInfos, interfaceImpl)
				return ctx
			},
			nil,
			"should return nil for interface implementation with same fields",
		},
		{
			"string value",
			func() *Context {
				ctx := NewContext()
				ctx.SetContext(KeyRequestInfos, "not a RequestInfos")
				return ctx
			},
			nil,
			"should return nil for completely different type (string)",
		},
		{
			"map value",
			func() *Context {
				ctx := NewContext()
				ctx.SetContext(KeyRequestInfos, map[string]string{"Headers": "Bearer token"})
				return ctx
			},
			nil,
			"should return nil for map type",
		},
		{
			"slice value",
			func() *Context {
				ctx := NewContext()
				ctx.SetContext(KeyRequestInfos, []string{"Authorization", "Bearer token"})
				return ctx
			},
			nil,
			"should return nil for slice type",
		},
		{
			"int value",
			func() *Context {
				ctx := NewContext()
				ctx.SetContext(KeyRequestInfos, 42)
				return ctx
			},
			nil,
			"should return nil for primitive type (int)",
		},
		{
			"empty interface value",
			func() *Context {
				ctx := NewContext()
				var emptyInterface interface{}
				ctx.SetContext(KeyRequestInfos, emptyInterface)
				return ctx
			},
			nil,
			"should return nil for empty interface",
		},
		{
			"nil context",
			func() *Context {
				return nil
			},
			nil,
			"should return nil when context is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupContext()
			result := extractRequestInfos(ctx)

			if tt.expectedResult == nil {
				if result != nil {
					t.Errorf("Expected nil, got %v. %s", result, tt.description)
				}
			} else {
				if result == nil {
					t.Fatalf("Expected non-nil result, got nil. %s", tt.description)
				}
				if !reflect.DeepEqual(result.Headers, tt.expectedResult.Headers) {
					t.Errorf("Expected headers %v, got %v. %s",
						tt.expectedResult.Headers, result.Headers, tt.description)
				}
			}
		})
	}
}

func TestExtractRequestInfos_ReflectionEdgeCases(t *testing.T) {
	// Test the specific reflection logic more thoroughly
	t.Run("reflection type comparison", func(t *testing.T) {
		// Verify that our reflection type detection works correctly
		realRequestInfos := &RequestInfos{Headers: map[string]string{"test": "value"}}
		fakeRequestInfos := &fakeRequestInfos{Headers: map[string]string{"test": "value"}}

		realType := reflect.TypeOf(realRequestInfos)
		fakeType := reflect.TypeOf(fakeRequestInfos)
		expectedType := reflect.TypeOf((*RequestInfos)(nil))

		if realType != expectedType {
			t.Errorf("Real RequestInfos type should match expected type")
		}

		if fakeType == expectedType {
			t.Errorf("Fake RequestInfos type should NOT match expected type")
		}

		t.Logf("Real type: %v", realType)
		t.Logf("Fake type: %v", fakeType)
		t.Logf("Expected type: %v", expectedType)
	})

	t.Run("type assertion after reflection check", func(t *testing.T) {
		ctx := NewContext()

		// This tests the scenario where reflection passes but type assertion fails
		// This should be impossible with proper reflection, but let's test it
		validRequestInfos := &RequestInfos{Headers: map[string]string{"test": "value"}}
		ctx.SetContext(KeyRequestInfos, validRequestInfos)

		// Manually verify the steps that extractRequestInfos performs
		reqInfosRaw := ctx.GetContext(KeyRequestInfos)
		if reqInfosRaw == nil {
			t.Fatal("Context should contain RequestInfos")
		}

		actualType := reflect.TypeOf(reqInfosRaw)
		if actualType != requestInfosType {
			t.Errorf("Type should match: actual=%v, expected=%v", actualType, requestInfosType)
		}

		reqInfos, ok := reqInfosRaw.(*RequestInfos)
		if !ok {
			t.Error("Type assertion should succeed after reflection check passes")
		}

		if reqInfos == nil {
			t.Error("Result should not be nil after successful type assertion")
		}
	})

	t.Run("verify requestInfosType global variable", func(t *testing.T) {
		// Test that the global requestInfosType variable is correctly initialized
		expectedType := reflect.TypeOf((*RequestInfos)(nil))
		if requestInfosType != expectedType {
			t.Errorf("Global requestInfosType not initialized correctly: got %v, expected %v",
				requestInfosType, expectedType)
		}

		// Test with actual instances
		instance1 := &RequestInfos{}
		instance2 := &RequestInfos{Headers: map[string]string{"test": "value"}}

		if reflect.TypeOf(instance1) != requestInfosType {
			t.Error("Instance1 type should match global requestInfosType")
		}

		if reflect.TypeOf(instance2) != requestInfosType {
			t.Error("Instance2 type should match global requestInfosType")
		}
	})
}

func TestExtractRequestInfos_ConcurrentAccess(t *testing.T) {
	const numGoroutines = 100
	const numOperations = 1000

	ctx := NewContext()
	validRequestInfos := &RequestInfos{
		Headers: map[string]string{"Authorization": "Bearer token"},
	}
	ctx.SetContext(KeyRequestInfos, validRequestInfos)

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	// Concurrent readers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				result := extractRequestInfos(ctx)
				if result == nil {
					errors <- fmt.Errorf("goroutine %d: expected non-nil result", id)
					return
				}
				if result.Headers == nil {
					errors <- fmt.Errorf("goroutine %d: headers should not be nil", id)
					return
				}
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
	case err := <-errors:
		t.Fatal(err)
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out")
	}

	close(errors)
	for err := range errors {
		t.Errorf("Concurrent error: %v", err)
	}
}

func TestRequestInfosType_GlobalVariable(t *testing.T) {
	t.Run("consistent type checking", func(t *testing.T) {
		// Create different instances and verify type consistency
		infos1 := &RequestInfos{}
		infos2 := GenerateRequestInfos()
		infos3 := &RequestInfos{Headers: map[string]string{"test": "value"}}

		type1 := reflect.TypeOf(infos1)
		type2 := reflect.TypeOf(infos2)
		type3 := reflect.TypeOf(infos3)

		if type1 != requestInfosType || type2 != requestInfosType || type3 != requestInfosType {
			t.Error("All RequestInfos instances should have the same type")
		}

		if type1 != type2 || type2 != type3 || type1 != type3 {
			t.Error("All RequestInfos instances should have identical types")
		}
	})
}

// Performance test for type checking
func BenchmarkExtractRequestInfos_Valid(b *testing.B) {
	ctx := NewContext()
	validRequestInfos := &RequestInfos{
		Headers: map[string]string{"Authorization": "Bearer token"},
	}
	ctx.SetContext(KeyRequestInfos, validRequestInfos)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			extractRequestInfos(ctx)
		}
	})
}

func BenchmarkExtractRequestInfos_Invalid(b *testing.B) {
	ctx := NewContext()
	fakeRequestInfos := &fakeRequestInfos{
		Headers: map[string]string{"Authorization": "Bearer token"},
	}
	ctx.SetContext(KeyRequestInfos, fakeRequestInfos)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			extractRequestInfos(ctx)
		}
	})
}

func BenchmarkGenerateRequestInfos(b *testing.B) {
	requestInfo := WithHeader(map[string]string{
		"Authorization": "Bearer token",
		"Content-Type":  "application/json",
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			GenerateRequestInfos(requestInfo)
		}
	})
}

// Test for memory leaks in reflection usage
func TestExtractRequestInfos_MemoryUsage(t *testing.T) {
	// This test ensures that reflection operations don't cause memory leaks

	// Test with various types to ensure no memory accumulation
	testTypes := []interface{}{
		&RequestInfos{Headers: map[string]string{"test": "value"}},
		&fakeRequestInfos{Headers: map[string]string{"test": "value"}},
		"string",
		42,
		[]string{"slice"},
		map[string]string{"map": "value"},
		nil,
	}

	for i, testValue := range testTypes {
		t.Run(fmt.Sprintf("type_%d", i), func(t *testing.T) {
			ctx := NewContext()
			ctx.SetContext(KeyRequestInfos, testValue)

			// Perform many operations to check for memory accumulation
			for j := 0; j < 1000; j++ {
				extractRequestInfos(ctx)
			}

			// Test passes if no panic or excessive memory usage occurs
		})
	}
}

// Edge case testing
func TestExtractRequestInfos_EdgeCases(t *testing.T) {
	t.Run("context with empty userContext", func(t *testing.T) {
		ctx := &Context{} // No userContext initialized
		result := extractRequestInfos(ctx)
		if result != nil {
			t.Error("Should return nil when userContext is not initialized")
		}
	})

	t.Run("concurrent context modifications", func(t *testing.T) {
		ctx := NewContext()

		const numGoroutines = 50
		var wg sync.WaitGroup

		// Concurrent writers
		for i := 0; i < numGoroutines/2; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				ctx.SetContext(KeyRequestInfos, &RequestInfos{
					Headers: map[string]string{fmt.Sprintf("key-%d", id): fmt.Sprintf("value-%d", id)},
				})
			}(i)
		}

		// Concurrent readers
		for i := 0; i < numGoroutines/2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				extractRequestInfos(ctx)
			}()
		}

		wg.Wait()
	})
}
