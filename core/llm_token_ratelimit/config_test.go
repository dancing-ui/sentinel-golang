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
	"testing"

	"gopkg.in/yaml.v3"
)

func TestIdentifierType_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		yamlData string
		expected IdentifierType
		wantErr  bool
	}{
		{"all identifier", `type: all`, AllIdentifier, false},
		{"header identifier", `type: header`, Header, false},
		{"unknown identifier", `type: unknown`, AllIdentifier, true},
		{"empty identifier", `type: ""`, AllIdentifier, true},
		{"number as identifier", `type: 123`, AllIdentifier, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data struct {
				Type IdentifierType `yaml:"type"`
			}

			err := yaml.Unmarshal([]byte(tt.yamlData), &data)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if data.Type != tt.expected {
					t.Errorf("Expected %v, got %v", tt.expected, data.Type)
				}
			}
		})
	}
}

func TestCountStrategy_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		yamlData string
		expected CountStrategy
		wantErr  bool
	}{
		{"total tokens", `strategy: total-tokens`, TotalTokens, false},
		{"input tokens", `strategy: input-tokens`, InputTokens, false},
		{"output tokens", `strategy: output-tokens`, OutputTokens, false},
		{"unknown strategy", `strategy: unknown-tokens`, TotalTokens, true},
		{"empty strategy", `strategy: ""`, TotalTokens, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data struct {
				Strategy CountStrategy `yaml:"strategy"`
			}

			err := yaml.Unmarshal([]byte(tt.yamlData), &data)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if data.Strategy != tt.expected {
					t.Errorf("Expected %v, got %v", tt.expected, data.Strategy)
				}
			}
		})
	}
}

func TestTimeUnit_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		yamlData string
		expected TimeUnit
		wantErr  bool
	}{
		{"second unit", `unit: second`, Second, false},
		{"minute unit", `unit: minute`, Minute, false},
		{"hour unit", `unit: hour`, Hour, false},
		{"day unit", `unit: day`, Day, false},
		{"unknown unit", `unit: week`, Second, true},
		{"empty unit", `unit: ""`, Second, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data struct {
				Unit TimeUnit `yaml:"unit"`
			}

			err := yaml.Unmarshal([]byte(tt.yamlData), &data)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if data.Unit != tt.expected {
					t.Errorf("Expected %v, got %v", tt.expected, data.Unit)
				}
			}
		})
	}
}

func TestStrategy_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		yamlData string
		expected Strategy
		wantErr  bool
	}{
		{"fixed window", `strategy: fixed-window`, FixedWindow, false},
		{"unknown strategy", `strategy: sliding-window`, FixedWindow, true},
		{"empty strategy", `strategy: ""`, FixedWindow, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data struct {
				Strategy Strategy `yaml:"strategy"`
			}

			err := yaml.Unmarshal([]byte(tt.yamlData), &data)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if data.Strategy != tt.expected {
					t.Errorf("Expected %v, got %v", tt.expected, data.Strategy)
				}
			}
		})
	}
}

