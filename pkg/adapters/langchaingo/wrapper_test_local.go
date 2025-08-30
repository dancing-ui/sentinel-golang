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
	"testing"

	sentinel "github.com/alibaba/sentinel-golang/api"
	llmtokenratelimit "github.com/alibaba/sentinel-golang/core/llm_token_ratelimit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmc/langchaingo/llms"
)

// TODO: Mock redis client
func initSentinel(_ *testing.T) {
	if err := sentinel.InitDefault(); err != nil {
		panic(err)
	}

	_, err := llmtokenratelimit.LoadRules([]*llmtokenratelimit.Rule{
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
	})

	if err != nil {
		panic(err)
	}
}

type MockLLM struct {
	response  *llms.ContentResponse
	err       error
	callCount int
}

func (m *MockLLM) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	if m.response != nil {
		return m.response, nil
	}

	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				Content: "Mock response",
				GenerationInfo: map[string]any{
					"PromptTokens":     10,
					"CompletionTokens": 5,
					"TotalTokens":      15,
				},
			},
		},
	}, nil
}

func (m *MockLLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return "", nil
}

func TestLLMWrapper(t *testing.T) {
	initSentinel(t)

	type args struct {
		model      string
		ctx        context.Context
		messages   []llms.MessageContent
		options    []Option
		llmOptions []llms.CallOption
		mockLLM    *MockLLM
	}
	type want struct {
		shouldSucceed bool
		errorContains string
		callCount     int
	}

	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "successful_request_with_default_options",
			args: args{
				model: "gpt-3.5-turbo",
				ctx:   context.Background(),
				messages: []llms.MessageContent{
					llms.TextParts(llms.ChatMessageType(llms.ChatMessageTypeHuman), "Hello, world!"),
				},
				options:    []Option{},
				llmOptions: []llms.CallOption{},
				mockLLM: &MockLLM{
					response: &llms.ContentResponse{
						Choices: []*llms.ContentChoice{
							{
								Content: "Hello back!",
								GenerationInfo: map[string]any{
									"PromptTokens":     5,
									"CompletionTokens": 3,
									"TotalTokens":      8,
								},
							},
						},
					},
				},
			},
			want: want{
				shouldSucceed: true,
				callCount:     1,
			},
		},
		{
			name: "request_with_custom_resource_allow",
			args: args{
				model: "gpt-3.5-turbo",
				ctx:   context.Background(),
				messages: []llms.MessageContent{
					llms.TextParts(llms.ChatMessageType(llms.ChatMessageTypeHuman), "Test message"),
				},
				options: []Option{
					WithDefaultResource("test-resource-allow"),
				},
				llmOptions: []llms.CallOption{},
				mockLLM:    &MockLLM{},
			},
			want: want{
				shouldSucceed: true,
				callCount:     1,
			},
		},
		{
			name: "request_with_rate_limit_block",
			args: args{
				model: "gpt-3.5-turbo",
				ctx:   context.Background(),
				messages: []llms.MessageContent{
					llms.TextParts(llms.ChatMessageType(llms.ChatMessageTypeHuman), "This should be blocked due to rate limit"),
				},
				options: []Option{
					WithDefaultResource("test-resource-block"),
					WithPromptsExtract(func(messages []llms.MessageContent) []string {
						prompts := make([]string, 0, len(messages))
						for _, msg := range messages {
							for _, part := range msg.Parts {
								if textPart, ok := part.(llms.TextContent); ok {
									prompts = append(prompts, textPart.Text)
								}
							}
						}
						return prompts
					}),
				},
				llmOptions: []llms.CallOption{},
				mockLLM:    &MockLLM{},
			},
			want: want{
				shouldSucceed: false,
				errorContains: "blocked",
				callCount:     0,
			},
		},
		{
			name: "request_with_custom_resource_extract",
			args: args{
				model: "gpt-3.5-turbo",
				ctx:   context.WithValue(context.Background(), "resource", "test-resource-allow"),
				messages: []llms.MessageContent{
					llms.TextParts(llms.ChatMessageType(llms.ChatMessageTypeHuman), "Custom resource test"),
				},
				options: []Option{
					WithResourceExtract(func(ctx context.Context) string {
						if resource := ctx.Value("resource"); resource != nil {
							return resource.(string)
						}
						return "default"
					}),
				},
				llmOptions: []llms.CallOption{},
				mockLLM:    &MockLLM{},
			},
			want: want{
				shouldSucceed: true,
				callCount:     1,
			},
		},
		{
			name: "request_with_custom_prompts_extract",
			args: args{
				model: "gpt-3.5-turbo",
				ctx:   context.Background(),
				messages: []llms.MessageContent{
					llms.TextParts(llms.ChatMessageType(llms.ChatMessageTypeHuman), "Message 1"),
					llms.TextParts(llms.ChatMessageType(llms.ChatMessageTypeAI), "Response 1"),
					llms.TextParts(llms.ChatMessageType(llms.ChatMessageTypeHuman), "Message 2"),
				},
				options: []Option{
					WithDefaultResource("test-resource-allow"),
					WithPromptsExtract(func(messages []llms.MessageContent) []string {
						prompts := make([]string, 0, len(messages))
						for _, msg := range messages {
							for _, part := range msg.Parts {
								if textPart, ok := part.(llms.TextContent); ok {
									prompts = append(prompts, textPart.Text)
								}
							}
						}
						return prompts
					}),
				},
				llmOptions: []llms.CallOption{},
				mockLLM:    &MockLLM{},
			},
			want: want{
				shouldSucceed: true,
				callCount:     1,
			},
		},
		{
			name: "request_with_custom_request_infos_extract",
			args: args{
				model: "gpt-3.5-turbo",
				ctx:   context.WithValue(context.Background(), "user_id", "test-user-123"),
				messages: []llms.MessageContent{
					llms.TextParts(llms.ChatMessageType(llms.ChatMessageTypeHuman), "User specific request"),
				},
				options: []Option{
					WithDefaultResource("test-resource-allow"),
					WithRequestInfosExtract(func(ctx context.Context) *llmtokenratelimit.RequestInfos {
						userId := "anonymous"
						if userVal := ctx.Value("user_id"); userVal != nil {
							userId = userVal.(string)
						}

						return llmtokenratelimit.GenerateRequestInfos(
							llmtokenratelimit.WithHeader(map[string][]string{
								"user_id": {userId},
							}),
						)
					}),
				},
				llmOptions: []llms.CallOption{},
				mockLLM:    &MockLLM{},
			},
			want: want{
				shouldSucceed: true,
				callCount:     1,
			},
		},
		{
			name: "request_with_block_fallback",
			args: args{
				model: "gpt-3.5-turbo",
				ctx:   context.Background(),
				messages: []llms.MessageContent{
					llms.TextParts(llms.ChatMessageType(llms.ChatMessageTypeHuman), "This should trigger fallback"),
				},
				options: []Option{
					WithDefaultResource("test-resource-block"),
					WithPromptsExtract(func(messages []llms.MessageContent) []string {
						prompts := make([]string, 0, len(messages))
						for _, msg := range messages {
							for _, part := range msg.Parts {
								if textPart, ok := part.(llms.TextContent); ok {
									prompts = append(prompts, textPart.Text)
								}
							}
						}
						return prompts
					}),
					WithBlockFallback(func(ctx context.Context) {
						fmt.Println("[Fallback] Request was blocked")
					}),
				},
				llmOptions: []llms.CallOption{},
				mockLLM:    &MockLLM{},
			},
			want: want{
				shouldSucceed: false,
				errorContains: "blocked",
				callCount:     0,
			},
		},
		{
			name: "request_with_llm_error",
			args: args{
				model: "gpt-3.5-turbo",
				ctx:   context.Background(),
				messages: []llms.MessageContent{
					llms.TextParts(llms.ChatMessageType(llms.ChatMessageTypeHuman), "This will cause LLM error"),
				},
				options: []Option{
					WithDefaultResource("test-resource-allow"),
				},
				llmOptions: []llms.CallOption{},
				mockLLM: &MockLLM{
					err: assert.AnError,
				},
			},
			want: want{
				shouldSucceed: false,
				errorContains: "assert.AnError",
				callCount:     1,
			},
		},
		{
			name: "request_with_custom_token_extractor",
			args: args{
				model: "gpt-3.5-turbo",
				ctx:   context.Background(),
				messages: []llms.MessageContent{
					llms.TextParts(llms.ChatMessageType(llms.ChatMessageTypeHuman), "Custom token extraction test"),
				},
				options: []Option{
					WithDefaultResource("test-resource-allow"),
					WithUsedTokenInfosExtract(func(response interface{}) *llmtokenratelimit.UsedTokenInfos {
						return llmtokenratelimit.GenerateUsedTokenInfos(
							llmtokenratelimit.WithInputTokens(20),
							llmtokenratelimit.WithOutputTokens(10),
							llmtokenratelimit.WithTotalTokens(30),
						)
					}),
				},
				llmOptions: []llms.CallOption{},
				mockLLM: &MockLLM{
					response: &llms.ContentResponse{
						Choices: []*llms.ContentChoice{
							{
								Content: "Custom token response",
							},
						},
					},
				},
			},
			want: want{
				shouldSucceed: true,
				callCount:     1,
			},
		},
		{
			name: "request_with_empty_messages",
			args: args{
				model:      "gpt-3.5-turbo",
				ctx:        context.Background(),
				messages:   []llms.MessageContent{},
				options:    []Option{WithDefaultResource("test-resource-allow")},
				llmOptions: []llms.CallOption{},
				mockLLM:    &MockLLM{},
			},
			want: want{
				shouldSucceed: true,
				callCount:     1,
			},
		},
		{
			name: "request_with_multiple_options",
			args: args{
				model: "gpt-3.5-turbo",
				ctx:   context.WithValue(context.Background(), "user_id", "multi-user"),
				messages: []llms.MessageContent{
					llms.TextParts(llms.ChatMessageType(llms.ChatMessageTypeHuman), "Multiple options test"),
				},
				options: []Option{
					WithDefaultResource("test-resource-allow"),
					WithResourceExtract(func(ctx context.Context) string {
						return "test-resource-allow" // 覆盖默认资源
					}),
					WithRequestInfosExtract(func(ctx context.Context) *llmtokenratelimit.RequestInfos {
						userId := "default"
						if userVal := ctx.Value("user_id"); userVal != nil {
							userId = userVal.(string)
						}
						return llmtokenratelimit.GenerateRequestInfos(
							llmtokenratelimit.WithHeader(map[string][]string{
								"user_id": {userId},
							}),
						)
					}),
					WithPromptsExtract(func(messages []llms.MessageContent) []string {
						return []string{"extracted prompt"}
					}),
					WithBlockFallback(func(ctx context.Context) {
						fmt.Println("[Fallback] Request was blocked")
					}),
				},
				llmOptions: []llms.CallOption{},
				mockLLM:    &MockLLM{},
			},
			want: want{
				shouldSucceed: true,
				callCount:     1,
			},
		},
		{
			name: "request_with_multiple_text_parts",
			args: args{
				model: "gpt-3.5-turbo",
				ctx:   context.Background(),
				messages: []llms.MessageContent{
					llms.TextParts(llms.ChatMessageType(llms.ChatMessageTypeSystem), "You are a helpful assistant"),
					llms.TextParts(llms.ChatMessageType(llms.ChatMessageTypeHuman), "What is the weather like?"),
					llms.TextParts(llms.ChatMessageType(llms.ChatMessageTypeAI), "I don't have access to weather data"),
					llms.TextParts(llms.ChatMessageType(llms.ChatMessageTypeHuman), "Thank you anyway"),
				},
				options: []Option{
					WithDefaultResource("test-resource-allow"),
				},
				llmOptions: []llms.CallOption{},
				mockLLM:    &MockLLM{},
			},
			want: want{
				shouldSucceed: true,
				callCount:     1,
			},
		},
		{
			name: "request_with_different_message_types",
			args: args{
				model: "gpt-3.5-turbo",
				ctx:   context.Background(),
				messages: []llms.MessageContent{
					llms.TextParts(llms.ChatMessageType(llms.ChatMessageTypeSystem), "System prompt"),
					llms.TextParts(llms.ChatMessageType(llms.ChatMessageTypeHuman), "Human message"),
					llms.TextParts(llms.ChatMessageType(llms.ChatMessageTypeAI), "AI response"),
					llms.TextParts(llms.ChatMessageType(llms.ChatMessageTypeFunction), "Function call"),
				},
				options: []Option{
					WithDefaultResource("test-resource-allow"),
				},
				llmOptions: []llms.CallOption{},
				mockLLM:    &MockLLM{},
			},
			want: want{
				shouldSucceed: true,
				callCount:     1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fmt.Println("=== Running test:", tt.name, "===")

			tt.args.mockLLM.callCount = 0

			llmWrapper := NewLLMWrapper(tt.args.mockLLM, tt.args.options...)
			require.NotNil(t, llmWrapper)

			response, err := llmWrapper.GenerateContent(tt.args.ctx, tt.args.messages, tt.args.llmOptions...)

			if tt.want.shouldSucceed {
				assert.NoError(t, err)
				assert.NotNil(t, response)
			} else {
				assert.Error(t, err)
				if tt.want.errorContains != "" {
					assert.Contains(t, err.Error(), tt.want.errorContains)
				}
			}

			assert.Equal(t, tt.want.callCount, tt.args.mockLLM.callCount)
		})
	}
}
