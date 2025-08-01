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

-- KEYS[1]: Sliding Window Key ("{peta-v<x>}:sliding-window:<redisRatelimitKey>")
-- KEYS[2]: Token Bucket Key ("{peta-v<x>}:token-bucket:<redisRatelimitKey>")

-- ARGV[1]: Estimated token consumption
-- ARGV[2]: Current timestamp (milliseconds)
-- ARGV[3]: Token bucket capacity
-- ARGV[4]: Window size (milliseconds)
-- ARGV[5]: Actual token consumption

local sliding_window_key = tostring(KEYS[1])
local token_bucket_key = tostring(KEYS[2])

local estimated = tonumber(ARGV[1])
local current_timestamp = tonumber(ARGV[2])
local bucket_capacity = tonumber(ARGV[3])
local window_size = tonumber(ARGV[4])
local actual = tonumber(ARGV[5])

-- Valid window start time
local window_start = current_timestamp - window_size
-- Get bucket
local bucket = redis.call('HMGET', token_bucket_key, 'capacity', 'max_capacity')
local current_capacity = tonumber(bucket[1])
local max_capacity = tonumber(bucket[2])
-- Initialize bucket manually if it doesn't exist
if not current_capacity then
    redis.call('HMSET', token_bucket_key, 
        'capacity', bucket_capacity, 
        'max_capacity', bucket_capacity
    )
    redis.call('ZADD', sliding_window_key, current_timestamp, 0)
end

-- Calculate expired tokens
local released_token_list = redis.call('ZRANGEBYSCORE', sliding_window_key, 0, window_start)
local released_tokens = 0
for i = 1, #released_token_list do
    released_tokens = released_tokens + tonumber(released_token_list[i])
end

if released_tokens > 0 then -- Expired tokens exist, attempt to replenish new tokens
    -- Clean up expired data
    redis.call('ZREMRANGEBYSCORE', sliding_window_key, 0, window_start)
    -- Calculate valid tokens
    local valid_token_list = redis.call('ZRANGE', sliding_window_key, 0, -1)
    local valid_tokens = 0
    for i = 1, #valid_token_list do
        valid_tokens = valid_tokens + tonumber(valid_token_list[i])
    end
    -- Update token count
    if current_capacity + released_tokens > max_capacity then -- If current capacity plus released tokens exceeds max capacity, reset to max capacity minus valid tokens
        current_capacity = max_capacity - valid_tokens
    else -- Otherwise, directly add the released tokens
        current_capacity = current_capacity + released_tokens
    end
    -- Immediately replenish new tokens
    redis.call('HSET', token_bucket_key, 'capacity', current_capacity)
end

-- Calculate prediction error
local predicted_error = math.abs(actual - estimated)
-- Correction result
local correct_result = 1
-- Mainly handle underestimation cases to properly limit actual usage; overestimation may reject requests but won't affect downstream services
if estimated < actual then -- Underestimation
    -- directly deduct all underestimated tokens
    current_capacity = current_capacity - predicted_error
    redis.call('HSET', token_bucket_key, 'capacity', current_capacity)
    -- Get the latest valid timestamp
    local last_valid_window = redis.call('ZRANGE', sliding_window_key, -1, -1, 'WITHSCORES')
    local compensation_start = tonumber(last_valid_window[2])
    if not compensation_start then -- Possibly all data just expired, use current timestamp minus window size as start
        compensation_start = current_timestamp
    end
    while predicted_error ~= 0 do -- Distribute to future windows until all error is distributed
        if max_capacity >= predicted_error then
            local L = compensation_start
            local R = compensation_start + window_size
            while L < R do
                local mid = math.floor((L + R) / 2)
                local valid_list = redis.call('ZRANGEBYSCORE', sliding_window_key, mid - window_size, mid)
                local valid_tokens = 0
                for i = 1, #valid_list do
                    valid_tokens = valid_tokens + tonumber(valid_list[i])
                end
                if valid_tokens + predicted_error <= max_capacity then
                    R = mid
                else
                    L = mid + 1
                end
            end
            redis.call('ZADD', sliding_window_key, L, predicted_error)
            predicted_error = 0
        else
            redis.call('ZADD', sliding_window_key, compensation_start, max_capacity)
            predicted_error = predicted_error - max_capacity
            compensation_start = compensation_start + window_size
        end
    end
end

-- Set expiration time to window size plus 5 seconds buffer
redis.call('pexpire', sliding_window_key, window_size + 5000)
redis.call('pexpire', token_bucket_key, window_size + 5000)

return {correct_result}


