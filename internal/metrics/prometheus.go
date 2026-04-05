package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	CheckLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "ratelimit_check_latency_seconds",
		Help:    "Latency of the rate limiter check requests.",
		Buckets: []float64{0.001, 0.002, 0.005, 0.010, 0.020, 0.050, 0.100, 0.250}, // Tight buckets since we aim for <20ms
	})

	RedisFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ratelimit_redis_failures_total",
		Help: "Total number of Redis connection/script errors triggering fallback.",
	})

	FallbackFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ratelimit_fallback_failures_total",
		Help: "Total number of fallback store errors.",
	})

	RateLimitRejections = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ratelimit_rejections_total",
			Help: "Total number of requests rejected due to rate limiting.",
		},
		[]string{"endpoint"},
	)
)
