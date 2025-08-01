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
	"github.com/alibaba/sentinel-golang/core/base"
)

const (
	RuleCheckSlotOrder uint32 = 6000
)

var (
	DefaultSlot = &Slot{}
)

type Slot struct{}

func (s *Slot) Order() uint32 {
	return RuleCheckSlotOrder
}

func (s *Slot) Check(ctx *base.EntryContext) *base.TokenResult {
	resource := ctx.Resource.Name()
	result := ctx.RuleCheckResult
	if len(resource) == 0 {
		return result
	}

	if passed, rule := checkPass(ctx); !passed {
		msg := "llm token ratelimit check blocked"
		if result == nil {
			result = base.NewTokenResultBlockedWithCause(base.BlockTypeLLMTokenRateLimit, msg, rule, nil)
		} else {
			result.ResetToBlockedWithCause(base.BlockTypeLLMTokenRateLimit, msg, rule, nil)
		}
	}

	return result
}

func checkPass(ctx *base.EntryContext) (bool, *Rule) {
	llmTokenRatelimitCtx := extractContextFromArgs(ctx)
	if llmTokenRatelimitCtx == nil {
		return true, nil
	}
	for _, rule := range getRulesOfResource(ctx.Resource.Name()) {
		if !globalRuleMatcher.checkPass(llmTokenRatelimitCtx, rule) {
			return false, rule
		}
	}
	return true, nil
}
