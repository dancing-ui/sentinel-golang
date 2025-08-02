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
	_ "embed"
	"fmt"

	"errors"

	"github.com/alibaba/sentinel-golang/logging"
	"github.com/alibaba/sentinel-golang/util"
	"github.com/pkoukk/tiktoken-go"
)

type FixedWindowChecker struct{}

func (c *FixedWindowChecker) Check(ctx *Context, rules []*MatchedRule) bool {
	if c == nil {
		return true
	}

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
	if c == nil {
		return true
	}

	keys := []string{rule.LimitKey}
	args := []interface{}{rule.TokenSize, rule.TimeWindow}
	response, err := globalRedisClient.Eval(FixedWindowQueryScript, keys, args...)
	if err != nil {
		logging.Error(err, "failed to execute redis script in llm_token_ratelimit.FixedWindowChecker.checkLimitKey()")
		return true
	}
	result := parseRedisResponse(response)
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

// ================================= PETAChecker ====================================

//go:embed script/peta/peta_withhold.lua
var globalPETAWithholdScript string

type PETAChecker struct{}

func (c *PETAChecker) Check(ctx *Context, rules []*MatchedRule) bool {
	if c == nil || ctx == nil {
		return true
	}

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

func (c *PETAChecker) checkLimitKey(ctx *Context, rule *MatchedRule) bool {
	if c == nil || ctx == nil || rule == nil {
		return true
	}

	promptsValue := ctx.GetContext(KeyLLMPrompts)
	if promptsValue == nil {
		return true
	}
	prompts, ok := promptsValue.(string)
	if !ok {
		return true
	}

	estimatedToken, err := c.countEncodingTokens(prompts, rule.Encoding)
	if err != nil {
		logging.Error(err, "failed to withhold tokens in llm_token_ratelimit.PETAChecker.checkLimitKey()")
		return true
	}

	slidingWindowKey := fmt.Sprintf(PETASlidingWindowKeyFormat, rule.LimitKey)
	tokenBucketKey := fmt.Sprintf(PETATokenBucketKeyFormat, rule.LimitKey)

	keys := []string{slidingWindowKey, tokenBucketKey}
	args := []interface{}{estimatedToken, util.CurrentTimeMillis(), rule.TokenSize, rule.TimeWindow * 1000, generateRandomString(PETARandomStringLength)}
	response, err := globalRedisClient.Eval(globalPETAWithholdScript, keys, args...)
	if err != nil {
		logging.Error(err, "failed to execute redis script in llm_token_ratelimit.PETAChecker.checkLimitKey()")
		return true
	}
	result := parseRedisResponse(response)
	if result == nil || len(result) != 1 {
		logging.Error(errors.New("invalid redis response"), "invalid redis response in llm_token_ratelimit.PETAChecker.checkLimitKey()", "response", response)
		return true
	}

	// TODO: add waiting callback
	waitingTime := result[0]
	if waitingTime != PETANoWaiting {
		// TODO: adding LLM response headers using Context​
		return false
	}
	c.setEstimatedToken(rule, int64(estimatedToken))
	return true
}

func (c *PETAChecker) countEncodingTokens(texts string, encoding TiktokenEncoding) (int, error) {
	if c == nil {
		return -1, fmt.Errorf("PETAChecker is nil")
	}

	tke, err := tiktoken.GetEncoding(encoding.String())
	if err != nil {
		return -1, fmt.Errorf("getEncoding: %v", err)
	}
	return len(tke.Encode(texts, nil, nil)), nil
}

func (c *PETAChecker) setEstimatedToken(rule *MatchedRule, count int64) {
	if c == nil || rule == nil {
		return
	}
	rule.EstimatedToken = count
}
