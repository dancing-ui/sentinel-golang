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

package langchaingo

import (
	"context"
	"fmt"
	"os"
	"testing"

	sentinel "github.com/alibaba/sentinel-golang/api"
	llmtokenratelimit "github.com/alibaba/sentinel-golang/core/llm_token_ratelimit"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

func initSentinel(t *testing.T) {
	if err := sentinel.InitDefault(); err != nil {
		t.Fatalf("Unexpected error: %+v", err)
	}

	if _, err := llmtokenratelimit.LoadRules([]*llmtokenratelimit.Rule{
		{
			Resource: "test-resource-allow",
			Strategy: llmtokenratelimit.PETA,
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
								Number:        10000,
								CountStrategy: llmtokenratelimit.TotalTokens,
							},
							Time: llmtokenratelimit.Time{
								Unit:  llmtokenratelimit.Second,
								Value: 1,
							},
						},
					},
				},
			},
		},
		{
			Resource: "test-resource-block",
			Strategy: llmtokenratelimit.PETA,
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
								Number:        1,
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
	}); err != nil {
		t.Fatalf("Unexpected error: %+v", err)
	}
}

type LangChainClient struct {
	llm llms.Model
}

func NewLangChainClient() (*LangChainClient, error) {
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("LLM_API_KEY environment variable is not set")
	}

	baseURL := os.Getenv("LLM_BASE_URL")
	if baseURL == "" {
		return nil, fmt.Errorf("LLM_BASE_URL environment variable is not set")
	}

	model := os.Getenv("LLM_MODEL")
	if baseURL == "" {
		return nil, fmt.Errorf("LLM_MODEL environment variable is not set")
	}

	llm, err := openai.New(
		openai.WithToken(apiKey),
		openai.WithBaseURL(baseURL),
		openai.WithModel(model),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create LangChain LLM client: %w", err)
	}

	return &LangChainClient{
		llm: llm,
	}, nil
}

func TestLLMWrapper(t *testing.T) {
	type args struct {
		opts      []Option
		resource  string
		messages  []llms.MessageContent
		llmOption []llms.CallOption
	}
	type want struct {
		pass bool
	}
	var (
		tests = []struct {
			name string
			args args
			want want
		}{
			{
				name: "default allow",
				args: args{
					opts:     []Option{},
					resource: "test-resource-allow",
					messages: []llms.MessageContent{
						llms.TextParts(llms.ChatMessageTypeSystem, "You are a helpful assistant."),
						llms.TextParts(llms.ChatMessageTypeHuman, "Hello, how are you?"),
					},
					llmOption: []llms.CallOption{},
				},
				want: want{
					pass: true,
				},
			},
			{
				name: "default block",
				args: args{
					opts:     []Option{},
					resource: "test-resource-block",
					messages: []llms.MessageContent{
						llms.TextParts(llms.ChatMessageTypeSystem, "You are a helpful assistant."),
						llms.TextParts(llms.ChatMessageTypeHuman, "Hello, how are you?"),
					},
					llmOption: []llms.CallOption{},
				},
				want: want{
					pass: false,
				},
			},
		}
	)
	initSentinel(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewLangChainClient()
			if err != nil {
				t.Fatalf("failed to create LangChain client: %v", err)
			}

			llm := NewLLMWrapper(client.llm, tt.args.opts...)

			response, err := llm.GenerateContent(context.Background(), tt.args.messages, tt.args.llmOption...)
			if err != nil {
				if tt.want.pass {
					t.Fatalf("LLMWrapper.GenerateContent() error = %v, want pass", err)
				} else {
					t.Logf("LLMWrapper.GenerateContent() error = %v", err)
				}
			}

			t.Logf("LLMWrapper.GenerateContent() response = %+v", response.Choices[0].Content)
			t.Logf("LLMWrapper.GenerateContent() usage = %+v", response.Choices[0].GenerationInfo)
		})
	}
}
