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

import "fmt"

// ================================= OpenAITokenExtractor ==============================

func OpenAITokenExtractor(response interface{}) (*UsedTokenInfos, error) {
	if response == nil {
		return nil, fmt.Errorf("response is nil")
	}

	resp, ok := response.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("response is not map[string]any")
	}

	inputTokens, ok := resp["PromptTokens"].(int)
	if !ok {
		return nil, fmt.Errorf("PromptTokens not found or not int")
	}
	outputTokens, ok := resp["CompletionTokens"].(int)
	if !ok {
		return nil, fmt.Errorf("CompletionTokens not found or not int")
	}
	totalTokens, ok := resp["TotalTokens"].(int)
	if !ok {
		return nil, fmt.Errorf("TotalTokens not found or not int")
	}

	return GenerateUsedTokenInfos(
		WithInputTokens(inputTokens),
		WithOutputTokens(outputTokens),
		WithTotalTokens(totalTokens),
	), nil
}
