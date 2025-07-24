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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterRules_EmptyInput(t *testing.T) {
	// Test empty input scenarios
	result := FilterRules(nil)
	assert.Empty(t, result)

	result = FilterRules([]*Rule{})
	assert.Empty(t, result)
}

func TestFilterRules_NilRule(t *testing.T) {
	// Test handling of nil rules - they should be skipped
	rules := []*Rule{
		nil, // nil rule should be skipped
		createValidRuleForRuleFilter("valid-rule", "/api/.*"),
	}

	result := FilterRules(rules)
	assert.Len(t, result, 1)
	assert.Equal(t, "valid-rule", result[0].RuleName)
}

func TestFilterRules_InvalidRules(t *testing.T) {
	// Test that invalid rules are filtered out by IsValidRule
	rules := []*Rule{
		{
			RuleName: "potentially-invalid-rule",
			Resource: "", // Will be set to default by setDefaultRuleOption
			Strategy: FixedWindow,
			RuleItems: []*RuleItem{
				{
					Identifier: Identifier{Type: Header, Value: ".*"},
					KeyItems: []*KeyItem{
						{
							Key:   ".*",
							Token: Token{Number: -100, CountStrategy: TotalTokens}, // Potentially invalid negative token
							Time:  Time{Unit: Second, Value: 60},
						},
					},
				},
			},
		},
		createValidRuleForRuleFilter("valid-rule", "/api/.*"),
	}

	result := FilterRules(rules)
	// The exact length depends on IsValidRule implementation
	// Here we just verify that valid rule exists in result
	foundValid := false
	for _, rule := range result {
		if rule.RuleName == "valid-rule" {
			foundValid = true
			break
		}
	}
	assert.True(t, foundValid, "Valid rule should be present in result")
}

func TestFilterRules_DuplicateRuleNames_KeepLatest(t *testing.T) {
	// Test deduplication of rules with same ruleName - should keep the last one (map overwrite behavior)
	rules := []*Rule{
		{
			RuleName: "duplicate-rule",
			Resource: "/api/v1/.*",
			Strategy: FixedWindow,
			RuleItems: []*RuleItem{
				{
					Identifier: Identifier{Type: Header, Value: "old-header"},
					KeyItems: []*KeyItem{
						{
							Key:   ".*",
							Token: Token{Number: 1000, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 60},
						},
					},
				},
			},
		},
		{
			RuleName: "unique-rule",
			Resource: "/api/v2/.*",
			Strategy: FixedWindow,
			RuleItems: []*RuleItem{
				{
					Identifier: Identifier{Type: Header, Value: ".*"},
					KeyItems: []*KeyItem{
						{
							Key:   ".*",
							Token: Token{Number: 500, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 30},
						},
					},
				},
			},
		},
		{
			RuleName: "duplicate-rule", // Duplicate ruleName - should overwrite the previous one
			Resource: "/api/v3/.*",
			Strategy: FixedWindow,
			RuleItems: []*RuleItem{
				{
					Identifier: Identifier{Type: Header, Value: "new-header"},
					KeyItems: []*KeyItem{
						{
							Key:   ".*",
							Token: Token{Number: 2000, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 60},
						},
					},
				},
			},
		},
	}

	result := FilterRules(rules)
	assert.Len(t, result, 2)

	// Verify that the last duplicate-rule is retained
	var duplicateRule *Rule
	var uniqueRule *Rule
	for _, rule := range result {
		if rule.RuleName == "duplicate-rule" {
			duplicateRule = rule
		} else if rule.RuleName == "unique-rule" {
			uniqueRule = rule
		}
	}

	require.NotNil(t, duplicateRule, "Duplicate rule should be present")
	assert.Equal(t, "/api/v3/.*", duplicateRule.Resource, "Should keep the latest rule configuration")
	assert.Equal(t, "new-header", duplicateRule.RuleItems[0].Identifier.Value, "Should keep the latest identifier value")
	assert.Equal(t, int64(2000), duplicateRule.RuleItems[0].KeyItems[0].Token.Number, "Should keep the latest token number")

	require.NotNil(t, uniqueRule, "Unique rule should be present")
	assert.Equal(t, "/api/v2/.*", uniqueRule.Resource)
}

