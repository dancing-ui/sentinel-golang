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

import "reflect"

type UsedTokenInfos struct {
	InputTokens  int64 `json:"inputTokens"`
	OutputTokens int64 `json:"outputTokens"`
	TotalTokens  int64 `json:"totalTokens"`
}

var (
	usedTokenInfosType = reflect.TypeOf((*UsedTokenInfos)(nil))
)

type UsedTokenInfo func(*UsedTokenInfos)

func WithInputTokens(inputTokens int64) UsedTokenInfo {
	return func(infos *UsedTokenInfos) {
		infos.InputTokens = inputTokens
	}
}

func WithOutputTokens(outputTokens int64) UsedTokenInfo {
	return func(infos *UsedTokenInfos) {
		infos.OutputTokens = outputTokens
	}
}

func WithTotalTokens(totalTokens int64) UsedTokenInfo {
	return func(infos *UsedTokenInfos) {
		infos.TotalTokens = totalTokens
	}
}

func GenerateUsedTokenInfos(uti ...UsedTokenInfo) *UsedTokenInfos {
	infos := new(UsedTokenInfos)
	for _, info := range uti {
		info(infos)
	}
	return infos
}

func extractUsedTokenInfos(ctx *Context) *UsedTokenInfos {
	usedTokenInfosRaw := ctx.GetContext(KeyUsedTokenInfos)
	if usedTokenInfosRaw == nil {
		return nil
	}

	usedTokenInfos, ok := usedTokenInfosRaw.(*UsedTokenInfos)
	if !ok {
		return nil
	}

	return usedTokenInfos
}
