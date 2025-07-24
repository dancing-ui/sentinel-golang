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

import "github.com/alibaba/sentinel-golang/util"

type AllIdentifierChecker struct{}

func (f *AllIdentifierChecker) Check(infos *RequestInfos, identifier Identifier, pattern string) bool {
	for identifierType, checker := range ruleMatcher.IdentifierCheckers {
		if identifierType == AllIdentifier {
			continue
		}
		if checker.Check(infos, identifier, pattern) {
			return true
		}
	}
	return false
}

type HeaderChecker struct{}

func (f *HeaderChecker) Check(infos *RequestInfos, identifier Identifier, pattern string) bool {
	if infos == nil || infos.Headers == nil {
		return false
	}
	for key, value := range infos.Headers {
		if util.RegexMatch(identifier.Value, key) && util.RegexMatch(pattern, value) {
			return true
		}
	}
	return false
}