func TestFilterRules_DuplicateKeyItems_KeepLatest(t *testing.T) {
	// Test deduplication of keyItems within the same rule (reverse iteration keeps latest)
	rules := []*Rule{
		{
			RuleName: "test-rule",
			Resource: "/api/.*",
			Strategy: FixedWindow,
			RuleItems: []*RuleItem{
				{
					Identifier: Identifier{Type: Header, Value: "user-id"},
					KeyItems: []*KeyItem{
						{
							Key:   ".*",
							Token: Token{Number: 1000, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 60},
						},
						{
							Key:   "api_key", // Different key
							Token: Token{Number: 500, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 60},
						},
						{
							Key:   ".*", // Duplicate configuration - should keep this one (latest)
							Token: Token{Number: 1500, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 60},
						},
						{
							Key:   "api_key", // Duplicate configuration - should keep this one (latest)
							Token: Token{Number: 800, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 60},
						},
					},
				},
			},
		},
	}

	result := FilterRules(rules)
	require.Len(t, result, 1)

	rule := result[0]
	require.Len(t, rule.RuleItems, 1)

	// Due to reverse iteration, should keep the last appearing configuration
	keyItems := rule.RuleItems[0].KeyItems
	assert.Len(t, keyItems, 2, "Should have only 2 unique keys after deduplication")

	// Verify that the latest configurations are retained
	keyItemMap := make(map[string]*KeyItem)
	for _, item := range keyItems {
		keyItemMap[item.Key] = item
	}

	require.Contains(t, keyItemMap, ".*", "Should contain '.*' key")
	assert.Equal(t, int64(1500), keyItemMap[".*"].Token.Number, "Should keep the latest configuration for '.*' key")

	require.Contains(t, keyItemMap, "api_key", "Should contain 'api_key' key")
	assert.Equal(t, int64(800), keyItemMap["api_key"].Token.Number, "Should keep the latest configuration for 'api_key' key")
}

func TestFilterRules_DifferentCountStrategies_NoDeduplication(t *testing.T) {
	// Test that keyItems with different CountStrategy are not considered duplicates
	rules := []*Rule{
		{
			RuleName: "test-rule",
			Resource: "/api/.*",
			Strategy: FixedWindow,
			RuleItems: []*RuleItem{
				{
					Identifier: Identifier{Type: Header, Value: "user-id"},
					KeyItems: []*KeyItem{
						{
							Key:   ".*",
							Token: Token{Number: 1000, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 60},
						},
						{
							Key:   ".*",
							Token: Token{Number: 1000, CountStrategy: InputTokens}, // Different CountStrategy
							Time:  Time{Unit: Second, Value: 60},
						},
						{
							Key:   ".*",
							Token: Token{Number: 1000, CountStrategy: OutputTokens}, // Different CountStrategy
							Time:  Time{Unit: Second, Value: 60},
						},
					},
				},
			},
		},
	}

	result := FilterRules(rules)
	require.Len(t, result, 1)

	rule := result[0]
	require.Len(t, rule.RuleItems, 1)

	// All different CountStrategy should be preserved
	keyItems := rule.RuleItems[0].KeyItems
	assert.Len(t, keyItems, 3, "All different count strategies should be preserved")

	strategies := make(map[CountStrategy]bool)
	for _, item := range keyItems {
		strategies[item.Token.CountStrategy] = true
	}
	assert.True(t, strategies[TotalTokens], "Should contain TotalTokens strategy")
	assert.True(t, strategies[InputTokens], "Should contain InputTokens strategy")
	assert.True(t, strategies[OutputTokens], "Should contain OutputTokens strategy")
}

