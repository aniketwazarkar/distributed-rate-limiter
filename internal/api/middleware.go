package api

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// AdminAuthMiddleware validates tokens to protect configuration endpoints
func AdminAuthMiddleware() gin.HandlerFunc {
	expectedToken := os.Getenv("ADMIN_TOKEN")
	if expectedToken == "" {
		// Provide a default simple token for demo/local testing purposes
		expectedToken = "secret-admin-token"
	}

	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader != "Bearer "+expectedToken {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized admin access"})
			return
		}
		c.Next()
	}
}
