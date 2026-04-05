package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisStore struct {
	client redis.UniversalClient
}

func NewRedisStore(client redis.UniversalClient) Storage {
	return &redisStore{client: client}
}

// Lua script for Token Bucket
const tokenBucketScript = `
local key = KEYS[1]
local rate = tonumber(ARGV[1])
local capacity = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = 1

local fill_time = capacity / rate
local ttl = math.floor(fill_time * 2)

local last_tokens = tonumber(redis.call("HGET", key, "tokens"))
if last_tokens == nil then
  last_tokens = capacity
end

local last_refreshed = tonumber(redis.call("HGET", key, "last_refreshed"))
if last_refreshed == nil then
  last_refreshed = 0
end

local delta = math.max(0, now - last_refreshed)
local filled_tokens = math.min(capacity, last_tokens + (delta * rate))
local allowed = filled_tokens >= requested

local remaining = filled_tokens
local retry_after = 0

if allowed then
  remaining = filled_tokens - requested
  redis.call("HSET", key, "tokens", remaining)
  redis.call("HSET", key, "last_refreshed", now)
else
  -- Calculate retry_after in ms
  local need = requested - filled_tokens
  retry_after = (need / rate) * 1000
end

redis.call("EXPIRE", key, ttl)

return {allowed and 1 or 0, math.floor(remaining), math.floor(retry_after)}
`

// Lua script for Fixed Window
const fixedWindowScript = `
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])

local current = tonumber(redis.call("GET", key) or "0")
if current >= limit then
  local ttl = redis.call("PTTL", key)
  if ttl < 0 then
    ttl = window * 1000
    redis.call("PEXPIRE", key, ttl)
  end
  return {0, 0, ttl}
end

redis.call("INCR", key)
if current == 0 then
  redis.call("EXPIRE", key, window)
end

return {1, limit - current - 1, 0}
`

// Lua script for Sliding Window Log using Sorted Sets
const slidingWindowLogScript = `
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2]) * 1000 -- Convert to ms
local now = tonumber(ARGV[3]) -- Current time in ms
local clear_before = now - window

-- Remove old elements
redis.call("ZREMRANGEBYSCORE", key, 0, clear_before)

-- Count current elements
local current_count = redis.call("ZCARD", key)

if current_count >= limit then
  -- Calculate when the oldest request falls out of the window
  local oldest = redis.call("ZRANGE", key, 0, 0, "WITHSCORES")
  local retry_after = 0
  if oldest and oldest[2] then
     retry_after = (tonumber(oldest[2]) + window) - now
  end
  return {0, 0, math.max(0, retry_after)}
end

-- Add new element
redis.call("ZADD", key, now, ARGV[4])
redis.call("PEXPIRE", key, window / 1000)

return {1, limit - current_count - 1, 0}
`

func (r *redisStore) CheckLimit(ctx context.Context, key string, rule LimitRule) (CheckResult, error) {
	var err error
	var res interface{}
	now := time.Now()

	// Using hashtag {} to ensure all keys for this rule route to same cluster node
	hashKey := fmt.Sprintf("{%s}:%s", rule.ID, key)

	switch rule.Strategy {
	case StrategyTokenBucket:
		capacity := rule.Burst
		if capacity == 0 {
			capacity = rule.Rate
		}
		nowFloat := float64(now.UnixNano()) / 1e9
		ratePerSec := float64(rule.Rate) / float64(rule.Period)

		res, err = r.client.Eval(ctx, tokenBucketScript, []string{hashKey}, ratePerSec, capacity, nowFloat).Result()

	case StrategyFixedWindow:
		res, err = r.client.Eval(ctx, fixedWindowScript, []string{hashKey}, rule.Rate, rule.Period).Result()

	case StrategySlidingWindow:
		nowMs := now.UnixNano() / 1e6
		uniq := fmt.Sprintf("%d-%d", nowMs, now.UnixNano())
		res, err = r.client.Eval(ctx, slidingWindowLogScript, []string{hashKey}, rule.Rate, rule.Period, nowMs, uniq).Result()

	default:
		return CheckResult{}, fmt.Errorf("unknown strategy: %v", rule.Strategy)
	}

	if err != nil {
		return CheckResult{}, err
	}

	arr, ok := res.([]interface{})
	if !ok || len(arr) != 3 {
		return CheckResult{}, fmt.Errorf("unexpected script result type")
	}

	allowed := arr[0].(int64) == 1
	remaining := arr[1].(int64)
	retryAfter := arr[2].(int64)

	return CheckResult{
		Allowed:    allowed,
		Remaining:  int(remaining),
		RetryAfter: time.Duration(retryAfter) * time.Millisecond,
	}, nil
}