func TestFilterRules_DifferentTimeUnits_Duplication(t *testing.T) {
	// Test that keyItems with different TimeUnit are not considered duplicates
	rules := []*Rule{
		{
			RuleName: "test-rule",
			Resource: "/api/.*",
			Strategy: FixedWindow,
			RuleItems: []*RuleItem{
				{
					Identifier: Identifier{Type: Header, Value: "user-id"},
					KeyItems: []*KeyItem{
						{
							Key:   ".*",
							Token: Token{Number: 1000, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 60},
						},
						{
							Key:   ".*",
							Token: Token{Number: 1000, CountStrategy: TotalTokens},
							Time:  Time{Unit: Minute, Value: 1}, // Different TimeUnit
						},
						{
							Key:   ".*",
							Token: Token{Number: 1000, CountStrategy: TotalTokens},
							Time:  Time{Unit: Hour, Value: 1}, // Different TimeUnit
						},
					},
				},
			},
		},
	}

	result := FilterRules(rules)
	require.Len(t, result, 1)

	rule := result[0]
	require.Len(t, rule.RuleItems, 1)

	keyItems := rule.RuleItems[0].KeyItems
	assert.Len(t, keyItems, 2, "only two different key items")

	timeUnits := make(map[TimeUnit]bool)
	for _, item := range keyItems {
		timeUnits[item.Time.Unit] = true
	}
	assert.True(t, timeUnits[Minute], "Should contain Minute time unit")
	assert.True(t, timeUnits[Hour], "Should contain Hour time unit")
}

func TestFilterRules_ComplexDuplication_KeepLatest(t *testing.T) {
	// Test complex scenario with multiple types of duplication
	rules := []*Rule{
		{
			RuleName: "complex-rule",
			Resource: "/api/.*",
			Strategy: FixedWindow,
			RuleItems: []*RuleItem{
				{
					Identifier: Identifier{Type: Header, Value: "user-id"},
					KeyItems: []*KeyItem{
						{
							Key:   ".*",
							Token: Token{Number: 1000, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 60},
						},
						{
							Key:   ".*", // Duplicate keyItem
							Token: Token{Number: 1500, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 60},
						},
					},
				},
				{
					Identifier: Identifier{Type: AllIdentifier, Value: ".*"},
					KeyItems: []*KeyItem{
						{
							Key:   ".*",
							Token: Token{Number: 800, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 30},
						},
					},
				},
			},
		},
		{
			RuleName: "complex-rule", // Duplicate ruleName - should overwrite the previous one
			Resource: "/api/v2/.*",
			Strategy: FixedWindow,
			RuleItems: []*RuleItem{
				{
					Identifier: Identifier{Type: AllIdentifier, Value: ".*"},
					KeyItems: []*KeyItem{
						{
							Key:   ".*",
							Token: Token{Number: 3000, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 30},
						},
					},
				},
			},
		},
	}

	result := FilterRules(rules)
	require.Len(t, result, 1, "Only one rule should remain after ruleName deduplication")

	rule := result[0]
	assert.Equal(t, "complex-rule", rule.RuleName)
	assert.Equal(t, "/api/v2/.*", rule.Resource, "Should keep the latest rule configuration")
	assert.Equal(t, FixedWindow, rule.Strategy)
	require.Len(t, rule.RuleItems, 1)
	assert.Equal(t, AllIdentifier, rule.RuleItems[0].Identifier.Type)
	assert.Equal(t, int64(3000), rule.RuleItems[0].KeyItems[0].Token.Number)
}

func TestFilterRules_MultipleUniqueRules(t *testing.T) {
	// Test that completely different rules are all preserved
	rules := []*Rule{
		createValidRuleForRuleFilter("rule-1", "/api/v1/.*"),
		createValidRuleForRuleFilter("rule-2", "/api/v2/.*"),
		createValidRuleForRuleFilter("rule-3", "/api/v3/.*"),
	}

	result := FilterRules(rules)
	assert.Len(t, result, 3, "All unique rules should be preserved")

	// Verify all rules are retained
	ruleNames := make(map[string]bool)
	for _, rule := range result {
		ruleNames[rule.RuleName] = true
	}
	assert.True(t, ruleNames["rule-1"], "Should contain rule-1")
	assert.True(t, ruleNames["rule-2"], "Should contain rule-2")
	assert.True(t, ruleNames["rule-3"], "Should contain rule-3")
}

