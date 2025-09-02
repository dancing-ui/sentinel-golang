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
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sentinel "github.com/alibaba/sentinel-golang/api"
	llmtokenratelimit "github.com/alibaba/sentinel-golang/core/llm_token_ratelimit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmc/langchaingo/llms"
)

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

func initSentinel() {
	if err := sentinel.InitDefault(); err != nil {
		panic(err)
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
		panic(err)
	}
}

type MockLLM struct {
	response             *llms.ContentResponse
	err                  error
	callCount            int
	mu                   sync.Mutex
	lastPromptTokens     int
	lastCompletionTokens int
}

func (m *MockLLM) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	if m.response != nil {
		return m.response, nil
	}
	baseTokens := 100
	promptTokens := m.lastPromptTokens + rng.Intn(10000) + baseTokens
	completionTokens := m.lastCompletionTokens + rng.Intn(10000) + baseTokens
	totalTokens := promptTokens + completionTokens
	m.lastPromptTokens = promptTokens
	m.lastCompletionTokens = completionTokens
	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				Content: "Mock response",
				GenerationInfo: map[string]any{
					"PromptTokens":     promptTokens,
					"CompletionTokens": completionTokens,
					"TotalTokens":      totalTokens,
				},
			},
		},
	}, nil
}

func (m *MockLLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return "", nil
}

