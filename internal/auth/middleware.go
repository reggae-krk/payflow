package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func RequireAuth(ctx *gin.Context) {
	authHeader := ctx.GetHeader("Authorization")

	if authHeader == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error" : "not authorized"})
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
    
    if token == authHeader {
        ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
        return
    }

	userId, err := VerifyToken(token)

	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, err)
		return
	}
	
	ctx.Set("user_id", userId)
	ctx.Next()
}