package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestRequireAuth(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-pass")
	t.Run("no token - returns 401", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		
		router := gin.New()
		router.Use(RequireAuth)
		
		var handlerCalled bool
		router.GET("/test", func(c *gin.Context) {
			handlerCalled = true
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})
		
		req, _ := http.NewRequest("GET", "/test", nil)
		
		router.ServeHTTP(w, req)
		
		assert.False(t, handlerCalled, "handler should not be called")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "not authorized")
	})
	
	t.Run("invalid token - returns 401", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		
		router := gin.New()
		router.Use(RequireAuth)
		
		var handlerCalled bool
		router.GET("/test", func(c *gin.Context) {
			handlerCalled = true
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})
		
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "invalid-token")
		
		router.ServeHTTP(w, req)
		
		assert.False(t, handlerCalled, "handler should not be called")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
	
	t.Run("valid token - sets user_id and continues", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		
		router := gin.New()
		router.Use(RequireAuth)
		
		token, err := GenerateToken(123)
		assert.NoError(t, err)
		
		var handlerCalled bool
		var capturedUserID interface{}
		router.GET("/test", func(c *gin.Context) {
			handlerCalled = true
			capturedUserID, _ = c.Get("user_id")
			c.JSON(http.StatusOK, gin.H{"user_id": capturedUserID})
		})
		
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		
		router.ServeHTTP(w, req)
		
		assert.True(t, handlerCalled, "handler should be called")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, int64(123), capturedUserID)
	})
}

func TestVerifyToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-pass")
	t.Run("valid token - returns user ID", func(t *testing.T) {
		userID := int64(456)
		token, err := GenerateToken(userID)
		assert.NoError(t, err)
		
		verifiedID, err := VerifyToken(token)
		assert.NoError(t, err)
		assert.Equal(t, userID, verifiedID)
	})
	
	t.Run("expired token - returns error", func(t *testing.T) {
		claims := Claims{
			UserID: 789,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)),
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Minute)),
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, _ := token.SignedString([]byte("test-secret"))
		
		_, err := VerifyToken(tokenString)
		assert.Error(t, err)
	})
	
	t.Run("invalid token - returns error", func(t *testing.T) {
		_, err := VerifyToken("invalid-token")
		assert.Error(t, err)
	})
}