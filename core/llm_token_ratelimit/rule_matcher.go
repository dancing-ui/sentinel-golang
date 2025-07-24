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

	"github.com/alibaba/sentinel-golang/logging"
)

type MatchedRule struct {
	Strategy      Strategy
	LimitKey      string
	TimeWindow    int64
	TokenSize     int64
	CountStrategy CountStrategy
}

type MatchedRuleCollector interface {
	Collect(ctx *Context, rule *Rule) []*MatchedRule
}

type StrategyChecker interface {
	Check(ctx *Context, rules []*MatchedRule) bool
}

type IdentifierChecker interface {
	Check(infos *RequestInfos, identifier Identifier, pattern string) bool
}

type TokenUpdater interface {
	Update(ctx *Context, rule *MatchedRule)
}

var ruleMatcher = NewDefaultRuleMatcher()

type RuleMatcher struct {
	MatchedRuleCollectors map[Strategy]MatchedRuleCollector
	StrategyCheckers      map[Strategy]StrategyChecker
	IdentifierCheckers    map[IdentifierType]IdentifierChecker
	TokenUpdaters         map[Strategy]TokenUpdater
}

func NewDefaultRuleMatcher() *RuleMatcher {
	return &RuleMatcher{
		MatchedRuleCollectors: map[Strategy]MatchedRuleCollector{
			FixedWindow: &FixedWindowCollector{},
		},
		StrategyCheckers: map[Strategy]StrategyChecker{
			FixedWindow: &FixedWindowChecker{},
		},
		TokenUpdaters: map[Strategy]TokenUpdater{
			FixedWindow: &FixedWindowUpdater{},
		},
		IdentifierCheckers: map[IdentifierType]IdentifierChecker{
			AllIdentifier: &AllIdentifierChecker{},
			Header:        &HeaderChecker{},
		},
	}
}

func (m *RuleMatcher) getMatchedRuleCollector(strategy Strategy) MatchedRuleCollector {
	collector, exists := m.MatchedRuleCollectors[strategy]
	if !exists {
		return nil
	}
	return collector
}

func (m *RuleMatcher) getStrategyChecker(strategy Strategy) StrategyChecker {
	checker, exists := m.StrategyCheckers[strategy]
	if !exists {
		return nil
	}
	return checker
}

func (m *RuleMatcher) getTokenUpdater(strategy Strategy) TokenUpdater {
	updater, exists := m.TokenUpdaters[strategy]
	if !exists {
		return nil
	}
	return updater
}

func (m *RuleMatcher) getIdentifierChecker(identifier IdentifierType) IdentifierChecker {
	checker, exists := m.IdentifierCheckers[identifier]
	if !exists {
		return nil
	}
	return checker
}

func (m *RuleMatcher) checkPass(ctx *Context, rule *Rule) bool {
	collector := m.getMatchedRuleCollector(rule.Strategy)
	if collector == nil {
		logging.Error(errors.New("unknown strategy"), "unknown strategy in llm_token_ratelimit.checkPass() when get collector", "strategy", rule.Strategy.String())
		return true
	}

	rules := collector.Collect(ctx, rule)
	if len(rules) == 0 {
		return true
	}

	checker := m.getStrategyChecker(rule.Strategy)
	if checker == nil {
		logging.Error(errors.New("unknown strategy"), "unknown strategy in llm_token_ratelimit.checkPass() when get checker", "strategy", rule.Strategy.String())
		return true
	}

	if passed := checker.Check(ctx, rules); !passed {
		return false
	}

	m.cacheMatchedRules(ctx, rules)
	return true
}

func (m *RuleMatcher) cacheMatchedRules(ctx *Context, newRules []*MatchedRule) {
	if len(newRules) == 0 {
		return
	}

	existingValue := ctx.GetContext(KeyMatchedRules)
	if existingValue == nil {
		ctx.SetContext(KeyMatchedRules, newRules)
	} else {
		existingRules, ok := existingValue.([]*MatchedRule)
		if !ok {
			ctx.SetContext(KeyMatchedRules, newRules)
		} else {
			allRules := make([]*MatchedRule, 0, len(existingRules)+len(newRules))
			allRules = append(allRules, existingRules...)
			allRules = append(allRules, newRules...)
			ctx.SetContext(KeyMatchedRules, allRules)
		}
	}
}

func (m *RuleMatcher) update(ctx *Context, rule *MatchedRule) {
	if rule == nil {
		return
	}
	updater := m.getTokenUpdater(rule.Strategy)
	if updater == nil {
		logging.Error(errors.New("unknown strategy"), "unknown strategy in llm_token_ratelimit.update() when get updater", "strategy", rule.Strategy.String())
		return
	}
	updater.Update(ctx, rule)
}
