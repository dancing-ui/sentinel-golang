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

	"github.com/alibaba/sentinel-golang/logging"
)

// TODO: 删除TokenSize
const (
	FixedWindowKeyFormat string = "sentinel-go:llm-token-ratelimit:%s:%s:%s:%d:%d:%s" // ruleName, strategy, identifierType, timeWindow, tokenSize, tokenCountStrategy
)

type FixedWindowLimitKeyParams struct {
	Strategy       Strategy
	IdentifierType IdentifierType
	RuleName       string
	TimeWindow     int64
	TokenSize      int64
	CountStrategy  CountStrategy
}
type FixedWindowCollector struct{}

func (c *FixedWindowCollector) Collect(ctx *Context, rule *Rule) []*MatchedRule {
	if rule == nil || rule.RuleItems == nil {
		return nil
	}

	reqInfos := extractRequestInfos(ctx)
	if reqInfos == nil {
		return nil
	}

	ruleMap := make(map[string]*MatchedRule)

	for _, ruleItem := range rule.RuleItems {
		identifierChecker := ruleMatcher.getIdentifierChecker(ruleItem.Identifier.Type)
		if identifierChecker == nil {
			logging.Error(errors.New("unknown identifier.type"), "unknown identifier.type in llm_token_ratelimit.FixedWindowChecker.Check()", "identifier.type", ruleItem.Identifier.Type.String())
			continue
		}
		if ruleItem.KeyItems == nil {
			continue
		}
		for _, keyItem := range ruleItem.KeyItems {
			if !identifierChecker.Check(reqInfos, ruleItem.Identifier, keyItem.Key) {
				continue
			}

			timeWindow := c.calculateTimeWindow(keyItem.Time)
			if timeWindow == ErrorTimeWindow {
				logging.Error(errors.New("error time window"), "error time window in llm_token_ratelimit.FixedWindowCollector.Collect()")
				continue
			}

			params := &FixedWindowLimitKeyParams{
				RuleName:       rule.RuleName,
				Strategy:       rule.Strategy,
				IdentifierType: ruleItem.Identifier.Type,
				TimeWindow:     timeWindow,
				TokenSize:      keyItem.Token.Number,
				CountStrategy:  keyItem.Token.CountStrategy,
			}
			c.addMatchedRule(params, ruleMap)
		}
	}

	rules := make([]*MatchedRule, 0, len(ruleMap))
	for _, rule := range ruleMap {
		rules = append(rules, rule)
	}
	return rules
}

func (c *FixedWindowCollector) calculateTimeWindow(tm Time) int64 {
	timeWindow := ErrorTimeWindow
	switch tm.Unit {
	case Second:
		timeWindow = tm.Value
	case Minute:
		timeWindow = tm.Value * 60
	case Hour:
		timeWindow = tm.Value * 60 * 60
	case Day:
		timeWindow = tm.Value * 60 * 60 * 24
	}
	return timeWindow
}

func (c *FixedWindowCollector) addMatchedRule(params *FixedWindowLimitKeyParams, ruleMap map[string]*MatchedRule) {
	if params.CountStrategy != TotalTokens {
		limitKey := c.generateLimitKey(params)
		ruleMap[limitKey] = &MatchedRule{
			Strategy:      params.Strategy,
			LimitKey:      limitKey,
			TimeWindow:    params.TimeWindow,
			TokenSize:     params.TokenSize,
			CountStrategy: params.CountStrategy,
		}
	}
	params.CountStrategy = TotalTokens
	limitKey := c.generateLimitKey(params)
	ruleMap[limitKey] = &MatchedRule{
		Strategy:      params.Strategy,
		LimitKey:      limitKey,
		TimeWindow:    params.TimeWindow,
		TokenSize:     params.TokenSize,
		CountStrategy: params.CountStrategy,
	}
}

func (c *FixedWindowCollector) generateLimitKey(params *FixedWindowLimitKeyParams) string {
	return fmt.Sprintf(FixedWindowKeyFormat,
		params.RuleName,
		params.Strategy.String(),
		params.IdentifierType.String(),
		params.TimeWindow,
		params.TokenSize,
		params.CountStrategy.String(),
	)
}
