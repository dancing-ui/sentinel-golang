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

type RequestInfos struct {
	Headers map[string]string `json:"headers"`
}

var (
	requestInfosType = reflect.TypeOf((*RequestInfos)(nil))
)

type RequestInfo func(*RequestInfos)

func WithHeader(headers map[string]string) RequestInfo {
	return func(infos *RequestInfos) {
		infos.Headers = headers
	}
}

func GenerateRequestInfos(ri ...RequestInfo) *RequestInfos {
	infos := new(RequestInfos)
	for _, info := range ri {
		info(infos)
	}
	return infos
}

func extractRequestInfos(ctx *Context) *RequestInfos {
	reqInfosRaw := ctx.GetContext(KeyRequestInfos)
	if reqInfosRaw == nil {
		return nil
	}

	reqInfos, ok := reqInfosRaw.(*RequestInfos)
	if !ok {
		return nil
	}

	return reqInfos
}
