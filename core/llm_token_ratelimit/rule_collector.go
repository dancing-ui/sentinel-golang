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

type BaseLimitKeyParams struct {
	Strategy       Strategy
	IdentifierType IdentifierType
	RuleName       string
	TimeWindow     int64
	TokenSize      int64
	CountStrategy  CountStrategy
	// PETA
	Encoding TiktokenEncoding
}
type BaseRuleCollector struct{}

func (c *BaseRuleCollector) Collect(ctx *Context, rule *Rule) []*MatchedRule {
	if c == nil || rule == nil || rule.RuleItems == nil {
		return nil
	}

	reqInfos := extractRequestInfos(ctx)
	if reqInfos == nil {
		return nil
	}

	ruleMap := make(map[string]*MatchedRule)

	for _, ruleItem := range rule.RuleItems {
		identifierChecker := globalRuleMatcher.getIdentifierChecker(ruleItem.Identifier.Type)
		if identifierChecker == nil {
			logging.Error(errors.New("unknown identifier.type"), "unknown identifier.type in llm_token_ratelimit.BaseRuleCollector.Collect()", "identifier.type", ruleItem.Identifier.Type.String())
			continue
		}
		if ruleItem.KeyItems == nil {
			continue
		}
		for _, keyItem := range ruleItem.KeyItems {
			if !identifierChecker.Check(reqInfos, ruleItem.Identifier, keyItem.Key) {
				continue
			}

			timeWindow := keyItem.Time.convertToSeconds()
			if timeWindow == ErrorTimeDuration {
				logging.Error(errors.New("error time window"), "error time window in llm_token_ratelimit.BaseRuleCollector.Collect()")
				continue
			}

			params := &BaseLimitKeyParams{
				RuleName:       rule.RuleName,
				Strategy:       rule.Strategy,
				IdentifierType: ruleItem.Identifier.Type,
				TimeWindow:     timeWindow,
				TokenSize:      keyItem.Token.Number,
				CountStrategy:  keyItem.Token.CountStrategy,
				// PETA
				Encoding: rule.Encoding,
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

func (c *BaseRuleCollector) addMatchedRule(params *BaseLimitKeyParams, ruleMap map[string]*MatchedRule) {
	if c == nil {
		return
	}
	if params.CountStrategy != TotalTokens {
		limitKey, err := c.generateLimitKey(params)
		if err != nil {
			logging.Error(err, "generateLimitKey failed in llm_token_ratelimit.BaseRuleCollector.addMatchedRule()",
				"params", params,
			)
			return
		}
		ruleMap[limitKey] = &MatchedRule{
			Strategy:      params.Strategy,
			LimitKey:      limitKey,
			TimeWindow:    params.TimeWindow,
			TokenSize:     params.TokenSize,
			CountStrategy: params.CountStrategy,
			// PETA
			Encoding: params.Encoding,
		}
	}
	params.CountStrategy = TotalTokens
	limitKey, err := c.generateLimitKey(params)
	if err != nil {
		logging.Error(err, "generateLimitKey failed in llm_token_ratelimit.BaseRuleCollector.addMatchedRule()",
			"params", params,
		)
		return
	}
	ruleMap[limitKey] = &MatchedRule{
		Strategy:      params.Strategy,
		LimitKey:      limitKey,
		TimeWindow:    params.TimeWindow,
		TokenSize:     params.TokenSize,
		CountStrategy: params.CountStrategy,
		// PETA
		Encoding: params.Encoding,
	}
}

func (c *BaseRuleCollector) generateLimitKey(params *BaseLimitKeyParams) (string, error) {
	if c == nil {
		return "", fmt.Errorf("BaseRuleCollector is nil")
	}
	return fmt.Sprintf(RedisRatelimitKeyFormat,
		params.RuleName,
		params.Strategy.String(),
		params.IdentifierType.String(),
		params.TimeWindow,
		params.CountStrategy.String(),
	), nil
}
