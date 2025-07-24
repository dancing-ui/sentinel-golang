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
	"strconv"

	"github.com/spaolacci/murmur3"
)

func generateHash(parts ...string) string {
	h := murmur3.New64()
	for i, part := range parts {
		if i > 0 {
			h.Write([]byte{0}) // separator
		}
		h.Write([]byte(part))
	}
	return strconv.FormatUint(h.Sum64(), 16)
}
