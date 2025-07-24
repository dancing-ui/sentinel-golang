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
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestRule_ResourceName(t *testing.T) {
	tests := []struct {
		name     string
		rule     *Rule
		expected string
	}{
		{
			"normal resource name",
			&Rule{Resource: "/api/chat"},
			"/api/chat",
		},
		{
			"empty resource name",
			&Rule{Resource: ""},
			"",
		},
		{
			"complex resource name",
			&Rule{Resource: "/api/v1/llm/chat/{model}"},
			"/api/v1/llm/chat/{model}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.rule.ResourceName()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestRule_String(t *testing.T) {
	tests := []struct {
		name     string
		rule     *Rule
		contains []string // Strings that should be contained in output
	}{
		{
			"nil rule",
			nil,
			[]string{"Rule{nil}"},
		},
		{
			"empty rule",
			&Rule{},
			[]string{
				"Rule{",
				"Resource:",
				"Strategy:fixed-window", // Default strategy
				"RuleName:",
				"RuleItems:[]",
				"}",
			},
		},
		{
			"rule with ID",
			&Rule{
				ID:       "test-rule-id",
				Resource: "/api/test",
				Strategy: FixedWindow,
				RuleName: "test-rule",
			},
			[]string{
				"ID:test-rule-id",
				"Resource:/api/test",
				"Strategy:fixed-window",
				"RuleName:test-rule",
				"RuleItems:[]",
			},
		},
		{
			"rule without ID",
			&Rule{
				Resource: "/api/test",
				Strategy: FixedWindow,
				RuleName: "test-rule",
			},
			[]string{
				"Resource:/api/test",
				"Strategy:fixed-window",
				"RuleName:test-rule",
				"RuleItems:[]",
			},
		},
		{
			"rule with rule items",
			&Rule{
				ID:       "complex-rule",
				Resource: "/api/llm",
				Strategy: FixedWindow,
				RuleName: "llm-limit",
				RuleItems: []*RuleItem{
					{
						Identifier: Identifier{Type: Header, Value: "user-id"},
						KeyItems: []*KeyItem{
							{
								Key:   "hourly",
								Token: Token{Number: 1000, CountStrategy: TotalTokens},
								Time:  Time{Value: 1, Unit: Hour},
							},
						},
					},
				},
			},
			[]string{
				"ID:complex-rule",
				"Resource:/api/llm",
				"Strategy:fixed-window",
				"RuleName:llm-limit",
				"RuleItems:[",
				"Identifier{Type:header, Value:user-id}",
				"KeyItem{Key:hourly",
				"Token{Number:1000, CountStrategy:total-tokens}",
				"Time{Value:3600 second}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.rule.String()

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected result to contain %q, got %q", expected, result)
				}
			}
		})
	}
}

func TestRule_String_ConcurrentAccess(t *testing.T) {
	rule := &Rule{
		ID:       "concurrent-test",
		Resource: "/api/concurrent",
		Strategy: FixedWindow,
		RuleName: "concurrent-rule",
		RuleItems: []*RuleItem{
			{
				Identifier: Identifier{Type: AllIdentifier, Value: ".*"},
				KeyItems: []*KeyItem{
					{
						Key:   "test",
						Token: Token{Number: 100, CountStrategy: TotalTokens},
						Time:  Time{Value: 60, Unit: Second},
					},
				},
			},
		},
	}

	const numGoroutines = 50
	const numOperations = 100

	var wg sync.WaitGroup
	results := make(chan string, numGoroutines*numOperations)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				result := rule.String()
				results <- result
			}
		}(i)
	}

	wg.Wait()
	close(results)

	// Verify all results are consistent
	var firstResult string
	count := 0
	for result := range results {
		if count == 0 {
			firstResult = result
		} else if result != firstResult {
			t.Errorf("Inconsistent result: expected %q, got %q", firstResult, result)
		}
		count++
	}

	if count != numGoroutines*numOperations {
		t.Errorf("Expected %d results, got %d", numGoroutines*numOperations, count)
	}
}

