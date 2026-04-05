package limiter

import (
	"context"
	"fmt"
	"log"
	"ratelimiter/internal/metrics"
	"time"
)

type Manager struct {
	primary  Storage
	fallback Storage
}

func NewManager(primary Storage, fallback Storage) *Manager {
	return &Manager{
		primary:  primary,
		fallback: fallback,
	}
}

func (m *Manager) Check(ctx context.Context, req CheckRequest, rules []LimitRule) (CheckResult, error) {
	start := time.Now()
	defer func() {
		metrics.CheckLatency.Observe(time.Since(start).Seconds())
	}()

	var finalAllowed = true
	var minRemaining = -1
	var maxRetryAfter time.Duration

	// Check all matching rules. If any rule denies, the request is denied.
	for _, rule := range rules {
		// Construct the key for the datastore based on the dimension
		var key string
		switch rule.Dimension {
		case "global":
			key = "global"
		case "user":
			key = req.UserID
		case "ip":
			key = req.IP
		case "endpoint":
			key = req.Endpoint
		default:
			key = fmt.Sprintf("%s:%s", req.UserID, req.IP)
		}

		// Use IP as fallback for empty user or something similar to avoid empty keys modifying global
		if key == "" {
			key = "unknown"
		}

		res, err := m.primary.CheckLimit(ctx, key, rule)
		if err != nil {
			log.Printf("Primary redis store failed for rule %s (circuit-broken): %v", rule.ID, err)
			metrics.RedisFailures.Inc()
			// Fallback to memory store
			res, err = m.fallback.CheckLimit(ctx, key, rule)
			if err != nil {
				log.Printf("Fallback store also failed: %v", err)
				metrics.FallbackFailures.Inc()
				// Fail-open or Fail-closed? Usually limiters fail-open to not break the service
				return CheckResult{Allowed: true, Remaining: 1, RetryAfter: 0}, nil
			}
		}

		if !res.Allowed {
			finalAllowed = false
		}
		if minRemaining == -1 || res.Remaining < minRemaining {
			minRemaining = res.Remaining
		}
		if res.RetryAfter > maxRetryAfter {
			maxRetryAfter = res.RetryAfter
		}
	}

	if minRemaining < 0 {
		minRemaining = 0
	}

	if !finalAllowed {
		metrics.RateLimitRejections.WithLabelValues(req.Endpoint).Inc()
	}

	return CheckResult{
		Allowed:    finalAllowed,
		Remaining:  minRemaining,
		RetryAfter: maxRetryAfter,
	}, nil
}
