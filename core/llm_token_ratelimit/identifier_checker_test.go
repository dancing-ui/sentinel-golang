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
	"sync"
	"testing"
	"time"

	"github.com/alibaba/sentinel-golang/util"
)

// Mock IdentifierChecker for testing
type mockIdentifierChecker struct {
	checkFunc func(*RequestInfos, Identifier, string) bool
	callCount int
}

func (m *mockIdentifierChecker) Check(infos *RequestInfos, identifier Identifier, pattern string) bool {
	m.callCount++
	if m.checkFunc != nil {
		return m.checkFunc(infos, identifier, pattern)
	}
	return false
}

func (m *mockIdentifierChecker) getCallCount() int {
	return m.callCount
}

func (m *mockIdentifierChecker) resetCallCount() {
	m.callCount = 0
}

// Helper function to save and restore globalRuleMatcher
func saveAndRestoreRuleMatcher(t *testing.T) func() {
	originalRuleMatcher := globalRuleMatcher
	return func() {
		globalRuleMatcher = originalRuleMatcher
	}
}

// Helper function to save and restore globalRuleMatcher for benchmarks
func saveAndRestoreRuleMatcherForBenchmark(b *testing.B) func() {
	originalRuleMatcher := globalRuleMatcher
	return func() {
		globalRuleMatcher = originalRuleMatcher
	}
}

func TestAllIdentifierChecker_Check(t *testing.T) {
	defer saveAndRestoreRuleMatcher(t)()

	tests := []struct {
		name               string
		setupMocks         func() map[IdentifierType]IdentifierChecker
		infos              *RequestInfos
		identifier         Identifier
		pattern            string
		expectedResult     bool
		expectedCallCounts map[IdentifierType]int
		description        string
	}{
		{
			"all checkers return false",
			func() map[IdentifierType]IdentifierChecker {
				return map[IdentifierType]IdentifierChecker{
					AllIdentifier: &AllIdentifierChecker{},
					Header:        &mockIdentifierChecker{checkFunc: func(*RequestInfos, Identifier, string) bool { return false }},
				}
			},
			&RequestInfos{Headers: map[string]string{"test": "value"}},
			Identifier{Type: AllIdentifier, Value: "test"},
			"value",
			false,
			map[IdentifierType]int{Header: 1},
			"returns false when no checker matches",
		},
		{
			"one checker returns true",
			func() map[IdentifierType]IdentifierChecker {
				return map[IdentifierType]IdentifierChecker{
					AllIdentifier: &AllIdentifierChecker{},
					Header:        &mockIdentifierChecker{checkFunc: func(*RequestInfos, Identifier, string) bool { return true }},
				}
			},
			&RequestInfos{Headers: map[string]string{"test": "value"}},
			Identifier{Type: AllIdentifier, Value: "test"},
			"value",
			true,
			map[IdentifierType]int{Header: 1},
			"returns true when at least one checker matches",
		},
		{
			"empty checker map",
			func() map[IdentifierType]IdentifierChecker {
				return map[IdentifierType]IdentifierChecker{
					AllIdentifier: &AllIdentifierChecker{},
				}
			},
			&RequestInfos{Headers: map[string]string{"test": "value"}},
			Identifier{Type: AllIdentifier, Value: "test"},
			"value",
			false,
			map[IdentifierType]int{},
			"returns false when no other checkers are present",
		},
		{
			"nil request infos",
			func() map[IdentifierType]IdentifierChecker {
				return map[IdentifierType]IdentifierChecker{
					AllIdentifier: &AllIdentifierChecker{},
					Header:        &mockIdentifierChecker{checkFunc: func(*RequestInfos, Identifier, string) bool { return false }},
				}
			},
			nil,
			Identifier{Type: AllIdentifier, Value: "test"},
			"value",
			false,
			map[IdentifierType]int{Header: 0},
			"handles nil request infos gracefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock checkers
			mockCheckers := tt.setupMocks()
			globalRuleMatcher = &RuleMatcher{
				IdentifierCheckers: mockCheckers,
			}

			checker := &AllIdentifierChecker{}
			result := checker.Check(tt.infos, tt.identifier, tt.pattern)

			if result != tt.expectedResult {
				t.Errorf("Expected %v, got %v. %s", tt.expectedResult, result, tt.description)
			}

			// Verify call counts
			for identifierType, expectedCount := range tt.expectedCallCounts {
				if mockChecker, ok := mockCheckers[identifierType].(*mockIdentifierChecker); ok {
					actualCount := mockChecker.getCallCount()
					if actualCount != expectedCount {
						t.Errorf("Expected %s checker to be called %d times, got %d",
							identifierType, expectedCount, actualCount)
					}
				}
			}
		})
	}
}

