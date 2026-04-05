package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"ratelimiter/internal/api"
	"ratelimiter/internal/config"
	"ratelimiter/internal/limiter"
)

func main() {
	cfg := config.LoadConfig()

	// Initialize Redis Cluster Client (or fallback to UniversalClient with 1 node for local tests)
	rdb := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    []string{cfg.RedisAddr},
		PoolSize: 1000, // High pool size for high concurrency
	})

	// Test Redis connection
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Printf("Warning: Failed to connect to Redis at %s: %v. Running in degraded mode (memory limiters only)", cfg.RedisAddr, err)
	} else {
		log.Printf("Connected to Redis successfully at %s", cfg.RedisAddr)
	}

	redisStore := limiter.NewRedisStore(rdb)
	memoryStore := limiter.NewMemoryStore()

	// Fallback mechanism handling
	manager := limiter.NewManager(redisStore, memoryStore)

	handler := api.NewHandler(manager)

	// Set Gin to release mode for performance if needed
	// gin.SetMode(gin.ReleaseMode)

	r := gin.Default()

	// Handle CORS for local/dashboard testing
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Serve the Frontend Dashboard
	r.Static("/dashboard", "./public")
	// Forward root to dashboard
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/dashboard/")
	})

	// Metric exposure
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Core check endpoint
	r.POST("/check", handler.CheckRateLimit)

	// Admin config endpoints
	admin := r.Group("/config")
	admin.Use(api.AdminAuthMiddleware())
	{
		admin.GET("", handler.GetConfig)
		admin.POST("", handler.UpdateConfig)
		admin.DELETE("/:id", handler.DeleteConfig)
	}

	// Basic health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "UP"})
	})

	log.Printf("Rate Limiter Service starting on port %s...", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
