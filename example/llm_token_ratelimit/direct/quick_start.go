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

package main

import (
	"time"

	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
	llmtokenratelimit "github.com/alibaba/sentinel-golang/core/llm_token_ratelimit"
	"github.com/alibaba/sentinel-golang/logging"
)

type TestCase struct {
	Resource string
	Data     map[string]interface{}
}

func main() {
	if err := sentinel.InitDefault(); err != nil {
		panic(err)
	}

	_, err := llmtokenratelimit.LoadRules([]*llmtokenratelimit.Rule{
		{

			Resource: "/a/.*",
			Strategy: llmtokenratelimit.FixedWindow,
			RuleName: "rule-a",
			RuleItems: []*llmtokenratelimit.RuleItem{
				{
					Identifier: llmtokenratelimit.Identifier{
						Type:  llmtokenratelimit.Header,
						Value: ".*",
					},
					KeyItems: []*llmtokenratelimit.KeyItem{
						{
							Key: ".*",
							Token: llmtokenratelimit.Token{
								Number:        1000,
								CountStrategy: llmtokenratelimit.InputTokens,
							},
							Time: llmtokenratelimit.Time{
								Unit:  llmtokenratelimit.Second,
								Value: 5,
							},
						},
						{
							Key: "12.*",
							Token: llmtokenratelimit.Token{
								Number:        1000,
								CountStrategy: llmtokenratelimit.InputTokens,
							},
							Time: llmtokenratelimit.Time{
								Unit:  llmtokenratelimit.Second,
								Value: 5,
							},
						},
					},
				},
			},
		},
		{

			Resource: "/b/.*",
			Strategy: llmtokenratelimit.FixedWindow,
			RuleName: "rule-b",
			RuleItems: []*llmtokenratelimit.RuleItem{
				{
					Identifier: llmtokenratelimit.Identifier{
						Type:  llmtokenratelimit.Header,
						Value: ".*",
					},
					KeyItems: []*llmtokenratelimit.KeyItem{
						{
							Key: ".*",
							Token: llmtokenratelimit.Token{
								Number:        1000,
								CountStrategy: llmtokenratelimit.InputTokens,
							},
							Time: llmtokenratelimit.Time{
								Unit:  llmtokenratelimit.Second,
								Value: 5,
							},
						},
						{
							Key: "12.*",
							Token: llmtokenratelimit.Token{
								Number:        1000,
								CountStrategy: llmtokenratelimit.InputTokens,
							},
							Time: llmtokenratelimit.Time{
								Unit:  llmtokenratelimit.Second,
								Value: 5,
							},
						},
					},
				},
			},
		},
	})

	if err != nil {
		panic(err)
	}

	testCases := []TestCase{
		{
			Resource: "/a/b",
			Data: map[string]interface{}{
				llmtokenratelimit.KeyRequestInfos: llmtokenratelimit.GenerateRequestInfos(
					llmtokenratelimit.WithHeader(map[string]string{
						"X-CA-A": "123",
					})),
				"llmRunningTime": time.Duration(1) * time.Second,
				llmtokenratelimit.KeyUsedTokenInfos: llmtokenratelimit.GenerateUsedTokenInfos(
					llmtokenratelimit.WithInputTokens(2000),
				),
			},
		},
		{
			Resource: "/a/c",
			Data: map[string]interface{}{
				llmtokenratelimit.KeyRequestInfos: llmtokenratelimit.GenerateRequestInfos(
					llmtokenratelimit.WithHeader(map[string]string{
						"X-CA-A": "123",
					})),
				"llmRunningTime": time.Duration(1) * time.Second,
				llmtokenratelimit.KeyUsedTokenInfos: llmtokenratelimit.GenerateUsedTokenInfos(
					llmtokenratelimit.WithInputTokens(1000),
				),
			},
		},
		{
			Resource: "/b/c",
			Data: map[string]interface{}{
				llmtokenratelimit.KeyRequestInfos: llmtokenratelimit.GenerateRequestInfos(
					llmtokenratelimit.WithHeader(map[string]string{
						"X-CA-A": "123",
					})),
				"llmRunningTime": time.Duration(1) * time.Second,
				llmtokenratelimit.KeyUsedTokenInfos: llmtokenratelimit.GenerateUsedTokenInfos(
					llmtokenratelimit.WithInputTokens(1000),
				),
			},
		},
	}

	for _, testCase := range testCases {
		resource := testCase.Resource
		data := testCase.Data
		logging.Info("resource: %s", resource)
		// wrapper request
		ctx := new(llmtokenratelimit.Context)
		ctx.SetContext(llmtokenratelimit.KeyRequestInfos, data[llmtokenratelimit.KeyRequestInfos])

		// check
		e, b := sentinel.Entry(resource, sentinel.WithTrafficType(base.Inbound), sentinel.WithArgs(ctx))

		if b != nil {
			// Blocked
			println(resource, ": Blocked")
			continue
		}
		// Pass
		println(resource, ": Pass")

		// simulate llm service wasted time
		time.Sleep(data["llmRunningTime"].(time.Duration))

		// update used token info
		ctx.SetContext(llmtokenratelimit.KeyUsedTokenInfos, data[llmtokenratelimit.KeyUsedTokenInfos])
		e.SetPair(llmtokenratelimit.KeyContext, ctx)

		e.Exit() // Must be executed immediately after the SetPair function
	}
}