func TestFilterRules_DefaultRuleGeneration(t *testing.T) {
	// Test handling of empty ruleName and default value generation
	rules := []*Rule{
		{
			RuleName: "", // Empty ruleName
			Resource: "", // Empty resource
			Strategy: FixedWindow,
			RuleItems: []*RuleItem{
				{
					Identifier: Identifier{Type: Header, Value: ""}, // Empty value
					KeyItems: []*KeyItem{
						{
							Key:   "", // Empty key
							Token: Token{Number: 1000, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 60},
						},
					},
				},
			},
		},
	}

	result := FilterRules(rules)
	require.Len(t, result, 1)

	// Verify that default values are set by setDefaultRuleOption
	rule := result[0]
	assert.Equal(t, DefaultRuleName, rule.RuleName, "Should set default rule name")
	assert.Equal(t, DefaultResourcePattern, rule.Resource, "Should set default resource pattern")
	assert.Equal(t, DefaultIdentifierValuePattern, rule.RuleItems[0].Identifier.Value, "Should set default identifier value")
	assert.Equal(t, DefaultKeyPattern, rule.RuleItems[0].KeyItems[0].Key, "Should set default key pattern")
}

func TestFilterRules_EmptyKeyItemsAfterDeduplication(t *testing.T) {
	// Test that ruleItems with no keyItems after deduplication are filtered out
	rules := []*Rule{
		{
			RuleName: "test-rule",
			Resource: "/api/.*",
			Strategy: FixedWindow,
			RuleItems: []*RuleItem{
				{
					Identifier: Identifier{Type: Header, Value: "user-id"},
					KeyItems: []*KeyItem{
						{
							Key:   ".*",
							Token: Token{Number: 1000, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 60},
						},
						{
							Key:   ".*", // Exact duplicate - will be filtered
							Token: Token{Number: 1000, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 60},
						},
					},
				},
				{
					Identifier: Identifier{Type: AllIdentifier, Value: ".*"},
					KeyItems: []*KeyItem{
						{
							Key:   "unique",
							Token: Token{Number: 500, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 30},
						},
					},
				},
			},
		},
	}

	result := FilterRules(rules)
	require.Len(t, result, 1)

	rule := result[0]
	assert.Len(t, rule.RuleItems, 2, "Both ruleItems should be preserved")

	// First ruleItem should have only one keyItem after deduplication
	var headerRuleItem *RuleItem
	for _, item := range rule.RuleItems {
		if item.Identifier.Type == Header {
			headerRuleItem = item
			break
		}
	}
	require.NotNil(t, headerRuleItem, "Header ruleItem should exist")
	assert.Len(t, headerRuleItem.KeyItems, 1, "Should have only one keyItem after deduplication")
}

func TestFilterRules_ReverseIterationPreservesLatest(t *testing.T) {
	// Test that reverse iteration logic correctly preserves the latest configuration
	rules := []*Rule{
		{
			RuleName: "test-rule",
			Resource: "/api/.*",
			Strategy: FixedWindow,
			RuleItems: []*RuleItem{
				{
					Identifier: Identifier{Type: Header, Value: "user-id"},
					KeyItems: []*KeyItem{
						{
							Key:   "key1", // First occurrence
							Token: Token{Number: 100, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 60},
						},
						{
							Key:   "key2",
							Token: Token{Number: 200, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 60},
						},
						{
							Key:   "key1", // Second occurrence - should be kept due to reverse iteration
							Token: Token{Number: 150, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 60},
						},
						{
							Key:   "key3",
							Token: Token{Number: 300, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 60},
						},
						{
							Key:   "key1", // Third occurrence - should be kept as the latest
							Token: Token{Number: 175, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 60},
						},
					},
				},
			},
		},
	}

	result := FilterRules(rules)
	require.Len(t, result, 1)

	rule := result[0]
	require.Len(t, rule.RuleItems, 1)

	keyItems := rule.RuleItems[0].KeyItems
	assert.Len(t, keyItems, 3, "Should have 3 unique keys")

	// Find key1 and verify it has the latest value (175)
	var key1Item *KeyItem
	for _, item := range keyItems {
		if item.Key == "key1" {
			key1Item = item
			break
		}
	}

	require.NotNil(t, key1Item, "key1 should be present")
	assert.Equal(t, int64(175), key1Item.Token.Number, "Should keep the latest configuration for key1")
}

