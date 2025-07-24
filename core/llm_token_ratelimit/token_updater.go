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
	"strconv"

	"github.com/alibaba/sentinel-golang/logging"
)

const (
	FixedWindowUpdateScript string = `
	local ttl = redis.call('ttl', KEYS[1])
	if ttl < 0 then
		redis.call('set', KEYS[1], ARGV[1]-ARGV[3], 'EX', ARGV[2])
		return {ARGV[1], ARGV[1]-ARGV[3], ARGV[2]}
	end
	return {ARGV[1], redis.call('decrby', KEYS[1], ARGV[3]), ttl}
	`
)

type FixedWindowUpdater struct{}

func (u *FixedWindowUpdater) Update(ctx *Context, rules []*MatchedRule) {
	if len(rules) == 0 {
		return
	}

	usedTokenInfos := extractUsedTokenInfos(ctx)
	if usedTokenInfos == nil {
		return
	}

	for _, rule := range rules {
		u.updateLimitKey(ctx, rule, usedTokenInfos)
	}
}

func (u *FixedWindowUpdater) updateLimitKey(ctx *Context, rule *MatchedRule, infos *UsedTokenInfos) {
	calculator := tokenCalculator.getCalculator(rule.CountStrategy)
	if calculator == nil {
		logging.Error(errors.New("unknown strategy"), "unknown strategy in llm_token_ratelimit.updateLimitKeys() when get calculator", "strategy", rule.CountStrategy.String())
		return
	}
	usedToken := calculator.Calculate(ctx, infos)
	keys := []string{rule.LimitKey}
	args := []interface{}{rule.TokenSize, rule.TimeWindow, usedToken}
	client := getRedisClient()
	if client == nil {
		logging.Error(errors.New("nil redis client"), "redis client is nil in llm_token_ratelimit.FixedWindowUpdater.updateLimitKey()")
		return
	}
	response, err := getRedisClient().Eval(FixedWindowUpdateScript, keys, args...).Result()
	if err != nil {
		logging.Error(err, "failed to execute redis script in llm_token_ratelimit.FixedWindowUpdater.updateLimitKey()")
		return
	}
	result := u.parseRedisResponse(response)
	if result == nil || len(result) != 3 {
		logging.Error(errors.New("invalid redis response"), "invalid redis response in llm_token_ratelimit.FixedWindowUpdater.updateLimitKey()", "response", response)
		return
	}
}

func (u *FixedWindowUpdater) parseRedisResponse(response interface{}) []int64 {
	resultSlice, ok := response.([]interface{})
	if !ok || len(resultSlice) != 3 {
		return nil
	}

	result := make([]int64, 3)
	for i, v := range resultSlice {
		switch val := v.(type) {
		case int64:
			result[i] = val
		case string:
			num, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				logging.Error(err, "failed to parse redis response element in llm_token_ratelimit.FixedWindowChecker.parseRedisResponse()",
					"index", i,
					"value", val,
					"error", err.Error(),
				)
				return nil
			}
			result[i] = num
		case int:
			result[i] = int64(val)
		case float64:
			result[i] = int64(val)
		default:
			logging.Error(nil, "unexpected redis response element type in llm_token_ratelimit.FixedWindowChecker.parseRedisResponse()",
				"index", i,
				"value", v,
				"type", fmt.Sprintf("%T", v),
			)
			return nil
		}
	}
	return result
}
