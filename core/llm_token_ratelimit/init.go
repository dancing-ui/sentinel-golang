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
	"fmt"
	"sync"

	"github.com/alibaba/sentinel-golang/logging"
)

type SafeConfig struct {
	mu     sync.RWMutex
	config *Config
}

var globalConfig = &SafeConfig{}

func (c *SafeConfig) SetConfig(newConfig *Config) error {
	if c == nil {
		return fmt.Errorf("safe config is nil")
	}
	if newConfig == nil {
		return fmt.Errorf("config cannot be nil")
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = newConfig
	return nil
}

func (c *SafeConfig) GetConfig() *Config {
	if c == nil {
		logging.Error(errors.New("safe config is nil"), "found safe config is nil")
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

func initRules() error {
	cfg := globalConfig.GetConfig()
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	if len(cfg.Rules) == 0 {
		return nil
	}

	if _, err := LoadRules(cfg.Rules); err != nil {
		return err
	}

	return nil
}

func Init(cfg *Config) error {
	if err := globalConfig.SetConfig(cfg); err != nil {
		return err
	}
	if err := globalRedisClient.Init(cfg.Redis); err != nil {
		return err
	}
	if err := initRules(); err != nil {
		return err
	}
	return nil
}