func TestFilterRules_MultipleRuleItemsWithSameIdentifier(t *testing.T) {
	// Test behavior when multiple ruleItems have different identifiers
	rules := []*Rule{
		{
			RuleName: "test-rule",
			Resource: "/api/.*",
			Strategy: FixedWindow,
			RuleItems: []*RuleItem{
				{
					Identifier: Identifier{Type: Header, Value: "user-id"},
					KeyItems: []*KeyItem{
						{
							Key:   ".*",
							Token: Token{Number: 1000, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 60},
						},
					},
				},
				{
					Identifier: Identifier{Type: Header, Value: "api_key"},
					KeyItems: []*KeyItem{
						{
							Key:   ".*",
							Token: Token{Number: 500, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 30},
						},
					},
				},
				{
					Identifier: Identifier{Type: Header, Value: "session-id"}, // Different value, different identifier
					KeyItems: []*KeyItem{
						{
							Key:   ".*",
							Token: Token{Number: 800, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 45},
						},
					},
				},
			},
		},
	}

	result := FilterRules(rules)
	require.Len(t, result, 1)

	rule := result[0]
	assert.Len(t, rule.RuleItems, 3, "All ruleItems with different identifiers should be preserved")

	// Verify all different identifiers are present
	identifierMap := make(map[string]bool)
	for _, item := range rule.RuleItems {
		key := item.Identifier.Type.String() + "|" + item.Identifier.Value
		identifierMap[key] = true
	}

	assert.True(t, identifierMap["header|user-id"], "Should contain header|user-id identifier")
	assert.True(t, identifierMap["header|api_key"], "Should contain header|api_key identifier")
	assert.True(t, identifierMap["header|session-id"], "Should contain header|session-id identifier")
}

func TestFilterRules_ExactDuplicateKeyItems(t *testing.T) {
	// Test exact duplicate keyItems are properly deduplicated
	rules := []*Rule{
		{
			RuleName: "test-rule",
			Resource: "/api/.*",
			Strategy: FixedWindow,
			RuleItems: []*RuleItem{
				{
					Identifier: Identifier{Type: Header, Value: "user-id"},
					KeyItems: []*KeyItem{
						{
							Key:   ".*",
							Token: Token{Number: 1000, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 60},
						},
						{
							Key:   ".*", // Exact duplicate
							Token: Token{Number: 1000, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 60},
						},
						{
							Key:   ".*", // Another exact duplicate
							Token: Token{Number: 1000, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 60},
						},
						{
							Key:   "different-key", // Different key
							Token: Token{Number: 500, CountStrategy: TotalTokens},
							Time:  Time{Unit: Second, Value: 30},
						},
					},
				},
			},
		},
	}

	result := FilterRules(rules)
	require.Len(t, result, 1)

	rule := result[0]
	require.Len(t, rule.RuleItems, 1)

	keyItems := rule.RuleItems[0].KeyItems
	assert.Len(t, keyItems, 2, "Should have only 2 unique keyItems after deduplication")

	// Verify both unique keys are present
	keys := make(map[string]bool)
	for _, item := range keyItems {
		keys[item.Key] = true
	}
	assert.True(t, keys[".*"], "Should contain '.*' key")
	assert.True(t, keys["different-key"], "Should contain 'different-key' key")
}

// Helper function to create a valid rule for testing
func createValidRuleForRuleFilter(ruleName, resource string) *Rule {
	return &Rule{
		RuleName: ruleName,
		Resource: resource,
		Strategy: FixedWindow,
		RuleItems: []*RuleItem{
			{
				Identifier: Identifier{Type: Header, Value: ".*"},
				KeyItems: []*KeyItem{
					{
						Key:   ".*",
						Token: Token{Number: 1000, CountStrategy: TotalTokens},
						Time:  Time{Unit: Second, Value: 60},
					},
				},
			},
		},
	}
}
