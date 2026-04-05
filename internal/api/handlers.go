package api

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"ratelimiter/internal/config"
	"ratelimiter/internal/limiter"
)

type Handler struct {
	manager *limiter.Manager
}

func NewHandler(manager *limiter.Manager) *Handler {
	return &Handler{manager: manager}
}

// POST /check
func (h *Handler) CheckRateLimit(c *gin.Context) {
	var req limiter.CheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	// Fallback to IP from connection if not provided
	if req.IP == "" {
		req.IP = c.ClientIP()
	}

	// Apply resolving layer
	rules := config.GetRulesForRequest(req)

	if len(rules) == 0 {
		c.Header("X-RateLimit-Remaining", "unlimited")
		c.JSON(http.StatusOK, gin.H{"allowed": true, "message": "no rules matched"})
		return
	}

	res, err := h.manager.Check(c.Request.Context(), req, rules)
	if err != nil {
		log.Printf("Error processing rate limit context: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Service unavailable or config error"})
		return
	}

	// X-RateLimit Headers
	// Typically X-RateLimit-Limit would also be returned, but because of multiple matching rules we summarize here
	c.Header("X-RateLimit-Remaining", strconv.Itoa(res.Remaining))

	if !res.Allowed {
		// Setting standard X-RateLimit-Retry-After header
		c.Header("X-RateLimit-Retry-After", strconv.FormatFloat(res.RetryAfter.Seconds(), 'f', 2, 64))
		// Http 429 Too Many Requests
		c.JSON(http.StatusTooManyRequests, gin.H{
			"allowed":     false,
			"remaining":   res.Remaining,
			"retry_after": res.RetryAfter.Seconds(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"allowed":     true,
		"remaining":   res.Remaining,
		"retry_after": 0,
	})
}

// Admin APIs

// GET /config
func (h *Handler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, config.ActiveRules)
}

// POST /config
func (h *Handler) UpdateConfig(c *gin.Context) {
	var rule limiter.LimitRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid rule payload"})
		return
	}

	if rule.ID == "" || rule.Strategy == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Rule ID and Strategy are required"})
		return
	}

	config.AddOrUpdateRule(rule)
	c.JSON(http.StatusOK, gin.H{"status": "success", "rule": rule})
}

// DELETE /config/:id
func (h *Handler) DeleteConfig(c *gin.Context) {
	id := c.Param("id")
	config.DeleteRule(id)
	c.JSON(http.StatusOK, gin.H{"status": "success", "deleted": id})
}
