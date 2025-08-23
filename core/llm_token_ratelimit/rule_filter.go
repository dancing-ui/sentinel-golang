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

import "github.com/alibaba/sentinel-golang/logging"

func FilterRules(rules []*Rule) []*Rule {
	if rules == nil {
		logging.Warn("[LLMTokenRateLimit FilterRules] No rules to filter, returning empty slice")
		return []*Rule{}
	}
	var copiedRules = make([]*Rule, len(rules))
	if err := deepCopyByJSON(rules, &copiedRules); err != nil {
		logging.Warn("[LLMTokenRateLimit FilterRules] Failed to deep copy rules, returning empty slice", "error", err.Error())
		return []*Rule{}
	}
	// 1. First, filter out invalid rules
	// 2. Retain the latest rule corresponding to each unique rule-name
	// 3. For each individual rule, retain the latest keyItems, so the ruleItem is unique
	ruleMap := make(map[string]*Rule, 16)
	for _, rule := range copiedRules {
		if rule == nil {
			continue
		}
		rule.setDefaultRuleOption()
		if err := IsValidRule(rule); err != nil {
			logging.Warn("[LLMTokenRateLimit FilterRules] Ignoring invalid llm_token_ratelimit rule", "rule", rule, "reason", err.Error())
			continue
		}
		ruleMap[rule.RuleName] = rule
	}
	resRules := make([]*Rule, 0, len(ruleMap))
	for _, rule := range ruleMap {
		rule.filterDuplicatedItem()
		resRules = append(resRules, rule)
	}
	return resRules
}
