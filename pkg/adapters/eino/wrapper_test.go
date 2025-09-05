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

package eino

import (
	"context"
	"fmt"
	"os"
	"testing"

	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/config"
	llmtokenratelimit "github.com/alibaba/sentinel-golang/core/llm_token_ratelimit"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func initSentinel(t *testing.T) {
	conf := config.NewDefaultConfig()
	conf.Sentinel.LLMTokenRateLimit = &llmtokenratelimit.Config{
		Enabled: true,
	}
	if err := sentinel.InitWithConfig(conf); err != nil {
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

type EinoClient struct {
	llm model.BaseChatModel
}

func NewEinoClient() (*EinoClient, error) {
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

	llm, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create LangChain LLM client: %w", err)
	}

	return &EinoClient{
		llm: llm,
	}, nil
}

func TestLLMWrapperGenerate(t *testing.T) {
	type args struct {
		opts      []Option
		messages  []*schema.Message
		llmOption []model.Option
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
					opts: []Option{
						WithResourceExtract(func(ctx context.Context) string {
							return "test-resource-allow"
						}),
					},
					messages: []*schema.Message{
						schema.SystemMessage("You are a helpful assistant."),
						schema.UserMessage("Hello, how are you?"),
					},
					llmOption: []model.Option{},
				},
				want: want{
					pass: true,
				},
			},
			{
				name: "default block",
				args: args{
					opts: []Option{
						WithResourceExtract(func(ctx context.Context) string {
							return "test-resource-block"
						}),
					},
					messages: []*schema.Message{
						schema.SystemMessage("You are a helpful assistant."),
						schema.UserMessage("Hello, how are you?"),
					},
					llmOption: []model.Option{},
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
			client, err := NewEinoClient()
			if err != nil {
				t.Fatalf("failed to create LangChain client: %v", err)
			}

			llm := NewLLMWrapper(client.llm, tt.args.opts...)

			response, err := llm.Generate(context.Background(), tt.args.messages, tt.args.llmOption...)
			_ = response
			if tt.want.pass {
				if err != nil {
					t.Fatalf("LLMWrapper.GenerateContent() error = %v, want pass", err)
				}
			} else {
				if err == nil {
					t.Fatalf("LLMWrapper.GenerateContent() error = %v, want block", err)
				}
			}
		})
	}
}
