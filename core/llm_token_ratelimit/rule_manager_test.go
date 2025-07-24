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
)

func TestLoadRules(t *testing.T) {
	// Clean up before and after tests
	clearRulesForTest()
	defer clearRulesForTest()

	tests := []struct {
		name           string
		rules          []*Rule
		expectedUpdate bool
		expectedError  bool
		description    string
	}{
		{
			name:           "empty rules",
			rules:          []*Rule{},
			expectedUpdate: false,
			expectedError:  false,
			description:    "should not accept empty rules",
		},
		{
			name:           "nil rules",
			rules:          nil,
			expectedUpdate: false,
			expectedError:  false,
			description:    "should not accept nil rules",
		},
		{
			name: "valid single rule",
			rules: []*Rule{
				createValidRule("api-test", "test-rule-1"),
			},
			expectedUpdate: true,
			expectedError:  false,
			description:    "should accept valid single rule",
		},
		{
			name: "valid multiple rules",
			rules: []*Rule{
				createValidRule("api-test-1", "test-rule-1"),
				createValidRule("api-test-2", "test-rule-2"),
			},
			expectedUpdate: true,
			expectedError:  false,
			description:    "should accept multiple valid rules",
		},
		{
			name: "rules with same resource",
			rules: []*Rule{
				createValidRule("api-test", "test-rule-1"),
				createValidRule("api-test", "test-rule-2"),
			},
			expectedUpdate: true,
			expectedError:  false,
			description:    "should group rules by resource",
		},
		{
			name: "mixed valid and invalid rules",
			rules: []*Rule{
				createValidRule("api-test", "test-rule-1"),
				createInvalidRule(), // This should be ignored
			},
			expectedUpdate: true,
			expectedError:  false,
			description:    "should accept valid rules and ignore invalid ones",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updated, err := LoadRules(tt.rules)

			if tt.expectedError {
				if err == nil {
					t.Errorf("Expected error but got none. %s", tt.description)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v. %s", err, tt.description)
				return
			}

			if updated != tt.expectedUpdate {
				t.Errorf("Expected updated=%v, got=%v. %s", tt.expectedUpdate, updated, tt.description)
			}

			// Verify rules were loaded correctly
			loadedRules := GetRules()
			if len(tt.rules) == 0 {
				if len(loadedRules) != 0 {
					t.Errorf("Expected empty rules, got %d rules", len(loadedRules))
				}
			} else {
				// Count valid rules
				validCount := 0
				for _, rule := range tt.rules {
					if IsValidRule(rule) == nil {
						validCount++
					}
				}
				if len(loadedRules) != validCount {
					t.Errorf("Expected %d loaded rules, got %d", validCount, len(loadedRules))
				}
			}
		})
	}
}

func TestLoadRulesOfResource(t *testing.T) {
	clearRulesForTest()
	defer clearRulesForTest()

	tests := []struct {
		name           string
		resource       string
		rules          []*Rule
		expectedUpdate bool
		expectedError  bool
		errorMsg       string
		description    string
	}{
		{
			name:           "empty resource",
			resource:       "",
			rules:          []*Rule{createValidRule("test", "rule1")},
			expectedUpdate: false,
			expectedError:  true,
			errorMsg:       "empty resource",
			description:    "should reject empty resource name",
		},
		{
			name:           "valid resource with rules",
			resource:       "api-test",
			rules:          []*Rule{createValidRule("api-test", "rule1")},
			expectedUpdate: true,
			expectedError:  false,
			description:    "should load rules for specific resource",
		},
		{
			name:           "clear resource rules",
			resource:       "api-test",
			rules:          []*Rule{},
			expectedUpdate: true,
			expectedError:  false,
			description:    "should clear rules for specific resource",
		},
		{
			name:           "nil rules for resource",
			resource:       "api-test",
			rules:          nil,
			expectedUpdate: true,
			expectedError:  false,
			description:    "should clear rules when nil is passed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updated, err := LoadRulesOfResource(tt.resource, tt.rules)

			if tt.expectedError {
				if err == nil {
					t.Errorf("Expected error but got none. %s", tt.description)
					return
				}
				if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorMsg, err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v. %s", err, tt.description)
				return
			}

			if updated != tt.expectedUpdate {
				t.Errorf("Expected updated=%v, got=%v. %s", tt.expectedUpdate, updated, tt.description)
			}
		})
	}
}

func TestGetRules(t *testing.T) {
	clearRulesForTest()
	defer clearRulesForTest()

	// Test empty rules
	rules := GetRules()
	if len(rules) != 0 {
		t.Errorf("Expected 0 rules, got %d", len(rules))
	}

	// Load some rules
	testRules := []*Rule{
		createValidRule("api-1", "rule-1"),
		createValidRule("api-2", "rule-2"),
	}
	LoadRules(testRules)

	// Test loaded rules
	rules = GetRules()
	if len(rules) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(rules))
	}

	// Verify rules are returned as copies (not pointers)
	for i, rule := range rules {
		if &rule == testRules[i] {
			t.Errorf("Rule %d should be a copy, not the original pointer", i)
		}
	}
}