func TestRule_setDefaultRuleOption(t *testing.T) {
	tests := []struct {
		name     string
		rule     *Rule
		expected *Rule
	}{
		{
			"empty rule gets all defaults",
			&Rule{
				RuleItems: []*RuleItem{
					{
						Identifier: Identifier{Type: AllIdentifier},
						KeyItems: []*KeyItem{
							{
								Token: Token{Number: 1000, CountStrategy: TotalTokens},
								Time:  Time{Value: 1, Unit: Hour},
							},
						},
					},
				},
			},
			&Rule{
				Resource: DefaultResourcePattern,
				RuleName: DefaultRuleName,
				RuleItems: []*RuleItem{
					{
						Identifier: Identifier{Type: AllIdentifier, Value: DefaultIdentifierValuePattern},
						KeyItems: []*KeyItem{
							{
								Key:   DefaultKeyPattern,
								Token: Token{Number: 1000, CountStrategy: TotalTokens},
								Time:  Time{Value: 1, Unit: Hour},
							},
						},
					},
				},
			},
		},
		{
			"partial defaults applied",
			&Rule{
				Resource: "/custom/api",
				RuleItems: []*RuleItem{
					{
						Identifier: Identifier{Type: Header, Value: "custom-header"},
						KeyItems: []*KeyItem{
							{
								Token: Token{Number: 500, CountStrategy: InputTokens},
								Time:  Time{Value: 30, Unit: Minute},
							},
						},
					},
				},
			},
			&Rule{
				Resource: "/custom/api",
				RuleName: "", // Not set because not all defaults
				RuleItems: []*RuleItem{
					{
						Identifier: Identifier{Type: Header, Value: "custom-header"},
						KeyItems: []*KeyItem{
							{
								Key:   DefaultKeyPattern,
								Token: Token{Number: 500, CountStrategy: InputTokens},
								Time:  Time{Value: 30, Unit: Minute},
							},
						},
					},
				},
			},
		},
		{
			"no defaults needed",
			&Rule{
				Resource: "/api/custom",
				RuleName: "custom-rule",
				RuleItems: []*RuleItem{
					{
						Identifier: Identifier{Type: Header, Value: "api-key"},
						KeyItems: []*KeyItem{
							{
								Key:   "rate-limit",
								Token: Token{Number: 2000, CountStrategy: OutputTokens},
								Time:  Time{Value: 2, Unit: Hour},
							},
						},
					},
				},
			},
			&Rule{
				Resource: "/api/custom",
				RuleName: "custom-rule",
				RuleItems: []*RuleItem{
					{
						Identifier: Identifier{Type: Header, Value: "api-key"},
						KeyItems: []*KeyItem{
							{
								Key:   "rate-limit",
								Token: Token{Number: 2000, CountStrategy: OutputTokens},
								Time:  Time{Value: 2, Unit: Hour},
							},
						},
					},
				},
			},
		},
		{
			"multiple rule items",
			&Rule{
				RuleItems: []*RuleItem{
					{
						Identifier: Identifier{Type: AllIdentifier},
						KeyItems: []*KeyItem{
							{
								Token: Token{Number: 1000, CountStrategy: TotalTokens},
								Time:  Time{Value: 1, Unit: Hour},
							},
						},
					},
					{
						Identifier: Identifier{Type: Header, Value: "user-type"},
						KeyItems: []*KeyItem{
							{
								Key:   "premium",
								Token: Token{Number: 5000, CountStrategy: TotalTokens},
								Time:  Time{Value: 1, Unit: Hour},
							},
						},
					},
				},
			},
			&Rule{
				Resource: DefaultResourcePattern,
				RuleName: "overall-rule",
				RuleItems: []*RuleItem{
					{
						Identifier: Identifier{Type: AllIdentifier, Value: DefaultIdentifierValuePattern},
						KeyItems: []*KeyItem{
							{
								Key:   DefaultKeyPattern,
								Token: Token{Number: 1000, CountStrategy: TotalTokens},
								Time:  Time{Value: 1, Unit: Hour},
							},
						},
					},
					{
						Identifier: Identifier{Type: Header, Value: "user-type"},
						KeyItems: []*KeyItem{
							{
								Key:   "premium",
								Token: Token{Number: 5000, CountStrategy: TotalTokens},
								Time:  Time{Value: 1, Unit: Hour},
							},
						},
					},
				},
			},
		},
		{
			"multiple key items",
			&Rule{
				RuleItems: []*RuleItem{
					{
						Identifier: Identifier{Type: AllIdentifier},
						KeyItems: []*KeyItem{
							{
								Token: Token{Number: 1000, CountStrategy: TotalTokens},
								Time:  Time{Value: 1, Unit: Hour},
							},
							{
								Key:   "daily",
								Token: Token{Number: 10000, CountStrategy: TotalTokens},
								Time:  Time{Value: 1, Unit: Day},
							},
						},
					},
				},
			},
			&Rule{
				Resource: DefaultResourcePattern,
				RuleName: "overall-rule",
				RuleItems: []*RuleItem{
					{
						Identifier: Identifier{Type: AllIdentifier, Value: DefaultIdentifierValuePattern},
						KeyItems: []*KeyItem{
							{
								Key:   DefaultKeyPattern,
								Token: Token{Number: 1000, CountStrategy: TotalTokens},
								Time:  Time{Value: 1, Unit: Hour},
							},
							{
								Key:   "daily",
								Token: Token{Number: 10000, CountStrategy: TotalTokens},
								Time:  Time{Value: 1, Unit: Day},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a deep copy to avoid modifying the original
			rule := deepCopyRule(tt.rule)
			rule.setDefaultRuleOption()

			if !reflect.DeepEqual(rule, tt.expected) {
				t.Errorf("Expected %+v, got %+v", tt.expected, rule)
			}
		})
	}
}

func TestRule_setDefaultRuleOption_ConcurrentAccess(t *testing.T) {
	baseRule := &Rule{
		RuleItems: []*RuleItem{
			{
				Identifier: Identifier{Type: AllIdentifier},
				KeyItems: []*KeyItem{
					{
						Token: Token{Number: 1000, CountStrategy: TotalTokens},
						Time:  Time{Value: 1, Unit: Hour},
					},
				},
			},
		},
	}

	const numGoroutines = 20
	var wg sync.WaitGroup
	results := make(chan *Rule, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Each goroutine works on its own copy
			rule := deepCopyRule(baseRule)
			rule.setDefaultRuleOption()
			results <- rule
		}(i)
	}

	wg.Wait()
	close(results)

	// Verify all results are consistent
	var firstResult *Rule
	count := 0
	for result := range results {
		if count == 0 {
			firstResult = result
		} else {
			if !reflect.DeepEqual(result, firstResult) {
				t.Errorf("Inconsistent result at iteration %d", count)
			}
		}
		count++
	}

	if count != numGoroutines {
		t.Errorf("Expected %d results, got %d", numGoroutines, count)
	}

	// Verify the result has expected defaults
	if firstResult.Resource != DefaultResourcePattern {
		t.Errorf("Expected Resource to be %q, got %q", DefaultResourcePattern, firstResult.Resource)
	}
	if firstResult.RuleName != DefaultRuleName {
		t.Errorf("Expected RuleName to be %q, got %q", DefaultRuleName, firstResult.RuleName)
	}
}

func TestRule_setDefaultRuleOption_EdgeCases(t *testing.T) {
	t.Run("nil rule items", func(t *testing.T) {
		rule := &Rule{}

		// Should not panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("setDefaultRuleOption panicked with nil rule items: %v", r)
			}
		}()

		rule.setDefaultRuleOption()

		if rule.Resource != DefaultResourcePattern {
			t.Errorf("Expected default resource, got %q", rule.Resource)
		}
	})

	t.Run("rule item with nil key items", func(t *testing.T) {
		rule := &Rule{
			RuleItems: []*RuleItem{
				{
					Identifier: Identifier{Type: AllIdentifier},
					KeyItems:   nil,
				},
			},
		}

		// Should not panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("setDefaultRuleOption panicked with nil key items: %v", r)
			}
		}()

		rule.setDefaultRuleOption()
	})

	t.Run("empty key items slice", func(t *testing.T) {
		rule := &Rule{
			RuleItems: []*RuleItem{
				{
					Identifier: Identifier{Type: AllIdentifier},
					KeyItems:   []*KeyItem{},
				},
			},
		}

		rule.setDefaultRuleOption()

		if rule.Resource != DefaultResourcePattern {
			t.Errorf("Expected default resource, got %q", rule.Resource)
		}
		if rule.RuleItems[0].Identifier.Value != DefaultIdentifierValuePattern {
			t.Errorf("Expected default identifier value, got %q", rule.RuleItems[0].Identifier.Value)
		}
	})
}

