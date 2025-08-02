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

// ================================= Config ====================================
const (
	DefaultResourcePattern        string = ".*"
	DefaultRuleName               string = "overall-rule"
	DefaultIdentifierValuePattern string = ".*"
	DefaultKeyPattern             string = ".*"

	DefaultRedisServiceName  string = "127.0.0.1"
	DefaultRedisServicePort  int32  = 6379
	DefaultRedisTimeout      int32  = 1000
	DefaultRedisPoolSize     int32  = 10
	DefaultRedisMinIdleConns int32  = 5
	DefaultRedisMaxRetries   int32  = 3

	DefaultErrorCode    int32  = 429
	DefaultErrorMessage string = "Too Many Requests"
)

// ================================= CommonError ==============================
const (
	ErrorTimeDuration int64 = -1
)

// ================================= Context ==================================
const (
	KeyContext        string = "llmTokenRatelimitContext"
	KeyRequestInfos   string = "llmTokenRatelimitReqInfos"
	KeyUsedTokenInfos string = "llmTokenRatelimitUsedTokenInfos"
	KeyMatchedRules   string = "llmTokenRatelimitMatchedRules"
	KeyLLMPrompts     string = "llmTokenRatelimitLLMPrompts"
)

// ================================= RedisRatelimitKeyFormat ==================
const (
	RedisRatelimitKeyFormat string = "sentinel-go:llm-token-ratelimit:%s:%s:%s:%d:%s" // ruleName, strategy, identifierType, timeWindow, tokenCountStrategy
)

// ================================= FixedWindowStrategy ======================
const (
	FixedWindowQueryScript string = `
	local ttl = redis.call('ttl', KEYS[1])
	if ttl < 0 then
		redis.call('set', KEYS[1], ARGV[1], 'EX', ARGV[2])
		return {ARGV[1], ARGV[1], ARGV[2]}
	end
	return {ARGV[1], redis.call('get', KEYS[1]), ttl}
	`
	FixedWindowUpdateScript string = `
	local ttl = redis.call('ttl', KEYS[1])
	if ttl < 0 then
		redis.call('set', KEYS[1], ARGV[1]-ARGV[3], 'EX', ARGV[2])
		return {ARGV[1], ARGV[1]-ARGV[3], ARGV[2]}
	end
	return {ARGV[1], redis.call('decrby', KEYS[1], ARGV[3]), ttl}
	`
)

// ================================= PETAStrategy =============================
const (
	PETANoWaiting              int64  = 0
	PETASlidingWindowKeyFormat string = "{peta-v1}:sliding-window:%s" // redisRatelimitKey
	PETATokenBucketKeyFormat   string = "{peta-v1}:token-bucket:%s"   // redisRatelimitKey
	PETARandomStringLength     int    = 16
)

// ================================= Generate Random String ===================
const (
	RandomLetterBytes   = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	RandomLetterIdxBits = 6
	RandomLetterIdxMask = 1<<RandomLetterIdxBits - 1
	RandomLetterIdxMax  = 63 / RandomLetterIdxBits
)

// ================================= RedisKeyForbiddenChars ===================
var RedisKeyForbiddenChars = map[string]string{
	// Control characters
	" ":    "space",
	"\n":   "newline",
	"\r":   "carriage return",
	"\t":   "tab",
	"\x00": "null byte",
	"\x01": "start of heading",
	"\x02": "start of text",
	"\x03": "end of text",
	"\x04": "end of transmission",
	"\x05": "enquiry",
	"\x06": "acknowledge",
	"\x07": "bell",
	"\x08": "backspace",
	"\x0B": "vertical tab",
	"\x0C": "form feed",
	"\x0E": "shift out",
	"\x0F": "shift in",
	"\x10": "data link escape",
	"\x11": "device control 1",
	"\x12": "device control 2",
	"\x13": "device control 3",
	"\x14": "device control 4",
	"\x15": "negative acknowledge",
	"\x16": "synchronous idle",
	"\x17": "end of transmission block",
	"\x18": "cancel",
	"\x19": "end of medium",
	"\x1A": "substitute",
	"\x1B": "escape",
	"\x1C": "file separator",
	"\x1D": "group separator",
	"\x1E": "record separator",
	"\x1F": "unit separator",
	"\x7F": "delete",

	// Special characters that may cause issues
	"*":  "asterisk (reserved for wildcard)",
	"?":  "question mark (reserved for wildcard)",
	"[":  "left bracket (reserved for character class)",
	"]":  "right bracket (reserved for character class)",
	"{":  "left brace (reserved for quantifier)",
	"}":  "right brace (reserved for quantifier)",
	"(":  "left parenthesis (reserved for grouping)",
	")":  "right parenthesis (reserved for grouping)",
	"|":  "pipe (reserved for alternation)",
	"^":  "caret (reserved for start anchor)",
	"$":  "dollar (reserved for end anchor)",
	"+":  "plus (reserved for quantifier)",
	".":  "dot (reserved for any character)",
	"\\": "backslash (reserved for escape)",
	"/":  "forward slash (potential path separator)",

	// Redis protocol related characters
	"\r\n": "CRLF sequence",

	// Unicode control characters (partial)
	"\u0080": "padding character",
	"\u0081": "high octet preset",
	"\u0082": "break permitted here",
	"\u0083": "no break here",
	"\u0084": "index",
	"\u0085": "next line",
	"\u0086": "start of selected area",
	"\u0087": "end of selected area",
	"\u0088": "character tabulation set",
	"\u0089": "character tabulation with justification",
	"\u008A": "line tabulation set",
	"\u008B": "partial line forward",
	"\u008C": "partial line backward",
	"\u008D": "reverse line feed",
	"\u008E": "single shift two",
	"\u008F": "single shift three",
	"\u0090": "device control string",
	"\u0091": "private use one",
	"\u0092": "private use two",
	"\u0093": "set transmit state",
	"\u0094": "cancel character",
	"\u0095": "message waiting",
	"\u0096": "start of guarded area",
	"\u0097": "end of guarded area",
	"\u0098": "start of string",
	"\u0099": "single graphic character introducer",
	"\u009A": "single character introducer",
	"\u009B": "control sequence introducer",
	"\u009C": "string terminator",
	"\u009D": "operating system command",
	"\u009E": "privacy message",
	"\u009F": "application program command",

	// Other potentially problematic characters
	"\"": "double quote (potential JSON escape)",
	"'":  "single quote (potential SQL injection)",
	"`":  "backtick (potential command injection)",
	"<":  "less than (potential XSS)",
	">":  "greater than (potential XSS)",
	"&":  "ampersand (potential HTML entity)",
	";":  "semicolon (potential command separator)",
	":":  "colon (potential protocol separator)",
	"=":  "equals (potential assignment)",
	",":  "comma (potential CSV separator)",
}
