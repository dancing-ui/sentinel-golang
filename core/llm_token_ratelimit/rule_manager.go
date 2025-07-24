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
	"errors"
	"reflect"
	"sync"

	"github.com/alibaba/sentinel-golang/logging"
	"github.com/alibaba/sentinel-golang/util"
)

var (
	ruleMap       = make(map[string][]*Rule)
	rwMux         = &sync.RWMutex{}
	currentRules  = make(map[string][]*Rule, 0)
	updateRuleMux = new(sync.Mutex)
)

func LoadRules(rules []*Rule) (bool, error) {
	resRulesMap := make(map[string][]*Rule, 16)
	for _, rule := range rules {
		rule.setDefaultRuleOption()

		resRules, exist := resRulesMap[rule.Resource]
		if !exist {
			resRules = make([]*Rule, 0, 1)
		}
		resRulesMap[rule.Resource] = append(resRules, rule)
	}

	updateRuleMux.Lock()
	defer updateRuleMux.Unlock()
	isEqual := reflect.DeepEqual(currentRules, resRulesMap)
	if isEqual {
		logging.Info("[LLMTokenRateLimit] Load rules is the same with current rules, so ignore load operation.")
		return false, nil
	}
	err := onRuleUpdate(resRulesMap)
	return true, err
}

func onRuleUpdate(rawResRulesMap map[string][]*Rule) (err error) {
	validResRulesMap := make(map[string][]*Rule, len(rawResRulesMap))
	for res, rules := range rawResRulesMap {
		validResRules := make([]*Rule, 0, len(rules))
		for _, rule := range rules {
			if err := IsValidRule(rule); err != nil {
				logging.Warn("[LLMTokenRateLimit onRuleUpdate] Ignoring invalid llm_token_ratelimit rule", "rule", rule, "reason", err.Error())
				continue
			}
			validResRules = append(validResRules, rule)
		}
		if len(validResRules) > 0 {
			validResRulesMap[res] = validResRules
		}
	}

	start := util.CurrentTimeNano()
	rwMux.Lock()
	ruleMap = validResRulesMap
	rwMux.Unlock()
	currentRules = rawResRulesMap

	logging.Debug("[LLMTokenRateLimit onRuleUpdate] Time statistic(ns) for updating llm_token_ratelimit rule", "timeCost", util.CurrentTimeNano()-start)
	logRuleUpdate(validResRulesMap)
	return nil
}

func LoadRulesOfResource(res string, rules []*Rule) (bool, error) {
	if len(res) == 0 {
		return false, errors.New("empty resource")
	}
	updateRuleMux.Lock()
	defer updateRuleMux.Unlock()
	if len(rules) == 0 {
		delete(currentRules, res)
		rwMux.Lock()
		delete(ruleMap, res)
		rwMux.Unlock()
		logging.Info("[LLMTokenRateLimit] clear resource level rules", "resource", res)
		return true, nil
	}

	isEqual := reflect.DeepEqual(currentRules[res], rules)
	if isEqual {
		logging.Info("[LLMTokenRateLimit] Load resource level rules is the same with current resource level rules, so ignore load operation.")
		return false, nil
	}

	err := onResourceRuleUpdate(res, rules)
	return true, err
}

func onResourceRuleUpdate(res string, rawResRules []*Rule) (err error) {
	validResRules := make([]*Rule, 0, len(rawResRules))
	for _, rule := range rawResRules {
		if err := IsValidRule(rule); err != nil {
			logging.Warn("[LLMTokenRateLimit onResourceRuleUpdate] Ignoring invalid llm_token_ratelimit rule", "rule", rule, "reason", err.Error())
			continue
		}
		validResRules = append(validResRules, rule)
	}

	start := util.CurrentTimeNano()
	rwMux.Lock()
	if len(validResRules) == 0 {
		delete(ruleMap, res)
	} else {
		ruleMap[res] = validResRules
	}
	rwMux.Unlock()
	currentRules[res] = rawResRules
	logging.Debug("[LLMTokenRateLimit onResourceRuleUpdate] Time statistic(ns) for updating llm_token_ratelimit rule", "timeCost", util.CurrentTimeNano()-start)
	logging.Info("[LLMTokenRateLimit] load resource level rules", "resource", res, "validResRules", validResRules)
	return nil
}

func ClearRules() error {
	_, err := LoadRules(nil)
	return err
}

func ClearRulesOfResource(res string) error {
	_, err := LoadRulesOfResource(res, nil)
	return err
}

func GetRules() []Rule {
	rules := getRules()
	ret := make([]Rule, 0, len(rules))
	for _, rule := range rules {
		ret = append(ret, *rule)
	}
	return ret
}

func GetRulesOfResource(res string) []Rule {
	rules := getRulesOfResource(res)
	ret := make([]Rule, 0, len(rules))
	for _, rule := range rules {
		ret = append(ret, *rule)
	}
	return ret
}

func getRules() []*Rule {
	rwMux.RLock()
	defer rwMux.RUnlock()

	return rulesFrom(ruleMap)
}

func getRulesOfResource(res string) []*Rule {
	rwMux.RLock()
	defer rwMux.RUnlock()

	ret := make([]*Rule, 0)
	for resource, rules := range ruleMap {
		if util.RegexMatch(resource, res) {
			for _, rule := range rules {
				if rule != nil {
					ret = append(ret, rule)
				}
			}
		}
	}
	return ret
}

func rulesFrom(m map[string][]*Rule) []*Rule {
	rules := make([]*Rule, 0, 8)
	if len(m) == 0 {
		return rules
	}
	for _, rs := range m {
		for _, r := range rs {
			if r != nil {
				rules = append(rules, r)
			}
		}
	}
	return rules
}

func logRuleUpdate(m map[string][]*Rule) {
	rs := rulesFrom(m)
	if len(rs) == 0 {
		logging.Info("[LLMTokenRateLimitRuleManager] LLMTokenRateLimit rules were cleared")
	} else {
		logging.Info("[LLMTokenRateLimitRuleManager] LLMTokenRateLimit rules were loaded", "rules", rs)
	}
}
