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
			SpecificItems: []*llmtokenratelimit.SpecificItem{
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
			Encoding: llmtokenratelimit.TokenEncoding{},
			SpecificItems: []*llmtokenratelimit.SpecificItem{
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

		prompts := []string{}
		if options.promptsExtract != nil {
			prompts = options.promptsExtract(c)
		}

		reqInfos := llmtokenratelimit.GenerateRequestInfos(
			llmtokenratelimit.WithHeader(c.Request.Header),
			llmtokenratelimit.WithPrompts(prompts),
		)

		if options.requestInfosExtract != nil {
			reqInfos = options.requestInfosExtract(c)
		}

		// check
		entry, err := sentinel.Entry(resource, sentinel.WithTrafficType(base.Inbound), sentinel.WithArgs(reqInfos))

		if err != nil {
			// Block
			if options.blockFallback != nil {
				options.blockFallback(c)
			} else {
				setResponseHeaders(c, err.TriggeredValue().(*llmtokenratelimit.ResponseHeader))
				c.AbortWithStatusJSON(int(llmtokenratelimit.GetErrorCode()), gin.H{
					"error": llmtokenratelimit.GetErrorMsg(),
				})
			}
			return
		}
		// Pass or Disabled
		c.Next()
		// Set response headers
		responseHeader, ok := entry.Context().GetPair(llmtokenratelimit.KeyResponseHeaders).(*llmtokenratelimit.ResponseHeader)
		if ok && responseHeader != nil {
			setResponseHeaders(c, responseHeader)
		}
		// Update used token info
		usedTokenInfos, exists := c.Get(llmtokenratelimit.KeyUsedTokenInfos)
		if exists && usedTokenInfos != nil {
			entry.SetPair(llmtokenratelimit.KeyUsedTokenInfos, usedTokenInfos)
		}
		entry.Exit() // Must be executed immediately after the SetPair function
	}
}

func setResponseHeaders(c *gin.Context, header *llmtokenratelimit.ResponseHeader) {
	if c == nil || header == nil {
		return
	}

	for key, value := range header.GetAll() {
		c.Header(key, value)
	}
}