func TestLLMWrapper(t *testing.T) {
	initSentinel()

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if tt.name == "request_with_custom_resource_extract" {
				fmt.Println("Note: Check logs to verify custom resource extraction")
			}

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

func BenchmarkLLMWrapper_GenerateContent(b *testing.B) {
	initSentinel()

	if _, err := llmtokenratelimit.LoadRules([]*llmtokenratelimit.Rule{
		{
			Resource: ".*",
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
								Number:        10000000000000,
								CountStrategy: llmtokenratelimit.TotalTokens,
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
		// {
		// 	Resource: "test-resou.*",
		// 	Strategy: llmtokenratelimit.PETA,
		// 	SpecificItems: []*llmtokenratelimit.SpecificItem{
		// 		{
		// 			Identifier: llmtokenratelimit.Identifier{
		// 				Type:  llmtokenratelimit.Header,
		// 				Value: ".*",
		// 			},
		// 			KeyItems: []*llmtokenratelimit.KeyItem{
		// 				{
		// 					Key: ".*",
		// 					Token: llmtokenratelimit.Token{
		// 						Number:        10000000000000,
		// 						CountStrategy: llmtokenratelimit.TotalTokens,
		// 					},
		// 					Time: llmtokenratelimit.Time{
		// 						Unit:  llmtokenratelimit.Second,
		// 						Value: 5,
		// 					},
		// 				},
		// 			},
		// 		},
		// 	},
		// },
		// {
		// 	Resource: "test-resour.*",
		// 	Strategy: llmtokenratelimit.PETA,
		// 	SpecificItems: []*llmtokenratelimit.SpecificItem{
		// 		{
		// 			Identifier: llmtokenratelimit.Identifier{
		// 				Type:  llmtokenratelimit.Header,
		// 				Value: ".*",
		// 			},
		// 			KeyItems: []*llmtokenratelimit.KeyItem{
		// 				{
		// 					Key: ".*",
		// 					Token: llmtokenratelimit.Token{
		// 						Number:        10000000000000,
		// 						CountStrategy: llmtokenratelimit.TotalTokens,
		// 					},
		// 					Time: llmtokenratelimit.Time{
		// 						Unit:  llmtokenratelimit.Second,
		// 						Value: 5,
		// 					},
		// 				},
		// 			},
		// 		},
		// 	},
		// },
		// {
		// 	Resource: "test-resourc.*",
		// 	Strategy: llmtokenratelimit.PETA,
		// 	SpecificItems: []*llmtokenratelimit.SpecificItem{
		// 		{
		// 			Identifier: llmtokenratelimit.Identifier{
		// 				Type:  llmtokenratelimit.Header,
		// 				Value: ".*",
		// 			},
		// 			KeyItems: []*llmtokenratelimit.KeyItem{
		// 				{
		// 					Key: ".*",
		// 					Token: llmtokenratelimit.Token{
		// 						Number:        10000000000000,
		// 						CountStrategy: llmtokenratelimit.TotalTokens,
		// 					},
		// 					Time: llmtokenratelimit.Time{
		// 						Unit:  llmtokenratelimit.Second,
		// 						Value: 5,
		// 					},
		// 				},
		// 			},
		// 		},
		// 	},
		// },{
		// 	Resource: "test-resource.*",
		// 	Strategy: llmtokenratelimit.PETA,
		// 	SpecificItems: []*llmtokenratelimit.SpecificItem{
		// 		{
		// 			Identifier: llmtokenratelimit.Identifier{
		// 				Type:  llmtokenratelimit.Header,
		// 				Value: ".*",
		// 			},
		// 			KeyItems: []*llmtokenratelimit.KeyItem{
		// 				{
		// 					Key: ".*",
		// 					Token: llmtokenratelimit.Token{
		// 						Number:        10000000000000,
		// 						CountStrategy: llmtokenratelimit.TotalTokens,
		// 					},
		// 					Time: llmtokenratelimit.Time{
		// 						Unit:  llmtokenratelimit.Second,
		// 						Value: 5,
		// 					},
		// 				},
		// 			},
		// 		},
		// 	},
		// },
		// {
		// 	Resource: "test-resource-.*",
		// 	Strategy: llmtokenratelimit.PETA,
		// 	SpecificItems: []*llmtokenratelimit.SpecificItem{
		// 		{
		// 			Identifier: llmtokenratelimit.Identifier{
		// 				Type:  llmtokenratelimit.Header,
		// 				Value: ".*",
		// 			},
		// 			KeyItems: []*llmtokenratelimit.KeyItem{
		// 				{
		// 					Key: ".*",
		// 					Token: llmtokenratelimit.Token{
		// 						Number:        10000000000000,
		// 						CountStrategy: llmtokenratelimit.TotalTokens,
		// 					},
		// 					Time: llmtokenratelimit.Time{
		// 						Unit:  llmtokenratelimit.Second,
		// 						Value: 5,
		// 					},
		// 				},
		// 			},
		// 		},
		// 	},
		// },
		// {
		// 	Resource: "test-resource-a.*",
		// 	Strategy: llmtokenratelimit.PETA,
		// 	SpecificItems: []*llmtokenratelimit.SpecificItem{
		// 		{
		// 			Identifier: llmtokenratelimit.Identifier{
		// 				Type:  llmtokenratelimit.Header,
		// 				Value: ".*",
		// 			},
		// 			KeyItems: []*llmtokenratelimit.KeyItem{
		// 				{
		// 					Key: ".*",
		// 					Token: llmtokenratelimit.Token{
		// 						Number:        10000000000000,
		// 						CountStrategy: llmtokenratelimit.TotalTokens,
		// 					},
		// 					Time: llmtokenratelimit.Time{
		// 						Unit:  llmtokenratelimit.Second,
		// 						Value: 5,
		// 					},
		// 				},
		// 			},
		// 		},
		// 	},
		// },
		// {
		// 	Resource: "test-resource-al.*",
		// 	Strategy: llmtokenratelimit.PETA,
		// 	SpecificItems: []*llmtokenratelimit.SpecificItem{
		// 		{
		// 			Identifier: llmtokenratelimit.Identifier{
		// 				Type:  llmtokenratelimit.Header,
		// 				Value: ".*",
		// 			},
		// 			KeyItems: []*llmtokenratelimit.KeyItem{
		// 				{
		// 					Key: ".*",
		// 					Token: llmtokenratelimit.Token{
		// 						Number:        10000000000000,
		// 						CountStrategy: llmtokenratelimit.TotalTokens,
		// 					},
		// 					Time: llmtokenratelimit.Time{
		// 						Unit:  llmtokenratelimit.Second,
		// 						Value: 5,
		// 					},
		// 				},
		// 			},
		// 		},
		// 	},
		// },
		// {
		// 	Resource: "test-resource-all.*",
		// 	Strategy: llmtokenratelimit.PETA,
		// 	SpecificItems: []*llmtokenratelimit.SpecificItem{
		// 		{
		// 			Identifier: llmtokenratelimit.Identifier{
		// 				Type:  llmtokenratelimit.Header,
		// 				Value: ".*",
		// 			},
		// 			KeyItems: []*llmtokenratelimit.KeyItem{
		// 				{
		// 					Key: ".*",
		// 					Token: llmtokenratelimit.Token{
		// 						Number:        10000000000000,
		// 						CountStrategy: llmtokenratelimit.TotalTokens,
		// 					},
		// 					Time: llmtokenratelimit.Time{
		// 						Unit:  llmtokenratelimit.Second,
		// 						Value: 5,
		// 					},
		// 				},
		// 			},
		// 		},
		// 	},
		// },
		// {
		// 	Resource: "test-resource-allo.*",
		// 	Strategy: llmtokenratelimit.PETA,
		// 	SpecificItems: []*llmtokenratelimit.SpecificItem{
		// 		{
		// 			Identifier: llmtokenratelimit.Identifier{
		// 				Type:  llmtokenratelimit.Header,
		// 				Value: ".*",
		// 			},
		// 			KeyItems: []*llmtokenratelimit.KeyItem{
		// 				{
		// 					Key: ".*",
		// 					Token: llmtokenratelimit.Token{
		// 						Number:        10000000000000,
		// 						CountStrategy: llmtokenratelimit.TotalTokens,
		// 					},
		// 					Time: llmtokenratelimit.Time{
		// 						Unit:  llmtokenratelimit.Second,
		// 						Value: 5,
		// 					},
		// 				},
		// 			},
		// 		},
		// 	},
		// },
	}); err != nil {
		panic(err)
	}

	mockLLM := &MockLLM{}

	llmWrapper := NewLLMWrapper(mockLLM, WithDefaultResource("test-resource-allow"))
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageType(llms.ChatMessageTypeHuman),
			"This is a benchmark test message for measuring QPS performance"),
	}

	var blockCount int64

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := llmWrapper.GenerateContent(context.Background(), messages)
			if err != nil {
				atomic.AddInt64(&blockCount, 1)
			}
		}
	})

	totalRequests := int64(b.N)
	successCount := totalRequests - atomic.LoadInt64(&blockCount)
	blockRate := float64(atomic.LoadInt64(&blockCount)) / float64(totalRequests) * 100

	b.ReportMetric(float64(atomic.LoadInt64(&blockCount)), "errors")
	b.ReportMetric(blockRate, "block_rate")
	b.ReportMetric(float64(successCount), "success")

	b.Logf("Total Requests: %d, Success: %d, Block: %d, Block Rate: %.2f%%",
		totalRequests, successCount, atomic.LoadInt64(&blockCount), blockRate)
}
