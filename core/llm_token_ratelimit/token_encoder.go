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
	"strings"

	"github.com/alibaba/sentinel-golang/logging"
	"github.com/go-redis/redis/v7"
	"github.com/pkoukk/tiktoken-go"
)

// ================================= TokenEncoder ====================================

//go:embed script/token_encoder/update.lua
var globalTokenEncoderUpdateScript string

type TokenEncoder interface {
	CountTokens(prompts []string, rule *MatchedRule) (int, error)
}

func GetTokenEncoder(encoding TokenEncoding) TokenEncoder {
	// TODO: cache the encoder for each model to avoid re-initialization
	switch encoding.Provider {
	case OpenAIEncoderProvider:
		return NewOpenAIEncoder(encoding)
	default:
		logging.Warn("unsupported token encoder provider: %s, falling back to OpenAIEncoder", encoding.Provider)
		return NewOpenAIEncoder(encoding) // Fallback to OpenAIEncoder for unsupported providers
	}
}

// ================================= OpenAIEncoder ====================================

type OpenAIEncoder struct {
	Model   string
	Encoder *tiktoken.Tiktoken
}

func NewOpenAIEncoder(encoding TokenEncoding) *OpenAIEncoder {
	encoder, err := tiktoken.EncodingForModel(encoding.Model)
	actualModel := encoding.Model

	if err != nil {
		actualModel = DefaultTokenEncodingModel[OpenAIEncoderProvider]
		logging.Warn("openai's model %s not supported, falling back to default model: %s", encoding.Model, actualModel)
		encoder, _ = tiktoken.EncodingForModel(actualModel)
	}

	return &OpenAIEncoder{
		Model:   actualModel,
		Encoder: encoder,
	}
}

func (e *OpenAIEncoder) CountTokens(prompts []string, rule *MatchedRule) (int, error) {
	if e == nil {
		return 0, fmt.Errorf("OpenAIEncoder is nil")
	}
	if e.Encoder == nil {
		return 0, fmt.Errorf("OpenAIEncoder's encoder is nil for model: %s", e.Model)
	}
	if len(prompts) == 0 {
		return 0, nil // No prompts to count tokens
	}
	// Concatenate prompts
	var builder strings.Builder
	for _, prompt := range prompts {
		builder.WriteString(prompt)
	}
	token := e.Encoder.Encode(builder.String(), nil, nil)
	if len(token) > 0 {
		difference, err := e.queryDifference(rule)
		if err != nil {
			return 0, err
		}
		return len(token) + difference, nil
	}
	return 0, nil
}

func (e *OpenAIEncoder) queryDifference(rule *MatchedRule) (int, error) {
	if e == nil {
		return 0, fmt.Errorf("OpenAIEncoder is nil")
	}
	key := fmt.Sprintf(TokenEncoderKeyFormat, rule.LimitKey, OpenAIEncoderProvider.String(), e.Model)
	response, err := globalRedisClient.Get(key)
	if err != nil {
		return 0, err
	}
	value, err := response.Int()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, err
	}
	return value, nil
}