func TestIdentifierType_String(t *testing.T) {
	tests := []struct {
		name     string
		it       IdentifierType
		expected string
	}{
		{"all identifier", AllIdentifier, "all"},
		{"header identifier", Header, "header"},
		{"undefined identifier", IdentifierType(999), "undefined"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.it.String()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestCountStrategy_String(t *testing.T) {
	tests := []struct {
		name     string
		cs       CountStrategy
		expected string
	}{
		{"total tokens", TotalTokens, "total-tokens"},
		{"input tokens", InputTokens, "input-tokens"},
		{"output tokens", OutputTokens, "output-tokens"},
		{"undefined strategy", CountStrategy(999), "undefined"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cs.String()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestTimeUnit_String(t *testing.T) {
	tests := []struct {
		name     string
		tu       TimeUnit
		expected string
	}{
		{"second", Second, "second"},
		{"minute", Minute, "minute"},
		{"hour", Hour, "hour"},
		{"day", Day, "day"},
		{"undefined unit", TimeUnit(999), "undefined"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.tu.String()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestStrategy_String(t *testing.T) {
	tests := []struct {
		name     string
		s        Strategy
		expected string
	}{
		{"fixed window", FixedWindow, "fixed-window"},
		{"undefined strategy", Strategy(999), "undefined"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.s.String()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestIdentifier_String(t *testing.T) {
	tests := []struct {
		name     string
		id       *Identifier
		expected string
	}{
		{"nil identifier", nil, "Identifier{nil}"},
		{"all identifier", &Identifier{Type: AllIdentifier, Value: ".*"}, "Identifier{Type:all, Value:.*}"},
		{"header identifier", &Identifier{Type: Header, Value: "user-id"}, "Identifier{Type:header, Value:user-id}"},
		{"empty value", &Identifier{Type: Header, Value: ""}, "Identifier{Type:header, Value:}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.id.String()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestToken_String(t *testing.T) {
	tests := []struct {
		name     string
		token    *Token
		expected string
	}{
		{"nil token", nil, "Token{nil}"},
		{"total tokens", &Token{Number: 1000, CountStrategy: TotalTokens}, "Token{Number:1000, CountStrategy:total-tokens}"},
		{"input tokens", &Token{Number: 500, CountStrategy: InputTokens}, "Token{Number:500, CountStrategy:input-tokens}"},
		{"output tokens", &Token{Number: 300, CountStrategy: OutputTokens}, "Token{Number:300, CountStrategy:output-tokens}"},
		{"zero tokens", &Token{Number: 0, CountStrategy: TotalTokens}, "Token{Number:0, CountStrategy:total-tokens}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.token.String()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestTime_String(t *testing.T) {
	tests := []struct {
		name     string
		time     *Time
		expected string
	}{
		{"nil time", nil, "Time{nil}"},
		{"second", &Time{Value: 30, Unit: Second}, "Time{Value:30 second}"},
		{"minute", &Time{Value: 5, Unit: Minute}, "Time{Value:300 second}"},
		{"hour", &Time{Value: 2, Unit: Hour}, "Time{Value:7200 second}"},
		{"day", &Time{Value: 1, Unit: Day}, "Time{Value:86400 second}"},
		{"zero value", &Time{Value: 0, Unit: Hour}, "Time{Value:0 second}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.time.String()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestKeyItem_String(t *testing.T) {
	tests := []struct {
		name     string
		ki       *KeyItem
		expected string
	}{
		{"nil keyitem", nil, "KeyItem{nil}"},
		{
			"complete keyitem",
			&KeyItem{
				Key:   "rate-limit",
				Token: Token{Number: 1000, CountStrategy: TotalTokens},
				Time:  Time{Value: 1, Unit: Hour},
			},
			"KeyItem{Key:rate-limit, Token:Token{Number:1000, CountStrategy:total-tokens}, Time:Time{Value:3600 second}}",
		},
		{
			"empty key",
			&KeyItem{
				Key:   "",
				Token: Token{Number: 500, CountStrategy: InputTokens},
				Time:  Time{Value: 30, Unit: Second},
			},
			"KeyItem{Key:, Token:Token{Number:500, CountStrategy:input-tokens}, Time:Time{Value:30 second}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ki.String()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestRuleItem_String(t *testing.T) {
	tests := []struct {
		name     string
		ri       *RuleItem
		expected string
	}{
		{"nil ruleitem", nil, "RuleItem{nil}"},
		{
			"empty keyitems",
			&RuleItem{
				Identifier: Identifier{Type: AllIdentifier, Value: ".*"},
				KeyItems:   []*KeyItem{},
			},
			"RuleItem{Identifier:Identifier{Type:all, Value:.*}, KeyItems:[]}",
		},
		{
			"single keyitem",
			&RuleItem{
				Identifier: Identifier{Type: Header, Value: "user-id"},
				KeyItems: []*KeyItem{
					{
						Key:   "limit1",
						Token: Token{Number: 1000, CountStrategy: TotalTokens},
						Time:  Time{Value: 1, Unit: Hour},
					},
				},
			},
			"RuleItem{Identifier:Identifier{Type:header, Value:user-id}, KeyItems:[KeyItem{Key:limit1, Token:Token{Number:1000, CountStrategy:total-tokens}, Time:Time{Value:3600 second}}]}",
		},
		{
			"multiple keyitems",
			&RuleItem{
				Identifier: Identifier{Type: Header, Value: "api-key"},
				KeyItems: []*KeyItem{
					{
						Key:   "limit1",
						Token: Token{Number: 500, CountStrategy: InputTokens},
						Time:  Time{Value: 30, Unit: Minute},
					},
					{
						Key:   "limit2",
						Token: Token{Number: 300, CountStrategy: OutputTokens},
						Time:  Time{Value: 1, Unit: Hour},
					},
				},
			},
			"RuleItem{Identifier:Identifier{Type:header, Value:api-key}, KeyItems:[KeyItem{Key:limit1, Token:Token{Number:500, CountStrategy:input-tokens}, Time:Time{Value:1800 second}}, KeyItem{Key:limit2, Token:Token{Number:300, CountStrategy:output-tokens}, Time:Time{Value:3600 second}}]}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ri.String()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestCompleteYAMLUnmarshaling(t *testing.T) {
	yamlData := `
rules:
  - id: "rule1"
    resource: "/api/chat"
    strategy: fixed-window
    ruleName: "chat-limit"
    ruleItems:
      - identifier:
          type: header
          value: "user-id"
        keyItems:
          - key: "hourly-limit"
            token:
              number: 1000
              countStrategy: total-tokens
            time:
              value: 1
              unit: hour
          - key: "daily-limit"
            token:
              number: 10000
              countStrategy: input-tokens
            time:
              value: 1
              unit: day
redis:
  serviceName: "localhost"
  servicePort: 6379
  username: "redis-user"
  password: "redis-pass"
  timeout: 5
  poolSize: 10
  minIdleConns: 2
  maxRetries: 3
errorCode: 429
errorMessage: "Rate limit exceeded"
`

	var config Config
	err := yaml.Unmarshal([]byte(yamlData), &config)
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Validate basic structure
	if len(config.Rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(config.Rules))
	}

	rule := config.Rules[0]
	if rule.ID != "rule1" {
		t.Errorf("Expected rule ID 'rule1', got %q", rule.ID)
	}

	if rule.Strategy != FixedWindow {
		t.Errorf("Expected FixedWindow strategy, got %v", rule.Strategy)
	}

	if len(rule.RuleItems) != 1 {
		t.Errorf("Expected 1 rule item, got %d", len(rule.RuleItems))
	}

	ruleItem := rule.RuleItems[0]
	if ruleItem.Identifier.Type != Header {
		t.Errorf("Expected Header identifier type, got %v", ruleItem.Identifier.Type)
	}

	if len(ruleItem.KeyItems) != 2 {
		t.Errorf("Expected 2 key items, got %d", len(ruleItem.KeyItems))
	}

	// Test Redis config
	if config.Redis.ServiceName != "localhost" {
		t.Errorf("Expected Redis service name 'localhost', got %q", config.Redis.ServiceName)
	}

	if config.Redis.ServicePort != 6379 {
		t.Errorf("Expected Redis port 6379, got %d", config.Redis.ServicePort)
	}

	// Test error config
	if config.ErrorCode != 429 {
		t.Errorf("Expected error code 429, got %d", config.ErrorCode)
	}
}

func TestYAMLUnmarshalingErrors(t *testing.T) {
	tests := []struct {
		name     string
		yamlData string
		wantErr  bool
	}{
		{
			"invalid identifier type",
			`
rules:
  - ruleItems:
      - identifier:
          type: invalid
`,
			true,
		},
		{
			"invalid count strategy",
			`
rules:
  - ruleItems:
      - keyItems:
          - token:
              countStrategy: invalid-strategy
`,
			true,
		},
		{
			"invalid time unit",
			`
rules:
  - ruleItems:
      - keyItems:
          - time:
              unit: invalid-unit
`,
			true,
		},
		{
			"invalid strategy",
			`
rules:
  - strategy: invalid-strategy
`,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config Config
			err := yaml.Unmarshal([]byte(tt.yamlData), &config)
			if tt.wantErr && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// Benchmark tests
func BenchmarkIdentifierType_String(b *testing.B) {
	it := Header
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = it.String()
	}
}

func BenchmarkCountStrategy_String(b *testing.B) {
	cs := TotalTokens
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = cs.String()
	}
}

func BenchmarkTimeUnit_String(b *testing.B) {
	tu := Hour
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = tu.String()
	}
}

func BenchmarkToken_String(b *testing.B) {
	token := &Token{Number: 1000, CountStrategy: TotalTokens}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = token.String()
	}
}

func BenchmarkRuleItem_String(b *testing.B) {
	ri := &RuleItem{
		Identifier: Identifier{Type: Header, Value: "user-id"},
		KeyItems: []*KeyItem{
			{
				Key:   "limit1",
				Token: Token{Number: 1000, CountStrategy: TotalTokens},
				Time:  Time{Value: 1, Unit: Hour},
			},
		},
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ri.String()
	}
}
