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
	"strconv"

	"errors"

	"github.com/alibaba/sentinel-golang/logging"
)

const (
	FixedWindowQueryScript string = `
	local ttl = redis.call('ttl', KEYS[1])
	if ttl < 0 then
		redis.call('set', KEYS[1], ARGV[1], 'EX', ARGV[2])
		return {ARGV[1], ARGV[1], ARGV[2]}
	end
	return {ARGV[1], redis.call('get', KEYS[1]), ttl}
	`
)

type FixedWindowChecker struct{}

func (c *FixedWindowChecker) Check(ctx *Context, rules []*MatchedRule) bool {
	if len(rules) == 0 {
		return true
	}

	for _, rule := range rules {
		if !c.checkLimitKey(ctx, rule) {
			return false
		}
	}
	return true
}

func (c *FixedWindowChecker) checkLimitKey(ctx *Context, rule *MatchedRule) bool {
	keys := []string{rule.LimitKey}
	args := []interface{}{rule.TokenSize, rule.TimeWindow}
	response, err := globalRedisClient.Eval(FixedWindowQueryScript, keys, args...)
	if err != nil {
		logging.Error(err, "failed to execute redis script in llm_token_ratelimit.FixedWindowChecker.checkLimitKey()")
		return true
	}
	result := c.parseRedisResponse(response)
	if result == nil || len(result) != 3 {
		logging.Error(errors.New("invalid redis response"), "invalid redis response in llm_token_ratelimit.FixedWindowChecker.checkLimitKey()", "response", response)
		return true
	}

	remaining := result[1]
	if remaining < 0 {
		// TODO: adding LLM response headers using Context​
		return false
	}

	return true
}

func (c *FixedWindowChecker) parseRedisResponse(response interface{}) []int64 {
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
