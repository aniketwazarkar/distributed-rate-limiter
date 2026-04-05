package limiter

import (
	"context"
	"time"
)

type Strategy string

const (
	StrategyTokenBucket   Strategy = "token_bucket"
	StrategyFixedWindow   Strategy = "fixed_window"
	StrategySlidingWindow Strategy = "sliding_window"
)

type LimitRule struct {
	ID        string   `json:"id"`
	Dimension string   `json:"dimension"` // e.g., "global", "ip", "user", "endpoint"
	Match     string   `json:"match"`     // e.g., "192.168.1.1", "/api/v1/users", "*"
	Strategy  Strategy `json:"strategy"`
	Rate      int      `json:"rate"`   // Allowed operations
	Burst     int      `json:"burst"`  // For Token Bucket (capacity)
	Period    int      `json:"period"` // Window duration in seconds
}

type CheckRequest struct {
	UserID   string `json:"user_id"`
	Endpoint string `json:"endpoint"`
	IP       string `json:"ip"`
}

type CheckResult struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
}

// Storage is the engine to execute the rate limiting checks against a datastore.
type Storage interface {
	CheckLimit(ctx context.Context, key string, rule LimitRule) (CheckResult, error)
}