func TestHeaderChecker_Check(t *testing.T) {
	tests := []struct {
		name           string
		infos          *RequestInfos
		identifier     Identifier
		pattern        string
		expectedResult bool
		description    string
	}{
		{
			"exact match",
			&RequestInfos{
				Headers: map[string]string{
					"Authorization": "Bearer token123",
					"Content-Type":  "application/json",
				},
			},
			Identifier{Type: Header, Value: "Authorization"},
			"Bearer token123",
			true,
			"exact match for header key and value",
		},
		{
			"regex match key",
			&RequestInfos{
				Headers: map[string]string{
					"X-Custom-Header": "custom-value",
					"Content-Type":    "application/json",
				},
			},
			Identifier{Type: Header, Value: "X-.*-Header"},
			"custom-value",
			true,
			"regex pattern matches header key",
		},
		{
			"regex match value",
			&RequestInfos{
				Headers: map[string]string{
					"Authorization": "Bearer token123",
					"Content-Type":  "application/json",
				},
			},
			Identifier{Type: Header, Value: "Authorization"},
			"Bearer.*",
			true,
			"regex pattern matches header value",
		},
		{
			"regex match both key and value",
			&RequestInfos{
				Headers: map[string]string{
					"X-API-Key": "key_12345",
					"X-Token":   "token_67890",
				},
			},
			Identifier{Type: Header, Value: "X-.*"},
			".*_.*",
			true,
			"regex patterns match both key and value",
		},
		{
			"no match - wrong key",
			&RequestInfos{
				Headers: map[string]string{
					"Authorization": "Bearer token123",
					"Content-Type":  "application/json",
				},
			},
			Identifier{Type: Header, Value: "X-API-Key"},
			"Bearer token123",
			false,
			"no match when header key doesn't exist",
		},
		{
			"no match - wrong value",
			&RequestInfos{
				Headers: map[string]string{
					"Authorization": "Bearer token123",
					"Content-Type":  "application/json",
				},
			},
			Identifier{Type: Header, Value: "Authorization"},
			"Basic.*",
			false,
			"no match when header value doesn't match pattern",
		},
		{
			"nil request infos",
			nil,
			Identifier{Type: Header, Value: "Authorization"},
			"Bearer token123",
			false,
			"returns false when RequestInfos is nil",
		},
		{
			"empty headers map",
			&RequestInfos{Headers: map[string]string{}},
			Identifier{Type: Header, Value: "Authorization"},
			"Bearer token123",
			false,
			"returns false when headers map is empty",
		},
		{
			"headers map is nil",
			&RequestInfos{Headers: nil},
			Identifier{Type: Header, Value: "Authorization"},
			"Bearer token123",
			false,
			"returns false when headers map is nil",
		},
		{
			"empty identifier value",
			&RequestInfos{
				Headers: map[string]string{
					"Authorization": "Bearer token123",
				},
			},
			Identifier{Type: Header, Value: ""},
			"Bearer token123",
			false,
			"returns false when identifier value is empty",
		},
		{
			"empty pattern",
			&RequestInfos{
				Headers: map[string]string{
					"Authorization": "Bearer token123",
				},
			},
			Identifier{Type: Header, Value: "Authorization"},
			"",
			false,
			"returns false when pattern is empty",
		},
		{
			"case sensitive match",
			&RequestInfos{
				Headers: map[string]string{
					"authorization": "bearer token123",
				},
			},
			Identifier{Type: Header, Value: "Authorization"},
			"Bearer token123",
			false,
			"performs case-sensitive matching",
		},
		{
			"multiple headers, match any",
			&RequestInfos{
				Headers: map[string]string{
					"Authorization": "Bearer token123",
					"X-API-Key":     "api_key_456",
					"Content-Type":  "application/json",
				},
			},
			Identifier{Type: Header, Value: "X-API-Key"},
			"api_key_.*",
			true,
			"matches any header in the collection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &HeaderChecker{}
			result := checker.Check(tt.infos, tt.identifier, tt.pattern)

			if result != tt.expectedResult {
				t.Errorf("Expected %v, got %v. %s", tt.expectedResult, result, tt.description)
			}
		})
	}
}