// Helper function to deep copy a rule
func deepCopyRule(original *Rule) *Rule {
	if original == nil {
		return nil
	}

	rule := &Rule{
		ID:       original.ID,
		Resource: original.Resource,
		Strategy: original.Strategy,
		RuleName: original.RuleName,
	}

	if original.RuleItems != nil {
		rule.RuleItems = make([]*RuleItem, len(original.RuleItems))
		for i, item := range original.RuleItems {
			if item != nil {
				rule.RuleItems[i] = &RuleItem{
					Identifier: Identifier{
						Type:  item.Identifier.Type,
						Value: item.Identifier.Value,
					},
				}

				if item.KeyItems != nil {
					rule.RuleItems[i].KeyItems = make([]*KeyItem, len(item.KeyItems))
					for j, keyItem := range item.KeyItems {
						if keyItem != nil {
							rule.RuleItems[i].KeyItems[j] = &KeyItem{
								Key: keyItem.Key,
								Token: Token{
									Number:        keyItem.Token.Number,
									CountStrategy: keyItem.Token.CountStrategy,
								},
								Time: Time{
									Value: keyItem.Time.Value,
									Unit:  keyItem.Time.Unit,
								},
							}
						}
					}
				}
			}
		}
	}

	return rule
}

// Benchmark tests
func BenchmarkRule_String(b *testing.B) {
	rule := &Rule{
		ID:       "benchmark-rule",
		Resource: "/api/benchmark",
		Strategy: FixedWindow,
		RuleName: "benchmark",
		RuleItems: []*RuleItem{
			{
				Identifier: Identifier{Type: Header, Value: "api-key"},
				KeyItems: []*KeyItem{
					{
						Key:   "rate-limit",
						Token: Token{Number: 1000, CountStrategy: TotalTokens},
						Time:  Time{Value: 1, Unit: Hour},
					},
				},
			},
		},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = rule.String()
		}
	})
}

