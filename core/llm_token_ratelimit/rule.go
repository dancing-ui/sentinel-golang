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
	"fmt"
	"strings"
)

type Rule struct {
	ID string `json:"id,omitempty" yaml:"id,omitempty"`

	Resource  string        `json:"resource" yaml:"resource"`
	Strategy  Strategy      `json:"strategy" yaml:"strategy"`
	RuleName  string        `json:"ruleName" yaml:"ruleName"`
	Encoding  TokenEncoding `json:"encoding" yaml:"encoding"`
	RuleItems []*RuleItem   `json:"ruleItems" yaml:"ruleItems"`
}

func (r *Rule) ResourceName() string {
	return r.Resource
}

// TODO: update rule string and tests
func (r *Rule) String() string {
	if r == nil {
		return "Rule{nil}"
	}

	var sb strings.Builder
	sb.WriteString("Rule{")

	if r.ID != "" {
		sb.WriteString(fmt.Sprintf("ID:%s, ", r.ID))
	}

	sb.WriteString(fmt.Sprintf("Resource:%s, ", r.Resource))
	sb.WriteString(fmt.Sprintf("Strategy:%s, ", r.Strategy.String()))
	sb.WriteString(fmt.Sprintf("RuleName:%s, ", r.RuleName))
	sb.WriteString(fmt.Sprintf("Encoding:%s", r.Encoding.String()))

	if len(r.RuleItems) > 0 {
		sb.WriteString(", RuleItems:[")
		for i, item := range r.RuleItems {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(item.String())
		}
		sb.WriteString("]")
	} else {
		sb.WriteString(", RuleItems:[]")
	}

	sb.WriteString("}")

	return sb.String()
}

func (r *Rule) setDefaultRuleOption() {
	if len(r.Resource) == 0 {
		r.Resource = DefaultResourcePattern
	}

	if len(r.Encoding.Model) == 0 {
		r.Encoding.Model = DefaultTokenEncodingModel[r.Encoding.Provider]
	}

	for idx1, ruleItem := range r.RuleItems {
		if len(ruleItem.Identifier.Value) == 0 {
			r.RuleItems[idx1].Identifier.Value = DefaultIdentifierValuePattern
		}
		for idx2, keyItem := range ruleItem.KeyItems {
			if len(keyItem.Key) == 0 {
				r.RuleItems[idx1].KeyItems[idx2].Key = DefaultKeyPattern
			}
			if len(r.RuleName) == 0 &&
				r.Resource == DefaultResourcePattern &&
				r.RuleItems[idx1].Identifier.Value == DefaultIdentifierValuePattern &&
				r.RuleItems[idx1].KeyItems[idx2].Key == DefaultKeyPattern {
				r.RuleName = DefaultRuleName
			}
		}
	}
}

func (r *Rule) filterDuplicatedItem() {
	occuredKeyItem := make(map[string]struct{})
	var ruleItems []*RuleItem
	for idx1 := len(r.RuleItems) - 1; idx1 >= 0; idx1-- {
		var keyItems []*KeyItem
		for idx2 := len(r.RuleItems[idx1].KeyItems) - 1; idx2 >= 0; idx2-- {
			hash := generateHash(
				r.RuleItems[idx1].Identifier.String(),
				r.RuleItems[idx1].KeyItems[idx2].Key,
				r.RuleItems[idx1].KeyItems[idx2].Token.CountStrategy.String(),
				r.RuleItems[idx1].KeyItems[idx2].Time.String(),
			)
			if _, exists := occuredKeyItem[hash]; exists {
				continue
			}
			occuredKeyItem[hash] = struct{}{}
			keyItems = append(keyItems, r.RuleItems[idx1].KeyItems[idx2])
		}
		if len(keyItems) == 0 {
			continue
		}
		ruleItems = append(ruleItems, &RuleItem{
			Identifier: r.RuleItems[idx1].Identifier,
			KeyItems:   keyItems,
		})
	}
	r.RuleItems = ruleItems
}
