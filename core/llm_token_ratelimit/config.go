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
	"strings"
	"sync"

	"github.com/alibaba/sentinel-golang/logging"
)

type TokenEncoderProvider uint32

const (
	OpenAIEncoderProvider TokenEncoderProvider = iota
)

type IdentifierType uint32

const (
	AllIdentifier IdentifierType = iota
	Header
)

type CountStrategy uint32

const (
	TotalTokens CountStrategy = iota
	InputTokens
	OutputTokens
)

type TimeUnit uint32

const (
	Second TimeUnit = iota
	Minute
	Hour
	Day
)

type Strategy uint32

const (
	FixedWindow Strategy = iota
	PETA
)

type TokenEncoding struct {
	Provider TokenEncoderProvider `json:"provider" yaml:"provider"`
	Model    string               `json:"model" yaml:"model"`
}

type Identifier struct {
	Type  IdentifierType `json:"type" yaml:"type"`
	Value string         `json:"value" yaml:"value"`
}

type Token struct {
	Number        int64         `json:"number" yaml:"number"`
	CountStrategy CountStrategy `json:"countStrategy" yaml:"countStrategy"`
}

type Time struct {
	Unit  TimeUnit `json:"unit" yaml:"unit"`
	Value int64    `json:"value" yaml:"value"`
}

type KeyItem struct {
	Key   string `json:"key" yaml:"key"`
	Token Token  `json:"token" yaml:"token"`
	Time  Time   `json:"time" yaml:"time"`
}

type RuleItem struct {
	Identifier Identifier `json:"identifier" yaml:"identifier"`
	KeyItems   []*KeyItem `json:"keyItems" yaml:"keyItems"`
}

type Redis struct {
	ServiceName string `json:"serviceName" yaml:"serviceName"`
	ServicePort int32  `json:"servicePort" yaml:"servicePort"`
	Username    string `json:"username" yaml:"username"`
	Password    string `json:"password" yaml:"password"`

	Timeout      int32 `json:"timeout" yaml:"timeout"`
	PoolSize     int32 `json:"poolSize" yaml:"poolSize"`
	MinIdleConns int32 `json:"minIdleConns" yaml:"minIdleConns"`
	MaxRetries   int32 `json:"maxRetries" yaml:"maxRetries"`
}

type Config struct {
	Rules        []*Rule `json:"rules" yaml:"rules"`
	Redis        *Redis  `json:"redis" yaml:"redis"`
	ErrorCode    int32   `json:"errorCode" yaml:"errorCode"`
	ErrorMessage string  `json:"errorMessage" yaml:"errorMessage"`
}

type SafeConfig struct {
	mu     sync.RWMutex
	config *Config
}

var globalConfig = NewGlobalConfig()

func NewGlobalConfig() *SafeConfig {
	return &SafeConfig{}
}

func (c *SafeConfig) SetConfig(newConfig *Config) error {
	if c == nil {
		return fmt.Errorf("safe config is nil")
	}
	if newConfig == nil {
		return fmt.Errorf("new config cannot be nil")
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

func GetErrorCode() int32 {
	cfg := globalConfig.GetConfig()
	if cfg == nil {
		logging.Error(errors.New("safe config is nil"), "found safe config is nil")
		return -1
	}
	return cfg.ErrorCode
}

func GetErrorMsg() string {
	cfg := globalConfig.GetConfig()
	if cfg == nil {
		logging.Error(errors.New("safe config is nil"), "found safe config is nil")
		return ""
	}
	return cfg.ErrorMessage
}

func (p *TokenEncoderProvider) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if p == nil {
		return fmt.Errorf("token encoder provider is nil")
	}
	var str string
	if err := unmarshal(&str); err != nil {
		return err
	}
	switch str {
	case "openai":
		*p = OpenAIEncoderProvider
	default:
		return fmt.Errorf("unknown token encoder provider: %s", str)
	}
	return nil
}

func (it *IdentifierType) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if it == nil {
		return fmt.Errorf("identifier type is nil")
	}
	var str string
	if err := unmarshal(&str); err != nil {
		return err
	}
	switch str {
	case "all":
		*it = AllIdentifier
	case "header":
		*it = Header
	default:
		return fmt.Errorf("unknown identifier type: %s", str)
	}
	return nil
}