func TestHeaderChecker_CrossValidation(t *testing.T) {
	// Cross-validation: verify HeaderChecker behavior against known regex behavior
	testCases := []struct {
		name         string
		headers      map[string]string
		keyPattern   string
		valuePattern string
		shouldMatch  bool
	}{
		{
			"literal match",
			map[string]string{"test": "value"},
			"test",
			"value",
			true,
		},
		{
			"dot wildcard",
			map[string]string{"test": "value"},
			"t.st",
			"val.e",
			true,
		},
		{
			"star quantifier",
			map[string]string{"test-header": "test-value-123"},
			"test.*",
			"test.*",
			true,
		},
		{
			"anchored regex",
			map[string]string{"test": "value"},
			"^test$",
			"^value$",
			true,
		},
		{
			"empty key",
			map[string]string{"": "value"},
			"^$",
			"^$",
			false,
		},
		{
			"empty value",
			map[string]string{"test": ""},
			"^$",
			"^$",
			false,
		},
		{
			"empty key and value",
			map[string]string{"": ""},
			"^$",
			"^$",
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// First verify util.RegexMatch behavior
			for key, value := range tc.headers {
				keyMatch := util.RegexMatch(tc.keyPattern, key)
				valueMatch := util.RegexMatch(tc.valuePattern, value)
				expectedRegexResult := keyMatch && valueMatch

				if expectedRegexResult != tc.shouldMatch {
					t.Errorf("Regex validation failed: key=%s, value=%s, keyPattern=%s, valuePattern=%s, expected=%v",
						key, value, tc.keyPattern, tc.valuePattern, tc.shouldMatch)
				}

				// Then verify HeaderChecker result matches regex result
				checker := &HeaderChecker{}
				infos := &RequestInfos{Headers: tc.headers}
				identifier := Identifier{Type: Header, Value: tc.keyPattern}

				checkerResult := checker.Check(infos, identifier, tc.valuePattern)
				if checkerResult != tc.shouldMatch {
					t.Errorf("HeaderChecker result doesn't match expected: got %v, expected %v",
						checkerResult, tc.shouldMatch)
				}
			}
		})
	}
}

func TestAllIdentifierChecker_Integration(t *testing.T) {
	defer saveAndRestoreRuleMatcher(t)()

	// Integration test: verify AllIdentifierChecker works with real HeaderChecker
	t.Run("integration with real HeaderChecker", func(t *testing.T) {
		globalRuleMatcher = NewDefaultRuleMatcher()

		infos := &RequestInfos{
			Headers: map[string]string{
				"Authorization": "Bearer token123",
				"X-API-Key":     "api_key_456",
			},
		}

		allChecker := &AllIdentifierChecker{}

		// Test matching case
		identifier := Identifier{Type: AllIdentifier, Value: "Authorization"}
		result := allChecker.Check(infos, identifier, "Bearer.*")
		if !result {
			t.Error("Expected true when HeaderChecker should match")
		}

		// Test non-matching case
		result = allChecker.Check(infos, identifier, "Basic.*")
		if result {
			t.Error("Expected false when HeaderChecker should not match")
		}
	})
}

func TestIdentifierChecker_ErrorCases(t *testing.T) {
	defer saveAndRestoreRuleMatcher(t)()

	t.Run("AllIdentifierChecker with nil globalRuleMatcher", func(t *testing.T) {
		// Test behavior when globalRuleMatcher is nil
		originalRuleMatcher := globalRuleMatcher
		globalRuleMatcher = nil
		defer func() {
			globalRuleMatcher = originalRuleMatcher
			if r := recover(); r != nil {
				t.Logf("Expected panic recovered: %v", r)
			}
		}()

		checker := &AllIdentifierChecker{}
		// This should panic because globalRuleMatcher is nil
		checker.Check(&RequestInfos{}, Identifier{}, "")
	})

	t.Run("HeaderChecker with invalid regex", func(t *testing.T) {
		// Test handling of invalid regular expressions
		checker := &HeaderChecker{}
		infos := &RequestInfos{
			Headers: map[string]string{
				"test": "value",
			},
		}

		// util.RegexMatch should handle invalid regular expressions
		// We test whether checker handles it gracefully
		identifier := Identifier{Type: Header, Value: "[invalid"}
		result := checker.Check(infos, identifier, "value")

		// Result depends on how util.RegexMatch handles invalid regex
		t.Logf("Result with invalid regex: %v", result)
	})
}

