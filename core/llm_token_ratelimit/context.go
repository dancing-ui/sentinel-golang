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
	"reflect"
	"sync"

	"github.com/alibaba/sentinel-golang/core/base"
)

const (
	KeyContext        = "llmTokenRatelimitContext"
	KeyRequestInfos   = "llmTokenRatelimitReqInfos"
	KeyUsedTokenInfos = "llmTokenRatelimitUsedTokenInfos"
	KeyMatchedRules   = "llmTokenRatelimitMatchedRules"
)

var (
	contextType = reflect.TypeOf((*Context)(nil))
)

type Context struct {
	mu          sync.RWMutex
	userContext map[string]interface{}
}

func NewContext() *Context {
	return &Context{
		userContext: make(map[string]interface{}),
	}
}

func (ctx *Context) SetContext(key string, value interface{}) {
	if ctx == nil {
		return
	}

	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	if ctx.userContext == nil {
		ctx.userContext = make(map[string]interface{})
	}
	ctx.userContext[key] = value
}

func (ctx *Context) GetContext(key string) interface{} {
	if ctx == nil {
		return nil
	}

	ctx.mu.RLock()
	if ctx.userContext != nil {
		value := ctx.userContext[key]
		ctx.mu.RUnlock()
		return value
	}
	ctx.mu.RUnlock()

	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	if ctx.userContext == nil {
		ctx.userContext = make(map[string]interface{})
	}
	return ctx.userContext[key]
}

func extractContextFromArgs(ctx *base.EntryContext) *Context {
	if ctx == nil {
		return nil
	}
	for _, arg := range ctx.Input.Args {
		if llmCtx, ok := arg.(*Context); ok {
			return llmCtx
		}
	}
	return nil
}

func extractContextFromData(ctx *base.EntryContext) *Context {
	if ctx == nil {
		return nil
	}
	for key, value := range ctx.Data {
		if key != KeyContext {
			continue
		}
		if llmCtx, ok := value.(*Context); ok {
			return llmCtx
		}
	}
	return nil
}