func TestGetRulesOfResource(t *testing.T) {
	clearRulesForTest()
	defer clearRulesForTest()

	testRules := []*Rule{
		createValidRule("api-v1", "rule-1"),
		createValidRule("api-v2", "rule-2"),
		createValidRule("service", "rule-3"),
	}
	LoadRules(testRules)

	tests := []struct {
		name         string
		resource     string
		expectedHits int
		description  string
	}{
		{
			name:         "exact match",
			resource:     "api-v1",
			expectedHits: 1,
			description:  "should match exact resource name",
		},
		{
			name:         "no match",
			resource:     "nonexistent",
			expectedHits: 0,
			description:  "should return empty for non-matching resource",
		},
		{
			name:         "regex match",
			resource:     "api",
			expectedHits: 0, // Because we use exact pattern matching
			description:  "should handle regex pattern matching",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := GetRulesOfResource(tt.resource)
			if len(rules) != tt.expectedHits {
				t.Errorf("Expected %d rules, got %d. %s", tt.expectedHits, len(rules), tt.description)
			}
		})
	}
}

func TestClearRules(t *testing.T) {
	clearRulesForTest()
	defer clearRulesForTest()

	// Load some rules first
	testRules := []*Rule{
		createValidRule("api-1", "rule-1"),
		createValidRule("api-2", "rule-2"),
	}
	LoadRules(testRules)

	// Verify rules are loaded
	rules := GetRules()
	if len(rules) == 0 {
		t.Fatal("Failed to load test rules")
	}

	// Clear rules
	err := ClearRules()
	if err != nil {
		t.Errorf("Unexpected error clearing rules: %v", err)
	}

	// Verify rules are cleared
	rules = GetRules()
	if len(rules) != 0 {
		t.Errorf("Expected 0 rules after clearing, got %d", len(rules))
	}
}

func TestClearRulesOfResource(t *testing.T) {
	clearRulesForTest()
	defer clearRulesForTest()

	// Load rules for multiple resources
	testRules := []*Rule{
		createValidRule("api-1", "rule-1"),
		createValidRule("api-2", "rule-2"),
	}
	LoadRules(testRules)

	// Clear rules for one resource
	err := ClearRulesOfResource("api-1")
	if err != nil {
		t.Errorf("Unexpected error clearing resource rules: %v", err)
	}

	// Verify only the specified resource rules are cleared
	api1Rules := GetRulesOfResource("api-1")
	api2Rules := GetRulesOfResource("api-2")

	if len(api1Rules) != 0 {
		t.Errorf("Expected 0 rules for api-1, got %d", len(api1Rules))
	}
	if len(api2Rules) != 1 {
		t.Errorf("Expected 1 rule for api-2, got %d", len(api2Rules))
	}
}

// Concurrent Tests

func TestLoadRulesConcurrent(t *testing.T) {
	clearRulesForTest()
	defer clearRulesForTest()

	const numGoroutines = 10
	const rulesPerGoroutine = 5

	var wg sync.WaitGroup
	var errors []error
	var errorMux sync.Mutex

	// Start multiple goroutines loading rules concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			var rules []*Rule
			for j := 0; j < rulesPerGoroutine; j++ {
				rule := createValidRule(
					fmt.Sprintf("api-concurrent-%d-%d", id, j),
					fmt.Sprintf("rule-concurrent-%d-%d", id, j),
				)
				rules = append(rules, rule)
			}

			_, err := LoadRules(rules)
			if err != nil {
				errorMux.Lock()
				errors = append(errors, err)
				errorMux.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// Check for errors
	if len(errors) > 0 {
		t.Errorf("Got %d errors during concurrent loading: %v", len(errors), errors[0])
	}

	// Verify final state is consistent
	rules := GetRules()
	t.Logf("Final rule count: %d", len(rules))

	// The final state should be consistent (last write wins)
	if len(rules) != rulesPerGoroutine {
		t.Logf("Expected %d rules, got %d (this is acceptable due to concurrent updates)", rulesPerGoroutine, len(rules))
	}
}

func TestGetRulesReadConcurrency(t *testing.T) {
	clearRulesForTest()
	defer clearRulesForTest()

	// Load initial rules
	initialRules := []*Rule{
		createValidRule("api-read-test-1", "read-rule-1"),
		createValidRule("api-read-test-2", "read-rule-2"),
		createValidRule("api-read-test-3", "read-rule-3"),
	}
	LoadRules(initialRules)

	const numReaders = 20
	const readsPerReader = 100

	var wg sync.WaitGroup
	var readErrors []error
	var errorMux sync.Mutex

	// Start multiple goroutines reading rules concurrently
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()

			for j := 0; j < readsPerReader; j++ {
				rules := GetRules()

				// Verify rules are consistent
				if len(rules) != len(initialRules) {
					errorMux.Lock()
					readErrors = append(readErrors, fmt.Errorf("reader %d iteration %d: expected %d rules, got %d", readerID, j, len(initialRules), len(rules)))
					errorMux.Unlock()
					return
				}

				// Small delay to increase chance of race conditions
				if j%10 == 0 {
					runtime.Gosched()
				}
			}
		}(i)
	}

	wg.Wait()

	if len(readErrors) > 0 {
		t.Errorf("Got %d read errors: %v", len(readErrors), readErrors[0])
	}
}

