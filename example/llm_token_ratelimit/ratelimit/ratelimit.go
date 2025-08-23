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

package ratelimit

import (
	"errors"
	"net/http"

	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
	llmtokenratelimit "github.com/alibaba/sentinel-golang/core/llm_token_ratelimit"
	"github.com/gin-gonic/gin"
)

func InitSentinel() {
	if err := sentinel.InitDefault(); err != nil {
		panic(err)
	}

	_, err := llmtokenratelimit.LoadRules([]*llmtokenratelimit.Rule{
		{

			Resource: "POST:/v1/chat/completion/fixed_window",
			Strategy: llmtokenratelimit.FixedWindow,
			RuleName: "rule-fixed-window",
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
								Number:        65,
								CountStrategy: llmtokenratelimit.TotalTokens,
							},
							Time: llmtokenratelimit.Time{
								Unit:  llmtokenratelimit.Second,
								Value: 10,
							},
						},
					},
				},
			},
		},
		{

			Resource: "POST:/v1/chat/completion/peta",
			Strategy: llmtokenratelimit.PETA,
			RuleName: "rule-peta",
			Encoding: llmtokenratelimit.TokenEncoding{},
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
								Number:        65,
								CountStrategy: llmtokenratelimit.TotalTokens,
							},
							Time: llmtokenratelimit.Time{
								Unit:  llmtokenratelimit.Second,
								Value: 10,
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
}

func SentinelMiddleware(opts ...Option) gin.HandlerFunc {
	options := evaluateOptions(opts)
	return func(c *gin.Context) {
		resource := c.Request.Method + ":" + c.FullPath()

		if options.resourceExtract != nil {
			resource = options.resourceExtract(c)
		}

		reqInfos := llmtokenratelimit.GenerateRequestInfos(
			llmtokenratelimit.WithHeader(c.Request.Header),
		)

		if options.requestInfosExtract != nil {
			reqInfos = options.requestInfosExtract(c)
		}

		prompts := []string{}
		if options.promptsExtract != nil {
			prompts = options.promptsExtract(c)
		}

		// wrapper request
		llmTokenRatelimitCtx := llmtokenratelimit.NewContext()
		if llmTokenRatelimitCtx == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": errors.New("failed to create llm token ratelimit context"),
			})
			return
		}
		llmTokenRatelimitCtx.Set(llmtokenratelimit.KeyRequestInfos, reqInfos)
		llmTokenRatelimitCtx.Set(llmtokenratelimit.KeyLLMPrompts, prompts)

		// check
		entry, err := sentinel.Entry(resource, sentinel.WithTrafficType(base.Inbound), sentinel.WithArgs(llmTokenRatelimitCtx))

		if err != nil {
			// Block
			if options.blockFallback != nil {
				options.blockFallback(c)
			} else {
				setResponseHeaders(c, llmTokenRatelimitCtx)
				c.AbortWithStatusJSON(int(llmtokenratelimit.GetErrorCode()), gin.H{
					"error": llmtokenratelimit.GetErrorMsg(),
				})
			}
			return
		}
		// Pass

		// Wait for the response to be written
		c.Next()

		// update used token info
		usedTokenInfos, exists := c.Get(llmtokenratelimit.KeyUsedTokenInfos)
		if !exists || usedTokenInfos == nil {
			return
		}
		llmTokenRatelimitCtx.Set(llmtokenratelimit.KeyUsedTokenInfos, usedTokenInfos)
		entry.SetPair(llmtokenratelimit.KeyContext, llmTokenRatelimitCtx)

		entry.Exit() // Must be executed immediately after the SetPair function
	}
}

func setResponseHeaders(c *gin.Context, ctx *llmtokenratelimit.Context) {
	if c == nil || ctx == nil {
		return
	}

	responseHeaders, ok := ctx.Get(llmtokenratelimit.KeyResponseHeaders).(*llmtokenratelimit.ResponseHeader)
	if !ok || responseHeaders == nil {
		return
	}

	for key, value := range responseHeaders.GetAll() {
		c.Header(key, value)
	}
}