func (ct *CountStrategy) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if ct == nil {
		return fmt.Errorf("count strategy is nil")
	}
	var str string
	if err := unmarshal(&str); err != nil {
		return err
	}
	switch str {
	case "total-tokens":
		*ct = TotalTokens
	case "input-tokens":
		*ct = InputTokens
	case "output-tokens":
		*ct = OutputTokens
	default:
		return fmt.Errorf("unknown count strategy: %s", str)
	}
	return nil
}

func (tu *TimeUnit) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if tu == nil {
		return fmt.Errorf("time unit is nil")
	}
	var str string
	if err := unmarshal(&str); err != nil {
		return err
	}
	switch str {
	case "second":
		*tu = Second
	case "minute":
		*tu = Minute
	case "hour":
		*tu = Hour
	case "day":
		*tu = Day
	default:
		return fmt.Errorf("unknown time unit: %s", str)
	}
	return nil
}

func (s *Strategy) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if s == nil {
		return fmt.Errorf("strategy is nil")
	}
	var str string
	if err := unmarshal(&str); err != nil {
		return err
	}
	switch str {
	case "fixed-window":
		*s = FixedWindow
	case "peta":
		*s = PETA
	default:
		return fmt.Errorf("unknown strategy: %s", str)
	}
	return nil
}

func (e *TokenEncoding) String() string {
	if e == nil {
		return "TokenEncoding{nil}"
	}
	return fmt.Sprintf("TokenEncoding{Provider:%s, Model:%s}", e.Provider.String(), e.Model)
}

func (it IdentifierType) String() string {
	switch it {
	case AllIdentifier:
		return "all"
	case Header:
		return "header"
	default:
		return "undefined"
	}
}

func (ct CountStrategy) String() string {
	switch ct {
	case TotalTokens:
		return "total-tokens"
	case InputTokens:
		return "input-tokens"
	case OutputTokens:
		return "output-tokens"
	default:
		return "undefined"
	}
}

func (tu TimeUnit) String() string {
	switch tu {
	case Second:
		return "second"
	case Minute:
		return "minute"
	case Hour:
		return "hour"
	case Day:
		return "day"
	default:
		return "undefined"
	}
}

func (s Strategy) String() string {
	switch s {
	case FixedWindow:
		return "fixed-window"
	case PETA:
		return "peta"
	default:
		return "undefined"
	}
}

func (p TokenEncoderProvider) String() string {
	switch p {
	case OpenAIEncoderProvider:
		return "openai"
	default:
		return "undefined"
	}
}

func (ri *RuleItem) String() string {
	if ri == nil {
		return "RuleItem{nil}"
	}

	var sb strings.Builder
	sb.WriteString("RuleItem{")
	sb.WriteString(fmt.Sprintf("Identifier:%s", ri.Identifier.String()))

	if len(ri.KeyItems) > 0 {
		sb.WriteString(", KeyItems:[")
		for i, item := range ri.KeyItems {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(item.String())
		}
		sb.WriteString("]")
	} else {
		sb.WriteString(", KeyItems:[]")
	}

	sb.WriteString("}")
	return sb.String()
}

func (id *Identifier) String() string {
	if id == nil {
		return "Identifier{nil}"
	}
	return fmt.Sprintf("Identifier{Type:%s, Value:%s}", id.Type.String(), id.Value)
}

func (ki *KeyItem) String() string {
	if ki == nil {
		return "KeyItem{nil}"
	}
	return fmt.Sprintf("KeyItem{Key:%s, Token:%s, Time:%s}",
		ki.Key, ki.Token.String(), ki.Time.String())
}

func (t *Token) String() string {
	if t == nil {
		return "Token{nil}"
	}
	return fmt.Sprintf("Token{Number:%d, CountStrategy:%s}",
		t.Number, t.CountStrategy.String())
}

func (t *Time) String() string {
	if t == nil {
		return "Time{nil}"
	}
	return fmt.Sprintf("Time{Value:%d second}", t.convertToSeconds())
}

func (t *Time) convertToSeconds() int64 {
	switch t.Unit {
	case Second:
		return t.Value
	case Minute:
		return t.Value * 60
	case Hour:
		return t.Value * 3600
	case Day:
		return t.Value * 86400
	default:
		return ErrorTimeDuration
	}
}