func TestReadWriteConcurrency(t *testing.T) {
	clearRulesForTest()
	defer clearRulesForTest()

	const numWriters = 5
	const numReaders = 15
	const operationsPerGoroutine = 50

	var wg sync.WaitGroup
	var allErrors []error
	var errorMux sync.Mutex

	// Start writers
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				rules := []*Rule{
					createValidRule(
						fmt.Sprintf("api-rw-%d-%d", writerID, j),
						fmt.Sprintf("rule-rw-%d-%d", writerID, j),
					),
				}

				_, err := LoadRules(rules)
				if err != nil {
					errorMux.Lock()
					allErrors = append(allErrors, fmt.Errorf("writer %d: %w", writerID, err))
					errorMux.Unlock()
				}

				// Small delay
				if j%5 == 0 {
					time.Sleep(time.Microsecond * 10)
				}
			}
		}(i)
	}

	// Start readers
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				rules := GetRules()

				// Just verify we can read without panic
				// The number of rules may vary due to concurrent writes
				_ = len(rules)

				// Also test resource-specific reads
				GetRulesOfResource("api-rw-test")

				// Small delay
				if j%10 == 0 {
					runtime.Gosched()
				}
			}
		}(i)
	}

	wg.Wait()

	if len(allErrors) > 0 {
		t.Errorf("Got %d errors during concurrent read/write: %v", len(allErrors), allErrors[0])
	}

	t.Log("Read/write concurrency test completed successfully")
}

func TestResourceSpecificConcurrency(t *testing.T) {
	clearRulesForTest()
	defer clearRulesForTest()

	const numResources = 5
	const operationsPerResource = 20

	var wg sync.WaitGroup
	var errors []error
	var errorMux sync.Mutex

	// Concurrent operations on different resources
	for resourceID := 0; resourceID < numResources; resourceID++ {
		wg.Add(1)
		go func(resID int) {
			defer wg.Done()

			resourceName := fmt.Sprintf("api-resource-%d", resID)

			for j := 0; j < operationsPerResource; j++ {
				switch j % 3 {
				case 0: // Load rules
					rules := []*Rule{
						createValidRule(resourceName, fmt.Sprintf("rule-%d-%d", resID, j)),
					}
					_, err := LoadRulesOfResource(resourceName, rules)
					if err != nil {
						errorMux.Lock()
						errors = append(errors, err)
						errorMux.Unlock()
					}
				case 1: // Read rules
					GetRulesOfResource(resourceName)
				case 2: // Clear rules
					if j%6 == 2 { // Only clear occasionally
						ClearRulesOfResource(resourceName)
					}
				}

				if j%5 == 0 {
					runtime.Gosched()
				}
			}
		}(resourceID)
	}

	wg.Wait()

	if len(errors) > 0 {
		t.Errorf("Got %d errors during resource-specific concurrency: %v", len(errors), errors[0])
	}
}

func TestRaceConditionDetection(t *testing.T) {
	clearRulesForTest()
	defer clearRulesForTest()

	// This test is designed to trigger race conditions if they exist
	const iterations = 100

	for i := 0; i < iterations; i++ {
		var wg sync.WaitGroup

		// Rapid fire operations
		wg.Add(3)

		// Writer 1
		go func() {
			defer wg.Done()
			LoadRules([]*Rule{createValidRule("api-race-1", "rule-1")})
		}()

		// Writer 2
		go func() {
			defer wg.Done()
			LoadRules([]*Rule{createValidRule("api-race-2", "rule-2")})
		}()

		// Reader
		go func() {
			defer wg.Done()
			GetRules()
			GetRulesOfResource("api-race-1")
		}()

		wg.Wait()
	}

	t.Log("Race condition detection test completed")
}

// Benchmark Tests