func BenchmarkRule_ResourceName(b *testing.B) {
	rule := &Rule{
		Resource: "/api/benchmark/very/long/resource/name/with/many/segments",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = rule.ResourceName()
		}
	})
}

func BenchmarkRule_setDefaultRuleOption(b *testing.B) {
	baseRule := &Rule{
		RuleItems: []*RuleItem{
			{
				Identifier: Identifier{Type: AllIdentifier},
				KeyItems: []*KeyItem{
					{
						Token: Token{Number: 1000, CountStrategy: TotalTokens},
						Time:  Time{Value: 1, Unit: Hour},
					},
				},
			},
		},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rule := deepCopyRule(baseRule)
			rule.setDefaultRuleOption()
		}
	})
}

// Test for memory allocation
func TestRule_String_MemoryAllocation(t *testing.T) {
	rule := &Rule{
		ID:       "memory-test",
		Resource: "/api/memory",
		Strategy: FixedWindow,
		RuleName: "memory-rule",
		RuleItems: []*RuleItem{
			{
				Identifier: Identifier{Type: Header, Value: "test"},
				KeyItems: []*KeyItem{
					{
						Key:   "test",
						Token: Token{Number: 100, CountStrategy: TotalTokens},
						Time:  Time{Value: 60, Unit: Second},
					},
				},
			},
		},
	}

	// Warm up
	for i := 0; i < 1000; i++ {
		_ = rule.String()
	}

	// This test primarily serves to document expected behavior
	// and can be used with -benchmem to monitor allocations
	result := rule.String()
	if len(result) == 0 {
		t.Error("String() should return non-empty result")
	}
}

func TestRule_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	rule := &Rule{
		ID:       "stress-test",
		Resource: "/api/stress",
		Strategy: FixedWindow,
		RuleName: "stress-rule",
		RuleItems: []*RuleItem{
			{
				Identifier: Identifier{Type: Header, Value: "stress"},
				KeyItems: []*KeyItem{
					{
						Key:   "stress",
						Token: Token{Number: 1000, CountStrategy: TotalTokens},
						Time:  Time{Value: 1, Unit: Hour},
					},
				},
			},
		},
	}

	const numGoroutines = 100
	const numOperations = 10000

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				// Mix of operations
				switch j % 3 {
				case 0:
					_ = rule.String()
				case 1:
					_ = rule.ResourceName()
				case 2:
					// Create a copy and set defaults
					copyRule := deepCopyRule(rule)
					copyRule.setDefaultRuleOption()
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Error(err)
	}
}
