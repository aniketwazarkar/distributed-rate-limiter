package config

import (
	"log"
	"os"
	"strings"

	"ratelimiter/internal/limiter"
)

type Config struct {
	RedisAddr  string
	ServerPort string
}

func LoadConfig() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	return Config{
		RedisAddr:  redisAddr,
		ServerPort: port,
	}
}

// Global rules store, to be initialized and queried by the HTTP limits middleware
// In a real prod scenario, this would be periodically polled from a DB or pushed via Redis Pub/Sub
var ActiveRules = map[string]limiter.LimitRule{
	"global_rule": {
		ID:        "global",
		Dimension: "global",
		Match:     "*",
		Strategy:  limiter.StrategyFixedWindow,
		Rate:      10000,
		Period:    1,
	},
	"api_example": {
		ID:        "api_check",
		Dimension: "endpoint",
		Match:     "/check",
		Strategy:  limiter.StrategySlidingWindow,
		Rate:      1000,
		Period:    1,
	},
}

func GetRulesForRequest(req limiter.CheckRequest) []limiter.LimitRule {
	var rules []limiter.LimitRule

	for _, r := range ActiveRules {
		switch r.Dimension {
		case "global":
			rules = append(rules, r)
		case "endpoint":
			if r.Match == req.Endpoint || strings.HasPrefix(req.Endpoint, r.Match) {
				rules = append(rules, r)
			}
		case "user":
			if r.Match == req.UserID || r.Match == "*" {
				rules = append(rules, r)
			}
		case "ip":
			if r.Match == req.IP || r.Match == "*" {
				rules = append(rules, r)
			}
		}
	}
	return rules
}

// AddOrUpdateRule sets a rule in memory
func AddOrUpdateRule(r limiter.LimitRule) {
	log.Printf("Updating rule %s", r.ID)
	ActiveRules[r.ID] = r
}

func DeleteRule(id string) {
	delete(ActiveRules, id)
}
