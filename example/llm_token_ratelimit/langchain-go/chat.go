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

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type LLMRequestInfos struct {
	Messages []LLMMessage `json:"messages"`
	Model    string       `json:"model"`
}

type LLMClient struct {
	llm *openai.LLM
}

func NewLLMClient(model string) (*LLMClient, error) {
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("LLM_API_KEY environment variable is not set")
	}

	baseURL := os.Getenv("LLM_BASE_URL")
	if baseURL == "" {
		return nil, fmt.Errorf("LLM_BASE_URL environment variable is not set")
	}

	llm, err := openai.New(
		openai.WithToken(apiKey),
		openai.WithBaseURL(baseURL),
		openai.WithModel(model),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	return &LLMClient{
		llm: llm,
	}, nil
}

func (c *LLMClient) Request(infos *LLMRequestInfos) (*llms.ContentResponse, error) {
	if infos == nil || infos.Messages == nil {
		return nil, fmt.Errorf("invalid request infos")
	}
	ctx := context.Background()
	content := make([]llms.MessageContent, len(infos.Messages))
	for i, msg := range infos.Messages {
		content[i] = llms.TextParts(llms.ChatMessageType(msg.Role), msg.Content)
	}
	completion, err := c.llm.GenerateContent(ctx, content)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}
	return completion, nil
}
