package limiter

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type memoryRecord struct {
	count     int
	expiresAt time.Time
}

type memoryStore struct {
	mu     sync.RWMutex
	limits map[string]*memoryRecord
}

func NewMemoryStore() Storage {
	m := &memoryStore{
		limits: make(map[string]*memoryRecord),
	}
	// Simple cleanup routine
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			m.cleanup()
		}
	}()
	return m
}

func (m *memoryStore) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for k, v := range m.limits {
		if now.After(v.expiresAt) {
			delete(m.limits, k)
		}
	}
}

// CheckLimit implements a very basic Fixed Window counter as a failsafe
func (m *memoryStore) CheckLimit(ctx context.Context, key string, rule LimitRule) (CheckResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	hashKey := fmt.Sprintf("%s:%s", rule.ID, key)
	now := time.Now()

	record, exists := m.limits[hashKey]
	if !exists || now.After(record.expiresAt) {
		record = &memoryRecord{
			count:     0,
			expiresAt: now.Add(time.Duration(rule.Period) * time.Second),
		}
		m.limits[hashKey] = record
	}

	if record.count >= rule.Rate {
		retryAfter := record.expiresAt.Sub(now)
		return CheckResult{
			Allowed:    false,
			Remaining:  0,
			RetryAfter: retryAfter,
		}, nil
	}

	record.count++
	return CheckResult{
		Allowed:    true,
		Remaining:  rule.Rate - record.count,
		RetryAfter: 0,
	}, nil
}
