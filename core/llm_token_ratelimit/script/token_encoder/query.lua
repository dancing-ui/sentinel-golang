-- Copyright 1999-2020 Alibaba Group Holding Ltd.
--
-- Licensed under the Apache License, Version 2.0 (the "License");
-- you may not use this file except in compliance with the License.
-- You may obtain a copy of the License at
--
--     http://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing, software
-- distributed under the License is distributed on an "AS IS" BASIS,
-- WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
-- See the License for the specific language governing permissions and
-- limitations under the License.
-- KEYS[1]: Token Encoder Key ("<redisRatelimitKey>:token-encoder:<provider>:<model>")
-- ARGV[1]: Tokenization Length
-- ARGV[2]: Expiration (milliseconds)
local key = KEYS[1]

local tokenization = tonumber(ARGV[1])
local expiration = tonumber(ARGV[2])

local ttl = redis.call('PTTL', key)
if ttl < 0 then
    return {tokenization, 0}
end

local difference = tonumber(redis.call('GET', key))
if difference + tokenization < 0 then
    redis.call('SET', key, 0, 'PX', expiration + 5000)
    return {tokenization, 0}
end
return {difference + tokenization, difference}