func TestIdentifierChecker_Performance(t *testing.T) {
	// Performance testing
	headers := make(map[string]string)
	for i := 0; i < 100; i++ {
		headers[fmt.Sprintf("header-%d", i)] = fmt.Sprintf("value-%d", i)
	}

	infos := &RequestInfos{Headers: headers}
	checker := &HeaderChecker{}
	identifier := Identifier{Type: Header, Value: "header-.*"}
	pattern := "value-.*"

	t.Run("performance test", func(t *testing.T) {
		start := time.Now()
		for i := 0; i < 1000; i++ {
			checker.Check(infos, identifier, pattern)
		}
		duration := time.Since(start)
		t.Logf("1000 header checks took: %v", duration)

		if duration > time.Second {
			t.Error("Header checking performance is too slow")
		}
	})
}

// Benchmark tests
func BenchmarkHeaderChecker_Check(b *testing.B) {
	headers := map[string]string{
		"Authorization": "Bearer token123",
		"Content-Type":  "application/json",
		"X-API-Key":     "api_key_456",
		"User-Agent":    "TestAgent/1.0",
		"Accept":        "application/json",
	}

	infos := &RequestInfos{Headers: headers}
	checker := &HeaderChecker{}
	identifier := Identifier{Type: Header, Value: "Authorization"}
	pattern := "Bearer.*"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			checker.Check(infos, identifier, pattern)
		}
	})
}

func BenchmarkAllIdentifierChecker_Check(b *testing.B) {
	defer saveAndRestoreRuleMatcherForBenchmark(b)()

	globalRuleMatcher = NewDefaultRuleMatcher()

	headers := map[string]string{
		"Authorization": "Bearer token123",
		"Content-Type":  "application/json",
	}

	infos := &RequestInfos{Headers: headers}
	checker := &AllIdentifierChecker{}
	identifier := Identifier{Type: AllIdentifier, Value: "Authorization"}
	pattern := "Bearer.*"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			checker.Check(infos, identifier, pattern)
		}
	})
}

func TestIdentifierChecker_ConcurrentAccess(t *testing.T) {
	defer saveAndRestoreRuleMatcher(t)()

	globalRuleMatcher = NewDefaultRuleMatcher()

	const numGoroutines = 100
	const numOperations = 1000

	headers := map[string]string{
		"Authorization": "Bearer token123",
		"X-API-Key":     "api_key_456",
	}

	infos := &RequestInfos{Headers: headers}

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	// Concurrent testing for HeaderChecker
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			checker := &HeaderChecker{}
			identifier := Identifier{Type: Header, Value: "Authorization"}

			for j := 0; j < numOperations; j++ {
				result := checker.Check(infos, identifier, "Bearer.*")
				if !result {
					errors <- fmt.Errorf("HeaderChecker should return true")
					return
				}
			}
		}()
	}

	// Concurrent testing for AllIdentifierChecker
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			checker := &AllIdentifierChecker{}
			identifier := Identifier{Type: AllIdentifier, Value: "Authorization"}

			for j := 0; j < numOperations; j++ {
				result := checker.Check(infos, identifier, "Bearer.*")
				if !result {
					errors <- fmt.Errorf("AllIdentifierChecker should return true")
					return
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
}

// Edge case testing
func TestIdentifierChecker_EdgeCases(t *testing.T) {
	t.Run("special characters in headers", func(t *testing.T) {
		headers := map[string]string{
			"X-Special-!@#": "value-$%^",
			"Content-Type":  "application/json; charset=utf-8",
		}

		infos := &RequestInfos{Headers: headers}
		checker := &HeaderChecker{}

		// Test handling of special characters
		result := checker.Check(infos,
			Identifier{Type: Header, Value: "X-Special-.*"},
			"value-.*")
		if !result {
			t.Error("Should match headers with special characters")
		}
	})

	t.Run("unicode characters", func(t *testing.T) {
		headers := map[string]string{
			"X-中文-Header": "值-测试",
			"X-Emoji-🚀":   "value-🎉",
		}

		infos := &RequestInfos{Headers: headers}
		checker := &HeaderChecker{}

		result := checker.Check(infos,
			Identifier{Type: Header, Value: "X-中文-.*"},
			"值-.*")
		if !result {
			t.Error("Should match headers with unicode characters")
		}
	})
}
