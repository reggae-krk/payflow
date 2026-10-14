package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func RequireAuth(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")

	if authHeader == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authorized"})
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")

	if token == authHeader {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
		return
	}

	userId, err := VerifyToken(token)

	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, err)
		return
	}

	c.Set("user_id", userId)
	c.Next()
}