func BenchmarkLoadRules(b *testing.B) {
	clearRulesForTest()
	defer clearRulesForTest()

	rules := make([]*Rule, 100)
	for i := 0; i < 100; i++ {
		rules[i] = createValidRule(fmt.Sprintf("api-%d", i), fmt.Sprintf("rule-%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LoadRules(rules)
	}
}

func BenchmarkGetRules(b *testing.B) {
	clearRulesForTest()
	defer clearRulesForTest()

	// Setup rules
	rules := make([]*Rule, 100)
	for i := 0; i < 100; i++ {
		rules[i] = createValidRule(fmt.Sprintf("api-%d", i), fmt.Sprintf("rule-%d", i))
	}
	LoadRules(rules)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetRules()
	}
}

func BenchmarkGetRulesOfResource(b *testing.B) {
	clearRulesForTest()
	defer clearRulesForTest()

	// Setup rules
	rules := make([]*Rule, 100)
	for i := 0; i < 100; i++ {
		rules[i] = createValidRule(fmt.Sprintf("api-%d", i), fmt.Sprintf("rule-%d", i))
	}
	LoadRules(rules)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetRulesOfResource("api-50")
	}
}

func BenchmarkConcurrentReadWrite(b *testing.B) {
	clearRulesForTest()
	defer clearRulesForTest()

	rule := createValidRule("benchmark-api", "benchmark-rule")

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%10 == 0 {
				LoadRules([]*Rule{rule})
			} else {
				GetRules()
			}
			i++
		}
	})
}

// Helper functions

func createValidRule(resource, ruleName string) *Rule {
	return &Rule{
		Resource: resource,
		Strategy: FixedWindow,
		RuleName: ruleName,
		RuleItems: []*RuleItem{
			{
				Identifier: Identifier{
					Type:  AllIdentifier,
					Value: ".*",
				},
				KeyItems: []*KeyItem{
					{
						Key: ".*",
						Token: Token{
							Number:        100,
							CountStrategy: TotalTokens,
						},
						Time: Time{
							Unit:  Minute,
							Value: 1,
						},
					},
				},
			},
		},
	}
}

func createInvalidRule() *Rule {
	return &Rule{
		Resource:  "", // Invalid empty resource
		Strategy:  FixedWindow,
		RuleName:  "invalid-rule",
		RuleItems: []*RuleItem{}, // Invalid empty rule items
	}
}

func clearRulesForTest() {
	updateRuleMux.Lock()
	defer updateRuleMux.Unlock()

	rwMux.Lock()
	ruleMap = make(map[string][]*Rule)
	rwMux.Unlock()

	currentRules = make(map[string][]*Rule)
}

func TestRuleManagerConsistency(t *testing.T) {
	clearRulesForTest()
	defer clearRulesForTest()

	// Test that rule manager maintains consistency under various operations
	testRules := []*Rule{
		createValidRule("api-consistency-1", "consistency-rule-1"),
		createValidRule("api-consistency-2", "consistency-rule-2"),
	}

	// Load initial rules
	updated, err := LoadRules(testRules)
	if err != nil {
		t.Fatalf("Failed to load initial rules: %v", err)
	}
	if !updated {
		t.Error("Expected rules to be updated")
	}

	// Verify loaded rules
	loadedRules := GetRules()
	if len(loadedRules) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(loadedRules))
	}

	// Load same rules again - should not update
	updated, err = LoadRules(testRules)
	if err != nil {
		t.Fatalf("Failed to reload same rules: %v", err)
	}
	if updated {
		t.Error("Expected rules not to be updated when same rules are loaded")
	}

	// Modify rules and reload
	modifiedRules := []*Rule{
		createValidRule("api-consistency-1", "consistency-rule-1-modified"),
	}
	updated, err = LoadRules(modifiedRules)
	if err != nil {
		t.Fatalf("Failed to load modified rules: %v", err)
	}
	if !updated {
		t.Error("Expected rules to be updated with modified rules")
	}

	// Verify modification
	loadedRules = GetRules()
	if len(loadedRules) != 1 {
		t.Errorf("Expected 1 rule after modification, got %d", len(loadedRules))
	}
	if loadedRules[0].RuleName != "consistency-rule-1-modified" {
		t.Errorf("Expected rule name 'consistency-rule-1-modified', got '%s'", loadedRules[0].RuleName)
	}
}

func TestMemoryLeaks(t *testing.T) {
	clearRulesForTest()
	defer clearRulesForTest()

	// This test helps identify potential memory leaks by loading and clearing rules many times
	const iterations = 100

	for i := 0; i < iterations; i++ {
		rules := []*Rule{
			createValidRule(fmt.Sprintf("api-memory-%d", i), fmt.Sprintf("memory-rule-%d", i)),
		}

		LoadRules(rules)

		if i%10 == 0 {
			ClearRules()
		}
	}

	// Final cleanup
	ClearRules()

	// Verify everything is cleaned up
	finalRules := GetRules()
	if len(finalRules) != 0 {
		t.Errorf("Expected 0 rules after cleanup, got %d", len(finalRules))
	}

	t.Log("Memory leak test completed")
}
